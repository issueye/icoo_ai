package bridge

import (
	"bufio"
	"context"
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

type MCPServerConfig struct {
	ID      string   `json:"id,omitempty"`
	Name    string   `json:"name,omitempty"`
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Enabled bool     `json:"enabled,omitempty"`
}

type ScheduleTaskConfig struct {
	ID      string `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Spec    string `json:"spec,omitempty"`
	Content string `json:"content,omitempty"`
	Enabled bool   `json:"enabled,omitempty"`
}

type AgentConfig struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Protocol    string   `json:"protocol,omitempty"`
	Description string   `json:"description,omitempty"`
	Models      []string `json:"models,omitempty"`
	Enabled     bool     `json:"enabled,omitempty"`
}

type AppSettings struct {
	GatewayBinaryPath string               `json:"gatewayBinaryPath,omitempty"`
	GatewayHost       string               `json:"gatewayHost,omitempty"`
	GatewayPort       int                  `json:"gatewayPort,omitempty"`
	GatewayToken      string               `json:"gatewayToken,omitempty"`
	LogLevel          string               `json:"logLevel,omitempty"`
	LogFormat         string               `json:"logFormat,omitempty"`
	LogFilePath       string               `json:"logFilePath,omitempty"`
	Channels          []ChannelConfig      `json:"channels,omitempty"`
	Agents            []AgentConfig        `json:"agents,omitempty"`
	MCPServers        []MCPServerConfig    `json:"mcpServers,omitempty"`
	ScheduleTasks     []ScheduleTaskConfig `json:"scheduleTasks,omitempty"`
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
	settings, err := loadAppSettings()
	if err != nil {
		return AppSettings{}, err
	}
	if s == nil {
		return settings, nil
	}
	remote, remoteErr := s.fetchGatewayManagementSettings(context.Background())
	if remoteErr != nil {
		return AppSettings{}, remoteErr
	}
	settings.Channels = normalizeChannels(remote.Channels)
	settings.Agents = normalizeAgents(remote.Agents)
	settings.MCPServers = normalizeMCPServers(remote.MCPServers)
	settings.ScheduleTasks = normalizeScheduleTasks(remote.ScheduleTasks)
	return settings, nil
}

func (s *AgentService) UpdateAppSettings(in AppSettings) (AppSettings, error) {
	path, err := settingsFilePath()
	if err != nil {
		logger.Error("resolve settings file path failed", "error", err)
		return AppSettings{}, err
	}
	settings := normalizeAppSettings(in)
	if s != nil {
		remote, remoteErr := s.updateGatewayManagementSettings(context.Background(), gatewayManagementSettingsPayload{
			Channels:      settings.Channels,
			Agents:        settings.Agents,
			MCPServers:    settings.MCPServers,
			ScheduleTasks: settings.ScheduleTasks,
		})
		if remoteErr != nil {
			logger.Error("update management settings through gateway failed", "error", remoteErr)
			return AppSettings{}, remoteErr
		}
		settings.Channels = normalizeChannels(remote.Channels)
		settings.Agents = normalizeAgents(remote.Agents)
		settings.MCPServers = normalizeMCPServers(remote.MCPServers)
		settings.ScheduleTasks = normalizeScheduleTasks(remote.ScheduleTasks)
	}
	if err := writeSettingsFile(path, settings); err != nil {
		logger.Error("write settings file failed", "path", path, "error", err)
		return AppSettings{}, err
	}
	logger.Info("settings updated",
		"path", path,
		"gatewayHost", settings.GatewayHost,
		"gatewayPort", settings.GatewayPort,
		"logLevel", settings.LogLevel,
		"logFormat", settings.LogFormat,
		"logFilePath", settings.LogFilePath,
		"channels", len(settings.Channels),
		"mcpServers", len(settings.MCPServers),
		"scheduleTasks", len(settings.ScheduleTasks),
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
		"logLevel", normalized.LogLevel,
		"logFormat", normalized.LogFormat,
		"logFilePath", normalized.LogFilePath,
		"channels", len(normalized.Channels),
		"mcpServers", len(normalized.MCPServers),
		"scheduleTasks", len(normalized.ScheduleTasks),
	)
	return normalized, nil
}

type gatewayManagementSettingsPayload struct {
	Channels      []ChannelConfig      `json:"channels,omitempty"`
	MCPServers    []MCPServerConfig    `json:"mcpServers,omitempty"`
	ScheduleTasks []ScheduleTaskConfig `json:"scheduleTasks,omitempty"`
	Agents        []AgentConfig        `json:"agents,omitempty"`
}

func (s *AgentService) fetchGatewayManagementSettings(ctx context.Context) (gatewayManagementSettingsPayload, error) {
	var out gatewayManagementSettingsPayload
	if err := s.gatewayJSON(ctx, "GET", "/v1/management/settings", nil, &out); err != nil {
		return gatewayManagementSettingsPayload{}, err
	}
	return out, nil
}

func (s *AgentService) updateGatewayManagementSettings(ctx context.Context, payload gatewayManagementSettingsPayload) (gatewayManagementSettingsPayload, error) {
	var out gatewayManagementSettingsPayload
	if err := s.gatewayJSON(ctx, "PUT", "/v1/management/settings", payload, &out); err != nil {
		return gatewayManagementSettingsPayload{}, err
	}
	return out, nil
}

func decodeSettingsTOML(data []byte) (AppSettings, error) {
	settings := AppSettings{}
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
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
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
		case "gateway_token":
			unquoted, err := strconv.Unquote(value)
			if err != nil {
				return AppSettings{}, err
			}
			settings.GatewayToken = strings.TrimSpace(unquoted)
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
	fmt.Fprintf(&builder, "gateway_token = %s\n", strconv.Quote(normalized.GatewayToken))
	fmt.Fprintf(&builder, "log_level = %s\n", strconv.Quote(normalized.LogLevel))
	fmt.Fprintf(&builder, "log_format = %s\n", strconv.Quote(normalized.LogFormat))
	fmt.Fprintf(&builder, "log_file_path = %s\n", strconv.Quote(normalized.LogFilePath))
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
		GatewayToken:      strings.TrimSpace(in.GatewayToken),
		LogLevel:          strings.TrimSpace(in.LogLevel),
		LogFormat:         strings.TrimSpace(in.LogFormat),
		LogFilePath:       strings.TrimSpace(in.LogFilePath),
		Channels:          normalizeChannels(in.Channels),
		Agents:            normalizeAgents(in.Agents),
		MCPServers:        normalizeMCPServers(in.MCPServers),
		ScheduleTasks:     normalizeScheduleTasks(in.ScheduleTasks),
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

func parseTOMLStringArray(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return []string{}, nil
	}
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return nil, fmt.Errorf("invalid toml array: %s", raw)
	}
	body := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if body == "" {
		return []string{}, nil
	}
	parts := strings.Split(body, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		unquoted, err := strconv.Unquote(item)
		if err != nil {
			return nil, err
		}
		result = append(result, strings.TrimSpace(unquoted))
	}
	return result, nil
}

func encodeTOMLStringArray(items []string) string {
	if len(items) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, strconv.Quote(strings.TrimSpace(item)))
	}
	return "[" + strings.Join(parts, ", ") + "]"
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

func ensureUniqueChannelID(rawID string, channelType string, used map[string]int) string {
	baseID := strings.TrimSpace(rawID)
	if baseID == "" {
		baseID = strings.TrimSpace(channelType)
	}
	if baseID == "" {
		baseID = channelTypeQQ
	}
	if _, exists := used[baseID]; !exists {
		used[baseID] = 1
		return baseID
	}
	for suffix := used[baseID] + 1; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", baseID, suffix)
		if _, exists := used[candidate]; exists {
			continue
		}
		used[baseID] = suffix
		used[candidate] = 1
		return candidate
	}
}

func normalizeChannels(in []ChannelConfig) []ChannelConfig {
	defaults := defaultChannelConfigs()
	if len(in) == 0 {
		return []ChannelConfig{}
	}
	usedIDs := map[string]int{}
	normalizedChannels := make([]ChannelConfig, 0, len(in))
	for index, channel := range in {
		fallbackType := ""
		if index < len(defaults) {
			fallbackType = defaults[index].Type
		}
		normalized := normalizeChannelConfig(channel, fallbackType)
		normalized.ID = ensureUniqueChannelID(normalized.ID, normalized.Type, usedIDs)
		normalizedChannels = append(normalizedChannels, normalized)
	}
	return normalizedChannels
}

func normalizeMCPServers(in []MCPServerConfig) []MCPServerConfig {
	if len(in) == 0 {
		return []MCPServerConfig{}
	}
	used := map[string]int{}
	normalized := make([]MCPServerConfig, 0, len(in))
	for index, server := range in {
		id := strings.TrimSpace(server.ID)
		if id == "" {
			id = fmt.Sprintf("mcp_%d", index+1)
		}
		id = ensureUniqueChannelID(id, "mcp", used)
		name := strings.TrimSpace(server.Name)
		if name == "" {
			name = id
		}
		args := make([]string, 0, len(server.Args))
		for _, arg := range server.Args {
			trimmed := strings.TrimSpace(arg)
			if trimmed == "" {
				continue
			}
			args = append(args, trimmed)
		}
		normalized = append(normalized, MCPServerConfig{
			ID:      id,
			Name:    name,
			Command: strings.TrimSpace(server.Command),
			Args:    args,
			Enabled: server.Enabled,
		})
	}
	return normalized
}

func normalizeAgents(in []AgentConfig) []AgentConfig {
	if len(in) == 0 {
		return []AgentConfig{}
	}
	used := map[string]int{}
	normalized := make([]AgentConfig, 0, len(in))
	for index, agent := range in {
		id := strings.TrimSpace(agent.ID)
		if id == "" {
			id = fmt.Sprintf("agent_%d", index+1)
		}
		id = ensureUniqueChannelID(id, "agent", used)
		name := strings.TrimSpace(agent.Name)
		if name == "" {
			name = id
		}
		models := make([]string, 0, len(agent.Models))
		for _, model := range agent.Models {
			text := strings.TrimSpace(model)
			if text == "" {
				continue
			}
			models = append(models, text)
		}
		normalized = append(normalized, AgentConfig{
			ID:          id,
			Name:        name,
			Protocol:    strings.TrimSpace(agent.Protocol),
			Description: strings.TrimSpace(agent.Description),
			Models:      models,
			Enabled:     agent.Enabled,
		})
	}
	return normalized
}

func normalizeScheduleTasks(in []ScheduleTaskConfig) []ScheduleTaskConfig {
	if len(in) == 0 {
		return []ScheduleTaskConfig{}
	}
	used := map[string]int{}
	normalized := make([]ScheduleTaskConfig, 0, len(in))
	for index, task := range in {
		id := strings.TrimSpace(task.ID)
		if id == "" {
			id = fmt.Sprintf("task_%d", index+1)
		}
		id = ensureUniqueChannelID(id, "task", used)
		name := strings.TrimSpace(task.Name)
		if name == "" {
			name = id
		}
		spec := strings.TrimSpace(task.Spec)
		if spec == "" {
			spec = "*/5 * * * *"
		}
		normalized = append(normalized, ScheduleTaskConfig{
			ID:      id,
			Name:    name,
			Spec:    spec,
			Content: strings.TrimSpace(task.Content),
			Enabled: task.Enabled,
		})
	}
	return normalized
}
