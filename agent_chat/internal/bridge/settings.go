package bridge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_chat/internal/gatewayclient"
)

type AppSettings struct {
	GatewayBinaryPath string `json:"gatewayBinaryPath,omitempty"`
}

func (s *AgentService) settingsFilePath() (string, error) {
	dir, err := gatewayclient.DefaultDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "agent_chat.settings.json"), nil
}

func (s *AgentService) GetAppSettings() (AppSettings, error) {
	path, err := s.settingsFilePath()
	if err != nil {
		return AppSettings{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AppSettings{}, nil
		}
		return AppSettings{}, fmt.Errorf("read app settings: %w", err)
	}
	var settings AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return AppSettings{}, fmt.Errorf("decode app settings: %w", err)
	}
	return settings, nil
}

func (s *AgentService) UpdateAppSettings(in AppSettings) (AppSettings, error) {
	path, err := s.settingsFilePath()
	if err != nil {
		return AppSettings{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return AppSettings{}, fmt.Errorf("create settings dir: %w", err)
	}
	settings := AppSettings{
		GatewayBinaryPath: strings.TrimSpace(in.GatewayBinaryPath),
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return AppSettings{}, fmt.Errorf("encode app settings: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return AppSettings{}, fmt.Errorf("write app settings: %w", err)
	}
	if settings.GatewayBinaryPath == "" {
		_ = os.Unsetenv("ICOO_GATEWAY_BIN")
	} else {
		_ = os.Setenv("ICOO_GATEWAY_BIN", settings.GatewayBinaryPath)
	}
	return settings, nil
}
