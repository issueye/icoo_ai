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
			auditLogger = audit.NewJSONLLogger(auditPath)
		}
	}

	registeredTools, err := buildTools(cwd, opts.Config, p, auditLogger)
	if err != nil {
		return Components{}, err
	}
	provider := opts.Provider
	if provider == nil {
		var err error
		provider, err = buildProvider(opts.Config)
		if err != nil {
			return Components{}, err
		}
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
		Loop:  loop,
		Store: session.NewFileStore(storeDir),
		CWD:   cwd,
		Model: opts.Config.Model,
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
		return nil, fmt.Errorf("OPENAI_API_KEY or ICOO_AI_OPENAI_API_KEY is required")
	}
	return llm.NewOpenAIResponsesProvider(llm.OpenAIResponsesConfig{
		APIKey:  apiKey,
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
}

func buildTools(cwd string, cfg config.Config, p policy.Policy, auditLogger audit.Logger) ([]tools.Tool, error) {
	var registered []tools.Tool
	fileTools, err := tools.NewFileTools(tools.FileToolOptions{
		WorkspaceRoot: cwd,
		Policy:        p,
	})
	if err != nil {
		return nil, err
	}
	registered = append(registered, fileTools...)
	timeout := time.Duration(cfg.ShellTimeoutSeconds) * time.Second
	registered = append(registered, tools.NewShellTool(tools.ShellToolOptions{
		WorkspaceRoot:  cwd,
		DefaultTimeout: timeout,
		Policy:         p,
	}))
	registered = append(registered, tools.NewGitTools(tools.GitToolOptions{
		WorkspaceRoot:  cwd,
		DefaultTimeout: timeout,
		Policy:         p,
	})...)
	registered = append(registered, tools.NewWebSearchTool(tools.WebSearchOptions{
		Policy:      p,
		AuditLogger: auditLogger,
	}))
	registered = append(registered, tools.NewWebFetchTool(tools.WebFetchOptions{
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
	registered = append(registered, mcpTools...)
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
