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
}

func (s *AgentService) settingsFilePath() (string, error) {
	return settingsFilePath()
}

func settingsFilePath() (string, error) {
	if exe, err := os.Executable(); err == nil && strings.TrimSpace(exe) != "" {
		exeDir := filepath.Dir(exe)
		if !isTemporaryRuntimeDir(exeDir) {
			return filepath.Join(exeDir, "chat.toml"), nil
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve settings directory: %w", err)
	}
	return filepath.Join(wd, "chat.toml"), nil
}

func isTemporaryRuntimeDir(dir string) bool {
	cleanDir := strings.ToLower(filepath.Clean(dir))
	cleanTemp := strings.ToLower(filepath.Clean(os.TempDir()))
	if cleanTemp != "" && strings.HasPrefix(cleanDir, cleanTemp) {
		return true
	}
	return strings.Contains(cleanDir, string(filepath.Separator)+"go-build"+string(filepath.Separator))
}

func (s *AgentService) GetAppSettings() (AppSettings, error) {
	return loadAppSettings()
}

func (s *AgentService) UpdateAppSettings(in AppSettings) (AppSettings, error) {
	path, err := settingsFilePath()
	if err != nil {
		return AppSettings{}, err
	}
	settings := AppSettings{
		GatewayBinaryPath: strings.TrimSpace(in.GatewayBinaryPath),
	}
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
			settings := AppSettings{}
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
	return settings, nil
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
		if key != "gateway_binary_path" {
			continue
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return AppSettings{}, err
		}
		settings.GatewayBinaryPath = strings.TrimSpace(unquoted)
	}
	if err := scanner.Err(); err != nil {
		return AppSettings{}, err
	}
	return settings, nil
}

func encodeSettingsTOML(settings AppSettings) []byte {
	return []byte(fmt.Sprintf("gateway_binary_path = %s", strconv.Quote(settings.GatewayBinaryPath)))
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
