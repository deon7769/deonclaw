package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildBackupPlanExistingTargetRecordsHashAndSize(t *testing.T) {
	for _, operation := range []MemoryOperation{OperationAppend, OperationUpdate} {
		t.Run(string(operation), func(t *testing.T) {
			tempDir := t.TempDir()
			targetPath := filepath.Join(tempDir, "target.md")
			content := []byte("existing content\n")
			if err := os.WriteFile(targetPath, content, 0o600); err != nil {
				t.Fatalf("WriteFile(target) error = %v", err)
			}
			proposal := testBackupPlanProposal("mem-backup-existing-"+string(operation), []MemoryPatch{
				{TargetPath: targetPath, Operation: operation, Content: "new content\n"},
			})
			policy := loadExamplePolicy(t)
			approval := mustBuildBackupPlanApproval(t, proposal, policy)

			plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{
				BackupRoot: filepath.Join(tempDir, "backups"),
				CreatedAt:  time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
			})
			if err != nil {
				t.Fatalf("BuildBackupPlan() error = %v", err)
			}
			if len(plan.Items) != 1 {
				t.Fatalf("items = %d, want 1", len(plan.Items))
			}
			item := plan.Items[0]
			if !item.Exists {
				t.Fatalf("exists = false, want true")
			}
			if item.Operation != operation {
				t.Fatalf("operation = %q, want %q", item.Operation, operation)
			}
			wantHashBytes := sha256.Sum256(content)
			wantHash := hex.EncodeToString(wantHashBytes[:])
			if item.SHA256 == nil || *item.SHA256 != wantHash {
				t.Fatalf("sha256 = %v, want %s", item.SHA256, wantHash)
			}
			if item.SizeBytes == nil || *item.SizeBytes != int64(len(content)) {
				t.Fatalf("size_bytes = %v, want %d", item.SizeBytes, len(content))
			}
			if !strings.HasPrefix(item.BackupPath, filepath.Join(tempDir, "backups")) {
				t.Fatalf("backup_path = %q, want under backup root", item.BackupPath)
			}
			if _, err := os.Stat(item.BackupPath); !os.IsNotExist(err) {
				t.Fatalf("backup path was written unexpectedly: %v", err)
			}
			if string(mustReadFile(t, targetPath)) != string(content) {
				t.Fatalf("target content changed")
			}
		})
	}
}

func TestBuildBackupPlanMissingTargetRecordsNotExists(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "missing.md")
	proposal := testBackupPlanProposal("mem-backup-missing", []MemoryPatch{
		{TargetPath: targetPath, Operation: OperationCreate, Content: "new file\n"},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)

	plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	item := plan.Items[0]
	if item.Exists {
		t.Fatalf("exists = true, want false")
	}
	if item.SHA256 != nil {
		t.Fatalf("sha256 = %v, want nil", *item.SHA256)
	}
	if item.SizeBytes != nil {
		t.Fatalf("size_bytes = %v, want nil", *item.SizeBytes)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestBuildBackupPlanMultiplePatchesGenerateMultipleItems(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "first.md")
	second := filepath.Join(tempDir, "second.md")
	proposal := testBackupPlanProposal("mem-backup-multiple", []MemoryPatch{
		{TargetPath: first, Operation: OperationAppend, Content: "first\n"},
		{TargetPath: second, Operation: OperationCreate, Content: "second\n"},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)

	plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	if len(plan.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(plan.Items))
	}
	if len(plan.RestorePlan.Items) != 2 {
		t.Fatalf("restore items = %d, want 2", len(plan.RestorePlan.Items))
	}
}

func TestBuildBackupPlanDeduplicatesRepeatedTargets(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "same.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	proposal := testBackupPlanProposal("mem-backup-dedupe", []MemoryPatch{
		{TargetPath: targetPath, Operation: OperationAppend, Content: "first\n"},
		{TargetPath: targetPath, Operation: OperationAppend, Content: "second\n"},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)

	plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("items = %d, want 1 deduplicated target", len(plan.Items))
	}
	if len(plan.RestorePlan.Items) != 1 {
		t.Fatalf("restore items = %d, want 1 deduplicated target", len(plan.RestorePlan.Items))
	}
	if got := string(mustReadFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestBuildBackupPlanPreflightFailureBlocksPlan(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := testBackupPlanProposal("mem-backup-preflight-failed", []MemoryPatch{
		{TargetPath: targetPath, Operation: OperationAppend, Content: "content\n"},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)
	approval.Decision = DecisionRejected

	_, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err == nil || !strings.Contains(err.Error(), "apply preflight failed") {
		t.Fatalf("BuildBackupPlan() error = %v, want preflight failure", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestBuildBackupPlanRejectsBackupRootTraversal(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := testBackupPlanProposal("mem-backup-root-traversal", []MemoryPatch{
		{TargetPath: targetPath, Operation: OperationAppend, Content: "content\n"},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)

	_, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{BackupRoot: tempDir + string(filepath.Separator) + ".." + string(filepath.Separator) + "escape"})
	if err == nil || !strings.Contains(err.Error(), "path traversal") {
		t.Fatalf("BuildBackupPlan() error = %v, want traversal failure", err)
	}
}

func TestBuildBackupPlanJSONValid(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := testBackupPlanProposal("mem-backup-json", []MemoryPatch{
		{TargetPath: targetPath, Operation: OperationAppend, Content: "content\n"},
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)
	plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}

	data, err := plan.JSON()
	if err != nil {
		t.Fatalf("plan.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("backup plan JSON invalid: %s", data)
	}
	output := string(data)
	for _, want := range []string{"\"items\"", "\"restore_plan\"", "\"backup_path\"", "\"target_path\""} {
		if !strings.Contains(output, want) {
			t.Fatalf("backup plan JSON = %s, want %s", output, want)
		}
	}
}

func testBackupPlanProposal(id string, patches []MemoryPatch) MemoryProposal {
	targetPath := "/vault/mysecondbrain/memory/inbox/run-001.md"
	if len(patches) > 0 && strings.TrimSpace(patches[0].TargetPath) != "" {
		targetPath = patches[0].TargetPath
	}
	return NewProposal(NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  OperationAppend,
		Reason:     "Backup plan proposal.",
		CreatedAt:  time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC),
		Patches:    patches,
	})
}

func mustBuildBackupPlanApproval(t *testing.T, proposal MemoryProposal, policy *MemoryPolicy) MemoryApproval {
	t.Helper()
	approval, err := BuildApproval(proposal, policy, NewApprovalOptions{
		ApprovalID: "approval-" + proposal.ProposalID,
		Reviewer:   "Davi",
		Decision:   DecisionApproved,
		Reason:     "Approved for backup plan.",
		CreatedAt:  time.Date(2026, 5, 23, 12, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	return approval
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}
