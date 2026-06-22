package insights

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestPlanEvaluationAndMaterializeReport(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 22, 13, 0, 0, 0, time.UTC)
	bundle := EvidenceBundle{
		EvidenceBundleID: "evb_test_plan",
		CreatedAt:        now.Format(time.RFC3339Nano),
		Scope: EvidenceScope{
			Repository: "deonclaw",
			AgentID:    LegacyManualAgentID,
			TaskID:     "task-1",
		},
		Trigger: TriggerValidationFailed,
		RunIDs:  []string{"run-1"},
		SHA256:  "abc123",
	}

	plan, prompt, err := PlanEvaluation(bundle, ReviewerCodex)
	if err != nil {
		t.Fatalf("PlanEvaluation() error = %v", err)
	}
	if plan.WorkerExecution {
		t.Fatal("worker_execution should be false")
	}
	if !strings.Contains(prompt, "DeonClaw learning reviewer") {
		t.Fatal("prompt should include reviewer instructions")
	}

	promptPath := filepath.Join(root, "reviewer-prompt.txt")
	promptSHA, err := WriteEvaluationPrompt(prompt, promptPath)
	if err != nil {
		t.Fatalf("WriteEvaluationPrompt() error = %v", err)
	}
	plan.PromptPath = promptPath
	plan.PromptSHA256 = promptSHA

	planPath := filepath.Join(root, "evaluation-plan.json")
	if err := WriteEvaluationPlan(plan, planPath); err != nil {
		t.Fatalf("WriteEvaluationPlan() error = %v", err)
	}

	responsePath := filepath.Join(root, "reviewer-response.json")
	if err := os.WriteFile(responsePath, []byte(exampleReviewerResponseJSON), 0o644); err != nil {
		t.Fatalf("WriteFile(reviewer-response.json) error = %v", err)
	}
	report, err := MaterializeInsightReport(EvaluateOptions{
		Evidence:     bundle,
		Reviewer:     ReviewerCodex,
		ResponsePath: responsePath,
		Now:          now,
	})
	if err != nil {
		t.Fatalf("MaterializeInsightReport() error = %v", err)
	}
	if report.ProposalCount != 1 {
		t.Fatalf("proposal_count = %d, want 1", report.ProposalCount)
	}
	if report.ContainsChainOfThought {
		t.Fatal("contains_chain_of_thought must be false")
	}
	if err := ValidateReport(report); err != nil {
		t.Fatalf("ValidateReport() error = %v", err)
	}

	reportPath := filepath.Join(root, "insight-report.json")
	if err := WriteReportJSON(report, reportPath); err != nil {
		t.Fatalf("WriteReportJSON() error = %v", err)
	}
	roundTrip, err := ReadReportJSON(reportPath)
	if err != nil {
		t.Fatalf("ReadReportJSON() error = %v", err)
	}
	if roundTrip.InsightID != report.InsightID {
		t.Fatalf("insight_id mismatch: %q vs %q", roundTrip.InsightID, report.InsightID)
	}
}

func TestReadReportRejectsChainOfThoughtField(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad-report.json")
	payload := []byte(`{
  "insight_id": "ins_bad",
  "status": "ok",
  "created_at": "2026-06-22T13:00:00Z",
  "trigger": "run_completed",
  "scope": {"repository":"deonclaw","agent_id":"legacy-manual"},
  "evidence_bundle_sha256": "abc",
  "reviewer": "codex",
  "observations": [],
  "what_worked": [],
  "what_failed": [],
  "reusable_lessons": [],
  "uncertainties": [],
  "risk_notes": [],
  "proposal_count": 0,
  "action_required": false,
  "contains_chain_of_thought": false,
  "sha256": "def",
  "chain_of_thought": "hidden"
}`)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := ReadReportJSON(path); err == nil {
		t.Fatal("ReadReportJSON() expected error for chain_of_thought field")
	}
}

func TestMaterializeProposalsFromInsight(t *testing.T) {
	now := time.Date(2026, 6, 22, 14, 0, 0, 0, time.UTC)
	report := InsightReport{
		InsightID:              "ins_test",
		Status:                 ReportStatusOK,
		CreatedAt:              now.Format(time.RFC3339Nano),
		Trigger:                TriggerValidationFailed,
		Scope:                  EvidenceScope{Repository: "deonclaw", AgentID: LegacyManualAgentID},
		EvidenceBundleSHA256:   "evidence-sha",
		Reviewer:               ReviewerCodex,
		Observations:           []string{},
		WhatWorked:             []string{},
		WhatFailed:             []string{},
		ReusableLessons:        []string{},
		Uncertainties:          []string{},
		RiskNotes:              []string{},
		ProposalCount:          1,
		ActionRequired:         true,
		ContainsChainOfThought: false,
	}
	hash, err := ReportHash(report)
	if err != nil {
		t.Fatalf("ReportHash() error = %v", err)
	}
	report.SHA256 = hash

	var response ReviewerResponse
	if err := json.Unmarshal([]byte(exampleReviewerResponseJSON), &response); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	bundle, err := MaterializeProposals(MaterializeProposalsOptions{
		Report:   report,
		Response: response,
		Policy:   Policy{AutoPropose: true, AutoApply: false},
		Now:      now,
	})
	if err != nil {
		t.Fatalf("MaterializeProposals() error = %v", err)
	}
	if len(bundle.Proposals) != 1 {
		t.Fatalf("proposal_count = %d, want 1", len(bundle.Proposals))
	}
	if bundle.Proposals[0].EvidenceBundleSHA256 != "evidence-sha" {
		t.Fatalf("evidence_bundle_sha256 = %q", bundle.Proposals[0].EvidenceBundleSHA256)
	}
	if bundle.Proposals[0].ApprovalRequired != true {
		t.Fatal("approval_required should be true")
	}
}

func TestAutoApplyNotAllowedForHighRiskProposalTypes(t *testing.T) {
	policy := Policy{AutoPropose: true, AutoApply: true}
	for _, proposalType := range highRiskProposalTypesList() {
		if autoApplyAllowed(policy, proposalType) {
			t.Fatalf("autoApplyAllowed() = true for high-risk type %q", proposalType)
		}
	}
	if err := ValidateProposalPolicy(policy); err != nil {
		t.Fatalf("ValidateProposalPolicy() error = %v", err)
	}
}

func TestMaterializeProposalsRejectsUnknownType(t *testing.T) {
	now := time.Now().UTC()
	report := InsightReport{
		InsightID:              "ins_bad",
		Status:                 ReportStatusOK,
		CreatedAt:              now.Format(time.RFC3339Nano),
		Trigger:                TriggerRunCompleted,
		Scope:                  EvidenceScope{Repository: "deonclaw", AgentID: LegacyManualAgentID},
		EvidenceBundleSHA256:   "evidence-sha",
		Reviewer:               ReviewerCodex,
		Observations:           []string{},
		WhatWorked:             []string{},
		WhatFailed:             []string{},
		ReusableLessons:        []string{},
		Uncertainties:          []string{},
		RiskNotes:              []string{},
		ContainsChainOfThought: false,
	}
	hash, err := ReportHash(report)
	if err != nil {
		t.Fatalf("ReportHash() error = %v", err)
	}
	report.SHA256 = hash

	_, err = MaterializeProposals(MaterializeProposalsOptions{
		Report: report,
		Response: ReviewerResponse{
			Proposals: []ReviewerProposalDraft{{
				Type:                  "not_allowed",
				Target:                "x",
				Reason:                "y",
				ProposedChangeSummary: "z",
			}},
		},
		Policy: Policy{AutoPropose: true},
		Now:    now,
	})
	if err == nil {
		t.Fatal("MaterializeProposals() expected error for unknown proposal type")
	}
}

func testLearningProposal(t *testing.T, proposalType string, patchPath string) (LearningProposal, LearningProposalBundle) {
	t.Helper()
	now := time.Date(2026, 6, 22, 15, 0, 0, 0, time.UTC)
	proposal := LearningProposal{
		ProposalID:            "lp_test_doc",
		Type:                  proposalType,
		Status:                ProposalStatusPending,
		Target:                "docs/INSIGHT_LEARNING_LOOP.md",
		InsightID:             "ins_test",
		InsightSHA256:         "insight-sha",
		EvidenceBundleSHA256:  "evidence-sha",
		Reason:                "Document validation_failed trigger behavior.",
		ProposedChangeSummary: "Add an example for validation_failed evidence bundles.",
		PatchPath:             patchPath,
		Confidence:            0.82,
		AutoApplyAllowed:      false,
		ApprovalRequired:      true,
		CreatedAt:             now.Format(time.RFC3339Nano),
	}
	hash, err := ProposalHash(proposal)
	if err != nil {
		t.Fatalf("ProposalHash() error = %v", err)
	}
	proposal.SHA256 = hash

	bundle := LearningProposalBundle{
		InsightID:            "ins_test",
		InsightSHA256:        "insight-sha",
		EvidenceBundleSHA256: "evidence-sha",
		CreatedAt:            now.Format(time.RFC3339Nano),
		Proposals:            []LearningProposal{proposal},
	}
	bundleHash, err := ProposalBundleHash(bundle)
	if err != nil {
		t.Fatalf("ProposalBundleHash() error = %v", err)
	}
	bundle.SHA256 = bundleHash
	return proposal, bundle
}

func TestValidateApprovalAgainstProposalRejectsStaleHash(t *testing.T) {
	proposal, bundle := testLearningProposal(t, ProposalTypeDocumentation, "")
	approval, err := BuildProposalApproval(proposal, bundle, BuildProposalApprovalOptions{
		Reviewer: ReviewerCodex,
		Decision: ApprovalDecisionApproved,
		Reason:   "Looks good.",
	})
	if err != nil {
		t.Fatalf("BuildProposalApproval() error = %v", err)
	}
	proposal.SHA256 = "stale-hash"
	err = ValidateApprovalAgainstProposal(approval, proposal)
	if err == nil {
		t.Fatal("ValidateApprovalAgainstProposal() expected stale hash error")
	}
	if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("error = %v, want stale hash message", err)
	}
}

func TestApplyDryRunBlockedWithoutApproval(t *testing.T) {
	proposal, _ := testLearningProposal(t, ProposalTypeDocumentation, "")
	result, err := ApplyDryRun(ApplyOptions{Proposal: proposal})
	if err != nil {
		t.Fatalf("ApplyDryRun() error = %v", err)
	}
	if result.WouldApply {
		t.Fatal("ApplyDryRun() would_apply = true without approval")
	}
	if result.BlockedReason == "" {
		t.Fatal("ApplyDryRun() blocked_reason is empty")
	}
}

func TestApplyDryRunOKWithApprovedApproval(t *testing.T) {
	proposal, bundle := testLearningProposal(t, ProposalTypeDocumentation, "")
	approval, err := BuildProposalApproval(proposal, bundle, BuildProposalApprovalOptions{
		Reviewer: ReviewerCodex,
		Decision: ApprovalDecisionApproved,
		Reason:   "Approved for preview apply.",
	})
	if err != nil {
		t.Fatalf("BuildProposalApproval() error = %v", err)
	}
	result, err := ApplyDryRun(ApplyOptions{Proposal: proposal, Approval: approval})
	if err != nil {
		t.Fatalf("ApplyDryRun() error = %v", err)
	}
	if !result.WouldApply {
		t.Fatalf("ApplyDryRun() would_apply = false, blocked_reason=%q", result.BlockedReason)
	}
	if result.Status != ApplyStatusOK {
		t.Fatalf("ApplyDryRun() status = %q, want %q", result.Status, ApplyStatusOK)
	}
}

func TestApplyDryRunSkillPatchBlockedWithoutPatchPath(t *testing.T) {
	proposal, bundle := testLearningProposal(t, ProposalTypeSkillPatch, "")
	approval, err := BuildProposalApproval(proposal, bundle, BuildProposalApprovalOptions{
		Reviewer: ReviewerCodex,
		Decision: ApprovalDecisionApproved,
		Reason:   "Approved for review.",
	})
	if err != nil {
		t.Fatalf("BuildProposalApproval() error = %v", err)
	}
	result, err := ApplyDryRun(ApplyOptions{Proposal: proposal, Approval: approval})
	if err != nil {
		t.Fatalf("ApplyDryRun() error = %v", err)
	}
	if result.WouldApply {
		t.Fatal("ApplyDryRun() would_apply = true for skill_patch without patch_path")
	}
	if !strings.Contains(result.BlockedReason, "patch_path") {
		t.Fatalf("blocked_reason = %q, want patch_path requirement", result.BlockedReason)
	}
}

func TestRecordEffectivenessLinksProposalToRun(t *testing.T) {
	proposal, _ := testLearningProposal(t, ProposalTypeDocumentation, "")
	record, err := RecordEffectiveness(RecordEffectivenessOptions{
		Proposal: proposal,
		RunID:    "run-abc",
		Metric:   EffectivenessMetricValidationPassRate,
		Value:    1.0,
	})
	if err != nil {
		t.Fatalf("RecordEffectiveness() error = %v", err)
	}
	if record.ProposalID != proposal.ProposalID {
		t.Fatalf("proposal_id = %q, want %q", record.ProposalID, proposal.ProposalID)
	}
	if record.RunID != "run-abc" {
		t.Fatalf("run_id = %q, want run-abc", record.RunID)
	}
	if record.Metric != EffectivenessMetricValidationPassRate {
		t.Fatalf("metric = %q", record.Metric)
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

const exampleReviewerResponseJSON = `{
  "observations": ["Validation failed after the run completed."],
  "what_worked": ["Evidence bundle captured validation status and diff summary."],
  "what_failed": ["go test ./... exited non-zero."],
  "reusable_lessons": ["Failed validation runs should trigger insight review before retry."],
  "uncertainties": [],
  "risk_notes": [],
  "proposals": [
    {
      "type": "documentation",
      "target": "docs/INSIGHT_LEARNING_LOOP.md",
      "reason": "Document validation_failed trigger behavior.",
      "proposed_change_summary": "Add an example for validation_failed evidence bundles.",
      "confidence": 0.82
    }
  ],
  "action_required": true
}`
