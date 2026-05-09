package workspace

import (
	"bufio"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type IgnoreMatcher struct {
	rules []ignoreRule
}

type ignoreRule struct {
	pattern       string
	negate        bool
	directoryOnly bool
	anchored      bool
	hasSlash      bool
}

func LoadIgnoreMatcher(root string) (*IgnoreMatcher, error) {
	m := &IgnoreMatcher{}
	for _, name := range []string{".gitignore", ".icooignore"} {
		if err := m.loadFile(filepath.Join(root, name)); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *IgnoreMatcher) Match(rel string, isDir bool) bool {
	if m == nil {
		return false
	}
	rel = cleanRel(rel)
	if rel == "" {
		return false
	}
	ignored := false
	for _, rule := range m.rules {
		if rule.matches(rel, isDir) {
			ignored = !rule.negate
		}
	}
	return ignored
}

func (m *IgnoreMatcher) loadFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		rule, ok := parseIgnoreRule(scanner.Text())
		if ok {
			m.rules = append(m.rules, rule)
		}
	}
	return scanner.Err()
}

func parseIgnoreRule(line string) (ignoreRule, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	rule := ignoreRule{}
	if strings.HasPrefix(line, "!") {
		rule.negate = true
		line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
	}
	if line == "" {
		return ignoreRule{}, false
	}
	if strings.HasPrefix(line, "\\#") || strings.HasPrefix(line, "\\!") {
		line = line[1:]
	}
	line = filepath.ToSlash(line)
	if strings.HasPrefix(line, "/") {
		rule.anchored = true
		line = strings.TrimPrefix(line, "/")
	}
	if strings.HasSuffix(line, "/") {
		rule.directoryOnly = true
		line = strings.TrimSuffix(line, "/")
	}
	line = strings.Trim(line, "/")
	if line == "" {
		return ignoreRule{}, false
	}
	rule.pattern = line
	rule.hasSlash = strings.Contains(line, "/")
	return rule, true
}

func (r ignoreRule) matches(rel string, isDir bool) bool {
	if r.directoryOnly && !isDir && !strings.HasPrefix(rel, r.pattern+"/") {
		return false
	}
	if r.hasSlash || r.anchored {
		return matchPathPattern(r.pattern, rel) || strings.HasPrefix(rel, r.pattern+"/")
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		if matchPathPattern(r.pattern, part) {
			return true
		}
		if r.directoryOnly && part == r.pattern && i < len(parts)-1 {
			return true
		}
	}
	return false
}

func matchPathPattern(pattern, value string) bool {
	if ok, err := path.Match(pattern, value); err == nil && ok {
		return true
	}
	if strings.ContainsAny(pattern, "*?[") {
		if ok, err := path.Match(pattern, value); err == nil {
			return ok
		}
		return false
	}
	return pattern == value
}

func cleanRel(rel string) string {
	rel = filepath.ToSlash(filepath.Clean(rel))
	rel = strings.TrimPrefix(rel, "./")
	if rel == "." {
		return ""
	}
	return strings.TrimPrefix(rel, "/")
}
