package testutil

import (
	"context"
	"errors"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/tools"
)

func TestMockToolRecordsCallsAndReturnsQueuedResults(t *testing.T) {
	tool := NewMockTool("lookup",
		tools.ToolResult{OK: true, Content: "first"},
		tools.ToolResult{OK: true, Content: "second"},
	).WithDescription("does lookup")

	first, err := tool.Execute(context.Background(), []byte(`{"q":"a"}`))
	if err != nil {
		t.Fatalf("first Execute returned error: %v", err)
	}
	second, err := tool.Execute(context.Background(), []byte(`{"q":"b"}`))
	if err != nil {
		t.Fatalf("second Execute returned error: %v", err)
	}

	if first.Content != "first" || second.Content != "second" {
		t.Fatalf("unexpected results: %q, %q", first.Content, second.Content)
	}

	calls := tool.Calls()
	if len(calls) != 2 {
		t.Fatalf("got %d calls, want 2", len(calls))
	}
	if string(calls[0].Input) != `{"q":"a"}` {
		t.Fatalf("first input = %s", calls[0].Input)
	}
	if tool.Definition().Name != "lookup" {
		t.Fatalf("definition name = %q, want lookup", tool.Definition().Name)
	}
	if tool.Description() != "does lookup" {
		t.Fatalf("description = %q, want does lookup", tool.Description())
	}
}

func TestMockToolQueuedError(t *testing.T) {
	want := errors.New("tool failed")
	tool := NewMockTool("lookup", tools.ToolResult{OK: true, Content: "first"})
	tool.EnqueueError(want)

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("first Execute returned error: %v", err)
	}
	if result.Content != "first" {
		t.Fatalf("content = %q, want first", result.Content)
	}

	_, err = tool.Execute(context.Background(), nil)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}
