package testutil

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type ToolCallRecord struct {
	Input json.RawMessage
}

type mockToolResponse struct {
	result tools.ToolResult
	err    error
}

type MockTool struct {
	name        string
	description string
	definition  tools.ToolDefinition

	mu        sync.Mutex
	calls     []ToolCallRecord
	responses []mockToolResponse

	DefaultResult tools.ToolResult
	DefaultErr    error
}

func NewMockTool(name string, results ...tools.ToolResult) *MockTool {
	if name == "" {
		name = "mock_tool"
	}

	t := &MockTool{
		name: name,
		DefaultResult: tools.ToolResult{
			OK:      true,
			Content: "",
		},
	}
	for _, result := range results {
		t.responses = append(t.responses, mockToolResponse{result: result})
	}
	return t
}

func (t *MockTool) Name() string {
	return t.name
}

func (t *MockTool) Description() string {
	return t.description
}

func (t *MockTool) Definition() tools.ToolDefinition {
	if t.definition.Name != "" {
		return t.definition
	}

	return tools.ToolDefinition{
		Name:        t.name,
		Description: t.description,
	}
}

func (t *MockTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.calls = append(t.calls, ToolCallRecord{Input: append(json.RawMessage(nil), input...)})
	index := len(t.calls) - 1

	if index < len(t.responses) {
		response := t.responses[index]
		return response.result, response.err
	}
	return t.DefaultResult, t.DefaultErr
}

func (t *MockTool) WithDescription(description string) *MockTool {
	t.description = description
	return t
}

func (t *MockTool) WithDefinition(def tools.ToolDefinition) *MockTool {
	t.definition = def
	return t
}

func (t *MockTool) EnqueueResult(result tools.ToolResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.responses = append(t.responses, mockToolResponse{result: result})
}

func (t *MockTool) EnqueueError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.responses = append(t.responses, mockToolResponse{err: err})
}

func (t *MockTool) Calls() []ToolCallRecord {
	t.mu.Lock()
	defer t.mu.Unlock()

	calls := make([]ToolCallRecord, len(t.calls))
	for i, call := range t.calls {
		calls[i] = ToolCallRecord{Input: append(json.RawMessage(nil), call.Input...)}
	}
	return calls
}
