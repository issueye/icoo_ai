package testutil

import (
	"os"
	"testing"
)

func TestWorkspaceFixtureWritesAndReadsFiles(t *testing.T) {
	workspace := NewWorkspaceFixture(t)

	path := workspace.WriteFile(t, "dir/file.txt", "content")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat fixture file: %v", err)
	}

	if got := workspace.ReadFile(t, "dir/file.txt"); got != "content" {
		t.Fatalf("content = %q, want content", got)
	}
}

func TestWorkspaceFixtureMkdir(t *testing.T) {
	workspace := NewWorkspaceFixture(t)

	path := workspace.Mkdir(t, "nested/dir")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat fixture dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", path)
	}
}
