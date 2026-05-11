package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	if err := os.WriteFile(path, []byte("host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	if cfg.Host != "127.0.0.1" {
		t.Fatalf("cfg.Host = %q, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 17889 {
		t.Fatalf("cfg.Port = %d, want 17889", cfg.Port)
	}
	if cfg.DataDir == "" {
		t.Fatal("cfg.DataDir is empty")
	}
}

func TestLoadFileRejectsUnsupportedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	if err := os.WriteFile(path, []byte("host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\nfoo = \"bar\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadFile(path); err == nil {
		t.Fatal("LoadFile() error = nil, want unsupported key error")
	}
}
