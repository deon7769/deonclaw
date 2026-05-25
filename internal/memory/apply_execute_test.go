package memory

import (
	"encoding/json"
	"errors"
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
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
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
	assertNoApplyTempFiles(t, fixture.targetPath)
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
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
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
	assertNoApplyTempFiles(t, fixture.targetPath)
}

func TestExecuteApplyUpdateReplacesContentAndResult(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationUpdate, "existing memory\n", "updated memory\n")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "updated memory\n" {
		t.Fatalf("target content = %q, want updated content", got)
	}
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != ApplyResultStatusUpdated {
		t.Fatalf("status = %q, want %q", item.Status, ApplyResultStatusUpdated)
	}
	if item.Operation != OperationUpdate || item.TargetPath != fixture.targetPath {
		t.Fatalf("item = %#v, want update target", item)
	}
	if item.BytesWritten != int64(len("updated memory\n")) {
		t.Fatalf("bytes_written = %d, want %d", item.BytesWritten, len("updated memory\n"))
	}
	if item.SHA256 == "" {
		t.Fatalf("sha256 is empty: %#v", item)
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, fixture.targetPath)
}

func TestExecuteApplyUpdateFailsIfTargetDoesNotExist(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationUpdate, "existing memory\n", "updated memory\n")
	if err := os.Remove(fixture.targetPath); err != nil {
		t.Fatalf("Remove(target) error = %v", err)
	}

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || (!strings.Contains(err.Error(), "cannot be hashed") && !strings.Contains(err.Error(), "does not exist")) {
		t.Fatalf("ExecuteApply() error = %v, want missing target failure", err)
	}
	if _, err := os.Stat(fixture.targetPath); !os.IsNotExist(err) {
		t.Fatalf("target exists after failed update: %v", err)
	}
	assertNoApplyTempFiles(t, fixture.targetPath)
}

func TestExecuteApplyUpdateFailsIfBackupPlanItemWasMissing(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationUpdate, "", "updated memory\n")
	if err := os.MkdirAll(filepath.Dir(fixture.targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(fixture.targetPath, []byte("appeared memory\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "appeared since backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want backup plan missing target failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "appeared memory\n" {
		t.Fatalf("target content = %q, want appeared content preserved", got)
	}
	assertNoApplyTempFiles(t, fixture.targetPath)
}

func TestExecuteApplyUpdateFailsIfTargetChangedSinceBackup(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationUpdate, "existing memory\n", "updated memory\n")
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
	assertNoApplyTempFiles(t, fixture.targetPath)
}

func TestWriteApplyContentAtomicallyHashMismatchLeavesTargetIntactAndRemovesTemp(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("existing memory\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	_, err := writeApplyContentAtomically(targetPath, "existing memory\nappended memory\n", atomicApplyWriteOptions{
		Mode:                  atomicApplyModeReplace,
		ExpectedCurrentSHA256: mustFileSHA256(t, targetPath),
		AfterTempWrite: func(tempPath string) error {
			return os.WriteFile(tempPath, []byte("corrupted temporary content\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "temporary apply file") || !strings.Contains(err.Error(), "content") {
		t.Fatalf("writeApplyContentAtomically() error = %v, want temporary content failure", err)
	}
	if got := string(mustReadFile(t, targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want original target intact", got)
	}
	assertNoApplyTempFiles(t, targetPath)
}

func TestWriteApplyContentAtomicallyTempWriteFailureLeavesTargetIntactAndRemovesTemp(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("existing memory\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	injectedErr := errors.New("injected temporary write failure")

	_, err := writeApplyContentAtomically(targetPath, "existing memory\nappended memory\n", atomicApplyWriteOptions{
		Mode:                  atomicApplyModeReplace,
		ExpectedCurrentSHA256: mustFileSHA256(t, targetPath),
		WriteTemp: func(tempPath string, content string) (int64, error) {
			if err := os.WriteFile(tempPath, []byte("partial temporary content\n"), 0o600); err != nil {
				return 0, err
			}
			return int64(len("partial temporary content\n")), injectedErr
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("writeApplyContentAtomically() error = %v, want injected write failure", err)
	}
	if got := string(mustReadFile(t, targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want original target intact", got)
	}
	assertNoApplyTempFiles(t, targetPath)
}

func TestWriteApplyContentAtomicallyCreateFailsIfTargetAppearsBeforeRename(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")

	_, err := writeApplyContentAtomically(targetPath, "created memory\n", atomicApplyWriteOptions{
		Mode: atomicApplyModeCreate,
		BeforeRename: func() error {
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return err
			}
			return os.WriteFile(targetPath, []byte("concurrent content\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("writeApplyContentAtomically() error = %v, want target exists failure", err)
	}
	if got := string(mustReadFile(t, targetPath)); got != "concurrent content\n" {
		t.Fatalf("target content = %q, want concurrent content preserved", got)
	}
	assertNoApplyTempFiles(t, targetPath)
}

func TestWriteApplyAppendAtomicHashFailureBeforeRenameLeavesTargetIntact(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("existing memory\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	expectedSHA256 := mustFileSHA256(t, targetPath)

	_, err := writeApplyContentAtomically(targetPath, "existing memory\nappended memory\n", atomicApplyWriteOptions{
		Mode:                  atomicApplyModeReplace,
		ExpectedCurrentSHA256: expectedSHA256,
		BeforeRename: func() error {
			return os.WriteFile(targetPath, []byte("changed before rename\n"), 0o600)
		},
	})
	if err == nil || !strings.Contains(err.Error(), "changed before rename") {
		t.Fatalf("writeApplyContentAtomically() error = %v, want target hash failure", err)
	}
	if got := string(mustReadFile(t, targetPath)); got != "changed before rename\n" {
		t.Fatalf("target content = %q, want externally changed target preserved", got)
	}
	assertNoApplyTempFiles(t, targetPath)
}

func TestExecuteApplyMultipleCreatePatchesPass(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "created-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "created-two.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, nil, []MemoryPatch{
		{TargetPath: firstTarget, Operation: OperationCreate, Content: "created one\n"},
		{TargetPath: secondTarget, Operation: OperationCreate, Content: "created two\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if got := string(mustReadFile(t, firstTarget)); got != "created one\n" {
		t.Fatalf("first target = %q, want created content", got)
	}
	if got := string(mustReadFile(t, secondTarget)); got != "created two\n" {
		t.Fatalf("second target = %q, want created content", got)
	}
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	for _, item := range result.Items {
		if item.Status != ApplyResultStatusCreated {
			t.Fatalf("item status = %q, want %q", item.Status, ApplyResultStatusCreated)
		}
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, firstTarget)
	assertNoApplyTempFiles(t, secondTarget)
}

func TestExecuteApplyMultipleAppendPatchesPass(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "append-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "append-two.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		firstTarget:  "first original\n",
		secondTarget: "second original\n",
	}, []MemoryPatch{
		{TargetPath: firstTarget, Operation: OperationAppend, Content: "first appended\n"},
		{TargetPath: secondTarget, Operation: OperationAppend, Content: "second appended\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if got := string(mustReadFile(t, firstTarget)); got != "first original\nfirst appended\n" {
		t.Fatalf("first target = %q, want appended content", got)
	}
	if got := string(mustReadFile(t, secondTarget)); got != "second original\nsecond appended\n" {
		t.Fatalf("second target = %q, want appended content", got)
	}
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	for _, item := range result.Items {
		if item.Status != ApplyResultStatusAppended {
			t.Fatalf("item status = %q, want %q", item.Status, ApplyResultStatusAppended)
		}
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, firstTarget)
	assertNoApplyTempFiles(t, secondTarget)
}

func TestExecuteApplyMultipleUpdateAndCreatePatchesPass(t *testing.T) {
	tempDir := t.TempDir()
	updateTarget := filepath.Join(tempDir, "memory", "updated.md")
	createTarget := filepath.Join(tempDir, "memory", "created.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		updateTarget: "original memory\n",
	}, []MemoryPatch{
		{TargetPath: updateTarget, Operation: OperationUpdate, Content: "updated memory\n"},
		{TargetPath: createTarget, Operation: OperationCreate, Content: "created memory\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if got := string(mustReadFile(t, updateTarget)); got != "updated memory\n" {
		t.Fatalf("updated target = %q, want updated content", got)
	}
	if got := string(mustReadFile(t, createTarget)); got != "created memory\n" {
		t.Fatalf("created target = %q, want created content", got)
	}
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	if result.Items[0].Status != ApplyResultStatusUpdated || result.Items[0].TargetPath != updateTarget {
		t.Fatalf("first item = %#v, want updated target", result.Items[0])
	}
	if result.Items[1].Status != ApplyResultStatusCreated || result.Items[1].TargetPath != createTarget {
		t.Fatalf("second item = %#v, want created target", result.Items[1])
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, updateTarget)
	assertNoApplyTempFiles(t, createTarget)
}

func TestExecuteApplyUpdateFirstTargetUnchangedWhenSecondPreparationFails(t *testing.T) {
	tempDir := t.TempDir()
	updateTarget := filepath.Join(tempDir, "memory", "updated.md")
	createTarget := filepath.Join(tempDir, "memory", "created.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		updateTarget: "original memory\n",
	}, []MemoryPatch{
		{TargetPath: updateTarget, Operation: OperationUpdate, Content: "updated memory\n"},
		{TargetPath: createTarget, Operation: OperationCreate, Content: "created memory\n"},
	})
	injectedErr := errors.New("injected update/create preparation failure")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(createTarget): {
				WriteTemp: func(tempPath string, content string) (int64, error) {
					if err := os.WriteFile(tempPath, []byte("partial created temp\n"), 0o600); err != nil {
						return 0, err
					}
					return int64(len("partial created temp\n")), injectedErr
				},
			},
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ExecuteApply() error = %v, want injected preparation failure", err)
	}
	if result.Status != ApplyResultStatusFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusFailed)
	}
	if len(result.Items) != 0 {
		t.Fatalf("items = %d, want no applied items before rename", len(result.Items))
	}
	if got := string(mustReadFile(t, updateTarget)); got != "original memory\n" {
		t.Fatalf("updated target = %q, want original content after preparation failure", got)
	}
	if _, err := os.Stat(createTarget); !os.IsNotExist(err) {
		t.Fatalf("created target was changed after preparation failure: %v", err)
	}
	assertNoApplyTempFiles(t, updateTarget)
	assertNoApplyTempFiles(t, createTarget)
}

func TestExecuteApplyUpdateFirstTargetUnchangedWhenSecondValidationFails(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "updated-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "updated-two.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		firstTarget:  "first original\n",
		secondTarget: "second original\n",
	}, []MemoryPatch{
		{TargetPath: firstTarget, Operation: OperationUpdate, Content: "first updated\n"},
		{TargetPath: secondTarget, Operation: OperationUpdate, Content: "second updated\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(secondTarget): {
				AfterTempWrite: func(tempPath string) error {
					return os.WriteFile(tempPath, []byte("corrupted second update temp\n"), 0o600)
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "temporary apply file") || !strings.Contains(err.Error(), "content") {
		t.Fatalf("ExecuteApply() error = %v, want second validation failure", err)
	}
	if result.Status != ApplyResultStatusFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusFailed)
	}
	if len(result.Items) != 0 {
		t.Fatalf("items = %d, want no applied items before rename", len(result.Items))
	}
	if got := string(mustReadFile(t, firstTarget)); got != "first original\n" {
		t.Fatalf("first target = %q, want unchanged after validation failure", got)
	}
	if got := string(mustReadFile(t, secondTarget)); got != "second original\n" {
		t.Fatalf("second target = %q, want unchanged after validation failure", got)
	}
	assertNoApplyTempFiles(t, firstTarget)
	assertNoApplyTempFiles(t, secondTarget)
}

func TestExecuteApplyUpdateRenamePartialFailureReportsPartialFailed(t *testing.T) {
	tempDir := t.TempDir()
	updateTarget := filepath.Join(tempDir, "memory", "updated.md")
	createTarget := filepath.Join(tempDir, "memory", "created.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		updateTarget: "original memory\n",
	}, []MemoryPatch{
		{TargetPath: updateTarget, Operation: OperationUpdate, Content: "updated memory\n"},
		{TargetPath: createTarget, Operation: OperationCreate, Content: "created memory\n"},
	})
	injectedErr := errors.New("injected update/create rename failure")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(createTarget): {
				Rename: func(tempPath string, targetPath string) error {
					return injectedErr
				},
			},
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ExecuteApply() error = %v, want injected rename failure", err)
	}
	if result.Status != ApplyResultStatusPartialFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusPartialFailed)
	}
	if got := string(mustReadFile(t, updateTarget)); got != "updated memory\n" {
		t.Fatalf("updated target = %q, want first update applied", got)
	}
	if _, err := os.Stat(createTarget); !os.IsNotExist(err) {
		t.Fatalf("created target was changed after rename failure: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want first update only", len(result.Items))
	}
	if result.Items[0].Status != ApplyResultStatusUpdated || result.Items[0].TargetPath != updateTarget {
		t.Fatalf("applied item = %#v, want updated target", result.Items[0])
	}
	if result.FailedItem == nil || result.FailedItem.TargetPath != createTarget {
		t.Fatalf("failed_item = %#v, want create target", result.FailedItem)
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, updateTarget)
	assertNoApplyTempFiles(t, createTarget)
}

func TestExecuteApplySecondRenameFailureReturnsPartialFailedResult(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "created-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "created-two.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, nil, []MemoryPatch{
		{TargetPath: firstTarget, Operation: OperationCreate, Content: "created one\n"},
		{TargetPath: secondTarget, Operation: OperationCreate, Content: "created two\n"},
	})
	injectedErr := errors.New("injected second rename failure")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(secondTarget): {
				Rename: func(tempPath string, targetPath string) error {
					return injectedErr
				},
			},
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ExecuteApply() error = %v, want injected rename failure", err)
	}
	var applyErr *ApplyExecutionError
	if !errors.As(err, &applyErr) {
		t.Fatalf("ExecuteApply() error = %T, want *ApplyExecutionError", err)
	}
	if result.Status != ApplyResultStatusPartialFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusPartialFailed)
	}
	if applyErr.Result.Status != ApplyResultStatusPartialFailed {
		t.Fatalf("error result status = %q, want %q", applyErr.Result.Status, ApplyResultStatusPartialFailed)
	}
	if got := string(mustReadFile(t, firstTarget)); got != "created one\n" {
		t.Fatalf("first target = %q, want created content after partial failure", got)
	}
	if _, err := os.Stat(secondTarget); !os.IsNotExist(err) {
		t.Fatalf("second target was changed after rename failure: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want only first renamed item", len(result.Items))
	}
	if result.Items[0].TargetPath != firstTarget {
		t.Fatalf("applied item target = %q, want first target %q", result.Items[0].TargetPath, firstTarget)
	}
	if result.FailedItem == nil {
		t.Fatalf("failed_item is nil, want second target")
	}
	if result.FailedItem.TargetPath != secondTarget || result.FailedItem.Operation != OperationCreate {
		t.Fatalf("failed_item = %#v, want second create target", result.FailedItem)
	}
	if !strings.Contains(result.Error, "injected second rename failure") {
		t.Fatalf("result error = %q, want injected error", result.Error)
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, firstTarget)
	assertNoApplyTempFiles(t, secondTarget)
}

func TestExecuteApplySecondPatchPreparationFailureDoesNotChangeFirstTarget(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "created-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "created-two.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, nil, []MemoryPatch{
		{TargetPath: firstTarget, Operation: OperationCreate, Content: "created one\n"},
		{TargetPath: secondTarget, Operation: OperationCreate, Content: "created two\n"},
	})
	injectedErr := errors.New("injected second preparation failure")

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(secondTarget): {
				WriteTemp: func(tempPath string, content string) (int64, error) {
					if err := os.WriteFile(tempPath, []byte("partial second temp\n"), 0o600); err != nil {
						return 0, err
					}
					return int64(len("partial second temp\n")), injectedErr
				},
			},
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ExecuteApply() error = %v, want injected preparation failure", err)
	}
	var applyErr *ApplyExecutionError
	if !errors.As(err, &applyErr) {
		t.Fatalf("ExecuteApply() error = %T, want *ApplyExecutionError", err)
	}
	if applyErr.Result.Status != ApplyResultStatusFailed {
		t.Fatalf("result status = %q, want %q", applyErr.Result.Status, ApplyResultStatusFailed)
	}
	if len(applyErr.Result.Items) != 0 {
		t.Fatalf("items = %d, want no applied items before rename", len(applyErr.Result.Items))
	}
	if _, err := os.Stat(firstTarget); !os.IsNotExist(err) {
		t.Fatalf("first target was changed after second preparation failure: %v", err)
	}
	if _, err := os.Stat(secondTarget); !os.IsNotExist(err) {
		t.Fatalf("second target was changed after preparation failure: %v", err)
	}
	assertNoApplyTempFiles(t, firstTarget)
	assertNoApplyTempFiles(t, secondTarget)
}

func TestExecuteApplySecondPatchValidationFailureDoesNotChangeFirstTarget(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "append-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "append-two.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		firstTarget:  "first original\n",
		secondTarget: "second original\n",
	}, []MemoryPatch{
		{TargetPath: firstTarget, Operation: OperationAppend, Content: "first appended\n"},
		{TargetPath: secondTarget, Operation: OperationAppend, Content: "second appended\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(secondTarget): {
				AfterTempWrite: func(tempPath string) error {
					return os.WriteFile(tempPath, []byte("corrupted second temp\n"), 0o600)
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "temporary apply file") || !strings.Contains(err.Error(), "content") {
		t.Fatalf("ExecuteApply() error = %v, want second validation failure", err)
	}
	var applyErr *ApplyExecutionError
	if !errors.As(err, &applyErr) {
		t.Fatalf("ExecuteApply() error = %T, want *ApplyExecutionError", err)
	}
	if result.Status != ApplyResultStatusFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusFailed)
	}
	if len(result.Items) != 0 {
		t.Fatalf("items = %d, want no applied items before rename", len(result.Items))
	}
	if applyErr.Result.FailedItem == nil || applyErr.Result.FailedItem.TargetPath != secondTarget {
		t.Fatalf("failed_item = %#v, want second target", applyErr.Result.FailedItem)
	}
	if got := string(mustReadFile(t, firstTarget)); got != "first original\n" {
		t.Fatalf("first target = %q, want unchanged after second validation failure", got)
	}
	if got := string(mustReadFile(t, secondTarget)); got != "second original\n" {
		t.Fatalf("second target = %q, want unchanged after validation failure", got)
	}
	assertNoApplyTempFiles(t, firstTarget)
	assertNoApplyTempFiles(t, secondTarget)
}

func TestExecuteApplyArchiveMovesTargetToArchivePathAndResult(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationArchive, "existing memory\n", "")
	archivePath := fixture.proposal.Patches[0].ArchivePath

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if _, err := os.Stat(fixture.targetPath); !os.IsNotExist(err) {
		t.Fatalf("target still exists after archive: %v", err)
	}
	if got := string(mustReadFile(t, archivePath)); got != "existing memory\n" {
		t.Fatalf("archive content = %q, want moved target content", got)
	}
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.TargetPath != fixture.targetPath || item.ArchivePath != archivePath || item.Operation != OperationArchive {
		t.Fatalf("item = %#v, want archive target and archive_path", item)
	}
	if item.Status != ApplyResultStatusArchived {
		t.Fatalf("status = %q, want %q", item.Status, ApplyResultStatusArchived)
	}
	if item.BytesWritten != int64(len("existing memory\n")) {
		t.Fatalf("bytes_written = %d, want moved bytes", item.BytesWritten)
	}
	if item.SHA256 != mustFileSHA256(t, archivePath) {
		t.Fatalf("sha256 = %q, want archive file hash", item.SHA256)
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, fixture.targetPath)
	assertNoApplyTempFiles(t, archivePath)
}

func TestExecuteApplyArchiveFailsWithoutArchivePath(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationArchive, "existing memory\n", "")
	fixture.proposal.Patches[0].ArchivePath = ""

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "apply preflight failed") {
		t.Fatalf("ExecuteApply() error = %v, want preflight failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestExecuteApplyArchiveFailsIfTargetMissing(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationArchive, "existing memory\n", "")
	archivePath := fixture.proposal.Patches[0].ArchivePath
	if err := os.Remove(fixture.targetPath); err != nil {
		t.Fatalf("Remove(target) error = %v", err)
	}

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || (!strings.Contains(err.Error(), "cannot be hashed") && !strings.Contains(err.Error(), "does not exist")) {
		t.Fatalf("ExecuteApply() error = %v, want missing target failure", err)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive path changed after failure: %v", err)
	}
}

func TestExecuteApplyArchiveFailsIfArchivePathExists(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationArchive, "existing memory\n", "")
	archivePath := fixture.proposal.Patches[0].ArchivePath
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(archive dir) error = %v", err)
	}
	if err := os.WriteFile(archivePath, []byte("already archived\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(archive path) error = %v", err)
	}

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "appeared since backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want archive appeared failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
	if got := string(mustReadFile(t, archivePath)); got != "already archived\n" {
		t.Fatalf("archive content = %q, want existing archive preserved", got)
	}
}

func TestExecuteApplyArchiveFailsIfTargetChangedSinceBackup(t *testing.T) {
	fixture := newApplyExecuteFixture(t, OperationArchive, "existing memory\n", "")
	archivePath := fixture.proposal.Patches[0].ArchivePath
	if err := os.WriteFile(fixture.targetPath, []byte("changed after backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}

	_, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err == nil || !strings.Contains(err.Error(), "changed since backup plan") {
		t.Fatalf("ExecuteApply() error = %v, want target changed failure", err)
	}
	if got := string(mustReadFile(t, fixture.targetPath)); got != "changed after backup\n" {
		t.Fatalf("target content = %q, want changed target preserved", got)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive path changed after failure: %v", err)
	}
}

func TestExecuteApplyArchiveAndUpdateMultiPatchPasses(t *testing.T) {
	tempDir := t.TempDir()
	archiveTarget := filepath.Join(tempDir, "memory", "archive-source.md")
	archivePath := filepath.Join(tempDir, "memory", "archive", "archive-source.md")
	updateTarget := filepath.Join(tempDir, "memory", "updated.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		archiveTarget: "archive original\n",
		updateTarget:  "update original\n",
	}, []MemoryPatch{
		{TargetPath: archiveTarget, Operation: OperationArchive, ArchivePath: archivePath},
		{TargetPath: updateTarget, Operation: OperationUpdate, Content: "update replacement\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{})
	if err != nil {
		t.Fatalf("ExecuteApply() error = %v", err)
	}
	if _, err := os.Stat(archiveTarget); !os.IsNotExist(err) {
		t.Fatalf("archive target still exists after apply: %v", err)
	}
	if got := string(mustReadFile(t, archivePath)); got != "archive original\n" {
		t.Fatalf("archive path content = %q, want original", got)
	}
	if got := string(mustReadFile(t, updateTarget)); got != "update replacement\n" {
		t.Fatalf("update target = %q, want replacement", got)
	}
	if result.Status != ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(result.Items))
	}
	if result.Items[0].Status != ApplyResultStatusArchived || result.Items[0].ArchivePath != archivePath {
		t.Fatalf("first item = %#v, want archived source", result.Items[0])
	}
	if result.Items[1].Status != ApplyResultStatusUpdated || result.Items[1].TargetPath != updateTarget {
		t.Fatalf("second item = %#v, want updated target", result.Items[1])
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, updateTarget)
	assertNoApplyTempFiles(t, archivePath)
}

func TestExecuteApplyArchiveFirstTargetUnchangedWhenSecondValidationFails(t *testing.T) {
	tempDir := t.TempDir()
	archiveTarget := filepath.Join(tempDir, "memory", "archive-source.md")
	archivePath := filepath.Join(tempDir, "memory", "archive", "archive-source.md")
	updateTarget := filepath.Join(tempDir, "memory", "updated.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		archiveTarget: "archive original\n",
		updateTarget:  "update original\n",
	}, []MemoryPatch{
		{TargetPath: archiveTarget, Operation: OperationArchive, ArchivePath: archivePath},
		{TargetPath: updateTarget, Operation: OperationUpdate, Content: "update replacement\n"},
	})

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(updateTarget): {
				AfterTempWrite: func(tempPath string) error {
					return os.WriteFile(tempPath, []byte("corrupted update temp\n"), 0o600)
				},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "temporary apply file") || !strings.Contains(err.Error(), "content") {
		t.Fatalf("ExecuteApply() error = %v, want second validation failure", err)
	}
	if result.Status != ApplyResultStatusFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusFailed)
	}
	if got := string(mustReadFile(t, archiveTarget)); got != "archive original\n" {
		t.Fatalf("archive target = %q, want unchanged", got)
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Fatalf("archive path changed after validation failure: %v", err)
	}
	if got := string(mustReadFile(t, updateTarget)); got != "update original\n" {
		t.Fatalf("update target = %q, want unchanged", got)
	}
	assertNoApplyTempFiles(t, updateTarget)
	assertNoApplyTempFiles(t, archivePath)
}

func TestExecuteApplyArchiveRenamePartialFailureReportsPartialFailed(t *testing.T) {
	tempDir := t.TempDir()
	archiveTarget := filepath.Join(tempDir, "memory", "archive-source.md")
	archivePath := filepath.Join(tempDir, "memory", "archive", "archive-source.md")
	updateTarget := filepath.Join(tempDir, "memory", "updated.md")
	fixture := newApplyExecuteMultiPatchFixture(t, tempDir, map[string]string{
		archiveTarget: "archive original\n",
		updateTarget:  "update original\n",
	}, []MemoryPatch{
		{TargetPath: archiveTarget, Operation: OperationArchive, ArchivePath: archivePath},
		{TargetPath: updateTarget, Operation: OperationUpdate, Content: "update replacement\n"},
	})
	injectedErr := errors.New("injected archive/update rename failure")

	result, err := ExecuteApply(fixture.proposal, fixture.approval, fixture.policy, fixture.backupPlan, fixture.backupResult, NewApplyExecuteOptions{
		atomicWriteOptionsByTarget: map[string]atomicApplyWriteOptions{
			applyPathKey(updateTarget): {
				Rename: func(tempPath string, targetPath string) error {
					return injectedErr
				},
			},
		},
	})
	if !errors.Is(err, injectedErr) {
		t.Fatalf("ExecuteApply() error = %v, want injected rename failure", err)
	}
	if result.Status != ApplyResultStatusPartialFailed {
		t.Fatalf("result status = %q, want %q", result.Status, ApplyResultStatusPartialFailed)
	}
	if _, err := os.Stat(archiveTarget); !os.IsNotExist(err) {
		t.Fatalf("archive target still exists after first rename: %v", err)
	}
	if got := string(mustReadFile(t, archivePath)); got != "archive original\n" {
		t.Fatalf("archive path = %q, want moved content", got)
	}
	if got := string(mustReadFile(t, updateTarget)); got != "update original\n" {
		t.Fatalf("update target = %q, want unchanged after partial failure", got)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want first archived item only", len(result.Items))
	}
	if result.Items[0].Status != ApplyResultStatusArchived || result.Items[0].ArchivePath != archivePath {
		t.Fatalf("applied item = %#v, want archived source", result.Items[0])
	}
	if result.FailedItem == nil || result.FailedItem.TargetPath != updateTarget || result.FailedItem.ArchivePath != "" {
		t.Fatalf("failed_item = %#v, want update target", result.FailedItem)
	}
	assertApplyResultJSONValid(t, result)
	assertNoApplyTempFiles(t, updateTarget)
	assertNoApplyTempFiles(t, archivePath)
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

	patch := MemoryPatch{TargetPath: targetPath, Operation: operation, Content: patchContent}
	if operation == OperationArchive {
		patch.ArchivePath = filepath.Join(tempDir, "memory", "archive", "target.md")
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
			patch,
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

func newApplyExecuteMultiPatchFixture(t *testing.T, tempDir string, initialContents map[string]string, patches []MemoryPatch) applyExecuteFixture {
	t.Helper()

	for targetPath, content := range initialContents {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(targetPath), err)
		}
		if err := os.WriteFile(targetPath, []byte(content), 0o600); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", targetPath, err)
		}
	}

	proposal := NewProposal(NewProposalOptions{
		ProposalID: "mem-apply-execute-multi",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: patches[0].TargetPath,
		Operation:  patches[0].Operation,
		Reason:     "Execute multi-patch apply proposal.",
		CreatedAt:  time.Date(2026, 5, 24, 13, 0, 0, 0, time.UTC),
		Patches:    patches,
	})
	policy := loadExamplePolicy(t)
	approval := mustBuildBackupPlanApproval(t, proposal, policy)
	plan, err := BuildBackupPlan(proposal, approval, policy, NewBackupPlanOptions{
		BackupRoot: filepath.Join(tempDir, "backups"),
		CreatedAt:  time.Date(2026, 5, 24, 13, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	result, err := MaterializeBackup(plan, NewBackupMaterializeOptions{
		CreatedAt: time.Date(2026, 5, 24, 13, 10, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("MaterializeBackup() error = %v", err)
	}

	return applyExecuteFixture{
		targetPath:   patches[0].TargetPath,
		proposal:     proposal,
		approval:     approval,
		policy:       policy,
		backupPlan:   plan,
		backupResult: result,
	}
}

func assertApplyResultJSONValid(t *testing.T, result ApplyResult) {
	t.Helper()

	data, err := result.JSON()
	if err != nil {
		t.Fatalf("result.JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("apply result JSON invalid: %s", data)
	}
}

func mustFileSHA256(t *testing.T, path string) string {
	t.Helper()

	sha256, err := fileSHA256(path)
	if err != nil {
		t.Fatalf("fileSHA256(%q) error = %v", path, err)
	}
	return sha256
}

func assertNoApplyTempFiles(t *testing.T, targetPath string) {
	t.Helper()

	pattern := filepath.Join(filepath.Dir(targetPath), "."+filepath.Base(targetPath)+".apply-*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("Glob(%q) error = %v", pattern, err)
	}
	if len(matches) > 0 {
		t.Fatalf("apply temp files left behind: %v", matches)
	}
}
