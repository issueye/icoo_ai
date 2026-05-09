package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/config"
)

func TestServerDefinitionsFromConfig(t *testing.T) {
	defs := ServerDefinitions(config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"disabled": {Enabled: false, Command: "skip"},
			"remote":   {Enabled: true, URL: "https://example.com/mcp"},
			"local": {
				Enabled:   true,
				Transport: "stdio",
				Command:   "mcp-server",
				Args:      []string{"--root", "."},
				Env:       map[string]string{"TOKEN": "secret"},
			},
		},
	})

	if len(defs) != 2 {
		t.Fatalf("definitions = %+v, want 2 enabled servers", defs)
	}
	if defs[0].Name != "local" || defs[0].Transport != TransportStdio || defs[0].Command != "mcp-server" {
		t.Fatalf("local definition = %+v", defs[0])
	}
	if defs[1].Name != "remote" || defs[1].Transport != TransportHTTP || defs[1].URL != "https://example.com/mcp" {
		t.Fatalf("remote definition = %+v", defs[1])
	}

	defs[0].Env["TOKEN"] = "changed"
	if got := defs[0].Env["TOKEN"]; got != "changed" {
		t.Fatalf("definition env was not mutable in test: %q", got)
	}
}

func TestServerDefinitionsDisabledConfigReturnsEmpty(t *testing.T) {
	defs := ServerDefinitions(config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"local": {Enabled: true, Command: "mcp-server", Env: map[string]string{"TOKEN": "secret"}},
		},
	})
	defs[0].Env["TOKEN"] = "changed"

	second := ServerDefinitions(config.MCPConfig{
		Enabled: true,
		Servers: map[string]config.MCPServerConfig{
			"local": {Enabled: true, Command: "mcp-server", Env: map[string]string{"TOKEN": "secret"}},
		},
	})
	if second[0].Env["TOKEN"] != "secret" {
		t.Fatalf("server env was not cloned: %+v", second[0].Env)
	}

	if got := ServerDefinitions(config.MCPConfig{Enabled: false}); got != nil {
		t.Fatalf("disabled definitions = %+v, want nil", got)
	}
}

func TestNormalizeInputSchema(t *testing.T) {
	schema, err := NormalizeInputSchema(nil, "srv", "tool")
	if err != nil {
		t.Fatalf("NormalizeInputSchema() error = %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(schema, &parsed); err != nil {
		t.Fatalf("schema is invalid json: %v", err)
	}
	if parsed["type"] != "object" {
		t.Fatalf("default schema = %s", schema)
	}

	_, err = NormalizeInputSchema(json.RawMessage(`{`), "srv", "tool")
	if !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}
