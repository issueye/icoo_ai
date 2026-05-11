package runtime

import (
	"context"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
)

type lazyConnector struct {
	mu      sync.Mutex
	factory func() (connector.AgentConnector, error)
	initReq connector.InitializeRequest
	conn    connector.AgentConnector
}

func newLazyConnector(factory func() (connector.AgentConnector, error), initReq connector.InitializeRequest) *lazyConnector {
	return &lazyConnector{factory: factory, initReq: initReq}
}

func (l *lazyConnector) Initialize(context.Context, connector.InitializeRequest) (connector.InitializeResponse, error) {
	return connector.InitializeResponse{}, nil
}

func (l *lazyConnector) ensureConnected(ctx context.Context) (connector.AgentConnector, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn != nil {
		return l.conn, nil
	}
	conn, err := l.factory()
	if err != nil {
		return nil, err
	}
	if _, err := conn.Initialize(ctx, l.initReq); err != nil {
		_ = conn.Close()
		return nil, err
	}
	l.conn = conn
	return l.conn, nil
}

func (l *lazyConnector) NewSession(ctx context.Context, req connector.NewSessionRequest) (connector.NewSessionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.NewSessionResponse{}, err
	}
	return conn.NewSession(ctx, req)
}

func (l *lazyConnector) ListSessions(ctx context.Context, req connector.ListSessionsRequest) (connector.ListSessionsResponse, error) {
	l.mu.Lock()
	conn := l.conn
	l.mu.Unlock()
	if conn == nil {
		return connector.ListSessionsResponse{Sessions: []connector.SessionInfo{}}, nil
	}
	return conn.ListSessions(ctx, req)
}

func (l *lazyConnector) ResumeSession(ctx context.Context, req connector.ResumeSessionRequest) (connector.ResumeSessionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.ResumeSessionResponse{}, err
	}
	return conn.ResumeSession(ctx, req)
}

func (l *lazyConnector) CloseSession(ctx context.Context, req connector.CloseSessionRequest) (connector.CloseSessionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.CloseSessionResponse{}, err
	}
	return conn.CloseSession(ctx, req)
}

func (l *lazyConnector) SetSessionMode(ctx context.Context, req connector.SetSessionModeRequest) (connector.SetSessionModeResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.SetSessionModeResponse{}, err
	}
	return conn.SetSessionMode(ctx, req)
}

func (l *lazyConnector) SetSessionConfigOption(ctx context.Context, req connector.SetSessionConfigOptionRequest) (connector.SetSessionConfigOptionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.SetSessionConfigOptionResponse{}, err
	}
	return conn.SetSessionConfigOption(ctx, req)
}

func (l *lazyConnector) Prompt(ctx context.Context, req connector.PromptRequest) (connector.PromptResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.PromptResponse{}, err
	}
	return conn.Prompt(ctx, req)
}

func (l *lazyConnector) Cancel(ctx context.Context, req connector.CancelRequest) (connector.CancelResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return connector.CancelResponse{}, err
	}
	return conn.Cancel(ctx, req)
}

func (l *lazyConnector) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return nil
	}
	err := l.conn.Close()
	l.conn = nil
	return err
}
