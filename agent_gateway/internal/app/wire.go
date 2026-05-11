package app

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/handlers"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
	crudservice "github.com/icoo-ai/icoo-ai/agent_gateway/internal/services"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

var defaultManagedAgents = []service.AgentProfile{
	{
		ID:          "icoo-ai-acp",
		Name:        "Icoo AI",
		Protocol:    "icoo_acp",
		Models:      []string{"gpt-5.4"},
		Description: "Icoo ACP agent profile.",
	},
	{
		ID:          "agent-acp",
		Name:        "Agent ACP",
		Protocol:    "agent_acp",
		Models:      []string{"gpt-5.4"},
		Description: "Generic ACP agent profile.",
	},
}

type BuildOptions struct {
	Config   config.Config
	Token    string
	Now      time.Time
	EventBus *events.Bus
}

type Components struct {
	Config            config.Config
	Token             string
	StartedAt         time.Time
	EventBus          *events.Bus
	ConversationStore store.Store
	GatewayCore       service.GatewayService
	Gateway           crudservice.GatewayCRUD
	HealthHandler     http.Handler
	Router            http.Handler

	closeFn func() error
}

func (c *Components) Close() error {
	if c == nil || c.closeFn == nil {
		return nil
	}
	return c.closeFn()
}

func Build(ctx context.Context, opts BuildOptions) (Components, error) {
	_ = ctx

	cfg := opts.Config
	if cfg.Version == "" {
		cfg.Version = config.Version
	}
	if err := cfg.Validate(); err != nil {
		return Components{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	token := strings.TrimSpace(opts.Token)
	if token == "" {
		return Components{}, fmt.Errorf("token is required")
	}

	eventBus := opts.EventBus
	if eventBus == nil {
		eventBus = events.DefaultBus()
	}

	conversationStore := store.NewMemoryStore()
	settingsStore, err := service.NewSQLiteManagementSettingsStore(filepath.Join(cfg.DataDir, "management.db"))
	if err != nil {
		return Components{}, err
	}

	core := service.NewGatewayServiceWithAgentsStoreAndSettingsStore(defaultManagedAgents, conversationStore, settingsStore)
	crud := crudservice.NewGateway(core)

	components := Components{
		Config:            cfg,
		Token:             token,
		StartedAt:         now,
		EventBus:          eventBus,
		ConversationStore: conversationStore,
		GatewayCore:       core,
		Gateway:           crud,
		HealthHandler:     handlers.HealthHandler(cfg.Version, now),
		Router:            handlers.NewRouter(crud),
		closeFn:           core.Close,
	}
	return components, nil
}
