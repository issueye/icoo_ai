package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/icoo-ai/icoo-ai/internal/policy"
	"github.com/icoo-ai/icoo-ai/internal/workspace"
)

const (
	defaultMaxReadBytes   int64 = 256 * 1024
	defaultMaxSearchBytes int64 = 1024 * 1024
	defaultMaxListResults       = 5000
	defaultMaxSearchHits        = 200
)

type FileToolOptions struct {
	WorkspaceRoot string
	MaxReadBytes  int64
	MaxSearchSize int64
	Policy        policy.Policy
}

func NewFileTools(opts FileToolOptions) ([]Tool, error) {
	ws, err := workspace.New(workspace.Options{Root: opts.WorkspaceRoot})
	if err != nil {
		return nil, err
	}
	base := fileToolBase{
		workspace:     ws,
		maxReadBytes:  valueOrDefault(opts.MaxReadBytes, defaultMaxReadBytes),
		maxSearchSize: valueOrDefault(opts.MaxSearchSize, defaultMaxSearchBytes),
		policy:        opts.Policy,
	}
	if base.policy == nil {
		base.policy = policy.New(policy.DefaultPermissionMode)
	}
	return []Tool{
		listFilesTool{base: base},
		searchFilesTool{base: base},
		readFileTool{base: base},
		writeFileTool{base: base},
		applyPatchTool{base: base},
	}, nil
}

type fileToolBase struct {
	workspace     *workspace.Workspace
	maxReadBytes  int64
	maxSearchSize int64
	policy        policy.Policy
}

type listFilesTool struct{ base fileToolBase }
type searchFilesTool struct{ base fileToolBase }
type readFileTool struct{ base fileToolBase }
type writeFileTool struct{ base fileToolBase }
type applyPatchTool struct{ base fileToolBase }

func (t listFilesTool) Name() string { return "list_files" }
func (t listFilesTool) Description() string {
	return "List files under the workspace, respecting ignore rules."
}
func (t listFilesTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","properties":{"path":{"type":"string"},"max_depth":{"type":"integer"},"include_hidden":{"type":"boolean"},"max_results":{"type":"integer"}}}`)
}
func (t listFilesTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		Path          string `json:"path"`
		MaxDepth      int    `json:"max_depth"`
		IncludeHidden bool   `json:"include_hidden"`
		MaxResults    int    `json:"max_results"`
	}
	if err := json.Unmarshal(input, &req); err != nil && len(input) > 0 {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > defaultMaxListResults {
		maxResults = defaultMaxListResults
	}
	var entries []workspace.FileEntry
	err := t.base.workspace.Walk(workspace.WalkOptions{
		Root:          req.Path,
		MaxDepth:      req.MaxDepth,
		IncludeHidden: req.IncludeHidden,
	}, func(entry workspace.FileEntry) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if len(entries) >= maxResults {
			return errStopWalk
		}
		entries = append(entries, entry)
		return nil
	})
	if errors.Is(err, errStopWalk) {
		err = nil
	}
	if err != nil {
		return pathError(err, req.Path), nil
	}
	var lines []string
	for _, entry := range entries {
		if entry.IsDir {
			lines = append(lines, entry.Path+"/")
		} else {
			lines = append(lines, entry.Path)
		}
	}
	return ToolResult{
		OK:      true,
		Content: strings.Join(lines, "\n"),
		Data: map[string]any{
			"files":     entries,
			"truncated": len(entries) >= maxResults,
		},
	}, nil
}

func (t searchFilesTool) Name() string { return "search_files" }
func (t searchFilesTool) Description() string {
	return "Search workspace text files for a literal query."
}
func (t searchFilesTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["query"],"properties":{"query":{"type":"string"},"path":{"type":"string"},"max_results":{"type":"integer"},"include_hidden":{"type":"boolean"}}}`)
}
func (t searchFilesTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		Query         string `json:"query"`
		Path          string `json:"path"`
		MaxResults    int    `json:"max_results"`
		IncludeHidden bool   `json:"include_hidden"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	if req.Query == "" {
		return toolError("invalid_input", "query is required", nil), nil
	}
	maxResults := req.MaxResults
	if maxResults <= 0 || maxResults > defaultMaxSearchHits {
		maxResults = defaultMaxSearchHits
	}
	type match struct {
		Path   string `json:"path"`
		Line   int    `json:"line"`
		Text   string `json:"text"`
		Binary bool   `json:"binary,omitempty"`
	}
	var matches []match
	err := t.base.workspace.Walk(workspace.WalkOptions{Root: req.Path, IncludeHidden: req.IncludeHidden}, func(entry workspace.FileEntry) error {
		if len(matches) >= maxResults {
			return errStopWalk
		}
		if entry.IsDir || entry.Size > t.base.maxSearchSize || t.base.workspace.IsSecret(entry.Path) {
			return nil
		}
		resolved, err := t.base.workspace.Resolve(entry.Path)
		if err != nil {
			return err
		}
		binary, err := workspace.IsBinaryFile(resolved.Abs)
		if err != nil || binary {
			return nil
		}
		file, err := os.Open(resolved.Abs)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNumber := 0
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			lineNumber++
			text := scanner.Text()
			if strings.Contains(text, req.Query) {
				matches = append(matches, match{Path: entry.Path, Line: lineNumber, Text: text})
				if len(matches) >= maxResults {
					break
				}
			}
		}
		return scanner.Err()
	})
	if errors.Is(err, errStopWalk) {
		err = nil
	}
	if err != nil {
		return pathError(err, req.Path), nil
	}
	lines := make([]string, 0, len(matches))
	for _, match := range matches {
		lines = append(lines, fmt.Sprintf("%s:%d:%s", match.Path, match.Line, match.Text))
	}
	return ToolResult{
		OK:      true,
		Content: strings.Join(lines, "\n"),
		Data: map[string]any{
			"matches":   matches,
			"truncated": len(matches) >= maxResults,
		},
	}, nil
}

func (t readFileTool) Name() string { return "read_file" }
func (t readFileTool) Description() string {
	return "Read a workspace text file with binary, secret, and size safeguards."
}
func (t readFileTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["path"],"properties":{"path":{"type":"string"},"max_bytes":{"type":"integer"}}}`)
}
func (t readFileTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		Path     string `json:"path"`
		MaxBytes int64  `json:"max_bytes"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	resolved, err := t.base.resolveForRead(req.Path)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	decision := t.base.policy.EvaluatePath(policy.PathRequest{
		Path:          resolved.Abs,
		WorkspaceRoot: t.base.workspace.Root,
		Operation:     policy.PathOperationRead,
	})
	if decision.Action == policy.DecisionBlock {
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}
	binary, err := workspace.IsBinaryFile(resolved.Abs)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	if binary {
		return toolError("binary_file", "binary files are not read by default", map[string]any{"path": resolved.Rel}), nil
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 || maxBytes > t.base.maxReadBytes {
		maxBytes = t.base.maxReadBytes
	}
	info, err := os.Stat(resolved.Abs)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	file, err := os.Open(resolved.Abs)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	defer file.Close()
	limited := io.LimitReader(file, maxBytes)
	content, err := io.ReadAll(limited)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	default:
	}
	truncated := info.Size() > int64(len(content))
	return ToolResult{
		OK:      true,
		Content: string(content),
		Data: map[string]any{
			"path":      resolved.Rel,
			"bytes":     len(content),
			"size":      info.Size(),
			"truncated": truncated,
		},
	}, nil
}

func (t writeFileTool) Name() string        { return "write_file" }
func (t writeFileTool) Description() string { return "Create or replace a workspace file atomically." }
func (t writeFileTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["path","content"],"properties":{"path":{"type":"string"},"content":{"type":"string"},"create_dirs":{"type":"boolean"}}}`)
}
func (t writeFileTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		Path       string `json:"path"`
		Content    string `json:"content"`
		CreateDirs bool   `json:"create_dirs"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	resolved, err := t.base.workspace.Resolve(req.Path)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	decision := t.base.policy.EvaluatePath(policy.PathRequest{
		Path:          resolved.Abs,
		WorkspaceRoot: t.base.workspace.Root,
		Operation:     policy.PathOperationWrite,
	})
	if decision.Action == policy.DecisionBlock {
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}
	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	default:
	}
	if req.CreateDirs {
		if err := os.MkdirAll(filepath.Dir(resolved.Abs), 0o755); err != nil {
			return pathError(err, req.Path), nil
		}
	}
	if err := atomicWriteFile(resolved.Abs, []byte(req.Content), 0o644); err != nil {
		return pathError(err, req.Path), nil
	}
	return ToolResult{
		OK:      true,
		Content: "wrote " + resolved.Rel,
		Data: map[string]any{
			"path":  resolved.Rel,
			"bytes": len(req.Content),
		},
	}, nil
}

func (t applyPatchTool) Name() string { return "apply_patch" }
func (t applyPatchTool) Description() string {
	return "Apply an atomic text replacement patch to a workspace file."
}
func (t applyPatchTool) Definition() ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["path","old","new"],"properties":{"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"}}}`)
}
func (t applyPatchTool) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	var req struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	if req.Old == "" {
		return toolError("invalid_input", "old text is required", nil), nil
	}
	resolved, err := t.base.workspace.Resolve(req.Path)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	decision := t.base.policy.EvaluatePath(policy.PathRequest{
		Path:          resolved.Abs,
		WorkspaceRoot: t.base.workspace.Root,
		Operation:     policy.PathOperationWrite,
	})
	if decision.Action == policy.DecisionBlock {
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}
	binary, err := workspace.IsBinaryFile(resolved.Abs)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	if binary {
		return toolError("binary_file", "binary files are not patched by default", map[string]any{"path": resolved.Rel}), nil
	}
	content, err := os.ReadFile(resolved.Abs)
	if err != nil {
		return pathError(err, req.Path), nil
	}
	count := bytes.Count(content, []byte(req.Old))
	if count != 1 {
		return toolError("patch_not_unique", fmt.Sprintf("old text matched %d times", count), map[string]any{"path": resolved.Rel}), nil
	}
	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	default:
	}
	updated := bytes.Replace(content, []byte(req.Old), []byte(req.New), 1)
	if err := atomicWriteFile(resolved.Abs, updated, 0o644); err != nil {
		return pathError(err, req.Path), nil
	}
	return ToolResult{
		OK:      true,
		Content: "patched " + resolved.Rel,
		Data: map[string]any{
			"path": resolved.Rel,
		},
	}, nil
}

func (b fileToolBase) resolveForRead(path string) (workspace.ResolvedPath, error) {
	resolved, err := b.workspace.Resolve(path)
	if err != nil {
		return workspace.ResolvedPath{}, err
	}
	if b.workspace.IsIgnored(resolved.Rel, false) {
		return workspace.ResolvedPath{}, fmt.Errorf("path %q is ignored", resolved.Rel)
	}
	if b.workspace.IsSecret(resolved.Rel) {
		return workspace.ResolvedPath{}, workspace.ErrSecretPath
	}
	return resolved, nil
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".icoo-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func definition(name, description, schema string) ToolDefinition {
	return ToolDefinition{Name: name, Description: description, InputSchema: json.RawMessage(schema)}
}

func toolError(code, message string, data map[string]any) ToolResult {
	if data == nil {
		data = map[string]any{}
	}
	data["code"] = code
	return ToolResult{OK: false, Error: message, Data: data}
}

func pathError(err error, inputPath string) ToolResult {
	code := "file_error"
	if errors.Is(err, workspace.ErrOutsideWorkspace) {
		code = "outside_workspace"
	} else if errors.Is(err, workspace.ErrSecretPath) {
		code = "secret_file"
	}
	return toolError(code, err.Error(), map[string]any{"path": inputPath})
}

func valueOrDefault(value, fallback int64) int64 {
	if value > 0 {
		return value
	}
	return fallback
}

var errStopWalk = errors.New("stop walk")
