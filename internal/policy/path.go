package policy

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

type Result struct {
	Violations []Violation
}

type Violation struct {
	Path    string
	Rule    string
	Pattern string
}

func (r Result) OK() bool {
	return len(r.Violations) == 0
}

func (r Result) Summary() string {
	if r.OK() {
		return "ok"
	}

	messages := make([]string, 0, len(r.Violations))
	for _, violation := range r.Violations {
		switch violation.Rule {
		case "read_only":
			messages = append(messages, fmt.Sprintf("read_only task changed files: %s", violation.Path))
		case "forbidden":
			messages = append(messages, fmt.Sprintf("%s matches forbidden path %q", violation.Path, violation.Pattern))
		case "not_allowed":
			messages = append(messages, fmt.Sprintf("%s is outside allowed paths", violation.Path))
		default:
			messages = append(messages, fmt.Sprintf("%s violates path policy", violation.Path))
		}
	}
	return strings.Join(messages, "; ")
}

func EvaluateChangedPaths(mode string, changedPaths []string, allowedPaths []string, forbiddenPaths []string) Result {
	var result Result
	for _, changedPath := range changedPaths {
		normalizedPath := normalizePath(changedPath)
		if normalizedPath == "" {
			continue
		}

		if mode == "read_only" {
			result.Violations = append(result.Violations, Violation{
				Path: normalizedPath,
				Rule: "read_only",
			})
			continue
		}

		if pattern, ok := firstMatchingPattern(normalizedPath, forbiddenPaths); ok {
			result.Violations = append(result.Violations, Violation{
				Path:    normalizedPath,
				Rule:    "forbidden",
				Pattern: pattern,
			})
			continue
		}

		if !matchesAnyPattern(normalizedPath, allowedPaths) {
			result.Violations = append(result.Violations, Violation{
				Path: normalizedPath,
				Rule: "not_allowed",
			})
		}
	}
	return result
}

func ChangedPathsFromGitDiff(diff []byte) []string {
	seen := make(map[string]struct{})
	var paths []string
	for _, line := range strings.Split(string(diff), "\n") {
		if !strings.HasPrefix(line, "diff --git ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		changedPath := normalizePath(strings.TrimPrefix(fields[3], "b/"))
		if changedPath == "" {
			continue
		}
		if _, ok := seen[changedPath]; ok {
			continue
		}
		seen[changedPath] = struct{}{}
		paths = append(paths, changedPath)
	}
	return paths
}

func matchesAnyPattern(changedPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPathPattern(pattern, changedPath) {
			return true
		}
	}
	return false
}

func firstMatchingPattern(changedPath string, patterns []string) (string, bool) {
	for _, pattern := range patterns {
		if matchPathPattern(pattern, changedPath) {
			return normalizePath(pattern), true
		}
	}
	return "", false
}

func matchPathPattern(pattern string, changedPath string) bool {
	normalizedPattern := normalizePath(pattern)
	if normalizedPattern == "" {
		return false
	}
	if normalizedPattern == "**" {
		return true
	}
	if strings.HasSuffix(normalizedPattern, "/**") {
		prefix := strings.TrimSuffix(normalizedPattern, "/**")
		return changedPath == prefix || strings.HasPrefix(changedPath, prefix+"/")
	}
	matched, err := path.Match(normalizedPattern, changedPath)
	return err == nil && matched
}

func normalizePath(value string) string {
	value = strings.TrimSpace(filepath.ToSlash(value))
	if value == "" {
		return ""
	}
	value = strings.TrimPrefix(value, "a/")
	value = strings.TrimPrefix(value, "b/")
	value = path.Clean(value)
	value = strings.TrimPrefix(value, "./")
	if value == "." || value == "/dev/null" {
		return ""
	}
	return value
}
