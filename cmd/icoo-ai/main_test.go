package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
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

func TestVersionCommandPrintsBuildInfo(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run([]string{"version"}); err != nil {
			t.Fatalf("run(version) error = %v", err)
		}
	})
	if !strings.Contains(output, "icoo-ai ") || !strings.Contains(output, "platform=") {
		t.Fatalf("version output = %q", output)
	}
}

func TestDoctorRedactsConfigAPIKey(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("Chdir() restore error = %v", err)
		}
	}()
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cwd, ".icoo-ai.toml"), []byte(`api_key = "secret-from-config"`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	output := captureStdout(t, func() {
		if err := run([]string{"doctor"}); err != nil {
			t.Fatalf("run(doctor) error = %v", err)
		}
	})
	if strings.Contains(output, "secret-from-config") {
		t.Fatalf("doctor leaked api key: %q", output)
	}
	if !strings.Contains(output, "config api_key=[set]") {
		t.Fatalf("doctor output = %q", output)
	}
}

func TestParseExplicitSkillPrompt(t *testing.T) {
	name, task, ok := parseExplicitSkillPrompt("/skill go-review review internal/agent")
	if !ok || name != "go-review" || task != "review internal/agent" {
		t.Fatalf("parse = %q %q %v", name, task, ok)
	}
	_, _, ok = parseExplicitSkillPrompt("plain prompt")
	if ok {
		t.Fatal("plain prompt should not be parsed as skill")
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
