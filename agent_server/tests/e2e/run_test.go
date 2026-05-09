package e2e

import (
	"context"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/app"
	"github.com/icoo-ai/icoo-ai/internal/config"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/testutil"
)

func TestRuntimePromptSmoke(t *testing.T) {
	cfg := config.Default()
	cfg.Model = "gpt-4.1"
	components, err := app.Build(context.Background(), app.BuildOptions{
		Config: cfg,
		CWD:    t.TempDir(),
		Home:   t.TempDir(),
		Provider: testutil.NewMockLLMProvider("mock", []llm.CompletionEvent{
			{Type: llm.CompletionEventMessageDelta, Delta: "hello"},
			{Type: llm.CompletionEventCompleted},
		}),
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	session, err := components.Runtime.NewSession(context.Background(), agent.NewSessionRequest{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	events, err := components.Runtime.Prompt(context.Background(), agent.PromptRequest{
		SessionID: session.ID,
		Prompt:    "say hello",
	})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	got, err := testutil.CollectRuntimeEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("CollectEvents() error = %v", err)
	}
	if content := eventContent(got, agent.EventMessageDelta); content != "hello" {
		t.Fatalf("content = %q, want hello", content)
	}
}

func eventContent(events []agent.Event, typ agent.EventType) string {
	var content string
	for _, event := range events {
		if event.Type == typ {
			content += event.Content
		}
	}
	return content
}
