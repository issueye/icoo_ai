package skills

import (
	"fmt"
	"strings"
)

type manifest struct {
	Name        string
	Description string
	Body        string
}

func parseSkillMarkdown(data []byte) (manifest, error) {
	text := strings.TrimPrefix(string(data), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return manifest{}, fmt.Errorf("SKILL.md missing YAML frontmatter")
	}

	rest := text[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		if strings.HasSuffix(rest, "\n---") {
			end = len(rest) - len("\n---")
		} else {
			return manifest{}, fmt.Errorf("SKILL.md missing closing frontmatter delimiter")
		}
	}

	header := rest[:end]
	bodyStart := end + len("\n---")
	if strings.HasPrefix(rest[bodyStart:], "\n") {
		bodyStart++
	}
	if strings.HasPrefix(rest[bodyStart:], "\n") {
		bodyStart++
	}
	values, err := parseSimpleYAML(header)
	if err != nil {
		return manifest{}, err
	}
	out := manifest{
		Name:        strings.TrimSpace(values["name"]),
		Description: strings.TrimSpace(values["description"]),
		Body:        rest[bodyStart:],
	}
	if out.Name == "" {
		return manifest{}, fmt.Errorf("SKILL.md frontmatter missing name")
	}
	if out.Description == "" {
		return manifest{}, fmt.Errorf("SKILL.md frontmatter missing description")
	}
	return out, nil
}

func parseSimpleYAML(header string) (map[string]string, error) {
	values := map[string]string{}
	for lineNo, raw := range strings.Split(header, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("invalid frontmatter line %d", lineNo+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("invalid frontmatter line %d", lineNo+1)
		}
		values[key] = unquoteYAMLScalar(strings.TrimSpace(value))
	}
	return values, nil
}

func unquoteYAMLScalar(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}
