package bridge

import (
	"strings"
	"testing"
)

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

func TestNormalizeAppSettings_AllowEmptyChannels(t *testing.T) {
	t.Parallel()

	settings := normalizeAppSettings(AppSettings{})
	if len(settings.Channels) != 0 {
		t.Fatalf("expected empty channels, got %d", len(settings.Channels))
	}
}

func TestNormalizeAppSettings_AllowDuplicateChannelTypes(t *testing.T) {
	t.Parallel()

	settings := normalizeAppSettings(AppSettings{
		Channels: []ChannelConfig{
			{ID: "qq", Type: "qq", Name: "QQ 机器人 A"},
			{ID: "qq", Type: "qq", Name: "QQ 机器人 B"},
			{ID: "lark", Type: "lark", Name: "飞书机器人 A"},
		},
	})

	if len(settings.Channels) != 3 {
		t.Fatalf("expected 3 channels after normalization, got %d", len(settings.Channels))
	}
	if settings.Channels[0].Type != "qq" || settings.Channels[1].Type != "qq" {
		t.Fatalf("expected duplicate qq channel types preserved, got %#v", settings.Channels)
	}
	if settings.Channels[0].ID == settings.Channels[1].ID {
		t.Fatalf("expected duplicate ids to be disambiguated, got %q", settings.Channels[0].ID)
	}
}

func TestDecodeSettingsTOML_ReadsHostPort(t *testing.T) {
	t.Parallel()

	data := []byte("gateway_binary_path = \"E:/bin/agent-gateway.exe\"\n" +
		"gateway_host = \"127.0.0.1\"\n" +
		"gateway_port = 18080\n" +
		"log_level = \"debug\"\n" +
		"log_format = \"json\"\n" +
		"log_file_path = \"logs/custom.log\"\n")
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
	if settings.LogLevel != "debug" {
		t.Fatalf("unexpected log level: %q", settings.LogLevel)
	}
	if settings.LogFormat != "json" {
		t.Fatalf("unexpected log format: %q", settings.LogFormat)
	}
	if settings.LogFilePath != "logs/custom.log" {
		t.Fatalf("unexpected log file path: %q", settings.LogFilePath)
	}
	if len(settings.Channels) != 0 {
		t.Fatalf("expected no channels from toml, got %d", len(settings.Channels))
	}
}

func TestNormalizeAppSettings_DefaultAndInvalidLogConfig(t *testing.T) {
	t.Parallel()

	settings := normalizeAppSettings(AppSettings{})
	if settings.LogLevel != "info" {
		t.Fatalf("expected default log level info, got %q", settings.LogLevel)
	}
	if settings.LogFormat != "text" {
		t.Fatalf("expected default log format text, got %q", settings.LogFormat)
	}
	if settings.LogFilePath != "logs/agent_chat.log" {
		t.Fatalf("expected default log file path logs/agent_chat.log, got %q", settings.LogFilePath)
	}

	invalid := normalizeAppSettings(AppSettings{
		LogLevel:  "trace",
		LogFormat: "yaml",
	})
	if invalid.LogLevel != "info" {
		t.Fatalf("expected normalized log level info, got %q", invalid.LogLevel)
	}
	if invalid.LogFormat != "text" {
		t.Fatalf("expected normalized log format text, got %q", invalid.LogFormat)
	}
}

func TestEncodeSettingsTOML_PreservesLogConfig(t *testing.T) {
	t.Parallel()

	data := string(encodeSettingsTOML(normalizeAppSettings(AppSettings{
		GatewayBinaryPath: "E:/bin/agent-gateway.exe",
		GatewayHost:       "127.0.0.1",
		GatewayPort:       18080,
		LogLevel:          "debug",
		LogFormat:         "json",
		LogFilePath:       "logs/runtime.log",
		Channels: []ChannelConfig{
			{
				ID:         "qq",
				Name:       "QQ机器人",
				Type:       "qq",
				Enabled:    true,
				AppID:      "qq-app",
				AppSecret:  "qq-secret",
				BotToken:   "qq-token",
				WebhookURL: "https://qq.example/webhook",
			},
		},
	})))

	if containsLine(data, "acp_enabled = true") || strings.Contains(data, "acp_command") || strings.Contains(data, "acp_args") {
		t.Fatalf("encoded settings should not include acp fields: %q", data)
	}
	if !containsLine(data, "log_level = \"debug\"") {
		t.Fatalf("encoded settings missing log_level: %q", data)
	}
	if !containsLine(data, "log_format = \"json\"") {
		t.Fatalf("encoded settings missing log_format: %q", data)
	}
	if !containsLine(data, "log_file_path = \"logs/runtime.log\"") {
		t.Fatalf("encoded settings missing log_file_path: %q", data)
	}
	if strings.Contains(data, "[[channels]]") || strings.Contains(data, "webhook_url") {
		t.Fatalf("encoded settings should not include channel fields: %q", data)
	}
}

func containsLine(data string, expected string) bool {
	for _, line := range strings.Split(data, "\n") {
		if line == expected {
			return true
		}
	}
	return false
}
