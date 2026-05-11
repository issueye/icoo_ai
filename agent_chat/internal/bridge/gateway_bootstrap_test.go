package bridge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayLaunchArgsFromSettingsUsesHostAndPortOnly(t *testing.T) {
	args := gatewayLaunchArgsFromSettings(AppSettings{
		GatewayHost: "127.0.0.1",
		GatewayPort: 18888,
	})
	if len(args) != 4 {
		t.Fatalf("len(args) = %d, want 4 (%#v)", len(args), args)
	}
	if args[0] != "-host" || args[1] != "127.0.0.1" || args[2] != "-port" || args[3] != "18888" {
		t.Fatalf("unexpected launch args: %#v", args)
	}
}

func TestResolveGatewayWorkingDirPrefersParentOfDistWhenConfigExists(t *testing.T) {
	root := t.TempDir()
	distDir := filepath.Join(root, "dist")
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(dist) error = %v", err)
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(config) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "agent-gateway.toml"), []byte("host = \"127.0.0.1\"\nport = 17889\ndata_dir = \"./.agent_gateway\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	bin := filepath.Join(distDir, "agent-gateway.exe")
	if err := os.WriteFile(bin, []byte("stub"), 0o644); err != nil {
		t.Fatalf("WriteFile(bin) error = %v", err)
	}

	wd := resolveGatewayWorkingDir(bin)
	if wd != root {
		t.Fatalf("resolveGatewayWorkingDir() = %q, want %q", wd, root)
	}
}

func TestUniqueNonEmptyPathsDeDuplicatesCaseInsensitive(t *testing.T) {
	out := uniqueNonEmptyPaths([]string{
		"C:\\Tmp\\Gateway",
		"c:\\tmp\\gateway",
		"",
		"  ",
		"C:\\Another",
	})
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3 (%#v)", len(out), out)
	}
	if out[0] != "C:\\Tmp\\Gateway" {
		t.Fatalf("out[0] = %q, want C:\\Tmp\\Gateway", out[0])
	}
	if out[1] != "" {
		t.Fatalf("out[1] = %q, want empty path marker", out[1])
	}
	if out[2] != "C:\\Another" {
		t.Fatalf("out[2] = %q, want C:\\Another", out[2])
	}
}
