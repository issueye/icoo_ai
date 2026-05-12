package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

type GatewayError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *GatewayError) Error() string {
	return e.Message
}

type GatewayCRUD interface {
	ListAgents(ctx context.Context) ([]models.Agent, error)
	ListSkills(ctx context.Context) ([]models.Skill, error)

	GetManagementSettings(ctx context.Context) (models.ManagementSettings, error)
	ReplaceManagementSettings(ctx context.Context, req models.ManagementSettings) (models.ManagementSettings, error)

	CreateSession(ctx context.Context, req models.CreateSessionRequest) (models.Session, error)
	ListSessions(ctx context.Context) ([]models.Session, error)
	GetSession(ctx context.Context, sessionID string) (models.Session, error)
	DeleteSession(ctx context.Context, sessionID string) (models.Session, error)

	ResumeSession(ctx context.Context, sessionID string, req models.ResumeSessionRequest) (models.Session, error)
	UpdateSessionMode(ctx context.Context, sessionID string, req models.SetSessionModeRequest) (models.Session, error)
	UpdateSessionConfig(ctx context.Context, sessionID string, req models.SetSessionConfigOptionRequest) (models.Session, error)

	ListSessionMessages(ctx context.Context, sessionID string) ([]models.Message, error)
	CreateSessionMessage(ctx context.Context, sessionID string, req models.PromptRequest) (models.PromptResponse, error)
	CancelSessionRun(ctx context.Context, sessionID string) (models.Run, error)

	ListRuns(ctx context.Context) ([]models.Run, error)
	ListApprovals(ctx context.Context) ([]models.Approval, error)
	UpdateApprovalDecision(ctx context.Context, approvalID string, req models.ApprovalDecisionRequest) (models.Approval, error)

	Agent() *Agent
	Channel() *Channel
	MCPServer() *MCPServer
	ScheduleTask() *ScheduleTask
}
