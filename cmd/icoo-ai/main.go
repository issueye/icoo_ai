package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/app"
	"github.com/icoo-ai/icoo-ai/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		fmt.Println("icoo-ai: use `serve`, `run`, `config`, `doctor`, or `migrate-claude-config`")
		return nil
	}

	switch args[0] {
	case "serve":
		return serve(context.Background())
	case "run":
		return runPrompt(context.Background(), strings.Join(args[1:], " "))
	case "config":
		return printConfig()
	case "doctor":
		return doctor(context.Background())
	case "migrate-claude-config":
		return migrateClaudeConfig(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func migrateClaudeConfig(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: icoo-ai migrate-claude-config <source.json> <dest.toml>")
	}
	return config.MigrateClaudeCodeConfig(config.ClaudeCodeMigrationOptions{
		SourcePath: args[0],
		DestPath:   args[1],
	})
}

func serve(ctx context.Context) error {
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		return err
	}
	server, err := app.NewACPServer(ctx, app.BuildOptions{
		Config: cfg,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	return server.Serve()
}

func runPrompt(ctx context.Context, prompt string) error {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		return err
	}
	components, err := app.Build(ctx, app.BuildOptions{Config: cfg})
	if err != nil {
		return err
	}
	session, err := components.Runtime.NewSession(ctx, agent.NewSessionRequest{})
	if err != nil {
		return err
	}
	events, err := components.Runtime.Prompt(ctx, agent.PromptRequest{
		SessionID: session.ID,
		Prompt:    prompt,
	})
	if err != nil {
		return err
	}
	for event := range events {
		switch event.Type {
		case agent.EventMessageDelta:
			fmt.Print(event.Content)
		case agent.EventToolCallStarted:
			if name, _ := event.Data["name"].(string); name != "" {
				fmt.Fprintf(os.Stderr, "\n[tool] %s\n", name)
			}
		case agent.EventToolCallCompleted:
			if event.Error != "" {
				fmt.Fprintf(os.Stderr, "[tool-error] %s\n", event.Error)
			}
		case agent.EventRunFailed:
			return errors.New(event.Error)
		case agent.EventRunCancelled:
			return ctx.Err()
		}
	}
	fmt.Println()
	return nil
}

func printConfig() error {
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		return err
	}
	fmt.Printf("provider=%s\n", cfg.Provider)
	fmt.Printf("api=%s\n", cfg.API)
	fmt.Printf("approval_mode=%s\n", cfg.ApprovalMode)
	fmt.Printf("agent_loop=%s\n", cfg.AgentLoop)
	fmt.Printf("web_search.provider=%s\n", cfg.WebSearch.Provider)
	return nil
}

func doctor(ctx context.Context) error {
	cfg, err := config.Load(config.LoadOptions{})
	if err != nil {
		return err
	}
	checks := []struct {
		name string
		ok   bool
		info string
	}{
		{name: "config", ok: true, info: "loaded TOML configuration"},
		{name: "provider", ok: cfg.Provider != "", info: cfg.Provider},
		{name: "api", ok: cfg.API == "responses", info: cfg.API},
		{name: "approval", ok: cfg.ApprovalMode != "", info: string(cfg.ApprovalMode)},
		{name: "openai key", ok: hasAnyEnv("OPENAI_API_KEY", "ICOO_AI_OPENAI_API_KEY"), info: redactedEnvStatus("OPENAI_API_KEY", "ICOO_AI_OPENAI_API_KEY")},
	}
	for _, check := range checks {
		status := "ok"
		if !check.ok {
			status = "warn"
		}
		fmt.Printf("%s\t%s\t%s\n", status, check.name, check.info)
	}
	return ctx.Err()
}

func hasAnyEnv(names ...string) bool {
	for _, name := range names {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

func redactedEnvStatus(names ...string) string {
	var present []string
	for _, name := range names {
		if os.Getenv(name) != "" {
			present = append(present, name+"=[set]")
		}
	}
	if len(present) == 0 {
		return "not set"
	}
	return strings.Join(present, ",")
}
