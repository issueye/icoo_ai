package tools

import (
	"testing"
)

func TestGitStatusRunsStatusCommand(t *testing.T) {
	root := t.TempDir()
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{
		Stdout:   "## main\n M internal/tools/git.go\n",
		ExitCode: 0,
	}})
	toolset := newTestGitTools(root, fake)

	result := runTool(t, toolset["git_status"], map[string]any{})
	if !result.OK || result.Content == "" {
		t.Fatalf("result = %+v", result)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Command != "git status --short --branch" || calls[0].Dir != root {
		t.Fatalf("call = %+v", calls[0])
	}
}

func TestGitDiffRunsDiffCommandWithOptions(t *testing.T) {
	root := t.TempDir()
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{
		Stdout:   "diff --git a/file.txt b/file.txt\n",
		ExitCode: 0,
	}})
	toolset := newTestGitTools(root, fake)

	result := runTool(t, toolset["git_diff"], map[string]any{
		"path":   "dir/it's.txt",
		"staged": true,
	})
	if !result.OK {
		t.Fatalf("result = %+v", result)
	}
	calls := fake.Calls()
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	want := "git diff --staged -- 'dir/it'\"'\"'s.txt'"
	if calls[0].Command != want {
		t.Fatalf("command = %q, want %q", calls[0].Command, want)
	}
}

func TestGitDiffReturnsExitCodeAndStderr(t *testing.T) {
	fake := newRecordingShellRunner(shellResponse{result: ShellResult{
		Stderr:   "not a git repository",
		ExitCode: 129,
	}})
	toolset := newTestGitTools("", fake)

	result := runTool(t, toolset["git_diff"], map[string]any{})
	if result.OK || result.Error != "not a git repository" || result.Data["exit_code"] != 129 {
		t.Fatalf("result = %+v", result)
	}
}

func newTestGitTools(root string, fake *recordingShellRunner) map[string]Tool {
	toolset := NewGitTools(GitToolOptions{
		WorkspaceRoot: root,
		Runner:        fake,
	})
	byName := map[string]Tool{}
	for _, tool := range toolset {
		byName[tool.Name()] = tool
	}
	return byName
}
