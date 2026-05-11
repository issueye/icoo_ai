package main

import "testing"

func TestParseConfigFromFlagsHostPortOnce(t *testing.T) {
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
