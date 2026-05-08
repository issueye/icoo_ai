package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	UserConfigPath    = ".icoo-ai/config.toml"
	ProjectConfigFile = ".icoo-ai.toml"
	envPrefix         = "ICOO_AI_"
)

type LoadOptions struct {
	HomeDir   string
	CWD       string
	Env       map[string]string
	Overrides ConfigPatch
}

func Load(opts LoadOptions) (Config, error) {
	cfg := Default()

	homeDir, err := resolveHomeDir(opts.HomeDir)
	if err != nil {
		return Config{}, err
	}
	cwd, err := resolveCWD(opts.CWD)
	if err != nil {
		return Config{}, err
	}
	env := opts.Env
	if env == nil {
		env = osEnv()
	}

	userPath := filepath.Join(homeDir, filepath.FromSlash(UserConfigPath))
	if err := applyConfigFile(&cfg, userPath); err != nil {
		return Config{}, err
	}
	projectPath := filepath.Join(cwd, ProjectConfigFile)
	if err := applyConfigFile(&cfg, projectPath); err != nil {
		return Config{}, err
	}
	patch, err := envPatch(env)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.applyPatch(patch); err != nil {
		return Config{}, fmt.Errorf("environment config: %w", err)
	}
	if err := cfg.applyPatch(opts.Overrides); err != nil {
		return Config{}, fmt.Errorf("explicit config overrides: %w", err)
	}

	return cfg, nil
}

func applyConfigFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config %s: %w", path, err)
	}

	var patch ConfigPatch
	if err := toml.Unmarshal(data, &patch); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.applyPatch(patch); err != nil {
		return fmt.Errorf("apply config %s: %w", path, err)
	}
	return nil
}

func resolveHomeDir(homeDir string) (string, error) {
	if homeDir != "" {
		return homeDir, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if homeDir == "" {
		return "", fmt.Errorf("resolve home directory: empty path")
	}
	return homeDir, nil
}

func resolveCWD(cwd string) (string, error) {
	if cwd != "" {
		return cwd, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve current working directory: %w", err)
	}
	return cwd, nil
}

func osEnv() map[string]string {
	env := make(map[string]string, len(os.Environ()))
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func envPatch(env map[string]string) (ConfigPatch, error) {
	var patch ConfigPatch

	setString(env, envPrefix+"MODEL", &patch.Model)
	setString(env, envPrefix+"PROVIDER", &patch.Provider)
	setString(env, envPrefix+"API_KEY", &patch.APIKey)
	setString(env, envPrefix+"OPENAI_API_KEY", &patch.APIKey)
	setString(env, envPrefix+"BASE_URL", &patch.BaseURL)
	setString(env, envPrefix+"API", &patch.API)
	setPermissionMode(env, envPrefix+"APPROVAL_MODE", &patch.ApprovalMode)
	if err := setInt(env, envPrefix+"MAX_CONTEXT_TOKENS", &patch.MaxContextTokens); err != nil {
		return ConfigPatch{}, err
	}
	if err := setInt(env, envPrefix+"SHELL_TIMEOUT_SECONDS", &patch.ShellTimeoutSeconds); err != nil {
		return ConfigPatch{}, err
	}
	if err := setBool(env, envPrefix+"RESPECT_GITIGNORE", &patch.RespectGitignore); err != nil {
		return ConfigPatch{}, err
	}
	setString(env, envPrefix+"AGENT_LOOP", &patch.AgentLoop)
	if err := setBool(env, envPrefix+"CLAUDE_CODE_COMPAT", &patch.ClaudeCodeCompat); err != nil {
		return ConfigPatch{}, err
	}

	var webSearch WebSearchPatch
	setString(env, envPrefix+"WEB_SEARCH_PROVIDER", &webSearch.Provider)
	if webSearch.Provider != nil {
		patch.WebSearch = &webSearch
	}

	var retry RetryPatch
	if err := setInt(env, envPrefix+"RETRY_MAX_ATTEMPTS", &retry.MaxAttempts); err != nil {
		return ConfigPatch{}, err
	}
	if err := setInt(env, envPrefix+"RETRY_INITIAL_DELAY_MILLIS", &retry.InitialDelayMillis); err != nil {
		return ConfigPatch{}, err
	}
	if err := setInt(env, envPrefix+"RETRY_MAX_DELAY_MILLIS", &retry.MaxDelayMillis); err != nil {
		return ConfigPatch{}, err
	}
	if retry.MaxAttempts != nil || retry.InitialDelayMillis != nil || retry.MaxDelayMillis != nil {
		patch.Retry = &retry
	}

	var skills SkillsPatch
	setStringSlice(env, envPrefix+"SKILLS_ENABLED", &skills.Enabled)
	setStringSlice(env, envPrefix+"SKILLS_DISABLED", &skills.Disabled)
	setStringSlice(env, envPrefix+"SKILLS_PATHS", &skills.Paths)
	if skills.Enabled != nil || skills.Disabled != nil || skills.Paths != nil {
		patch.Skills = &skills
	}

	var hooks HooksPatch
	if err := setBool(env, envPrefix+"HOOKS_ENABLED", &hooks.Enabled); err != nil {
		return ConfigPatch{}, err
	}
	if hooks.Enabled != nil {
		patch.Hooks = &hooks
	}

	var audit AuditPatch
	if err := setBool(env, envPrefix+"AUDIT_ENABLED", &audit.Enabled); err != nil {
		return ConfigPatch{}, err
	}
	setString(env, envPrefix+"AUDIT_PATH", &audit.Path)
	setString(env, envPrefix+"AUDIT_FORMAT", &audit.Format)
	if err := setInt(env, envPrefix+"AUDIT_MAX_SIZE_MB", &audit.MaxSizeMB); err != nil {
		return ConfigPatch{}, err
	}
	if err := setInt(env, envPrefix+"AUDIT_MAX_BACKUPS", &audit.MaxBackups); err != nil {
		return ConfigPatch{}, err
	}
	if audit.Enabled != nil || audit.Path != nil || audit.Format != nil || audit.MaxSizeMB != nil || audit.MaxBackups != nil {
		patch.Audit = &audit
	}

	var mcp MCPPatch
	if err := setBool(env, envPrefix+"MCP_ENABLED", &mcp.Enabled); err != nil {
		return ConfigPatch{}, err
	}
	if mcp.Enabled != nil {
		patch.MCP = &mcp
	}

	return patch, nil
}

func setString(env map[string]string, name string, target **string) {
	if value, ok := env[name]; ok {
		v := value
		*target = &v
	}
}

func setPermissionMode(env map[string]string, name string, target **ApprovalMode) {
	if value, ok := env[name]; ok {
		mode := ApprovalMode(value)
		*target = &mode
	}
}

func setInt(env map[string]string, name string, target **int) error {
	if value, ok := env[name]; ok {
		v, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse environment variable %s as int: %w", name, err)
		}
		*target = &v
	}
	return nil
}

func setBool(env map[string]string, name string, target **bool) error {
	if value, ok := env[name]; ok {
		v, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse environment variable %s as bool: %w", name, err)
		}
		*target = &v
	}
	return nil
}

func setStringSlice(env map[string]string, name string, target **[]string) {
	if value, ok := env[name]; ok {
		items := strings.Split(value, ",")
		out := make([]string, 0, len(items))
		for _, item := range items {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		*target = &out
	}
}
