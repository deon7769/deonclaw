package memoryindex

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type CandidateFile struct {
	Domain   string `json:"domain"`
	Root     string `json:"root"`
	Path     string `json:"path"`
	RelPath  string `json:"rel_path"`
	SHA256   string `json:"sha256,omitempty"`
	Skipped  bool   `json:"skipped,omitempty"`
	SkipNote string `json:"skip_note,omitempty"`
}

type SkippedStats struct {
	Symlinks    int `json:"skipped_symlinks"`
	SecretPaths int `json:"skipped_secret_paths"`
	Excluded    int `json:"skipped_excluded"`
}

type ScanResult struct {
	Candidates   []CandidateFile `json:"candidates"`
	SkippedCount int             `json:"skipped_count"`
	Skipped      SkippedStats    `json:"skipped"`
}

func matchesAnyPattern(patterns []string, relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	for _, pattern := range patterns {
		if matchGlobPattern(pattern, relPath) {
			return true
		}
	}
	return false
}

func matchGlobPattern(pattern string, relPath string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	relPath = filepath.ToSlash(strings.TrimSpace(relPath))
	if pattern == "" {
		return false
	}
	if strings.Contains(pattern, "**") {
		return matchDoubleStar(pattern, relPath)
	}
	matched, err := filepath.Match(pattern, relPath)
	if err == nil && matched {
		return true
	}
	matched, err = filepath.Match(pattern, filepath.Base(relPath))
	return err == nil && matched
}

func matchDoubleStar(pattern string, relPath string) bool {
	switch pattern {
	case "**", "**/*":
		return true
	case "**/*.md":
		return strings.HasSuffix(relPath, ".md")
	}
	if strings.HasPrefix(pattern, "**/") && strings.HasSuffix(pattern, "/**") {
		segment := strings.TrimSuffix(strings.TrimPrefix(pattern, "**/"), "/**")
		return pathContainsSegment(relPath, segment)
	}
	if strings.HasPrefix(pattern, "**/") {
		suffix := strings.TrimPrefix(pattern, "**/")
		matched, err := filepath.Match(suffix, relPath)
		if err == nil && matched {
			return true
		}
		return strings.HasSuffix(relPath, "/"+suffix) || relPath == suffix || strings.HasSuffix(relPath, suffix)
	}
	matched, err := filepath.Match(pattern, relPath)
	return err == nil && matched
}

func pathContainsSegment(path string, segment string) bool {
	if segment == "" {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func isSymlinkEntry(entry fs.DirEntry) bool {
	return entry.Type()&fs.ModeSymlink != 0
}

func scanSource(source Source) (ScanResult, error) {
	root := source.Root
	info, err := os.Stat(root)
	if err != nil {
		return ScanResult{}, fmt.Errorf("stat source root %q: %w", root, err)
	}
	if !info.IsDir() {
		return ScanResult{}, fmt.Errorf("source root %q is not a directory", root)
	}

	result := ScanResult{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !pathWithinRoot(root, path) {
			result.SkippedCount++
			return fs.SkipDir
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(relPath)
		if relSlash == "." {
			return nil
		}

		if isSymlinkEntry(entry) {
			result.Skipped.Symlinks++
			result.SkippedCount++
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if isSecretsPath(relSlash) {
			result.Skipped.SecretPaths++
			result.SkippedCount++
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			if matchesAnyPattern(source.Exclude, relSlash) || matchesAnyPattern(source.Exclude, relSlash+"/") {
				result.Skipped.Excluded++
				result.SkippedCount++
				return fs.SkipDir
			}
			return nil
		}

		if !matchesAnyPattern(source.Include, relSlash) {
			result.Skipped.Excluded++
			result.SkippedCount++
			return nil
		}
		if matchesAnyPattern(source.Exclude, relSlash) {
			result.Skipped.Excluded++
			result.SkippedCount++
			return nil
		}

		result.Candidates = append(result.Candidates, CandidateFile{
			Domain:  source.Domain,
			Root:    root,
			Path:    path,
			RelPath: relSlash,
		})
		return nil
	})
	if err != nil {
		return ScanResult{}, err
	}
	return result, nil
}

func scanConfig(cfg Config) (ScanResult, error) {
	combined := ScanResult{}
	for _, source := range cfg.MemoryIndex.Sources {
		part, err := scanSource(source)
		if err != nil {
			return ScanResult{}, err
		}
		combined.Candidates = append(combined.Candidates, part.Candidates...)
		combined.SkippedCount += part.SkippedCount
		combined.Skipped.Symlinks += part.Skipped.Symlinks
		combined.Skipped.SecretPaths += part.Skipped.SecretPaths
		combined.Skipped.Excluded += part.Skipped.Excluded
	}
	return combined, nil
}
