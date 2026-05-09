package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverUsesGitRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	ws, err := Discover(nested)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if ws.Root != root {
		t.Fatalf("root = %q, want %q", ws.Root, root)
	}
	if ws.GitRoot != root {
		t.Fatalf("git root = %q, want %q", ws.GitRoot, root)
	}
}

func TestResolveBlocksPathTraversal(t *testing.T) {
	root := t.TempDir()
	ws, err := New(Options{Root: root})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if _, err := ws.Resolve("../outside.txt"); err == nil {
		t.Fatal("Resolve traversal succeeded, want error")
	}
}

func TestIgnoreMatcherReadsGitignoreAndIcooignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".gitignore"), "vendor/\n*.log\n")
	writeFile(t, filepath.Join(root, ".icooignore"), "private/\n")

	ws, err := New(Options{Root: root})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		rel   string
		isDir bool
		want  bool
	}{
		{rel: "vendor", isDir: true, want: true},
		{rel: "vendor/pkg/file.go", want: true},
		{rel: "debug.log", want: true},
		{rel: "private/notes.txt", want: true},
		{rel: "src/main.go", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.rel, func(t *testing.T) {
			if got := ws.IsIgnored(tt.rel, tt.isDir); got != tt.want {
				t.Fatalf("IsIgnored() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWalkSkipsIgnoredAndHiddenFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".icooignore"), "ignored/\n")
	writeFile(t, filepath.Join(root, "keep.txt"), "keep")
	writeFile(t, filepath.Join(root, "ignored", "skip.txt"), "skip")
	writeFile(t, filepath.Join(root, ".hidden"), "hidden")

	ws, err := New(Options{Root: root})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	entries, err := ws.ListFiles(WalkOptions{})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Path] = true
	}
	if !seen["keep.txt"] {
		t.Fatalf("keep.txt not listed: %+v", entries)
	}
	if seen["ignored"] || seen["ignored/skip.txt"] || seen[".hidden"] {
		t.Fatalf("ignored or hidden file listed: %+v", entries)
	}
}

func TestWalkHonorsMaxDepth(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "top.txt"), "top")
	writeFile(t, filepath.Join(root, "nested", "child.txt"), "child")

	ws, err := New(Options{Root: root})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	entries, err := ws.ListFiles(WalkOptions{MaxDepth: 1})
	if err != nil {
		t.Fatalf("ListFiles() error = %v", err)
	}

	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Path] = true
	}
	if !seen["top.txt"] || !seen["nested"] {
		t.Fatalf("top-level entries missing: %+v", entries)
	}
	if seen["nested/child.txt"] {
		t.Fatalf("nested child listed despite MaxDepth=1: %+v", entries)
	}
}

func TestSecretPathDetection(t *testing.T) {
	paths := []string{
		".env",
		"keys/id_rsa",
		"certs/service.pem",
		filepath.Join("home", ".ssh", "config"),
	}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			if !IsSecretPath(p) {
				t.Fatalf("IsSecretPath(%q) = false, want true", p)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
