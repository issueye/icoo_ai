package llm

import (
	"context"

	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type Provider interface {
	Name() string
	Stream(ctx context.Context, req CompletionRequest) (<-chan CompletionEvent, error)
}

type Message struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []tools.ToolCall `json:"tool_calls,omitempty"`
	Metadata  map[string]any   `json:"metadata,omitempty"`
}

type CompletionRequest struct {
	Model    string                 `json:"model"`
	Messages []Message              `json:"messages"`
	Tools    []tools.ToolDefinition `json:"tools,omitempty"`
	Options  CompletionOptions      `json:"options,omitempty"`
}

type CompletionOptions struct {
	Temperature      *float64       `json:"temperature,omitempty"`
	MaxOutputTokens  int            `json:"max_output_tokens,omitempty"`
	ReasoningEffort  string         `json:"reasoning_effort,omitempty"`
	StructuredOutput map[string]any `json:"structured_output,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

type CompletionEventType string

const (
	CompletionEventMessageDelta CompletionEventType = "message_delta"
	CompletionEventToolCall     CompletionEventType = "tool_call"
	CompletionEventCompleted    CompletionEventType = "completed"
	CompletionEventFailed       CompletionEventType = "failed"
)

type CompletionEvent struct {
	Type     CompletionEventType `json:"type"`
	Delta    string              `json:"delta,omitempty"`
	ToolCall *tools.ToolCall     `json:"tool_call,omitempty"`
	Error    string              `json:"error,omitempty"`
	Metadata map[string]any      `json:"metadata,omitempty"`
}
