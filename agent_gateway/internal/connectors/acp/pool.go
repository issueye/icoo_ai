package acp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
)

// Pool wraps multiple ACP connectors and exposes a single AgentConnector.
// New sessions are distributed in round-robin mode, while prompt/cancel are
// routed back to the backend that created the session.
type Pool struct {
	backends []connector.AgentConnector

	initMu      sync.Mutex
	initialized bool
	initResp    connector.InitializeResponse
	initErr     error

	next uint64

	sessionMu      sync.RWMutex
	sessionBackend map[string]int

	closeOnce sync.Once
	closeErr  error
}

func NewPool(backends []connector.AgentConnector) (*Pool, error) {
	if len(backends) == 0 {
		return nil, connector.NewError(connector.ErrCodeInvalidConnectorConfig, "acp connector pool requires at least one backend")
	}
	cloned := make([]connector.AgentConnector, len(backends))
	copy(cloned, backends)
	for idx, backend := range cloned {
		if backend == nil {
			return nil, connector.NewError(
				connector.ErrCodeInvalidConnectorConfig,
				fmt.Sprintf("acp connector pool backend %d is nil", idx),
			)
		}
	}
	return &Pool{
		backends:       cloned,
		sessionBackend: make(map[string]int),
	}, nil
}

func (p *Pool) Initialize(ctx context.Context, req connector.InitializeRequest) (connector.InitializeResponse, error) {
	p.initMu.Lock()
	defer p.initMu.Unlock()
	if p.initialized {
		return p.initResp, p.initErr
	}

	var errs []error
	for idx, backend := range p.backends {
		resp, err := backend.Initialize(ctx, req)
		if err != nil {
			errs = append(errs, fmt.Errorf("acp connector pool backend %d initialize: %w", idx, err))
			continue
		}
		if p.initResp.ServerName == "" && p.initResp.ServerVersion == "" {
			p.initResp = resp
		}
	}
	p.initialized = true
	if len(errs) > 0 {
		p.initErr = errors.Join(errs...)
	}
	return p.initResp, p.initErr
}

func (p *Pool) NewSession(ctx context.Context, req connector.NewSessionRequest) (connector.NewSessionResponse, error) {
	idx, backend, err := p.nextBackend()
	if err != nil {
		return connector.NewSessionResponse{}, err
	}

	resp, err := backend.NewSession(ctx, req)
	if err != nil {
		return connector.NewSessionResponse{}, err
	}
	if resp.SessionID == "" {
		return connector.NewSessionResponse{}, connector.NewError(connector.ErrCodeProtocolError, "acp connector returned empty session id")
	}

	p.sessionMu.Lock()
	p.sessionBackend[resp.SessionID] = idx
	p.sessionMu.Unlock()
	return resp, nil
}

func (p *Pool) Prompt(ctx context.Context, req connector.PromptRequest) (connector.PromptResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		return connector.PromptResponse{}, err
	}
	return backend.Prompt(ctx, req)
}

func (p *Pool) Cancel(ctx context.Context, req connector.CancelRequest) (connector.CancelResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		return connector.CancelResponse{}, err
	}
	return backend.Cancel(ctx, req)
}

func (p *Pool) Close() error {
	p.closeOnce.Do(func() {
		var errs []error
		for idx, backend := range p.backends {
			if err := backend.Close(); err != nil {
				errs = append(errs, fmt.Errorf("acp connector pool backend %d close: %w", idx, err))
			}
		}
		if len(errs) > 0 {
			p.closeErr = errors.Join(errs...)
		}
		p.sessionMu.Lock()
		clear(p.sessionBackend)
		p.sessionMu.Unlock()
	})
	return p.closeErr
}

func (p *Pool) nextBackend() (int, connector.AgentConnector, error) {
	size := len(p.backends)
	if size == 0 {
		return 0, nil, connector.NewError(connector.ErrCodeInvalidConnectorConfig, "acp connector pool has no backend")
	}
	idx := int(atomic.AddUint64(&p.next, 1)-1) % size
	return idx, p.backends[idx], nil
}

func (p *Pool) backendForSession(sessionID string) (connector.AgentConnector, error) {
	if sessionID == "" {
		return nil, connector.NewError(connector.ErrCodeProtocolError, "acp session id is required")
	}
	p.sessionMu.RLock()
	idx, ok := p.sessionBackend[sessionID]
	p.sessionMu.RUnlock()
	if !ok {
		return nil, connector.NewError(connector.ErrCodeProtocolError, fmt.Sprintf("acp session %q is not managed by connector pool", sessionID))
	}
	if idx < 0 || idx >= len(p.backends) {
		return nil, connector.NewError(connector.ErrCodeInvalidConnectorConfig, "acp connector pool session route is invalid")
	}
	return p.backends[idx], nil
}
