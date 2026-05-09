package acp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

type fakeProcess struct {
	stdinR *io.PipeReader
	stdinW *io.PipeWriter

	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter

	mu      sync.Mutex
	methods []string
	params  map[string]map[string]any
}

func newFakeProcess() *fakeProcess {
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	return &fakeProcess{
		stdinR:  stdinR,
		stdinW:  stdinW,
		stdoutR: stdoutR,
		stdoutW: stdoutW,
		params:  make(map[string]map[string]any),
	}
}

func (f *fakeProcess) Start() error { return nil }
func (f *fakeProcess) Stdin() io.WriteCloser {
	return f.stdinW
}
func (f *fakeProcess) Stdout() io.ReadCloser {
	return f.stdoutR
}
func (f *fakeProcess) Wait() error { return nil }
func (f *fakeProcess) Kill() error {
	_ = f.stdinR.Close()
	_ = f.stdinW.Close()
	_ = f.stdoutR.Close()
	return f.stdoutW.Close()
}

func (f *fakeProcess) run(t *testing.T) {
	t.Helper()
	go func() {
		reader := bufio.NewReader(f.stdinR)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				return
			}
			var req rpcRequest
			if err := json.Unmarshal(line, &req); err != nil {
				t.Errorf("unmarshal request: %v", err)
				return
			}
			f.mu.Lock()
			f.methods = append(f.methods, req.Method)
			f.params[req.Method] = req.Params
			f.mu.Unlock()

			if req.Method == "prompt" {
				update := map[string]any{
					"jsonrpc": "2.0",
					"method":  "session.update",
					"params": map[string]any{
						"type":      "run.updated",
						"agentId":   "icoo-ai-acp",
						"sessionId": asString(req.Params["sessionId"]),
						"runId":     "run_fake_1",
						"payload": map[string]any{
							"status": "in_progress",
						},
						"createdAt": "2026-05-09T12:34:56Z",
					},
				}
				rawUpdate, _ := json.Marshal(update)
				rawUpdate = append(rawUpdate, '\n')
				if _, err := f.stdoutW.Write(rawUpdate); err != nil {
					return
				}
			}

			resp := rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: f.resultFor(req.Method)}
			if req.Method == "prompt_error" {
				resp.Error = &rpcError{Code: -32000, Message: "fake protocol error"}
				resp.Result = nil
			}
			raw, _ := json.Marshal(resp)
			raw = append(raw, '\n')
			if _, err := f.stdoutW.Write(raw); err != nil {
				return
			}
		}
	}()
}

func asString(v any) string {
	out, _ := v.(string)
	return out
}

func (f *fakeProcess) resultFor(method string) map[string]any {
	switch method {
	case "initialize":
		return map[string]any{"serverName": "fake-acp", "serverVersion": "0.1.0"}
	case "newSession":
		return map[string]any{"sessionId": "sess_fake_1"}
	case "prompt":
		return map[string]any{
			"runId":  "run_fake_1",
			"output": "ok",
			"approvals": []any{
				map[string]any{
					"requestId": "req_approval_1",
					"action":    "write_file",
					"message":   "allow write",
				},
			},
		}
	case "cancel":
		return map[string]any{"runId": "run_fake_1", "status": "cancelled"}
	default:
		return map[string]any{}
	}
}

func TestFakeProcessProtocolMapping(t *testing.T) {
	fake := newFakeProcess()
	fake.run(t)

	c, err := New(Options{Process: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	sub, _ := events.DefaultBus().Subscribe(ctx, "")
	defer sub.Close()

	initResp, err := c.Initialize(ctx, connector.InitializeRequest{ClientName: "gateway", ClientVersion: "1.0.0"})
	if err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	if initResp.ServerName != "fake-acp" {
		t.Fatalf("Initialize() server name = %q", initResp.ServerName)
	}

	sessionResp, err := c.NewSession(ctx, connector.NewSessionRequest{
		AgentID: "icoo-ai-acp",
		Model:   "mock-gpt",
		CWD:     "E:/code",
	})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if sessionResp.SessionID != "sess_fake_1" {
		t.Fatalf("NewSession() session id = %q", sessionResp.SessionID)
	}

	promptResp, err := c.Prompt(ctx, connector.PromptRequest{
		SessionID: sessionResp.SessionID,
		Content:   "hello",
		RequestID: "req_1",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if promptResp.RunID != "run_fake_1" || promptResp.Output != "ok" {
		t.Fatalf("Prompt() response = %#v", promptResp)
	}
	if len(promptResp.Approvals) != 1 {
		t.Fatalf("Prompt() approvals = %#v", promptResp.Approvals)
	}
	if promptResp.Approvals[0].RequestID != "req_approval_1" || promptResp.Approvals[0].Action != "write_file" {
		t.Fatalf("Prompt() approval mapping mismatch: %#v", promptResp.Approvals[0])
	}
	select {
	case evt := <-sub.Events():
		if evt.Type != "run.updated" || evt.AgentID != "icoo-ai-acp" || evt.SessionID != sessionResp.SessionID || evt.RunID != "run_fake_1" {
			t.Fatalf("unexpected async event identity: %#v", evt)
		}
		if evt.CreatedAt.IsZero() {
			t.Fatal("expected async event createdAt")
		}
		payload, ok := evt.Payload.(map[string]any)
		if !ok || payload["status"] != "in_progress" {
			t.Fatalf("unexpected async event payload: %#v", evt.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async session.update event")
	}

	cancelResp, err := c.Cancel(ctx, connector.CancelRequest{
		SessionID: sessionResp.SessionID,
		RunID:     promptResp.RunID,
		Reason:    "user_cancel",
	})
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if cancelResp.Status != "cancelled" {
		t.Fatalf("Cancel() status = %q", cancelResp.Status)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.methods) != 4 {
		t.Fatalf("expected 4 rpc methods, got %d (%v)", len(fake.methods), fake.methods)
	}
	if fake.methods[0] != "initialize" || fake.methods[1] != "newSession" || fake.methods[2] != "prompt" || fake.methods[3] != "cancel" {
		t.Fatalf("unexpected method order: %v", fake.methods)
	}
	if got := fake.params["newSession"]["cwd"]; got != "E:/code" {
		t.Fatalf("newSession.cwd = %#v", got)
	}
	if got := fake.params["prompt"]["content"]; got != "hello" {
		t.Fatalf("prompt.content = %#v", got)
	}
}

func TestProtocolErrorToStructuredError(t *testing.T) {
	fake := newFakeProcess()
	fake.run(t)
	c, err := New(Options{Process: fake})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close()

	_, err = c.call(context.Background(), "prompt_error", nil)
	if err == nil {
		t.Fatal("expected protocol error")
	}
	structured, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if structured.Code != "connector_protocol_error" {
		t.Fatalf("unexpected error code: %q", structured.Code)
	}
}

func TestNewDefaultConnectorRequiresCommand(t *testing.T) {
	_, err := NewDefaultConnector(DefaultConnectorOptions{})
	if err == nil {
		t.Fatal("expected structured error")
	}
	structured, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if structured.Code != "invalid_connector_config" {
		t.Fatalf("unexpected error code: %q", structured.Code)
	}
}

type exitProcess struct {
	stdoutR *io.PipeReader
	stdoutW *io.PipeWriter
}

func newExitProcess() *exitProcess {
	stdoutR, stdoutW := io.Pipe()
	return &exitProcess{
		stdoutR: stdoutR,
		stdoutW: stdoutW,
	}
}

func (e *exitProcess) Start() error {
	_ = e.stdoutW.Close()
	return nil
}

func (e *exitProcess) Stdin() io.WriteCloser { return nopWriteCloser{Writer: io.Discard} }
func (e *exitProcess) Stdout() io.ReadCloser { return e.stdoutR }
func (e *exitProcess) Wait() error           { return errors.New("exit status 2") }
func (e *exitProcess) Kill() error {
	_ = e.stdoutR.Close()
	return nil
}

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

func TestProcessExitMapsToStructuredError(t *testing.T) {
	c, err := New(Options{Process: newExitProcess()})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err = c.Initialize(ctx, connector.InitializeRequest{ClientName: "gateway", ClientVersion: "1.0.0"})
	if err == nil {
		t.Fatal("expected process exit error")
	}
	structured, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if structured.Code != connector.ErrCodeProcessExited {
		t.Fatalf("unexpected error code: %q", structured.Code)
	}
}
