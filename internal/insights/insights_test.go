package insights

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestValidatePolicyOK(t *testing.T) {
	cfg, err := ParsePolicy([]byte(exampleInsightPolicyYAML))
	if err != nil {
		t.Fatalf("ParsePolicy() error = %v", err)
	}
	if err := ValidatePolicy(cfg); err != nil {
		t.Fatalf("ValidatePolicy() error = %v", err)
	}
}

func TestValidatePolicyRejectsAutoApplyWithoutAutoPropose(t *testing.T) {
	cfg, err := ParsePolicy([]byte(exampleInsightPolicyYAML))
	if err != nil {
		t.Fatalf("ParsePolicy() error = %v", err)
	}
	cfg.InsightPolicy.AutoPropose = false
	cfg.InsightPolicy.AutoApply = true
	if err := ValidatePolicy(cfg); err == nil {
		t.Fatal("ValidatePolicy() expected error for auto_apply without auto_propose")
	}
}

func TestValidatePolicyRejectsUnknownAlwaysOnTrigger(t *testing.T) {
	cfg, err := ParsePolicy([]byte(exampleInsightPolicyYAML))
	if err != nil {
		t.Fatalf("ParsePolicy() error = %v", err)
	}
	cfg.InsightPolicy.AlwaysOn = append(cfg.InsightPolicy.AlwaysOn, "not_a_trigger")
	if err := ValidatePolicy(cfg); err == nil {
		t.Fatal("ValidatePolicy() expected error for unknown always_on trigger")
	}
}

func TestEvaluateTriggerAlwaysOn(t *testing.T) {
	cfg, err := ParsePolicy([]byte(exampleInsightPolicyYAML))
	if err != nil {
		t.Fatalf("ParsePolicy() error = %v", err)
	}
	decision, err := EvaluateTrigger(cfg.InsightPolicy, TriggerValidationFailed, TriggerCounters{})
	if err != nil {
		t.Fatalf("EvaluateTrigger() error = %v", err)
	}
	if !decision.ShouldEvaluate {
		t.Fatalf("EvaluateTrigger() should_evaluate = false, want true")
	}
}

func TestEvaluateTriggerRunThreshold(t *testing.T) {
	cfg, err := ParsePolicy([]byte(exampleInsightPolicyYAML))
	if err != nil {
		t.Fatalf("ParsePolicy() error = %v", err)
	}
	decision, err := EvaluateTrigger(cfg.InsightPolicy, TriggerRunCompleted, TriggerCounters{RunsSinceLastInsight: 1})
	if err != nil {
		t.Fatalf("EvaluateTrigger() error = %v", err)
	}
	if !decision.ShouldEvaluate {
		t.Fatalf("EvaluateTrigger() should_evaluate = false, want true")
	}
}

func TestBuildEvidenceFromRun(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dbPath := filepath.Join(root, "deonclaw.db")
	db, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	runID := "run-insight-001"
	taskID := "task-insight-001"
	artifactDir := filepath.Join(root, "artifacts", runID)
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	validationPath := filepath.Join(artifactDir, "validation.json")
	validationJSON := []byte(`{
  "status": "failed",
  "commands": [
    {"name": "go-test", "command": "go test ./...", "status": "failed"}
  ]
}
`)
	if err := os.WriteFile(validationPath, validationJSON, 0o644); err != nil {
		t.Fatalf("WriteFile(validation.json) error = %v", err)
	}

	changedFilesPath := filepath.Join(artifactDir, "changed-files.json")
	if err := os.WriteFile(changedFilesPath, []byte(`[{"path":"internal/insights/evidence.go"}]`), 0o644); err != nil {
		t.Fatalf("WriteFile(changed-files.json) error = %v", err)
	}

	diffPath := filepath.Join(artifactDir, "diff.patch")
	if err := os.WriteFile(diffPath, []byte("--- a/file\n+++ b/file\n+added\n-removed\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(diff.patch) error = %v", err)
	}

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	if err := db.SaveTask(ctx, &tasks.Task{
		ID:     taskID,
		Title:  "Insight evidence test",
		Domain: "deonclaw",
		Worker: "codex",
		Goal:   "test",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "worktree",
			Path:     filepath.Join(root, "workspace"),
		},
	}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := db.SaveRun(ctx, &runs.Run{
		ID:            runID,
		TaskID:        taskID,
		Status:        runs.StatusFailed,
		Worker:        "codex",
		WorkspacePath: filepath.Join(root, "workspace"),
		CreatedAt:     now,
		UpdatedAt:     now,
	}); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	if err := db.SaveEvent(ctx, &events.Event{
		ID:        "evt-001",
		RunID:     runID,
		Type:      events.TypeRunFailed,
		Timestamp: now,
	}); err != nil {
		t.Fatalf("SaveEvent() error = %v", err)
	}

	for _, spec := range []struct {
		id   string
		path string
		kind artifacts.Kind
	}{
		{"art-validation", validationPath, artifacts.KindOther},
		{"art-changed", changedFilesPath, artifacts.KindOther},
		{"art-diff", diffPath, artifacts.KindDiff},
	} {
		content, err := os.ReadFile(spec.path)
		if err != nil {
			t.Fatalf("ReadFile(%s) error = %v", spec.path, err)
		}
		if err := db.SaveArtifact(ctx, &artifacts.Artifact{
			ID:        spec.id,
			RunID:     runID,
			Path:      spec.path,
			Kind:      spec.kind,
			Content:   content,
			SizeBytes: int64(len(content)),
			CreatedAt: now,
		}); err != nil {
			t.Fatalf("SaveArtifact(%s) error = %v", spec.id, err)
		}
	}

	bundle, err := BuildEvidenceFromRun(ctx, db, BuildEvidenceOptions{
		RunID:     runID,
		PolicyRef: "configs/examples/insight-policy.yaml",
		Now:       now,
	})
	if err != nil {
		t.Fatalf("BuildEvidenceFromRun() error = %v", err)
	}

	if bundle.Trigger != TriggerValidationFailed {
		t.Fatalf("trigger = %q, want %q", bundle.Trigger, TriggerValidationFailed)
	}
	if bundle.Scope.AgentID != LegacyManualAgentID {
		t.Fatalf("agent_id = %q, want %q", bundle.Scope.AgentID, LegacyManualAgentID)
	}
	if len(bundle.RunIDs) != 1 || bundle.RunIDs[0] != runID {
		t.Fatalf("run_ids = %#v, want [%q]", bundle.RunIDs, runID)
	}
	if len(bundle.ValidationResults) != 1 || bundle.ValidationResults[0].Status != "failed" {
		t.Fatalf("validation_results = %#v", bundle.ValidationResults)
	}
	if bundle.DiffSummary.FilesChanged != 1 || bundle.DiffSummary.Insertions != 1 || bundle.DiffSummary.Deletions != 1 {
		t.Fatalf("diff_summary = %#v", bundle.DiffSummary)
	}
	if bundle.ContainsTextExcerpt {
		t.Fatal("contains_text_excerpt should be false")
	}
	if bundle.SHA256 == "" {
		t.Fatal("sha256 should be set")
	}

	encoded, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := scanBundleForSecrets(bundle); err != nil {
		t.Fatalf("scanBundleForSecrets() error = %v; payload=%s", err, encoded)
	}
}

func TestBuildEvidenceRejectsSecretLikeValues(t *testing.T) {
	bundle := EvidenceBundle{
		EvidenceBundleID: "evb_test",
		CreatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
		Scope: EvidenceScope{
			Repository: "deonclaw",
			AgentID:    LegacyManualAgentID,
		},
		Trigger:    TriggerManualInsightRequest,
		RunIDs:     []string{"run-1"},
		PolicyRefs: []string{"api_key=super-secret-value"},
	}
	if err := scanBundleForSecrets(bundle); err == nil {
		t.Fatal("scanBundleForSecrets() expected error for secret-like policy ref")
	}
}

const exampleInsightPolicyYAML = `
insight_policy:
  enabled: true
  evaluate_after_runs: 1
  evaluate_after_commits: 2
  evaluate_after_turns: 6
  always_on:
    - validation_failed
    - user_correction
    - approval_denied
    - repeated_error
  reviewer:
    preferred: codex
    fallback: opencode
  auto_propose: true
  auto_apply: false
`
