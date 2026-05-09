package bridge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewSession_UsesGatewayWhenAvailable(t *testing.T) {
	t.Parallel()

	expected := Conversation{
		ID:        "sess_gateway_1",
		Type:      "main",
		Title:     "from gateway",
		Subtitle:  "ok",
		Status:    "idle",
		UpdatedAt: time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/sessions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
		token:   "token",
	}
	svc.devFallback = true

	got, err := svc.NewSession(context.Background(), NewSessionRequest{Title: "new title"})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if got.ID != expected.ID || got.Title != expected.Title {
		t.Fatalf("unexpected conversation: %#v", got)
	}
}

func TestNewSession_FallbackInDevWhenGatewayUnavailable(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  &http.Client{Timeout: 100 * time.Millisecond},
		baseURL: "http://127.0.0.1:1",
	}
	svc.devFallback = true

	got, err := svc.NewSession(context.Background(), NewSessionRequest{Title: "dev fallback"})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if !strings.HasPrefix(got.ID, "sess_mock_") {
		t.Fatalf("expected mock session id, got: %s", got.ID)
	}
}

func TestNewSession_NoFallbackInProdWhenGatewayUnavailable(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  &http.Client{Timeout: 100 * time.Millisecond},
		baseURL: "http://127.0.0.1:1",
	}
	svc.devFallback = false

	_, err := svc.NewSession(context.Background(), NewSessionRequest{Title: "prod no fallback"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	bridgeErr, ok := err.(*BridgeError)
	if !ok {
		t.Fatalf("expected *BridgeError, got %T", err)
	}
	if bridgeErr.Code != ErrorCodeGatewayUnavailable {
		t.Fatalf("expected %s, got %s", ErrorCodeGatewayUnavailable, bridgeErr.Code)
	}
}

func TestNewSession_ReturnsAuthErrorOn401(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
		token:   "bad-token",
	}
	svc.devFallback = true

	_, err := svc.NewSession(context.Background(), NewSessionRequest{Title: "auth fail"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	bridgeErr, ok := err.(*BridgeError)
	if !ok {
		t.Fatalf("expected *BridgeError, got %T", err)
	}
	if bridgeErr.Code != ErrorCodeGatewayAuthFailed {
		t.Fatalf("expected %s, got %s", ErrorCodeGatewayAuthFailed, bridgeErr.Code)
	}
	if bridgeErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", bridgeErr.StatusCode)
	}
}

func TestStreamGatewayEvents_ForwardsEventAndUpdatesLastEventID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events/stream" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		payload := `{"id":"evt_1","sessionId":"sess_1","kind":"message","role":"assistant","content":"hello"}`
		_, _ = w.Write([]byte("data: {\"id\":\"evt_1\",\"type\":\"message\",\"sessionId\":\"sess_1\",\"payload\":" + payload + "}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
	}
	received := make(chan MessageEvent, 1)
	svc.eventSink = func(event MessageEvent) {
		select {
		case received <- event:
		default:
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.streamGatewayEvents(ctx)

	select {
	case evt := <-received:
		if evt.ID != "evt_1" || evt.SessionID != "sess_1" || evt.Content != "hello" {
			t.Fatalf("unexpected forwarded event: %#v", evt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for forwarded event")
	}
	cancel()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if svc.lastStreamEventID() == "evt_1" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected last event id to be evt_1, got %q", svc.lastStreamEventID())
}

func TestMapEnvelopeToMessageEvent_MapsGatewayTypeToStableKind(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	envelope := GatewayEventEnvelope{
		ID:        "evt_tool_result_1",
		Type:      "tool.result",
		SessionID: "sess_1",
		Payload:   json.RawMessage(`{"content":"done"}`),
		CreatedAt: time.Date(2026, 5, 9, 12, 1, 0, 0, time.UTC),
	}

	event := svc.mapEnvelopeToMessageEvent(envelope)
	if event.Kind != "tool_result" {
		t.Fatalf("expected kind tool_result, got %q", event.Kind)
	}
	if event.ID != "evt_tool_result_1" {
		t.Fatalf("expected id from envelope, got %q", event.ID)
	}
	if event.SessionID != "sess_1" {
		t.Fatalf("expected session id from envelope, got %q", event.SessionID)
	}
}

func TestMapEnvelopeToMessageEvent_UnknownTypeFallsBackWithoutLosingType(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	envelope := GatewayEventEnvelope{
		ID:        "evt_unknown_1",
		Type:      "runtime.custom.delta",
		SessionID: "sess_2",
		Payload:   json.RawMessage(`{"kind":"runtime.custom.delta","content":"partial"}`),
	}

	event := svc.mapEnvelopeToMessageEvent(envelope)
	if event.Kind != "gateway_event" {
		t.Fatalf("expected fallback kind gateway_event, got %q", event.Kind)
	}
	if event.SafeMeta == nil {
		t.Fatal("expected safeMeta to include gateway type")
	}
	if got := event.SafeMeta["gatewayType"]; got != "runtime.custom.delta" {
		t.Fatalf("expected safeMeta.gatewayType=runtime.custom.delta, got %#v", got)
	}
	if got := event.SafeMeta["gatewayKind"]; got != "runtime.custom.delta" {
		t.Fatalf("expected safeMeta.gatewayKind=runtime.custom.delta, got %#v", got)
	}
	if event.Content != "partial" {
		t.Fatalf("expected payload content preserved, got %q", event.Content)
	}
}
