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
	t.Setenv("ICOO_AI_OPENAI_API_KEY", "")
	t.Setenv("ICOO_AI_API_KEY", "")
	_, err := Build(context.Background(), BuildOptions{
		Config: config.Default(),
		CWD:    t.TempDir(),
		Home:   t.TempDir(),
	})
	if err == nil {
		t.Fatal("Build() error = nil, want missing key")
	}
}

func TestBuildUsesConfigAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("ICOO_AI_OPENAI_API_KEY", "")
	t.Setenv("ICOO_AI_API_KEY", "")
	cfg := config.Default()
	cfg.Model = "gpt-4.1"
	cfg.APIKey = "config-key"
	components, err := Build(context.Background(), BuildOptions{
		Config: cfg,
		CWD:    t.TempDir(),
		Home:   t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if components.Runtime == nil {
		t.Fatal("Runtime = nil")
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
	toolNames := map[string]bool{}
	for _, tool := range components.Tools {
		toolNames[tool.Name()] = true
	}
	for _, name := range []string{"subagent_run", "skill_list", "skill_get", "skill_add", "skill_delete", "skill_execute"} {
		if !toolNames[name] {
			t.Fatalf("tool %q was not registered", name)
		}
	}
}

func TestBuildReturnsMCPConfigError(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key")
	cfg := config.Default()
	cfg.Model = "gpt-4.1"
	cfg.MCP.Enabled = true
	cfg.MCP.Servers = map[string]config.MCPServerConfig{
		"fs": {Enabled: true, Transport: "stdio", Command: "mcp-server"},
	}

	_, err := Build(context.Background(), BuildOptions{
		Config:   cfg,
		CWD:      t.TempDir(),
		Home:     t.TempDir(),
		Provider: testutil.NewMockLLMProvider("mock"),
	})
	if err == nil {
		t.Fatal("Build() error = nil, want MCP command error")
	}
}
