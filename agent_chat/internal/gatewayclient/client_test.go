package gatewayclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHealthSendsAuthorization(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/health" {
			t.Fatalf("path = %s, want /health", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want %q", got, "Bearer test-token")
		}
		_ = json.NewEncoder(w).Encode(HealthResponse{
			Status:       "ok",
			Version:      "0.1.0",
			Capabilities: []string{"health", "local-auth"},
			StartedAt:    startedAt,
		})
	}))
	defer server.Close()

	health, err := New(server.URL+"/", "test-token").Health(context.Background())
	if err != nil {
		t.Fatalf("Health() error = %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("Status = %q, want %q", health.Status, "ok")
	}
	if health.Version != "0.1.0" {
		t.Fatalf("Version = %q, want %q", health.Version, "0.1.0")
	}
	if len(health.Capabilities) != 2 || health.Capabilities[0] != "health" || health.Capabilities[1] != "local-auth" {
		t.Fatalf("Capabilities = %#v", health.Capabilities)
	}
	if !health.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %s, want %s", health.StartedAt, startedAt)
	}
}

func TestHealthWithoutTokenOmitsAuthorization(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("Authorization = %q, want empty", got)
		}
		_ = json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
	}))
	defer server.Close()

	if _, err := New(server.URL, "").Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}
}

func TestHealthReturnsErrorForNon2xx(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not healthy", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := New(server.URL, "test-token").Health(context.Background())
	if err == nil {
		t.Fatal("Health() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Fatalf("Health() error = %q, want status code", err.Error())
	}
}
