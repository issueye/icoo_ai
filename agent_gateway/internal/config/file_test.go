package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileParsesCurrentKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	content := "host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
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
	if cfg.DataDir != ".agent_gateway" {
		t.Fatalf("cfg.DataDir = %q, want .agent_gateway", cfg.DataDir)
	}
}

func TestLoadFileRejectsUnsupportedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	content := "host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\nfoo = \"bar\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadFile(path); err == nil {
		t.Fatal("LoadFile() error = nil, want unsupported config key error")
	}
}

func TestLoadFileRejectsUnquotedStringValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	content := "host = 127.0.0.1\nport = 17889\ndata_dir = \"./.agent_gateway\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadFile(path); err == nil {
		t.Fatal("LoadFile() error = nil, want host parse error")
	}
}
