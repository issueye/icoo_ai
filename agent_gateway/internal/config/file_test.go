package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFileParsesCurrentKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	content := "host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\nauth_token = \"abc-token\"\n"
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
	if cfg.AuthToken != "abc-token" {
		t.Fatalf("cfg.AuthToken = %q, want abc-token", cfg.AuthToken)
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

func TestEnsureAuthTokenGeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-gateway.toml")
	content := "host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\nauth_token = \"\"\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() error = %v", err)
	}
	updated, err := EnsureAuthToken(path, cfg)
	if err != nil {
		t.Fatalf("EnsureAuthToken() error = %v", err)
	}
	if updated.AuthToken == "" {
		t.Fatal("updated.AuthToken = empty, want generated token")
	}

	reloaded, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile() after ensure error = %v", err)
	}
	if reloaded.AuthToken == "" {
		t.Fatal("reloaded.AuthToken = empty, want persisted token")
	}
}
