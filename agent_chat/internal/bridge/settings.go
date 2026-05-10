package bridge

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type AppSettings struct {
	GatewayBinaryPath string `json:"gatewayBinaryPath,omitempty"`
	GatewayHost       string `json:"gatewayHost,omitempty"`
	GatewayPort       int    `json:"gatewayPort,omitempty"`
	ACPEnabled        bool   `json:"acpEnabled,omitempty"`
	ACPCommand        string `json:"acpCommand,omitempty"`
	ACPArgs           string `json:"acpArgs,omitempty"`
	LogLevel          string `json:"logLevel,omitempty"`
	LogFormat         string `json:"logFormat,omitempty"`
	LogFilePath       string `json:"logFilePath,omitempty"`
}

const (
	defaultGatewayHost = "127.0.0.1"
	defaultGatewayPort = 17889
	defaultLogLevel    = "info"
	defaultLogFormat   = "text"
	defaultLogFilePath = "logs/agent_chat.log"
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
	)
	return normalized, nil
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
	data := []byte(
		fmt.Sprintf(
			"gateway_binary_path = %s\ngateway_host = %s\ngateway_port = %d",
			strconv.Quote(settings.GatewayBinaryPath),
			strconv.Quote(settings.GatewayHost),
			settings.GatewayPort,
		),
	)
	data = append(data, '\n')
	data = append(data, []byte(fmt.Sprintf("acp_enabled = %t", settings.ACPEnabled))...)
	data = append(data, '\n')
	data = append(data, []byte(fmt.Sprintf("acp_command = %s", strconv.Quote(settings.ACPCommand)))...)
	data = append(data, '\n')
	data = append(data, []byte(fmt.Sprintf("acp_args = %s", strconv.Quote(settings.ACPArgs)))...)
	data = append(data, '\n')
	data = append(data, []byte(fmt.Sprintf("log_level = %s", strconv.Quote(settings.LogLevel)))...)
	data = append(data, '\n')
	data = append(data, []byte(fmt.Sprintf("log_format = %s", strconv.Quote(settings.LogFormat)))...)
	data = append(data, '\n')
	data = append(data, []byte(fmt.Sprintf("log_file_path = %s", strconv.Quote(settings.LogFilePath)))...)
	return data
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
