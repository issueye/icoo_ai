package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	gwruntime "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, once, err := parseConfigFromFlags(args)
	if err != nil {
		return err
	}

	server, err := gwruntime.NewServer(cfg)
	if err != nil {
		return err
	}
	if err := server.Start(); err != nil {
		return err
	}
	endpoint := server.Endpoint()
	fmt.Fprintf(os.Stderr, "agent-gateway listening on %s\n", endpoint.BaseURL)

	if once {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func parseConfigFromFlags(args []string) (config.Config, bool, error) {
	cfg, err := config.LoadFile(config.DefaultConfigPath)
	if err != nil {
		return config.Config{}, false, fmt.Errorf("load config file %q: %w", config.DefaultConfigPath, err)
	}
	var once bool

	flags := flag.NewFlagSet("agent-gateway", flag.ContinueOnError)
	flags.StringVar(&cfg.Host, "host", cfg.Host, "loopback host to bind")
	flags.IntVar(&cfg.Port, "port", cfg.Port, "port to bind, 0 chooses a random port")
	flags.BoolVar(&once, "once", false, "start, write endpoint files, then stop")
	if err := flags.Parse(args); err != nil {
		return config.Config{}, false, err
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, false, err
	}
	return cfg, once, nil
}
