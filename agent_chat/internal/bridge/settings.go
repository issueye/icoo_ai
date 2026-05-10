package bridge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type ChannelConfig struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name,omitempty"`
	Type       string `json:"type,omitempty"`
	Enabled    bool   `json:"enabled,omitempty"`
	AppID      string `json:"appId,omitempty"`
	AppSecret  string `json:"appSecret,omitempty"`
	BotToken   string `json:"botToken,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty"`
}

type AppSettings struct {
	GatewayBinaryPath string          `json:"gatewayBinaryPath,omitempty"`
	GatewayHost       string          `json:"gatewayHost,omitempty"`
	GatewayPort       int             `json:"gatewayPort,omitempty"`
	ACPEnabled        bool            `json:"acpEnabled,omitempty"`
	ACPCommand        string          `json:"acpCommand,omitempty"`
	ACPArgs           string          `json:"acpArgs,omitempty"`
	LogLevel          string          `json:"logLevel,omitempty"`
	LogFormat         string          `json:"logFormat,omitempty"`
	LogFilePath       string          `json:"logFilePath,omitempty"`
	Channels          []ChannelConfig `json:"channels,omitempty"`
}

const (
	defaultGatewayHost = "127.0.0.1"
	defaultGatewayPort = 17889
	defaultLogLevel    = "info"
	defaultLogFormat   = "text"
	defaultLogFilePath = "logs/agent_chat.log"
	channelTypeQQ      = "qq"
	channelTypeLark    = "lark"
	channelTypeWechat  = "wechat"
)

func settingsFilePath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve settings directory: %w", err)
	}
	return filepath.Join(wd, "chat.toml"), nil
}

func (s *AgentService) GetAppSettings() (AppSettings, error) {
	return loadAppSettings()
}

func (s *AgentService) UpdateAppSettings(in AppSettings) (AppSettings, error) {
	path, err := settingsFilePath()
	if err != nil {
		logger.Error("resolve settings file path failed", "error", err)
		return AppSettings{}, err
	}
	settings := normalizeAppSettings(in)
	if err := writeSettingsFile(path, settings); err != nil {
		logger.Error("write settings file failed", "path", path, "error", err)
		return AppSettings{}, err
	}
	logger.Info("settings updated",
		"path", path,
		"gatewayHost", settings.GatewayHost,
		"gatewayPort", settings.GatewayPort,
		"acpEnabled", settings.ACPEnabled,
		"acpCommand", settings.ACPCommand,
		"acpArgs", settings.ACPArgs,
		"logLevel", settings.LogLevel,
		"logFormat", settings.LogFormat,
		"logFilePath", settings.LogFilePath,
		"channels", len(settings.Channels),
	)
	return settings, nil
}

func loadAppSettings() (AppSettings, error) {
	path, err := settingsFilePath()
	if err != nil {
		logger.Error("resolve settings file path failed", "error", err)
		return AppSettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings := normalizeAppSettings(AppSettings{})
			if writeErr := writeSettingsFile(path, settings); writeErr != nil {
				logger.Error("initialize settings file failed", "path", path, "error", writeErr)
				return AppSettings{}, writeErr
			}
			logger.Info("initialized settings file with defaults", "path", path)
			return settings, nil
		}
		logger.Error("read settings file failed", "path", path, "error", err)
		return AppSettings{}, fmt.Errorf("read app settings: %w", err)
	}
	settings, err := decodeSettingsTOML(data)
	if err != nil {
		logger.Error("decode settings file failed", "path", path, "error", err)
		return AppSettings{}, fmt.Errorf("decode app settings: %w", err)
	}
	normalized := normalizeAppSettings(settings)
	logger.Debug("settings loaded",
		"path", path,
		"gatewayHost", normalized.GatewayHost,
		"gatewayPort", normalized.GatewayPort,
		"acpEnabled", normalized.ACPEnabled,
		"acpCommand", normalized.ACPCommand,
		"acpArgs", normalized.ACPArgs,
		"logLevel", normalized.LogLevel,
		"logFormat", normalized.LogFormat,
		"logFilePath", normalized.LogFilePath,
		"channels", len(normalized.Channels),
	)
	return normalized, nil
}

func decodeSettingsTOML(data []byte) (AppSettings, error) {
	settings := AppSettings{}
	currentChannelIndex := -1
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		commentIndex := strings.Index(line, "#")
		if commentIndex >= 0 {
			line = strings.TrimSpace(line[:commentIndex])
			if line == "" {
				continue
			}
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if line == "[[channels]]" {
				settings.Channels = append(settings.Channels, ChannelConfig{})
				currentChannelIndex = len(settings.Channels) - 1
			} else {
				currentChannelIndex = -1
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if currentChannelIndex >= 0 {
			channel := settings.Channels[currentChannelIndex]
			handled := true
			switch key {
			case "id":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.ID = strings.TrimSpace(unquoted)
			case "name":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.Name = strings.TrimSpace(unquoted)
			case "type":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.Type = strings.TrimSpace(unquoted)
			case "enabled":
				parsed, err := strconv.ParseBool(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.Enabled = parsed
			case "app_id":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.AppID = strings.TrimSpace(unquoted)
			case "app_secret":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.AppSecret = strings.TrimSpace(unquoted)
			case "bot_token":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.BotToken = strings.TrimSpace(unquoted)
			case "webhook_url":
				unquoted, err := strconv.Unquote(value)
				if err != nil {
					return AppSettings{}, err
				}
				channel.WebhookURL = strings.TrimSpace(unquoted)
			default:
				handled = false
			}
			if handled {
				settings.Channels[currentChannelIndex] = channel
				continue
			}
		}
		switch key {
		case "gateway_binary_path":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.GatewayBinaryPath = strings.TrimSpace(unquoted)
		case "gateway_host":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.GatewayHost = strings.TrimSpace(unquoted)
		case "gateway_port":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.GatewayPort = parsed
		case "acp_enabled":
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.ACPEnabled = parsed
		case "acp_command":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.ACPCommand = strings.TrimSpace(unquoted)
		case "acp_args":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.ACPArgs = strings.TrimSpace(unquoted)
		case "log_level":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.LogLevel = strings.TrimSpace(unquoted)
		case "log_format":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.LogFormat = strings.TrimSpace(unquoted)
		case "log_file_path":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.LogFilePath = strings.TrimSpace(unquoted)
		}
	}
	if err := scanner.Err(); err != nil {
		return AppSettings{}, err
	}
	return settings, nil
}

func encodeSettingsTOML(settings AppSettings) []byte {
	normalized := normalizeAppSettings(settings)
	var builder strings.Builder
	fmt.Fprintf(&builder, "gateway_binary_path = %s\n", strconv.Quote(normalized.GatewayBinaryPath))
	fmt.Fprintf(&builder, "gateway_host = %s\n", strconv.Quote(normalized.GatewayHost))
	fmt.Fprintf(&builder, "gateway_port = %d\n", normalized.GatewayPort)
	fmt.Fprintf(&builder, "acp_enabled = %t\n", normalized.ACPEnabled)
	fmt.Fprintf(&builder, "acp_command = %s\n", strconv.Quote(normalized.ACPCommand))
	fmt.Fprintf(&builder, "acp_args = %s\n", strconv.Quote(normalized.ACPArgs))
	fmt.Fprintf(&builder, "log_level = %s\n", strconv.Quote(normalized.LogLevel))
	fmt.Fprintf(&builder, "log_format = %s\n", strconv.Quote(normalized.LogFormat))
	fmt.Fprintf(&builder, "log_file_path = %s\n", strconv.Quote(normalized.LogFilePath))
	for _, channel := range normalized.Channels {
		builder.WriteString("\n[[channels]]\n")
		fmt.Fprintf(&builder, "id = %s\n", strconv.Quote(channel.ID))
		fmt.Fprintf(&builder, "name = %s\n", strconv.Quote(channel.Name))
		fmt.Fprintf(&builder, "type = %s\n", strconv.Quote(channel.Type))
		fmt.Fprintf(&builder, "enabled = %t\n", channel.Enabled)
		fmt.Fprintf(&builder, "app_id = %s\n", strconv.Quote(channel.AppID))
		fmt.Fprintf(&builder, "app_secret = %s\n", strconv.Quote(channel.AppSecret))
		fmt.Fprintf(&builder, "bot_token = %s\n", strconv.Quote(channel.BotToken))
		fmt.Fprintf(&builder, "webhook_url = %s\n", strconv.Quote(channel.WebhookURL))
	}
	return []byte(builder.String())
}

func writeSettingsFile(path string, settings AppSettings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}
	data := encodeSettingsTOML(settings)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write app settings: %w", err)
	}
	return nil
}

func normalizeAppSettings(in AppSettings) AppSettings {
	settings := AppSettings{
		GatewayBinaryPath: strings.TrimSpace(in.GatewayBinaryPath),
		GatewayHost:       strings.TrimSpace(in.GatewayHost),
		GatewayPort:       in.GatewayPort,
		ACPEnabled:        in.ACPEnabled,
		ACPCommand:        strings.TrimSpace(in.ACPCommand),
		ACPArgs:           strings.TrimSpace(in.ACPArgs),
		LogLevel:          strings.TrimSpace(in.LogLevel),
		LogFormat:         strings.TrimSpace(in.LogFormat),
		LogFilePath:       strings.TrimSpace(in.LogFilePath),
		Channels:          normalizeChannels(in.Channels),
	}
	if settings.GatewayHost == "" {
		settings.GatewayHost = defaultGatewayHost
	}
	if settings.GatewayPort <= 0 || settings.GatewayPort > 65535 {
		settings.GatewayPort = defaultGatewayPort
	}
	if settings.LogLevel == "" {
		settings.LogLevel = defaultLogLevel
	}
	switch strings.ToLower(settings.LogLevel) {
	case "debug", "info", "warn", "warning", "error":
	default:
		settings.LogLevel = defaultLogLevel
	}
	if settings.LogFormat == "" {
		settings.LogFormat = defaultLogFormat
	}
	switch strings.ToLower(settings.LogFormat) {
	case "text", "json":
	default:
		settings.LogFormat = defaultLogFormat
	}
	if settings.LogFilePath == "" {
		settings.LogFilePath = defaultLogFilePath
	}
	return settings
}

func defaultChannelConfigs() []ChannelConfig {
	return []ChannelConfig{
		{
			ID:   channelTypeQQ,
			Name: "QQ机器人",
			Type: channelTypeQQ,
		},
		{
			ID:   channelTypeLark,
			Name: "飞书机器人",
			Type: channelTypeLark,
		},
		{
			ID:   channelTypeWechat,
			Name: "微信机器人",
			Type: channelTypeWechat,
		},
	}
}

func normalizeChannelType(raw string, fallback string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case channelTypeQQ, channelTypeLark, channelTypeWechat:
		return normalized
	}
	fallback = strings.ToLower(strings.TrimSpace(fallback))
	switch fallback {
	case channelTypeQQ, channelTypeLark, channelTypeWechat:
		return fallback
	}
	return channelTypeQQ
}

func channelConfigForType(channelType string) ChannelConfig {
	switch normalizeChannelType(channelType, channelTypeQQ) {
	case channelTypeLark:
		return ChannelConfig{
			ID:   channelTypeLark,
			Name: "飞书机器人",
			Type: channelTypeLark,
		}
	case channelTypeWechat:
		return ChannelConfig{
			ID:   channelTypeWechat,
			Name: "微信机器人",
			Type: channelTypeWechat,
		}
	default:
		return ChannelConfig{
			ID:   channelTypeQQ,
			Name: "QQ机器人",
			Type: channelTypeQQ,
		}
	}
}

func normalizeChannelConfig(in ChannelConfig, fallbackType string) ChannelConfig {
	channelType := normalizeChannelType(in.Type, fallbackType)
	defaults := channelConfigForType(channelType)
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = defaults.ID
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = defaults.Name
	}
	return ChannelConfig{
		ID:         id,
		Name:       name,
		Type:       channelType,
		Enabled:    in.Enabled,
		AppID:      strings.TrimSpace(in.AppID),
		AppSecret:  strings.TrimSpace(in.AppSecret),
		BotToken:   strings.TrimSpace(in.BotToken),
		WebhookURL: strings.TrimSpace(in.WebhookURL),
	}
}

func normalizeChannels(in []ChannelConfig) []ChannelConfig {
	defaults := defaultChannelConfigs()
	if len(in) == 0 {
		return defaults
	}
	channelsByType := map[string]ChannelConfig{}
	for index, channel := range in {
		fallbackType := ""
		if index < len(defaults) {
			fallbackType = defaults[index].Type
		}
		normalized := normalizeChannelConfig(channel, fallbackType)
		if _, exists := channelsByType[normalized.Type]; exists {
			continue
		}
		channelsByType[normalized.Type] = normalized
	}
	normalizedChannels := make([]ChannelConfig, 0, len(defaults))
	for _, fallback := range defaults {
		if channel, ok := channelsByType[fallback.Type]; ok {
			normalizedChannels = append(normalizedChannels, channel)
			continue
		}
		normalizedChannels = append(normalizedChannels, fallback)
	}
	return normalizedChannels
}
