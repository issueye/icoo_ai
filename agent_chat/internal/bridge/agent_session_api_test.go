package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentServiceUsesAgentScopedSessionAPI(t *testing.T) {
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.Method+" "+r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "GET /v1/agents":
			writeTestGatewayResponse(t, w, map[string]any{
				"items": []map[string]any{{"id": "agent-1", "name": "Agent One", "protocol": "acp", "enabled": true}},
				"total": 1,
			})
		case "POST /v1/agents/agent-1/start":
			writeTestGatewayResponse(t, w, map[string]any{"id": "agent-1", "state": "running"})
		case "POST /v1/agents/agent-1/sessions":
			writeTestGatewayResponse(t, w, map[string]any{"sessionId": "session-1"})
		case "POST /v1/agents/agent-1/sessions/session-1/prompts":
			writeTestGatewayResponse(t, w, map[string]any{"stopReason": "end_turn"})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	service := &AgentService{
		messages:       []MessageEvent{},
		conversations:  []Conversation{},
		activeSessions: map[string]struct{}{},
		sessionAgents:  map[string]string{},
		gateway:        &gatewayProxy{client: server.Client(), baseURL: server.URL, token: "token"},
	}

	conversation, err := service.NewSession(context.Background(), NewSessionRequest{Title: "Chat", Cwd: "E:/codes/icoo_ai", Mode: "agent-1"})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if conversation.ID != "session-1" {
		t.Fatalf("conversation.ID = %q", conversation.ID)
	}

	events, err := service.Prompt(context.Background(), PromptRequest{SessionID: "session-1", Content: "hello"})
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if len(events) != 2 || events[0].Role != "user" {
		t.Fatalf("events = %#v", events)
	}
	if seen["POST /v1/agents/agent-1/sessions/session-1/prompts"] != 1 {
		t.Fatalf("prompt endpoint calls = %d", seen["POST /v1/agents/agent-1/sessions/session-1/prompts"])
	}
}

func writeTestGatewayResponse(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(map[string]any{"code": "ok", "data": data}); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
