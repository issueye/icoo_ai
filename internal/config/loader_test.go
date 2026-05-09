package config

import (
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
	if cfg.Retry.MaxAttempts != 3 || cfg.Retry.InitialDelayMillis != 500 || cfg.Retry.MaxDelayMillis != 5000 {
		t.Fatalf("Retry defaults = %+v", cfg.Retry)
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
api_key = "user-key"
approval_mode = "readonly"
respect_gitignore = false

[web_search]
provider = "user-search"

[network]
http_proxy = "http://user-proxy:8080"
https_proxy = "http://user-secure-proxy:8080"
no_proxy = "localhost,.internal"

[network.llm]
https_proxy = "http://user-llm-proxy:8080"

[network.duckduckgo]
http_proxy = "http://user-ddg-proxy:8080"

[retry]
max_attempts = 4
initial_delay_millis = 100
max_delay_millis = 1000

[skills]
enabled = ["user-skill"]

[audit]
path = "user-audit.jsonl"
max_size_mb = 2
max_backups = 3
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
	if cfg.APIKey != "user-key" {
		t.Fatalf("APIKey = %q, want user-key", cfg.APIKey)
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
	if cfg.Network.HTTPProxy != "http://user-proxy:8080" || cfg.Network.HTTPSProxy != "http://user-secure-proxy:8080" || cfg.Network.NoProxy != "localhost,.internal" {
		t.Fatalf("Network = %+v", cfg.Network)
	}
	if cfg.Network.LLM.HTTPSProxy != "http://user-llm-proxy:8080" {
		t.Fatalf("Network.LLM = %+v", cfg.Network.LLM)
	}
	if cfg.Network.DuckDuckGo.HTTPProxy != "http://user-ddg-proxy:8080" {
		t.Fatalf("Network.DuckDuckGo = %+v", cfg.Network.DuckDuckGo)
	}
	if cfg.Retry.MaxAttempts != 4 || cfg.Retry.InitialDelayMillis != 100 || cfg.Retry.MaxDelayMillis != 1000 {
		t.Fatalf("Retry = %+v, want user retry config", cfg.Retry)
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
	if cfg.Audit.MaxSizeMB != 2 || cfg.Audit.MaxBackups != 3 {
		t.Fatalf("Audit rotation = %d/%d, want 2/3", cfg.Audit.MaxSizeMB, cfg.Audit.MaxBackups)
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
			"ICOO_AI_MODEL":                      "env-model",
			"ICOO_AI_PROVIDER":                   "env-provider",
			"ICOO_AI_API_KEY":                    "env-key",
			"ICOO_AI_APPROVAL_MODE":              "full-auto",
			"ICOO_AI_RESPECT_GITIGNORE":          "false",
			"ICOO_AI_WEB_SEARCH_PROVIDER":        "env-search",
			"ICOO_AI_HTTP_PROXY":                 "http://env-proxy:8080",
			"ICOO_AI_HTTPS_PROXY":                "http://env-secure-proxy:8080",
			"ICOO_AI_NO_PROXY":                   "localhost",
			"ICOO_AI_LLM_HTTPS_PROXY":            "http://env-llm-proxy:8080",
			"ICOO_AI_DUCKDUCKGO_HTTP_PROXY":      "http://env-ddg-proxy:8080",
			"ICOO_AI_RETRY_MAX_ATTEMPTS":         "5",
			"ICOO_AI_RETRY_INITIAL_DELAY_MILLIS": "25",
			"ICOO_AI_RETRY_MAX_DELAY_MILLIS":     "250",
			"ICOO_AI_SKILLS_ENABLED":             "go-code-review,tests",
			"ICOO_AI_HOOKS_ENABLED":              "true",
			"ICOO_AI_AUDIT_ENABLED":              "false",
			"ICOO_AI_AUDIT_MAX_SIZE_MB":          "4",
			"ICOO_AI_AUDIT_MAX_BACKUPS":          "7",
			"ICOO_AI_MCP_ENABLED":                "true",
			"ICOO_AI_SUBAGENTS_MODEL":            "sub-model",
			"ICOO_AI_SUBAGENTS_MAX_TOOL_ROUNDS":  "9",
			"ICOO_AI_SUBAGENTS_POOL_CONCURRENCY": "3",
			"ICOO_AI_SUBAGENTS_POOL_QUEUE_SIZE":  "21",
			"ICOO_AI_SHELL_TIMEOUT_SECONDS":      "90",
		},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Model != "env-model" || cfg.Provider != "env-provider" {
		t.Fatalf("environment did not override top-level config: %+v", cfg)
	}
	if cfg.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want env-key", cfg.APIKey)
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
	if cfg.Network.HTTPProxy != "http://env-proxy:8080" || cfg.Network.HTTPSProxy != "http://env-secure-proxy:8080" || cfg.Network.NoProxy != "localhost" {
		t.Fatalf("Network = %+v, want env proxy config", cfg.Network)
	}
	if cfg.Network.LLM.HTTPSProxy != "http://env-llm-proxy:8080" {
		t.Fatalf("Network.LLM = %+v, want env llm proxy config", cfg.Network.LLM)
	}
	if cfg.Network.DuckDuckGo.HTTPProxy != "http://env-ddg-proxy:8080" {
		t.Fatalf("Network.DuckDuckGo = %+v, want env duckduckgo proxy config", cfg.Network.DuckDuckGo)
	}
	if cfg.Retry.MaxAttempts != 5 || cfg.Retry.InitialDelayMillis != 25 || cfg.Retry.MaxDelayMillis != 250 {
		t.Fatalf("Retry = %+v, want env retry config", cfg.Retry)
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
	if cfg.Audit.MaxSizeMB != 4 || cfg.Audit.MaxBackups != 7 {
		t.Fatalf("Audit rotation = %d/%d, want 4/7", cfg.Audit.MaxSizeMB, cfg.Audit.MaxBackups)
	}
	if !cfg.MCP.Enabled {
		t.Fatalf("MCP.Enabled = false, want true")
	}
	if cfg.Subagents.Model != "sub-model" || cfg.Subagents.MaxToolRounds != 9 {
		t.Fatalf("Subagents = %+v, want env subagent config", cfg.Subagents)
	}
	if cfg.Subagents.Pool.Concurrency != 3 || cfg.Subagents.Pool.QueueSize != 21 {
		t.Fatalf("Subagents.Pool = %+v, want env pool config", cfg.Subagents.Pool)
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
	overrideAPIKey := "explicit-key"
	overrideAPI := "explicit-api"
	overrideApproval := ApprovalModeReadonly
	overrideSearch := "explicit-search"
	overrideHTTPProxy := "http://explicit-proxy:8080"
	overrideLLMProxy := "http://explicit-llm-proxy:8080"
	overrideRetryAttempts := 6
	overrideHooks := false
	cfg, err := Load(LoadOptions{
		HomeDir: home,
		CWD:     cwd,
		Env: map[string]string{
			"ICOO_AI_PROVIDER":      "env-provider",
			"ICOO_AI_API_KEY":       "env-key",
			"ICOO_AI_API":           "env-api",
			"ICOO_AI_HOOKS_ENABLED": "true",
		},
		Overrides: ConfigPatch{
			Provider:     &overrideProvider,
			APIKey:       &overrideAPIKey,
			API:          &overrideAPI,
			ApprovalMode: &overrideApproval,
			WebSearch: &WebSearchPatch{
				Provider: &overrideSearch,
			},
			Network: &NetworkPatch{
				HTTPProxy: &overrideHTTPProxy,
				LLM: &NetworkProxyPatch{
					HTTPSProxy: &overrideLLMProxy,
				},
			},
			Retry: &RetryPatch{
				MaxAttempts: &overrideRetryAttempts,
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
	if cfg.APIKey != "explicit-key" {
		t.Fatalf("APIKey = %q, want explicit-key", cfg.APIKey)
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
	if cfg.Network.HTTPProxy != "http://explicit-proxy:8080" {
		t.Fatalf("Network.HTTPProxy = %q, want explicit proxy", cfg.Network.HTTPProxy)
	}
	if cfg.Network.LLM.HTTPSProxy != "http://explicit-llm-proxy:8080" {
		t.Fatalf("Network.LLM.HTTPSProxy = %q, want explicit llm proxy", cfg.Network.LLM.HTTPSProxy)
	}
	if cfg.Retry.MaxAttempts != 6 {
		t.Fatalf("Retry.MaxAttempts = %d, want 6", cfg.Retry.MaxAttempts)
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

func TestMigrateClaudeCodeConfig(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "claude.json")
	dest := filepath.Join(dir, ".icoo-ai", "config.toml")
	writeFile(t, source, `{
		"model": "gpt-4.1",
		"apiKey": "claude-key",
		"permissionMode": "workspace-write",
		"shellTimeoutSeconds": 42,
		"skills": ["go-code-review"],
		"mcpServers": {
			"filesystem": {
				"transport": "stdio",
				"command": "mcp-server-filesystem",
				"args": ["."]
			}
		}
	}`)

	if err := MigrateClaudeCodeConfig(ClaudeCodeMigrationOptions{SourcePath: source, DestPath: dest}); err != nil {
		t.Fatalf("MigrateClaudeCodeConfig() error = %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `model = 'gpt-4.1'`) && !strings.Contains(text, `model = "gpt-4.1"`) {
		t.Fatalf("migrated config missing model: %s", text)
	}
	if !strings.Contains(text, `api_key = 'claude-key'`) && !strings.Contains(text, `api_key = "claude-key"`) {
		t.Fatalf("migrated config missing api_key: %s", text)
	}
	if !strings.Contains(text, "claude_code_compat = true") {
		t.Fatalf("migrated config missing compat flag: %s", text)
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
