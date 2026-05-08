package hooks

import (
	"context"
	"encoding/json"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/policy"
)

type EventType string

const (
	EventBeforeRun          EventType = "before_run"
	EventAfterRun           EventType = "after_run"
	EventBeforeToolCall     EventType = "before_tool_call"
	EventAfterToolCall      EventType = "after_tool_call"
	EventBeforeFileWrite    EventType = "before_file_write"
	EventAfterFileWrite     EventType = "after_file_write"
	EventBeforeShellCommand EventType = "before_shell_command"
	EventAfterShellCommand  EventType = "after_shell_command"
	EventOnError            EventType = "on_error"
)

type Action string

const (
	ActionContinue        Action = "continue"
	ActionModify          Action = "modify"
	ActionBlock           Action = "block"
	ActionRequestApproval Action = "request_approval"
)

type Event struct {
	Type      EventType      `json:"type"`
	SessionID string         `json:"session_id,omitempty"`
	Name      string         `json:"name,omitempty"`
	CWD       string         `json:"cwd,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
	Error     string         `json:"error,omitempty"`
	CreatedAt time.Time      `json:"created_at,omitempty"`
}

type Result struct {
	Action  Action         `json:"action"`
	Reason  string         `json:"reason,omitempty"`
	Patches map[string]any `json:"patches,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type DispatchResult struct {
	Action  Action         `json:"action"`
	Event   Event          `json:"event"`
	Reason  string         `json:"reason,omitempty"`
	Results []HookRun      `json:"results,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

type HookRun struct {
	Name   string `json:"name"`
	Result Result `json:"result"`
}

type Hook interface {
	Name() string
	Match(event Event) bool
	Execute(ctx context.Context, event Event) (Result, error)
}

type Dispatcher interface {
	Register(hook Hook)
	Dispatch(ctx context.Context, event Event) (DispatchResult, error)
}

type PolicyRequest struct {
	Command *policy.CommandRequest `json:"command,omitempty"`
	Path    *policy.PathRequest    `json:"path,omitempty"`
	Network *policy.NetworkRequest `json:"network,omitempty"`
	MCP     *policy.MCPRequest     `json:"mcp,omitempty"`
}

func Continue() Result {
	return Result{Action: ActionContinue}
}

func Modify(patches map[string]any) Result {
	return Result{Action: ActionModify, Patches: patches}
}

func Block(reason string) Result {
	return Result{Action: ActionBlock, Reason: reason}
}

func RequestApproval(reason string) Result {
	return Result{Action: ActionRequestApproval, Reason: reason}
}

func CloneEvent(event Event) Event {
	cloned := event
	cloned.Data = cloneMap(event.Data)
	return cloned
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneValue(item)
		}
		return out
	case json.RawMessage:
		return append(json.RawMessage(nil), typed...)
	case []byte:
		return append([]byte(nil), typed...)
	default:
		return typed
	}
}
