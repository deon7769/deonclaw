package lancedbpolicy

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var safeIdentifierPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

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

func validateRelativePath(field string, configured string) error {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fmt.Errorf("lancedb_policy.%s is required", field)
	}
	if filepath.IsAbs(configured) {
		return fmt.Errorf("lancedb_policy.%s must be a relative path", field)
	}
	slash := filepath.ToSlash(configured)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("lancedb_policy.%s must not contain ..", field)
	}
	if isBlockedPath(slash) {
		return fmt.Errorf("lancedb_policy.%s uses a blocked path", field)
	}
	return nil
}

func validateSafeIdentifier(field string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("lancedb_policy.%s is required", field)
	}
	if !safeIdentifierPattern.MatchString(value) {
		return fmt.Errorf("lancedb_policy.%s %q is not a safe identifier", field, value)
	}
	return nil
}

func artifactsDirFromInput(inputPath string) string {
	return filepath.Dir(strings.TrimSpace(inputPath))
}

func validateArtifactsDirPath(artifactsDir string) error {
	artifactsDir = strings.TrimSpace(artifactsDir)
	if artifactsDir == "" {
		return fmt.Errorf("artifacts dir is required")
	}
	if filepath.IsAbs(artifactsDir) {
		return fmt.Errorf("artifacts dir must be a relative path")
	}
	slash := filepath.ToSlash(artifactsDir)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("artifacts dir must not contain ..")
	}
	if isBlockedPath(slash) {
		return fmt.Errorf("artifacts dir uses a blocked path")
	}
	return nil
}

func validateArtifactsDir(cfg Config, artifactsDir string) error {
	if err := validateArtifactsDirPath(artifactsDir); err != nil {
		return err
	}
	expected := artifactsDirFromInput(cfg.LanceDBPolicy.Input.VectorsPath)
	if filepath.Clean(strings.TrimSpace(artifactsDir)) != filepath.Clean(expected) {
		return fmt.Errorf("artifacts dir %q does not match policy input base %q", artifactsDir, expected)
	}
	return nil
}
