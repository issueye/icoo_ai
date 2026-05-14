package gatewayclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestStreamEventsWithFilterUsesWebSocketEvents(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events" {
			t.Fatalf("path = %q, want /v1/events", r.URL.Path)
		}
		if r.URL.Query().Get("sessionId") != "session-1" {
			t.Fatalf("sessionId query = %q", r.URL.Query().Get("sessionId"))
		}
		if r.Header.Get("Authorization") != "Bearer token-1" {
			t.Fatalf("authorization header = %q", r.Header.Get("Authorization"))
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		defer conn.Close()
		_ = conn.WriteJSON(map[string]any{
			"jsonrpc": "2.0",
			"method":  "event",
			"params": map[string]any{
				"id":        "evt-1",
				"type":      "acp.session_update",
				"sessionId": "session-1",
			},
		})
	}))
	defer server.Close()

	client := New(server.URL, "token-1")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var got StreamEnvelope
	err := client.StreamEventsWithFilter(ctx, "", "session-1", "", func(event StreamEnvelope) error {
		got = event
		cancel()
		return nil
	})
	if err != context.Canceled {
		t.Fatalf("StreamEventsWithFilter() error = %v, want context canceled", err)
	}
	if got.ID != "evt-1" || got.SessionID != "session-1" {
		t.Fatalf("event = %#v", got)
	}
}

func TestProbeEventsUsesWebSocketPath(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v1/events") {
			t.Fatalf("path = %q, want /v1/events", r.URL.Path)
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		_ = conn.Close()
	}))
	defer server.Close()

	if err := New(server.URL, "").ProbeEvents(context.Background()); err != nil {
		t.Fatalf("ProbeEvents() error = %v", err)
	}
}
