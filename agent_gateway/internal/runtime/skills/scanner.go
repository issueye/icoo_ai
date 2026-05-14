package skills

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"
)

type Store interface {
	UpsertSkill(ctx context.Context, skill Skill) error
	MarkSkillMissing(ctx context.Context, id string) error
}

type ScanError struct {
	Path  string
	Error string
}

type ScanResult struct {
	Added      []Skill
	Updated    []Skill
	Removed    []string
	Unchanged  []Skill
	Errors     []ScanError
	Duration   time.Duration
	ScannedDir []string
}

type Scanner struct {
	loader   *Loader
	registry *Registry
	store    Store
	roots    []string
}

func NewScanner(loader *Loader, registry *Registry, store Store, roots ...string) *Scanner {
	if loader == nil {
		loader = NewLoader()
	}
	if registry == nil {
		registry = NewRegistry()
	}
	return &Scanner{loader: loader, registry: registry, store: store, roots: roots}
}

func (s *Scanner) Scan(ctx context.Context) (ScanResult, error) {
	started := time.Now()
	result := ScanResult{}
	seen := map[string]struct{}{}

	dirs, err := s.discoverDirs(ctx)
	if err != nil {
		return result, err
	}
	result.ScannedDir = dirs

	for _, dir := range dirs {
		skill, err := s.loader.Load(ctx, dir)
		if err != nil {
			result.Errors = append(result.Errors, ScanError{Path: dir, Error: err.Error()})
			continue
		}
		seen[skill.ID] = struct{}{}

		existing, ok := s.registry.Get(skill.ID)
		switch {
		case !ok:
			result.Added = append(result.Added, skill)
			s.registry.Register(skill)
		case existing.ContentHash != skill.ContentHash || existing.BasePath != skill.BasePath:
			result.Updated = append(result.Updated, skill)
			s.registry.Register(skill)
		default:
			result.Unchanged = append(result.Unchanged, skill)
		}

		if s.store != nil {
			if err := s.store.UpsertSkill(ctx, skill); err != nil {
				result.Errors = append(result.Errors, ScanError{Path: dir, Error: err.Error()})
			}
		}
	}

	for _, known := range s.registry.Snapshot() {
		if _, ok := seen[known.ID]; ok {
			continue
		}
		if s.registry.Unregister(known.ID) {
			result.Removed = append(result.Removed, known.ID)
			if s.store != nil {
				if err := s.store.MarkSkillMissing(ctx, known.ID); err != nil {
					result.Errors = append(result.Errors, ScanError{Path: known.BasePath, Error: err.Error()})
				}
			}
		}
	}

	result.Duration = time.Since(started)
	return result, nil
}

func (s *Scanner) Reload(ctx context.Context, id string) (Skill, error) {
	current, ok := s.registry.Get(id)
	if !ok {
		return Skill{}, ErrSkillNotFound
	}
	loaded, err := s.loader.Load(ctx, current.BasePath)
	if err != nil {
		return Skill{}, err
	}
	if loaded.ID != id {
		return Skill{}, errors.New("reloaded skill id changed")
	}
	s.registry.Register(loaded)
	if s.store != nil {
		if err := s.store.UpsertSkill(ctx, loaded); err != nil {
			return Skill{}, err
		}
	}
	return loaded, nil
}

func (s *Scanner) Documentation(ctx context.Context, id string) (string, error) {
	skill, ok := s.registry.Get(id)
	if !ok {
		return "", ErrSkillNotFound
	}
	return s.loader.Documentation(ctx, skill)
}

func (s *Scanner) discoverDirs(ctx context.Context) ([]string, error) {
	dirs := make([]string, 0)
	for _, root := range s.roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		stat, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !stat.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "SKILL.md")); err == nil {
			dirs = append(dirs, root)
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if !entry.IsDir() || len(entry.Name()) == 0 || entry.Name()[0] == '.' {
				continue
			}
			dir := filepath.Join(root, entry.Name())
			if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
				dirs = append(dirs, dir)
			}
		}
	}
	slices.Sort(dirs)
	return slices.Compact(dirs), nil
}

var ErrSkillNotFound = errors.New("skill not found")
