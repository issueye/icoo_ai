package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestRunUnknownCommand(t *testing.T) {
	err := run([]string{"missing"})
	if err == nil || !strings.Contains(err.Error(), `unknown command "missing"`) {
		t.Fatalf("run() error = %v", err)
	}
}

func TestMigrateClaudeConfigUsage(t *testing.T) {
	err := run([]string{"migrate-claude-config"})
	if err == nil || !strings.Contains(err.Error(), "usage: icoo-ai migrate-claude-config") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestConfigCommandPrintsDefaults(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	output := captureStdout(t, func() {
		if err := run([]string{"config"}); err != nil {
			t.Fatalf("run(config) error = %v", err)
		}
	})
	if !strings.Contains(output, "provider=openai") || !strings.Contains(output, "approval_mode=workspace-write") {
		t.Fatalf("config output = %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe() error = %v", err)
	}
	os.Stdout = writer
	defer func() { os.Stdout = old }()

	fn()
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	return buf.String()
}
