package main

import (
	"os"
	"path/filepath"
	"testing"
)

func withConfigFile(t *testing.T, content string) {
	t.Helper()
	wd := t.TempDir()
	configDir := filepath.Join(wd, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "agent-gateway.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
}

func TestParseConfigFromFlags_CurrentCLIContract(t *testing.T) {
	withConfigFile(t, "host = \"127.0.0.1\"\nport = 19000\ndata_dir = \"./.agent_gateway\"\n")

	cfg, once, err := parseConfigFromFlags([]string{
		"-host", "127.0.0.1",
		"-port", "17889",
		"-once",
	})
	if err != nil {
		t.Fatalf("parseConfigFromFlags() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("cfg.Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 17889 {
		t.Fatalf("cfg.Port = %d, want 17889", cfg.Port)
	}
	if !once {
		t.Fatal("once = false, want true")
	}
}

func TestParseConfigFromFlags_RejectsRemovedLegacyFlags(t *testing.T) {
	withConfigFile(t, "host = \"127.0.0.1\"\nport = 19000\ndata_dir = \"./.agent_gateway\"\n")

	tests := [][]string{
		{"-data-dir", "./tmp"},
		{"-acp-enabled"},
		{"-acp-command", "icoo-ai"},
		{"-acp-args", "serve --transport stdio"},
		{"-acp-pool-size", "2"},
	}
	for _, args := range tests {
		if _, _, err := parseConfigFromFlags(args); err == nil {
			t.Fatalf("parseConfigFromFlags(%v) error = nil, want error", args)
		}
	}
}

func TestParseConfigFromFlags_RequiresConfigFile(t *testing.T) {
	wd := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(wd); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	if _, _, err := parseConfigFromFlags(nil); err == nil {
		t.Fatal("parseConfigFromFlags() error = nil, want missing config file error")
	}
}
