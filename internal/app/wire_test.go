package app

import (
	"context"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/config"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/testutil"
)

func TestBuildRequiresOpenAIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	_, err := Build(context.Background(), BuildOptions{
		Config: config.Default(),
		CWD:    t.TempDir(),
		Home:   t.TempDir(),
	})
	if err == nil {
		t.Fatal("Build() error = nil, want missing key")
	}
}

func TestBuildCreatesComponents(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg := config.Default()
	cfg.Model = "gpt-4.1"
	cfg.ApprovalMode = config.ApprovalModeWorkspaceWrite
	components, err := Build(context.Background(), BuildOptions{
		Config:   cfg,
		CWD:      t.TempDir(),
		Home:     t.TempDir(),
		Provider: testutil.NewMockLLMProvider("mock", []llm.CompletionEvent{{Type: llm.CompletionEventCompleted}}),
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if components.Runtime == nil || components.Loop == nil || len(components.Tools) == 0 {
		t.Fatalf("components incomplete: %+v", components)
	}
}
