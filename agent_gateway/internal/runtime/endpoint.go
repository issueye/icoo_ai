package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Endpoint struct {
	PID       int       `json:"pid"`
	BaseURL   string    `json:"baseUrl"`
	TokenFile string    `json:"tokenFile"`
	StartedAt time.Time `json:"startedAt"`
}

func DefaultDataDir() (string, error) {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "icoo-ai", "gateway"), nil
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "icoo-ai", "gateway"), nil
}

func WriteRuntimeFiles(dir string, endpoint Endpoint, token string) (Endpoint, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Endpoint{}, err
	}
	tokenFile := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenFile, []byte(token), 0o600); err != nil {
		return Endpoint{}, err
	}
	endpoint.TokenFile = tokenFile
	data, err := json.MarshalIndent(endpoint, "", "  ")
	if err != nil {
		return Endpoint{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, "endpoint.json"), append(data, '\n'), 0o600); err != nil {
		return Endpoint{}, err
	}
	return endpoint, nil
}
