package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/memory"
	"github.com/deon7769/deonclaw/internal/runs"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deonclaw ") {
		t.Fatalf("stdout = %q, want version output", stdout.String())
	}
}

func TestRunTaskValidate(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"task", "validate", taskPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "task worker-mismatch-001: valid") {
		t.Fatalf("stdout = %q, want task valid output", stdout.String())
	}
}

func TestRunDomainsValidate(t *testing.T) {
	configPath := writeDomainsConfigFile(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "domains config valid: 2 domains") {
		t.Fatalf("stdout = %q, want domains valid output", stdout.String())
	}
}

func TestRunDomainsValidateRejectsBadArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --config") {
		t.Fatalf("stderr = %q, want missing --config", stderr.String())
	}
}

func TestRunDomainsList(t *testing.T) {
	configPath := writeDomainsConfigFile(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"name",
		"type",
		"root",
		"default",
		"isolated",
		"default_agent",
		"general",
		"canonical_memory",
		"/vault/mysecondbrain",
		"escalasoft",
		"isolated_domain",
		"/domains/escalasoft_brain",
		"escalasoft-agent",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunContextBuild(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")
	domainsPath := writeDomainsConfigFile(t)
	outputPath := filepath.Join(t.TempDir(), "context.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"context", "build", "--task", taskPath, "--domains", domainsPath, "--output", outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "context pack written:") {
		t.Fatalf("stdout = %q, want context pack written output", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	output := string(data)
	for _, want := range []string{"id: worker-mismatch-001", "domain: general", "context sources"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunMemoryProposalNew(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "memory-proposal.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"memory",
		"proposal",
		"new",
		"--run",
		"run-001",
		"--task",
		"task-001",
		"--domain",
		"general",
		"--target",
		"memory/context/example.md",
		"--operation",
		"append",
		"--reason",
		"Capture stable workflow.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "memory proposal written: "+outputPath) {
		t.Fatalf("stdout = %q, want proposal written output", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	var proposal memory.MemoryProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		t.Fatalf("Unmarshal(output) error = %v", err)
	}
	if proposal.Status != memory.StatusProposed {
		t.Fatalf("status = %q, want %q", proposal.Status, memory.StatusProposed)
	}
	if proposal.Operation != memory.OperationAppend {
		t.Fatalf("operation = %q, want append", proposal.Operation)
	}
	if proposal.Domain != "general" || proposal.TargetPath != "memory/context/example.md" {
		t.Fatalf("proposal = %#v, want domain and target", proposal)
	}
	if len(proposal.Evidence) != 1 || proposal.Evidence[0].RunID != "run-001" {
		t.Fatalf("evidence = %#v, want run evidence", proposal.Evidence)
	}
	if len(proposal.Patches) != 1 || proposal.Patches[0].TargetPath != "memory/context/example.md" {
		t.Fatalf("patches = %#v, want target patch", proposal.Patches)
	}
}

func TestRunMemoryProposalNewRejectsInvalidOperation(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "memory-proposal.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"memory",
		"proposal",
		"new",
		"--run",
		"run-001",
		"--task",
		"task-001",
		"--domain",
		"general",
		"--target",
		"memory/context/example.md",
		"--operation",
		"delete",
		"--reason",
		"Invalid operation should fail.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `operation "delete" is not supported`) {
		t.Fatalf("stderr = %q, want invalid operation", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output file exists after invalid operation: %v", err)
	}
}

func TestRunMemoryProposalLint(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := filepath.Join(tempDir, "memory-proposal.json")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-lint-001",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/domains/escalasoft_brain/cases/case-001.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Lint CLI proposal.",
		CreatedAt:  time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
	})
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	if err := os.WriteFile(proposalPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(proposal) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"lint",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"proposal_id: mem-cli-lint-001",
		"status: ok",
		"violations: 0",
		"warnings: 0",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunMemoryProposalApplyDryRunAppend(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory-target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-append",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preview append.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "append content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"proposal_id: mem-cli-apply-append",
		"domain: general",
		"target_path: " + targetPath,
		"operation: append",
		"status: dry_run_ok",
		"patch_count: 1",
		"would append content",
		"append content",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run: %v", err)
	}
}

func TestRunMemoryProposalApplyDryRunCreate(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "new-memory.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-create",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationCreate,
		Reason:     "Preview create.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationCreate, Content: "new memory\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "would create file") {
		t.Fatalf("stdout = %q, want create preview", stdout.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run: %v", err)
	}
}

func TestRunMemoryProposalApplyRejectsLintFailure(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-lint-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/vault/mysecondbrain/MEMORY.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Should fail lint.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "status: failed") {
		t.Fatalf("stdout = %q, want failed preview", stdout.String())
	}
	if !strings.Contains(stdout.String(), "lint violations:") {
		t.Fatalf("stdout = %q, want lint violations", stdout.String())
	}
}

func TestRunMemoryProposalApplyRequiresDryRun(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-require-dry-run",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: filepath.Join(tempDir, "target.md"),
		Operation:  memory.OperationAppend,
		Reason:     "Requires dry-run.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: filepath.Join(tempDir, "target.md"), Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --dry-run") {
		t.Fatalf("stderr = %q, want missing --dry-run", stderr.String())
	}
}

func TestRunMemoryProposalApplyOutputWritesPreviewJSON(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	outputPath := filepath.Join(tempDir, "apply-preview.json")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-output",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preview output.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	var preview memory.ApplyPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatalf("Unmarshal(output) error = %v", err)
	}
	if preview.Status != memory.ApplyStatusDryRunOK {
		t.Fatalf("preview status = %q, want %q", preview.Status, memory.ApplyStatusDryRunOK)
	}
}

func TestRunMemoryProposalApplyRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Do not write preview to target.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
		"--output",
		targetPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write apply preview to target_path") {
		t.Fatalf("stderr = %q, want target_path refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyInvalidOperationFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-invalid-operation",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: filepath.Join(tempDir, "target.md"),
		Operation:  memory.OperationAppend,
		Reason:     "Invalid operation.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: filepath.Join(tempDir, "target.md"), Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	proposal.Operation = memory.MemoryOperation("delete")
	proposal.Patches[0].Operation = memory.MemoryOperation("delete")
	proposalPath := writeRawJSONFile(t, tempDir, "memory-proposal-invalid.json", proposal)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), `operation "delete" is not supported`) {
		t.Fatalf("stdout = %q, want invalid operation", stdout.String())
	}
}

func TestRunMemoryProposalApproveApprovedWithLintOKGeneratesApproval(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory-target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-ok",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Approve lint-ok proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Reviewed and approved.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "memory approval written: "+outputPath) {
		t.Fatalf("stdout = %q, want approval written", stdout.String())
	}

	approval := readMemoryApprovalFile(t, outputPath)
	if approval.ProposalID != "mem-cli-approve-ok" {
		t.Fatalf("proposal_id = %q, want mem-cli-approve-ok", approval.ProposalID)
	}
	if approval.Decision != memory.DecisionApproved {
		t.Fatalf("decision = %q, want approved", approval.Decision)
	}
	if approval.Reviewer != "Davi" || approval.Reason != "Reviewed and approved." {
		t.Fatalf("approval = %#v, want reviewer and reason", approval)
	}
	if approval.LintStatus != memory.LintStatusOK {
		t.Fatalf("lint_status = %q, want ok", approval.LintStatus)
	}
	if len(approval.LintViolations) != 0 {
		t.Fatalf("lint_violations = %#v, want none", approval.LintViolations)
	}
	if approval.ApplyStatus != memory.ApplyStatusDryRunOK {
		t.Fatalf("apply_status = %q, want %q", approval.ApplyStatus, memory.ApplyStatusDryRunOK)
	}
	if approval.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", approval.PatchCount)
	}
	if approval.PatchWarnings == nil {
		t.Fatalf("patch_warnings is nil, want JSON array")
	}
	if approval.PatchViolations == nil || len(approval.PatchViolations) != 0 {
		t.Fatalf("patch_violations = %#v, want empty JSON array", approval.PatchViolations)
	}
	if approval.ProposalSHA256 == "" || approval.ApplyPreviewSHA256 == "" {
		t.Fatalf("approval hashes are empty: %#v", approval)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval: %v", err)
	}
}

func TestRunMemoryProposalApproveApprovedWithLintFailedFails(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-lint-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/vault/mysecondbrain/MEMORY.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Should not approve failed lint.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 5, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	}))
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Approve anyway.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "approved decision requires proposal lint status ok") {
		t.Fatalf("stderr = %q, want approval lint failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("approval output exists after failed approve: %v", err)
	}
}

func TestRunMemoryProposalApproveRejectedWithLintFailedGeneratesApproval(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-reject-lint-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/vault/mysecondbrain/MEMORY.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Reject failed lint.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 10, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	}))
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"rejected",
		"--reason",
		"Violates protected memory boundary.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	approval := readMemoryApprovalFile(t, outputPath)
	if approval.Decision != memory.DecisionRejected {
		t.Fatalf("decision = %q, want rejected", approval.Decision)
	}
	if approval.LintStatus != memory.LintStatusFailed {
		t.Fatalf("lint_status = %q, want failed", approval.LintStatus)
	}
	if len(approval.LintViolations) == 0 {
		t.Fatalf("lint_violations = %#v, want violations", approval.LintViolations)
	}
	if approval.ApplyStatus != memory.ApplyStatusFailed {
		t.Fatalf("apply_status = %q, want failed", approval.ApplyStatus)
	}
}

func TestRunMemoryProposalApproveApprovedWithPatchViolationFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-patch-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Approve patch violation.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 11, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	})
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Approve despite patch violation.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "approved decision requires apply_status dry_run_ok") {
		t.Fatalf("stderr = %q, want apply status failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("approval output exists after failed approve: %v", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval failure: %v", err)
	}
}

func TestRunMemoryProposalApproveRejectedWithPatchViolationGeneratesApproval(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-reject-patch-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Reject patch violation.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 12, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	})
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"rejected",
		"--reason",
		"Patch violates protected path.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	approval := readMemoryApprovalFile(t, outputPath)
	if approval.LintStatus != memory.LintStatusOK {
		t.Fatalf("lint_status = %q, want ok", approval.LintStatus)
	}
	if approval.ApplyStatus != memory.ApplyStatusFailed {
		t.Fatalf("apply_status = %q, want failed", approval.ApplyStatus)
	}
	if approval.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", approval.PatchCount)
	}
	if len(approval.PatchViolations) != 1 || !strings.Contains(approval.PatchViolations[0].Violation, "protected path") {
		t.Fatalf("patch_violations = %#v, want protected path violation", approval.PatchViolations)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval: %v", err)
	}
}

func TestRunMemoryProposalApproveRequiresReviewer(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-missing-reviewer")
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--decision", "approved",
		"--reason", "Reviewed.",
		"--output", outputPath,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --reviewer") {
		t.Fatalf("stderr = %q, want missing reviewer", stderr.String())
	}
}

func TestRunMemoryProposalApproveRequiresReason(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-missing-reason")
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer", "Davi",
		"--decision", "approved",
		"--output", outputPath,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --reason") {
		t.Fatalf("stderr = %q, want missing reason", stderr.String())
	}
}

func TestRunMemoryProposalApproveRejectsInvalidDecision(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-bad-decision")
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer", "Davi",
		"--decision", "maybe",
		"--reason", "Invalid decision.",
		"--output", outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "decision \"maybe\" is not supported") {
		t.Fatalf("stderr = %q, want invalid decision", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("approval output exists after invalid decision: %v", err)
	}
}

func TestRunMemoryProposalApproveRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-output-memory-domain")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Reviewed.",
		"--output",
		"/vault/mysecondbrain/memory-approval.json",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write approval artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalApproveRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Do not write approval to target.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 20, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer", "Davi",
		"--decision", "approved",
		"--reason", "Reviewed.",
		"--output", targetPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write approval artifact to target_path") {
		t.Fatalf("stderr = %q, want target_path refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightOKWritesOutput(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-preflight-ok",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preflight ok.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.ApplyPreflightJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply-preflight",
		"--proposal",
		proposalPath,
		"--approval",
		approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: preflight_ok") {
		t.Fatalf("stdout = %q, want preflight_ok", stdout.String())
	}

	preflight := readApplyPreflightFile(t, outputPath)
	if preflight.Status != memory.ApplyPreflightStatusOK {
		t.Fatalf("status = %q, want preflight_ok", preflight.Status)
	}
	if preflight.LintStatus != memory.LintStatusOK || preflight.ApplyStatus != memory.ApplyStatusDryRunOK {
		t.Fatalf("preflight = %#v, want lint/apply ok", preflight)
	}
	if preflight.ProposalSHA256 == "" || preflight.ApprovalProposalSHA256 == "" || preflight.ApplyPreviewSHA256 == "" || preflight.ApprovalApplyPreviewSHA256 == "" {
		t.Fatalf("preflight hashes are empty: %#v", preflight)
	}
	if preflight.ProposalSHA256 != preflight.ApprovalProposalSHA256 {
		t.Fatalf("proposal hashes = %q/%q, want equal", preflight.ProposalSHA256, preflight.ApprovalProposalSHA256)
	}
	if preflight.ApplyPreviewSHA256 != preflight.ApprovalApplyPreviewSHA256 {
		t.Fatalf("apply preview hashes = %q/%q, want equal", preflight.ApplyPreviewSHA256, preflight.ApprovalApplyPreviewSHA256)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after preflight: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightApprovalForDifferentProposalFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-mismatch")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	approval.ProposalID = "mem-other"
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "approval proposal_id") {
		t.Fatalf("stdout = %q, want proposal_id failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightRejectedDecisionFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-rejected")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionRejected)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "approval decision") {
		t.Fatalf("stdout = %q, want decision failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightCurrentLintFailureFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-lint-failed")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposal.TargetPath = "/vault/mysecondbrain/SOUL.md"
	approval.TargetPath = proposal.TargetPath
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "current lint_status") {
		t.Fatalf("stdout = %q, want current lint failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightCurrentApplyFailureFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	approvedProposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-preflight-apply-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preflight apply failed.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 10, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	proposal := approvedProposal
	proposal.Patches = []memory.MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Content: "bad\n"},
	}
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, approvedProposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "current apply_status") {
		t.Fatalf("stdout = %q, want current apply failure", stdout.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after preflight: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightApprovalPatchViolationsFail(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-approval-patch-violations")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	approval.PatchViolations = []memory.ApplyPatchViolation{
		{PatchIndex: 0, TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Violation: "protected path"},
	}
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "approval patch_violations") {
		t.Fatalf("stdout = %q, want approval patch violation failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-preflight-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preflight output target.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 15, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, targetPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write preflight artifact to target_path") {
		t.Fatalf("stderr = %q, want target_path refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after preflight output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-output-memory-domain")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "/vault/mysecondbrain/apply-preflight.json", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write preflight artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalBackupPlanExistingTargetWritesPlan(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-existing",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup existing target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "new content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "items: 1") {
		t.Fatalf("stdout = %q, want items", stdout.String())
	}

	plan := readBackupPlanFile(t, outputPath)
	if len(plan.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(plan.Items))
	}
	item := plan.Items[0]
	if !item.Exists {
		t.Fatalf("exists = false, want true")
	}
	if item.SHA256 == nil || *item.SHA256 == "" {
		t.Fatalf("sha256 is empty: %#v", item)
	}
	if item.SizeBytes == nil || *item.SizeBytes != int64(len(content)) {
		t.Fatalf("size_bytes = %v, want %d", item.SizeBytes, len(content))
	}
	if len(plan.RestorePlan.Items) != 1 {
		t.Fatalf("restore items = %d, want 1", len(plan.RestorePlan.Items))
	}
	if _, err := os.Stat(item.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("backup path was written unexpectedly: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupPlanMissingTargetWritesNotExists(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "missing.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-missing",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationCreate,
		Reason:     "Backup missing target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 5, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationCreate, Content: "new content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	plan := readBackupPlanFile(t, outputPath)
	item := plan.Items[0]
	if item.Exists {
		t.Fatalf("exists = true, want false")
	}
	if item.SHA256 != nil || item.SizeBytes != nil {
		t.Fatalf("item = %#v, want no hash/size for missing target", item)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupPlanMultiplePatches(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "first.md")
	second := filepath.Join(tempDir, "second.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-multiple",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: first,
		Operation:  memory.OperationAppend,
		Reason:     "Backup multiple targets.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 10, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: first, Operation: memory.OperationAppend, Content: "first\n"},
			{TargetPath: second, Operation: memory.OperationCreate, Content: "second\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	plan := readBackupPlanFile(t, outputPath)
	if len(plan.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(plan.Items))
	}
}

func TestRunMemoryProposalBackupPlanDeduplicatesRepeatedTargets(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "same.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-dedupe",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup repeated target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 12, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "first\n"},
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "second\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	plan := readBackupPlanFile(t, outputPath)
	if len(plan.Items) != 1 {
		t.Fatalf("items = %d, want 1 deduplicated target", len(plan.Items))
	}
	if len(plan.RestorePlan.Items) != 1 {
		t.Fatalf("restore items = %d, want 1 deduplicated target", len(plan.RestorePlan.Items))
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupPlanPreflightFailureBlocksOutput(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-preflight-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup preflight failed.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 15, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	approval.Decision = memory.DecisionRejected
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "apply preflight failed") {
		t.Fatalf("stderr = %q, want preflight failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("backup plan output exists after preflight failure: %v", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupPlanRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup output target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 20, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, targetPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup plan artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupPlanRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-backup-output-memory-domain")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, "/vault/mysecondbrain/backup-plan.json", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup plan artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalBackupMaterializeExistingTargetWritesResult(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)
	outputPath := filepath.Join(tempDir, memory.BackupResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "items: 1") {
		t.Fatalf("stdout = %q, want items", stdout.String())
	}
	result := readBackupResultFile(t, outputPath)
	if len(result.Items) != 1 {
		t.Fatalf("result items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != memory.BackupResultStatusCopied {
		t.Fatalf("status = %q, want %q", item.Status, memory.BackupResultStatusCopied)
	}
	if item.BackupSHA256 == nil || *item.BackupSHA256 == "" {
		t.Fatalf("backup_sha256 is empty: %#v", item)
	}
	if got := string(mustReadCLIFile(t, plan.Items[0].BackupPath)); got != string(content) {
		t.Fatalf("backup content = %q, want %q", got, content)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupMaterializeMissingTargetSkipped(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "missing.md")
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationCreate)
	planPath := writeBackupPlanFile(t, tempDir, plan)
	outputPath := filepath.Join(tempDir, memory.BackupResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	result := readBackupResultFile(t, outputPath)
	if got := result.Items[0].Status; got != memory.BackupResultStatusSkippedMissing {
		t.Fatalf("status = %q, want %q", got, memory.BackupResultStatusSkippedMissing)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupMaterializeRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, targetPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup result artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupMaterializeRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	if err := os.WriteFile(targetPath, []byte("existing content\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, "/vault/mysecondbrain/backup-result.json", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup result artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunWorkerCodexDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker",
		"codex",
		"dry-run",
		filepath.Join("..", "..", "examples", "tasks", "codex-smoke.yaml"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "workspace: .") {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "command: codex exec --json --sandbox read-only --cd . -") {
		t.Fatalf("stdout = %q, want planned command", output)
	}
}

func TestRunWorkerCodexDryRunRejectsMismatchedTaskWorker(t *testing.T) {
	taskPath := writeTaskFile(t, "opencode")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"worker", "codex", "dry-run", taskPath}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	want := `task worker "opencode" does not match requested worker "codex"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunWorkerCodexRunRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing store",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--artifacts-dir", t.TempDir()},
			want: "missing --store",
		},
		{
			name: "missing artifacts dir",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--store", filepath.Join(t.TempDir(), "deonclaw.db")},
			want: "missing --artifacts-dir",
		},
		{
			name: "unknown argument",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--store", "db.sqlite", "--artifacts-dir", "artifacts", "--unknown"},
			want: `unknown argument "--unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run() exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestParseCodexRunOptions(t *testing.T) {
	opts, err := parseCodexRunOptions([]string{"task.yaml", "--store", "deonclaw.db", "--artifacts-dir", "artifacts", "--domains", "domains.yaml", "--memory-policy", "memory-policy.yaml"})
	if err != nil {
		t.Fatalf("parseCodexRunOptions() error = %v", err)
	}
	if opts.taskPath != "task.yaml" {
		t.Fatalf("taskPath = %q, want task.yaml", opts.taskPath)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
	if opts.domainsPath != "domains.yaml" {
		t.Fatalf("domainsPath = %q, want domains.yaml", opts.domainsPath)
	}
	if opts.memoryPolicyPath != "memory-policy.yaml" {
		t.Fatalf("memoryPolicyPath = %q, want memory-policy.yaml", opts.memoryPolicyPath)
	}
}

func TestRunArtifactsPruneDryRun(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	artifactPath := filepath.Join(artifactsDir, "run-old", "summary.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("summary\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Task",
		Domain: "general",
		Worker: "codex",
		Goal:   "Test prune",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"dry-run lists artifacts"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	runRecord := &runs.Run{
		ID:            "run-old",
		TaskID:        task.ID,
		Status:        runs.StatusSucceeded,
		Worker:        "codex",
		WorkspacePath: "workspace",
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	if err := db.SaveArtifact(ctx, &artifacts.Artifact{
		ID:        "artifact-old",
		RunID:     runRecord.ID,
		Path:      artifactPath,
		Kind:      artifacts.KindSummary,
		SizeBytes: 8,
		CreatedAt: oldTime,
	}); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"artifacts",
		"prune",
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
		"--older-than",
		"30d",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{"dry_run: true", "older_than: 30d", "candidates: 1", "deleted: 0", "skipped: 0"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("artifact file was removed in dry-run: %v", err)
	}

	db, err = storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	gotArtifacts, err := db.ArtifactsByRun(ctx, runRecord.ID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	if len(gotArtifacts) != 1 {
		t.Fatalf("len(artifacts) = %d, want 1 after dry-run", len(gotArtifacts))
	}
}

func TestRunArtifactsList(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Task",
		Domain: "general",
		Worker: "codex",
		Goal:   "List artifacts",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"artifacts are listed"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	createdAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	runRecords := []runs.Run{
		{ID: "run-succeeded", TaskID: task.ID, Status: runs.StatusSucceeded, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "run-failed", TaskID: task.ID, Status: runs.StatusFailed, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	for i := range runRecords {
		if err := db.SaveRun(ctx, &runRecords[i]); err != nil {
			t.Fatalf("SaveRun() error = %v", err)
		}
	}
	artifactRecords := []artifacts.Artifact{
		{
			ID:        "artifact-summary",
			RunID:     "run-succeeded",
			Path:      filepath.Join("artifacts", "run-succeeded", "summary.md"),
			Kind:      artifacts.KindSummary,
			SizeBytes: 12,
			SHA256:    strings.Repeat("a", 64),
			CreatedAt: createdAt,
		},
		{
			ID:        "artifact-stderr",
			RunID:     "run-failed",
			Path:      filepath.Join("artifacts", "run-failed", "stderr.log"),
			Kind:      artifacts.KindLog,
			SizeBytes: 9,
			SHA256:    strings.Repeat("b", 64),
			Keep:      true,
			CreatedAt: createdAt.Add(time.Second),
		},
	}
	for i := range artifactRecords {
		if err := db.SaveArtifact(ctx, &artifactRecords[i]); err != nil {
			t.Fatalf("SaveArtifact() error = %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"artifacts", "list", "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"artifact_id", "run_id", "kind", "path", "size_bytes", "sha256", "keep", "created_at", "artifact-summary", "artifact-stderr"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"artifacts", "list", "--store", storePath, "--run", "run-succeeded"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--run) exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "artifact-summary") || strings.Contains(stdout.String(), "artifact-stderr") {
		t.Fatalf("stdout with --run = %q, want only artifact-summary", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"artifacts", "list", "--store", storePath, "--status", string(runs.StatusFailed)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--status) exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "artifact-stderr") || strings.Contains(stdout.String(), "artifact-summary") {
		t.Fatalf("stdout with --status = %q, want only artifact-stderr", stdout.String())
	}
}

func TestParseArtifactPruneOptions(t *testing.T) {
	opts, err := parseArtifactPruneOptions([]string{"--store", "deonclaw.db", "--artifacts-dir", "artifacts", "--older-than", "30d", "--dry-run"})
	if err != nil {
		t.Fatalf("parseArtifactPruneOptions() error = %v", err)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
	if opts.olderThan != 30*24*time.Hour {
		t.Fatalf("olderThan = %s, want 720h", opts.olderThan)
	}
	if !opts.dryRun {
		t.Fatal("dryRun = false, want true")
	}
}

func writeTaskFile(t *testing.T, worker string) string {
	t.Helper()

	content := `id: worker-mismatch-001
title: "Worker mismatch"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeDomainsConfigFile(t *testing.T) string {
	t.Helper()

	content := `domains:
  general:
    type: canonical_memory
    root: /vault/mysecondbrain
    default: true
    read_only_for_workers: true

  escalasoft:
    type: isolated_domain
    root: /domains/escalasoft_brain
    default: false
    read_only_for_general_agents: true
    bridge_files:
      - /vault/mysecondbrain/memory/context/escalasoft-operacao.md
    structured_data:
      historical_sqlite: /data/escalasoft-sqlite/escalasoft_docs.db
    staging:
      - /tmp/drive-escalasoft-docs
    default_agent: escalasoft-agent
`
	path := filepath.Join(t.TempDir(), "domains.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write domains config file: %v", err)
	}
	return path
}

func writeMemoryProposalFile(t *testing.T, dir string, proposal memory.MemoryProposal) string {
	t.Helper()

	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	path := filepath.Join(dir, "memory-proposal.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write memory proposal file: %v", err)
	}
	return path
}

func writeRawJSONFile(t *testing.T, dir string, name string, value any) string {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	data = append(data, '\n')

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write raw json file: %v", err)
	}
	return path
}

func writeMemoryApprovalFile(t *testing.T, dir string, approval memory.MemoryApproval) string {
	t.Helper()

	data, err := approval.JSON()
	if err != nil {
		t.Fatalf("approval.JSON() error = %v", err)
	}
	path := filepath.Join(dir, memory.ApprovalJSONArtifactName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write memory approval file: %v", err)
	}
	return path
}

func readBackupPlanFile(t *testing.T, path string) memory.BackupPlan {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(backup plan) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("backup plan output is invalid JSON: %s", data)
	}
	var plan memory.BackupPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("Unmarshal(backup plan) error = %v", err)
	}
	return plan
}

func writeBackupPlanFile(t *testing.T, dir string, plan memory.BackupPlan) string {
	t.Helper()

	data, err := plan.JSON()
	if err != nil {
		t.Fatalf("plan.JSON() error = %v", err)
	}
	path := filepath.Join(dir, memory.BackupPlanJSONArtifactName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write backup plan file: %v", err)
	}
	return path
}

func readBackupResultFile(t *testing.T, path string) memory.BackupResult {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(backup result) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("backup result output is invalid JSON: %s", data)
	}
	var result memory.BackupResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal(backup result) error = %v", err)
	}
	return result
}

func runBackupPlan(t *testing.T, proposalPath string, approvalPath string, outputPath string, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	return run([]string{
		"memory",
		"proposal",
		"backup-plan",
		"--proposal",
		proposalPath,
		"--approval",
		approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--output",
		outputPath,
	}, stdout, stderr)
}

func runBackupMaterialize(t *testing.T, backupPlanPath string, outputPath string, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	return run([]string{
		"memory",
		"proposal",
		"backup-materialize",
		"--backup-plan",
		backupPlanPath,
		"--output",
		outputPath,
	}, stdout, stderr)
}

func buildCLIBackupPlan(t *testing.T, tempDir string, targetPath string, operation memory.MemoryOperation) memory.BackupPlan {
	t.Helper()
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-materialize-" + string(operation),
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Backup materialize.",
		CreatedAt:  time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: operation, Content: "new content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	plan, err := memory.BuildBackupPlan(proposal, approval, policy, memory.NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	return plan
}

func mustReadCLIFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func readApplyPreflightFile(t *testing.T, path string) memory.ApplyPreflight {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(preflight) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("preflight output is invalid JSON: %s", data)
	}
	var preflight memory.ApplyPreflight
	if err := json.Unmarshal(data, &preflight); err != nil {
		t.Fatalf("Unmarshal(preflight) error = %v", err)
	}
	return preflight
}

func readMemoryApprovalFile(t *testing.T, path string) memory.MemoryApproval {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(approval) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("approval output is invalid JSON: %s", data)
	}
	var approval memory.MemoryApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		t.Fatalf("Unmarshal(approval) error = %v", err)
	}
	return approval
}

func writeApprovalTestProposal(t *testing.T, dir string, id string) string {
	t.Helper()
	return writeMemoryProposalFile(t, dir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: filepath.Join(dir, "target.md"),
		Operation:  memory.OperationAppend,
		Reason:     "Approval test proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 15, 0, 0, time.UTC),
		Patches:    []memory.MemoryPatch{{TargetPath: filepath.Join(dir, "target.md"), Operation: memory.OperationAppend, Content: "content\n"}},
	}))
}

func testCLIPreflightProposal(t *testing.T, dir string, id string) memory.MemoryProposal {
	t.Helper()
	targetPath := filepath.Join(dir, "target.md")
	return memory.NewProposal(memory.NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "CLI preflight proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
}

func loadCLIExamplePolicy(t *testing.T) *memory.MemoryPolicy {
	t.Helper()
	policy, err := memory.LoadPolicyFromFile(filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"))
	if err != nil {
		t.Fatalf("LoadPolicyFromFile() error = %v", err)
	}
	return policy
}

func buildCLIMemoryApproval(t *testing.T, proposal memory.MemoryProposal, policy *memory.MemoryPolicy, decision memory.ApprovalDecision) memory.MemoryApproval {
	t.Helper()
	approval, err := memory.BuildApproval(proposal, policy, memory.NewApprovalOptions{
		ApprovalID: "approval-" + proposal.ProposalID,
		Reviewer:   "Davi",
		Decision:   decision,
		Reason:     "Approval for preflight.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	return approval
}

func runApplyPreflight(t *testing.T, proposalPath string, approvalPath string, outputPath string, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	args := []string{
		"memory",
		"proposal",
		"apply-preflight",
		"--proposal",
		proposalPath,
		"--approval",
		approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}
	return run(args, stdout, stderr)
}
