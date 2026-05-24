package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildRestorePreviewExistingTargetPasses(t *testing.T) {
	fixture := newRestoreFixture(t, OperationAppend, "original memory\n")
	if err := os.WriteFile(fixture.targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}

	preview, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{
		CreatedAt: time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRestorePreview() error = %v", err)
	}
	if preview.Status != RestorePreviewStatusDryRunOK {
		t.Fatalf("status = %q, want %q", preview.Status, RestorePreviewStatusDryRunOK)
	}
	if len(preview.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(preview.Items))
	}
	item := preview.Items[0]
	if item.Action != RestorePreviewActionRestoreFromBackup {
		t.Fatalf("action = %q, want %q", item.Action, RestorePreviewActionRestoreFromBackup)
	}
	if item.TargetPath != fixture.targetPath || item.BackupPath != fixture.backupPlan.Items[0].BackupPath {
		t.Fatalf("item = %#v, want target and backup paths", item)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged dry-run target", got)
	}
}

func TestBuildRestorePreviewMissingOriginalTargetShowsRemoveIfExists(t *testing.T) {
	fixture := newRestoreFixture(t, OperationCreate, "")
	if err := os.MkdirAll(filepath.Dir(fixture.targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(fixture.targetPath, []byte("created by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(created target) error = %v", err)
	}

	preview, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{})
	if err != nil {
		t.Fatalf("BuildRestorePreview() error = %v", err)
	}
	if len(preview.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(preview.Items))
	}
	item := preview.Items[0]
	if item.Action != RestorePreviewActionRemoveIfExists {
		t.Fatalf("action = %q, want %q", item.Action, RestorePreviewActionRemoveIfExists)
	}
	if item.Exists {
		t.Fatalf("exists = true, want false")
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "created by apply\n" {
		t.Fatalf("target content = %q, want unchanged dry-run target", got)
	}
}

func TestBuildRestorePreviewCorruptedBackupPathFails(t *testing.T) {
	fixture := newRestoreFixture(t, OperationAppend, "original memory\n")
	if err := os.WriteFile(fixture.backupPlan.Items[0].BackupPath, []byte("corrupted backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupted backup) error = %v", err)
	}

	_, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup_path") || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("BuildRestorePreview() error = %v, want backup hash failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestBuildRestorePreviewBackupResultMismatchFails(t *testing.T) {
	fixture := newRestoreFixture(t, OperationAppend, "original memory\n")
	fixture.backupResult.ProposalID = "other-proposal"

	_, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup result proposal_id") {
		t.Fatalf("BuildRestorePreview() error = %v, want backup result mismatch", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestBuildRestorePreviewMissingRestorePlanFails(t *testing.T) {
	fixture := newRestoreFixture(t, OperationAppend, "original memory\n")
	fixture.backupPlan.RestorePlan = RestorePlan{}

	_, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{})
	if err == nil || !strings.Contains(err.Error(), "restore_plan") {
		t.Fatalf("BuildRestorePreview() error = %v, want restore_plan failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestBuildRestorePreviewJSONValid(t *testing.T) {
	fixture := newRestoreFixture(t, OperationAppend, "original memory\n")
	preview, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{})
	if err != nil {
		t.Fatalf("BuildRestorePreview() error = %v", err)
	}

	data, err := preview.JSON()
	if err != nil {
		t.Fatalf("preview.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("restore preview JSON invalid: %s", data)
	}
	output := string(data)
	for _, want := range []string{"\"items\"", "\"restore_from_backup\"", "\"target_path\"", "\"backup_path\""} {
		if !strings.Contains(output, want) {
			t.Fatalf("restore preview JSON = %s, want %s", output, want)
		}
	}
}

type restoreFixture struct {
	targetPath   string
	backupPlan   BackupPlan
	backupResult BackupResult
}

func newRestoreFixture(t *testing.T, operation MemoryOperation, initialContent string) restoreFixture {
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
	plan := mustBuildBackupPlanForMaterialize(t, tempDir, targetPath, operation)
	result, err := MaterializeBackup(plan, NewBackupMaterializeOptions{})
	if err != nil {
		t.Fatalf("MaterializeBackup() error = %v", err)
	}
	return restoreFixture{
		targetPath:   targetPath,
		backupPlan:   plan,
		backupResult: result,
	}
}
