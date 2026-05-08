package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestSlogLoggerRotatesBySize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger := NewSlogLogger(LoggerOptions{
		Path:       path,
		MaxSizeMB:  1,
		MaxBackups: 2,
	})
	payload := strings.Repeat("x", 700*1024)

	for i := 0; i < 3; i++ {
		if err := logger.Log(context.Background(), Event{
			Type:    EventToolCall,
			Summary: "large event",
			Data:    map[string]any{"payload": payload, "index": i},
		}); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("current audit log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "audit.1.jsonl")); err != nil {
		t.Fatalf("rotated audit log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "audit.2.jsonl")); err != nil {
		t.Fatalf("second rotated audit log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "audit.3.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("unexpected third rotated audit log: %v", err)
	}
}
