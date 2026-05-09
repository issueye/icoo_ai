package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type mockProvider struct {
	mu      sync.Mutex
	streams [][]llm.CompletionEvent
}

type staticApprover struct {
	decision ApprovalDecision
}

func (a staticApprover) Approve(ctx context.Context, req ApprovalRequest) (ApprovalDecision, error) {
	return a.decision, ctx.Err()
}

type approvalMockTool struct {
	name          string
	result        tools.ToolResult
	approvedCalls int
}

func (t *approvalMockTool) Name() string        { return t.name }
func (t *approvalMockTool) Description() string { return t.name }
func (t *approvalMockTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{Name: t.name, Description: t.name}
}
func (t *approvalMockTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	return tools.ToolResult{OK: false, Error: "needs approval", Data: map[string]any{"code": "approval_required"}}, nil
}
func (t *approvalMockTool) ApprovalKey(input json.RawMessage) (string, bool) {
	return string(input), true
}
func (t *approvalMockTool) ExecuteApproved(ctx context.Context, input json.RawMessage, scope tools.ApprovalScope) (tools.ToolResult, error) {
	t.approvedCalls++
	if t.result.Content == "" {
		t.result = tools.ToolResult{OK: true, Content: "approved"}
	}
	return t.result, nil
}

func newMockProvider(streams [][]llm.CompletionEvent) *mockProvider {
	return &mockProvider{streams: streams}
}

func (p *mockProvider) Name() string {
	return "mock"
}

func (p *mockProvider) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.CompletionEvent, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.streams) == 0 {
		return nil, errors.New("no mock stream configured")
	}
	events := p.streams[0]
	p.streams = p.streams[1:]

	ch := make(chan llm.CompletionEvent)
	go func() {
		defer close(ch)
		for _, event := range events {
			select {
			case <-ctx.Done():
				return
			case ch <- event:
			}
		}
	}()
	return ch, nil
}

type mockTool struct {
	name   string
	result tools.ToolResult
	calls  []json.RawMessage
}

func newMockTool(name string, result tools.ToolResult) *mockTool {
	return &mockTool{name: name, result: result}
}

func (t *mockTool) Name() string {
	return t.name
}

func (t *mockTool) Description() string {
	return t.name
}

func (t *mockTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{Name: t.name, Description: t.name}
}

func (t *mockTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	t.calls = append(t.calls, append(json.RawMessage(nil), input...))
	return t.result, nil
}

func (t *mockTool) Calls() []json.RawMessage {
	return t.calls
}

func collectEvents(ctx context.Context, events <-chan Event) ([]Event, error) {
	var out []Event
	for {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		case event, ok := <-events:
			if !ok {
				return out, nil
			}
			out = append(out, event)
		}
	}
}

func eventContent(events []Event, typ EventType) string {
	var content string
	for _, event := range events {
		if event.Type == typ {
			content += event.Content
		}
	}
	return content
}
