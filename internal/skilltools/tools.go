package skilltools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/audit"
	"github.com/icoo-ai/icoo-ai/internal/policy"
	"github.com/icoo-ai/icoo-ai/internal/skills"
	"github.com/icoo-ai/icoo-ai/internal/subagent"
	"github.com/icoo-ai/icoo-ai/internal/tools"
)

const skillFileName = "SKILL.md"

type Options struct {
	Sources       []skills.Source
	WorkspaceRoot string
	CWD           string
	Model         string
	Policy        policy.Policy
	Runner        subagent.Runner
	Approver      agent.Approver
	AuditLogger   audit.Logger
	Now           func() time.Time
}

func NewTools(opts Options) []tools.Tool {
	m := newManager(opts)
	return []tools.Tool{
		skillListTool{manager: m},
		skillGetTool{manager: m},
		skillAddTool{manager: m},
		skillDeleteTool{manager: m},
		skillExecuteTool{manager: m},
	}
}

type manager struct {
	sources       []skills.Source
	workspaceRoot string
	cwd           string
	model         string
	policy        policy.Policy
	runner        subagent.Runner
	approver      agent.Approver
	auditLogger   audit.Logger
	now           func() time.Time
}

func newManager(opts Options) *manager {
	p := opts.Policy
	if p == nil {
		p = policy.New(policy.DefaultPermissionMode)
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &manager{
		sources:       append([]skills.Source(nil), opts.Sources...),
		workspaceRoot: opts.WorkspaceRoot,
		cwd:           opts.CWD,
		model:         opts.Model,
		policy:        p,
		runner:        opts.Runner,
		approver:      opts.Approver,
		auditLogger:   opts.AuditLogger,
		now:           now,
	}
}

func (m *manager) discover() ([]agent.Skill, error) {
	return skills.Discover(skills.DiscoverOptions{
		Sources:        m.sources,
		ConflictPolicy: skills.ConflictPreferHigherPriority,
	})
}

func (m *manager) findByName(name string) (agent.Skill, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return agent.Skill{}, errors.New("skill name is required")
	}
	found, err := m.discover()
	if err != nil {
		return agent.Skill{}, err
	}
	for _, skill := range found {
		if skill.Name == name {
			return skill, nil
		}
	}
	return agent.Skill{}, fmt.Errorf("skill %q was not found", name)
}

func (m *manager) writableRoot(scope string) (skills.Source, string, error) {
	scope = strings.TrimSpace(strings.ToLower(scope))
	if scope == "" {
		scope = string(skills.SourceProject)
	}
	var want skills.SourceKind
	switch scope {
	case string(skills.SourceProject):
		want = skills.SourceProject
	case string(skills.SourceUser):
		want = skills.SourceUser
	case string(skills.SourceCustom):
		want = skills.SourceCustom
	default:
		return skills.Source{}, "", fmt.Errorf("unsupported skill scope %q", scope)
	}
	for i := len(m.sources) - 1; i >= 0; i-- {
		source := m.sources[i]
		if source.Kind == want && strings.TrimSpace(source.Path) != "" {
			root, err := filepath.Abs(source.Path)
			if err != nil {
				return skills.Source{}, "", err
			}
			return source, filepath.Clean(root), nil
		}
	}
	return skills.Source{}, "", fmt.Errorf("skill scope %q is not configured", scope)
}

func (m *manager) writableRootForPath(path string) (skills.Source, string, bool) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return skills.Source{}, "", false
	}
	absPath = filepath.Clean(absPath)
	for _, source := range m.sources {
		if source.Kind == skills.SourceBuiltin || strings.TrimSpace(source.Path) == "" {
			continue
		}
		root, err := filepath.Abs(source.Path)
		if err != nil {
			continue
		}
		root = filepath.Clean(root)
		if isSubpath(root, absPath) {
			return source, root, true
		}
	}
	return skills.Source{}, "", false
}

type skillListTool struct{ manager *manager }
type skillGetTool struct{ manager *manager }
type skillAddTool struct{ manager *manager }
type skillDeleteTool struct{ manager *manager }
type skillExecuteTool struct{ manager *manager }

func (t skillListTool) Name() string { return "skill_list" }
func (t skillListTool) Description() string {
	return "List discovered skills from built-in, user, project, and configured custom skill directories."
}
func (t skillListTool) Definition() tools.ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","properties":{}}`)
}
func (t skillListTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	found, err := t.manager.discover()
	if err != nil {
		return toolError("skill_discover_failed", err.Error(), nil), nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })
	lines := make([]string, 0, len(found))
	for _, skill := range found {
		lines = append(lines, fmt.Sprintf("%s - %s", skill.Name, skill.Description))
	}
	return tools.ToolResult{OK: true, Content: strings.Join(lines, "\n"), Data: map[string]any{"skills": found}}, nil
}

func (t skillGetTool) Name() string { return "skill_get" }
func (t skillGetTool) Description() string {
	return "Load a skill by name, including its SKILL.md body and indexed resource paths."
}
func (t skillGetTool) Definition() tools.ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
}
func (t skillGetTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	skill, err := t.manager.findByName(req.Name)
	if err != nil {
		return toolError("skill_not_found", err.Error(), nil), nil
	}
	loaded, err := skills.LoadDiscovered(skill)
	if err != nil {
		return toolError("skill_load_failed", err.Error(), nil), nil
	}
	return tools.ToolResult{
		OK:      true,
		Content: loaded.Body,
		Data:    map[string]any{"skill": loaded},
	}, nil
}

func (t skillAddTool) Name() string { return "skill_add" }
func (t skillAddTool) Description() string {
	return "Create a new Codex-style skill directory with a SKILL.md file in the project, user, or custom skill scope."
}
func (t skillAddTool) Definition() tools.ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["name","description","body"],"properties":{"name":{"type":"string"},"description":{"type":"string"},"body":{"type":"string"},"scope":{"type":"string","enum":["project","user","custom"]}}}`)
}
func (t skillAddTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Body        string `json:"body"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return toolError("invalid_json", err.Error(), nil), nil
	}
	name := strings.TrimSpace(req.Name)
	description := strings.TrimSpace(req.Description)
	if name == "" || description == "" {
		return toolError("invalid_input", "name and description are required", nil), nil
	}
	dirName, err := safeSkillDirName(name)
	if err != nil {
		return toolError("invalid_input", err.Error(), nil), nil
	}
	source, root, err := t.manager.writableRoot(req.Scope)
	if err != nil {
		return toolError("skill_scope_unavailable", err.Error(), nil), nil
	}
	skillDir := filepath.Join(root, dirName)
	if !isSubpath(root, skillDir) {
		return toolError("outside_skill_root", "skill path escaped the configured skill root", nil), nil
	}
	decision := t.manager.policy.EvaluatePath(policy.PathRequest{
		Path:          filepath.Join(skillDir, skillFileName),
		WorkspaceRoot: root,
		Operation:     policy.PathOperationWrite,
	})
	if decision.Action == policy.DecisionBlock || decision.Action == policy.DecisionRequestApproval {
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}
	if _, err := os.Stat(skillDir); err == nil {
		return toolError("skill_exists", "skill directory already exists", map[string]any{"path": skillDir}), nil
	} else if !os.IsNotExist(err) {
		return toolError("skill_create_failed", err.Error(), nil), nil
	}
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return toolError("skill_create_failed", err.Error(), nil), nil
	}
	content := skillMarkdown(name, description, req.Body)
	if err := os.WriteFile(filepath.Join(skillDir, skillFileName), []byte(content), 0o644); err != nil {
		return toolError("skill_create_failed", err.Error(), nil), nil
	}
	loaded, err := skills.Load(skillDir)
	if err != nil {
		return toolError("skill_load_failed", err.Error(), nil), nil
	}
	loaded.Metadata = map[string]any{"source": string(source.Kind), "priority": source.Priority}
	return tools.ToolResult{
		OK:      true,
		Content: "created skill " + loaded.Name,
		Data: map[string]any{
			"skill": loaded,
			"path":  skillDir,
		},
	}, nil
}

func (t skillDeleteTool) Name() string { return "skill_delete" }
func (t skillDeleteTool) Description() string {
	return "Delete a writable project, user, or custom skill directory after policy approval when required."
}
func (t skillDeleteTool) Definition() tools.ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`)
}
func (t skillDeleteTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	return t.execute(ctx, input, false)
}
func (t skillDeleteTool) ApprovalKey(input json.RawMessage) (string, bool) {
	skill, root, err := t.resolveDeleteTarget(input)
	if err != nil {
		return "", false
	}
	return root + "\x00" + skill.Path, true
}
func (t skillDeleteTool) ExecuteApproved(ctx context.Context, input json.RawMessage, scope tools.ApprovalScope) (tools.ToolResult, error) {
	_ = scope
	return t.execute(ctx, input, true)
}
func (t skillDeleteTool) execute(ctx context.Context, input json.RawMessage, approved bool) (tools.ToolResult, error) {
	skill, root, err := t.resolveDeleteTarget(input)
	if err != nil {
		return toolError("skill_delete_failed", err.Error(), nil), nil
	}
	decision := t.manager.policy.EvaluatePath(policy.PathRequest{
		Path:          skill.Path,
		WorkspaceRoot: root,
		Operation:     policy.PathOperationDelete,
	})
	if decision.Action == policy.DecisionBlock {
		return toolError("policy_blocked", decision.Reason, decision.Details), nil
	}
	if decision.Action == policy.DecisionRequestApproval && !approved {
		return toolError("approval_required", decision.Reason, decision.Details), nil
	}
	select {
	case <-ctx.Done():
		return tools.ToolResult{}, ctx.Err()
	default:
	}
	if err := os.RemoveAll(skill.Path); err != nil {
		return toolError("skill_delete_failed", err.Error(), nil), nil
	}
	return tools.ToolResult{OK: true, Content: "deleted skill " + skill.Name, Data: map[string]any{"path": skill.Path}}, nil
}
func (t skillDeleteTool) resolveDeleteTarget(input json.RawMessage) (agent.Skill, string, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return agent.Skill{}, "", err
	}
	skill, err := t.manager.findByName(req.Name)
	if err != nil {
		return agent.Skill{}, "", err
	}
	_, root, ok := t.manager.writableRootForPath(skill.Path)
	if !ok {
		return agent.Skill{}, "", fmt.Errorf("skill %q is not in a writable skill source", skill.Name)
	}
	if !fileExists(filepath.Join(skill.Path, skillFileName)) {
		return agent.Skill{}, "", fmt.Errorf("skill %q is missing SKILL.md", skill.Name)
	}
	return skill, root, nil
}

func (t skillExecuteTool) Name() string { return "skill_execute" }
func (t skillExecuteTool) Description() string {
	return "Execute a named skill by delegating the task and skill instructions to a subagent."
}
func (t skillExecuteTool) Definition() tools.ToolDefinition {
	return definition(t.Name(), t.Description(), `{"type":"object","required":["name","task"],"properties":{"name":{"type":"string"},"task":{"type":"string"},"context":{"type":"array","items":{"type":"string"}},"max_tool_rounds":{"type":"integer"}}}`)
}
func (t skillExecuteTool) Execute(ctx context.Context, input json.RawMessage) (tools.ToolResult, error) {
	if t.manager.runner == nil {
		return toolError("subagent_unavailable", "subagent runner is not configured", nil), nil
	}
	var req struct {
		Name          string   `json:"name"`
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
	skill, err := t.manager.findByName(req.Name)
	if err != nil {
		return toolError("skill_not_found", err.Error(), nil), nil
	}
	loaded, err := skills.LoadDiscovered(skill)
	if err != nil {
		return toolError("skill_load_failed", err.Error(), nil), nil
	}
	_ = skills.LogSkillUse(ctx, t.manager.auditLogger, "skill_execute", loaded)
	startedAt := t.manager.now().UTC()
	sessionID := subagent.NewSessionID("skill-" + loaded.Name)
	result, err := t.manager.runner.Run(ctx, subagent.Request{
		SessionID:     sessionID,
		CWD:           t.manager.cwd,
		Task:          req.Task,
		Skill:         &loaded,
		Context:       req.Context,
		Model:         t.manager.model,
		MaxToolRounds: req.MaxToolRounds,
		Approver:      t.manager.approver,
		Metadata:      map[string]any{"skill": loaded.Name},
	})
	if err != nil {
		_ = t.manager.logSubagent(ctx, startedAt, false, err.Error(), loaded.Name)
		return toolError("skill_execute_failed", err.Error(), nil), nil
	}
	_ = t.manager.logSubagent(ctx, startedAt, true, "", loaded.Name)
	return tools.ToolResult{
		OK:      true,
		Content: result.Content,
		Data: map[string]any{
			"skill":       loaded.Name,
			"content":     result.Content,
			"event_count": len(result.Events),
			"session_id":  sessionID,
		},
	}, nil
}

func (m *manager) logSubagent(ctx context.Context, at time.Time, ok bool, errText, skillName string) error {
	if m.auditLogger == nil {
		return nil
	}
	data := map[string]any{"ok": ok, "skill": skillName}
	if errText != "" {
		data["error"] = errText
	}
	return m.auditLogger.Log(ctx, audit.Event{
		Type:      audit.EventSubagentRun,
		Timestamp: at.UTC(),
		Summary:   "skill subagent run: " + skillName,
		Data:      data,
	})
}

func definition(name, description, schema string) tools.ToolDefinition {
	return tools.ToolDefinition{Name: name, Description: description, InputSchema: json.RawMessage(schema)}
}

func toolError(code, message string, data map[string]any) tools.ToolResult {
	if data == nil {
		data = map[string]any{}
	}
	data["code"] = code
	return tools.ToolResult{OK: false, Error: message, Data: data}
}

func safeSkillDirName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "", errors.New("invalid skill name")
	}
	for _, r := range name {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
		if !ok {
			return "", fmt.Errorf("skill name %q contains unsupported characters", name)
		}
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return "", errors.New("skill name must not contain path traversal")
	}
	return name, nil
}

func skillMarkdown(name, description, body string) string {
	return "---\nname: " + yamlQuote(name) + "\ndescription: " + yamlQuote(description) + "\n---\n\n" + strings.TrimLeft(body, "\r\n") + "\n"
}

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	return `"` + value + `"`
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isSubpath(root, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if samePath(root, path) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func samePath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
