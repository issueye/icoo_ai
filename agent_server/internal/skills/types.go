package skills

import (
	"path/filepath"
)

const (
	UserSkillsPath    = ".icoo-ai/skills"
	ProjectSkillsPath = ".icoo-ai/skills"
)

type SourceKind string

const (
	SourceBuiltin SourceKind = "builtin"
	SourceUser    SourceKind = "user"
	SourceProject SourceKind = "project"
	SourceCustom  SourceKind = "custom"
)

type Source struct {
	Kind     SourceKind
	Path     string
	Priority int
}

type SourceOptions struct {
	BuiltinDir       string
	HomeDir          string
	UserDir          string
	ProjectDir       string
	ProjectSkillsDir string
	CustomDirs       []string
}

type ConflictPolicy int

const (
	ConflictPolicyError ConflictPolicy = iota
	ConflictPreferHigherPriority
)

type DiscoverOptions struct {
	Sources        []Source
	ConflictPolicy ConflictPolicy
}

type SkillRef struct {
	Name     string
	Path     string
	Source   SourceKind
	Priority int
}

type ConflictError struct {
	Name      string
	Existing  SkillRef
	Duplicate SkillRef
}

func (e *ConflictError) Error() string {
	return "skill conflict for " + e.Name + ": " + e.Existing.Path + " conflicts with " + e.Duplicate.Path
}

func DefaultSources(opts SourceOptions) []Source {
	var sources []Source
	if opts.BuiltinDir != "" {
		sources = append(sources, Source{Kind: SourceBuiltin, Path: opts.BuiltinDir, Priority: 10})
	}
	userDir := opts.UserDir
	if userDir == "" && opts.HomeDir != "" {
		userDir = filepath.Join(opts.HomeDir, filepath.FromSlash(UserSkillsPath))
	}
	if userDir != "" {
		sources = append(sources, Source{Kind: SourceUser, Path: userDir, Priority: 20})
	}
	projectDir := opts.ProjectSkillsDir
	if projectDir == "" && opts.ProjectDir != "" {
		projectDir = filepath.Join(opts.ProjectDir, filepath.FromSlash(ProjectSkillsPath))
	}
	if projectDir != "" {
		sources = append(sources, Source{Kind: SourceProject, Path: projectDir, Priority: 30})
	}
	for i, dir := range opts.CustomDirs {
		if dir == "" {
			continue
		}
		sources = append(sources, Source{Kind: SourceCustom, Path: dir, Priority: 40 + i})
	}
	return sources
}
