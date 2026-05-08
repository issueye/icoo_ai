package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

func TestReactLoopStreamsText(t *testing.T) {
	provider := newMockProvider([][]llm.CompletionEvent{{
		{Type: llm.CompletionEventMessageDelta, Delta: "hello"},
		{Type: llm.CompletionEventCompleted},
	}})
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}

	events, err := loop.Run(context.Background(), RunRequest{SessionID: "s1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	got, err := collectEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if eventContent(got, EventMessageDelta) != "hello" {
		t.Fatalf("message delta = %q", eventContent(got, EventMessageDelta))
	}
	if got[len(got)-1].Type != EventRunCompleted {
		t.Fatalf("last event = %s", got[len(got)-1].Type)
	}
}

func TestReactLoopExecutesToolAndContinues(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"path": "README.md"})
	provider := newMockProvider([][]llm.CompletionEvent{
		{
			{Type: llm.CompletionEventToolCall, ToolCall: &tools.ToolCall{ID: "tc1", Name: "read_file", Arguments: args}},
			{Type: llm.CompletionEventCompleted},
		},
		{
			{Type: llm.CompletionEventMessageDelta, Delta: "done"},
			{Type: llm.CompletionEventCompleted},
		},
	})
	tool := newMockTool("read_file", tools.ToolResult{OK: true, Content: "file content"})
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider, Tools: []tools.Tool{tool}})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}

	events, err := loop.Run(context.Background(), RunRequest{SessionID: "s1"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got, err := collectEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if len(tool.Calls()) != 1 {
		t.Fatalf("tool calls = %d", len(tool.Calls()))
	}
	if eventContent(got, EventMessageDelta) != "done" {
		t.Fatalf("message delta = %q", eventContent(got, EventMessageDelta))
	}
}
