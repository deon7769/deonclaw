package retrievalcontext

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

func validateRelativeSafePath(field string, configured string) error {
	configured = strings.TrimSpace(configured)
	if configured == "" {
		return fmt.Errorf("%s is required", field)
	}
	if filepath.IsAbs(configured) {
		return fmt.Errorf("%s must be a relative path", field)
	}
	slash := filepath.ToSlash(configured)
	if strings.Contains(slash, "..") {
		return fmt.Errorf("%s must not contain ..", field)
	}
	if isBlockedPath(slash) {
		return fmt.Errorf("%s uses a blocked path", field)
	}
	return nil
}
