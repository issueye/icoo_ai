package acp

import (
	"context"
	"encoding/json"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/google/uuid"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
)

// Client implements the ACP client callbacks used by connected agents.
// The implementation is deliberately conservative for the first M6 cut:
// gateway management is exposed through ExtensionGateway, while file and
// terminal callbacks are denied until a workspace policy is added.
type Client struct {
	AgentID    string
	Extensions *ExtensionGateway
	Events     *events.Bus
	Approvals  *ApprovalBroker
}

func NewClient(agentID string, extensions *ExtensionGateway, bus *events.Bus, approvals *ApprovalBroker) *Client {
	return &Client{AgentID: agentID, Extensions: extensions, Events: bus, Approvals: approvals}
}

func (c *Client) HandleExtensionMethod(ctx context.Context, method string, params json.RawMessage) (any, error) {
	if c.Extensions == nil {
		return nil, acpsdk.NewMethodNotFound(method)
	}
	if c.AgentID != "" {
		ctx = ContextWithAgentID(ctx, c.AgentID)
	}
	return c.Extensions.HandleExtensionMethod(ctx, method, params)
}

func (c *Client) ReadTextFile(context.Context, acpsdk.ReadTextFileRequest) (acpsdk.ReadTextFileResponse, error) {
	return acpsdk.ReadTextFileResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodFsReadTextFile)
}

func (c *Client) WriteTextFile(context.Context, acpsdk.WriteTextFileRequest) (acpsdk.WriteTextFileResponse, error) {
	return acpsdk.WriteTextFileResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodFsWriteTextFile)
}

func (c *Client) RequestPermission(ctx context.Context, params acpsdk.RequestPermissionRequest) (acpsdk.RequestPermissionResponse, error) {
	if c.Approvals == nil {
		return acpsdk.RequestPermissionResponse{}, acpsdk.NewInternalError(map[string]any{"error": "approval service is not connected"})
	}
	return c.Approvals.Request(ctx, c.AgentID, params)
}

func (c *Client) SessionUpdate(_ context.Context, params acpsdk.SessionNotification) error {
	if c.Events == nil {
		return nil
	}
	c.Events.Publish(models.EventEnvelope{
		BaseModel: models.BaseModel{ID: uuid.NewString()},
		Type:      "acp.session_update",
		AgentID:   c.AgentID,
		SessionID: string(params.SessionId),
		Payload: map[string]any{
			"kind":         sessionUpdateKind(params.Update),
			"notification": params,
		},
		CreatedAt: time.Now(),
	})
	return nil
}

func (c *Client) CreateTerminal(context.Context, acpsdk.CreateTerminalRequest) (acpsdk.CreateTerminalResponse, error) {
	return acpsdk.CreateTerminalResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodTerminalCreate)
}

func (c *Client) KillTerminal(context.Context, acpsdk.KillTerminalRequest) (acpsdk.KillTerminalResponse, error) {
	return acpsdk.KillTerminalResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodTerminalKill)
}

func (c *Client) TerminalOutput(context.Context, acpsdk.TerminalOutputRequest) (acpsdk.TerminalOutputResponse, error) {
	return acpsdk.TerminalOutputResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodTerminalOutput)
}

func (c *Client) ReleaseTerminal(context.Context, acpsdk.ReleaseTerminalRequest) (acpsdk.ReleaseTerminalResponse, error) {
	return acpsdk.ReleaseTerminalResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodTerminalRelease)
}

func (c *Client) WaitForTerminalExit(context.Context, acpsdk.WaitForTerminalExitRequest) (acpsdk.WaitForTerminalExitResponse, error) {
	return acpsdk.WaitForTerminalExitResponse{}, acpsdk.NewMethodNotFound(acpsdk.ClientMethodTerminalWaitForExit)
}

var _ acpsdk.Client = (*Client)(nil)

func sessionUpdateKind(update acpsdk.SessionUpdate) string {
	switch {
	case update.UserMessageChunk != nil:
		return "user_message_chunk"
	case update.AgentMessageChunk != nil:
		return "agent_message_chunk"
	case update.AgentThoughtChunk != nil:
		return "agent_thought_chunk"
	case update.ToolCall != nil:
		return "tool_call"
	case update.ToolCallUpdate != nil:
		return "tool_call_update"
	case update.Plan != nil:
		return "plan"
	case update.AvailableCommandsUpdate != nil:
		return "available_commands_update"
	case update.CurrentModeUpdate != nil:
		return "current_mode_update"
	case update.ConfigOptionUpdate != nil:
		return "config_option_update"
	case update.SessionInfoUpdate != nil:
		return "session_info_update"
	case update.UsageUpdate != nil:
		return "usage_update"
	default:
		return "unknown"
	}
}
