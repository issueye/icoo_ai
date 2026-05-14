package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TransportType identifies the wire transport used to reach an MCP server.
// The concrete SDK binding is intentionally outside this package for now.
type TransportType string

const (
	TransportAuto  TransportType = ""
	TransportStdio TransportType = "stdio"
	TransportSSE   TransportType = "sse"
	TransportHTTP  TransportType = "http"
)

// ServerConfig is the runtime-facing MCP server configuration.
//
// It is deliberately broader than the current persistence model so the runtime
// boundary can stay stable while database fields are added in later milestones.
type ServerConfig struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Type    TransportType     `json:"type,omitempty"`
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`
	CWD     string            `json:"cwd,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	EnvFile string            `json:"envFile,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ServerKey returns the stable identity used by the manager.
func (c ServerConfig) ServerKey() string {
	if strings.TrimSpace(c.ID) != "" {
		return strings.TrimSpace(c.ID)
	}
	return strings.TrimSpace(c.Name)
}

// ResolveTransport returns the explicit or inferred transport type.
func (c ServerConfig) ResolveTransport() (TransportType, error) {
	if c.Type != TransportAuto {
		switch c.Type {
		case TransportStdio, TransportSSE, TransportHTTP:
			return c.Type, nil
		default:
			return "", fmt.Errorf("unsupported MCP transport type %q", c.Type)
		}
	}

	if strings.TrimSpace(c.URL) != "" {
		return TransportSSE, nil
	}
	if strings.TrimSpace(c.Command) != "" {
		return TransportStdio, nil
	}
	return "", fmt.Errorf("either url or command is required")
}

func (c ServerConfig) Normalized() (ServerConfig, error) {
	if strings.HasPrefix(c.Command, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, err
		}
		switch c.Command {
		case "~":
			c.Command = home
		default:
			next := strings.TrimPrefix(c.Command, "~")
			next = strings.TrimPrefix(next, string(filepath.Separator))
			next = strings.TrimPrefix(next, "/")
			c.Command = filepath.Join(home, next)
		}
	}
	return c, nil
}

// Environment builds the process environment for stdio transports. Values in
// Env override values loaded from EnvFile.
func (c ServerConfig) Environment() (map[string]string, error) {
	env := make(map[string]string)
	if c.EnvFile != "" {
		fromFile, err := loadEnvFile(c.EnvFile)
		if err != nil {
			return nil, err
		}
		for key, value := range fromFile {
			env[key] = value
		}
	}
	for key, value := range c.Env {
		env[key] = value
	}
	return env, nil
}

// Tool describes an MCP tool discovered from a server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
}

// State is the runtime connection state for a configured MCP server.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateFailed       State = "failed"
	StateDisabled     State = "disabled"
	StateClosed       State = "closed"
)

// ServerStatus is a point-in-time snapshot of a server connection.
type ServerStatus struct {
	ID          string    `json:"id"`
	Name        string    `json:"name,omitempty"`
	State       State     `json:"state"`
	Transport   string    `json:"transport,omitempty"`
	ToolCount   int       `json:"toolCount"`
	LastError   string    `json:"lastError,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
	ConnectedAt time.Time `json:"connectedAt,omitempty"`
}

func loadEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open env file %s: %w", path, err)
	}
	defer file.Close()

	out := make(map[string]string)
	scanner := bufio.NewScanner(file)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		key, value, ok := strings.Cut(text, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid env file %s line %d", path, line)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		out[strings.TrimSpace(key)] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read env file %s: %w", path, err)
	}
	return out, nil
}
