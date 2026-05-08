package main

import (
	"context"
	"fmt"
	"os"
	"strings"

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
		fmt.Println("icoo-ai: use `serve`, `run`, `config`, or `doctor`")
		return nil
	}

	switch args[0] {
	case "serve":
		fmt.Println("icoo-ai serve is not implemented yet")
	case "run":
		fmt.Println("icoo-ai run is not implemented yet")
	case "config":
		return printConfig()
	case "doctor":
		return doctor(context.Background())
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}

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
