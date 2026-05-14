package smoke

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/config"
	gwruntime "github.com/icoo-ai/icoo-ai/agent_gateway/internal/runtime"
)

func TestGatewaySmokeRESTAndWebSocket(t *testing.T) {
	server, baseURL, token := startGateway(t)

	getJSON(t, baseURL+"/health", "", http.StatusOK)

	agent := postJSON(t, baseURL+"/v1/agents", token, map[string]any{
		"name":    "smoke-agent",
		"enabled": false,
	}, http.StatusCreated)
	if agent.Code != "ok" {
		t.Fatalf("create agent code = %q, want ok", agent.Code)
	}

	page := getJSON(t, baseURL+"/v1/agents?page=1&pageSize=10", token, http.StatusOK)
	if page.Code != "ok" {
		t.Fatalf("list agents code = %q, want ok", page.Code)
	}

	status := getJSON(t, baseURL+"/v1/agents/runtime-status", token, http.StatusOK)
	if status.Code != "ok" {
		t.Fatalf("runtime status code = %q, want ok", status.Code)
	}

	wsURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/v1/events"
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "missing"}); err != nil {
		t.Fatalf("write websocket request: %v", err)
	}
	var response map[string]any
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatalf("read websocket response: %v", err)
	}
	if response["error"] == nil {
		t.Fatalf("websocket response = %#v, want JSON-RPC error for missing method", response)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func startGateway(t *testing.T) (*gwruntime.Server, string, string) {
	t.Helper()
	token := "smoke-token"
	server, err := gwruntime.NewServer(config.Config{
		Host:      "127.0.0.1",
		Port:      0,
		DataDir:   t.TempDir(),
		AuthToken: token,
	})
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
	return server, server.Endpoint().BaseURL, token
}

type apiResponse struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func getJSON(t *testing.T, url string, token string, status int) apiResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return doJSON(t, req, status)
}

func postJSON(t *testing.T, url string, token string, body any, status int) apiResponse {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return doJSON(t, req, status)
}

func doJSON(t *testing.T, req *http.Request, status int) apiResponse {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != status {
		t.Fatalf("%s %s status = %d, want %d", req.Method, req.URL.String(), resp.StatusCode, status)
	}
	var out apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return out
}
