package acp

import (
	"context"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

func TestClientSessionUpdatePublishesGatewayEvent(t *testing.T) {
	bus := events.NewBus(8)
	sub, _ := bus.Subscribe(context.Background(), "")
	defer sub.Close()

	client := NewClient("agent-1", nil, bus, nil)
	err := client.SessionUpdate(context.Background(), acpsdk.SessionNotification{
		SessionId: acpsdk.SessionId("session-1"),
		Update:    acpsdk.UpdateAgentMessageText("hello"),
	})
	if err != nil {
		t.Fatalf("SessionUpdate() error = %v", err)
	}

	select {
	case event := <-sub.Events():
		if event.Type != "acp.session_update" {
			t.Fatalf("event.Type = %q, want acp.session_update", event.Type)
		}
		if event.AgentID != "agent-1" || event.SessionID != "session-1" {
			t.Fatalf("event agent/session = %q/%q", event.AgentID, event.SessionID)
		}
	case <-time.After(time.Second):
		t.Fatal("event was not published")
	}
}
