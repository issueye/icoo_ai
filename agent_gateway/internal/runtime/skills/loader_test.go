package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLoaderLoadSkill(t *testing.T) {
	dir := writeSkill(t, "writer", `---
name: writer
description: Writes concise copy
tags: [copy, writing]
---
# Instructions

Keep it short.
`)

	skill, err := NewLoader().Load(context.Background(), dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if skill.ID != "writer" {
		t.Fatalf("ID = %q", skill.ID)
	}
	if skill.ContentHash == "" {
		t.Fatal("ContentHash is empty")
	}
	if skill.Instructions != "# Instructions\n\nKeep it short." {
		t.Fatalf("Instructions = %q", skill.Instructions)
	}
	if len(skill.Manifest.Tags) != 2 || skill.Manifest.Tags[0] != "copy" {
		t.Fatalf("Tags = %#v", skill.Manifest.Tags)
	}
}

func TestLoaderRejectsInvalidSkill(t *testing.T) {
	dir := writeSkill(t, "bad", `---
name: Bad Name
description: Invalid
---
body
`)

	_, err := NewLoader().Load(context.Background(), dir)
	if err == nil {
		t.Fatal("Load() error = nil")
	}
}

func TestScannerScanReloadDocumentationAndRemoved(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "writer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "SKILL.md"), `---
name: writer
description: Writes concise copy
---
Version one.
`)

	registry := NewRegistry()
	store := &memorySkillStore{items: map[string]Skill{}}
	scanner := NewScanner(NewLoader(), registry, store, root)

	result, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Added) != 1 || result.Added[0].ID != "writer" {
		t.Fatalf("Added = %#v", result.Added)
	}
	if _, ok := store.items["writer"]; !ok {
		t.Fatal("store was not synced")
	}

	writeFile(t, filepath.Join(dir, "SKILL.md"), `---
name: writer
description: Writes concise copy
---
Version two.
`)
	reloaded, err := scanner.Reload(context.Background(), "writer")
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if reloaded.Instructions != "Version two." {
		t.Fatalf("Instructions = %q", reloaded.Instructions)
	}

	doc, err := scanner.Documentation(context.Background(), "writer")
	if err != nil {
		t.Fatalf("Documentation() error = %v", err)
	}
	if doc != "Version two." {
		t.Fatalf("doc = %q", doc)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	result, err = scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() after remove error = %v", err)
	}
	if len(result.Removed) != 1 || result.Removed[0] != "writer" {
		t.Fatalf("Removed = %#v", result.Removed)
	}
	if !store.missing["writer"] {
		t.Fatal("removed skill was not marked missing")
	}
}

func TestScannerInvalidSkillReportsError(t *testing.T) {
	root := t.TempDir()
	writeSkillAt(t, root, "bad", `no frontmatter`)

	result, err := NewScanner(NewLoader(), NewRegistry(), nil, root).Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("Errors = %#v", result.Errors)
	}
}

type memorySkillStore struct {
	items   map[string]Skill
	missing map[string]bool
}

func (s *memorySkillStore) UpsertSkill(_ context.Context, skill Skill) error {
	s.items[skill.ID] = skill
	return nil
}

func (s *memorySkillStore) MarkSkillMissing(_ context.Context, id string) error {
	if s.missing == nil {
		s.missing = map[string]bool{}
	}
	s.missing[id] = true
	return nil
}

func writeSkill(t *testing.T, name string, content string) string {
	t.Helper()
	return writeSkillAt(t, t.TempDir(), name, content)
}

func writeSkillAt(t *testing.T, root, name string, content string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "SKILL.md"), content)
	return dir
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
