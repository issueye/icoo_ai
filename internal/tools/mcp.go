package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/config"
	"github.com/icoo-ai/icoo-ai/internal/mcp"
	"github.com/icoo-ai/icoo-ai/internal/policy"
)

type MCPToolOptions struct {
	Config       config.MCPConfig
	Servers      []mcp.ServerDefinition
	Factory      mcp.ClientFactory
	SchemaMapper mcp.SchemaMapper
	Policy       policy.Policy
	AuditLogger  audit.Logger
	Now          func() time.Time
}

func NewMCPTools(ctx context.Context, opts MCPToolOptions) ([]Tool, error) {
	servers := opts.Servers
	if servers == nil {
		servers = mcp.ServerDefinitions(opts.Config)
	}
	if len(servers) == 0 {
		return nil, nil
	}

	factory := opts.Factory
	if factory == nil {
		factory = mcp.UnsupportedClientFactory{}
	}
	mapper := opts.SchemaMapper
	if mapper == nil {
		mapper = mcp.JSONSchemaMapper{}
	}
	p := opts.Policy
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	out := make([]Tool, 0)
	seen := map[string]string{}
	for _, server := range servers {
		if strings.TrimSpace(server.Name) == "" || !server.Enabled {
			continue
		}
		client, err := factory.NewClient(ctx, server)
		if err != nil {
			return nil, err
		}
		defs, err := client.ListTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("list mcp tools for %s: %w", server.Name, err)
		}
		sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
		for _, def := range defs {
			def.Name = strings.TrimSpace(def.Name)
			if def.Name == "" {
				continue
			}
			toolName := MCPToolName(server.Name, def.Name)
			if owner, ok := seen[toolName]; ok {
				return nil, fmt.Errorf("mcp tool name conflict %q from %s and %s", toolName, owner, server.Name)
			}
			schema, err := mapper.MapInputSchema(server, def)
			if err != nil {
				return nil, err
			}
			def.InputSchema = schema
			out = append(out, mcpTool{
				name:        toolName,
				server:      server,
				definition:  def,
				client:      client,
				policy:      p,
				auditLogger: opts.AuditLogger,
				now:         now,
			})
			seen[toolName] = server.Name
		}
	}
	return out, nil
}

func MCPToolName(serverName, toolName string) string {
	return "mcp__" + sanitizeToolName(serverName) + "__" + sanitizeToolName(toolName)
}

type mcpTool struct {
	name        string
	server      mcp.ServerDefinition
	definition  mcp.ToolDefinition
	client      mcp.ToolClient
	policy      policy.Policy
	auditLogger audit.Logger
	now         func() time.Time
}

func (t mcpTool) Name() string { return t.name }

func (t mcpTool) Description() string {
	if strings.TrimSpace(t.definition.Description) != "" {
		return t.definition.Description
	}
	return "MCP tool " + t.definition.Name + " from server " + t.server.Name + "."
}

func (t mcpTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: append(json.RawMessage(nil), t.definition.InputSchema...),
	}
}

func (t mcpTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	args := map[string]any{}
	if len(strings.TrimSpace(string(input))) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return toolError("invalid_json", err.Error(), nil), nil
		}
	}

	startedAt := t.now().UTC()
	req := policy.MCPRequest{
		Server:    t.server.Name,
		Name:      t.definition.Name,
		Kind:      "tool",
		Arguments: args,
	}
	decision := t.policy.EvaluateMCP(req)
	if decision.Action == policy.DecisionBlock {
		_ = t.logMCP(ctx, startedAt, false, decision.Reason, decision)
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}

	result, err := t.client.CallTool(ctx, mcp.ToolCall{Name: t.definition.Name, Arguments: args})
	if err != nil {
		_ = t.logMCP(ctx, startedAt, false, err.Error(), decision)
		return toolError("mcp_call_failed", err.Error(), map[string]any{
			"server": t.server.Name,
			"name":   t.definition.Name,
		}), nil
	}

	toolResult := mcpCallResultToToolResult(t.server, t.definition, result)
	if !toolResult.OK && toolResult.Error == "" {
		toolResult.Error = "mcp tool returned an error"
	}
	_ = t.logMCP(ctx, startedAt, toolResult.OK, toolResult.Error, decision)
	return toolResult, nil
}

func (t mcpTool) logMCP(ctx context.Context, at time.Time, ok bool, errText string, decision policy.Decision) error {
	if t.auditLogger == nil {
		return nil
	}
	data := map[string]any{
		"server": t.server.Name,
		"name":   t.definition.Name,
		"kind":   "tool",
		"ok":     ok,
	}
	if errText != "" {
		data["error"] = errText
	}
	if decision.Action != "" {
		data["policy_decision"] = decision
	}
	return t.auditLogger.Log(ctx, audit.Event{
		Type:      audit.EventMCPCall,
		Timestamp: at.UTC(),
		Summary:   "mcp " + t.server.Name + "/" + t.definition.Name,
		Data:      data,
	})
}

func mcpCallResultToToolResult(server mcp.ServerDefinition, def mcp.ToolDefinition, result mcp.CallResult) ToolResult {
	data := cloneAnyMap(result.Data)
	if data == nil {
		data = map[string]any{}
	}
	data["server"] = server.Name
	data["name"] = def.Name

	metadata := cloneAnyMap(result.Metadata)
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["server"] = server.Name
	metadata["name"] = def.Name

	return ToolResult{
		OK:       !result.IsError && result.Error == "",
		Content:  result.Content,
		Data:     data,
		Error:    result.Error,
		Metadata: metadata,
	}
}

func sanitizeToolName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unnamed"
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
