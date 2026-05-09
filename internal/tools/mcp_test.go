package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/config"
	"github.com/icoo-ai/icoo-ai/internal/mcp"
	"github.com/icoo-ai/icoo-ai/internal/policy"
)

func TestNewMCPToolsDiscoversAndMapsToolsFromConfig(t *testing.T) {
	client := &fakeMCPClient{
		tools: []mcp.ToolDefinition{
			{
				Name:        "read_file",
				Description: "Read a remote file",
				InputSchema: json.RawMessage(`{"type":"object","required":["path"],"properties":{"path":{"type":"string"}}}`),
			},
		},
		result: mcp.CallResult{
			Content: "hello",
			Data:    map[string]any{"bytes": float64(5)},
		},
	}
	logger := &captureAuditLogger{}
	toolset, err := NewMCPTools(context.Background(), MCPToolOptions{
		Config: config.MCPConfig{
			Enabled: true,
			Servers: map[string]config.MCPServerConfig{
				"filesystem": {
					Enabled: true,
					Command: "mcp-server-filesystem",
				},
			},
		},
		Factory:     fakeMCPFactory{"filesystem": client},
		Policy:      allowPolicy{},
		AuditLogger: logger,
		Now:         fixedNow,
	})
	if err != nil {
		t.Fatalf("NewMCPTools() error = %v", err)
	}
	if len(toolset) != 1 {
		t.Fatalf("tool count = %d, want 1", len(toolset))
	}

	tool := toolset[0]
	if tool.Name() != "mcp__filesystem__read_file" {
		t.Fatalf("tool name = %q", tool.Name())
	}
	def := tool.Definition()
	if def.Description != "Read a remote file" {
		t.Fatalf("description = %q", def.Description)
	}
	if string(def.InputSchema) != `{"type":"object","required":["path"],"properties":{"path":{"type":"string"}}}` {
		t.Fatalf("schema = %s", def.InputSchema)
	}

	result := runTool(t, tool, map[string]any{"path": "README.md"})
	if !result.OK || result.Content != "hello" {
		t.Fatalf("result = %+v", result)
	}
	if client.calls != 1 || client.last.Name != "read_file" || client.last.Arguments["path"] != "README.md" {
		t.Fatalf("client call = %+v calls=%d", client.last, client.calls)
	}
	if result.Data["server"] != "filesystem" || result.Data["name"] != "read_file" || result.Data["bytes"] != float64(5) {
		t.Fatalf("result data = %+v", result.Data)
	}
	if len(logger.events) != 1 || logger.events[0].Type != audit.EventMCPCall || logger.events[0].Timestamp != fixedNow() {
		t.Fatalf("audit events = %+v", logger.events)
	}
}

func TestMCPToolUsesPolicyAndBlocksClientCall(t *testing.T) {
	client := &fakeMCPClient{
		tools: []mcp.ToolDefinition{{Name: "danger", InputSchema: json.RawMessage(`{"type":"object"}`)}},
	}
	logger := &captureAuditLogger{}
	toolset, err := NewMCPTools(context.Background(), MCPToolOptions{
		Servers:     []mcp.ServerDefinition{{Name: "srv", Enabled: true, Transport: mcp.TransportStdio}},
		Factory:     fakeMCPFactory{"srv": client},
		Policy:      blockMCPPolicy{},
		Now:         fixedNow,
		AuditLogger: logger,
	})
	if err != nil {
		t.Fatalf("NewMCPTools() error = %v", err)
	}

	result := runTool(t, toolset[0], map[string]any{"force": true})
	if result.OK || result.Data["code"] != "policy_blocked" {
		t.Fatalf("result = %+v, want policy_blocked", result)
	}
	if client.calls != 0 {
		t.Fatalf("client was called %d times", client.calls)
	}
	if len(logger.events) != 1 || logger.events[0].Data["ok"] != false {
		t.Fatalf("audit events = %+v", logger.events)
	}
}

func TestMCPToolConvertsRemoteErrors(t *testing.T) {
	client := &fakeMCPClient{
		tools:  []mcp.ToolDefinition{{Name: "fail"}},
		result: mcp.CallResult{IsError: true, Error: "remote failed", Content: "failure details"},
	}
	toolset, err := NewMCPTools(context.Background(), MCPToolOptions{
		Servers: []mcp.ServerDefinition{{Name: "srv", Enabled: true, Transport: mcp.TransportStdio}},
		Factory: fakeMCPFactory{"srv": client},
		Policy:  allowPolicy{},
	})
	if err != nil {
		t.Fatalf("NewMCPTools() error = %v", err)
	}

	result := runTool(t, toolset[0], map[string]any{})
	if result.OK || result.Error != "remote failed" || result.Content != "failure details" {
		t.Fatalf("result = %+v", result)
	}
}

func TestNewMCPToolsReportsConflictsAndInvalidSchema(t *testing.T) {
	_, err := NewMCPTools(context.Background(), MCPToolOptions{
		Servers: []mcp.ServerDefinition{
			{Name: "A", Enabled: true},
			{Name: "a", Enabled: true},
		},
		Factory: fakeMCPFactory{
			"A": &fakeMCPClient{tools: []mcp.ToolDefinition{{Name: "tool"}}},
			"a": &fakeMCPClient{tools: []mcp.ToolDefinition{{Name: "tool"}}},
		},
	})
	if err == nil {
		t.Fatal("expected name conflict")
	}

	_, err = NewMCPTools(context.Background(), MCPToolOptions{
		Servers: []mcp.ServerDefinition{{Name: "srv", Enabled: true}},
		Factory: fakeMCPFactory{
			"srv": &fakeMCPClient{tools: []mcp.ToolDefinition{{Name: "tool", InputSchema: json.RawMessage(`{`)}}},
		},
	})
	if !errors.Is(err, mcp.ErrInvalidSchema) {
		t.Fatalf("err = %v, want ErrInvalidSchema", err)
	}
}

func TestMCPToolRetriesTransientClientErrors(t *testing.T) {
	client := &fakeMCPClient{
		tools: []mcp.ToolDefinition{{Name: "flaky"}},
		errs:  []error{errTemporaryMCP{}, nil},
		result: mcp.CallResult{
			Content: "ok",
		},
	}
	toolset, err := NewMCPTools(context.Background(), MCPToolOptions{
		Servers: []mcp.ServerDefinition{{Name: "srv", Enabled: true}},
		Factory: fakeMCPFactory{"srv": client},
		Policy:  allowPolicy{},
	})
	if err != nil {
		t.Fatalf("NewMCPTools() error = %v", err)
	}

	result := runTool(t, toolset[0], map[string]any{})
	if !result.OK || result.Content != "ok" {
		t.Fatalf("result = %+v", result)
	}
	if client.calls != 2 || result.Data["retry_attempts"] != 2 {
		t.Fatalf("calls=%d data=%+v", client.calls, result.Data)
	}
}

func TestMCPToolTimeoutReturnsClearError(t *testing.T) {
	client := &fakeMCPClient{
		tools: []mcp.ToolDefinition{{Name: "slow"}},
		delay: 50 * time.Millisecond,
	}
	toolset, err := NewMCPTools(context.Background(), MCPToolOptions{
		Servers: []mcp.ServerDefinition{{Name: "srv", Enabled: true}},
		Factory: fakeMCPFactory{"srv": client},
		Policy:  allowPolicy{},
		Timeout: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewMCPTools() error = %v", err)
	}

	result := runTool(t, toolset[0], map[string]any{})
	if result.OK || result.Data["code"] != "mcp_timeout" || !strings.Contains(result.Error, "timed out") {
		t.Fatalf("result = %+v", result)
	}
}

type fakeMCPFactory map[string]*fakeMCPClient

func (f fakeMCPFactory) NewClient(_ context.Context, def mcp.ServerDefinition) (mcp.Client, error) {
	client := f[def.Name]
	if client == nil {
		return nil, errors.New("missing fake client")
	}
	client.def = def
	return client, nil
}

type fakeMCPClient struct {
	def    mcp.ServerDefinition
	tools  []mcp.ToolDefinition
	result mcp.CallResult
	err    error
	errs   []error
	delay  time.Duration
	calls  int
	last   mcp.ToolCall
}

func (c *fakeMCPClient) ListTools(ctx context.Context) ([]mcp.ToolDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]mcp.ToolDefinition(nil), c.tools...), nil
}

func (c *fakeMCPClient) CallTool(ctx context.Context, call mcp.ToolCall) (mcp.CallResult, error) {
	if err := ctx.Err(); err != nil {
		return mcp.CallResult{}, err
	}
	if c.delay > 0 {
		select {
		case <-ctx.Done():
			return mcp.CallResult{}, ctx.Err()
		case <-time.After(c.delay):
		}
	}
	c.calls++
	c.last = call
	if len(c.errs) > 0 {
		err := c.errs[0]
		c.errs = c.errs[1:]
		if err != nil {
			return mcp.CallResult{}, err
		}
	}
	if c.err != nil {
		return mcp.CallResult{}, c.err
	}
	return c.result, nil
}

type errTemporaryMCP struct{}

func (errTemporaryMCP) Error() string   { return "temporary mcp failure" }
func (errTemporaryMCP) Timeout() bool   { return false }
func (errTemporaryMCP) Temporary() bool { return true }

func (c *fakeMCPClient) ListResources(ctx context.Context) ([]mcp.ResourceDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *fakeMCPClient) ReadResource(ctx context.Context, uri string) (mcp.ResourceContent, error) {
	if err := ctx.Err(); err != nil {
		return mcp.ResourceContent{}, err
	}
	return mcp.ResourceContent{URI: uri}, nil
}

func (c *fakeMCPClient) ListPrompts(ctx context.Context) ([]mcp.PromptDefinition, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *fakeMCPClient) GetPrompt(ctx context.Context, name string, arguments map[string]any) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "", nil
}

type blockMCPPolicy struct{}

func (blockMCPPolicy) EvaluateCommand(policy.CommandRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (blockMCPPolicy) EvaluatePath(policy.PathRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (blockMCPPolicy) EvaluateNetwork(policy.NetworkRequest) policy.Decision {
	return policy.Decision{Action: policy.DecisionAllow, Risk: policy.RiskLevelLow}
}
func (blockMCPPolicy) EvaluateMCP(req policy.MCPRequest) policy.Decision {
	return policy.Decision{
		Action: policy.DecisionBlock,
		Risk:   policy.RiskLevelHigh,
		Reason: "blocked " + req.Server + "/" + req.Name,
		Details: map[string]any{
			"server": req.Server,
			"name":   req.Name,
		},
	}
}

type captureAuditLogger struct {
	events []audit.Event
}

func (l *captureAuditLogger) Log(ctx context.Context, event audit.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.events = append(l.events, event)
	return nil
}
