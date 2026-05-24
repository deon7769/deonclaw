package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteApplyCreateCreatesFileAndResult(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationCreate, "", "created memory\n")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		CreatedAt: time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "created memory\n" {
		t.Fatalf("target content = %q, want created content", got)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != ApplyResultStatusCreated {
		t.Fatalf("status = %q, want %q", item.Status, ApplyResultStatusCreated)
	}
	if item.Operation != OperationCreate || item.TargetPath != fixture.targetPath {
		t.Fatalf("item = %#v, want create target", item)
	}
	if item.BytesWritten != int64(len("created memory\n")) {
		t.Fatalf("bytes_written = %d, want %d", item.BytesWritten, len("created memory\n"))
	}
	if item.SHA256 == "" {
		t.Fatalf("sha256 is empty: %#v", item)
	}
	data, err := result.JSON()
	if err != nil {
		t.Fatalf("result.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("apply result JSON invalid: %s", data)
	}
}

func TestExecuteApplyAppendAppendsContentAndResult(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\nappended memory\n" {
		t.Fatalf("target content = %q, want appended content", got)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != ApplyResultStatusAppended {
		t.Fatalf("status = %q, want %q", item.Status, ApplyResultStatusAppended)
	}
	if item.BytesWritten != int64(len("appended memory\n")) {
		t.Fatalf("bytes_written = %d, want %d", item.BytesWritten, len("appended memory\n"))
	}
	if item.SHA256 == "" {
		t.Fatalf("sha256 is empty: %#v", item)
	}
}

func TestExecuteApplyRejectsUnimplementedOperations(t *testing.T) {
	for _, operation := range []MemoryOperation{OperationUpdate, OperationArchive} {
		t.Run(string(operation), func(t *testing.T) {
			fixture := newApplyExecuteFixture(t, operation, "existing memory\n", "changed memory\n")

			_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
			if err == nil || !strings.Contains(err.Error(), "operation not implemented") {
				t.Fatalf("ExecuteApply() error = %v, want operation not implemented", err)
			}
			if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
				t.Fatalf("target content = %q, want unchanged", got)
			}
		})
	}
}

func TestExecuteApplyTargetChangedAfterBackupResultFails(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	if err := os.WriteFile(fixture.targetPath, []byte("changed after backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "changed since backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want changed target failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "changed after backup\n" {
		t.Fatalf("target content = %q, want changed content preserved", got)
	}
}

func TestExecuteApplyIncompleteBackupResultFails(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	fixture.backupResult.Items = nil

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup result missing item") {
		t.Fatalf("ExecuteApply() error = %v, want missing backup result item", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestExecuteApplyExistingBackupNotCopiedFails(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	fixture.backupResult.Items[0].Status = BackupResultStatusSkippedMissing
	fixture.backupResult.Items[0].BackupSHA256 = nil

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup was not copied") {
		t.Fatalf("ExecuteApply() error = %v, want copied backup failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestExecuteApplyCorruptedBackupFileMatchingTamperedResultHashFails(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	backupPath := fixture.backupPlan.Items[0].BackupPath
	if err := os.WriteFile(backupPath, []byte("corrupted backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupted backup) error = %v", err)
	}
	corruptedSHA256, err := fileSHA256(backupPath)
	if err != nil {
		t.Fatalf("fileSHA256(corrupted backup) error = %v", err)
	}
	fixture.backupResult.Items[0].BackupSHA256 = &corruptedSHA256

	_, err = ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup_sha256 does not match backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want backup hash mismatch", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestExecuteApplyTamperedBackupResultBackupSHA256Fails(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	tamperedSHA256 := strings.Repeat("0", 64)
	fixture.backupResult.Items[0].BackupSHA256 = &tamperedSHA256

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup_sha256 does not match backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want backup result backup_sha256 mismatch", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestExecuteApplyBackupResultSHA256DifferentFromPlanFails(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	tamperedSHA256 := strings.Repeat("1", 64)
	fixture.backupResult.Items[0].SHA256 = &tamperedSHA256

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "sha256 does not match backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want backup result sha256 mismatch", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestExecuteApplyPreflightFailureBlocksApply(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationAppend, "existing memory\n", "appended memory\n")
	fixture.approval.Decision = DecisionRejected

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "apply preflight failed") {
		t.Fatalf("ExecuteApply() error = %v, want preflight failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

type applyExecuteFixture struct {
	targetPath   string
	proposal     MemoryProposal
	approval     MemoryApproval
	policy       *MemoryPolicy
	backupPlan   BackupPlan
	backupResult BackupResult
}

func newApplyExecuteFixture(t *testing.T, operation MemoryOperation, initialContent string, patchContent string) applyExecuteFixture {
	t.Helper()

	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	if initialContent != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(target dir) error = %v", err)
		}
		if err := os.WriteFile(targetPath, []byte(initialContent), 0o600); err != nil {
			t.Fatalf("WriteFile(target) error = %v", err)
		}
	}

	proposal := NewProposal(NewProposalOptions{
		ProposalID: "mem-apply-execute-" + string(operation),
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Execute apply proposal.",
		CreatedAt:  time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC),
		Patches: []MemoryPatch{
			{TargetPath: targetPath, Operation: operation, Content: patchContent},
		},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)
	plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{
		BackupRoot: filepath.Join(tempDir, "backups"),
		CreatedAt:  time.Date(2026, 5, 24, 11, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	result, err := MaterializeBackup(plan, NewBackupMaterializeOptions{
		CreatedAt: time.Date(2026, 5, 24, 11, 10, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("MaterializeBackup() error = %v", err)
	}

	return applyExecuteFixture{
		targetPath:   targetPath,
		proposal:     proposal,
		approval:     approval,
		policy:       policy,
		backupPlan:   plan,
		backupResult: result,
	}
}
