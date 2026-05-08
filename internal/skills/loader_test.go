package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/icoo-ai/icoo-ai/internal/audit"
)

func TestDefaultSourcesUseCallerProvidedRoots(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	project := filepath.Join(t.TempDir(), "project")
	builtin := filepath.Join(t.TempDir(), "builtin")

	sources := DefaultSources(SourceOptions{
		BuiltinDir: builtin,
		HomeDir:    home,
		ProjectDir: project,
		CustomDirs: []string{filepath.Join(t.TempDir(), "custom")},
	})

	if len(sources) != 4 {
		t.Fatalf("len(sources) = %d, want 4", len(sources))
	}
	if sources[0].Path != builtin || sources[0].Kind != SourceBuiltin {
		t.Fatalf("builtin source = %+v", sources[0])
	}
	if sources[1].Path != filepath.Join(home, filepath.FromSlash(UserSkillsPath)) || sources[1].Kind != SourceUser {
		t.Fatalf("user source = %+v", sources[1])
	}
	if sources[2].Path != filepath.Join(project, filepath.FromSlash(ProjectSkillsPath)) || sources[2].Kind != SourceProject {
		t.Fatalf("project source = %+v", sources[2])
	}
	if sources[3].Kind != SourceCustom || sources[3].Priority <= sources[2].Priority {
		t.Fatalf("custom source = %+v", sources[3])
	}
}

func TestDiscoverLoadsMetadataAndIndexesResourcesOnly(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "go-review"), "go-review", "Review Go code", "# Body\nUse this later.\n")
	writeFile(t, filepath.Join(root, "go-review", "scripts", "lint.ps1"), "go test ./...\n")
	writeFile(t, filepath.Join(root, "go-review", "references", "large.md"), strings.Repeat("reference ", 1000))
	writeFile(t, filepath.Join(root, "go-review", "assets", "icon.png"), "png")
	writeFile(t, filepath.Join(root, "missing-skill", "README.md"), "ignored")

	found, err := Discover(DiscoverOptions{
		Sources: []Source{{Kind: SourceBuiltin, Path: root, Priority: 10}},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("len(found) = %d, want 1: %+v", len(found), found)
	}
	skill := found[0]
	if skill.Name != "go-review" || skill.Description != "Review Go code" {
		t.Fatalf("skill metadata = %+v", skill)
	}
	if skill.Body != "" {
		t.Fatalf("Discover() loaded body %q, want empty", skill.Body)
	}
	if got := strings.Join(skill.Resources.Scripts, ","); got != "scripts/lint.ps1" {
		t.Fatalf("scripts = %q", got)
	}
	if got := strings.Join(skill.Resources.References, ","); got != "references/large.md" {
		t.Fatalf("references = %q", got)
	}
	if got := strings.Join(skill.Resources.Assets, ","); got != "assets/icon.png" {
		t.Fatalf("assets = %q", got)
	}
}

func TestLoadDiscoveredLoadsBodyProgressively(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "review"), "review", "Review code", "# Review\nInstructions.\n")

	found, err := Discover(DiscoverOptions{
		Sources: []Source{{Kind: SourceUser, Path: root, Priority: 20}},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	loaded, err := LoadDiscovered(found[0])
	if err != nil {
		t.Fatalf("LoadDiscovered() error = %v", err)
	}
	if loaded.Body != "# Review\nInstructions.\n" {
		t.Fatalf("Body = %q", loaded.Body)
	}
	if loaded.Metadata["source"] != "user" || loaded.Metadata["priority"] != 20 {
		t.Fatalf("metadata = %+v", loaded.Metadata)
	}
}

func TestDiscoverReturnsConflictErrorByDefault(t *testing.T) {
	builtin := t.TempDir()
	project := t.TempDir()
	writeSkill(t, filepath.Join(builtin, "one"), "same", "Built in", "body")
	writeSkill(t, filepath.Join(project, "two"), "same", "Project", "body")

	_, err := Discover(DiscoverOptions{
		Sources: []Source{
			{Kind: SourceBuiltin, Path: builtin, Priority: 10},
			{Kind: SourceProject, Path: project, Priority: 30},
		},
	})
	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("Discover() error = %v, want ConflictError", err)
	}
	if conflict.Name != "same" || !strings.Contains(conflict.Error(), "same") {
		t.Fatalf("conflict = %+v", conflict)
	}
}

func TestDiscoverCanPreferHigherPriorityConflict(t *testing.T) {
	builtin := t.TempDir()
	project := t.TempDir()
	writeSkill(t, filepath.Join(builtin, "one"), "same", "Built in", "built in body")
	writeSkill(t, filepath.Join(project, "two"), "same", "Project", "project body")

	found, err := Discover(DiscoverOptions{
		ConflictPolicy: ConflictPreferHigherPriority,
		Sources: []Source{
			{Kind: SourceBuiltin, Path: builtin, Priority: 10},
			{Kind: SourceProject, Path: project, Priority: 30},
		},
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(found) != 1 || found[0].Description != "Project" {
		t.Fatalf("found = %+v, want project override", found)
	}
	if found[0].Metadata["source"] != "project" {
		t.Fatalf("metadata = %+v", found[0].Metadata)
	}
}

func TestDiscoverRejectsInvalidFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "bad", skillFileName), "---\nname: bad\n---\nbody\n")

	_, err := Discover(DiscoverOptions{
		Sources: []Source{{Kind: SourceUser, Path: root, Priority: 20}},
	})
	if err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("Discover() error = %v, want missing description", err)
	}
}

func TestLogSkillUseWritesAuditEvent(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "audit-skill", "Auditable", "body")
	skill, err := Load(root)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger := audit.NewJSONLLogger(path)

	if err := LogSkillUse(context.Background(), logger, "s1", skill); err != nil {
		t.Fatalf("LogSkillUse() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if !strings.Contains(string(data), `"type":"skill_use"`) || !strings.Contains(string(data), `"name":"audit-skill"`) {
		t.Fatalf("audit line = %s", data)
	}
}

func writeSkill(t *testing.T, dir, name, description, body string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, skillFileName), "---\nname: "+name+"\ndescription: "+description+"\n---\n\n"+body)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
