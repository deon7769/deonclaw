package embeddingpolicy

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

func isBlockedPath(path string) bool {
	slash := filepath.ToSlash(path)
	return isSecretsPath(slash) || isDotEnvPath(slash) || strings.Contains(slash, "..")
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

func validateConfiguredPath(field string, configured string) error {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fmt.Errorf("embedding_policy.%s is required", field)
	}
	if filepath.IsAbs(configured) {
		return fmt.Errorf("embedding_policy.%s must be a relative path", field)
	}
	slash := filepath.ToSlash(configured)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("embedding_policy.%s must not contain ..", field)
	}
	if isBlockedPath(slash) {
		return fmt.Errorf("embedding_policy.%s uses a blocked path", field)
	}
	return nil
}

func validateOutputPath(field string, configured string, artifactsDir string) error {
	if err := validateConfiguredPath(field, configured); err != nil {
		return err
	}
	if !pathUnderArtifacts(artifactsDir, configured) {
		return fmt.Errorf("embedding_policy.%s must be under artifacts dir %q", field, artifactsDir)
	}
	return nil
}

func pathUnderArtifacts(artifactsDir string, configured string) bool {
	rel := filepath.ToSlash(filepath.Clean(configured))
	base := filepath.ToSlash(filepath.Clean(artifactsDir))
	if base == "." || base == "" {
		return false
	}
	return rel == base || strings.HasPrefix(rel, base+"/")
}

func artifactsDirFromInput(chunksPath string) string {
	return filepath.Dir(strings.TrimSpace(chunksPath))
}
