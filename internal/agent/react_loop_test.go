package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/hooks"
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

func TestReactLoopApprovalOnceRetriesTool(t *testing.T) {
	provider := newMockProvider([][]llm.CompletionEvent{
		{{Type: llm.CompletionEventToolCall, ToolCall: &tools.ToolCall{ID: "tc1", Name: "needs_approval", Arguments: json.RawMessage(`{"x":1}`)}}},
		{{Type: llm.CompletionEventMessageDelta, Delta: "done"}, {Type: llm.CompletionEventCompleted}},
	})
	tool := &approvalMockTool{name: "needs_approval", result: tools.ToolResult{OK: true, Content: "approved"}}
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider, Tools: []tools.Tool{tool}})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}
	events, err := loop.Run(context.Background(), RunRequest{
		SessionID: "s1",
		Options:   RunOptions{Approver: staticApprover{decision: ApprovalDecisionOnce}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got, err := collectEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if tool.approvedCalls != 1 {
		t.Fatalf("approved calls = %d", tool.approvedCalls)
	}
	if eventContent(got, EventMessageDelta) != "done" {
		t.Fatalf("message delta = %q", eventContent(got, EventMessageDelta))
	}
}

func TestReactLoopApprovalDeniedReturnsToolResult(t *testing.T) {
	provider := newMockProvider([][]llm.CompletionEvent{
		{{Type: llm.CompletionEventToolCall, ToolCall: &tools.ToolCall{ID: "tc1", Name: "needs_approval", Arguments: json.RawMessage(`{"x":1}`)}}},
		{{Type: llm.CompletionEventMessageDelta, Delta: "denied"}, {Type: llm.CompletionEventCompleted}},
	})
	tool := &approvalMockTool{name: "needs_approval"}
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider, Tools: []tools.Tool{tool}})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}
	events, err := loop.Run(context.Background(), RunRequest{
		SessionID: "s1",
		Options:   RunOptions{Approver: staticApprover{decision: ApprovalDecisionDeny}},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	_, err = collectEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if tool.approvedCalls != 0 {
		t.Fatalf("approved calls = %d", tool.approvedCalls)
	}
}

func TestReactLoopBlocksToolCallWhenHookBlocks(t *testing.T) {
	provider := newMockProvider([][]llm.CompletionEvent{
		{{Type: llm.CompletionEventToolCall, ToolCall: &tools.ToolCall{ID: "tc1", Name: "read_file", Arguments: json.RawMessage(`{"path":"README.md"}`)}}},
		{{Type: llm.CompletionEventMessageDelta, Delta: "blocked"}, {Type: llm.CompletionEventCompleted}},
	})
	tool := newMockTool("read_file", tools.ToolResult{OK: true, Content: "file content"})
	loop, err := NewReactLoop(ReactLoopOptions{Provider: provider, Tools: []tools.Tool{tool}})
	if err != nil {
		t.Fatalf("NewReactLoop() error = %v", err)
	}
	dispatcher := hooks.NewDispatcher(hooks.TypedHook{
		HookName: "block-read",
		Events:   []hooks.EventType{hooks.EventBeforeToolCall},
		Func: func(ctx context.Context, event hooks.Event) (hooks.Result, error) {
			return hooks.Block("blocked by hook"), nil
		},
	})
	events, err := loop.Run(context.Background(), RunRequest{
		SessionID: "s1",
		Options:   RunOptions{Hooks: dispatcher},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	got, err := collectEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if len(tool.Calls()) != 0 {
		t.Fatalf("tool should not have run, calls = %d", len(tool.Calls()))
	}
	var completed Event
	for _, event := range got {
		if event.Type == EventToolCallCompleted {
			completed = event
		}
	}
	if completed.Error != "blocked by hook" {
		t.Fatalf("completed error = %q", completed.Error)
	}
}
