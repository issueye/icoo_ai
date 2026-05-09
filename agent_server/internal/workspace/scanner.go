package workspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type FileEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

type WalkOptions struct {
	Root          string
	MaxDepth      int
	IncludeHidden bool
}

func (w *Workspace) Walk(opts WalkOptions, visit func(FileEntry) error) error {
	resolved, err := w.Resolve(opts.Root)
	if err != nil {
		return err
	}
	baseDepth := depth(resolved.Rel)
	return filepath.WalkDir(resolved.Abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if samePath(path, resolved.Abs) {
			return nil
		}
		rel, err := filepath.Rel(w.Root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		isDir := d.IsDir()
		name := d.Name()
		if !opts.IncludeHidden && len(name) > 1 && name[0] == '.' {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		if w.IsIgnored(rel, isDir) {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		if opts.MaxDepth > 0 && depth(rel)-baseDepth > opts.MaxDepth {
			if isDir {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return visit(FileEntry{Path: rel, IsDir: isDir, Size: info.Size()})
	})
}

func (w *Workspace) ListFiles(opts WalkOptions) ([]FileEntry, error) {
	var entries []FileEntry
	err := w.Walk(opts, func(entry FileEntry) error {
		entries = append(entries, entry)
		return nil
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})
	return entries, err
}

func IsBinaryFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buf := make([]byte, 8000)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return false, err
	}
	for _, b := range buf[:n] {
		if b == 0 {
			return true, nil
		}
	}
	return false, nil
}

func depth(rel string) int {
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "" || rel == "." {
		return 0
	}
	return slashCount(rel) + 1
}

func slashCount(s string) int {
	n := 0
	for _, ch := range s {
		if ch == '/' {
			n++
		}
	}
	return n
}
