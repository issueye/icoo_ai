package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/config"
	"github.com/icoo-ai/icoo-ai/internal/llm"
	"github.com/icoo-ai/icoo-ai/internal/mcp"
	"github.com/icoo-ai/icoo-ai/internal/policy"
	"github.com/icoo-ai/icoo-ai/internal/protocol/acp"
	"github.com/icoo-ai/icoo-ai/internal/session"
	"github.com/icoo-ai/icoo-ai/internal/skills"
	"github.com/icoo-ai/icoo-ai/internal/skilltools"
	"github.com/icoo-ai/icoo-ai/internal/subagent"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type BuildOptions struct {
	Config   config.Config
	CWD      string
	Home     string
	Stdin    *os.File
	Stdout   *os.File
	Stderr   *os.File
	Provider llm.Provider
	Approver agent.Approver
}

type Components struct {
	Config  cfgView
	Policy  policy.Policy
	Audit   audit.Logger
	Tools   []tools.Tool
	Loop    agent.Loop
	Runtime agent.Runtime
}

type cfgView = config.Config

func Build(ctx context.Context, opts BuildOptions) (Components, error) {
	cwd := opts.CWD
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return Components{}, err
		}
	}
	home := opts.Home
	if home == "" {
		if resolved, err := os.UserHomeDir(); err == nil {
			home = resolved
		}
	}

	p := policy.New(policy.PermissionMode(opts.Config.ApprovalMode))
	auditLogger := audit.Logger(nil)
	if opts.Config.Audit.Enabled {
		auditPath := opts.Config.Audit.Path
		if auditPath == "" && home != "" {
			auditPath = audit.DefaultPath(home)
		}
		if auditPath != "" {
			auditLogger = audit.NewSlogLogger(audit.LoggerOptions{
				Path:       auditPath,
				MaxSizeMB:  opts.Config.Audit.MaxSizeMB,
				MaxBackups: opts.Config.Audit.MaxBackups,
			})
		}
	}

	provider := opts.Provider
	if provider == nil {
		var err error
		provider, err = buildProvider(opts.Config)
		if err != nil {
			return Components{}, err
		}
	}
	registeredTools, err := buildTools(cwd, home, opts.Config, p, auditLogger, provider, opts.Approver)
	if err != nil {
		return Components{}, err
	}
	loop, err := agent.NewReactLoop(agent.ReactLoopOptions{
		Provider:      provider,
		Tools:         registeredTools,
		MaxToolRounds: 8,
	})
	if err != nil {
		return Components{}, err
	}
	storeDir := session.DefaultDir(home)
	runtime, err := agent.NewRuntime(agent.RuntimeOptions{
		Loop:     loop,
		Store:    session.NewFileStore(storeDir),
		CWD:      cwd,
		Model:    opts.Config.Model,
		Approver: opts.Approver,
	})
	if err != nil {
		return Components{}, err
	}

	_ = ctx
	return Components{
		Config:  opts.Config,
		Policy:  p,
		Audit:   auditLogger,
		Tools:   registeredTools,
		Loop:    loop,
		Runtime: runtime,
	}, nil
}

func buildProvider(cfg config.Config) (llm.Provider, error) {
	if cfg.Provider != "openai" || cfg.API != "responses" {
		return nil, fmt.Errorf("unsupported provider/api %q/%q", cfg.Provider, cfg.API)
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ICOO_AI_OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ICOO_AI_API_KEY")
	}
	if apiKey == "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY, ICOO_AI_OPENAI_API_KEY, ICOO_AI_API_KEY, or config api_key is required")
	}
	return llm.NewOpenAIResponsesProvider(llm.OpenAIResponsesConfig{
		APIKey:  apiKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
		Retry: llm.RetryConfig{
			MaxAttempts:  cfg.Retry.MaxAttempts,
			InitialDelay: time.Duration(cfg.Retry.InitialDelayMillis) * time.Millisecond,
			MaxDelay:     time.Duration(cfg.Retry.MaxDelayMillis) * time.Millisecond,
		},
	})
}

func buildTools(cwd, home string, cfg config.Config, p policy.Policy, auditLogger audit.Logger, provider llm.Provider, approver agent.Approver) ([]tools.Tool, error) {
	var baseTools []tools.Tool
	fileTools, err := tools.NewFileTools(tools.FileToolOptions{
		WorkspaceRoot: cwd,
		Policy:        p,
	})
	if err != nil {
		return nil, err
	}
	baseTools = append(baseTools, fileTools...)
	timeout := time.Duration(cfg.ShellTimeoutSeconds) * time.Second
	baseTools = append(baseTools, tools.NewShellTool(tools.ShellToolOptions{
		WorkspaceRoot:  cwd,
		DefaultTimeout: timeout,
		Policy:         p,
	}))
	baseTools = append(baseTools, tools.NewGitTools(tools.GitToolOptions{
		WorkspaceRoot:  cwd,
		DefaultTimeout: timeout,
		Policy:         p,
	})...)
	baseTools = append(baseTools, tools.NewWebSearchTool(tools.WebSearchOptions{
		Policy:      p,
		AuditLogger: auditLogger,
	}))
	baseTools = append(baseTools, tools.NewWebFetchTool(tools.WebFetchOptions{
		Policy:      p,
		AuditLogger: auditLogger,
	}))
	mcpTools, err := tools.NewMCPTools(context.Background(), tools.MCPToolOptions{
		Config:      cfg.MCP,
		Factory:     mcp.Mark3LabsClientFactory{},
		Policy:      p,
		AuditLogger: auditLogger,
	})
	if err != nil {
		return nil, err
	}
	baseTools = append(baseTools, mcpTools...)

	registered := append([]tools.Tool(nil), baseTools...)
	if provider != nil {
		runner, err := subagent.NewLocalRunner(subagent.LocalRunnerOptions{
			Provider:      provider,
			Tools:         baseTools,
			Model:         cfg.Model,
			MaxToolRounds: 6,
			Approver:      approver,
		})
		if err != nil {
			return nil, err
		}
		registered = append(registered, subagent.NewTool(subagent.ToolOptions{
			Runner:      runner,
			CWD:         cwd,
			Model:       cfg.Model,
			AuditLogger: auditLogger,
		}))
		sources := skills.DefaultSources(skills.SourceOptions{
			HomeDir:    home,
			ProjectDir: cwd,
			CustomDirs: cfg.Skills.Paths,
		})
		registered = append(registered, skilltools.NewTools(skilltools.Options{
			Sources:       sources,
			WorkspaceRoot: cwd,
			CWD:           cwd,
			Model:         cfg.Model,
			Policy:        p,
			Runner:        runner,
			Approver:      approver,
			AuditLogger:   auditLogger,
		})...)
	}
	return registered, nil
}

func NewACPServer(ctx context.Context, opts BuildOptions) (*acp.Server, error) {
	components, err := Build(ctx, opts)
	if err != nil {
		return nil, err
	}
	input := opts.Stdin
	if input == nil {
		input = os.Stdin
	}
	output := opts.Stdout
	if output == nil {
		output = os.Stdout
	}
	return acp.NewServer(acp.ServerOptions{
		Runtime: components.Runtime,
		Input:   input,
		Output:  output,
		Name:    "icoo-ai",
		Version: "0.1.0",
	})
}

func DefaultConfigPath(home string) string {
	return filepath.Join(home, ".icoo-ai", "config.toml")
}
