package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/store"
)

func NewEvidenceBuilder(artifactsDir string) EvidenceBuilder {
	artifactsDir = strings.TrimSpace(artifactsDir)
	return func(ctx context.Context, repo Repository, runID string, now time.Time) (insights.EvidenceBundle, error) {
		if artifactsDir == "" {
			return insights.EvidenceBundle{}, fmt.Errorf("artifacts dir is required for evidence")
		}
		db, ok := repo.(store.Store)
		if !ok {
			return insights.EvidenceBundle{}, fmt.Errorf("repository does not support evidence build")
		}
		bundle, err := insights.BuildEvidenceFromRun(ctx, db, insights.BuildEvidenceOptions{
			RunID: runID, Trigger: insights.TriggerRunCompleted, Now: now,
		})
		if err != nil {
			return insights.EvidenceBundle{}, err
		}
		path := filepath.Join(artifactsDir, "evidence", runID, "evidence-bundle.json")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return insights.EvidenceBundle{}, err
		}
		if err := insights.WriteEvidenceJSON(bundle, path); err != nil {
			return insights.EvidenceBundle{}, err
		}
		return bundle, nil
	}
}
