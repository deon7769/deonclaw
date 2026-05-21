package memory

import (
	"os"
	"path/filepath"
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

func TestApplyDryRunPatchProtectedTargetFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-patch-protected", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend, "safe\n")
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: OperationUpdate, Content: "bad\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err == nil {
		t.Fatal("BuildApplyDryRunPreview() error = nil, want patch policy failure")
	}
	if preview.Status != ApplyStatusFailed {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusFailed)
	}
	assertApplyPatchViolationContains(t, preview, "protected path")
}

func TestApplyDryRunPatchForbiddenGlobalWriteFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-patch-forbidden-global", "escalasoft", "/domains/escalasoft_brain/cases/case-001.md", OperationUpdate, "safe\n")
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/memory/context/escalasoft-raw.md", Operation: OperationUpdate, Content: "bad\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err == nil {
		t.Fatal("BuildApplyDryRunPreview() error = nil, want patch policy failure")
	}
	if preview.Status != ApplyStatusFailed {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusFailed)
	}
	assertApplyPatchViolationContains(t, preview, "forbidden_global_write")
}

func TestApplyDryRunPatchInvalidOperationFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-patch-invalid-op", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend, "safe\n")
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/memory/inbox/run-001.md", Operation: MemoryOperation("delete"), Content: "bad\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err == nil {
		t.Fatal("BuildApplyDryRunPreview() error = nil, want patch operation failure")
	}
	if preview.Status != ApplyStatusFailed {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusFailed)
	}
	assertApplyPatchViolationContains(t, preview, `operation "delete" is not supported`)
}

func TestApplyDryRunPatchAllowedGlobalBridgeWarns(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-patch-bridge", "escalasoft", "/domains/escalasoft_brain/cases/case-001.md", OperationUpdate, "bridge\n")
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/memory/context/escalasoft-operacao.md", Operation: OperationUpdate, Content: "bridge\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if preview.Status != ApplyStatusDryRunOK {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusDryRunOK)
	}
	if len(preview.PatchViolations) != 0 {
		t.Fatalf("patch violations = %#v, want none", preview.PatchViolations)
	}
	assertApplyPatchWarningContains(t, preview, "allowed_global_bridge")
}

func TestApplyDryRunMultiplePatchesOneInvalidFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-multiple-one-invalid", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend, "safe\n")
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/memory/inbox/run-001.md", Operation: OperationAppend, Content: "safe\n"},
		{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: OperationUpdate, Content: "bad\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err == nil {
		t.Fatal("BuildApplyDryRunPreview() error = nil, want one patch failure")
	}
	if preview.Status != ApplyStatusFailed {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusFailed)
	}
	if preview.PatchCount != 2 || len(preview.Actions) != 2 {
		t.Fatalf("preview patch_count/actions = %d/%d, want 2/2", preview.PatchCount, len(preview.Actions))
	}
	if len(preview.PatchViolations) != 1 || preview.PatchViolations[0].PatchIndex != 1 {
		t.Fatalf("patch violations = %#v, want one violation for patch index 1", preview.PatchViolations)
	}
}

func TestApplyDryRunPatchWithoutTargetUsesProposalTarget(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-patch-default-target", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend, "safe\n")
	proposal.Patches = []MemoryPatch{
		{Operation: OperationAppend, Content: "safe\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if len(preview.Actions) != 1 || preview.Actions[0].TargetPath != proposal.TargetPath {
		t.Fatalf("actions = %#v, want proposal target_path %q", preview.Actions, proposal.TargetPath)
	}
}

func TestApplyDryRunPatchWithoutOperationUsesProposalOperation(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testApplyProposal("mem-apply-patch-default-operation", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationUpdate, "safe\n")
	proposal.Patches = []MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/memory/inbox/run-001.md", Content: "safe\n"},
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if len(preview.Actions) != 1 || preview.Actions[0].Operation != proposal.Operation {
		t.Fatalf("actions = %#v, want proposal operation %q", preview.Actions, proposal.Operation)
	}
}

func TestApplyDryRunDoesNotWriteTargetPath(t *testing.T) {
	policy := loadExamplePolicy(t)
	targetPath := filepath.Join(t.TempDir(), "target.md")
	proposal := testApplyProposal("mem-apply-no-write", "general", targetPath, OperationCreate, "new memory\n")

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
	}
	if preview.Status != ApplyStatusDryRunOK {
		t.Fatalf("status = %q, want %q", preview.Status, ApplyStatusDryRunOK)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run: %v", err)
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

func assertApplyPatchViolationContains(t *testing.T, preview ApplyPreview, want string) {
	t.Helper()
	for _, violation := range preview.PatchViolations {
		if strings.Contains(violation.Violation, want) {
			return
		}
	}
	t.Fatalf("patch violations = %#v, want %q", preview.PatchViolations, want)
}

func assertApplyPatchWarningContains(t *testing.T, preview ApplyPreview, want string) {
	t.Helper()
	for _, warning := range preview.PatchWarnings {
		if strings.Contains(warning, want) {
			return
		}
	}
	t.Fatalf("patch warnings = %#v, want %q", preview.PatchWarnings, want)
}
