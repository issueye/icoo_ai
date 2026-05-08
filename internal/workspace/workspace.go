package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrOutsideWorkspace = errors.New("path is outside workspace")
	ErrSecretPath       = errors.New("secret paths are blocked by default")
)

type Workspace struct {
	Root    string
	GitRoot string
	CWD     string
	Ignore  *IgnoreMatcher
}

type Options struct {
	Root string
	CWD  string
}

type ResolvedPath struct {
	Abs string
	Rel string
}

func Discover(cwd string) (*Workspace, error) {
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get current directory: %w", err)
		}
	}
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve cwd: %w", err)
	}
	absCWD = filepath.Clean(absCWD)

	root := absCWD
	gitRoot, ok := findGitRoot(absCWD)
	if ok {
		root = gitRoot
	}
	return New(Options{Root: root, CWD: absCWD})
}

func New(opts Options) (*Workspace, error) {
	root := opts.Root
	if root == "" {
		root = opts.CWD
	}
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get current directory: %w", err)
		}
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)

	cwd := opts.CWD
	if cwd == "" {
		cwd = absRoot
	}
	absCWD, err := filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace cwd: %w", err)
	}
	absCWD = filepath.Clean(absCWD)
	if !isSubpath(absRoot, absCWD) {
		return nil, fmt.Errorf("workspace cwd %q is outside root %q", absCWD, absRoot)
	}

	gitRoot, _ := findGitRoot(absRoot)
	matcher, err := LoadIgnoreMatcher(absRoot)
	if err != nil {
		return nil, err
	}
	return &Workspace{
		Root:    absRoot,
		GitRoot: gitRoot,
		CWD:     absCWD,
		Ignore:  matcher,
	}, nil
}

func (w *Workspace) Resolve(input string) (ResolvedPath, error) {
	if w == nil {
		return ResolvedPath{}, errors.New("workspace is nil")
	}
	if input == "" {
		input = "."
	}
	var candidate string
	if filepath.IsAbs(input) {
		candidate = input
	} else {
		candidate = filepath.Join(w.Root, input)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("resolve path: %w", err)
	}
	abs = filepath.Clean(abs)
	if !isSubpath(w.Root, abs) {
		return ResolvedPath{}, ErrOutsideWorkspace
	}
	rel, err := filepath.Rel(w.Root, abs)
	if err != nil {
		return ResolvedPath{}, fmt.Errorf("make relative path: %w", err)
	}
	if rel == "." {
		rel = ""
	}
	return ResolvedPath{Abs: abs, Rel: filepath.ToSlash(rel)}, nil
}

func (w *Workspace) IsIgnored(rel string, isDir bool) bool {
	if w == nil || w.Ignore == nil {
		return false
	}
	return w.Ignore.Match(filepath.ToSlash(rel), isDir)
}

func (w *Workspace) IsSecret(relOrAbs string) bool {
	if filepath.IsAbs(relOrAbs) {
		return IsSecretPath(relOrAbs)
	}
	return IsSecretPath(filepath.Join(w.Root, filepath.FromSlash(relOrAbs)))
}

func IsSecretPath(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".pem" || ext == ".key" || ext == ".p12" || ext == ".pfx" {
		return true
	}
	secretNames := map[string]struct{}{
		".env":                 {},
		".env.local":           {},
		".env.production":      {},
		".npmrc":               {},
		".pypirc":              {},
		".netrc":               {},
		"id_rsa":               {},
		"id_dsa":               {},
		"id_ecdsa":             {},
		"id_ed25519":           {},
		"credentials":          {},
		"credentials.json":     {},
		"service-account.json": {},
	}
	if _, ok := secretNames[base]; ok {
		return true
	}
	normalized := strings.ToLower(filepath.ToSlash(path))
	return strings.Contains(normalized, "/.ssh/") ||
		strings.Contains(normalized, "/.aws/credentials") ||
		strings.Contains(normalized, "/.config/gcloud/")
}

func findGitRoot(start string) (string, bool) {
	dir := filepath.Clean(start)
	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if samePath(parent, dir) {
			return "", false
		}
		dir = parent
	}
}

func isSubpath(root, path string) bool {
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
