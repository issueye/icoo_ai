package skills

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type Manifest struct {
	Name        string
	Description string
	Tags        []string
	Metadata    map[string]string
}

type Skill struct {
	ID           string
	BasePath     string
	Manifest     Manifest
	ContentHash  string
	ModifiedAt   time.Time
	Instructions string
}

type Loader struct{}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load(ctx context.Context, dir string) (Skill, error) {
	if err := ctx.Err(); err != nil {
		return Skill{}, err
	}

	path := filepath.Join(dir, "SKILL.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}

	manifest, body, err := parseSkillMarkdown(string(content))
	if err != nil {
		return Skill{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := validateManifest(manifest); err != nil {
		return Skill{}, fmt.Errorf("%s: %w", path, err)
	}

	stat, err := os.Stat(path)
	if err != nil {
		return Skill{}, err
	}

	sum := sha256.Sum256(content)
	return Skill{
		ID:           manifest.Name,
		BasePath:     dir,
		Manifest:     manifest,
		ContentHash:  hex.EncodeToString(sum[:]),
		ModifiedAt:   stat.ModTime(),
		Instructions: strings.TrimSpace(body),
	}, nil
}

func (l *Loader) Documentation(ctx context.Context, skill Skill) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if skill.BasePath == "" {
		return "", errors.New("skill base path is empty")
	}
	loaded, err := l.Load(ctx, skill.BasePath)
	if err != nil {
		return "", err
	}
	return loaded.Instructions, nil
}

func parseSkillMarkdown(content string) (Manifest, string, error) {
	if !strings.HasPrefix(content, "---") {
		return Manifest{}, "", errors.New("missing YAML frontmatter")
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return Manifest{}, "", errors.New("missing YAML frontmatter")
	}

	var frontmatter []string
	var body []string
	inFrontmatter := true
	for scanner.Scan() {
		line := scanner.Text()
		if inFrontmatter {
			if strings.TrimSpace(line) == "---" {
				inFrontmatter = false
				continue
			}
			frontmatter = append(frontmatter, line)
			continue
		}
		body = append(body, line)
	}
	if err := scanner.Err(); err != nil {
		return Manifest{}, "", err
	}
	if inFrontmatter {
		return Manifest{}, "", errors.New("unterminated YAML frontmatter")
	}

	manifest := Manifest{Metadata: map[string]string{}}
	for _, raw := range frontmatter {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Manifest{}, "", fmt.Errorf("invalid frontmatter line %q", raw)
		}
		key = strings.TrimSpace(key)
		value = trimYAMLScalar(strings.TrimSpace(value))
		switch key {
		case "name":
			manifest.Name = value
		case "description":
			manifest.Description = value
		case "tags":
			manifest.Tags = parseStringList(value)
		default:
			manifest.Metadata[key] = value
		}
	}

	return manifest, strings.Join(body, "\n"), nil
}

func validateManifest(manifest Manifest) error {
	if !skillNamePattern.MatchString(manifest.Name) {
		return errors.New("skill name must be lowercase letters, numbers, and hyphens, max 64 characters")
	}
	if manifest.Description == "" || len(manifest.Description) > 1024 {
		return errors.New("skill description is required and must be <=1024 characters")
	}
	return nil
}

func trimYAMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func parseStringList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"))
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = trimYAMLScalar(strings.TrimSpace(part))
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
