package gatewayclient

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDiscoverFromPathReadsEndpointAndToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte(" test-token \n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	startedAt := time.Date(2026, 5, 9, 14, 0, 0, 0, time.UTC)
	endpoint := Endpoint{
		PID:       12345,
		BaseURL:   "http://127.0.0.1:49152",
		TokenFile: tokenPath,
		StartedAt: startedAt,
	}
	data, err := json.Marshal(endpoint)
	if err != nil {
		t.Fatalf("marshal endpoint: %v", err)
	}
	endpointPath := filepath.Join(dir, endpointFileName)
	if err := os.WriteFile(endpointPath, data, 0o600); err != nil {
		t.Fatalf("write endpoint: %v", err)
	}

	gotEndpoint, gotToken, err := DiscoverFromPath(dir)
	if err != nil {
		t.Fatalf("DiscoverFromPath() error = %v", err)
	}
	if gotEndpoint.PID != endpoint.PID {
		t.Fatalf("PID = %d, want %d", gotEndpoint.PID, endpoint.PID)
	}
	if gotEndpoint.BaseURL != endpoint.BaseURL {
		t.Fatalf("BaseURL = %q, want %q", gotEndpoint.BaseURL, endpoint.BaseURL)
	}
	if gotEndpoint.TokenFile != tokenPath {
		t.Fatalf("TokenFile = %q, want %q", gotEndpoint.TokenFile, tokenPath)
	}
	if !gotEndpoint.StartedAt.Equal(startedAt) {
		t.Fatalf("StartedAt = %s, want %s", gotEndpoint.StartedAt, startedAt)
	}
	if gotToken != "test-token" {
		t.Fatalf("token = %q, want %q", gotToken, "test-token")
	}
}
