package store

import "time"

// SafeMeta carries small, non-sensitive metadata that is safe to persist.
type SafeMeta map[string]any

type Conversation struct {
	ID        string    `json:"id,omitempty"`
	AgentID   string    `json:"agentId"`
	SessionID string    `json:"sessionId"`
	RunID     string    `json:"runId,omitempty"`
	Title     string    `json:"title,omitempty"`
	Status    string    `json:"status,omitempty"`
	Model     string    `json:"model,omitempty"`
	CWD       string    `json:"cwd,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	SafeMeta  SafeMeta  `json:"safeMeta,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

type MessageEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	AgentID   string    `json:"agentId"`
	SessionID string    `json:"sessionId"`
	RunID     string    `json:"runId"`
	Role      string    `json:"role,omitempty"`
	Status    string    `json:"status,omitempty"`
	Summary   string    `json:"summary,omitempty"`
	SafeMeta  SafeMeta  `json:"safeMeta,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type RunSummary struct {
	ID          string     `json:"id,omitempty"`
	AgentID     string     `json:"agentId"`
	SessionID   string     `json:"sessionId"`
	RunID       string     `json:"runId"`
	Status      string     `json:"status"`
	Summary     string     `json:"summary,omitempty"`
	SafeMeta    SafeMeta   `json:"safeMeta,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type ApprovalDecision struct {
	ID                 string     `json:"id"`
	AgentID            string     `json:"agentId"`
	SessionID          string     `json:"sessionId"`
	RunID              string     `json:"runId"`
	ConnectorRequestID string     `json:"connectorRequestId"`
	Status             string     `json:"status"`
	Decision           string     `json:"decision,omitempty"`
	Actor              string     `json:"actor,omitempty"`
	Summary            string     `json:"summary,omitempty"`
	SafeMeta           SafeMeta   `json:"safeMeta,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt,omitempty"`
	DecidedAt          *time.Time `json:"decidedAt,omitempty"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Level     string    `json:"level,omitempty"`
	AgentID   string    `json:"agentId"`
	SessionID string    `json:"sessionId,omitempty"`
	RunID     string    `json:"runId,omitempty"`
	Summary   string    `json:"summary"`
	SafeMeta  SafeMeta  `json:"safeMeta,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}
