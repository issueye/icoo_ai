package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

type Mark3LabsConnector struct{}

func (Mark3LabsConnector) Connect(ctx context.Context, cfg ServerConfig) (Client, error) {
	transportType, err := cfg.ResolveTransport()
	if err != nil {
		return nil, err
	}
	switch transportType {
	case TransportStdio:
		if strings.TrimSpace(cfg.Command) == "" {
			return nil, fmt.Errorf("mcp stdio server %q requires command", cfg.Name)
		}
		env, err := cfg.Environment()
		if err != nil {
			return nil, err
		}
		client, err := mcpclient.NewStdioMCPClient(cfg.Command, envList(env), cfg.Args...)
		if err != nil {
			return nil, err
		}
		wrapped := &mark3LabsClient{client: client}
		if err := wrapped.start(ctx); err != nil {
			_ = client.Close()
			return nil, err
		}
		return wrapped, nil
	case TransportSSE:
		if strings.TrimSpace(cfg.URL) == "" {
			return nil, fmt.Errorf("mcp sse server %q requires url", cfg.Name)
		}
		client, err := mcpclient.NewSSEMCPClient(cfg.URL, mcpclient.WithHeaders(cfg.Headers))
		if err != nil {
			return nil, err
		}
		wrapped := &mark3LabsClient{client: client}
		if err := wrapped.start(ctx); err != nil {
			_ = client.Close()
			return nil, err
		}
		return wrapped, nil
	case TransportHTTP:
		if strings.TrimSpace(cfg.URL) == "" {
			return nil, fmt.Errorf("mcp http server %q requires url", cfg.Name)
		}
		client, err := mcpclient.NewStreamableHttpClient(cfg.URL, transport.WithHTTPHeaders(cfg.Headers))
		if err != nil {
			return nil, err
		}
		wrapped := &mark3LabsClient{client: client}
		if err := wrapped.start(ctx); err != nil {
			_ = client.Close()
			return nil, err
		}
		return wrapped, nil
	default:
		return nil, fmt.Errorf("%w %q for server %q", ErrUnsupportedTransport, transportType, cfg.Name)
	}
}

type mark3LabsClient struct {
	client *mcpclient.Client
}

func (c *mark3LabsClient) start(ctx context.Context) error {
	if err := c.client.Start(ctx); err != nil {
		return err
	}
	_, err := c.client.Initialize(ctx, mcpsdk.InitializeRequest{
		Params: mcpsdk.InitializeParams{
			ProtocolVersion: mcpsdk.LATEST_PROTOCOL_VERSION,
			Capabilities:    mcpsdk.ClientCapabilities{},
			ClientInfo: mcpsdk.Implementation{
				Name:    "icoo-agent-gateway",
				Version: "0.1.0-dev",
			},
		},
	})
	return err
}

func (c *mark3LabsClient) ListTools(ctx context.Context) ([]Tool, error) {
	result, err := c.client.ListTools(ctx, mcpsdk.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]Tool, 0, len(result.Tools))
	for _, tool := range result.Tools {
		schema := tool.RawInputSchema
		if len(schema) == 0 {
			data, err := json.Marshal(tool.InputSchema)
			if err == nil {
				schema = data
			}
		}
		out = append(out, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: schema,
		})
	}
	return out, nil
}

func (c *mark3LabsClient) CallTool(ctx context.Context, call ToolCall) (CallResult, error) {
	result, err := c.client.CallTool(ctx, mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name:      call.Name,
			Arguments: call.Arguments,
		},
	})
	if err != nil {
		return CallResult{}, err
	}
	var parts []string
	for _, content := range result.Content {
		if text, ok := mcpsdk.AsTextContent(content); ok {
			parts = append(parts, text.Text)
		}
	}
	data := map[string]any{}
	if result.StructuredContent != nil {
		data["structuredContent"] = result.StructuredContent
	}
	return CallResult{
		Content: strings.Join(parts, "\n"),
		Data:    data,
		IsError: result.IsError,
	}, nil
}

func (c *mark3LabsClient) Close() error {
	return c.client.Close()
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}
