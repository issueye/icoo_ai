package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/policy"
)

const (
	defaultShellTimeout = 120 * time.Second
	defaultMaxTimeout   = 10 * time.Minute
)

type ShellToolOptions struct {
	WorkspaceRoot  string
	DefaultTimeout time.Duration
	MaxTimeout     time.Duration
	Policy         policy.Policy
	Runner         ShellRunner
}

type ShellRunner interface {
	Run(context.Context, ShellCommand) (ShellResult, error)
}

type ShellCommand struct {
	Command string
	Dir     string
	Env     []string
	Timeout time.Duration
}

type ShellResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func NewShellTool(opts ShellToolOptions) Tool {
	base := newShellToolBase(opts)
	return runShellTool{base: base}
}

type shellToolBase struct {
	workspaceRoot  string
	defaultTimeout time.Duration
	maxTimeout     time.Duration
	policy         policy.Policy
	runner         ShellRunner
}

type runShellTool struct{ base shellToolBase }

func newShellToolBase(opts ShellToolOptions) shellToolBase {
	defaultTimeout := opts.DefaultTimeout
	if defaultTimeout <= 0 {
		defaultTimeout = defaultShellTimeout
	}
	maxTimeout := opts.MaxTimeout
	if maxTimeout <= 0 {
		maxTimeout = defaultMaxTimeout
	}
	if defaultTimeout > maxTimeout {
		defaultTimeout = maxTimeout
	}
	p := opts.Policy
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	runner := opts.Runner
	if runner == nil {
		runner = OSExecShellRunner{}
	}
	root := opts.WorkspaceRoot
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = filepath.Clean(abs)
		}
	}
	return shellToolBase{
		workspaceRoot:  root,
		defaultTimeout: defaultTimeout,
		maxTimeout:     maxTimeout,
		policy:         p,
		runner:         runner,
	}
}

func (t runShellTool) Name() string { return "run_shell" }
func (t runShellTool) Description() string {
	return "Run a shell command with policy evaluation, timeout, working directory, stdout, stderr, and exit code."
}
func (t runShellTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["command"],"properties":{"command":{"type":"string"},"working_dir":{"type":"string"},"timeout_seconds":{"type":"integer","minimum":1}}}`)
}
func (t runShellTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	req, result, ok := t.parseRequest(input)
	if !ok {
		return result, nil
	}

	decision := t.base.policy.EvaluateCommand(policy.CommandRequest{
		Command:    req.Command,
		WorkingDir: req.WorkingDir,
	})
	if decision.Action == policy.DecisionBlock {
		return policyToolResult("policy_blocked", decision), nil
	}
	if decision.Action == policy.DecisionRequestApproval {
		return policyToolResult("approval_required", decision), nil
	}
	return t.executeParsed(ctx, req, "")
}

func (t runShellTool) ApprovalKey(input json.RawMessage) (string, bool) {
	req, _, ok := t.parseRequest(input)
	if !ok {
		return "", false
	}
	return req.WorkingDir + "\x00" + req.Command, true
}

func (t runShellTool) ExecuteApproved(ctx context.Context, input json.RawMessage, scope ApprovalScope) (ToolResult, error) {
	req, result, ok := t.parseRequest(input)
	if !ok {
		return result, nil
	}
	return t.executeParsed(ctx, req, scope)
}

type shellRequest struct {
	Command        string
	WorkingDir     string
	TimeoutSeconds int
}

func (t runShellTool) parseRequest(input json.RawMessage) (shellRequest, ToolResult, bool) {
	var req struct {
		Command        string `json:"command"`
		WorkingDir     string `json:"working_dir"`
		TimeoutSeconds int    `json:"timeout_seconds"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return shellRequest{}, toolError("invalid_json", err.Error(), nil), false
	}
	if req.Command == "" {
		return shellRequest{}, toolError("invalid_input", "command is required", nil), false
	}

	workingDir, err := t.base.resolveWorkingDir(req.WorkingDir)
	if err != nil {
		return shellRequest{}, toolError("invalid_working_dir", err.Error(), map[string]any{"working_dir": req.WorkingDir}), false
	}
	return shellRequest{Command: req.Command, WorkingDir: workingDir, TimeoutSeconds: req.TimeoutSeconds}, ToolResult{}, true
}

func (t runShellTool) executeParsed(ctx context.Context, req shellRequest, approval ApprovalScope) (ToolResult, error) {
	timeout := t.base.timeout(time.Duration(req.TimeoutSeconds) * time.Second)
	data := map[string]any{}
	if approval != "" {
		data["approval"] = approval
	}

	start := time.Now()
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result, err := t.base.runner.Run(runCtx, ShellCommand{
		Command: req.Command,
		Dir:     req.WorkingDir,
		Timeout: timeout,
	})
	elapsed := time.Since(start)
	if err != nil {
		code := "shell_error"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			code = "timeout"
		}
		return toolError(code, err.Error(), map[string]any{
			"command":         req.Command,
			"working_dir":     req.WorkingDir,
			"timeout_seconds": int(timeout.Seconds()),
			"elapsed_ms":      elapsed.Milliseconds(),
			"stdout":          result.Stdout,
			"stderr":          result.Stderr,
			"exit_code":       result.ExitCode,
		}), nil
	}

	data["command"] = req.Command
	data["working_dir"] = req.WorkingDir
	data["timeout_seconds"] = int(timeout.Seconds())
	data["elapsed_ms"] = elapsed.Milliseconds()
	data["stdout"] = result.Stdout
	data["stderr"] = result.Stderr
	data["exit_code"] = result.ExitCode
	return ToolResult{
		OK:      result.ExitCode == 0,
		Content: shellContent(result),
		Error:   shellError(result),
		Data:    data,
	}, nil
}

func (b shellToolBase) resolveWorkingDir(dir string) (string, error) {
	if dir == "" {
		if b.workspaceRoot != "" {
			return b.workspaceRoot, nil
		}
		return "", nil
	}
	if !filepath.IsAbs(dir) && b.workspaceRoot != "" {
		dir = filepath.Join(b.workspaceRoot, dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func (b shellToolBase) timeout(requested time.Duration) time.Duration {
	if requested <= 0 {
		return b.defaultTimeout
	}
	if requested > b.maxTimeout {
		return b.maxTimeout
	}
	return requested
}

func policyToolResult(code string, decision policy.Decision) ToolResult {
	return toolError(code, decision.Reason, map[string]any{
		"decision": decision,
		"risk":     decision.Risk,
		"action":   decision.Action,
	})
}

func shellContent(result ShellResult) string {
	if result.Stdout != "" {
		return result.Stdout
	}
	return result.Stderr
}

func shellError(result ShellResult) string {
	if result.ExitCode == 0 {
		return ""
	}
	if result.Stderr != "" {
		return result.Stderr
	}
	return fmt.Sprintf("command exited with code %d", result.ExitCode)
}

type OSExecShellRunner struct{}

func (OSExecShellRunner) Run(ctx context.Context, cmd ShellCommand) (ShellResult, error) {
	if cmd.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cmd.Timeout)
		defer cancel()
	}
	invocation := shellInvocation(runtime.GOOS)
	execCmd := exec.CommandContext(ctx, invocation.name, append(invocation.args, cmd.Command)...)
	if cmd.Dir != "" {
		execCmd.Dir = cmd.Dir
	}
	if len(cmd.Env) > 0 {
		execCmd.Env = append(os.Environ(), cmd.Env...)
	}
	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr
	err := execCmd.Run()
	result := ShellResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode(err),
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, ctx.Err()
	}
	return result, nil
}

type shellInvocationSpec struct {
	name string
	args []string
}

func shellInvocation(goos string) shellInvocationSpec {
	if goos == "windows" {
		return shellInvocationSpec{
			name: "powershell.exe",
			args: []string{"-NoProfile", "-NonInteractive", "-Command"},
		}
	}
	return shellInvocationSpec{name: "/bin/sh", args: []string{"-c"}}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}
