package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const registryFilename = "registry.json"

func DefaultRegistryRoot(home string) string {
	if strings.TrimSpace(home) == "" {
		home, _ = os.UserHomeDir()
	}
	return filepath.Join(home, ".deonclaw", "skills")
}

func LoadRegistry(root string) (Registry, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" {
		return Registry{}, fmt.Errorf("registry root is required")
	}
	path := filepath.Join(root, registryFilename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewRegistry(root), nil
		}
		return Registry{}, fmt.Errorf("read registry %q: %w", path, err)
	}
	var registry Registry
	if err := json.Unmarshal(data, &registry); err != nil {
		return Registry{}, fmt.Errorf("parse registry %q: %w", path, err)
	}
	if registry.Skills == nil {
		registry.Skills = map[string]RegistrySkill{}
	}
	if registry.AgentAllowlists == nil {
		registry.AgentAllowlists = map[string][]string{}
	}
	registry.RegistryRoot = root
	if registry.Version == 0 {
		registry.Version = RegistryVersion
	}
	return registry, nil
}

func NewRegistry(root string) Registry {
	return Registry{
		Version:         RegistryVersion,
		RegistryRoot:    filepath.Clean(root),
		Skills:          map[string]RegistrySkill{},
		AgentAllowlists: map[string][]string{},
	}
}

func SaveRegistry(registry Registry) error {
	if strings.TrimSpace(registry.RegistryRoot) == "" {
		return fmt.Errorf("registry root is required")
	}
	hash, err := registryHash(registry)
	if err != nil {
		return err
	}
	registry.SHA256 = hash
	if err := os.MkdirAll(registry.RegistryRoot, 0o755); err != nil {
		return fmt.Errorf("create registry root: %w", err)
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	data = append(data, '\n')
	path := filepath.Join(registry.RegistryRoot, registryFilename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write registry %q: %w", path, err)
	}
	return nil
}

func registryHash(registry Registry) (string, error) {
	copy := registry
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func RevisionDir(registryRoot string, skillName string, revisionID string) string {
	return filepath.Join(registryRoot, "skills", skillName, "revisions", revisionID)
}

func CurrentLinkPath(registryRoot string, skillName string) string {
	return filepath.Join(registryRoot, "skills", skillName, "current")
}

func NewRevisionID(now time.Time, contentSHA256 string) string {
	stamp := now.UTC().Format("2006-01-02T15-04-05Z")
	short := contentSHA256
	if len(short) > 8 {
		short = short[:8]
	}
	return stamp + "-" + short
}

func copySkillTree(sourceDir string, destDir string) error {
	sourceDir, err := DiscoverSkillDirectory(sourceDir)
	if err != nil {
		return err
	}
	return filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destDir, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not copied into managed registry: %s", rel)
		}
		return copyFile(path, target)
	})
}

func copyFile(source string, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
