package config

import (
	"errors"
)

const (
	DefaultProvider          = "openai"
	DefaultAPI               = "responses"
	DefaultApprovalMode      = ApprovalModeWorkspaceWrite
	DefaultWebSearchProvider = "duckduckgo"
	DefaultAgentLoop         = "react"
)

var ErrMigrationNotImplemented = errors.New("claude code config migration is not implemented")

type Config struct {
	Model               string          `json:"model,omitempty" toml:"model,omitempty"`
	Provider            string          `json:"provider" toml:"provider"`
	BaseURL             string          `json:"base_url,omitempty" toml:"base_url,omitempty"`
	API                 string          `json:"api" toml:"api"`
	ApprovalMode        ApprovalMode    `json:"approval_mode" toml:"approval_mode"`
	MaxContextTokens    int             `json:"max_context_tokens,omitempty" toml:"max_context_tokens,omitempty"`
	ShellTimeoutSeconds int             `json:"shell_timeout_seconds,omitempty" toml:"shell_timeout_seconds,omitempty"`
	RespectGitignore    bool            `json:"respect_gitignore" toml:"respect_gitignore"`
	AgentLoop           string          `json:"agent_loop" toml:"agent_loop"`
	ClaudeCodeCompat    bool            `json:"claude_code_compat,omitempty" toml:"claude_code_compat,omitempty"`
	WebSearch           WebSearchConfig `json:"web_search" toml:"web_search"`
	Skills              SkillsConfig    `json:"skills" toml:"skills"`
	Hooks               HooksConfig     `json:"hooks" toml:"hooks"`
	Audit               AuditConfig     `json:"audit" toml:"audit"`
	MCP                 MCPConfig       `json:"mcp" toml:"mcp"`
}

type ApprovalMode string

const (
	ApprovalModeReadonly       ApprovalMode = "readonly"
	ApprovalModeSuggest        ApprovalMode = "suggest"
	ApprovalModeWorkspaceWrite ApprovalMode = "workspace-write"
	ApprovalModeFullAuto       ApprovalMode = "full-auto"
)

type WebSearchConfig struct {
	Provider string `json:"provider" toml:"provider"`
}

type SkillsConfig struct {
	Enabled  []string `json:"enabled,omitempty" toml:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty" toml:"disabled,omitempty"`
	Paths    []string `json:"paths,omitempty" toml:"paths,omitempty"`
}

type HooksConfig struct {
	Enabled            bool         `json:"enabled" toml:"enabled"`
	BeforeRun          []HookConfig `json:"before_run,omitempty" toml:"before_run,omitempty"`
	AfterRun           []HookConfig `json:"after_run,omitempty" toml:"after_run,omitempty"`
	BeforeToolCall     []HookConfig `json:"before_tool_call,omitempty" toml:"before_tool_call,omitempty"`
	AfterToolCall      []HookConfig `json:"after_tool_call,omitempty" toml:"after_tool_call,omitempty"`
	BeforeFileWrite    []HookConfig `json:"before_file_write,omitempty" toml:"before_file_write,omitempty"`
	AfterFileWrite     []HookConfig `json:"after_file_write,omitempty" toml:"after_file_write,omitempty"`
	BeforeShellCommand []HookConfig `json:"before_shell_command,omitempty" toml:"before_shell_command,omitempty"`
	AfterShellCommand  []HookConfig `json:"after_shell_command,omitempty" toml:"after_shell_command,omitempty"`
	OnError            []HookConfig `json:"on_error,omitempty" toml:"on_error,omitempty"`
}

type HookConfig struct {
	Name    string            `json:"name" toml:"name"`
	Type    string            `json:"type" toml:"type"`
	Command string            `json:"command,omitempty" toml:"command,omitempty"`
	Args    []string          `json:"args,omitempty" toml:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty" toml:"env,omitempty"`
	Enabled bool              `json:"enabled,omitempty" toml:"enabled,omitempty"`
}

type AuditConfig struct {
	Enabled bool              `json:"enabled" toml:"enabled"`
	Path    string            `json:"path,omitempty" toml:"path,omitempty"`
	Format  string            `json:"format,omitempty" toml:"format,omitempty"`
	Remote  AuditRemoteConfig `json:"remote,omitempty" toml:"remote,omitempty"`
}

type AuditRemoteConfig struct {
	Enabled bool              `json:"enabled" toml:"enabled"`
	URL     string            `json:"url,omitempty" toml:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty" toml:"headers,omitempty"`
}

type MCPConfig struct {
	Enabled bool                       `json:"enabled" toml:"enabled"`
	Servers map[string]MCPServerConfig `json:"servers,omitempty" toml:"servers,omitempty"`
}

type MCPServerConfig struct {
	Enabled   bool              `json:"enabled" toml:"enabled"`
	Transport string            `json:"transport,omitempty" toml:"transport,omitempty"`
	Command   string            `json:"command,omitempty" toml:"command,omitempty"`
	Args      []string          `json:"args,omitempty" toml:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty" toml:"env,omitempty"`
	URL       string            `json:"url,omitempty" toml:"url,omitempty"`
}

type ConfigPatch struct {
	Model               *string         `toml:"model,omitempty"`
	Provider            *string         `toml:"provider,omitempty"`
	BaseURL             *string         `toml:"base_url,omitempty"`
	API                 *string         `toml:"api,omitempty"`
	ApprovalMode        *ApprovalMode   `toml:"approval_mode,omitempty"`
	MaxContextTokens    *int            `toml:"max_context_tokens,omitempty"`
	ShellTimeoutSeconds *int            `toml:"shell_timeout_seconds,omitempty"`
	RespectGitignore    *bool           `toml:"respect_gitignore,omitempty"`
	AgentLoop           *string         `toml:"agent_loop,omitempty"`
	ClaudeCodeCompat    *bool           `toml:"claude_code_compat,omitempty"`
	WebSearch           *WebSearchPatch `toml:"web_search,omitempty"`
	Skills              *SkillsPatch    `toml:"skills,omitempty"`
	Hooks               *HooksPatch     `toml:"hooks,omitempty"`
	Audit               *AuditPatch     `toml:"audit,omitempty"`
	MCP                 *MCPPatch       `toml:"mcp,omitempty"`
}

type WebSearchPatch struct {
	Provider *string `toml:"provider,omitempty"`
}

type SkillsPatch struct {
	Enabled  *[]string `toml:"enabled,omitempty"`
	Disabled *[]string `toml:"disabled,omitempty"`
	Paths    *[]string `toml:"paths,omitempty"`
}

type HooksPatch struct {
	Enabled            *bool         `toml:"enabled,omitempty"`
	BeforeRun          *[]HookConfig `toml:"before_run,omitempty"`
	AfterRun           *[]HookConfig `toml:"after_run,omitempty"`
	BeforeToolCall     *[]HookConfig `toml:"before_tool_call,omitempty"`
	AfterToolCall      *[]HookConfig `toml:"after_tool_call,omitempty"`
	BeforeFileWrite    *[]HookConfig `toml:"before_file_write,omitempty"`
	AfterFileWrite     *[]HookConfig `toml:"after_file_write,omitempty"`
	BeforeShellCommand *[]HookConfig `toml:"before_shell_command,omitempty"`
	AfterShellCommand  *[]HookConfig `toml:"after_shell_command,omitempty"`
	OnError            *[]HookConfig `toml:"on_error,omitempty"`
}

type AuditPatch struct {
	Enabled *bool             `toml:"enabled,omitempty"`
	Path    *string           `toml:"path,omitempty"`
	Format  *string           `toml:"format,omitempty"`
	Remote  *AuditRemotePatch `toml:"remote,omitempty"`
}

type AuditRemotePatch struct {
	Enabled *bool              `toml:"enabled,omitempty"`
	URL     *string            `toml:"url,omitempty"`
	Headers *map[string]string `toml:"headers,omitempty"`
}

type MCPPatch struct {
	Enabled *bool                     `toml:"enabled,omitempty"`
	Servers map[string]MCPServerPatch `toml:"servers,omitempty"`
}

type MCPServerPatch struct {
	Enabled   *bool              `toml:"enabled,omitempty"`
	Transport *string            `toml:"transport,omitempty"`
	Command   *string            `toml:"command,omitempty"`
	Args      *[]string          `toml:"args,omitempty"`
	Env       *map[string]string `toml:"env,omitempty"`
	URL       *string            `toml:"url,omitempty"`
}

type ClaudeCodeMigrationOptions struct {
	SourcePath string
	DestPath   string
}

func Default() Config {
	return Config{
		Provider:         DefaultProvider,
		API:              DefaultAPI,
		ApprovalMode:     DefaultApprovalMode,
		RespectGitignore: true,
		AgentLoop:        DefaultAgentLoop,
		WebSearch: WebSearchConfig{
			Provider: DefaultWebSearchProvider,
		},
		Skills: SkillsConfig{},
		Hooks:  HooksConfig{},
		Audit: AuditConfig{
			Enabled: true,
			Format:  "jsonl",
		},
		MCP: MCPConfig{
			Servers: map[string]MCPServerConfig{},
		},
	}
}

func MigrateClaudeCodeConfig(opts ClaudeCodeMigrationOptions) error {
	return ErrMigrationNotImplemented
}
