package bridge

import "testing"

func TestNormalizeAppSettings_DefaultHostAndPort(t *testing.T) {
	t.Parallel()

	settings := normalizeAppSettings(AppSettings{})
	if settings.GatewayHost != "127.0.0.1" {
		t.Fatalf("expected default host 127.0.0.1, got %q", settings.GatewayHost)
	}
	if settings.GatewayPort != 17889 {
		t.Fatalf("expected default port 17889, got %d", settings.GatewayPort)
	}
}

func TestDecodeSettingsTOML_ReadsHostAndPort(t *testing.T) {
	t.Parallel()

	data := []byte("gateway_binary_path = \"E:/bin/agent-gateway.exe\"\n" +
		"gateway_host = \"127.0.0.1\"\n" +
		"gateway_port = 18080\n")
	settings, err := decodeSettingsTOML(data)
	if err != nil {
		t.Fatalf("decodeSettingsTOML returned error: %v", err)
	}
	if settings.GatewayBinaryPath != "E:/bin/agent-gateway.exe" {
		t.Fatalf("unexpected binary path: %q", settings.GatewayBinaryPath)
	}
	if settings.GatewayHost != "127.0.0.1" {
		t.Fatalf("unexpected host: %q", settings.GatewayHost)
	}
	if settings.GatewayPort != 18080 {
		t.Fatalf("unexpected port: %d", settings.GatewayPort)
	}
}
