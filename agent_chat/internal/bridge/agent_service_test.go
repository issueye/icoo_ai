package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
	"github.com/wailsapp/wails/v3/pkg/application"
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

	got, err := svc.NewSession(context.Background(), NewSessionRequest{Title: "new title"})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	if got.ID != expected.ID || got.Title != expected.Title {
		t.Fatalf("unexpected conversation: %#v", got)
	}
}

func TestNewSession_ReturnsErrorWhenGatewayUnavailable(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  &http.Client{Timeout: 100 * time.Millisecond},
		baseURL: "http://127.0.0.1:1",
	}

	_, err := svc.NewSession(context.Background(), NewSessionRequest{Title: "gateway unavailable"})
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

func TestPrompt_UsesContentFieldAndMapsStructuredResponse(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/v1/sessions/sess_1/prompt" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, exists := body["prompt"]; exists {
			t.Fatalf("request unexpectedly included prompt field: %#v", body)
		}
		if got, ok := body["content"].(string); !ok || got != "hello gateway" {
			t.Fatalf("content field mismatch: %#v", body)
		}
		if got, ok := body["workspaceId"].(string); !ok || got != "workspace_main" {
			t.Fatalf("workspaceId field mismatch: %#v", body)
		}
		if got, ok := body["cwd"].(string); !ok || got != "E:/codes/icoo_ai/agent_chat" {
			t.Fatalf("cwd field mismatch: %#v", body)
		}
		if got, ok := body["mode"].(string); !ok || got != "agent.main" {
			t.Fatalf("mode field mismatch: %#v", body)
		}
		if got, ok := body["agentId"].(string); !ok || got != "agent.main" {
			t.Fatalf("agentId field mismatch: %#v", body)
		}
		if got, ok := body["model"].(string); !ok || got != "gpt-5" {
			t.Fatalf("model field mismatch: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"run": map[string]any{
				"id":        "run_1",
				"sessionId": "sess_1",
				"agentId":   "agent.main",
				"status":    "running",
				"startedAt": "2026-05-09T12:00:00Z",
			},
			"messages": []map[string]any{
				{
					"id":        "msg_1",
					"sessionId": "sess_1",
					"role":      "assistant",
					"content":   "ok",
					"createdAt": "2026-05-09T12:00:01Z",
				},
			},
			"approval": map[string]any{
				"id":        "appr_1",
				"sessionId": "sess_1",
				"status":    "pending",
				"decision":  "pending",
				"message":   "need approval",
				"createdAt": "2026-05-09T12:00:02Z",
			},
		})
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	events, err := svc.Prompt(context.Background(), PromptRequest{
		SessionID:   "sess_1",
		Content:     "hello gateway",
		Cwd:         "E:/codes/icoo_ai/agent_chat",
		WorkspaceID: "workspace_main",
		Mode:        "agent.main",
		Model:       "gpt-5",
	})
	if err != nil {
		t.Fatalf("Prompt returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Kind != BridgeEventKindMessage || events[0].Content != "ok" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].Kind != BridgeEventKindApproval || events[1].Decision != "pending" {
		t.Fatalf("unexpected approval event: %#v", events[1])
	}
}

func TestNewSession_ForwardsWorkspaceModeModelAndAgent(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/sessions" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got := body["workspaceId"]; got != "workspace_a" {
			t.Fatalf("workspaceId mismatch: %#v", body)
		}
		if got := body["cwd"]; got != "E:/codes/icoo_ai" {
			t.Fatalf("cwd mismatch: %#v", body)
		}
		if got := body["mode"]; got != "agent.main" {
			t.Fatalf("mode mismatch: %#v", body)
		}
		if got := body["agentId"]; got != "agent.main" {
			t.Fatalf("agentId mismatch: %#v", body)
		}
		if got := body["model"]; got != "gpt-5" {
			t.Fatalf("model mismatch: %#v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":        "sess_1",
			"title":     "demo",
			"agentId":   "agent.main",
			"status":    "idle",
			"createdAt": "2026-05-10T00:00:00Z",
			"updatedAt": "2026-05-10T00:00:01Z",
		})
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	_, err := svc.NewSession(context.Background(), NewSessionRequest{
		Title:       "demo",
		Cwd:         "E:/codes/icoo_ai",
		WorkspaceID: "workspace_a",
		Mode:        "agent.main",
		Model:       "gpt-5",
	})
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
}

func TestListRuns_MapsEndedAtToCompletedAt(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/runs" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"run_1","sessionId":"sess_1","agentId":"agent.main","status":"completed","startedAt":"2026-05-09T12:00:00Z","endedAt":"2026-05-09T12:00:10Z"}
		]`))
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	runs, err := svc.ListRuns(context.Background())
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].CompletedAt == nil {
		t.Fatalf("expected completedAt mapped from endedAt, got %#v", runs[0])
	}
	if runs[0].Label != "已完成" {
		t.Fatalf("expected status label 已完成, got %q", runs[0].Label)
	}
}

func TestListSkills_MapsGatewaySkillDTO(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/skills" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"skill_1","name":"security-auditor","description":"review policies"}]`))
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	skills, err := svc.ListSkills(context.Background())
	if err != nil {
		t.Fatalf("ListSkills returned error: %v", err)
	}
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}
	if skills[0].Name != "security-auditor" || skills[0].Description != "review policies" {
		t.Fatalf("unexpected skill mapping: %#v", skills[0])
	}
}

func TestListAgents_MapsGatewayAgentDTO(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/agents" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"icoo-ai-acp","name":"Icoo AI","protocol":"acp","models":["gpt-5.4"],"description":"Default ACP connector profile."}]`))
	}))
	defer srv.Close()

	svc := NewAgentService()
	svc.gateway = &gatewayProxy{
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	agents, err := svc.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents returned error: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].ID != "icoo-ai-acp" || agents[0].Protocol != "acp" {
		t.Fatalf("unexpected agent mapping: %#v", agents[0])
	}
	if len(agents[0].Models) != 1 || agents[0].Models[0] != "gpt-5.4" {
		t.Fatalf("unexpected agent models: %#v", agents[0].Models)
	}
}

func TestStreamGatewayEvents_ForwardsEventAndUpdatesLastEventID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events/stream" {
			http.NotFound(w, r)
			return
		}
		if got := r.URL.Query().Get("sessionId"); got != "sess_1" {
			t.Fatalf("expected sessionId query sess_1, got %q", got)
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
	svc.setCurrentStreamSessionID("sess_1")

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

func TestMapMessageToAuditEvent_NormalizesLevelAndSummary(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, 5, 10, 10, 30, 0, 0, time.UTC)
	audit := mapMessageToAuditEvent(
		GatewayEventEnvelope{
			Type: "audit.event",
		},
		MessageEvent{
			ID:        "evt_audit_1",
			SessionID: "sess_1",
			Status:    "warning",
			Summary:   "",
			Content:   "fallback summary",
			CreatedAt: createdAt,
			SafeMeta: map[string]any{
				"severity": "error",
			},
		},
	)

	if audit.Level != "warn" {
		t.Fatalf("expected normalized warning->warn level, got %q", audit.Level)
	}
	if audit.Summary != "fallback summary" {
		t.Fatalf("expected content fallback summary, got %q", audit.Summary)
	}
	if audit.Type != "audit.event" {
		t.Fatalf("expected audit type from envelope, got %q", audit.Type)
	}
	if audit.CreatedAt != createdAt {
		t.Fatalf("expected createdAt from message, got %v", audit.CreatedAt)
	}
}

func TestForwardGatewayEvent_TrimsAuditEventCache(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.eventSink = nil
	baseTime := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)

	for index := 0; index < maxAuditEventCacheSize+2; index++ {
		envelopeID := fmt.Sprintf("evt_audit_%d", index)
		payload := json.RawMessage(fmt.Sprintf("{\"id\":\"%s\",\"kind\":\"audit\",\"summary\":\"audit-%d\",\"status\":\"info\"}", envelopeID, index))
		err := svc.forwardGatewayEvent(gatewayclient.StreamEnvelope{
			ID:        envelopeID,
			Type:      "audit",
			SessionID: "sess_1",
			Payload:   payload,
			CreatedAt: baseTime.Add(time.Duration(index) * time.Second).Format(time.RFC3339),
		})
		if err != nil {
			t.Fatalf("forwardGatewayEvent returned error at index %d: %v", index, err)
		}
	}

	if len(svc.auditEvents) != maxAuditEventCacheSize {
		t.Fatalf("expected cached audit events %d, got %d", maxAuditEventCacheSize, len(svc.auditEvents))
	}
	if svc.auditEvents[0].ID != "evt_audit_2" {
		t.Fatalf("expected oldest cached audit id evt_audit_2, got %q", svc.auditEvents[0].ID)
	}
}

func TestServiceStartup_ReturnsBootstrapErrorInProdMode(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.gateway = nil
	svc.bootstrap = &gatewayBootstrapper{
		discoveryPath: "",
		waitTimeout:   time.Millisecond,
		pollInterval:  time.Millisecond,
		now:           time.Now,
		sleep:         func(time.Duration) {},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			return gatewayclient.Endpoint{}, "", errors.New("not found")
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			return nil, errors.New("start failed")
		},
	}

	err := svc.ServiceStartup(context.Background(), application.ServiceOptions{})
	if err == nil {
		t.Fatal("expected startup error, got nil")
	}
	bridgeErr, ok := err.(*BridgeError)
	if !ok {
		t.Fatalf("expected *BridgeError, got %T", err)
	}
	if bridgeErr.Code != ErrorCodeGatewayBootstrap {
		t.Fatalf("expected %s, got %s", ErrorCodeGatewayBootstrap, bridgeErr.Code)
	}
}

func TestServiceStartup_EmitsGatewayFailedStatusOnBootstrapError(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.gateway = nil
	var captured []MessageEvent
	svc.eventSink = func(event MessageEvent) {
		captured = append(captured, event)
	}
	svc.bootstrap = &gatewayBootstrapper{
		discoveryPath: "",
		waitTimeout:   time.Millisecond,
		pollInterval:  time.Millisecond,
		now:           time.Now,
		sleep:         func(time.Duration) {},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			return gatewayclient.Endpoint{}, "", errors.New("not found")
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			return nil, errors.New("start failed")
		},
	}

	if err := svc.ServiceStartup(context.Background(), application.ServiceOptions{}); err == nil {
		t.Fatal("expected startup error, got nil")
	}

	if len(captured) < 2 {
		t.Fatalf("expected at least 2 status events, got %d", len(captured))
	}
	if captured[0].Status != GatewayStatusConnecting {
		t.Fatalf("expected first status %q, got %q", GatewayStatusConnecting, captured[0].Status)
	}
	if captured[len(captured)-1].Status != GatewayStatusFailed {
		t.Fatalf("expected last status %q, got %q", GatewayStatusFailed, captured[len(captured)-1].Status)
	}
}

func TestStreamGatewayEvents_EmitsFailedStatusOnAuthError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/events/stream" {
			http.NotFound(w, r)
			return
		}
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

	statuses := make(chan string, 4)
	svc.eventSink = func(event MessageEvent) {
		if event.Kind == BridgeEventKindGateway && event.Status != "" {
			select {
			case statuses <- event.Status:
			default:
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go svc.streamGatewayEvents(ctx)

	deadline := time.After(2 * time.Second)
	for {
		select {
		case status := <-statuses:
			if status == GatewayStatusFailed {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for gateway_failed status")
		}
	}
}

func TestParseGatewayPromptResponse_StructuredShape(t *testing.T) {
	t.Parallel()

	raw := json.RawMessage(`{
		"run":{"id":"run_1"},
		"messages":[{"id":"msg_1","sessionId":"sess_1","role":"assistant","content":"ok","createdAt":"2026-05-09T12:00:00Z"}],
		"approval":{"id":"appr_1","sessionId":"sess_1","status":"pending","decision":"pending","message":"need approval","createdAt":"2026-05-09T12:00:01Z"}
	}`)

	events, err := parseGatewayPromptResponse(raw, "sess_1")
	if err != nil {
		t.Fatalf("parseGatewayPromptResponse returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events (message+approval), got %d", len(events))
	}
	if events[0].ID != "msg_1" || events[1].Kind != BridgeEventKindApproval {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestNormalizeMessageEvents_DefaultsKindToMessage(t *testing.T) {
	t.Parallel()

	in := []MessageEvent{
		{ID: "msg_1", Role: "assistant", Content: "hello"},
	}
	out := normalizeMessageEvents(in, "sess_1")
	if len(out) != 1 {
		t.Fatalf("unexpected len: %d", len(out))
	}
	if out[0].SessionID != "sess_1" {
		t.Fatalf("expected session fallback, got %q", out[0].SessionID)
	}
	if out[0].Kind != BridgeEventKindMessage {
		t.Fatalf("expected kind message, got %q", out[0].Kind)
	}
}

func TestServiceShutdown_StopsManagedGatewayProcess(t *testing.T) {
	t.Parallel()

	stopped := false
	process := &os.Process{Pid: 12345}
	svc := NewAgentService()
	svc.bootstrap = &gatewayBootstrapper{
		stopProcess: func(p *os.Process) error {
			if p != process {
				t.Fatalf("unexpected process pointer: %#v", p)
			}
			stopped = true
			return nil
		},
		managedProcess: process,
		managedOwned:   true,
	}

	if err := svc.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown returned error: %v", err)
	}
	if !stopped {
		t.Fatal("expected managed gateway process to be stopped")
	}
}

func TestServiceShutdown_DoesNotStopWhenNoManagedProcess(t *testing.T) {
	t.Parallel()

	stopCalls := 0
	svc := NewAgentService()
	svc.bootstrap = &gatewayBootstrapper{
		stopProcess: func(*os.Process) error {
			stopCalls++
			return nil
		},
	}

	if err := svc.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown returned error: %v", err)
	}
	if stopCalls != 0 {
		t.Fatalf("expected stop calls = 0, got %d", stopCalls)
	}
}

func TestRestartGateway_StopsManagedProcessAndReconnects(t *testing.T) {
	t.Parallel()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	managedProcess := &os.Process{Pid: 4242}
	stopCalled := false
	svc := NewAgentService()
	svc.serviceCtx = cancelledCtx
	svc.gateway = &gatewayProxy{baseURL: "http://127.0.0.1:50001", token: "old"}
	svc.probeEventStream = func(context.Context, *gatewayProxy) error {
		return nil
	}
	svc.bootstrap = &gatewayBootstrapper{
		managedProcess: managedProcess,
		managedOwned:   true,
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			return gatewayclient.Endpoint{BaseURL: "http://127.0.0.1:60001", PID: 9999}, "new-token", nil
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			return nil, nil
		},
		stopProcess: func(p *os.Process) error {
			if p != managedProcess {
				t.Fatalf("unexpected process to stop: %#v", p)
			}
			stopCalled = true
			return nil
		},
		stopProcessByPID: func(pid int) error {
			if pid != managedProcess.Pid {
				t.Fatalf("unexpected pid to stop: %d", pid)
			}
			stopCalled = true
			return nil
		},
	}

	status, err := svc.RestartGateway(context.Background())
	if err != nil {
		t.Fatalf("RestartGateway returned error: %v", err)
	}
	if !stopCalled {
		t.Fatal("expected managed gateway process to be stopped")
	}
	if svc.gateway == nil {
		t.Fatal("expected gateway proxy to be reloaded")
	}
	if svc.gateway.baseURL != "http://127.0.0.1:60001" {
		t.Fatalf("unexpected gateway baseURL: %s", svc.gateway.baseURL)
	}
	if svc.gateway.token != "new-token" {
		t.Fatalf("unexpected gateway token: %s", svc.gateway.token)
	}
	if status.Status != GatewayStatusReady {
		t.Fatalf("expected status %q, got %q", GatewayStatusReady, status.Status)
	}
}

func TestRestartGateway_ContinuesWhenStopManagedProcessFails(t *testing.T) {
	t.Parallel()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := NewAgentService()
	svc.serviceCtx = cancelledCtx
	svc.gateway = &gatewayProxy{baseURL: "http://127.0.0.1:50001", token: "old"}
	svc.probeEventStream = func(context.Context, *gatewayProxy) error {
		return nil
	}
	svc.bootstrap = &gatewayBootstrapper{
		managedProcess: &os.Process{Pid: 1234},
		managedOwned:   true,
		stopProcess: func(*os.Process) error {
			return errors.New("access denied")
		},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			return gatewayclient.Endpoint{BaseURL: "http://127.0.0.1:60001", PID: 9999}, "new-token", nil
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			return nil, nil
		},
	}

	status, err := svc.RestartGateway(context.Background())
	if err != nil {
		t.Fatalf("RestartGateway returned error: %v", err)
	}
	if svc.gateway == nil {
		t.Fatal("expected gateway proxy to be reloaded")
	}
	if svc.gateway.baseURL != "http://127.0.0.1:60001" {
		t.Fatalf("unexpected gateway baseURL: %s", svc.gateway.baseURL)
	}
	if status.Status != GatewayStatusReady {
		t.Fatalf("expected status %q, got %q", GatewayStatusReady, status.Status)
	}
}

func TestRestartGateway_ReturnsReconnectingWhenStreamProbeFails(t *testing.T) {
	t.Parallel()

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	svc := NewAgentService()
	svc.serviceCtx = cancelledCtx
	svc.gateway = &gatewayProxy{baseURL: "http://127.0.0.1:50001", token: "old"}
	svc.probeEventStream = func(context.Context, *gatewayProxy) error {
		return errors.New("dial tcp 127.0.0.1:17889: connectex: target machine actively refused it")
	}
	svc.bootstrap = &gatewayBootstrapper{
		managedProcess: &os.Process{Pid: 2233},
		managedOwned:   true,
		stopProcess: func(*os.Process) error {
			return nil
		},
		discover: func(string) (gatewayclient.Endpoint, string, error) {
			return gatewayclient.Endpoint{BaseURL: "http://127.0.0.1:60001", PID: 9999}, "new-token", nil
		},
		healthCheck: func(context.Context, gatewayclient.Endpoint, string) error {
			return nil
		},
		startProcess: func(context.Context) (*os.Process, error) {
			return nil, nil
		},
	}

	status, err := svc.RestartGateway(context.Background())
	if err != nil {
		t.Fatalf("RestartGateway returned error: %v", err)
	}
	if status.Status != GatewayStatusReconnecting {
		t.Fatalf("expected status %q, got %q", GatewayStatusReconnecting, status.Status)
	}
	if status.Summary != "网关已重启，事件流连接中" {
		t.Fatalf("expected reconnecting summary, got %q", status.Summary)
	}
}

func TestResolveAgentIDFromMode_IgnoresGenericMode(t *testing.T) {
	t.Parallel()

	if got := resolveAgentIDFromMode("agent"); got != "" {
		t.Fatalf("expected empty agent id for mode agent, got %q", got)
	}
	if got := resolveAgentIDFromMode("default"); got != "" {
		t.Fatalf("expected empty agent id for mode default, got %q", got)
	}
	if got := resolveAgentIDFromMode("main"); got != "" {
		t.Fatalf("expected empty agent id for mode main, got %q", got)
	}
	if got := resolveAgentIDFromMode("agent.main"); got != "agent.main" {
		t.Fatalf("expected passthrough for concrete agent mode, got %q", got)
	}
}

func TestStopGateway_StopsManagedProcessAndUpdatesStatus(t *testing.T) {
	t.Parallel()

	stopCalled := false
	svc := NewAgentService()
	svc.gateway = &gatewayProxy{baseURL: "http://127.0.0.1:50001", token: "old"}
	svc.bootstrap = &gatewayBootstrapper{
		managedProcess: &os.Process{Pid: 1234},
		managedOwned:   true,
		stopProcess: func(*os.Process) error {
			stopCalled = true
			return nil
		},
	}

	status, err := svc.StopGateway(context.Background())
	if err != nil {
		t.Fatalf("StopGateway returned error: %v", err)
	}
	if !stopCalled {
		t.Fatal("expected managed gateway process to be stopped")
	}
	if svc.gateway != nil {
		t.Fatal("expected gateway proxy to be cleared")
	}
	if status.Status != GatewayStatusFailed {
		t.Fatalf("expected status %q, got %q", GatewayStatusFailed, status.Status)
	}
	if status.Summary != "网关已关闭" {
		t.Fatalf("expected summary 网关已关闭, got %q", status.Summary)
	}
}

func TestStopGateway_ReturnsErrorWhenStopFails(t *testing.T) {
	t.Parallel()

	svc := NewAgentService()
	svc.bootstrap = &gatewayBootstrapper{
		managedProcess: &os.Process{Pid: 1234},
		managedOwned:   true,
		stopProcess: func(*os.Process) error {
			return errors.New("access denied")
		},
	}

	_, err := svc.StopGateway(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	bridgeErr, ok := err.(*BridgeError)
	if !ok {
		t.Fatalf("expected *BridgeError, got %T", err)
	}
	if bridgeErr.Code != ErrorCodeGatewayBootstrap {
		t.Fatalf("expected %s, got %s", ErrorCodeGatewayBootstrap, bridgeErr.Code)
	}
}
