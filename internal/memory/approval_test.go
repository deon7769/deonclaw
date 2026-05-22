package memory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuildApprovalApprovedWithLintOK(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-approval-ok", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend)

	approval, err := BuildApproval(proposal, policy, NewApprovalOptions{
		ApprovalID: "approval-001",
		Reviewer:   "Davi",
		Decision:   DecisionApproved,
		Reason:     "Looks correct.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}

	if approval.ApprovalID != "approval-001" {
		t.Fatalf("approval_id = %q, want approval-001", approval.ApprovalID)
	}
	if approval.ProposalID != proposal.ProposalID || approval.RunID != proposal.RunID || approval.TaskID != proposal.TaskID {
		t.Fatalf("approval linkage = %#v, want proposal ids", approval)
	}
	if approval.Decision != DecisionApproved {
		t.Fatalf("decision = %q, want approved", approval.Decision)
	}
	if approval.LintStatus != LintStatusOK {
		t.Fatalf("lint_status = %q, want ok", approval.LintStatus)
	}
	if len(approval.LintWarnings) != 0 || len(approval.LintViolations) != 0 {
		t.Fatalf("lint warnings/violations = %#v/%#v, want none", approval.LintWarnings, approval.LintViolations)
	}
	if approval.ApplyStatus != ApplyStatusDryRunOK {
		t.Fatalf("apply_status = %q, want %q", approval.ApplyStatus, ApplyStatusDryRunOK)
	}
	if approval.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", approval.PatchCount)
	}
	if len(approval.PatchViolations) != 0 {
		t.Fatalf("patch_violations = %#v, want none", approval.PatchViolations)
	}

	data, err := approval.JSON()
	if err != nil {
		t.Fatalf("approval.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("approval JSON is invalid: %s", data)
	}
	output := string(data)
	for _, want := range []string{
		`"apply_status"`,
		`"patch_count"`,
		`"patch_warnings"`,
		`"patch_violations"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("approval JSON = %s, want field %s", output, want)
		}
	}
}

func TestBuildApprovalApprovedWithLintFailedFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-approval-failed-approve", "escalasoft", "/vault/mysecondbrain/MEMORY.md", OperationUpdate)

	_, err := BuildApproval(proposal, policy, NewApprovalOptions{
		Reviewer:  "Davi",
		Decision:  DecisionApproved,
		Reason:    "Approve despite lint.",
		CreatedAt: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("BuildApproval() error = nil, want lint failure")
	}
	if !strings.Contains(err.Error(), "approved decision requires proposal lint status ok") {
		t.Fatalf("BuildApproval() error = %q, want lint status error", err.Error())
	}
}

func TestBuildApprovalApprovedWithPatchViolationFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApprovalPatchViolationProposal("mem-approval-patch-failed-approve")

	_, err := BuildApproval(proposal, policy, NewApprovalOptions{
		Reviewer:  "Davi",
		Decision:  DecisionApproved,
		Reason:    "Approve despite patch violation.",
		CreatedAt: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("BuildApproval() error = nil, want apply dry-run failure")
	}
	if !strings.Contains(err.Error(), "approved decision requires apply_status dry_run_ok") {
		t.Fatalf("BuildApproval() error = %q, want apply status error", err.Error())
	}
}

func TestBuildApprovalRejectedWithLintFailedPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-approval-failed-reject", "escalasoft", "/vault/mysecondbrain/MEMORY.md", OperationUpdate)

	approval, err := BuildApproval(proposal, policy, NewApprovalOptions{
		Reviewer:  "Davi",
		Decision:  DecisionRejected,
		Reason:    "Reject policy violation.",
		CreatedAt: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	if approval.Decision != DecisionRejected {
		t.Fatalf("decision = %q, want rejected", approval.Decision)
	}
	if approval.LintStatus != LintStatusFailed {
		t.Fatalf("lint_status = %q, want failed", approval.LintStatus)
	}
	if len(approval.LintViolations) == 0 {
		t.Fatalf("lint_violations = %#v, want policy violations", approval.LintViolations)
	}
	if approval.ApplyStatus != ApplyStatusFailed {
		t.Fatalf("apply_status = %q, want failed", approval.ApplyStatus)
	}
}

func TestBuildApprovalRejectedWithPatchViolationPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApprovalPatchViolationProposal("mem-approval-patch-failed-reject")

	approval, err := BuildApproval(proposal, policy, NewApprovalOptions{
		Reviewer:  "Davi",
		Decision:  DecisionRejected,
		Reason:    "Reject patch violation.",
		CreatedAt: time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	if approval.ApplyStatus != ApplyStatusFailed {
		t.Fatalf("apply_status = %q, want failed", approval.ApplyStatus)
	}
	if approval.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", approval.PatchCount)
	}
	if len(approval.PatchViolations) != 1 || !strings.Contains(approval.PatchViolations[0].Violation, "protected path") {
		t.Fatalf("patch_violations = %#v, want protected path violation", approval.PatchViolations)
	}
}

func TestBuildApprovalRequiresReviewer(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-approval-reviewer", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend)

	_, err := BuildApproval(proposal, policy, NewApprovalOptions{
		Decision: DecisionApproved,
		Reason:   "Reviewed.",
	})
	if err == nil {
		t.Fatal("BuildApproval() error = nil, want missing reviewer")
	}
	if !strings.Contains(err.Error(), "reviewer is required") {
		t.Fatalf("BuildApproval() error = %q, want reviewer required", err.Error())
	}
}

func TestBuildApprovalRequiresReason(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-approval-reason", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend)

	_, err := BuildApproval(proposal, policy, NewApprovalOptions{
		Reviewer: "Davi",
		Decision: DecisionApproved,
	})
	if err == nil {
		t.Fatal("BuildApproval() error = nil, want missing reason")
	}
	if !strings.Contains(err.Error(), "reason is required") {
		t.Fatalf("BuildApproval() error = %q, want reason required", err.Error())
	}
}

func TestBuildApprovalRejectsInvalidDecision(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-approval-bad-decision", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend)

	_, err := BuildApproval(proposal, policy, NewApprovalOptions{Reviewer: "Davi", Decision: ApprovalDecision("maybe"), Reason: "Invalid."})
	if err == nil {
		t.Fatal("BuildApproval() error = nil, want invalid decision")
	}
	if !strings.Contains(err.Error(), "decision \"maybe\" is not supported") {
		t.Fatalf("BuildApproval() error = %q, want invalid decision", err.Error())
	}
}

func testApprovalPatchViolationProposal(id string) MemoryProposal {
	proposal := testProposal(id, "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend)
	proposal.Patches = []MemoryPatch{
		{
			TargetPath: "/vault/mysecondbrain/SOUL.md",
			Operation:  OperationUpdate,
			Content:    "bad\n",
		},
	}
	return proposal
}
