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
}

const (
	defaultGatewayHost = "127.0.0.1"
	defaultGatewayPort = 17889
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
		return AppSettings{}, err
	}
	settings := normalizeAppSettings(in)
	if err := writeSettingsFile(path, settings); err != nil {
		return AppSettings{}, err
	}
	return settings, nil
}

func loadAppSettings() (AppSettings, error) {
	path, err := settingsFilePath()
	if err != nil {
		return AppSettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			settings := normalizeAppSettings(AppSettings{})
			if writeErr := writeSettingsFile(path, settings); writeErr != nil {
				return AppSettings{}, writeErr
			}
			return settings, nil
		}
		return AppSettings{}, fmt.Errorf("read app settings: %w", err)
	}
	settings, err := decodeSettingsTOML(data)
	if err != nil {
		return AppSettings{}, fmt.Errorf("decode app settings: %w", err)
	}
	return normalizeAppSettings(settings), nil
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
		}
	}
	if err := scanner.Err(); err != nil {
		return AppSettings{}, err
	}
	return settings, nil
}

func encodeSettingsTOML(settings AppSettings) []byte {
	return []byte(
		fmt.Sprintf(
			"gateway_binary_path = %s\ngateway_host = %s\ngateway_port = %d",
			strconv.Quote(settings.GatewayBinaryPath),
			strconv.Quote(settings.GatewayHost),
			settings.GatewayPort,
		),
	)
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
	}
	if settings.GatewayHost == "" {
		settings.GatewayHost = defaultGatewayHost
	}
	if settings.GatewayPort <= 0 || settings.GatewayPort > 65535 {
		settings.GatewayPort = defaultGatewayPort
	}
	return settings
}
