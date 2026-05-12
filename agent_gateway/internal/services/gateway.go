package services

import (
	"context"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	legacy "github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

type GatewayError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *GatewayError) Error() string {
	return e.Message
}

type GatewayCRUD interface {
	ListAgents(ctx context.Context) ([]models.AgentProfile, error)
	ListSkills(ctx context.Context) ([]models.Skill, error)

	GetManagementSettings(ctx context.Context) (models.ManagementSettings, error)
	ReplaceManagementSettings(ctx context.Context, req models.ManagementSettings) (models.ManagementSettings, error)
	ListChannelStatuses(ctx context.Context) ([]models.ChannelRuntimeStatus, error)
	StartChannels(ctx context.Context) error
	StopChannels(ctx context.Context) error

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

	AgentConfigs() AgentConfigService
	ChannelConfigs() ChannelConfigService
	MCPServerConfigs() MCPServerConfigService
	ScheduleTaskConfigs() ScheduleTaskConfigService
}

type Gateway struct {
	core          legacy.GatewayService
	agentConfigs  AgentConfigService
	channels      ChannelConfigService
	mcpServers    MCPServerConfigService
	scheduleTasks ScheduleTaskConfigService
}

func NewGateway(core legacy.GatewayService) *Gateway {
	return &Gateway{core: core}
}

func NewGatewayWithManagementCRUD(core legacy.GatewayService, configStore *store.ManagementConfigStore) *Gateway {
	return &Gateway{
		core:          core,
		agentConfigs:  NewAgentConfigCRUD(configStore),
		channels:      NewChannelConfigCRUD(configStore),
		mcpServers:    NewMCPServerConfigCRUD(configStore),
		scheduleTasks: NewScheduleTaskConfigCRUD(configStore),
	}
}

func (g *Gateway) AgentConfigs() AgentConfigService {
	return g.agentConfigs
}

func (g *Gateway) ChannelConfigs() ChannelConfigService {
	return g.channels
}

func (g *Gateway) MCPServerConfigs() MCPServerConfigService {
	return g.mcpServers
}

func (g *Gateway) ScheduleTaskConfigs() ScheduleTaskConfigService {
	return g.scheduleTasks
}

func (g *Gateway) ListAgents(ctx context.Context) ([]models.AgentProfile, error) {
	out, err := g.core.ListAgents(ctx)
	return out, mapError(err)
}

func (g *Gateway) ListSkills(ctx context.Context) ([]models.Skill, error) {
	out, err := g.core.ListSkills(ctx)
	return out, mapError(err)
}

func (g *Gateway) GetManagementSettings(ctx context.Context) (models.ManagementSettings, error) {
	out, err := g.core.GetManagementSettings(ctx)
	return out, mapError(err)
}

func (g *Gateway) ReplaceManagementSettings(ctx context.Context, req models.ManagementSettings) (models.ManagementSettings, error) {
	out, err := g.core.UpdateManagementSettings(ctx, req)
	return out, mapError(err)
}

func (g *Gateway) ListChannelStatuses(ctx context.Context) ([]models.ChannelRuntimeStatus, error) {
	out, err := g.core.GetChannelStatuses(ctx)
	return out, mapError(err)
}

func (g *Gateway) StartChannels(ctx context.Context) error {
	return mapError(g.core.StartChannels(ctx))
}

func (g *Gateway) StopChannels(ctx context.Context) error {
	return mapError(g.core.StopChannels(ctx))
}

func (g *Gateway) CreateSession(ctx context.Context, req models.CreateSessionRequest) (models.Session, error) {
	out, err := g.core.CreateSession(ctx, req)
	return out, mapError(err)
}

func (g *Gateway) ListSessions(ctx context.Context) ([]models.Session, error) {
	out, err := g.core.ListSessions(ctx)
	return out, mapError(err)
}

func (g *Gateway) GetSession(ctx context.Context, sessionID string) (models.Session, error) {
	out, err := g.core.GetSession(ctx, sessionID)
	return out, mapError(err)
}

func (g *Gateway) DeleteSession(ctx context.Context, sessionID string) (models.Session, error) {
	out, err := g.core.CloseSession(ctx, sessionID)
	return out, mapError(err)
}

func (g *Gateway) ResumeSession(ctx context.Context, sessionID string, req models.ResumeSessionRequest) (models.Session, error) {
	out, err := g.core.ResumeSession(ctx, sessionID, req)
	return out, mapError(err)
}

func (g *Gateway) UpdateSessionMode(ctx context.Context, sessionID string, req models.SetSessionModeRequest) (models.Session, error) {
	out, err := g.core.SetSessionMode(ctx, sessionID, req)
	return out, mapError(err)
}

func (g *Gateway) UpdateSessionConfig(ctx context.Context, sessionID string, req models.SetSessionConfigOptionRequest) (models.Session, error) {
	out, err := g.core.SetSessionConfigOption(ctx, sessionID, req)
	return out, mapError(err)
}

func (g *Gateway) ListSessionMessages(ctx context.Context, sessionID string) ([]models.Message, error) {
	out, err := g.core.ListMessages(ctx, sessionID)
	return out, mapError(err)
}

func (g *Gateway) CreateSessionMessage(ctx context.Context, sessionID string, req models.PromptRequest) (models.PromptResponse, error) {
	out, err := g.core.Prompt(ctx, sessionID, req)
	return out, mapError(err)
}

func (g *Gateway) CancelSessionRun(ctx context.Context, sessionID string) (models.Run, error) {
	out, err := g.core.Cancel(ctx, sessionID)
	return out, mapError(err)
}

func (g *Gateway) ListRuns(ctx context.Context) ([]models.Run, error) {
	out, err := g.core.ListRuns(ctx)
	return out, mapError(err)
}

func (g *Gateway) ListApprovals(ctx context.Context) ([]models.Approval, error) {
	out, err := g.core.ListApprovals(ctx)
	return out, mapError(err)
}

func (g *Gateway) UpdateApprovalDecision(ctx context.Context, approvalID string, req models.ApprovalDecisionRequest) (models.Approval, error) {
	out, err := g.core.DecideApproval(ctx, approvalID, req)
	return out, mapError(err)
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	serviceErr, ok := err.(*legacy.Error)
	if !ok {
		return err
	}
	return &GatewayError{
		Code:    serviceErr.Code,
		Message: serviceErr.Message,
	}
}
