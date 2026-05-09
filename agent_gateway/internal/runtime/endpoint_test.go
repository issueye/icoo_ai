package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteRuntimeFilesWritesEndpointAndToken(t *testing.T) {
	dir := t.TempDir()
	startedAt := time.Date(2026, 5, 9, 15, 30, 0, 0, time.UTC)
	endpoint, err := WriteRuntimeFiles(dir, Endpoint{
		PID:       42,
		BaseURL:   "http://127.0.0.1:49152",
		StartedAt: startedAt,
	}, "secret-token")
	if err != nil {
		t.Fatalf("WriteRuntimeFiles() error = %v", err)
	}
	if endpoint.TokenFile == "" {
		t.Fatal("TokenFile is empty")
	}

	tokenData, err := os.ReadFile(endpoint.TokenFile)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	if string(tokenData) != "secret-token" {
		t.Fatalf("token file = %q, want secret-token", string(tokenData))
	}

	endpointData, err := os.ReadFile(filepath.Join(dir, "endpoint.json"))
	if err != nil {
		t.Fatalf("read endpoint file: %v", err)
	}
	var decoded Endpoint
	if err := json.Unmarshal(endpointData, &decoded); err != nil {
		t.Fatalf("decode endpoint file: %v", err)
	}
	if decoded.BaseURL != "http://127.0.0.1:49152" {
		t.Fatalf("BaseURL = %q", decoded.BaseURL)
	}
	if decoded.TokenFile != endpoint.TokenFile {
		t.Fatalf("TokenFile = %q, want %q", decoded.TokenFile, endpoint.TokenFile)
	}
}
