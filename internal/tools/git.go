package tools

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/policy"
)

type GitToolOptions struct {
	WorkspaceRoot  string
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
	Policy         policy.Policy
	Runner         ShellRunner
}

func NewGitTools(opts GitToolOptions) []Tool {
	shellBase := newShellToolBase(ShellToolOptions{
		WorkspaceRoot:  opts.WorkspaceRoot,
		DefaultTimeout: opts.DefaultTimeout,
		MaxTimeout:     opts.MaxTimeout,
		Policy:         opts.Policy,
		Runner:         opts.Runner,
	})
	return []Tool{
		gitStatusTool{base: shellBase},
		gitDiffTool{base: shellBase},
	}
}

type gitStatusTool struct{ base shellToolBase }
type gitDiffTool struct{ base shellToolBase }

func (t gitStatusTool) Name() string { return "git_status" }
func (t gitStatusTool) Description() string {
	return "Show git branch and working tree status for the current workspace."
}
func (t gitStatusTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","properties":{"working_dir":{"type":"string"}}}`)
}
func (t gitStatusTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		WorkingDir string `json:"working_dir"`
	}
	if err := json.Unmarshal(input, &req); err != nil && len(input) > 0 {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	return runGitCommand(ctx, t.base, "git status --short --branch", req.WorkingDir)
}

func (t gitDiffTool) Name() string { return "git_diff" }
func (t gitDiffTool) Description() string {
	return "Show git diff for the current workspace, optionally scoped to a path or staged changes."
}
func (t gitDiffTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","properties":{"working_dir":{"type":"string"},"path":{"type":"string"},"staged":{"type":"boolean"}}}`)
}
func (t gitDiffTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		WorkingDir string `json:"working_dir"`
		Path       string `json:"path"`
		Staged     bool   `json:"staged"`
	}
	if err := json.Unmarshal(input, &req); err != nil && len(input) > 0 {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	command := "git diff"
	if req.Staged {
		command += " --staged"
	}
	if req.Path != "" {
		command += " -- " + shellQuote(req.Path)
	}
	return runGitCommand(ctx, t.base, command, req.WorkingDir)
}

func runGitCommand(ctx context.Context, base shellToolBase, command, workingDir string) (ToolResult, error) {
	dir, err := base.resolveWorkingDir(workingDir)
	if err != nil {
		return toolError("invalid_working_dir", err.Error(), map[string]any{"working_dir": workingDir}), nil
	}
	decision := base.policy.EvaluateCommand(policy.CommandRequest{
		Command:    command,
		WorkingDir: dir,
	})
	if decision.Action == policy.DecisionBlock {
		return policyToolResult("policy_blocked", decision), nil
	}
	if decision.Action == policy.DecisionRequestApproval {
		return policyToolResult("approval_required", decision), nil
	}
	result, err := base.runner.Run(ctx, ShellCommand{
		Command: command,
		Dir:     dir,
		Timeout: base.defaultTimeout,
	})
	if err != nil {
		return toolError("git_error", err.Error(), map[string]any{
			"command":     command,
			"working_dir": dir,
			"stdout":      result.Stdout,
			"stderr":      result.Stderr,
			"exit_code":   result.ExitCode,
		}), nil
	}
	return ToolResult{
		OK:      result.ExitCode == 0,
		Content: shellContent(result),
		Error:   shellError(result),
		Data: map[string]any{
			"command":     command,
			"working_dir": dir,
			"stdout":      result.Stdout,
			"stderr":      result.Stderr,
			"exit_code":   result.ExitCode,
			"policy":      decision,
		},
	}, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
