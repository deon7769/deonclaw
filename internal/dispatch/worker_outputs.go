package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/events"
)

type workerOutputPersistResult struct {
	Artifacts int
	Events    int
}

func persistWorkerOutputs(ctx context.Context, repo Repository, artifactsDir string, runID string, result WorkerRunResult, now time.Time) (workerOutputPersistResult, error) {
	persisted := workerOutputPersistResult{}
	if len(result.Events) == 0 && len(result.Artifacts) == 0 {
		return persisted, nil
	}
	if strings.TrimSpace(runID) == "" {
		return persisted, fmt.Errorf("run id is required to persist worker outputs")
	}
	if unsafePathSegment(runID) {
		return persisted, fmt.Errorf("run id %q is not safe for worker artifact paths", runID)
	}
	for i, event := range result.Events {
		event.ID = fmt.Sprintf("evt_%s_worker_%03d", runID, i+1)
		event.RunID = runID
		if event.Type == "" {
			event.Type = events.TypeWorkerMessage
		}
		if event.Timestamp.IsZero() {
			event.Timestamp = now.Add(time.Duration(i) * time.Nanosecond)
		}
		if err := repo.SaveEvent(ctx, &event); err != nil {
			return persisted, err
		}
		persisted.Events++
	}
	if len(result.Artifacts) == 0 {
		return persisted, nil
	}
	root := strings.TrimSpace(artifactsDir)
	if root == "" {
		return persisted, fmt.Errorf("artifacts dir is required to persist worker artifacts")
	}
	for i, artifact := range result.Artifacts {
		targetPath, err := workerArtifactTargetPath(root, runID, artifact.Path, i)
		if err != nil {
			return persisted, err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return persisted, fmt.Errorf("create worker artifact dir: %w", err)
		}
		content := append([]byte(nil), artifact.Content...)
		if err := os.WriteFile(targetPath, content, 0o600); err != nil {
			return persisted, fmt.Errorf("write worker artifact %q: %w", targetPath, err)
		}
		sum := sha256.Sum256(content)
		artifact.ID = fmt.Sprintf("art_%s_worker_%03d", runID, i+1)
		artifact.RunID = runID
		artifact.Path = targetPath
		if artifact.Kind == "" {
			artifact.Kind = artifacts.KindOther
		}
		artifact.SizeBytes = int64(len(content))
		artifact.SHA256 = hex.EncodeToString(sum[:])
		if artifact.CreatedAt.IsZero() {
			artifact.CreatedAt = now.Add(time.Duration(i) * time.Nanosecond)
		}
		if err := repo.SaveArtifact(ctx, &artifact); err != nil {
			return persisted, err
		}
		persisted.Artifacts++
	}
	return persisted, nil
}

func workerArtifactTargetPath(root string, runID string, rawPath string, index int) (string, error) {
	if unsafePathSegment(runID) {
		return "", fmt.Errorf("run id %q is not safe for worker artifact paths", runID)
	}
	rel := strings.TrimSpace(rawPath)
	if rel == "" {
		rel = fmt.Sprintf("artifact-%03d.bin", index+1)
	}
	rel = filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(rel) || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("worker artifact path %q is outside artifact root", rawPath)
	}
	rootPath, err := filepath.Abs(filepath.Join(root, "worker", runID))
	if err != nil {
		return "", fmt.Errorf("resolve worker artifact root: %w", err)
	}
	targetPath, err := filepath.Abs(filepath.Join(rootPath, rel))
	if err != nil {
		return "", fmt.Errorf("resolve worker artifact path: %w", err)
	}
	within, err := pathWithinRoot(rootPath, targetPath)
	if err != nil {
		return "", err
	}
	if !within {
		return "", fmt.Errorf("worker artifact path %q is outside artifact root", rawPath)
	}
	return targetPath, nil
}

func unsafePathSegment(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || value == "." || value == ".." || strings.Contains(value, "/") || strings.Contains(value, "\\")
}

func pathWithinRoot(root string, target string) (bool, error) {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false, fmt.Errorf("compare artifact path to root: %w", err)
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}
