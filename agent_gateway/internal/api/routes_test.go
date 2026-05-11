package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/service"
)

func doRequest(t *testing.T, h http.Handler, method, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("Encode() error = %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decodeResponse[T any](t *testing.T, rr *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.NewDecoder(rr.Body).Decode(&out); err != nil {
		t.Fatalf("Decode() error = %v body=%s", err, rr.Body.String())
	}
	return out
}

func TestManagementSettingsPutThenGet(t *testing.T) {
	router := NewRouter(service.NewGatewayService())
	payload := service.ManagementSettings{
		Agents: []service.AgentConfig{
			{ID: "a1", Name: "Agent One", Protocol: "acp", Models: []string{"gpt-5.4"}, Enabled: true},
		},
	}

	putResp := doRequest(t, router, http.MethodPut, "/v1/management/settings", payload)
	if putResp.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 body=%s", putResp.Code, putResp.Body.String())
	}

	getResp := doRequest(t, router, http.MethodGet, "/v1/management/settings", nil)
	if getResp.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200 body=%s", getResp.Code, getResp.Body.String())
	}
	got := decodeResponse[service.ManagementSettings](t, getResp)
	if len(got.Agents) != 1 || got.Agents[0].ID != "a1" {
		t.Fatalf("unexpected settings agents: %#v", got.Agents)
	}
}

func TestPromptRouteReturnsServiceUnavailableWithoutConnector(t *testing.T) {
	router := NewRouter(service.NewGatewayService())

	createResp := doRequest(t, router, http.MethodPost, "/v1/sessions", service.CreateSessionRequest{Title: "demo"})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create session status = %d, want 201 body=%s", createResp.Code, createResp.Body.String())
	}
	session := decodeResponse[service.Session](t, createResp)

	promptResp := doRequest(t, router, http.MethodPost, "/v1/sessions/"+session.ID+"/prompt", service.PromptRequest{Content: "hello"})
	if promptResp.Code != http.StatusServiceUnavailable {
		t.Fatalf("prompt status = %d, want 503 body=%s", promptResp.Code, promptResp.Body.String())
	}
	errResp := decodeResponse[ErrorResponse](t, promptResp)
	if errResp.Code != "connector_unavailable" {
		t.Fatalf("error code = %q, want connector_unavailable", errResp.Code)
	}
}

func TestManagementSettingsRejectsUnsupportedMethod(t *testing.T) {
	router := NewRouter(service.NewGatewayService())
	resp := doRequest(t, router, http.MethodPost, "/v1/management/settings", nil)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 body=%s", resp.Code, resp.Body.String())
	}
}
