package models

import (
	"time"

	channelmodels "github.com/icoo-ai/icoo-ai/agent_gateway/internal/channels/models"
)

type AgentProfile struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Protocol    string   `json:"protocol"`
	Command     string   `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	Endpoint    string   `json:"endpoint,omitempty"`
	Models      []string `json:"models,omitempty"`
	Description string   `json:"description,omitempty"`
}

type Skill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type MCPServerConfig struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Enabled bool     `json:"enabled"`
}

type ScheduleTaskConfig struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Spec    string `json:"spec,omitempty"`
	Content string `json:"content,omitempty"`
	Enabled bool   `json:"enabled"`
}

type AgentConfig struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Protocol    string   `json:"protocol,omitempty"`
	Description string   `json:"description,omitempty"`
	Models      []string `json:"models,omitempty"`
	Enabled     bool     `json:"enabled"`
}

type ChannelConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enabled    bool   `json:"enabled"`
	AppID      string `json:"appId,omitempty"`
	AppSecret  string `json:"appSecret,omitempty"`
	BotToken   string `json:"botToken,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty"`
}

type ManagementSettings struct {
	Channels      []ChannelConfig      `json:"channels,omitempty"`
	MCPServers    []MCPServerConfig    `json:"mcpServers,omitempty"`
	ScheduleTasks []ScheduleTaskConfig `json:"scheduleTasks,omitempty"`
	Agents        []AgentConfig        `json:"agents,omitempty"`
}

type ChannelRuntimeStatus = channelmodels.ChannelStatus

type Session struct {
	ID                    string    `json:"id"`
	Title                 string    `json:"title"`
	WorkspaceID           string    `json:"workspaceId,omitempty"`
	CWD                   string    `json:"cwd,omitempty"`
	AdditionalDirectories []string  `json:"additionalDirectories,omitempty"`
	StartupCommand        string    `json:"startupCommand,omitempty"`
	Mode                  string    `json:"mode,omitempty"`
	AgentID               string    `json:"agentId"`
	Model                 string    `json:"model,omitempty"`
	Status                string    `json:"status"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
}

type CreateSessionRequest struct {
	Title                 string   `json:"title"`
	WorkspaceID           string   `json:"workspaceId,omitempty"`
	CWD                   string   `json:"cwd,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	StartupCommand        string   `json:"startupCommand,omitempty"`
	Mode                  string   `json:"mode,omitempty"`
	AgentID               string   `json:"agentId,omitempty"`
	Model                 string   `json:"model,omitempty"`
}

type ResumeSessionRequest struct {
	CWD                   string   `json:"cwd,omitempty"`
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
}

type SetSessionModeRequest struct {
	Mode string `json:"mode"`
}

type SetSessionConfigOptionRequest struct {
	ConfigID     string `json:"configId"`
	BooleanValue *bool  `json:"booleanValue,omitempty"`
	ValueID      string `json:"valueId,omitempty"`
}

type Message struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	RunID     string    `json:"runId,omitempty"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

type PromptRequest struct {
	Content     string `json:"content"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	CWD         string `json:"cwd,omitempty"`
	Mode        string `json:"mode,omitempty"`
	AgentID     string `json:"agentId,omitempty"`
	Model       string `json:"model,omitempty"`
}

type PromptResponse struct {
	Run      Run       `json:"run"`
	Messages []Message `json:"messages"`
	Approval *Approval `json:"approval,omitempty"`
}

type Run struct {
	ID        string     `json:"id"`
	SessionID string     `json:"sessionId"`
	AgentID   string     `json:"agentId"`
	Status    string     `json:"status"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

type Approval struct {
	ID                 string     `json:"id"`
	AgentID            string     `json:"agentId"`
	SessionID          string     `json:"sessionId"`
	RunID              string     `json:"runId"`
	ConnectorRequestID string     `json:"connectorRequestId"`
	Status             string     `json:"status"`
	Action             string     `json:"action"`
	Message            string     `json:"message"`
	Decision           string     `json:"decision,omitempty"`
	DecidedAt          *time.Time `json:"decidedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
}

type ApprovalDecisionRequest struct {
	Decision string `json:"decision"`
	Message  string `json:"message,omitempty"`
}
