package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecuteRestoreExistingTargetRestoresFile(t *testing.T) {
	fixture := newRestoreExecuteFixture(t, OperationAppend, "original memory\n")
	if err := os.WriteFile(fixture.targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}

	result, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, fixture.restorePreview, NewRestoreExecuteOptions{
		CreatedAt: time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ExecuteRestore() error = %v", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want restored original", got)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != RestoreResultStatusRestored {
		t.Fatalf("status = %q, want %q", item.Status, RestoreResultStatusRestored)
	}
	if item.TargetPath != fixture.targetPath || item.BackupPath != fixture.backupPlan.Items[0].BackupPath {
		t.Fatalf("item = %#v, want target and backup paths", item)
	}
	if item.BytesRestored != int64(len("original memory\n")) {
		t.Fatalf("bytes_restored = %d, want %d", item.BytesRestored, len("original memory\n"))
	}
	if item.SHA256 == nil || item.BackupSHA256 == nil || *item.SHA256 != *item.BackupSHA256 {
		t.Fatalf("hashes = sha256 %v backup_sha256 %v, want matching hashes", item.SHA256, item.BackupSHA256)
	}
	assertNoRestoreTempFiles(t, fixture.targetPath)
}

func TestExecuteRestoreMissingOriginalTargetRemovesTarget(t *testing.T) {
	fixture := newRestoreExecuteFixture(t, OperationCreate, "")
	if err := os.MkdirAll(filepath.Dir(fixture.targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(fixture.targetPath, []byte("created by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(created target) error = %v", err)
	}

	result, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, fixture.restorePreview, NewRestoreExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteRestore() error = %v", err)
	}
	if _, err := os.Stat(fixture.targetPath); !os.IsNotExist(err) {
		t.Fatalf("target exists after restore removal: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	if got := result.Items[0].Status; got != RestoreResultStatusRemoved {
		t.Fatalf("status = %q, want %q", got, RestoreResultStatusRemoved)
	}
}

func TestExecuteRestoreUndoesArchiveApply(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationArchive, "original memory\n", "")
	archivePath := fixture.proposal.Patches[0].ArchivePath
	if _, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{}); err != nil {
		t.Fatalf("ExecuteApply(archive) error = %v", err)
	}
	if _, err := os.Stat(fixture.targetPath); !os.IsNotExist(err) {
		t.Fatalf("target exists after archive apply: %v", err)
	}
	if got := string(mustReadFile(t, archivePath)); got != "original memory\n" {
		t.Fatalf("archive path content = %q, want moved original", got)
	}
	preview, err := BuildRestorePreview(fixture.backupPlan, fixture.backupResult, NewRestorePreviewOptions{})
	if err != nil {
		t.Fatalf("BuildRestorePreview() error = %v", err)
	}

	result, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, preview, NewRestoreExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteRestore() error = %v", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want restored original", got)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive path exists after restore: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want target restore and archive removal", len(result.Items))
	}
	statuses := map[string]RestoreResultItemStatus{}
	for _, item := range result.Items {
		statuses[item.TargetPath] = item.Status
	}
	if statuses[fixture.targetPath] != RestoreResultStatusRestored {
		t.Fatalf("target restore status = %q, want %q", statuses[fixture.targetPath], RestoreResultStatusRestored)
	}
	if statuses[archivePath] != RestoreResultStatusRemoved {
		t.Fatalf("archive restore status = %q, want %q", statuses[archivePath], RestoreResultStatusRemoved)
	}
}

func TestExecuteRestoreMissingOriginalTargetSkippedWhenAbsent(t *testing.T) {
	fixture := newRestoreExecuteFixture(t, OperationCreate, "")

	result, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, fixture.restorePreview, NewRestoreExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteRestore() error = %v", err)
	}
	if _, err := os.Stat(fixture.targetPath); !os.IsNotExist(err) {
		t.Fatalf("target exists after skipped restore: %v", err)
	}
	if got := result.Items[0].Status; got != RestoreResultStatusSkippedMissing {
		t.Fatalf("status = %q, want %q", got, RestoreResultStatusSkippedMissing)
	}
}

func TestExecuteRestoreCorruptedBackupFailsBeforeWriting(t *testing.T) {
	fixture := newRestoreExecuteFixture(t, OperationAppend, "original memory\n")
	if err := os.WriteFile(fixture.targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	if err := os.WriteFile(fixture.backupPlan.Items[0].BackupPath, []byte("corrupted backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupted backup) error = %v", err)
	}

	_, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, fixture.restorePreview, NewRestoreExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "backup_path") || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("ExecuteRestore() error = %v, want backup hash failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged after restore failure", got)
	}
}

func TestCopyRestoreFileHashMismatchLeavesTargetIntactAndRemovesTemp(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	backupPath := filepath.Join(tempDir, "backups", "target.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(backup dir) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("target before failed restore\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	if err := os.WriteFile(backupPath, []byte("backup content\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(backup) error = %v", err)
	}

	_, err := copyRestoreFile(backupPath, targetPath, strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "temporary restore") || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("copyRestoreFile() error = %v, want temporary sha256 failure", err)
	}
	if got := string(mustReadFile(t, targetPath)); got != "target before failed restore\n" {
		t.Fatalf("target content = %q, want original target intact", got)
	}
	assertNoRestoreTempFiles(t, targetPath)
}

func TestCopyRestoreFileCopyFailureLeavesTargetIntactAndRemovesTemp(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	backupPath := filepath.Join(tempDir, "backup-as-directory")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.MkdirAll(backupPath, 0o755); err != nil {
		t.Fatalf("MkdirAll(backup directory) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("target before failed copy\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	_, err := copyRestoreFile(backupPath, targetPath, strings.Repeat("0", 64))
	if err == nil {
		t.Fatalf("copyRestoreFile() error = nil, want copy failure")
	}
	if got := string(mustReadFile(t, targetPath)); got != "target before failed copy\n" {
		t.Fatalf("target content = %q, want original target intact", got)
	}
	assertNoRestoreTempFiles(t, targetPath)
}

func TestExecuteRestorePreviewDivergenceFails(t *testing.T) {
	fixture := newRestoreExecuteFixture(t, OperationAppend, "original memory\n")
	if err := os.WriteFile(fixture.targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	fixture.restorePreview.Items[0].Action = RestorePreviewActionRemoveIfExists

	_, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, fixture.restorePreview, NewRestoreExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "restore-preview") || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("ExecuteRestore() error = %v, want restore-preview divergence", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged after preview divergence", got)
	}
}

func TestExecuteRestoreResultJSONValid(t *testing.T) {
	fixture := newRestoreExecuteFixture(t, OperationAppend, "original memory\n")
	if err := os.WriteFile(fixture.targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}

	result, err := ExecuteRestore(fixture.backupPlan, fixture.backupResult, fixture.restorePreview, NewRestoreExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteRestore() error = %v", err)
	}
	data, err := result.JSON()
	if err != nil {
		t.Fatalf("result.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("restore result JSON invalid: %s", data)
	}
	output := string(data)
	for _, want := range []string{"\"items\"", "\"restored\"", "\"target_path\"", "\"backup_path\""} {
		if !strings.Contains(output, want) {
			t.Fatalf("restore result JSON = %s, want %s", output, want)
		}
	}
}

type restoreExecuteFixture struct {
	targetPath     string
	backupPlan     BackupPlan
	backupResult   BackupResult
	restorePreview RestorePreview
}

func newRestoreExecuteFixture(t *testing.T, operation MemoryOperation, initialContent string) restoreExecuteFixture {
	t.Helper()

	base := newRestoreFixture(t, operation, initialContent)
	preview, err := BuildRestorePreview(base.backupPlan, base.backupResult, NewRestorePreviewOptions{
		CreatedAt: time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRestorePreview() error = %v", err)
	}
	return restoreExecuteFixture{
		targetPath:     base.targetPath,
		backupPlan:     base.backupPlan,
		backupResult:   base.backupResult,
		restorePreview: preview,
	}
}

func assertNoRestoreTempFiles(t *testing.T, targetPath string) {
	t.Helper()

	pattern := filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".restore-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Glob(%q) error = %v", pattern, err)
	}
	if len(matches) > 0 {
		t.Fatalf("restore temp files left behind: %v", matches)
	}
}
