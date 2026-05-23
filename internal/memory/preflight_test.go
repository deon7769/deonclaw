package memory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestApplyPreflightOKPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-ok")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{
		CreatedAt: time.Date(2026, 5, 22, 13, 0, 0, 0, time.UTC),
	})
	if preflight.Status != ApplyPreflightStatusOK {
		t.Fatalf("status = %q, want %q: failures=%#v", preflight.Status, ApplyPreflightStatusOK, preflight.Failures)
	}
	if len(preflight.Failures) != 0 {
		t.Fatalf("failures = %#v, want none", preflight.Failures)
	}
	if preflight.LintStatus != LintStatusOK {
		t.Fatalf("lint_status = %q, want ok", preflight.LintStatus)
	}
	if preflight.ApplyStatus != ApplyStatusDryRunOK {
		t.Fatalf("apply_status = %q, want dry_run_ok", preflight.ApplyStatus)
	}
	if preflight.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", preflight.PatchCount)
	}
	if preflight.ProposalSHA256 == "" || preflight.ApplyPreviewSHA256 == "" {
		t.Fatalf("preflight hashes are empty: %#v", preflight)
	}
	if preflight.ProposalSHA256 != approval.ProposalSHA256 {
		t.Fatalf("proposal_sha256 = %q, want approval hash %q", preflight.ProposalSHA256, approval.ProposalSHA256)
	}
	if preflight.ApplyPreviewSHA256 != approval.ApplyPreviewSHA256 {
		t.Fatalf("apply_preview_sha256 = %q, want approval hash %q", preflight.ApplyPreviewSHA256, approval.ApplyPreviewSHA256)
	}
	if preflight.ApprovalProposalSHA256 != approval.ProposalSHA256 {
		t.Fatalf("approval_proposal_sha256 = %q, want %q", preflight.ApprovalProposalSHA256, approval.ProposalSHA256)
	}
	if preflight.ApprovalApplyPreviewSHA256 != approval.ApplyPreviewSHA256 {
		t.Fatalf("approval_apply_preview_sha256 = %q, want %q", preflight.ApprovalApplyPreviewSHA256, approval.ApplyPreviewSHA256)
	}
}

func TestApplyPreflightProposalChangedAfterApprovalFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-proposal-hash-changed")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	proposal.Reason = "Changed after approval."

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "proposal_sha256")
}

func TestApplyPreflightPatchContentChangedAfterApprovalFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-patch-content-changed")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	proposal.Patches[0].Content = "changed content\n"

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "proposal_sha256")
	assertApplyPreflightFailureContains(t, preflight, "apply_preview_sha256")
}

func TestApplyPreflightPatchTargetChangedAfterApprovalFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-patch-target-changed")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	proposal.Patches[0].TargetPath = "/vault/mysecondbrain/memory/inbox/run-002.md"

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "proposal_sha256")
	assertApplyPreflightFailureContains(t, preflight, "apply_preview_sha256")
}

func TestApplyPreflightApplyPreviewHashMismatchFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-apply-preview-hash-mismatch")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	approval.ApplyPreviewSHA256 = strings.Repeat("0", 64)

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "apply_preview_sha256")
}

func TestApplyPreflightApprovalForDifferentProposalFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-proposal")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	approval.ProposalID = "mem-other"

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "approval proposal_id")
}

func TestApplyPreflightRejectedDecisionFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-rejected")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	approval.Decision = DecisionRejected

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "approval decision")
}

func TestApplyPreflightCurrentLintFailureFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-lint-changed")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	proposal.TargetPath = "/vault/mysecondbrain/SOUL.md"
	approval.TargetPath = proposal.TargetPath

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "current lint_status")
}

func TestApplyPreflightCurrentApplyFailureFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-apply-changed")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: OperationUpdate, Content: "bad\n"},
	}

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "current apply_status")
	assertApplyPreflightFailureContains(t, preflight, "current patch_violations")
}

func TestApplyPreflightApprovalPatchViolationsFail(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-approval-patch-violations")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	approval.PatchViolations = []ApplyPatchViolation{
		{PatchIndex: 0, TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: OperationUpdate, Violation: "protected path"},
	}

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	assertApplyPreflightFailureContains(t, preflight, "approval patch_violations")
}

func TestApplyPreflightJSONValid(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyPreflightProposal("mem-preflight-json")
	approval := mustBuildApplyPreflightApproval(t, proposal, policy)
	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})

	data, err := preflight.JSON()
	if err != nil {
		t.Fatalf("preflight.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("preflight JSON is invalid: %s", data)
	}
	output := string(data)
	for _, want := range []string{
		`"proposal_sha256"`,
		`"approval_proposal_sha256"`,
		`"apply_preview_sha256"`,
		`"approval_apply_preview_sha256"`,
		`"status"`,
		`"failures"`,
		`"lint_status"`,
		`"apply_status"`,
		`"patch_violations"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("preflight JSON = %s, want field %s", output, want)
		}
	}
}

func testApplyPreflightProposal(id string) MemoryProposal {
	return NewProposal(NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: "/vault/mysecondbrain/memory/inbox/run-001.md",
		Operation:  OperationAppend,
		Reason:     "Preflight proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 13, 0, 0, 0, time.UTC),
		Patches: []MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/memory/inbox/run-001.md", Operation: OperationAppend, Content: "content\n"},
		},
	})
}

func mustBuildApplyPreflightApproval(t *testing.T, proposal MemoryProposal, policy *MemoryPolicy) MemoryApproval {
	t.Helper()
	approval, err := BuildApproval(proposal, policy, NewApprovalOptions{
		ApprovalID: "approval-" + proposal.ProposalID,
		Reviewer:   "Davi",
		Decision:   DecisionApproved,
		Reason:     "Approved.",
		CreatedAt:  time.Date(2026, 5, 22, 13, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	return approval
}

func assertApplyPreflightFailureContains(t *testing.T, preflight ApplyPreflight, want string) {
	t.Helper()
	if preflight.Status != ApplyPreflightStatusFailed {
		t.Fatalf("status = %q, want failed", preflight.Status)
	}
	for _, failure := range preflight.Failures {
		if strings.Contains(failure, want) {
			return
		}
	}
	t.Fatalf("failures = %#v, want one containing %q", preflight.Failures, want)
}
