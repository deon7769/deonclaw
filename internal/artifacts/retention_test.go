package artifacts

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/runs"
)

func TestPruneDryRunDoesNotRemoveFilesOrMetadata(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "run-001", "summary.md")
	writeTestFile(t, artifactPath, "summary\n")

	store := &fakePruneStore{
		candidates: []PruneCandidate{
			{
				Artifact: Artifact{
					ID:        "artifact-001",
					RunID:     "run-001",
					Path:      artifactPath,
					Kind:      KindSummary,
					SizeBytes: 8,
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				RunStatus: runs.StatusSucceeded,
			},
		},
	}

	result, err := Prune(context.Background(), store, RetentionConfig{
		ArtifactsDir: root,
		OlderThan:    30 * 24 * time.Hour,
		DryRun:       true,
		Now:          time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if result.Candidates != 1 || result.Deleted != 0 || result.Skipped != 0 {
		t.Fatalf("Prune() result = %#v, want one dry-run candidate", result)
	}
	if len(store.deletedIDs) != 0 {
		t.Fatalf("deleted IDs = %#v, want none in dry-run", store.deletedIDs)
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("artifact file was removed in dry-run: %v", err)
	}
	if result.Items[0].Action != "would_delete" {
		t.Fatalf("item action = %q, want would_delete", result.Items[0].Action)
	}
}

func TestPruneRemovesSucceededArtifactsAndMetadata(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "run-001", "summary.md")
	writeTestFile(t, artifactPath, "summary\n")

	store := &fakePruneStore{
		candidates: []PruneCandidate{
			{
				Artifact: Artifact{
					ID:        "artifact-001",
					RunID:     "run-001",
					Path:      artifactPath,
					Kind:      KindSummary,
					SizeBytes: 8,
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				RunStatus: runs.StatusSucceeded,
			},
			{
				Artifact: Artifact{
					ID:        "artifact-failed",
					RunID:     "run-failed",
					Path:      filepath.Join(root, "run-failed", "summary.md"),
					Kind:      KindSummary,
					SizeBytes: 9,
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				RunStatus: runs.StatusFailed,
			},
			{
				Artifact: Artifact{
					ID:        "artifact-kept",
					RunID:     "run-kept",
					Path:      filepath.Join(root, "run-kept", "summary.md"),
					Kind:      KindSummary,
					Keep:      true,
					SizeBytes: 10,
					CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				},
				RunStatus: runs.StatusSucceeded,
			},
		},
	}

	result, err := Prune(context.Background(), store, RetentionConfig{
		ArtifactsDir: root,
		OlderThan:    30 * 24 * time.Hour,
		Now:          time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if result.Candidates != 3 || result.Deleted != 1 || result.Skipped != 2 {
		t.Fatalf("Prune() result = %#v, want one deleted and two skipped", result)
	}
	if len(store.deletedIDs) != 1 || store.deletedIDs[0] != "artifact-001" {
		t.Fatalf("deleted IDs = %#v, want artifact-001", store.deletedIDs)
	}
	if _, err := os.Stat(artifactPath); !os.IsNotExist(err) {
		t.Fatalf("artifact file still exists after prune: %v", err)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

type fakePruneStore struct {
	candidates []PruneCandidate
	deletedIDs []string
}

func (s *fakePruneStore) PrunableArtifacts(context.Context, time.Time) ([]PruneCandidate, error) {
	return append([]PruneCandidate(nil), s.candidates...), nil
}

func (s *fakePruneStore) DeleteArtifacts(ctx context.Context, ids []string) error {
	s.deletedIDs = append(s.deletedIDs, ids...)
	return nil
}
