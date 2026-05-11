package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/icoo-ai/icoo-ai/agent_gateway/internal/security"
)

const DefaultConfigPath = "config/agent-gateway.toml"

// LoadFile reads a minimal TOML config used by gateway runtime.
// Supported keys: host, port, data_dir, auth_token.
func LoadFile(path string) (Config, error) {
	cfg := Default()

	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return Config{}, fmt.Errorf("invalid config line %d", lineNo)
		}
		key := strings.TrimSpace(parts[0])
		rawValue := strings.TrimSpace(parts[1])
		if idx := strings.Index(rawValue, "#"); idx >= 0 {
			rawValue = strings.TrimSpace(rawValue[:idx])
		}

		switch key {
		case "host":
			value, err := parseTomlString(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("invalid host at line %d: %w", lineNo, err)
			}
			cfg.Host = value
		case "port":
			value, err := strconv.Atoi(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("invalid port at line %d: %w", lineNo, err)
			}
			cfg.Port = value
		case "data_dir":
			value, err := parseTomlString(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("invalid data_dir at line %d: %w", lineNo, err)
			}
			cfg.DataDir = filepath.Clean(value)
		case "auth_token":
			value, err := parseTomlString(rawValue)
			if err != nil {
				return Config{}, fmt.Errorf("invalid auth_token at line %d: %w", lineNo, err)
			}
			cfg.AuthToken = strings.TrimSpace(value)
		default:
			return Config{}, fmt.Errorf("unsupported config key %q at line %d", key, lineNo)
		}
	}
	if err := scanner.Err(); err != nil {
		return Config{}, err
	}

	return cfg, cfg.Validate()
}

func parseTomlString(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return "", fmt.Errorf("must be a quoted string")
	}
	return raw[1 : len(raw)-1], nil
}

func SaveFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(
		"host = %q\nport = %d\ndata_dir = %q\nauth_token = %q\n",
		cfg.Host,
		cfg.Port,
		cfg.DataDir,
		strings.TrimSpace(cfg.AuthToken),
	)
	return os.WriteFile(path, []byte(content), 0o644)
}

func EnsureAuthToken(path string, cfg Config) (Config, error) {
	if strings.TrimSpace(cfg.AuthToken) != "" {
		return cfg, nil
	}
	token, err := security.GenerateToken()
	if err != nil {
		return Config{}, err
	}
	cfg.AuthToken = token
	if err := SaveFile(path, cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
