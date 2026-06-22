package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MaterializeResult struct {
	Workspace      string   `json:"workspace"`
	SnapshotSHA256 string   `json:"snapshot_sha256"`
	Materialized   []string `json:"materialized"`
	Skipped        []string `json:"skipped"`
}

type MaterializeOptions struct {
	Snapshot     SessionSnapshot
	Workspace    string
	RegistryRoot string
}

func MaterializeSnapshot(opts MaterializeOptions) (MaterializeResult, error) {
	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		return MaterializeResult{}, fmt.Errorf("workspace is required")
	}
	registryRoot := strings.TrimSpace(opts.RegistryRoot)
	if registryRoot == "" {
		return MaterializeResult{}, fmt.Errorf("registry root is required")
	}

	result := MaterializeResult{
		Workspace:      filepath.Clean(workspace),
		SnapshotSHA256: opts.Snapshot.SHA256,
		Materialized:   []string{},
		Skipped:        []string{},
	}
	if err := ValidateSnapshotAgainstRegistry(opts.Snapshot, registryRoot); err != nil {
		return MaterializeResult{}, err
	}
	if ok, err := PathWithinRoot(workspace, filepath.Join(workspace, AgentsSkillsDir)); err != nil {
		return MaterializeResult{}, err
	} else if !ok {
		return MaterializeResult{}, fmt.Errorf("skills materialize root escapes workspace")
	}

	for _, skill := range opts.Snapshot.Skills {
		revisionDir := RevisionDir(registryRoot, skill.Name, skill.Revision)
		if _, err := os.Stat(revisionDir); err != nil {
			result.Skipped = append(result.Skipped, fmt.Sprintf("%s: revision not found", skill.Name))
			continue
		}
		destRoot := filepath.Join(workspace, AgentsSkillsDir, skill.Name)
		if ok, err := PathWithinRoot(workspace, destRoot); err != nil {
			return MaterializeResult{}, err
		} else if !ok {
			return MaterializeResult{}, fmt.Errorf("materialize destination for %q escapes workspace", skill.Name)
		}
		if ok, err := PathWithinRoot(registryRoot, revisionDir); err != nil {
			return MaterializeResult{}, err
		} else if !ok {
			return MaterializeResult{}, fmt.Errorf("revision source for %q escapes registry root", skill.Name)
		}
		if err := os.RemoveAll(destRoot); err != nil {
			return MaterializeResult{}, err
		}
		if err := copySkillTree(revisionDir, destRoot); err != nil {
			return MaterializeResult{}, fmt.Errorf("materialize %q: %w", skill.Name, err)
		}
		result.Materialized = append(result.Materialized, filepath.Join(AgentsSkillsDir, skill.Name))
	}
	return result, nil
}

func WriteMaterializeResultJSON(result MaterializeResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialize result: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialize result %q: %w", path, err)
	}
	return nil
}
