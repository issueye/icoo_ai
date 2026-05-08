package subagent

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

type ToolOptions struct {
	Runner      Runner
	CWD         string
	Model       string
	AuditLogger audit.Logger
	Now         func() time.Time
}

func NewTool(opts ToolOptions) tools.Tool {
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return subagentTool{
		runner:      opts.Runner,
		cwd:         opts.CWD,
		model:       opts.Model,
		auditLogger: opts.AuditLogger,
		now:         now,
	}
}

type subagentTool struct {
	runner      Runner
	cwd         string
	model       string
	auditLogger audit.Logger
	now         func() time.Time
}

func (t subagentTool) Name() string { return "subagent_run" }

func (t subagentTool) Description() string {
	return "Delegate a focused task to a subagent that can use the workspace tools and return a concise result."
}

func (t subagentTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: json.RawMessage(`{"type":"object","required":["task"],"properties":{"task":{"type":"string"},"context":{"type":"array","items":{"type":"string"}},"max_tool_rounds":{"type":"integer"}}}`),
	}
}

func (t subagentTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	if t.runner == nil {
		return toolError("subagent_unavailable", "subagent runner is not configured", nil), nil
	}
	var req struct {
		Task          string   `json:"task"`
		Context       []string `json:"context"`
		MaxToolRounds int      `json:"max_tool_rounds"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	if strings.TrimSpace(req.Task) == "" {
		return toolError("invalid_input", "task is required", nil), nil
	}
	startedAt := t.now().UTC()
	result, err := t.runner.Run(ctx, Request{
		SessionID:     "subagent-tool",
		CWD:           t.cwd,
		Task:          req.Task,
		Context:       req.Context,
		Model:         t.model,
		MaxToolRounds: req.MaxToolRounds,
	})
	if err != nil {
		_ = t.log(ctx, startedAt, false, err.Error(), req.Task)
		return toolError("subagent_failed", err.Error(), nil), nil
	}
	_ = t.log(ctx, startedAt, true, "", req.Task)
	return tools.ToolResult{
		OK:      true,
		Content: result.Content,
		Data: map[string]any{
			"content":     result.Content,
			"event_count": len(result.Events),
		},
	}, nil
}

func (t subagentTool) log(ctx context.Context, at time.Time, ok bool, errText, task string) error {
	if t.auditLogger == nil {
		return nil
	}
	data := map[string]any{
		"ok":         ok,
		"task_bytes": len(task),
	}
	if errText != "" {
		data["error"] = errText
	}
	return t.auditLogger.Log(ctx, audit.Event{
		Type:      audit.EventSubagentRun,
		Timestamp: at.UTC(),
		Summary:   "subagent run",
		Data:      data,
	})
}

func toolError(code, message string, data map[string]any) tools.ToolResult {
	if data == nil {
		data = map[string]any{}
	}
	data["code"] = code
	return tools.ToolResult{OK: false, Error: message, Data: data}
}
