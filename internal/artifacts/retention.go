package artifacts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/runs"
)

type RetentionConfig struct {
	ArtifactsDir string
	OlderThan    time.Duration
	DryRun       bool
	Now          time.Time
}

type PruneStore interface {
	PrunableArtifacts(context.Context, time.Time) ([]PruneCandidate, error)
	DeleteArtifacts(context.Context, []string) error
}

type PruneCandidate struct {
	Artifact  Artifact       `json:"artifact"`
	RunStatus runs.RunStatus `json:"run_status"`
}

type PruneResult struct {
	DryRun     bool        `json:"dry_run"`
	Cutoff     time.Time   `json:"cutoff"`
	Candidates int         `json:"candidates"`
	Deleted    int         `json:"deleted"`
	Skipped    int         `json:"skipped"`
	SizeBytes  int64       `json:"size_bytes"`
	Items      []PruneItem `json:"items"`
}

type PruneItem struct {
	ArtifactID string `json:"artifact_id"`
	RunID      string `json:"run_id"`
	Path       string `json:"path"`
	Action     string `json:"action"`
	Reason     string `json:"reason,omitempty"`
	SizeBytes  int64  `json:"size_bytes"`
}

func Prune(ctx context.Context, store PruneStore, config RetentionConfig) (*PruneResult, error) {
	if store == nil {
		return nil, errors.New("store is required")
	}
	if strings.TrimSpace(config.ArtifactsDir) == "" {
		return nil, errors.New("artifacts dir is required")
	}
	if config.OlderThan <= 0 {
		return nil, errors.New("older-than must be positive")
	}

	now := config.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	cutoff := now.UTC().Add(-config.OlderThan)

	candidates, err := store.PrunableArtifacts(ctx, cutoff)
	if err != nil {
		return nil, err
	}

	root, err := filepath.Abs(config.ArtifactsDir)
	if err != nil {
		return nil, fmt.Errorf("resolve artifacts dir: %w", err)
	}

	result := &PruneResult{
		DryRun:     config.DryRun,
		Cutoff:     cutoff,
		Candidates: len(candidates),
		Items:      make([]PruneItem, 0, len(candidates)),
	}
	var deletedIDs []string
	for _, candidate := range candidates {
		item := PruneItem{
			ArtifactID: candidate.Artifact.ID,
			RunID:      candidate.Artifact.RunID,
			Path:       candidate.Artifact.Path,
			SizeBytes:  candidate.Artifact.SizeBytes,
		}

		switch {
		case candidate.RunStatus != runs.StatusSucceeded:
			item.Action = "skipped"
			item.Reason = "run is not succeeded"
			result.Skipped++
		case candidate.Artifact.Keep:
			item.Action = "skipped"
			item.Reason = "artifact keep=true"
			result.Skipped++
		case !pathWithinRoot(root, candidate.Artifact.Path):
			item.Action = "skipped"
			item.Reason = "artifact path outside artifacts dir"
			result.Skipped++
		case config.DryRun:
			item.Action = "would_delete"
		default:
			removed, err := removeArtifactFile(candidate.Artifact.Path)
			if err != nil {
				return result, err
			}
			if removed {
				cleanupEmptyParents(candidate.Artifact.Path, root)
			}
			item.Action = "deleted"
			result.Deleted++
			result.SizeBytes += candidate.Artifact.SizeBytes
			deletedIDs = append(deletedIDs, candidate.Artifact.ID)
		}
		result.Items = append(result.Items, item)
	}

	if !config.DryRun && len(deletedIDs) > 0 {
		if err := store.DeleteArtifacts(ctx, deletedIDs); err != nil {
			return result, err
		}
	}
	return result, nil
}

func pathWithinRoot(root string, path string) bool {
	artifactPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, artifactPath)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func removeArtifactFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("stat artifact %q: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("artifact path %q is a directory", path)
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("remove artifact %q: %w", path, err)
	}
	return true, nil
}

func cleanupEmptyParents(path string, root string) {
	dir := filepath.Dir(path)
	for pathWithinRoot(root, dir) {
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
