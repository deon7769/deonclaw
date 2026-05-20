package memory

import (
	"strings"
	"testing"
	"time"
)

func TestLintValidProposalPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-lint-valid", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", OperationAppend)

	result := LintProposal(proposal, policy)
	if result.Status != LintStatusOK {
		t.Fatalf("status = %q, want %q: violations=%v", result.Status, LintStatusOK, result.Violations)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %#v, want none", result.Violations)
	}
}

func TestLintInvalidOperationFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-lint-bad-op", "general", "/vault/mysecondbrain/memory/inbox/run-001.md", MemoryOperation("delete"))

	result := LintProposal(proposal, policy)
	if result.Status != LintStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, LintStatusFailed)
	}
	assertLintViolationContains(t, result, `operation "delete" is not supported`)
}

func TestLintEscalasoftMemoryMDFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-lint-escalasoft-memory", "escalasoft", "/vault/mysecondbrain/MEMORY.md", OperationUpdate)

	result := LintProposal(proposal, policy)
	if result.Status != LintStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, LintStatusFailed)
	}
	assertLintViolationContains(t, result, "MEMORY.md")
}

func TestLintEscalasoftBrainPasses(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-lint-escalasoft-root", "escalasoft", "/domains/escalasoft_brain/cases/case-001.md", OperationUpdate)

	result := LintProposal(proposal, policy)
	if result.Status != LintStatusOK {
		t.Fatalf("status = %q, want %q: violations=%v", result.Status, LintStatusOK, result.Violations)
	}
	if len(result.Violations) != 0 {
		t.Fatalf("violations = %#v, want none", result.Violations)
	}
}

func TestLintProtectedPathFails(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-lint-protected", "general", "/vault/mysecondbrain/SOUL.md", OperationUpdate)

	result := LintProposal(proposal, policy)
	if result.Status != LintStatusFailed {
		t.Fatalf("status = %q, want %q", result.Status, LintStatusFailed)
	}
	assertLintViolationContains(t, result, "protected path")
}

func TestLintAllowedGlobalBridgePassesWithWarning(t *testing.T) {
	policy := loadExamplePolicy(t)
	proposal := testProposal("mem-lint-bridge", "escalasoft", "/vault/mysecondbrain/memory/context/escalasoft-operacao.md", OperationUpdate)

	result := LintProposal(proposal, policy)
	if result.Status != LintStatusOK {
		t.Fatalf("status = %q, want %q: violations=%v", result.Status, LintStatusOK, result.Violations)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("warnings = %#v, want allowed bridge warning", result.Warnings)
	}
	if !strings.Contains(result.Warnings[0], "allowed_global_bridge") {
		t.Fatalf("warnings = %#v, want allowed_global_bridge warning", result.Warnings)
	}
}

func loadExamplePolicy(t *testing.T) *MemoryPolicy {
	t.Helper()
	policy, err := LoadPolicyFromFile("../../configs/examples/memory-policy.yaml")
	if err != nil {
		t.Fatalf("LoadPolicyFromFile() error = %v", err)
	}
	return policy
}

func testProposal(id string, domain string, targetPath string, operation MemoryOperation) MemoryProposal {
	return NewProposal(NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     domain,
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Lint proposal.",
		CreatedAt:  time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
	})
}

func assertLintViolationContains(t *testing.T, result LintResult, want string) {
	t.Helper()
	for _, violation := range result.Violations {
		if strings.Contains(violation, want) {
			return
		}
	}
	t.Fatalf("violations = %#v, want one containing %q", result.Violations, want)
}
