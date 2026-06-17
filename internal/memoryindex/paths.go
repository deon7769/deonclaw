package memoryindex

import (
	"fmt"
	"path/filepath"
	"strings"
)

func isSecretsPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, segment := range strings.Split(path, "/") {
		if segment == "secrets" {
			return true
		}
	}
	return false
}

func isDotEnvPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	for _, segment := range strings.Split(path, "/") {
		if segment == ".env" || strings.HasSuffix(segment, ".env") {
			return true
		}
	}
	return false
}

func isBlockedOutputPath(path string) bool {
	return isSecretsPath(path) || isDotEnvPath(path) || strings.Contains(filepath.ToSlash(path), "..")
}

func pathWithinRoot(root string, path string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func validateConfiguredOutputPath(field string, configured string) error {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fmt.Errorf("memory_index.output.%s is required", field)
	}
	if filepath.IsAbs(configured) {
		return fmt.Errorf("memory_index.output.%s must be relative to artifacts dir", field)
	}
	slash := filepath.ToSlash(configured)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("memory_index.output.%s must not contain ..", field)
	}
	if isBlockedOutputPath(slash) {
		return fmt.Errorf("memory_index.output.%s uses a blocked path", field)
	}
	return nil
}

func resolveOutputPath(artifactsDir string, field string, configured string) (string, error) {
	if err := validateConfiguredOutputPath(field, configured); err != nil {
		return "", err
	}
	artifactsAbs, err := filepath.Abs(strings.TrimSpace(artifactsDir))
	if err != nil {
		return "", fmt.Errorf("resolve artifacts dir: %w", err)
	}
	rel, err := sanitizeRelativePath(configured)
	if err != nil {
		return "", fmt.Errorf("memory_index.output.%s: %w", field, err)
	}
	resolved := filepath.Join(artifactsAbs, rel)
	if !pathWithinRoot(artifactsAbs, resolved) {
		return "", fmt.Errorf("memory_index.output.%s resolves outside artifacts dir", field)
	}
	return resolved, nil
}

func sanitizeRelativePath(configured string) (string, error) {
	slash := filepath.ToSlash(strings.TrimSpace(configured))
	if slash == "" || slash == "." {
		return "", fmt.Errorf("path is required")
	}
	parts := strings.Split(slash, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return "", fmt.Errorf("path must not contain ..")
		default:
			clean = append(clean, part)
		}
	}
	if len(clean) == 0 {
		return "", fmt.Errorf("path is required")
	}
	if isBlockedOutputPath(strings.Join(clean, "/")) {
		return "", fmt.Errorf("path is blocked")
	}
	return filepath.Join(clean...), nil
}

func resolveOutputPaths(cfg Config, artifactsDir string) (manifestPath string, chunksPath string, err error) {
	manifestPath, err = resolveOutputPath(artifactsDir, "manifest", cfg.MemoryIndex.Output.Manifest)
	if err != nil {
		return "", "", err
	}
	chunksPath, err = resolveOutputPath(artifactsDir, "chunks", cfg.MemoryIndex.Output.Chunks)
	if err != nil {
		return "", "", err
	}
	return manifestPath, chunksPath, nil
}
