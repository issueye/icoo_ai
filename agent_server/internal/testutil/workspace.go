package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

type WorkspaceFixture struct {
	Root string
}

func NewWorkspaceFixture(t testing.TB) *WorkspaceFixture {
	t.Helper()

	root := t.TempDir()
	return &WorkspaceFixture{Root: root}
}

func (w *WorkspaceFixture) Path(parts ...string) string {
	all := append([]string{w.Root}, parts...)
	return filepath.Join(all...)
}

func (w *WorkspaceFixture) WriteFile(t testing.TB, rel string, content string) string {
	t.Helper()

	path := w.Path(rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture file: %v", err)
	}
	return path
}

func (w *WorkspaceFixture) Mkdir(t testing.TB, rel string) string {
	t.Helper()

	path := w.Path(rel)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("create fixture dir: %v", err)
	}
	return path
}

func (w *WorkspaceFixture) ReadFile(t testing.TB, rel string) string {
	t.Helper()

	content, err := os.ReadFile(w.Path(rel))
	if err != nil {
		t.Fatalf("read fixture file: %v", err)
	}
	return string(content)
}
