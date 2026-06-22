package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathWithinRoot reports whether target is equal to or nested under root.
func PathWithinRoot(root string, target string) (bool, error) {
	root = filepath.Clean(strings.TrimSpace(root))
	target = filepath.Clean(strings.TrimSpace(target))
	if root == "" || target == "" {
		return false, fmt.Errorf("root and target are required")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false, err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, nil
	}
	return true, nil
}

// ValidateSnapshotAgainstRegistry ensures snapshot skills match registry name, revision, and hash.
func ValidateSnapshotAgainstRegistry(snapshot SessionSnapshot, registryRoot string) error {
	registry, err := LoadRegistry(registryRoot)
	if err != nil {
		return err
	}
	for _, skill := range snapshot.Skills {
		entry, ok := registry.Skills[skill.Name]
		if !ok {
			return fmt.Errorf("snapshot skill %q is not in registry", skill.Name)
		}
		if entry.State != LifecycleActive {
			return fmt.Errorf("snapshot skill %q is not active", skill.Name)
		}
		if strings.TrimSpace(skill.Revision) != strings.TrimSpace(entry.CurrentRevision) {
			return fmt.Errorf("snapshot skill %q revision %q does not match registry revision %q", skill.Name, skill.Revision, entry.CurrentRevision)
		}
		if strings.TrimSpace(skill.SHA256) != strings.TrimSpace(entry.ContentSHA256) {
			return fmt.Errorf("snapshot skill %q sha256 does not match registry content hash", skill.Name)
		}
		revisionDir := RevisionDir(registryRoot, skill.Name, skill.Revision)
		if ok, err := PathWithinRoot(registryRoot, revisionDir); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("revision path for skill %q escapes registry root", skill.Name)
		}
	}
	return ValidateSnapshotIntegrity(snapshot)
}

// ValidateSnapshotIntegrity recomputes and checks the snapshot content hash.
func ValidateSnapshotIntegrity(snapshot SessionSnapshot) error {
	expected := strings.TrimSpace(snapshot.SHA256)
	if expected == "" {
		return fmt.Errorf("snapshot sha256 is required")
	}
	actual, err := snapshotHash(snapshot)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("snapshot sha256 mismatch")
	}
	return nil
}
