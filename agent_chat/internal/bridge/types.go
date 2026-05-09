package bridge

import (
	"encoding/json"
	"time"
)

type ErrorCode string

const (
	ErrorCodeGatewayUnavailable ErrorCode = "gateway_unavailable"
	ErrorCodeGatewayAuthFailed  ErrorCode = "gateway_auth_failed"
	ErrorCodeGatewayRequest     ErrorCode = "gateway_request_failed"
	ErrorCodeGatewayStream      ErrorCode = "gateway_stream_failed"
	ErrorCodeGatewayBootstrap   ErrorCode = "gateway_bootstrap_failed"
	ErrorCodeInvalidArgument    ErrorCode = "invalid_argument"
)

const (
	GatewayStatusConnecting   = "gateway_connecting"
	GatewayStatusReady        = "gateway_ready"
	GatewayStatusReconnecting = "gateway_reconnecting"
	GatewayStatusFailed       = "gateway_failed"
)

type BridgeError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"statusCode,omitempty"`
	Retryable  bool      `json:"retryable"`
}

func (e *BridgeError) Error() string {
	if e == nil {
		return ""
	}
	return string(e.Code) + ": " + e.Message
}

type Conversation struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Title           string    `json:"title"`
	Subtitle        string    `json:"subtitle"`
	Status          string    `json:"status"`
	UnreadCount     int       `json:"unreadCount"`
	UpdatedAt       time.Time `json:"updatedAt"`
	ParentSessionID string    `json:"parentSessionId,omitempty"`
	Skill           string    `json:"skill,omitempty"`
	WorkspaceID     string    `json:"workspaceId,omitempty"`
	CWD             string    `json:"cwd,omitempty"`
	Mode            string    `json:"mode,omitempty"`
	Model           string    `json:"model,omitempty"`
}

type NewSessionRequest struct {
	Title       string `json:"title"`
	Cwd         string `json:"cwd,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Model       string `json:"model,omitempty"`
}

type PromptRequest struct {
	SessionID   string `json:"sessionId"`
	Prompt      string `json:"prompt"`
	Cwd         string `json:"cwd,omitempty"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Model       string `json:"model,omitempty"`
}

type ApprovalDecisionRequest struct {
	ID        string `json:"id"`
	SessionID string `json:"sessionId"`
	Decision  string `json:"decision"`
}

type MessageEvent struct {
	ID              string         `json:"id"`
	SessionID       string         `json:"sessionId"`
	Kind            string         `json:"kind"`
	Role            string         `json:"role,omitempty"`
	Content         string         `json:"content,omitempty"`
	ToolName        string         `json:"toolName,omitempty"`
	Status          string         `json:"status,omitempty"`
	DurationMs      int            `json:"durationMs,omitempty"`
	Summary         string         `json:"summary,omitempty"`
	SafeMeta        map[string]any `json:"safeMeta,omitempty"`
	Decision        string         `json:"decision,omitempty"`
	SubSessionID    string         `json:"subSessionId,omitempty"`
	ParentSessionID string         `json:"parentSessionId,omitempty"`
	Task            string         `json:"task,omitempty"`
	EventCount      int            `json:"eventCount,omitempty"`
	CreatedAt       time.Time      `json:"createdAt"`
}

const (
	BridgeEventKindMessage    = "message"
	BridgeEventKindToolCall   = "tool_call"
	BridgeEventKindToolResult = "tool_result"
	BridgeEventKindApproval   = "approval"
	BridgeEventKindSubagent   = "subagent_run"
	BridgeEventKindRun        = "run"
	BridgeEventKindAudit      = "audit"
	BridgeEventKindGateway    = "gateway_event"
)

type RunSummary struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"sessionId"`
	ParentSessionID string     `json:"parentSessionId,omitempty"`
	Status          string     `json:"status"`
	Label           string     `json:"label"`
	StartedAt       time.Time  `json:"startedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
}

type ApprovalDecision struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Decision  string    `json:"decision"`
	Actor     string    `json:"actor"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
}

type SkillInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Type      string    `json:"type"`
	Level     string    `json:"level"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"createdAt"`
}

type GatewayStatus struct {
	Status    string    `json:"status"`
	Summary   string    `json:"summary,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type GatewayEventEnvelope struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	AgentID   string          `json:"agentId,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	RunID     string          `json:"runId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"createdAt"`
}
