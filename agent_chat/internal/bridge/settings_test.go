package bridge

import (
	"strings"
	"testing"
)

func TestNormalizeAppSettingsAppliesDefaultsAndSanitizesInvalidValues(t *testing.T) {
	normalized := normalizeAppSettings(AppSettings{
		GatewayHost:  "",
		GatewayPort:  -1,
		GatewayToken: "  demo-token  ",
		LogLevel:     "verbose",
		LogFormat:    "yaml",
		LogFilePath:  "",
	})
	if normalized.GatewayHost != "127.0.0.1" {
		t.Fatalf("GatewayHost = %q, want 127.0.0.1", normalized.GatewayHost)
	}
	if normalized.GatewayPort != 17889 {
		t.Fatalf("GatewayPort = %d, want 17889", normalized.GatewayPort)
	}
	if normalized.GatewayToken != "demo-token" {
		t.Fatalf("GatewayToken = %q, want demo-token", normalized.GatewayToken)
	}
	if normalized.LogLevel != "info" {
		t.Fatalf("LogLevel = %q, want info", normalized.LogLevel)
	}
	if normalized.LogFormat != "text" {
		t.Fatalf("LogFormat = %q, want text", normalized.LogFormat)
	}
	if normalized.LogFilePath != "logs/agent_chat.log" {
		t.Fatalf("LogFilePath = %q, want logs/agent_chat.log", normalized.LogFilePath)
	}
}

func TestEncodeSettingsTOMLIncludesOnlyLocalGatewayAndLogKeys(t *testing.T) {
	data := string(encodeSettingsTOML(AppSettings{
		GatewayBinaryPath: "E:/bin/agent-gateway.exe",
		GatewayHost:       "127.0.0.1",
		GatewayPort:       18080,
		LogLevel:          "debug",
		LogFormat:         "json",
		LogFilePath:       "logs/custom.log",
		Channels: []ChannelConfig{
			{ID: "qq", Name: "QQ", Type: "qq", Enabled: true},
		},
		Agents: []AgentConfig{
			{ID: "a1", Name: "A1", Protocol: "acp", Models: []string{"gpt-5.4"}, Enabled: true},
		},
	}))

	required := []string{
		"gateway_binary_path = ",
		"gateway_host = ",
		"gateway_port = 18080",
		"gateway_token = ",
		"log_level = ",
		"log_format = ",
		"log_file_path = ",
	}
	for _, key := range required {
		if !strings.Contains(data, key) {
			t.Fatalf("encoded settings missing key %q: %s", key, data)
		}
	}
	disallowed := []string{
		"[[channels]]",
		"[[agents]]",
	}
	for _, key := range disallowed {
		if strings.Contains(data, key) {
			t.Fatalf("encoded settings should not include %q: %s", key, data)
		}
	}
}

func TestDecodeSettingsTOMLReadsCurrentKeys(t *testing.T) {
	raw := []byte(
		"gateway_binary_path = \"E:/bin/agent-gateway.exe\"\n" +
			"gateway_host = \"127.0.0.1\"\n" +
			"gateway_port = 18080\n" +
			"gateway_token = \"test-token\"\n" +
			"log_level = \"debug\"\n" +
			"log_format = \"json\"\n" +
			"log_file_path = \"logs/custom.log\"\n",
	)
	settings, err := decodeSettingsTOML(raw)
	if err != nil {
		t.Fatalf("decodeSettingsTOML() error = %v", err)
	}
	if settings.GatewayBinaryPath != "E:/bin/agent-gateway.exe" {
		t.Fatalf("GatewayBinaryPath = %q, want E:/bin/agent-gateway.exe", settings.GatewayBinaryPath)
	}
	if settings.GatewayHost != "127.0.0.1" || settings.GatewayPort != 18080 {
		t.Fatalf("gateway endpoint mismatch: host=%q port=%d", settings.GatewayHost, settings.GatewayPort)
	}
	if settings.GatewayToken != "test-token" {
		t.Fatalf("GatewayToken = %q, want test-token", settings.GatewayToken)
	}
	if settings.LogLevel != "debug" || settings.LogFormat != "json" || settings.LogFilePath != "logs/custom.log" {
		t.Fatalf("log settings mismatch: %#v", settings)
	}
}
