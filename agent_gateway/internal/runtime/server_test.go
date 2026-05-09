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
