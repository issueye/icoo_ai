package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/policy"
)

type recordingShellRunner struct {
	mu        sync.Mutex
	calls     []ShellCommand
	responses []shellResponse
}

type shellResponse struct {
	result ShellResult
	err    error
}

func newRecordingShellRunner(responses ...shellResponse) *recordingShellRunner {
	return &recordingShellRunner{responses: responses}
}

func (r *recordingShellRunner) Run(ctx context.Context, cmd ShellCommand) (ShellResult, error) {
	select {
	case <-ctx.Done():
		return ShellResult{}, ctx.Err()
	default:
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, cmd)
	index := len(r.calls) - 1
	if index < len(r.responses) {
		return r.responses[index].result, r.responses[index].err
	}
	return ShellResult{}, nil
}

func (r *recordingShellRunner) Calls() []ShellCommand {
	r.mu.Lock()
	defer r.mu.Unlock()
	calls := make([]ShellCommand, len(r.calls))
	copy(calls, r.calls)
	return calls
}

func TestRunShellExecutesAllowedCommand(t *testing.T) {
	root := t.TempDir()
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{Stdout: "ok\n", ExitCode: 0}})
	tool := NewShellTool(ShellToolOptions{
		WorkspaceRoot: root,
		Runner:        fake,
	})

	result := runTool(t, tool, map[string]any{
		"command":         "go test ./...",
		"working_dir":     ".",
		"timeout_seconds": 5,
	})
	if !result.OK || result.Content != "ok\n" {
		t.Fatalf("result = %+v", result)
	}
	if result.Data["exit_code"] != 0 {
		t.Fatalf("exit_code = %v, want 0", result.Data["exit_code"])
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Command != "go test ./..." || calls[0].Dir != root {
		t.Fatalf("call = %+v", calls[0])
	}
	if calls[0].Timeout != 5*time.Second {
		t.Fatalf("timeout = %s, want 5s", calls[0].Timeout)
	}
}

func TestRunShellReturnsExitCodeStdoutStderr(t *testing.T) {
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{
		Stdout:   "partial",
		Stderr:   "failed",
		ExitCode: 2,
	}})
	tool := NewShellTool(ShellToolOptions{Runner: fake})

	result := runTool(t, tool, map[string]any{"command": "go test"})
	if result.OK {
		t.Fatalf("result.OK = true, want false")
	}
	if result.Error != "failed" || result.Data["stdout"] != "partial" || result.Data["stderr"] != "failed" || result.Data["exit_code"] != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestRunShellRequestsApprovalForHighRiskCommand(t *testing.T) {
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{Stdout: "should not run"}})
	tool := NewShellTool(ShellToolOptions{Runner: fake})

	result := runTool(t, tool, map[string]any{"command": "git reset --hard"})
	if result.OK || result.Data["code"] != "approval_required" {
		t.Fatalf("result = %+v, want approval_required", result)
	}
	if result.Data["action"] != policy.DecisionRequestApproval || result.Data["risk"] != policy.RiskLevelHigh {
		t.Fatalf("policy data = %+v", result.Data)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("dangerous command executed: %+v", fake.Calls())
	}
}

func TestRunShellExecuteApprovedBypassesApprovalRequest(t *testing.T) {
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{Stdout: "ok\n", ExitCode: 0}})
	tool := NewShellTool(ShellToolOptions{Runner: fake})
	capable, ok := tool.(ApprovalCapable)
	if !ok {
		t.Fatal("run_shell does not implement ApprovalCapable")
	}

	payload := marshalToolInput(t, map[string]any{"command": "git reset --hard"})
	result, err := capable.ExecuteApproved(context.Background(), payload, ApprovalScopeOnce)
	if err != nil {
		t.Fatalf("ExecuteApproved() error = %v", err)
	}
	if !result.OK || result.Content != "ok\n" {
		t.Fatalf("result = %+v", result)
	}
	if result.Data["approval"] != ApprovalScopeOnce {
		t.Fatalf("approval = %v", result.Data["approval"])
	}
	if len(fake.Calls()) != 1 {
		t.Fatalf("calls = %d, want 1", len(fake.Calls()))
	}
}

func TestRunShellApprovalKey(t *testing.T) {
	tool := NewShellTool(ShellToolOptions{WorkspaceRoot: t.TempDir()})
	capable := tool.(ApprovalCapable)
	key, ok := capable.ApprovalKey(marshalToolInput(t, map[string]any{"command": "git reset --hard"}))
	if !ok || !strings.Contains(key, "git reset --hard") {
		t.Fatalf("key=%q ok=%v", key, ok)
	}
}

func marshalToolInput(t *testing.T, input map[string]any) []byte {
	t.Helper()
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	return payload
}

func TestRunShellBlocksCriticalCommand(t *testing.T) {
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{Stdout: "should not run"}})
	tool := NewShellTool(ShellToolOptions{Runner: fake})

	result := runTool(t, tool, map[string]any{"command": "rm -rf /"})
	if result.OK || result.Data["code"] != "policy_blocked" {
		t.Fatalf("result = %+v, want policy_blocked", result)
	}
	if len(fake.Calls()) != 0 {
		t.Fatalf("blocked command executed: %+v", fake.Calls())
	}
}

func TestRunShellRunnerErrorIncludesOutput(t *testing.T) {
	fake := newRecordingShellRunner(shellResponse{
		result: ShellResult{Stdout: "out", Stderr: "err", ExitCode: -1},
		err:    errors.New("boom"),
	})
	tool := NewShellTool(ShellToolOptions{Runner: fake})

	result := runTool(t, tool, map[string]any{"command": "go test"})
	if result.OK || result.Data["code"] != "shell_error" {
		t.Fatalf("result = %+v, want shell_error", result)
	}
	if result.Data["stdout"] != "out" || result.Data["stderr"] != "err" || result.Data["exit_code"] != -1 {
		t.Fatalf("result data = %+v", result.Data)
	}
}

func TestShellInvocationUsesPowerShellOnWindowsAndShOnUnix(t *testing.T) {
	windows := shellInvocation("windows")
	if windows.name != "powershell.exe" || !contains(windows.args, "-Command") {
		t.Fatalf("windows invocation = %+v", windows)
	}
	unix := shellInvocation("linux")
	if unix.name != "/bin/sh" || strings.Join(unix.args, " ") != "-c" {
		t.Fatalf("unix invocation = %+v", unix)
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
