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

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

func shutdownServer(t *testing.T, srv *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func authedJSONRequest(t *testing.T, srv *Server, method, path string, payload any) *http.Response {
	t.Helper()
	endpoint := srv.Endpoint()
	if endpoint.BaseURL == "" {
		t.Fatal("server endpoint base URL is empty")
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, endpoint.BaseURL+path, body)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+srv.Token())
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	return resp
}

func TestServerStartExposesHealthAndProtectedRoutes(t *testing.T) {
	dataDir := t.TempDir()
	cfg := config.Default()
	cfg.Host = "127.0.0.1"
	cfg.Port = 0
	cfg.DataDir = dataDir

	srv, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer shutdownServer(t, srv)

	if _, err := os.Stat(filepath.Join(dataDir, "endpoint.json")); err != nil {
		t.Fatalf("endpoint.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "token")); err != nil {
		t.Fatalf("token missing: %v", err)
	}

	endpoint := srv.Endpoint()
	healthResp, err := http.Get(endpoint.BaseURL + "/health")
	if err != nil {
		t.Fatalf("GET /health error = %v", err)
	}
	defer healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", healthResp.StatusCode)
	}

	noAuthResp, err := http.Get(endpoint.BaseURL + "/v1/agents")
	if err != nil {
		t.Fatalf("GET /v1/agents error = %v", err)
	}
	defer noAuthResp.Body.Close()
	if noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", noAuthResp.StatusCode)
	}

	authResp := authedJSONRequest(t, srv, http.MethodGet, "/v1/agents", nil)
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusOK {
		t.Fatalf("authorized /v1/agents status = %d, want 200", authResp.StatusCode)
	}
	var agents []service.AgentProfile
	if err := json.NewDecoder(authResp.Body).Decode(&agents); err != nil {
		t.Fatalf("Decode agents error = %v", err)
	}
	if len(agents) == 0 {
		t.Fatal("agents list is empty, want at least one managed agent")
	}
}

func TestServerPersistsManagementSettingsAcrossRestart(t *testing.T) {
	dataDir := t.TempDir()
	baseCfg := config.Default()
	baseCfg.Host = "127.0.0.1"
	baseCfg.Port = 0
	baseCfg.DataDir = dataDir

	first, err := NewServer(baseCfg)
	if err != nil {
		t.Fatalf("NewServer(first) error = %v", err)
	}
	if err := first.Start(); err != nil {
		t.Fatalf("first.Start() error = %v", err)
	}

	putResp := authedJSONRequest(t, first, http.MethodPut, "/v1/management/settings", service.ManagementSettings{
		Agents: []service.AgentConfig{
			{ID: "persisted-agent", Name: "Persisted", Protocol: "acp", Models: []string{"gpt-5.4"}, Enabled: true},
		},
	})
	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		putResp.Body.Close()
		t.Fatalf("PUT /v1/management/settings status = %d, want 200 body=%s", putResp.StatusCode, string(body))
	}
	putResp.Body.Close()
	shutdownServer(t, first)

	second, err := NewServer(baseCfg)
	if err != nil {
		t.Fatalf("NewServer(second) error = %v", err)
	}
	if err := second.Start(); err != nil {
		t.Fatalf("second.Start() error = %v", err)
	}
	defer shutdownServer(t, second)

	getResp := authedJSONRequest(t, second, http.MethodGet, "/v1/management/settings", nil)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/management/settings status = %d, want 200", getResp.StatusCode)
	}
	var settings service.ManagementSettings
	if err := json.NewDecoder(getResp.Body).Decode(&settings); err != nil {
		t.Fatalf("Decode settings error = %v", err)
	}
	if len(settings.Agents) != 1 || settings.Agents[0].ID != "persisted-agent" {
		t.Fatalf("persisted settings mismatch: %#v", settings.Agents)
	}
}
