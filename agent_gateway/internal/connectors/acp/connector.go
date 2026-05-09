package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

type Connector struct {
	writeMu     sync.Mutex
	pendingMu   sync.Mutex
	process     Process
	reader      *bufio.Reader
	writer      io.Writer
	idCounter   uint64
	eventSeq    uint64
	closeOnce   sync.Once
	waitErrChan chan error
	closed      chan struct{}
	pending     map[string]chan rpcResponse
	asyncErr    error
}

type Options struct {
	Command string
	Args    []string
	Stderr  io.Writer
	Process Process
}

type DefaultConnectorOptions struct {
	Command string
	Args    []string
	Stderr  io.Writer
}

func NewDefaultConnector(opts DefaultConnectorOptions) (*Connector, error) {
	command := strings.TrimSpace(opts.Command)
	if command == "" {
		return nil, connector.NewError("invalid_connector_config", "default acp connector requires command")
	}
	return New(Options{
		Command: command,
		Args:    opts.Args,
		Stderr:  opts.Stderr,
	})
}

func New(opts Options) (*Connector, error) {
	var (
		proc Process
		err  error
	)
	if opts.Process != nil {
		proc = opts.Process
	} else {
		if opts.Command == "" {
			return nil, connector.NewError(connector.ErrCodeInvalidConnectorConfig, "acp connector requires command or process")
		}
		proc, err = newCmdProcess(opts.Command, opts.Args, opts.Stderr)
		if err != nil {
			return nil, connector.WrapError(connector.ErrCodeConnectorStartFailed, "failed to create acp process", err)
		}
	}
	if err := proc.Start(); err != nil {
		return nil, connector.WrapError(connector.ErrCodeConnectorStartFailed, "failed to start acp process", err)
	}
	c := &Connector{
		process:     proc,
		reader:      bufio.NewReader(proc.Stdout()),
		writer:      proc.Stdin(),
		waitErrChan: make(chan error, 1),
		closed:      make(chan struct{}),
		pending:     make(map[string]chan rpcResponse),
	}
	go func() {
		c.waitErrChan <- proc.Wait()
	}()
	go c.waitLoop()
	go c.readLoop()
	return c, nil
}

func (c *Connector) Initialize(ctx context.Context, req connector.InitializeRequest) (connector.InitializeResponse, error) {
	resp, err := c.call(ctx, "initialize", initializeParams(req))
	if err != nil {
		return connector.InitializeResponse{}, err
	}
	return mapInitializeResponse(resp), nil
}

func (c *Connector) NewSession(ctx context.Context, req connector.NewSessionRequest) (connector.NewSessionResponse, error) {
	resp, err := c.call(ctx, "newSession", newSessionParams(req))
	if err != nil {
		return connector.NewSessionResponse{}, err
	}
	return mapNewSessionResponse(resp), nil
}

func (c *Connector) Prompt(ctx context.Context, req connector.PromptRequest) (connector.PromptResponse, error) {
	resp, err := c.call(ctx, "prompt", promptParams(req))
	if err != nil {
		return connector.PromptResponse{}, err
	}
	return mapPromptResponse(resp), nil
}

func (c *Connector) Cancel(ctx context.Context, req connector.CancelRequest) (connector.CancelResponse, error) {
	resp, err := c.call(ctx, "cancel", cancelParams(req))
	if err != nil {
		return connector.CancelResponse{}, err
	}
	return mapCancelResponse(resp), nil
}

func (c *Connector) Close() error {
	var closeErr error
	c.closeOnce.Do(func() {
		close(c.closed)
		c.failAllPending()
		closeErr = c.process.Kill()
	})
	return closeErr
}

func (c *Connector) call(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	id := fmt.Sprintf("%d", atomic.AddUint64(&c.idCounter, 1))
	req := rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
	respCh := make(chan rpcResponse, 1)
	if err := c.registerPending(id, respCh); err != nil {
		return nil, err
	}
	defer c.unregisterPending(id)

	if err := c.write(ctx, req); err != nil {
		return nil, err
	}
	resp, err := c.awaitResponse(ctx, id, respCh)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, connector.NewError(connector.ErrCodeProtocolError, resp.Error.Message)
	}
	return resp.Result, nil
}

func (c *Connector) write(ctx context.Context, req rpcRequest) error {
	if err := ctx.Err(); err != nil {
		return connector.WrapError("connector_request_cancelled", "acp request cancelled", err)
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	raw, err := json.Marshal(req)
	if err != nil {
		return connector.WrapError(connector.ErrCodeProtocolError, "failed to encode acp request", err)
	}
	raw = append(raw, '\n')
	if _, err := c.writer.Write(raw); err != nil {
		return connector.WrapError(connector.ErrCodeIOError, "failed to write acp request", err)
	}
	return nil
}

func (c *Connector) awaitResponse(ctx context.Context, id string, respCh <-chan rpcResponse) (rpcResponse, error) {
	if err := ctx.Err(); err != nil {
		return rpcResponse{}, connector.WrapError(connector.ErrCodeRequestCancelled, "acp request cancelled", err)
	}
	select {
	case <-ctx.Done():
		return rpcResponse{}, connector.WrapError(connector.ErrCodeRequestCancelled, "acp request cancelled", ctx.Err())
	case <-c.closed:
		return rpcResponse{}, connector.NewError(connector.ErrCodeClosed, "acp connector closed")
	case resp, ok := <-respCh:
		if !ok {
			return rpcResponse{}, c.currentAsyncError()
		}
		if resp.ID != id {
			return rpcResponse{}, connector.NewError(connector.ErrCodeProtocolError, "acp response id mismatch")
		}
		return resp, nil
	}
}

func (c *Connector) readLoop() {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				c.onAsyncFailure(connector.NewError(connector.ErrCodeProcessExited, "acp process exited"))
				return
			}
			c.onAsyncFailure(connector.WrapError(connector.ErrCodeIOError, "failed to read acp response", err))
			return
		}
		var frame map[string]any
		if err := json.Unmarshal(line, &frame); err != nil {
			c.onAsyncFailure(connector.WrapError(connector.ErrCodeProtocolError, "failed to decode acp response", err))
			return
		}
		if id := stringField(frame, "id"); id != "" {
			var resp rpcResponse
			if err := json.Unmarshal(line, &resp); err != nil {
				c.onAsyncFailure(connector.WrapError(connector.ErrCodeProtocolError, "failed to decode acp response", err))
				return
			}
			c.deliverResponse(resp)
			continue
		}
		if !c.handleSessionUpdate(frame) {
			c.onAsyncFailure(connector.NewError(connector.ErrCodeProtocolError, "received unknown acp frame"))
			return
		}
	}
}

func (c *Connector) waitLoop() {
	select {
	case <-c.closed:
		return
	case err := <-c.waitErrChan:
		if err == nil {
			return
		}
		c.onAsyncFailure(connector.WrapError(connector.ErrCodeProcessExited, "acp process exited", err))
	}
}

func (c *Connector) handleSessionUpdate(frame map[string]any) bool {
	method := stringField(frame, "method")
	if method == "" {
		method = stringField(frame, "type")
	}
	if method == "" {
		return false
	}
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(method, "_", "."), "-", "."))
	if normalized != "session.update" && normalized != "sessionupdate" {
		return false
	}
	params := mapField(frame, "params")
	if params == nil {
		params = mapField(frame, "update")
	}
	event, ok := mapSessionUpdateToEnvelope(nextEventID(atomic.AddUint64(&c.eventSeq, 1)), params)
	if !ok {
		return false
	}
	events.DefaultBus().Publish(event)
	return true
}

func (c *Connector) deliverResponse(resp rpcResponse) {
	c.pendingMu.Lock()
	respCh, ok := c.pending[resp.ID]
	c.pendingMu.Unlock()
	if !ok {
		return
	}
	select {
	case respCh <- resp:
	default:
	}
}

func (c *Connector) registerPending(id string, ch chan rpcResponse) error {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.asyncErr != nil {
		return c.asyncErr
	}
	c.pending[id] = ch
	return nil
}

func (c *Connector) unregisterPending(id string) {
	c.pendingMu.Lock()
	delete(c.pending, id)
	c.pendingMu.Unlock()
}

func (c *Connector) onAsyncFailure(err error) {
	c.pendingMu.Lock()
	if c.asyncErr == nil {
		c.asyncErr = err
	}
	c.pendingMu.Unlock()
	c.failAllPending()
}

func (c *Connector) failAllPending() {
	c.pendingMu.Lock()
	for id, ch := range c.pending {
		delete(c.pending, id)
		close(ch)
	}
	c.pendingMu.Unlock()
}

func (c *Connector) currentAsyncError() error {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	if c.asyncErr != nil {
		return c.asyncErr
	}
	return connector.NewError(connector.ErrCodeIOError, "acp response stream closed")
}
