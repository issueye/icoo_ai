package acp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

// Pool wraps multiple ACP connectors and exposes a single AgentConnector.
// New sessions are distributed in round-robin mode, while prompt/cancel are
// routed back to the backend that created the session.
type Pool struct {
	backends []connector.AgentConnector

	initMu      sync.Mutex
	initialized bool
	initResp    models.ConnectorInitializeResponse
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

func (p *Pool) Initialize(ctx context.Context, req models.ConnectorInitializeRequest) (models.ConnectorInitializeResponse, error) {
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

func (p *Pool) NewSession(ctx context.Context, req models.ConnectorNewSessionRequest) (models.ConnectorNewSessionResponse, error) {
	idx, backend, err := p.nextBackend()
	if err != nil {
		return models.ConnectorNewSessionResponse{}, err
	}

	resp, err := backend.NewSession(ctx, req)
	if err != nil {
		return models.ConnectorNewSessionResponse{}, err
	}
	if resp.SessionID == "" {
		return models.ConnectorNewSessionResponse{}, connector.NewError(connector.ErrCodeProtocolError, "acp connector returned empty session id")
	}

	p.sessionMu.Lock()
	p.sessionBackend[resp.SessionID] = idx
	p.sessionMu.Unlock()
	return resp, nil
}

func (p *Pool) ListSessions(ctx context.Context, req models.ConnectorListSessionsRequest) (models.ConnectorListSessionsResponse, error) {
	var (
		errs   []error
		seen   = map[string]struct{}{}
		merged models.ConnectorListSessionsResponse
	)
	for idx, backend := range p.backends {
		resp, err := backend.ListSessions(ctx, req)
		if err != nil {
			errs = append(errs, fmt.Errorf("acp connector pool backend %d list sessions: %w", idx, err))
			continue
		}
		for _, session := range resp.Sessions {
			if session.SessionID == "" {
				continue
			}
			p.bindSession(session.SessionID, idx)
			if _, ok := seen[session.SessionID]; ok {
				continue
			}
			seen[session.SessionID] = struct{}{}
			merged.Sessions = append(merged.Sessions, session)
		}
	}
	if len(merged.Sessions) == 0 && len(errs) > 0 {
		return models.ConnectorListSessionsResponse{}, errors.Join(errs...)
	}
	return merged, nil
}

func (p *Pool) ResumeSession(ctx context.Context, req models.ConnectorResumeSessionRequest) (models.ConnectorResumeSessionResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err == nil {
		resp, callErr := backend.ResumeSession(ctx, req)
		if callErr == nil {
			return resp, nil
		}
	}
	return p.resumeSessionDiscover(ctx, req)
}

func (p *Pool) CloseSession(ctx context.Context, req models.ConnectorCloseSessionRequest) (models.ConnectorCloseSessionResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		var discoverErr error
		backend, discoverErr = p.discoverSessionBackend(ctx, req.SessionID)
		if discoverErr != nil {
			return models.ConnectorCloseSessionResponse{}, discoverErr
		}
	}
	resp, err := backend.CloseSession(ctx, req)
	if err != nil {
		return models.ConnectorCloseSessionResponse{}, err
	}
	p.unbindSession(req.SessionID)
	return resp, nil
}

func (p *Pool) SetSessionMode(ctx context.Context, req models.ConnectorSetSessionModeRequest) (models.ConnectorSetSessionModeResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		var discoverErr error
		backend, discoverErr = p.discoverSessionBackend(ctx, req.SessionID)
		if discoverErr != nil {
			return models.ConnectorSetSessionModeResponse{}, discoverErr
		}
	}
	return backend.SetSessionMode(ctx, req)
}

func (p *Pool) SetSessionConfigOption(ctx context.Context, req models.ConnectorSetSessionConfigOptionRequest) (models.ConnectorSetSessionConfigOptionResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		var discoverErr error
		backend, discoverErr = p.discoverSessionBackend(ctx, req.SessionID)
		if discoverErr != nil {
			return models.ConnectorSetSessionConfigOptionResponse{}, discoverErr
		}
	}
	return backend.SetSessionConfigOption(ctx, req)
}

func (p *Pool) Prompt(ctx context.Context, req models.ConnectorPromptRequest) (models.ConnectorPromptResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		return models.ConnectorPromptResponse{}, err
	}
	return backend.Prompt(ctx, req)
}

func (p *Pool) Cancel(ctx context.Context, req models.ConnectorCancelRequest) (models.ConnectorCancelResponse, error) {
	backend, err := p.backendForSession(req.SessionID)
	if err != nil {
		return models.ConnectorCancelResponse{}, err
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

func (p *Pool) bindSession(sessionID string, backendIndex int) {
	p.sessionMu.Lock()
	p.sessionBackend[sessionID] = backendIndex
	p.sessionMu.Unlock()
}

func (p *Pool) unbindSession(sessionID string) {
	p.sessionMu.Lock()
	delete(p.sessionBackend, sessionID)
	p.sessionMu.Unlock()
}

func (p *Pool) resumeSessionDiscover(ctx context.Context, req models.ConnectorResumeSessionRequest) (models.ConnectorResumeSessionResponse, error) {
	var errs []error
	for idx, backend := range p.backends {
		resp, err := backend.ResumeSession(ctx, req)
		if err != nil {
			errs = append(errs, fmt.Errorf("acp connector pool backend %d resume session: %w", idx, err))
			continue
		}
		p.bindSession(req.SessionID, idx)
		return resp, nil
	}
	if len(errs) > 0 {
		return models.ConnectorResumeSessionResponse{}, errors.Join(errs...)
	}
	return models.ConnectorResumeSessionResponse{}, connector.NewError(connector.ErrCodeProtocolError, "acp connector pool could not resume session")
}

func (p *Pool) discoverSessionBackend(ctx context.Context, sessionID string) (connector.AgentConnector, error) {
	if sessionID == "" {
		return nil, connector.NewError(connector.ErrCodeProtocolError, "acp session id is required")
	}
	var errs []error
	for idx, backend := range p.backends {
		resp, err := backend.ListSessions(ctx, models.ConnectorListSessionsRequest{})
		if err != nil {
			errs = append(errs, fmt.Errorf("acp connector pool backend %d discover session: %w", idx, err))
			continue
		}
		for _, session := range resp.Sessions {
			if session.SessionID != sessionID {
				continue
			}
			p.bindSession(sessionID, idx)
			return backend, nil
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return nil, connector.NewError(connector.ErrCodeProtocolError, fmt.Sprintf("acp session %q is not managed by connector pool", sessionID))
}
