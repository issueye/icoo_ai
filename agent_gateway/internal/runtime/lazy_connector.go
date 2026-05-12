package runtime

import (
	"context"
	"sync"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type lazyConnector struct {
	mu      sync.Mutex
	factory func() (connector.AgentConnector, error)
	initReq models.ConnectorInitializeRequest
	conn    connector.AgentConnector
}

func newLazyConnector(factory func() (connector.AgentConnector, error), initReq models.ConnectorInitializeRequest) *lazyConnector {
	return &lazyConnector{factory: factory, initReq: initReq}
}

func (l *lazyConnector) Initialize(context.Context, models.ConnectorInitializeRequest) (models.ConnectorInitializeResponse, error) {
	return models.ConnectorInitializeResponse{}, nil
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

func (l *lazyConnector) NewSession(ctx context.Context, req models.ConnectorNewSessionRequest) (models.ConnectorNewSessionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorNewSessionResponse{}, err
	}
	return conn.NewSession(ctx, req)
}

func (l *lazyConnector) ListSessions(ctx context.Context, req models.ConnectorListSessionsRequest) (models.ConnectorListSessionsResponse, error) {
	l.mu.Lock()
	conn := l.conn
	l.mu.Unlock()
	if conn == nil {
		return models.ConnectorListSessionsResponse{Sessions: []models.ConnectorSessionInfo{}}, nil
	}
	return conn.ListSessions(ctx, req)
}

func (l *lazyConnector) ResumeSession(ctx context.Context, req models.ConnectorResumeSessionRequest) (models.ConnectorResumeSessionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorResumeSessionResponse{}, err
	}
	return conn.ResumeSession(ctx, req)
}

func (l *lazyConnector) CloseSession(ctx context.Context, req models.ConnectorCloseSessionRequest) (models.ConnectorCloseSessionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorCloseSessionResponse{}, err
	}
	return conn.CloseSession(ctx, req)
}

func (l *lazyConnector) SetSessionMode(ctx context.Context, req models.ConnectorSetSessionModeRequest) (models.ConnectorSetSessionModeResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorSetSessionModeResponse{}, err
	}
	return conn.SetSessionMode(ctx, req)
}

func (l *lazyConnector) SetSessionConfigOption(ctx context.Context, req models.ConnectorSetSessionConfigOptionRequest) (models.ConnectorSetSessionConfigOptionResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorSetSessionConfigOptionResponse{}, err
	}
	return conn.SetSessionConfigOption(ctx, req)
}

func (l *lazyConnector) Prompt(ctx context.Context, req models.ConnectorPromptRequest) (models.ConnectorPromptResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorPromptResponse{}, err
	}
	return conn.Prompt(ctx, req)
}

func (l *lazyConnector) Cancel(ctx context.Context, req models.ConnectorCancelRequest) (models.ConnectorCancelResponse, error) {
	conn, err := l.ensureConnected(ctx)
	if err != nil {
		return models.ConnectorCancelResponse{}, err
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
