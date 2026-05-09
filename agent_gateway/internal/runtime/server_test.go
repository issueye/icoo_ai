package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/connector"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
)

func TestServerStartWritesEndpointAndServesHealth(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dir

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	endpoint := server.Endpoint()
	if endpoint.BaseURL == "" {
		t.Fatal("Endpoint().BaseURL is empty")
	}
	if endpoint.TokenFile == "" {
		t.Fatal("Endpoint().TokenFile is empty")
	}

	if _, err := os.Stat(filepath.Join(dir, "endpoint.json")); err != nil {
		t.Fatalf("endpoint.json missing: %v", err)
	}
	tokenData, err := os.ReadFile(endpoint.TokenFile)
	if err != nil {
		t.Fatalf("read token: %v", err)
	}
	if string(tokenData) != server.Token() {
		t.Fatalf("token file does not match server token")
	}

	resp, err := http.Get(endpoint.BaseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var health api.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("health status = %q, want ok", health.Status)
	}
	if health.Version != config.Version {
		t.Fatalf("health version = %q, want %q", health.Version, config.Version)
	}

	unauthorizedResp, err := http.Get(endpoint.BaseURL + "/v1/agents")
	if err != nil {
		t.Fatalf("GET /v1/agents without token error = %v", err)
	}
	defer unauthorizedResp.Body.Close()
	if unauthorizedResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorizedResp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodGet, endpoint.BaseURL+"/v1/agents", nil)
	if err != nil {
		t.Fatalf("create agents request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+server.Token())
	agentsResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/agents with token error = %v", err)
	}
	defer agentsResp.Body.Close()
	if agentsResp.StatusCode != http.StatusOK {
		t.Fatalf("agents status = %d, want 200", agentsResp.StatusCode)
	}
}

func TestServerStartReturnsStructuredErrorWhenACPConnectorFails(t *testing.T) {
	cfg := config.Default()
	cfg.ACP.Enabled = true
	cfg.ACP.Command = "definitely-not-a-real-command-12345"

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	err = server.Start()
	if err == nil {
		t.Fatal("Start() error = nil, want structured connector error")
	}
	structured, ok := err.(*connector.Error)
	if !ok {
		t.Fatalf("expected *connector.Error, got %T", err)
	}
	if structured.Code != "connector_start_failed" {
		t.Fatalf("error code = %q, want connector_start_failed", structured.Code)
	}
}

func TestServerProjectsPublishedEventsToStore(t *testing.T) {
	cfg := config.Default()
	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.eventBus = events.NewBus(64)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	})

	now := time.Now().UTC()
	server.eventBus.Publish(events.Envelope{
		ID:        "evt_run_1",
		Type:      "run.updated",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_proj_1",
		RunID:     "run_proj_1",
		Payload:   map[string]any{"status": "in_progress"},
		CreatedAt: now,
	})
	server.eventBus.Publish(events.Envelope{
		ID:        "evt_msg_1",
		Type:      "message.created",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_proj_1",
		RunID:     "run_proj_1",
		Payload:   map[string]any{"role": "assistant", "content": "hello from acp"},
		CreatedAt: now,
	})

	server.eventBus.Publish(events.Envelope{
		ID:        "evt_appr_1",
		Type:      "approval.requested",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_proj_1",
		RunID:     "run_proj_1",
		Payload: map[string]any{
			"id":        "approval_proj_1",
			"requestId": "connreq_proj_1",
			"action":    "write_file",
			"message":   "need write approval",
		},
		CreatedAt: now,
	})

	waitFor(t, 2*time.Second, func() bool {
		runs, err := server.store.ListRuns(context.Background(), "sess_proj_1")
		if err != nil || len(runs) == 0 {
			return false
		}
		messages, err := server.store.ListMessages(context.Background(), "sess_proj_1")
		if err != nil || len(messages) == 0 {
			return false
		}
		approvals, err := server.store.ListApprovals(context.Background())
		if err != nil || len(approvals) == 0 {
			return false
		}
		return true
	})

	runs, err := server.store.ListRuns(context.Background(), "sess_proj_1")
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if runs[0].RunID != "run_proj_1" || runs[0].Status != "in_progress" {
		t.Fatalf("projected run mismatch: %#v", runs[0])
	}

	messages, err := server.store.ListMessages(context.Background(), "sess_proj_1")
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	var foundMessage bool
	for _, message := range messages {
		if message.ID == "evt_msg_1" {
			foundMessage = true
			if message.Summary == "" {
				t.Fatalf("projected message summary should not be empty: %#v", message)
			}
		}
	}
	if !foundMessage {
		t.Fatalf("evt_msg_1 not found in projected messages: %#v", messages)
	}

	approvals, err := server.store.ListApprovals(context.Background())
	if err != nil {
		t.Fatalf("ListApprovals() error = %v", err)
	}
	var found bool
	for _, approval := range approvals {
		if approval.ID == "approval_proj_1" {
			found = true
			if approval.ConnectorRequestID != "connreq_proj_1" || approval.Status != "pending" {
				t.Fatalf("projected approval mismatch: %#v", approval)
			}
		}
	}
	if !found {
		t.Fatalf("approval_proj_1 not found in approvals: %#v", approvals)
	}
}

func TestServerShutdownStopsEventProjectionConsumption(t *testing.T) {
	cfg := config.Default()
	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.eventBus = events.NewBus(64)

	if err := server.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	server.eventBus.Publish(events.Envelope{
		ID:        "evt_before_shutdown",
		Type:      "run.updated",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_shutdown_1",
		RunID:     "run_before_shutdown",
		Payload:   map[string]any{"status": "running"},
		CreatedAt: time.Now().UTC(),
	})
	waitFor(t, 2*time.Second, func() bool {
		runs, err := server.store.ListRuns(context.Background(), "sess_shutdown_1")
		return err == nil && len(runs) == 1
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	server.eventBus.Publish(events.Envelope{
		ID:        "evt_after_shutdown",
		Type:      "run.updated",
		AgentID:   "icoo-ai-acp",
		SessionID: "sess_shutdown_1",
		RunID:     "run_after_shutdown",
		Payload:   map[string]any{"status": "running"},
		CreatedAt: time.Now().UTC(),
	})

	time.Sleep(150 * time.Millisecond)
	runs, err := server.store.ListRuns(context.Background(), "sess_shutdown_1")
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run_before_shutdown" {
		t.Fatalf("expected no projection after shutdown, got runs=%#v", runs)
	}
}

func waitFor(t *testing.T, timeout time.Duration, predicate func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
