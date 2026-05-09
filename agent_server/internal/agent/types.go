package agent

import (
	"context"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/hooks"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/policy"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type Runtime interface {
	NewSession(ctx context.Context, req NewSessionRequest) (Session, error)
	Prompt(ctx context.Context, req PromptRequest) (<-chan Event, error)
	Cancel(ctx context.Context, sessionID string) error
	LoadSession(ctx context.Context, sessionID string) (Session, error)
}

type Loop interface {
	Name() string
	Run(ctx context.Context, req RunRequest) (<-chan Event, error)
}

type NewSessionRequest struct {
	CWD      string         `json:"cwd"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type PromptRequest struct {
	SessionID string         `json:"session_id"`
	Prompt    string         `json:"prompt"`
	CWD       string         `json:"cwd,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type RunRequest struct {
	SessionID string           `json:"session_id"`
	CWD       string           `json:"cwd"`
	Messages  []llm.Message    `json:"messages"`
	Context   WorkspaceContext `json:"context"`
	Tools     []tools.Tool     `json:"-"`
	Skills    []Skill          `json:"skills,omitempty"`
	Options   RunOptions       `json:"options,omitempty"`
}

type RunOptions struct {
	Model          string                `json:"model,omitempty"`
	PermissionMode policy.PermissionMode `json:"permission_mode,omitempty"`
	MaxToolRounds  int                   `json:"max_tool_rounds,omitempty"`
	Approver       Approver              `json:"-"`
	Hooks          hooks.Dispatcher      `json:"-"`
	AuditLogger    audit.Logger          `json:"-"`
	Metadata       map[string]any        `json:"metadata,omitempty"`
}

type ApprovalDecision string

const (
	ApprovalDecisionOnce   ApprovalDecision = "once"
	ApprovalDecisionAlways ApprovalDecision = "always"
	ApprovalDecisionDeny   ApprovalDecision = "deny"
)

type ApprovalRequest struct {
	SessionID string         `json:"session_id"`
	ToolName  string         `json:"tool_name"`
	ToolCall  string         `json:"tool_call"`
	Reason    string         `json:"reason"`
	Data      map[string]any `json:"data,omitempty"`
}

type Approver interface {
	Approve(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error)
}

type WorkspaceContext struct {
	Root     string            `json:"root"`
	GitRoot  string            `json:"git_root,omitempty"`
	Branch   string            `json:"branch,omitempty"`
	Files    []string          `json:"files,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Skill struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Path        string         `json:"path"`
	Body        string         `json:"body,omitempty"`
	Resources   SkillResources `json:"resources,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type SkillResources struct {
	Scripts    []string `json:"scripts,omitempty"`
	References []string `json:"references,omitempty"`
	Assets     []string `json:"assets,omitempty"`
}

type EventType string

const (
	EventRunStarted        EventType = "run_started"
	EventMessageDelta      EventType = "message_delta"
	EventToolCallStarted   EventType = "tool_call_started"
	EventToolCallCompleted EventType = "tool_call_completed"
	EventApprovalRequested EventType = "approval_requested"
	EventApprovalDecided   EventType = "approval_decided"
	EventPlanUpdated       EventType = "plan_updated"
	EventRunCompleted      EventType = "run_completed"
	EventRunFailed         EventType = "run_failed"
	EventRunCancelled      EventType = "run_cancelled"
)

type Event struct {
	Type      EventType      `json:"type"`
	SessionID string         `json:"session_id"`
	Content   string         `json:"content,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

type Session struct {
	ID        string                `json:"id"`
	CWD       string                `json:"cwd"`
	Model     string                `json:"model,omitempty"`
	Messages  []llm.Message         `json:"messages,omitempty"`
	Events    []SessionEventSummary `json:"events,omitempty"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

type SessionEventSummary struct {
	Type      EventType      `json:"type"`
	Content   string         `json:"content,omitempty"`
	Error     string         `json:"error,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}
