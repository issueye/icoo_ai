package connector

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type AgentConnector interface {
	Initialize(ctx context.Context, req models.ConnectorInitializeRequest) (models.ConnectorInitializeResponse, error)
	NewSession(ctx context.Context, req models.ConnectorNewSessionRequest) (models.ConnectorNewSessionResponse, error)
	ListSessions(ctx context.Context, req models.ConnectorListSessionsRequest) (models.ConnectorListSessionsResponse, error)
	ResumeSession(ctx context.Context, req models.ConnectorResumeSessionRequest) (models.ConnectorResumeSessionResponse, error)
	CloseSession(ctx context.Context, req models.ConnectorCloseSessionRequest) (models.ConnectorCloseSessionResponse, error)
	SetSessionMode(ctx context.Context, req models.ConnectorSetSessionModeRequest) (models.ConnectorSetSessionModeResponse, error)
	SetSessionConfigOption(ctx context.Context, req models.ConnectorSetSessionConfigOptionRequest) (models.ConnectorSetSessionConfigOptionResponse, error)
	Prompt(ctx context.Context, req models.ConnectorPromptRequest) (models.ConnectorPromptResponse, error)
	Cancel(ctx context.Context, req models.ConnectorCancelRequest) (models.ConnectorCancelResponse, error)
	Close() error
}
