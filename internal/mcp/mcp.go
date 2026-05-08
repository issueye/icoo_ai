package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/icoo-ai/icoo-ai/internal/config"
)

const (
	TransportStdio = "stdio"
	TransportHTTP  = "http"
)

var (
	ErrUnsupportedTransport = errors.New("unsupported mcp transport")
	ErrInvalidSchema        = errors.New("invalid mcp tool input schema")
)

type ServerDefinition struct {
	Name      string            `json:"name"`
	Enabled   bool              `json:"enabled"`
	Transport string            `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
}

func ServerDefinitions(cfg config.MCPConfig) []ServerDefinition {
	if !cfg.Enabled || len(cfg.Servers) == 0 {
		return nil
	}

	names := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		names = append(names, name)
	}
	sort.Strings(names)

	defs := make([]ServerDefinition, 0, len(names))
	for _, name := range names {
		server := cfg.Servers[name]
		if !server.Enabled {
			continue
		}
		defs = append(defs, ServerDefinitionFromConfig(name, server))
	}
	return defs
}

func ServerDefinitionFromConfig(name string, server config.MCPServerConfig) ServerDefinition {
	transport := strings.TrimSpace(server.Transport)
	if transport == "" {
		if strings.TrimSpace(server.URL) != "" {
			transport = TransportHTTP
		} else {
			transport = TransportStdio
		}
	}

	return ServerDefinition{
		Name:      strings.TrimSpace(name),
		Enabled:   server.Enabled,
		Transport: strings.ToLower(transport),
		Command:   strings.TrimSpace(server.Command),
		Args:      append([]string(nil), server.Args...),
		Env:       cloneStringMap(server.Env),
		URL:       strings.TrimSpace(server.URL),
	}
}

type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

type ToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type CallResult struct {
	Content  string         `json:"content,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Error    string         `json:"error,omitempty"`
	IsError  bool           `json:"is_error,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type ResourceDefinition struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MIMEType    string `json:"mime_type,omitempty"`
}

type ResourceContent struct {
	URI      string `json:"uri"`
	MIMEType string `json:"mime_type,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

type PromptDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Arguments   json.RawMessage `json:"arguments,omitempty"`
}

type ToolClient interface {
	ListTools(ctx context.Context) ([]ToolDefinition, error)
	CallTool(ctx context.Context, call ToolCall) (CallResult, error)
}

type ResourceClient interface {
	ListResources(ctx context.Context) ([]ResourceDefinition, error)
	ReadResource(ctx context.Context, uri string) (ResourceContent, error)
}

type PromptClient interface {
	ListPrompts(ctx context.Context) ([]PromptDefinition, error)
	GetPrompt(ctx context.Context, name string, arguments map[string]any) (string, error)
}

type Client interface {
	ToolClient
	ResourceClient
	PromptClient
}

type ClientFactory interface {
	NewClient(ctx context.Context, def ServerDefinition) (Client, error)
}

type UnsupportedClientFactory struct{}

func (UnsupportedClientFactory) NewClient(_ context.Context, def ServerDefinition) (Client, error) {
	return nil, fmt.Errorf("%w %q for server %q", ErrUnsupportedTransport, def.Transport, def.Name)
}

type SchemaMapper interface {
	MapInputSchema(server ServerDefinition, tool ToolDefinition) (json.RawMessage, error)
}

type JSONSchemaMapper struct{}

func (JSONSchemaMapper) MapInputSchema(server ServerDefinition, tool ToolDefinition) (json.RawMessage, error) {
	return NormalizeInputSchema(tool.InputSchema, server.Name, tool.Name)
}

func NormalizeInputSchema(schema json.RawMessage, serverName, toolName string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(string(schema))
	if trimmed == "" {
		return json.RawMessage(`{"type":"object","additionalProperties":true}`), nil
	}
	if !json.Valid([]byte(trimmed)) {
		return nil, fmt.Errorf("%w for %s/%s", ErrInvalidSchema, serverName, toolName)
	}
	return append(json.RawMessage(nil), []byte(trimmed)...), nil
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
