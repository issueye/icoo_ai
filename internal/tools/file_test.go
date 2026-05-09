package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/hooks"
)

func TestFileToolsListSearchReadRespectWorkspaceAndIgnore(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, ".icooignore"), "ignored/\n")
	writeTestFile(t, filepath.Join(root, "src", "main.go"), "package main\nfunc main() {}\n")
	writeTestFile(t, filepath.Join(root, "ignored", "secret.go"), "needle\n")
	writeTestFile(t, filepath.Join(root, ".env"), "TOKEN=secret")

	toolset := newTestFileTools(t, root, 0)
	list := runTool(t, toolset["list_files"], map[string]any{})
	if !list.OK {
		t.Fatalf("list failed: %+v", list)
	}
	if !strings.Contains(list.Content, "src/") || strings.Contains(list.Content, "ignored") {
		t.Fatalf("unexpected list content:\n%s", list.Content)
	}

	search := runTool(t, toolset["search_files"], map[string]any{"query": "func main"})
	if !search.OK {
		t.Fatalf("search failed: %+v", search)
	}
	if !strings.Contains(search.Content, "src/main.go:2:func main") {
		t.Fatalf("unexpected search content:\n%s", search.Content)
	}

	read := runTool(t, toolset["read_file"], map[string]any{"path": ".env"})
	if read.OK || read.Data["code"] != "secret_file" {
		t.Fatalf("secret read result = %+v, want secret_file error", read)
	}

	outside := runTool(t, toolset["read_file"], map[string]any{"path": "../outside.txt"})
	if outside.OK || outside.Data["code"] != "outside_workspace" {
		t.Fatalf("outside read result = %+v, want outside_workspace error", outside)
	}
}

func TestReadFileTruncatesAndSkipsBinary(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "big.txt"), strings.Repeat("a", 20))
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{1, 2, 0, 3}, 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	toolset := newTestFileTools(t, root, 8)

	big := runTool(t, toolset["read_file"], map[string]any{"path": "big.txt"})
	if !big.OK || big.Content != "aaaaaaaa" || big.Data["truncated"] != true {
		t.Fatalf("big read = %+v", big)
	}

	binary := runTool(t, toolset["read_file"], map[string]any{"path": "bin.dat"})
	if binary.OK || binary.Data["code"] != "binary_file" {
		t.Fatalf("binary read = %+v, want binary_file error", binary)
	}
}

func TestWriteFileCreatesInsideWorkspaceOnly(t *testing.T) {
	root := t.TempDir()
	toolset := newTestFileTools(t, root, 0)

	result := runTool(t, toolset["write_file"], map[string]any{
		"path":        "nested/file.txt",
		"content":     "hello",
		"create_dirs": true,
	})
	if !result.OK {
		t.Fatalf("write failed: %+v", result)
	}
	if got := readTestFile(t, filepath.Join(root, "nested", "file.txt")); got != "hello" {
		t.Fatalf("written content = %q", got)
	}

	outside := runTool(t, toolset["write_file"], map[string]any{
		"path":    "../outside.txt",
		"content": "nope",
	})
	if outside.OK || outside.Data["code"] != "outside_workspace" {
		t.Fatalf("outside write = %+v, want outside_workspace error", outside)
	}
}

func TestWriteFileHookBlockPreventsWrite(t *testing.T) {
	root := t.TempDir()
	tools, err := NewFileTools(FileToolOptions{
		WorkspaceRoot: root,
		Hooks: hooks.NewDispatcher(hooks.TypedHook{
			HookName: "block-write",
			Events:   []hooks.EventType{hooks.EventBeforeFileWrite},
			Func: func(ctx context.Context, event hooks.Event) (hooks.Result, error) {
				return hooks.Block("blocked by file hook"), nil
			},
		}),
	})
	if err != nil {
		t.Fatalf("NewFileTools() error = %v", err)
	}
	byName := map[string]Tool{}
	for _, tool := range tools {
		byName[tool.Name()] = tool
	}
	result := runTool(t, byName["write_file"], map[string]any{
		"path":    "blocked.txt",
		"content": "hello",
	})
	if result.OK || result.Error != "blocked by file hook" {
		t.Fatalf("result = %+v", result)
	}
	if _, err := os.Stat(filepath.Join(root, "blocked.txt")); !os.IsNotExist(err) {
		t.Fatalf("blocked file exists or stat failed: %v", err)
	}
}

func TestApplyPatchIsAtomicOnFailedMatch(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "file.txt")
	writeTestFile(t, target, "alpha\nbeta\n")
	toolset := newTestFileTools(t, root, 0)

	failed := runTool(t, toolset["apply_patch"], map[string]any{
		"path": "file.txt",
		"old":  "missing",
		"new":  "changed",
	})
	if failed.OK || failed.Data["code"] != "patch_not_unique" {
		t.Fatalf("failed patch = %+v, want patch_not_unique", failed)
	}
	if got := readTestFile(t, target); got != "alpha\nbeta\n" {
		t.Fatalf("content changed after failed patch: %q", got)
	}

	ok := runTool(t, toolset["apply_patch"], map[string]any{
		"path": "file.txt",
		"old":  "beta",
		"new":  "gamma",
	})
	if !ok.OK {
		t.Fatalf("patch failed: %+v", ok)
	}
	if got := readTestFile(t, target); got != "alpha\ngamma\n" {
		t.Fatalf("patched content = %q", got)
	}
}

func newTestFileTools(t *testing.T, root string, maxRead int64) map[string]Tool {
	t.Helper()
	tools, err := NewFileTools(FileToolOptions{WorkspaceRoot: root, MaxReadBytes: maxRead})
	if err != nil {
		t.Fatalf("NewFileTools() error = %v", err)
	}
	byName := map[string]Tool{}
	for _, tool := range tools {
		byName[tool.Name()] = tool
	}
	return byName
}

func runTool(t *testing.T, tool Tool, input map[string]any) ToolResult {
	t.Helper()
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	result, err := tool.Execute(context.Background(), payload)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	return result
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return string(content)
}
