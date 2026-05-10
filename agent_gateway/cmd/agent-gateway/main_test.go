package main

import (
	"reflect"
	"testing"
)

func TestParseConfigFromFlagsACPOptions(t *testing.T) {
	cfg, once, err := parseConfigFromFlags([]string{
		"-host", "127.0.0.1",
		"-port", "17889",
		"-acp-enabled",
		"-acp-command", "icoo-ai",
		"-acp-args", "serve --transport stdio",
		"-once",
	})
	if err != nil {
		t.Fatalf("parseConfigFromFlags() error = %v", err)
	}
	if !once {
		t.Fatal("once = false, want true")
	}
	if !cfg.ACP.Enabled {
		t.Fatal("cfg.ACP.Enabled = false, want true")
	}
	if cfg.ACP.Command != "icoo-ai" {
		t.Fatalf("cfg.ACP.Command = %q, want %q", cfg.ACP.Command, "icoo-ai")
	}
	wantArgs := []string{"serve", "--transport", "stdio"}
	if !reflect.DeepEqual(cfg.ACP.Args, wantArgs) {
		t.Fatalf("cfg.ACP.Args = %#v, want %#v", cfg.ACP.Args, wantArgs)
	}
}

func TestParseACPArgs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "blank",
			in:   "   ",
			want: nil,
		},
		{
			name: "whitespace separated",
			in:   "serve --transport stdio",
			want: []string{"serve", "--transport", "stdio"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseACPArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseACPArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
