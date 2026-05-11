package gatewayclient

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverFromPathReadsEndpointAndToken(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte("abc123\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(token) error = %v", err)
	}
	endpointPath := filepath.Join(dir, "endpoint.json")
	payload := Endpoint{
		PID:       123,
		BaseURL:   "http://127.0.0.1:17889",
		TokenFile: tokenPath,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal(endpoint) error = %v", err)
	}
	if err := os.WriteFile(endpointPath, raw, 0o600); err != nil {
		t.Fatalf("WriteFile(endpoint) error = %v", err)
	}

	endpoint, token, err := DiscoverFromPath(dir)
	if err != nil {
		t.Fatalf("DiscoverFromPath() error = %v", err)
	}
	if endpoint.BaseURL != payload.BaseURL {
		t.Fatalf("endpoint.BaseURL = %q, want %q", endpoint.BaseURL, payload.BaseURL)
	}
	if token != "abc123" {
		t.Fatalf("token = %q, want abc123", token)
	}
}

func TestReadEndpointRejectsEmptyBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "endpoint.json")
	if err := os.WriteFile(path, []byte(`{"pid":1,"baseUrl":"","tokenFile":"token"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := ReadEndpoint(path); err == nil {
		t.Fatal("ReadEndpoint() error = nil, want empty baseUrl error")
	}
}

func TestReadTokenFallsBackToSiblingTokenFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "token"), []byte("fallback-token"), 0o600); err != nil {
		t.Fatalf("WriteFile(token) error = %v", err)
	}
	token, err := ReadToken(Endpoint{}, dir)
	if err != nil {
		t.Fatalf("ReadToken() error = %v", err)
	}
	if token != "fallback-token" {
		t.Fatalf("token = %q, want fallback-token", token)
	}
}
