package main

import (
	"os"
	"path/filepath"
	"testing"
)

func withGatewayConfig(t *testing.T, content string) {
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

func TestParseConfigFromFlagsHostPortOnce(t *testing.T) {
	withGatewayConfig(t, "host = \"127.0.0.1\"\nport = 10001\ndata_dir = \"./.agent_gateway\"\n")
	cfg, once, err := parseConfigFromFlags([]string{
		"-host", "127.0.0.1",
		"-port", "17889",
		"-once",
	})
	if err != nil {
		t.Fatalf("parseConfigFromFlags() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("cfg.Host = %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 17889 {
		t.Fatalf("cfg.Port = %d, want %d", cfg.Port, 17889)
	}
	if !once {
		t.Fatal("once = false, want true")
	}
}

func TestParseConfigFromFlagsRejectsLegacyFlags(t *testing.T) {
	withGatewayConfig(t, "host = \"127.0.0.1\"\nport = 10001\ndata_dir = \"./.agent_gateway\"\n")
	tests := []struct {
		name string
		args []string
	}{
		{name: "data-dir", args: []string{"-data-dir", "./tmp"}},
		{name: "acp-enabled", args: []string{"-acp-enabled"}},
		{name: "acp-command", args: []string{"-acp-command", "icoo-ai"}},
		{name: "acp-args", args: []string{"-acp-args", "serve --transport stdio"}},
		{name: "acp-pool-size", args: []string{"-acp-pool-size", "2"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := parseConfigFromFlags(tt.args); err == nil {
				t.Fatalf("parseConfigFromFlags(%v) error = nil, want error", tt.args)
			}
		})
	}
}

func TestParseConfigFromFlagsRequiresConfigFile(t *testing.T) {
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
