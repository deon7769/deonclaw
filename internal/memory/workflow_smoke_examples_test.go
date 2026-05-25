package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryWorkflowSmokeExamplesRoundTrip(t *testing.T) {
	examples := []struct {
		name              string
		file              string
		initialTarget     string
		wantAfterApply    string
		wantAfterRestore  string
		expectTargetAfter bool
	}{
		{
			name:             "create",
			file:             "memory-proposal-create.json",
			wantAfterApply:   "Created smoke memory.\n",
			wantAfterRestore: "",
		},
		{
			name:              "append",
			file:              "memory-proposal-append.json",
			initialTarget:     "Existing smoke memory.\n",
			wantAfterApply:    "Existing smoke memory.\nAppended smoke memory.\n",
			wantAfterRestore:  "Existing smoke memory.\n",
			expectTargetAfter: true,
		},
		{
			name:              "update",
			file:              "memory-proposal-update.json",
			initialTarget:     "Old smoke memory.\n",
			wantAfterApply:    "Updated smoke memory.\n",
			wantAfterRestore:  "Old smoke memory.\n",
			expectTargetAfter: true,
		},
		{
			name:              "archive",
			file:              "memory-proposal-archive.json",
			initialTarget:     "Archive smoke memory.\n",
			wantAfterApply:    "Archive smoke memory.\n",
			wantAfterRestore:  "Archive smoke memory.\n",
			expectTargetAfter: true,
		},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			tempDir := t.TempDir()
			proposal := loadSmokeExampleProposal(t, example.file, tempDir)
			targetPath := proposal.Patches[0].TargetPath
			archivePath := proposal.Patches[0].ArchivePath
			if example.initialTarget != "" {
				writeTestFile(t, targetPath, example.initialTarget)
			}

			policy := loadExamplePolicy(t)
			if lint := LintProposal(proposal, policy); lint.Status != LintStatusOK {
				t.Fatalf("LintProposal() status = %q violations = %v", lint.Status, lint.Violations)
			}
			preview, err := BuildApplyDryRunPreview(proposal, policy)
			if err != nil {
				t.Fatalf("BuildApplyDryRunPreview() error = %v", err)
			}
			if preview.Status != ApplyStatusDryRunOK {
				t.Fatalf("preview status = %q, want %q", preview.Status, ApplyStatusDryRunOK)
			}
			approval := mustBuildBackupPlanApproval(t, proposal, policy)
			preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
			if preflight.Status != ApplyPreflightStatusOK {
				t.Fatalf("preflight status = %q failures = %v", preflight.Status, preflight.Failures)
			}
			backupPlan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{
				BackupRoot: filepath.Join(tempDir, "artifacts", "backups"),
			})
			if err != nil {
				t.Fatalf("BuildBackupPlan() error = %v", err)
			}
			backupResult, err := MaterializeBackup(backupPlan, NewBackupMaterializeOptions{})
			if err != nil {
				t.Fatalf("MaterializeBackup() error = %v", err)
			}

			applyResult, err := ExecuteApply(proposal, approval, policy, backupPlan, backupResult, NewApplyExecuteOptions{})
			if err != nil {
				t.Fatalf("ExecuteApply() error = %v", err)
			}
			if applyResult.Status != ApplyResultStatusSucceeded {
				t.Fatalf("apply status = %q, want %q", applyResult.Status, ApplyResultStatusSucceeded)
			}
			if proposal.Operation == OperationArchive {
				assertPathMissing(t, targetPath)
				if got := string(mustReadFile(t, archivePath)); got != example.wantAfterApply {
					t.Fatalf("archive content = %q, want %q", got, example.wantAfterApply)
				}
			} else if got := string(mustReadFile(t, targetPath)); got != example.wantAfterApply {
				t.Fatalf("target content = %q, want %q", got, example.wantAfterApply)
			}

			restorePreview, err := BuildRestorePreview(backupPlan, backupResult, NewRestorePreviewOptions{})
			if err != nil {
				t.Fatalf("BuildRestorePreview() error = %v", err)
			}
			restoreResult, err := ExecuteRestore(backupPlan, backupResult, restorePreview, NewRestoreExecuteOptions{})
			if err != nil {
				t.Fatalf("ExecuteRestore() error = %v", err)
			}
			if len(restoreResult.Items) == 0 {
				t.Fatalf("restore result has no items")
			}

			if example.expectTargetAfter {
				if got := string(mustReadFile(t, targetPath)); got != example.wantAfterRestore {
					t.Fatalf("restored target content = %q, want %q", got, example.wantAfterRestore)
				}
			} else {
				assertPathMissing(t, targetPath)
			}
			if archivePath != "" {
				assertPathMissing(t, archivePath)
			}
		})
	}
}

func loadSmokeExampleProposal(t *testing.T, file string, root string) MemoryProposal {
	t.Helper()

	path := filepath.Join("..", "..", "examples", "memory", file)
	proposal, err := LoadProposalFromFile(path)
	if err != nil {
		t.Fatalf("LoadProposalFromFile(%q) error = %v", path, err)
	}
	replace := func(value string) string {
		return strings.ReplaceAll(value, "{{SMOKE_ROOT}}", filepath.ToSlash(root))
	}
	proposal.TargetPath = replace(proposal.TargetPath)
	for index := range proposal.Patches {
		proposal.Patches[index].TargetPath = replace(proposal.Patches[index].TargetPath)
		proposal.Patches[index].ArchivePath = replace(proposal.Patches[index].ArchivePath)
	}
	return proposal
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assertPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("path %q exists, stat error = %v", path, err)
	}
}
