package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/api"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/events"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/store"
)

func TestServerStartWritesEndpointAndServesHealth(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.DataDir = dir

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.gatewayServiceFactory = func(st store.Store) (service.GatewayService, error) {
		return service.NewGatewayServiceWithStore(st), nil
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

func TestServerStartWhenACPDisabledStillServesManagementAPI(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()

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
		_ = server.Shutdown(ctx)
	})

	settings := mustAuthorizedJSON[service.ManagementSettings](t, server, http.MethodGet, "/v1/management/settings", nil)
	if len(settings.Agents) == 0 {
		t.Fatalf("expected default agents when acp disabled, got %#v", settings)
	}
}

func TestServerCreateSessionReturnsStructuredErrorWhenACPConnectorFails(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	cfg.ACP.Enabled = true
	cfg.ACP.Command = "definitely-not-a-real-command-12345"

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
		_ = server.Shutdown(ctx)
	})

	var reqBody = map[string]string{
		"title":   "lazy connect fail",
		"cwd":     "E:/code/issueye/icoo_ai",
		"agentId": "icoo-ai-acp",
	}
	reqRaw, _ := json.Marshal(reqBody)
	req, err := http.NewRequest(http.MethodPost, server.Endpoint().BaseURL+"/v1/sessions", bytes.NewReader(reqRaw))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+server.Token())
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/sessions error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 502, body=%s", resp.StatusCode, string(data))
	}
}

func TestServerProjectsPublishedEventsToStore(t *testing.T) {
	cfg := config.Default()
	cfg.DataDir = t.TempDir()
	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.eventBus = events.NewBus(64)
	server.gatewayServiceFactory = func(st store.Store) (service.GatewayService, error) {
		return service.NewGatewayServiceWithStore(st), nil
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
	cfg.DataDir = t.TempDir()
	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	server.eventBus = events.NewBus(64)
	server.gatewayServiceFactory = func(st store.Store) (service.GatewayService, error) {
		return service.NewGatewayServiceWithStore(st), nil
	}

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

func TestServerPersistsManagementSettingsAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "management.db")

	newConfiguredServer := func(t *testing.T) *Server {
		t.Helper()
		cfg := config.Default()
		cfg.DataDir = dir
		server, err := NewServer(cfg)
		if err != nil {
			t.Fatalf("NewServer() error = %v", err)
		}
		server.gatewayServiceFactory = func(st store.Store) (service.GatewayService, error) {
			settingsStore, err := service.NewSQLiteManagementSettingsStore(settingsPath)
			if err != nil {
				return nil, err
			}
			return service.NewGatewayServiceWithAgentsStoreAndSettingsStore(
				nil,
				st,
				settingsStore,
			), nil
		}
		return server
	}

	server := newConfiguredServer(t)
	if err := server.Start(); err != nil {
		t.Fatalf("first Start() error = %v", err)
	}

	updated := mustAuthorizedJSON[service.ManagementSettings](t, server, http.MethodPut, "/v1/management/settings", service.ManagementSettings{
		Channels: []service.ChannelConfig{
			{ID: "channel_saved", Name: "Saved Channel", Type: "lark", Enabled: true, AppID: "app", AppSecret: "secret"},
		},
		MCPServers: []service.MCPServerConfig{
			{ID: "mcp_saved", Name: "Saved MCP", Command: "node", Args: []string{"saved.js"}, Enabled: true},
		},
		ScheduleTasks: []service.ScheduleTaskConfig{
			{ID: "task_saved", Name: "Saved Task", Spec: "*/15 * * * *", Content: "每15分钟同步一次状态", Enabled: true},
		},
		Agents: []service.AgentConfig{
			{ID: "persisted_agent", Name: "Persisted Agent", Protocol: "acp", Models: []string{"gpt-5.4"}, Enabled: true},
		},
	})
	if len(updated.Agents) != 1 || updated.Agents[0].ID != "persisted_agent" {
		t.Fatalf("unexpected persisted settings from PUT: %#v", updated)
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	if err := server.Shutdown(ctx1); err != nil {
		t.Fatalf("first Shutdown() error = %v", err)
	}

	restarted := newConfiguredServer(t)
	if err := restarted.Start(); err != nil {
		t.Fatalf("second Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := restarted.Shutdown(ctx); err != nil {
			t.Fatalf("second Shutdown() error = %v", err)
		}
	})

	settings := mustAuthorizedJSON[service.ManagementSettings](t, restarted, http.MethodGet, "/v1/management/settings", nil)
	if len(settings.Channels) != 1 || settings.Channels[0].ID != "channel_saved" {
		t.Fatalf("channels were not restored after restart: %#v", settings)
	}
	if len(settings.MCPServers) != 1 || settings.MCPServers[0].ID != "mcp_saved" {
		t.Fatalf("settings were not restored after restart: %#v", settings)
	}
	if len(settings.ScheduleTasks) != 1 || settings.ScheduleTasks[0].ID != "task_saved" {
		t.Fatalf("schedule tasks were not restored after restart: %#v", settings)
	}
	if settings.ScheduleTasks[0].Content != "每15分钟同步一次状态" {
		t.Fatalf("schedule task content was not restored after restart: %#v", settings.ScheduleTasks[0])
	}
	if len(settings.Agents) != 1 || settings.Agents[0].ID != "persisted_agent" {
		t.Fatalf("agents were not restored after restart: %#v", settings)
	}

	agents := mustAuthorizedJSON[[]service.AgentProfile](t, restarted, http.MethodGet, "/v1/agents", nil)
	if len(agents) != 1 || agents[0].ID != "persisted_agent" {
		t.Fatalf("/v1/agents not consistent after restart: %#v", agents)
	}
}

func mustAuthorizedJSON[T any](t *testing.T, server *Server, method, path string, payload any) T {
	t.Helper()

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequest(method, server.Endpoint().BaseURL+path, body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+server.Token())
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s error = %v", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s status = %d body=%s", method, path, resp.StatusCode, string(data))
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
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
