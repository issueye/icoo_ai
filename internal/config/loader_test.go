package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()

	cfg, err := Load(LoadOptions{
		HomeDir: home,
		CWD:     cwd,
		Env:     map[string]string{},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != "openai" {
		t.Fatalf("Provider = %q, want openai", cfg.Provider)
	}
	if cfg.API != "responses" {
		t.Fatalf("API = %q, want responses", cfg.API)
	}
	if cfg.ApprovalMode != ApprovalModeWorkspaceWrite {
		t.Fatalf("ApprovalMode = %q, want workspace-write", cfg.ApprovalMode)
	}
	if cfg.WebSearch.Provider != "duckduckgo" {
		t.Fatalf("WebSearch.Provider = %q, want duckduckgo", cfg.WebSearch.Provider)
	}
	if cfg.AgentLoop != "react" {
		t.Fatalf("AgentLoop = %q, want react", cfg.AgentLoop)
	}
	if !cfg.RespectGitignore {
		t.Fatalf("RespectGitignore = false, want true")
	}
}

func TestLoadAppliesUserThenProjectConfig(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(home, ".icoo-ai", "config.toml"), `
model = "user-model"
provider = "anthropic"
approval_mode = "readonly"
respect_gitignore = false

[web_search]
provider = "user-search"

[skills]
enabled = ["user-skill"]

[audit]
path = "user-audit.jsonl"
`)
	writeFile(t, filepath.Join(cwd, ".icoo-ai.toml"), `
model = "project-model"
api = "chat_completions"
approval_mode = "suggest"
agent_loop = "plan-act"

[skills]
disabled = ["project-disabled"]

[hooks]
enabled = true

[[hooks.before_shell_command]]
name = "block-dangerous-rm"
type = "builtin"

[mcp]
enabled = true

[mcp.servers.filesystem]
enabled = true
transport = "stdio"
command = "mcp-server-filesystem"
args = ["."]
`)

	cfg, err := Load(LoadOptions{
		HomeDir: home,
		CWD:     cwd,
		Env:     map[string]string{},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "project-model" {
		t.Fatalf("Model = %q, want project-model", cfg.Model)
	}
	if cfg.Provider != "anthropic" {
		t.Fatalf("Provider = %q, want user provider to remain", cfg.Provider)
	}
	if cfg.API != "chat_completions" {
		t.Fatalf("API = %q, want project value", cfg.API)
	}
	if cfg.ApprovalMode != ApprovalModeSuggest {
		t.Fatalf("ApprovalMode = %q, want suggest", cfg.ApprovalMode)
	}
	if cfg.WebSearch.Provider != "user-search" {
		t.Fatalf("WebSearch.Provider = %q, want user-search", cfg.WebSearch.Provider)
	}
	if cfg.RespectGitignore {
		t.Fatalf("RespectGitignore = true, want explicit user false")
	}
	if cfg.AgentLoop != "plan-act" {
		t.Fatalf("AgentLoop = %q, want plan-act", cfg.AgentLoop)
	}
	if got := strings.Join(cfg.Skills.Enabled, ","); got != "user-skill" {
		t.Fatalf("Skills.Enabled = %q, want user-skill", got)
	}
	if got := strings.Join(cfg.Skills.Disabled, ","); got != "project-disabled" {
		t.Fatalf("Skills.Disabled = %q, want project-disabled", got)
	}
	if !cfg.Hooks.Enabled || len(cfg.Hooks.BeforeShellCommand) != 1 {
		t.Fatalf("Hooks config not applied: %+v", cfg.Hooks)
	}
	if !cfg.MCP.Enabled {
		t.Fatalf("MCP.Enabled = false, want true")
	}
	server := cfg.MCP.Servers["filesystem"]
	if !server.Enabled || server.Transport != "stdio" || server.Command != "mcp-server-filesystem" {
		t.Fatalf("MCP server not applied: %+v", server)
	}
}

func TestLoadEnvironmentOverridesFiles(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, ".icoo-ai.toml"), `
model = "project-model"
provider = "project-provider"
approval_mode = "readonly"
respect_gitignore = true
[web_search]
provider = "project-search"
`)

	cfg, err := Load(LoadOptions{
		HomeDir: home,
		CWD:     cwd,
		Env: map[string]string{
			"ICOO_AI_MODEL":                 "env-model",
			"ICOO_AI_PROVIDER":              "env-provider",
			"ICOO_AI_APPROVAL_MODE":         "full-auto",
			"ICOO_AI_RESPECT_GITIGNORE":     "false",
			"ICOO_AI_WEB_SEARCH_PROVIDER":   "env-search",
			"ICOO_AI_SKILLS_ENABLED":        "go-code-review,tests",
			"ICOO_AI_HOOKS_ENABLED":         "true",
			"ICOO_AI_AUDIT_ENABLED":         "false",
			"ICOO_AI_MCP_ENABLED":           "true",
			"ICOO_AI_SHELL_TIMEOUT_SECONDS": "90",
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "env-model" || cfg.Provider != "env-provider" {
		t.Fatalf("environment did not override top-level config: %+v", cfg)
	}
	if cfg.ApprovalMode != ApprovalModeFullAuto {
		t.Fatalf("ApprovalMode = %q, want full-auto", cfg.ApprovalMode)
	}
	if cfg.RespectGitignore {
		t.Fatalf("RespectGitignore = true, want false")
	}
	if cfg.WebSearch.Provider != "env-search" {
		t.Fatalf("WebSearch.Provider = %q, want env-search", cfg.WebSearch.Provider)
	}
	if got := strings.Join(cfg.Skills.Enabled, ","); got != "go-code-review,tests" {
		t.Fatalf("Skills.Enabled = %q, want go-code-review,tests", got)
	}
	if !cfg.Hooks.Enabled {
		t.Fatalf("Hooks.Enabled = false, want true")
	}
	if cfg.Audit.Enabled {
		t.Fatalf("Audit.Enabled = true, want false")
	}
	if !cfg.MCP.Enabled {
		t.Fatalf("MCP.Enabled = false, want true")
	}
	if cfg.ShellTimeoutSeconds != 90 {
		t.Fatalf("ShellTimeoutSeconds = %d, want 90", cfg.ShellTimeoutSeconds)
	}
}

func TestLoadExplicitOverridesHaveHighestPriority(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, ".icoo-ai.toml"), `provider = "project"`)

	overrideProvider := "explicit-provider"
	overrideAPI := "explicit-api"
	overrideApproval := ApprovalModeReadonly
	overrideSearch := "explicit-search"
	overrideHooks := false
	cfg, err := Load(LoadOptions{
		HomeDir: home,
		CWD:     cwd,
		Env: map[string]string{
			"ICOO_AI_PROVIDER":      "env-provider",
			"ICOO_AI_API":           "env-api",
			"ICOO_AI_HOOKS_ENABLED": "true",
		},
		Overrides: ConfigPatch{
			Provider:     &overrideProvider,
			API:          &overrideAPI,
			ApprovalMode: &overrideApproval,
			WebSearch: &WebSearchPatch{
				Provider: &overrideSearch,
			},
			Hooks: &HooksPatch{
				Enabled: &overrideHooks,
			},
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Provider != "explicit-provider" {
		t.Fatalf("Provider = %q, want explicit-provider", cfg.Provider)
	}
	if cfg.API != "explicit-api" {
		t.Fatalf("API = %q, want explicit-api", cfg.API)
	}
	if cfg.ApprovalMode != ApprovalModeReadonly {
		t.Fatalf("ApprovalMode = %q, want readonly", cfg.ApprovalMode)
	}
	if cfg.WebSearch.Provider != "explicit-search" {
		t.Fatalf("WebSearch.Provider = %q, want explicit-search", cfg.WebSearch.Provider)
	}
	if cfg.Hooks.Enabled {
		t.Fatalf("Hooks.Enabled = true, want explicit false")
	}
}

func TestLoadReturnsClearParseError(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, ".icoo-ai.toml"), `provider = [`)

	_, err := Load(LoadOptions{
		HomeDir: home,
		CWD:     cwd,
		Env:     map[string]string{},
	})
	if err == nil {
		t.Fatalf("Load() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), ".icoo-ai.toml") || !strings.Contains(err.Error(), "parse config") {
		t.Fatalf("error = %q, want path and parse config context", err.Error())
	}
}

func TestLoadReturnsClearEnvironmentError(t *testing.T) {
	_, err := Load(LoadOptions{
		HomeDir: t.TempDir(),
		CWD:     t.TempDir(),
		Env: map[string]string{
			"ICOO_AI_HOOKS_ENABLED": "not-bool",
		},
	})
	if err == nil {
		t.Fatalf("Load() error = nil, want env error")
	}
	if !strings.Contains(err.Error(), "ICOO_AI_HOOKS_ENABLED") {
		t.Fatalf("error = %q, want env var name", err.Error())
	}
}

func TestLoadRejectsUnsupportedApprovalMode(t *testing.T) {
	cwd := t.TempDir()
	writeFile(t, filepath.Join(cwd, ".icoo-ai.toml"), `approval_mode = "danger"`)

	_, err := Load(LoadOptions{
		HomeDir: t.TempDir(),
		CWD:     cwd,
		Env:     map[string]string{},
	})
	if err == nil {
		t.Fatalf("Load() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), `approval_mode "danger" is not supported`) {
		t.Fatalf("error = %q, want unsupported approval_mode", err.Error())
	}
}

func TestMigrateClaudeCodeConfigPlaceholder(t *testing.T) {
	err := MigrateClaudeCodeConfig(ClaudeCodeMigrationOptions{})
	if !errors.Is(err, ErrMigrationNotImplemented) {
		t.Fatalf("MigrateClaudeCodeConfig() error = %v, want ErrMigrationNotImplemented", err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
