package subagent

import (
	"context"
	"strings"
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

func TestLocalRunnerGeneratesUniqueDefaultSessionIDs(t *testing.T) {
	provider := testutil.NewMockLLMProvider("mock",
		[]llm.CompletionEvent{{Type: llm.CompletionEventCompleted}},
		[]llm.CompletionEvent{{Type: llm.CompletionEventCompleted}},
	)
	runner, err := NewLocalRunner(LocalRunnerOptions{Provider: provider, Model: "gpt-test"})
	if err != nil {
		t.Fatalf("NewLocalRunner() error = %v", err)
	}

	first, err := runner.Run(context.Background(), Request{Task: "first"})
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	second, err := runner.Run(context.Background(), Request{Task: "second"})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if len(first.Events) == 0 || len(second.Events) == 0 {
		t.Fatalf("missing events: first=%+v second=%+v", first.Events, second.Events)
	}
	firstID := first.Events[0].SessionID
	secondID := second.Events[0].SessionID
	if firstID == "" || secondID == "" || firstID == secondID {
		t.Fatalf("session ids = %q and %q, want unique non-empty ids", firstID, secondID)
	}
	if !strings.HasPrefix(firstID, "subsess_") || !strings.HasPrefix(secondID, "subsess_") {
		t.Fatalf("session ids = %q and %q, want subagent prefix", firstID, secondID)
	}
}
