package app

import (
	"context"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
)

func TestBuildWiresGatewayComponents(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()

	components, err := Build(context.Background(), BuildOptions{
		Config: cfg,
		Token:  "fixed-test-token",
		Now:    time.Now(),
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	if components.Router == nil {
		t.Fatal("components.Router = nil, want non-nil router")
	}
	if components.HealthHandler == nil {
		t.Fatal("components.HealthHandler = nil, want non-nil health handler")
	}
	if components.ConversationStore == nil {
		t.Fatal("components.ConversationStore = nil, want non-nil store")
	}
	if components.GatewayCore == nil {
		t.Fatal("components.GatewayCore = nil, want non-nil gateway core")
	}
	if components.Gateway == nil {
		t.Fatal("components.Gateway = nil, want non-nil gateway facade")
	}

	if err := components.Close(); err != nil {
		t.Fatalf("components.Close() error = %v", err)
	}
}
