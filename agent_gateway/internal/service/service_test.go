package service

import (
	"context"
	"errors"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestUpdateManagementSettingsAffectsListAgents(t *testing.T) {
	svc := NewGatewayServiceWithStore(store.NewMemoryStore())
	ctx := context.Background()

	updated, err := svc.UpdateManagementSettings(ctx, ManagementSettings{
		Agents: []AgentConfig{
			{ID: "a-enabled", Name: "Enabled Agent", Protocol: "acp", Models: []string{"gpt-5.4", ""}, Enabled: true},
			{ID: "a-disabled", Name: "Disabled Agent", Protocol: "acp", Models: []string{"gpt-5.4"}, Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("UpdateManagementSettings() error = %v", err)
	}
	if len(updated.Agents) != 2 {
		t.Fatalf("updated.Agents length = %d, want 2", len(updated.Agents))
	}

	agents, err := svc.ListAgents(ctx)
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

func TestCreateSessionDefaultsToFirstEnabledAgent(t *testing.T) {
	svc := NewGatewayServiceWithStore(store.NewMemoryStore())
	ctx := context.Background()
	_, err := svc.UpdateManagementSettings(ctx, ManagementSettings{
		Agents: []AgentConfig{
			{ID: "primary-agent", Name: "Primary", Protocol: "acp", Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("UpdateManagementSettings() error = %v", err)
	}

	session, err := svc.CreateSession(ctx, CreateSessionRequest{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if session.AgentID != "primary-agent" {
		t.Fatalf("session.AgentID = %q, want primary-agent", session.AgentID)
	}
	if session.Mode != "primary-agent" {
		t.Fatalf("session.Mode = %q, want primary-agent", session.Mode)
	}
	if session.Title != "New Agent Session" {
		t.Fatalf("session.Title = %q, want New Agent Session", session.Title)
	}
}

func TestPromptWithoutConnectorReturnsConnectorUnavailable(t *testing.T) {
	svc := NewGatewayServiceWithStore(store.NewMemoryStore())
	ctx := context.Background()

	session, err := svc.CreateSession(ctx, CreateSessionRequest{})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	_, err = svc.Prompt(ctx, session.ID, PromptRequest{Content: "hello"})
	if err == nil {
		t.Fatal("Prompt() error = nil, want connector_unavailable")
	}

	var serviceErr *Error
	if !errors.As(err, &serviceErr) {
		t.Fatalf("Prompt() error type = %T, want *service.Error", err)
	}
	if serviceErr.Code != "connector_unavailable" {
		t.Fatalf("serviceErr.Code = %q, want connector_unavailable", serviceErr.Code)
	}
}
