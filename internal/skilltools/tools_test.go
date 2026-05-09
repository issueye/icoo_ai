package skilltools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/skills"
	"github.com/icoo-ai/icoo-ai/internal/subagent"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

func TestSkillToolsAddListGetAndDelete(t *testing.T) {
	project := t.TempDir()
	ts := NewTools(Options{
		Sources: []skills.Source{{Kind: skills.SourceProject, Path: filepath.Join(project, ".icoo-ai", "skills"), Priority: 30}},
	})
	byName := indexTools(ts)

	addResult := executeTool(t, byName["skill_add"], map[string]any{
		"name":        "go-review",
		"description": "Review Go code",
		"body":        "# Review\nCheck tests.\n",
		"scope":       "project",
	})
	if !addResult.OK {
		t.Fatalf("skill_add failed: %+v", addResult)
	}

	listResult := executeTool(t, byName["skill_list"], map[string]any{})
	if !strings.Contains(listResult.Content, "go-review - Review Go code") {
		t.Fatalf("skill_list content = %q", listResult.Content)
	}

	getResult := executeTool(t, byName["skill_get"], map[string]any{"name": "go-review"})
	if !getResult.OK || !strings.Contains(getResult.Content, "Check tests.") {
		t.Fatalf("skill_get result = %+v", getResult)
	}

	deleteTool, ok := byName["skill_delete"].(tools.ApprovalCapable)
	if !ok {
		t.Fatal("skill_delete does not implement ApprovalCapable")
	}
	deleteResult, err := deleteTool.ExecuteApproved(context.Background(), mustJSON(t, map[string]any{"name": "go-review"}), tools.ApprovalScopeOnce)
	if err != nil {
		t.Fatalf("ExecuteApproved() error = %v", err)
	}
	if !deleteResult.OK {
		t.Fatalf("skill_delete result = %+v", deleteResult)
	}
	if _, err := os.Stat(filepath.Join(project, ".icoo-ai", "skills", "go-review")); !os.IsNotExist(err) {
		t.Fatalf("skill directory still exists or stat failed: %v", err)
	}
}

func TestSkillAddRejectsPathTraversalName(t *testing.T) {
	project := t.TempDir()
	ts := NewTools(Options{
		Sources: []skills.Source{{Kind: skills.SourceProject, Path: filepath.Join(project, ".icoo-ai", "skills"), Priority: 30}},
	})
	result := executeTool(t, indexTools(ts)["skill_add"], map[string]any{
		"name":        "../escape",
		"description": "bad",
		"body":        "bad",
	})
	if result.OK || result.Data["code"] != "invalid_input" {
		t.Fatalf("skill_add result = %+v, want invalid_input", result)
	}
}

func TestSkillExecuteDelegatesToSubagent(t *testing.T) {
	project := t.TempDir()
	writeSkill(t, filepath.Join(project, ".icoo-ai", "skills", "summarize"), "summarize", "Summarize context", "# Instructions\nBe brief.\n")
	runner := &fakeRunner{result: subagent.Result{Content: "summary"}}
	ts := NewTools(Options{
		Sources: []skills.Source{{Kind: skills.SourceProject, Path: filepath.Join(project, ".icoo-ai", "skills"), Priority: 30}},
		CWD:     project,
		Model:   "gpt-test",
		Runner:  runner,
	})

	result := executeTool(t, indexTools(ts)["skill_execute"], map[string]any{
		"name": "summarize",
		"task": "summarize README",
	})
	if !result.OK || result.Content != "summary" {
		t.Fatalf("skill_execute result = %+v", result)
	}
	if len(runner.requests) != 1 {
		t.Fatalf("runner requests = %d, want 1", len(runner.requests))
	}
	req := runner.requests[0]
	if req.Skill == nil || req.Skill.Name != "summarize" || !strings.Contains(req.Skill.Body, "Be brief.") {
		t.Fatalf("runner skill = %+v", req.Skill)
	}
	if req.Task != "summarize README" || req.Model != "gpt-test" {
		t.Fatalf("runner request = %+v", req)
	}
	if req.SessionID == "" || !strings.HasPrefix(req.SessionID, "skill_summarize_") {
		t.Fatalf("runner session id = %q, want unique skill-prefixed id", req.SessionID)
	}
	if result.Data["session_id"] != req.SessionID {
		t.Fatalf("result session_id = %+v, want %q", result.Data["session_id"], req.SessionID)
	}
}

type fakeRunner struct {
	result   subagent.Result
	err      error
	requests []subagent.Request
}

func (r *fakeRunner) Run(ctx context.Context, req subagent.Request) (subagent.Result, error) {
	r.requests = append(r.requests, req)
	return r.result, r.err
}

func indexTools(ts []tools.Tool) map[string]tools.Tool {
	out := map[string]tools.Tool{}
	for _, tool := range ts {
		out[tool.Name()] = tool
	}
	return out
}

func executeTool(t *testing.T, tool tools.Tool, input any) tools.ToolResult {
	t.Helper()
	if tool == nil {
		t.Fatal("tool is nil")
	}
	result, err := tool.Execute(context.Background(), mustJSON(t, input))
	if err != nil {
		t.Fatalf("%s.Execute() error = %v", tool.Name(), err)
	}
	return result
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}

func writeSkill(t *testing.T, dir, name, description, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	data := "---\nname: " + name + "\ndescription: " + description + "\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(data), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
}
