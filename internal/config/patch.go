package config

import "fmt"

func (cfg *Config) applyPatch(patch ConfigPatch) error {
	if patch.Model != nil {
		cfg.Model = *patch.Model
	}
	if patch.Provider != nil {
		cfg.Provider = *patch.Provider
	}
	if patch.APIKey != nil {
		cfg.APIKey = *patch.APIKey
	}
	if patch.BaseURL != nil {
		cfg.BaseURL = *patch.BaseURL
	}
	if patch.API != nil {
		cfg.API = *patch.API
	}
	if patch.ApprovalMode != nil {
		cfg.ApprovalMode = *patch.ApprovalMode
	}
	if patch.MaxContextTokens != nil {
		cfg.MaxContextTokens = *patch.MaxContextTokens
	}
	if patch.ShellTimeoutSeconds != nil {
		cfg.ShellTimeoutSeconds = *patch.ShellTimeoutSeconds
	}
	if patch.RespectGitignore != nil {
		cfg.RespectGitignore = *patch.RespectGitignore
	}
	if patch.AgentLoop != nil {
		cfg.AgentLoop = *patch.AgentLoop
	}
	if patch.ClaudeCodeCompat != nil {
		cfg.ClaudeCodeCompat = *patch.ClaudeCodeCompat
	}
	if patch.Network != nil {
		cfg.Network.applyPatch(*patch.Network)
	}
	if patch.WebSearch != nil {
		cfg.WebSearch.applyPatch(*patch.WebSearch)
	}
	if patch.Retry != nil {
		cfg.Retry.applyPatch(*patch.Retry)
	}
	if patch.Skills != nil {
		cfg.Skills.applyPatch(*patch.Skills)
	}
	if patch.Hooks != nil {
		cfg.Hooks.applyPatch(*patch.Hooks)
	}
	if patch.Audit != nil {
		cfg.Audit.applyPatch(*patch.Audit)
	}
	if patch.MCP != nil {
		cfg.MCP.applyPatch(*patch.MCP)
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	return nil
}

func (cfg *WebSearchConfig) applyPatch(patch WebSearchPatch) {
	if patch.Provider != nil {
		cfg.Provider = *patch.Provider
	}
}

func (cfg *NetworkConfig) applyPatch(patch NetworkPatch) {
	if patch.HTTPProxy != nil {
		cfg.HTTPProxy = *patch.HTTPProxy
	}
	if patch.HTTPSProxy != nil {
		cfg.HTTPSProxy = *patch.HTTPSProxy
	}
	if patch.NoProxy != nil {
		cfg.NoProxy = *patch.NoProxy
	}
	if patch.LLM != nil {
		cfg.LLM.applyPatch(*patch.LLM)
	}
	if patch.DuckDuckGo != nil {
		cfg.DuckDuckGo.applyPatch(*patch.DuckDuckGo)
	}
}

func (cfg *NetworkProxyConfig) applyPatch(patch NetworkProxyPatch) {
	if patch.HTTPProxy != nil {
		cfg.HTTPProxy = *patch.HTTPProxy
	}
	if patch.HTTPSProxy != nil {
		cfg.HTTPSProxy = *patch.HTTPSProxy
	}
	if patch.NoProxy != nil {
		cfg.NoProxy = *patch.NoProxy
	}
}

func (cfg *RetryConfig) applyPatch(patch RetryPatch) {
	if patch.MaxAttempts != nil {
		cfg.MaxAttempts = *patch.MaxAttempts
	}
	if patch.InitialDelayMillis != nil {
		cfg.InitialDelayMillis = *patch.InitialDelayMillis
	}
	if patch.MaxDelayMillis != nil {
		cfg.MaxDelayMillis = *patch.MaxDelayMillis
	}
}

func (cfg *SkillsConfig) applyPatch(patch SkillsPatch) {
	if patch.Enabled != nil {
		cfg.Enabled = append([]string(nil), (*patch.Enabled)...)
	}
	if patch.Disabled != nil {
		cfg.Disabled = append([]string(nil), (*patch.Disabled)...)
	}
	if patch.Paths != nil {
		cfg.Paths = append([]string(nil), (*patch.Paths)...)
	}
}

func (cfg *HooksConfig) applyPatch(patch HooksPatch) {
	if patch.Enabled != nil {
		cfg.Enabled = *patch.Enabled
	}
	if patch.BeforeRun != nil {
		cfg.BeforeRun = cloneHooks(*patch.BeforeRun)
	}
	if patch.AfterRun != nil {
		cfg.AfterRun = cloneHooks(*patch.AfterRun)
	}
	if patch.BeforeToolCall != nil {
		cfg.BeforeToolCall = cloneHooks(*patch.BeforeToolCall)
	}
	if patch.AfterToolCall != nil {
		cfg.AfterToolCall = cloneHooks(*patch.AfterToolCall)
	}
	if patch.BeforeFileWrite != nil {
		cfg.BeforeFileWrite = cloneHooks(*patch.BeforeFileWrite)
	}
	if patch.AfterFileWrite != nil {
		cfg.AfterFileWrite = cloneHooks(*patch.AfterFileWrite)
	}
	if patch.BeforeShellCommand != nil {
		cfg.BeforeShellCommand = cloneHooks(*patch.BeforeShellCommand)
	}
	if patch.AfterShellCommand != nil {
		cfg.AfterShellCommand = cloneHooks(*patch.AfterShellCommand)
	}
	if patch.OnError != nil {
		cfg.OnError = cloneHooks(*patch.OnError)
	}
}

func (cfg *AuditConfig) applyPatch(patch AuditPatch) {
	if patch.Enabled != nil {
		cfg.Enabled = *patch.Enabled
	}
	if patch.Path != nil {
		cfg.Path = *patch.Path
	}
	if patch.Format != nil {
		cfg.Format = *patch.Format
	}
	if patch.MaxSizeMB != nil {
		cfg.MaxSizeMB = *patch.MaxSizeMB
	}
	if patch.MaxBackups != nil {
		cfg.MaxBackups = *patch.MaxBackups
	}
	if patch.Remote != nil {
		cfg.Remote.applyPatch(*patch.Remote)
	}
}

func (cfg *AuditRemoteConfig) applyPatch(patch AuditRemotePatch) {
	if patch.Enabled != nil {
		cfg.Enabled = *patch.Enabled
	}
	if patch.URL != nil {
		cfg.URL = *patch.URL
	}
	if patch.Headers != nil {
		cfg.Headers = cloneStringMap(*patch.Headers)
	}
}

func (cfg *MCPConfig) applyPatch(patch MCPPatch) {
	if patch.Enabled != nil {
		cfg.Enabled = *patch.Enabled
	}
	if patch.Servers != nil {
		if cfg.Servers == nil {
			cfg.Servers = map[string]MCPServerConfig{}
		}
		for name, serverPatch := range patch.Servers {
			server := cfg.Servers[name]
			server.applyPatch(serverPatch)
			cfg.Servers[name] = server
		}
	}
}

func (cfg *MCPServerConfig) applyPatch(patch MCPServerPatch) {
	if patch.Enabled != nil {
		cfg.Enabled = *patch.Enabled
	}
	if patch.Transport != nil {
		cfg.Transport = *patch.Transport
	}
	if patch.Command != nil {
		cfg.Command = *patch.Command
	}
	if patch.Args != nil {
		cfg.Args = append([]string(nil), (*patch.Args)...)
	}
	if patch.Env != nil {
		cfg.Env = cloneStringMap(*patch.Env)
	}
	if patch.URL != nil {
		cfg.URL = *patch.URL
	}
}

func (cfg Config) validate() error {
	if cfg.Provider == "" {
		return fmt.Errorf("provider must not be empty")
	}
	if cfg.API == "" {
		return fmt.Errorf("api must not be empty")
	}
	if cfg.ApprovalMode == "" {
		return fmt.Errorf("approval_mode must not be empty")
	}
	switch cfg.ApprovalMode {
	case ApprovalModeReadonly,
		ApprovalModeSuggest,
		ApprovalModeWorkspaceWrite,
		ApprovalModeFullAuto:
	default:
		return fmt.Errorf("approval_mode %q is not supported", cfg.ApprovalMode)
	}
	if cfg.WebSearch.Provider == "" {
		return fmt.Errorf("web_search.provider must not be empty")
	}
	if cfg.AgentLoop == "" {
		return fmt.Errorf("agent_loop must not be empty")
	}
	if cfg.Retry.MaxAttempts < 0 {
		return fmt.Errorf("retry.max_attempts must not be negative")
	}
	if cfg.Retry.InitialDelayMillis < 0 {
		return fmt.Errorf("retry.initial_delay_millis must not be negative")
	}
	if cfg.Retry.MaxDelayMillis < 0 {
		return fmt.Errorf("retry.max_delay_millis must not be negative")
	}
	if cfg.Audit.MaxSizeMB < 0 {
		return fmt.Errorf("audit.max_size_mb must not be negative")
	}
	if cfg.Audit.MaxBackups < 0 {
		return fmt.Errorf("audit.max_backups must not be negative")
	}
	return nil
}

func cloneHooks(in []HookConfig) []HookConfig {
	out := append([]HookConfig(nil), in...)
	for i := range out {
		out[i].Args = append([]string(nil), out[i].Args...)
		out[i].Env = cloneStringMap(out[i].Env)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
