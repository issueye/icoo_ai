package subagent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type recordingRunner struct {
	requests []Request
}

func executeSubagentTool(t *testing.T, tool tools.Tool, input map[string]any) tools.ToolResult {
	t.Helper()
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	result, err := tool.Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return result
}

func (r *recordingRunner) Run(ctx context.Context, req Request) (Result, error) {
	r.requests = append(r.requests, req)
	return Result{Content: req.Task}, nil
}

func TestToolGeneratesUniqueSessionIDPerRun(t *testing.T) {
	runner := &recordingRunner{}
	tool := NewTool(ToolOptions{Runner: runner})

	first := executeSubagentTool(t, tool, map[string]any{"task": "first"})
	second := executeSubagentTool(t, tool, map[string]any{"task": "second"})
	if !first.OK || !second.OK {
		t.Fatalf("results = %+v %+v", first, second)
	}
	if len(runner.requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(runner.requests))
	}
	firstID := runner.requests[0].SessionID
	secondID := runner.requests[1].SessionID
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("session ids = %q and %q, want unique non-empty ids", firstID, secondID)
	}
	if !strings.HasPrefix(firstID, "subsess_subagent_tool_") || !strings.HasPrefix(secondID, "subsess_subagent_tool_") {
		t.Fatalf("session ids = %q and %q, want subagent tool prefix", firstID, secondID)
	}
	if first.Data["session_id"] != firstID || second.Data["session_id"] != secondID {
		t.Fatalf("result session ids = %+v %+v", first.Data, second.Data)
	}
}
