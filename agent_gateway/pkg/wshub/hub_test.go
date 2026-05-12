package wshub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type testEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	SessionID string `json:"sessionId"`
	AgentID   string `json:"agentId"`
}

type testMessage struct {
	Content string `json:"content"`
}

type testSource struct {
	buffered []any
	events   chan any
}

type testSubscription struct {
	events chan any
}

func (s testSource) Subscribe(context.Context, string) (Subscription, []any) {
	return testSubscription{events: s.events}, append([]any(nil), s.buffered...)
}

func (s testSubscription) Events() <-chan any {
	return s.events
}

func (s testSubscription) Close() {}

func TestHubWritesBufferedEventsAsJSONRPCNotifications(t *testing.T) {
	source := testSource{
		buffered: []any{
			testEvent{ID: "evt_1", Type: "message", SessionID: "sess_1", AgentID: "agent_1"},
			testEvent{ID: "evt_2", Type: "message", SessionID: "sess_2", AgentID: "agent_1"},
		},
		events: make(chan any),
	}
	hub := New(source, WithFilter(func(event any, r *http.Request) bool {
		got, ok := event.(testEvent)
		return ok && got.SessionID == r.URL.Query().Get("sessionId")
	}))
	server := newTestServer(hub)
	defer server.Close()

	conn := dialTestHub(t, server.URL+"?sessionId=sess_1")
	defer conn.Close()

	var got notification
	if err := conn.ReadJSON(&got); err != nil {
		t.Fatalf("ReadJSON() error = %v", err)
	}
	if got.JSONRPC != "2.0" || got.Method != "event" {
		t.Fatalf("notification = %#v, want JSON-RPC event notification", got)
	}

	raw, err := json.Marshal(got.Params)
	if err != nil {
		t.Fatalf("Marshal(params) error = %v", err)
	}
	var event testEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		t.Fatalf("Unmarshal(event) error = %v", err)
	}
	if event.ID != "evt_1" || event.SessionID != "sess_1" {
		t.Fatalf("event = %#v, want filtered evt_1", event)
	}
}

func TestHubDispatchesRegisteredJSONRPCRoute(t *testing.T) {
	hub := New(testSource{events: make(chan any)})
	received := make(chan testMessage, 1)
	Handle[testMessage](hub, "message block", func(ctx context.Context, data testMessage) error {
		received <- data
		return nil
	})

	server := newTestServer(hub)
	defer server.Close()

	conn := dialTestHub(t, server.URL)
	defer conn.Close()

	id := json.RawMessage(`"req_1"`)
	if err := conn.WriteJSON(request{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "message block",
		Params:  json.RawMessage(`{"content":"hello"}`),
	}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	select {
	case got := <-received:
		if got.Content != "hello" {
			t.Fatalf("handler data = %#v, want hello", got)
		}
	case <-time.After(time.Second):
		t.Fatal("handler was not called")
	}

	var resp response
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("ReadJSON(response) error = %v", err)
	}
	if resp.JSONRPC != "2.0" || resp.Error != nil || resp.ID == nil || string(*resp.ID) != `"req_1"` {
		t.Fatalf("response = %#v, want successful JSON-RPC response", resp)
	}
}

func TestHubReturnsMethodNotFound(t *testing.T) {
	hub := New(testSource{events: make(chan any)})
	server := newTestServer(hub)
	defer server.Close()

	conn := dialTestHub(t, server.URL)
	defer conn.Close()

	id := json.RawMessage(`1`)
	if err := conn.WriteJSON(request{JSONRPC: "2.0", ID: &id, Method: "missing"}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	var resp response
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatalf("ReadJSON(response) error = %v", err)
	}
	if resp.Error == nil || resp.Error.Code != errMethodNotFound {
		t.Fatalf("response = %#v, want method not found", resp)
	}
}

func newTestServer(hub *Hub) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.Serve(r.Context(), w, r)
	}))
}

func dialTestHub(t *testing.T, serverURL string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	return conn
}
