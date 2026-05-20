package memory

import (
	"strings"
	"testing"
	"time"
)

func TestApplyDryRunAppendValidPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-append", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend, "append this\n")

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if preview.Status != ApplyStatusDryRunOK {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusDryRunOK)
	}
	if preview.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", preview.PatchCount)
	}
	if len(preview.Actions) != 1 || !strings.Contains(preview.Actions[0].Description, "would append content") {
		t.Fatalf("actions = %#v, want append preview", preview.Actions)
	}
	if preview.Actions[0].Content != "append this\n" {
		t.Fatalf("action content = %q, want patch content", preview.Actions[0].Content)
	}
}

func TestApplyDryRunCreateValidPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-create", "general", "/vault/mysecondbrain/memory/inbox/new.md", OperationCreate, "new file\n")

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if preview.Status != ApplyStatusDryRunOK {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusDryRunOK)
	}
	if len(preview.Actions) != 1 || !strings.Contains(preview.Actions[0].Description, "would create file") {
		t.Fatalf("actions = %#v, want create preview", preview.Actions)
	}
}

func TestApplyDryRunLintFailureBlocksPreview(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-lint-failed", "escalasoft", "/vault/mysecondbrain/MEMORY.md", OperationUpdate, "bad\n")

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err == nil {
		t.Fatal("BuildApplyDryRunPreview() error = nil, want lint failure")
	}
	if preview.Status != ApplyStatusFailed {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusFailed)
	}
	if len(preview.LintViolations) == 0 {
		t.Fatalf("lint violations = %#v, want violations", preview.LintViolations)
	}
}

func TestApplyDryRunInvalidOperationFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-invalid", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", MemoryOperation("delete"), "bad\n")

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err == nil {
		t.Fatal("BuildApplyDryRunPreview() error = nil, want invalid operation")
	}
	if preview.Status != ApplyStatusFailed {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusFailed)
	}
	if len(preview.LintViolations) == 0 || !strings.Contains(preview.LintViolations[0], `operation "delete" is not supported`) {
		t.Fatalf("lint violations = %#v, want invalid operation", preview.LintViolations)
	}
}

func TestApplyDryRunEmptyPatchContentWarns(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-empty", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend, "")

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if len(preview.PatchWarnings) == 0 {
		t.Fatalf("patch warnings = %#v, want empty content warning", preview.PatchWarnings)
	}
}

func testApplyProposal(id string, domain string, targetPath string, operation MemoryOperation, content string) MemoryProposal {
	return NewProposal(NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     domain,
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Apply dry-run proposal.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []MemoryPatch{
			{TargetPath: targetPath, Operation: operation, Content: content},
		},
	})
}
