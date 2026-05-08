package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONLLoggerWritesAndRedacts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger := NewJSONLLogger(path)

	err := logger.Log(context.Background(), Event{
		Type:      EventToolCall,
		SessionID: "s1",
		Data: map[string]any{
			"api_key": "secret",
			"nested":  map[string]any{"token": "secret-token"},
			"ok":      true,
		},
	})
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one audit line")
	}
	var event Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if event.Data["api_key"] != "[REDACTED]" {
		t.Fatalf("api_key not redacted: %#v", event.Data)
	}
	nested := event.Data["nested"].(map[string]any)
	if nested["token"] != "[REDACTED]" {
		t.Fatalf("nested token not redacted: %#v", nested)
	}
	if event.Timestamp.IsZero() {
		t.Fatal("timestamp was not set")
	}
}
