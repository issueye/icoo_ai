package skills

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/icoo-ai/icoo-ai/internal/agent"
	"github.com/icoo-ai/icoo-ai/internal/audit"
)

const skillFileName = "SKILL.md"

type candidate struct {
	path     string
	source   SourceKind
	priority int
}

func Discover(opts DiscoverOptions) ([]agent.Skill, error) {
	policy := opts.ConflictPolicy
	if policy != ConflictPolicyError && policy != ConflictPreferHigherPriority {
		policy = ConflictPolicyError
	}

	byName := map[string]agent.Skill{}
	refs := map[string]SkillRef{}
	for _, source := range opts.Sources {
		candidates, err := sourceCandidates(source)
		if err != nil {
			return nil, err
		}
		for _, item := range candidates {
			skill, err := readSkill(item.path, false)
			if err != nil {
				return nil, err
			}
			skill.Metadata = metadata(item.source, item.priority)
			ref := SkillRef{Name: skill.Name, Path: skill.Path, Source: item.source, Priority: item.priority}
			if existing, ok := byName[skill.Name]; ok {
				existingRef := refs[skill.Name]
				if policy == ConflictPreferHigherPriority && item.priority != existingRef.Priority {
					if item.priority > existingRef.Priority {
						byName[skill.Name] = skill
						refs[skill.Name] = ref
					}
					continue
				}
				return nil, &ConflictError{
					Name:      skill.Name,
					Existing:  SkillRef{Name: existing.Name, Path: existing.Path, Source: existingRef.Source, Priority: existingRef.Priority},
					Duplicate: ref,
				}
			}
			byName[skill.Name] = skill
			refs[skill.Name] = ref
		}
	}

	out := make([]agent.Skill, 0, len(byName))
	for _, skill := range byName {
		out = append(out, skill)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func Load(path string) (agent.Skill, error) {
	return readSkill(path, true)
}

func LoadDiscovered(skill agent.Skill) (agent.Skill, error) {
	loaded, err := Load(skill.Path)
	if err != nil {
		return agent.Skill{}, err
	}
	loaded.Metadata = cloneMetadata(skill.Metadata)
	return loaded, nil
}

func SkillUseEvent(sessionID string, skill agent.Skill) audit.Event {
	return audit.Event{
		Type:      audit.EventSkillUse,
		SessionID: sessionID,
		Summary:   "skill injected: " + skill.Name,
		Data: map[string]any{
			"name":        skill.Name,
			"description": skill.Description,
			"path":        skill.Path,
			"resources":   skill.Resources,
			"metadata":    skill.Metadata,
			"body_bytes":  len(skill.Body),
		},
	}
}

func LogSkillUse(ctx context.Context, logger audit.Logger, sessionID string, skill agent.Skill) error {
	if logger == nil {
		return nil
	}
	return logger.Log(ctx, SkillUseEvent(sessionID, skill))
}

func sourceCandidates(source Source) ([]candidate, error) {
	if source.Path == "" {
		return nil, nil
	}
	root, err := filepath.Abs(source.Path)
	if err != nil {
		return nil, fmt.Errorf("resolve skill source %q: %w", source.Path, err)
	}
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat skill source %q: %w", root, err)
	}
	if !info.IsDir() {
		return nil, nil
	}
	if fileExists(filepath.Join(root, skillFileName)) {
		return []candidate{{path: root, source: source.Kind, priority: source.Priority}}, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read skill source %q: %w", root, err)
	}
	var out []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if fileExists(filepath.Join(path, skillFileName)) {
			out = append(out, candidate{path: path, source: source.Kind, priority: source.Priority})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].path < out[j].path
	})
	return out, nil
}

func readSkill(path string, includeBody bool) (agent.Skill, error) {
	root, err := filepath.Abs(path)
	if err != nil {
		return agent.Skill{}, fmt.Errorf("resolve skill path %q: %w", path, err)
	}
	data, err := os.ReadFile(filepath.Join(root, skillFileName))
	if err != nil {
		return agent.Skill{}, fmt.Errorf("read %s: %w", filepath.Join(root, skillFileName), err)
	}
	parsed, err := parseSkillMarkdown(data)
	if err != nil {
		return agent.Skill{}, fmt.Errorf("%s: %w", filepath.Join(root, skillFileName), err)
	}
	resources, err := indexResources(root)
	if err != nil {
		return agent.Skill{}, err
	}
	body := ""
	if includeBody {
		body = parsed.Body
	}
	return agent.Skill{
		Name:        parsed.Name,
		Description: parsed.Description,
		Path:        root,
		Body:        body,
		Resources:   resources,
	}, nil
}

func indexResources(root string) (agent.SkillResources, error) {
	scripts, err := indexResourceDir(root, "scripts")
	if err != nil {
		return agent.SkillResources{}, err
	}
	references, err := indexResourceDir(root, "references")
	if err != nil {
		return agent.SkillResources{}, err
	}
	assets, err := indexResourceDir(root, "assets")
	if err != nil {
		return agent.SkillResources{}, err
	}
	return agent.SkillResources{
		Scripts:    scripts,
		References: references,
		Assets:     assets,
	}, nil
}

func indexResourceDir(root, name string) ([]string, error) {
	dir := filepath.Join(root, name)
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat skill resource dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, nil
	}

	var paths []string
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("index skill resource dir %q: %w", dir, err)
	}
	sort.Strings(paths)
	return paths, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func metadata(source SourceKind, priority int) map[string]any {
	return map[string]any{
		"source":   string(source),
		"priority": priority,
	}
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
