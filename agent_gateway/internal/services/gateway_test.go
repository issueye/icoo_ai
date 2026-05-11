package services

import (
	"context"
	"errors"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/models"
	legacy "github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestReplaceManagementSettingsAffectsListAgents(t *testing.T) {
	gateway := NewGateway(legacy.NewGatewayServiceWithStore(store.NewMemoryStore()))
	ctx := context.Background()

	updated, err := gateway.ReplaceManagementSettings(ctx, models.ManagementSettings{
		Agents: []models.AgentConfig{
			{ID: "a-enabled", Name: "Enabled Agent", Protocol: "acp", Models: []string{"gpt-5.4", ""}, Enabled: true},
			{ID: "a-disabled", Name: "Disabled Agent", Protocol: "acp", Models: []string{"gpt-5.4"}, Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceManagementSettings() error = %v", err)
	}
	if len(updated.Agents) != 2 {
		t.Fatalf("updated.Agents length = %d, want 2", len(updated.Agents))
	}

	agents, err := gateway.ListAgents(ctx)
	if err != nil {
		t.Fatalf("ListAgents() error = %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("ListAgents() length = %d, want 1 enabled agent", len(agents))
	}
	if agents[0].ID != "a-enabled" {
		t.Fatalf("agents[0].ID = %q, want a-enabled", agents[0].ID)
	}
}

func TestCreateSessionMessageMapsConnectorUnavailableError(t *testing.T) {
	gateway := NewGateway(legacy.NewGatewayServiceWithStore(store.NewMemoryStore()))
	ctx := context.Background()

	session, err := gateway.CreateSession(ctx, models.CreateSessionRequest{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}

	_, err = gateway.CreateSessionMessage(ctx, session.ID, models.PromptRequest{Content: "hello"})
	if err == nil {
		t.Fatal("CreateSessionMessage() error = nil, want connector_unavailable")
	}

	var gatewayErr *GatewayError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("error type = %T, want *GatewayError", err)
	}
	if gatewayErr.Code != "connector_unavailable" {
		t.Fatalf("gatewayErr.Code = %q, want connector_unavailable", gatewayErr.Code)
	}
}
