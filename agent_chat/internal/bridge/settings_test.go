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

func TestNormalizeAppSettings_DefaultChannels(t *testing.T) {
	t.Parallel()

	settings := normalizeAppSettings(AppSettings{})
	if len(settings.Channels) != 3 {
		t.Fatalf("expected 3 default channels, got %d", len(settings.Channels))
	}
	expected := []struct {
		name string
		typ  string
	}{
		{name: "QQ机器人", typ: "qq"},
		{name: "飞书机器人", typ: "lark"},
		{name: "微信机器人", typ: "wechat"},
	}
	for i, item := range expected {
		if settings.Channels[i].Type != item.typ {
			t.Fatalf("expected channel type %q at index %d, got %q", item.typ, i, settings.Channels[i].Type)
		}
		if settings.Channels[i].Name != item.name {
			t.Fatalf("expected channel name %q at index %d, got %q", item.name, i, settings.Channels[i].Name)
		}
	}
}

func TestDecodeSettingsTOML_ReadsHostPortAndChannels(t *testing.T) {
	t.Parallel()

	data := []byte("gateway_binary_path = \"E:/bin/agent-gateway.exe\"\n" +
		"gateway_host = \"127.0.0.1\"\n" +
		"gateway_port = 18080\n" +
		"acp_enabled = true\n" +
		"acp_command = \"npx\"\n" +
		"acp_args = \"-y @acp/server --stdio\"\n" +
		"log_level = \"debug\"\n" +
		"log_format = \"json\"\n" +
		"log_file_path = \"logs/custom.log\"\n" +
		"\n[[channels]]\n" +
		"id = \"qq\"\n" +
		"name = \"QQ机器人\"\n" +
		"type = \"qq\"\n" +
		"enabled = true\n" +
		"app_id = \"qq-app\"\n" +
		"app_secret = \"qq-secret\"\n" +
		"bot_token = \"qq-token\"\n" +
		"webhook_url = \"https://qq.example/webhook\"\n")
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
	if !settings.ACPEnabled {
		t.Fatal("expected acp enabled true")
	}
	if settings.ACPCommand != "npx" {
		t.Fatalf("unexpected acp command: %q", settings.ACPCommand)
	}
	if settings.ACPArgs != "-y @acp/server --stdio" {
		t.Fatalf("unexpected acp args: %q", settings.ACPArgs)
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
	if len(settings.Channels) != 1 {
		t.Fatalf("expected decoded channels length 1, got %d", len(settings.Channels))
	}
	first := settings.Channels[0]
	if first.Type != "qq" || first.ID != "qq" {
		t.Fatalf("unexpected channel id/type: id=%q type=%q", first.ID, first.Type)
	}
	if !first.Enabled {
		t.Fatal("expected qq channel enabled true")
	}
	if first.AppID != "qq-app" || first.AppSecret != "qq-secret" || first.BotToken != "qq-token" {
		t.Fatalf("unexpected channel credentials: appID=%q appSecret=%q botToken=%q", first.AppID, first.AppSecret, first.BotToken)
	}
	if first.WebhookURL != "https://qq.example/webhook" {
		t.Fatalf("unexpected webhook url: %q", first.WebhookURL)
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
		ACPEnabled:        true,
		ACPCommand:        "npx",
		ACPArgs:           "-y @acp/server --stdio",
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

	if !containsLine(data, "acp_enabled = true") {
		t.Fatalf("encoded settings missing acp_enabled: %q", data)
	}
	if !containsLine(data, "acp_command = \"npx\"") {
		t.Fatalf("encoded settings missing acp_command: %q", data)
	}
	if !containsLine(data, "acp_args = \"-y @acp/server --stdio\"") {
		t.Fatalf("encoded settings missing acp_args: %q", data)
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
	if !containsLine(data, "[[channels]]") {
		t.Fatalf("encoded settings missing channel table: %q", data)
	}
	if !containsLine(data, "type = \"qq\"") {
		t.Fatalf("encoded settings missing qq channel type: %q", data)
	}
	if !containsLine(data, "enabled = true") {
		t.Fatalf("encoded settings missing qq channel enabled flag: %q", data)
	}
	if !containsLine(data, "webhook_url = \"https://qq.example/webhook\"") {
		t.Fatalf("encoded settings missing qq webhook: %q", data)
	}
	if !containsLine(data, "type = \"lark\"") {
		t.Fatalf("encoded settings missing default lark channel: %q", data)
	}
	if !containsLine(data, "type = \"wechat\"") {
		t.Fatalf("encoded settings missing default wechat channel: %q", data)
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
