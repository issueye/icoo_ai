package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
)

type Mark3LabsClientFactory struct{}

func (Mark3LabsClientFactory) NewClient(ctx context.Context, def ServerDefinition) (Client, error) {
	switch def.Transport {
	case "", TransportStdio:
		if def.Command == "" {
			return nil, fmt.Errorf("mcp stdio server %q requires command", def.Name)
		}
		client, err := mcpclient.NewStdioMCPClient(def.Command, envList(def.Env), def.Args...)
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
		return nil, fmt.Errorf("%w %q for server %q", ErrUnsupportedTransport, def.Transport, def.Name)
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
				Name:    "icoo-ai",
				Version: "0.1.0",
			},
		},
	})
	return err
}

func (c *mark3LabsClient) Close() error {
	return c.client.Close()
}

func (c *mark3LabsClient) ListTools(ctx context.Context) ([]ToolDefinition, error) {
	result, err := c.client.ListTools(ctx, mcpsdk.ListToolsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]ToolDefinition, 0, len(result.Tools))
	for _, tool := range result.Tools {
		schema := tool.RawInputSchema
		if len(schema) == 0 {
			data, err := json.Marshal(tool.InputSchema)
			if err == nil {
				schema = data
			}
		}
		out = append(out, ToolDefinition{
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
		data["structured_content"] = result.StructuredContent
	}
	return CallResult{
		Content: strings.Join(parts, "\n"),
		Data:    data,
		IsError: result.IsError,
	}, nil
}

func (c *mark3LabsClient) ListResources(ctx context.Context) ([]ResourceDefinition, error) {
	result, err := c.client.ListResources(ctx, mcpsdk.ListResourcesRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]ResourceDefinition, 0, len(result.Resources))
	for _, resource := range result.Resources {
		out = append(out, ResourceDefinition{
			URI:         resource.URI,
			Name:        resource.Name,
			Description: resource.Description,
			MIMEType:    resource.MIMEType,
		})
	}
	return out, nil
}

func (c *mark3LabsClient) ReadResource(ctx context.Context, uri string) (ResourceContent, error) {
	result, err := c.client.ReadResource(ctx, mcpsdk.ReadResourceRequest{
		Params: mcpsdk.ReadResourceParams{URI: uri},
	})
	if err != nil {
		return ResourceContent{}, err
	}
	if len(result.Contents) == 0 {
		return ResourceContent{URI: uri}, nil
	}
	content := result.Contents[0]
	if text, ok := mcpsdk.AsTextResourceContents(content); ok {
		return ResourceContent{URI: text.URI, MIMEType: text.MIMEType, Text: text.Text}, nil
	}
	if blob, ok := mcpsdk.AsBlobResourceContents(content); ok {
		return ResourceContent{URI: blob.URI, MIMEType: blob.MIMEType, Blob: []byte(blob.Blob)}, nil
	}
	return ResourceContent{URI: uri}, nil
}

func (c *mark3LabsClient) ListPrompts(ctx context.Context) ([]PromptDefinition, error) {
	result, err := c.client.ListPrompts(ctx, mcpsdk.ListPromptsRequest{})
	if err != nil {
		return nil, err
	}
	out := make([]PromptDefinition, 0, len(result.Prompts))
	for _, prompt := range result.Prompts {
		args, _ := json.Marshal(prompt.Arguments)
		out = append(out, PromptDefinition{Name: prompt.Name, Description: prompt.Description, Arguments: args})
	}
	return out, nil
}

func (c *mark3LabsClient) GetPrompt(ctx context.Context, name string, arguments map[string]any) (string, error) {
	result, err := c.client.GetPrompt(ctx, mcpsdk.GetPromptRequest{
		Params: mcpsdk.GetPromptParams{Name: name, Arguments: stringArguments(arguments)},
	})
	if err != nil {
		return "", err
	}
	var parts []string
	for _, message := range result.Messages {
		if text, ok := mcpsdk.AsTextContent(message.Content); ok {
			parts = append(parts, text.Text)
		}
	}
	return strings.Join(parts, "\n"), nil
}

func stringArguments(in map[string]any) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = fmt.Sprint(value)
	}
	return out
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}
