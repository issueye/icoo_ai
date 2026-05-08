package subagent

import (
	"context"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/testutil"
)

func TestLocalRunnerCollectsAssistantOutput(t *testing.T) {
	provider := testutil.NewMockLLMProvider("mock", []llm.CompletionEvent{
		{Type: llm.CompletionEventMessageDelta, Delta: "hello"},
		{Type: llm.CompletionEventMessageDelta, Delta: " world"},
		{Type: llm.CompletionEventCompleted},
	})
	runner, err := NewLocalRunner(LocalRunnerOptions{Provider: provider, Model: "gpt-test"})
	if err != nil {
		t.Fatalf("NewLocalRunner() error = %v", err)
	}

	result, err := runner.Run(context.Background(), Request{
		Task:  "say hello",
		Skill: &agent.Skill{Name: "test-skill", Description: "Test skill", Body: "Use short answers."},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Content != "hello world" {
		t.Fatalf("Content = %q", result.Content)
	}
	call, ok := provider.LastCall()
	if !ok {
		t.Fatal("provider was not called")
	}
	if call.Model != "gpt-test" || len(call.Messages) != 2 {
		t.Fatalf("call = %+v", call)
	}
	if call.Messages[0].Role != "system" || call.Messages[1].Role != "user" {
		t.Fatalf("messages = %+v", call.Messages)
	}
}
