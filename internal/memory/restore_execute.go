package memory

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const RestoreResultJSONArtifactName = "restore-result.json"

type RestoreResultItemStatus string

const (
	RestoreResultStatusRestored       RestoreResultItemStatus = "restored"
	RestoreResultStatusRemoved        RestoreResultItemStatus = "removed"
	RestoreResultStatusSkippedMissing RestoreResultItemStatus = "skipped_missing"
)

type RestoreResult struct {
	ProposalID string              `json:"proposal_id"`
	ApprovalID string              `json:"approval_id"`
	RunID      string              `json:"run_id"`
	TaskID     string              `json:"task_id"`
	Domain     string              `json:"domain"`
	BackupRoot string              `json:"backup_root"`
	Items      []RestoreResultItem `json:"items"`
	CreatedAt  time.Time           `json:"created_at"`
}

type RestoreResultItem struct {
	TargetPath    string                  `json:"target_path"`
	BackupPath    string                  `json:"backup_path,omitempty"`
	Exists        bool                    `json:"exists"`
	Operation     MemoryOperation         `json:"operation"`
	Status        RestoreResultItemStatus `json:"status"`
	BytesRestored int64                   `json:"bytes_restored"`
	SHA256        *string                 `json:"sha256,omitempty"`
	BackupSHA256  *string                 `json:"backup_sha256,omitempty"`
}

type NewRestoreExecuteOptions struct {
	CreatedAt time.Time
}

func LoadRestorePreviewFromFile(previewPath string) (RestorePreview, error) {
	data, err := os.ReadFile(previewPath)
	if err != nil {
		return RestorePreview{}, fmt.Errorf("read restore preview %q: %w", previewPath, err)
	}
	return ParseRestorePreviewJSON(data)
}

func ParseRestorePreviewJSON(data []byte) (RestorePreview, error) {
	var preview RestorePreview
	if err := json.Unmarshal(data, &preview); err != nil {
		return RestorePreview{}, fmt.Errorf("parse restore preview json: %w", err)
	}
	return preview, nil
}

func ExecuteRestore(plan BackupPlan, backupResult BackupResult, restorePreview RestorePreview, opts NewRestoreExecuteOptions) (RestoreResult, error) {
	prepared, err := prepareRestoreExecution(plan, backupResult, restorePreview)
	if err != nil {
		return RestoreResult{}, err
	}

	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	result := RestoreResult{
		ProposalID: plan.ProposalID,
		ApprovalID: plan.ApprovalID,
		RunID:      plan.RunID,
		TaskID:     plan.TaskID,
		Domain:     plan.Domain,
		BackupRoot: plan.BackupRoot,
		Items:      []RestoreResultItem{},
		CreatedAt:  createdAt.UTC(),
	}

	for _, item := range prepared {
		resultItem := RestoreResultItem{
			TargetPath:   item.previewItem.TargetPath,
			BackupPath:   item.previewItem.BackupPath,
			Exists:       item.previewItem.Exists,
			Operation:    item.previewItem.Operation,
			BackupSHA256: cloneStringPointer(item.previewItem.BackupSHA256),
		}

		if item.previewItem.Exists {
			bytesRestored, err := copyRestoreFile(item.previewItem.BackupPath, item.previewItem.TargetPath)
			if err != nil {
				return RestoreResult{}, err
			}
			sha256, err := fileSHA256(item.previewItem.TargetPath)
			if err != nil {
				return RestoreResult{}, err
			}
			resultItem.Status = RestoreResultStatusRestored
			resultItem.BytesRestored = bytesRestored
			resultItem.SHA256 = &sha256
			result.Items = append(result.Items, resultItem)
			continue
		}

		targetInfo, err := os.Stat(item.previewItem.TargetPath)
		if err != nil {
			if os.IsNotExist(err) {
				resultItem.Status = RestoreResultStatusSkippedMissing
				result.Items = append(result.Items, resultItem)
				continue
			}
			return RestoreResult{}, fmt.Errorf("stat target_path %q: %w", item.previewItem.TargetPath, err)
		}
		if targetInfo.IsDir() {
			return RestoreResult{}, fmt.Errorf("target_path %q is a directory", item.previewItem.TargetPath)
		}
		if err := os.Remove(item.previewItem.TargetPath); err != nil {
			return RestoreResult{}, fmt.Errorf("remove target_path %q: %w", item.previewItem.TargetPath, err)
		}
		resultItem.Status = RestoreResultStatusRemoved
		result.Items = append(result.Items, resultItem)
	}

	return result, nil
}

func (r RestoreResult) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

type preparedRestoreItem struct {
	previewItem RestorePreviewItem
}

func prepareRestoreExecution(plan BackupPlan, backupResult BackupResult, loadedPreview RestorePreview) ([]preparedRestoreItem, error) {
	currentPreview, err := BuildRestorePreview(plan, backupResult, NewRestorePreviewOptions{})
	if err != nil {
		return nil, err
	}
	currentPreview.CreatedAt = loadedPreview.CreatedAt

	if err := validateRestorePreviewBelongsToPlan(plan, loadedPreview); err != nil {
		return nil, err
	}
	if err := validateRestorePreviewMatchesCurrent(currentPreview, loadedPreview); err != nil {
		return nil, err
	}

	prepared := make([]preparedRestoreItem, 0, len(currentPreview.Items))
	for index, item := range currentPreview.Items {
		if err := validateRestoreExecutionItem(plan.BackupRoot, index, item); err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedRestoreItem{previewItem: item})
	}
	return prepared, nil
}

func validateRestorePreviewBelongsToPlan(plan BackupPlan, preview RestorePreview) error {
	if preview.ProposalID != plan.ProposalID {
		return fmt.Errorf("restore-preview proposal_id %q does not match backup plan proposal_id %q", preview.ProposalID, plan.ProposalID)
	}
	if preview.ApprovalID != plan.ApprovalID {
		return fmt.Errorf("restore-preview approval_id %q does not match backup plan approval_id %q", preview.ApprovalID, plan.ApprovalID)
	}
	if preview.RunID != plan.RunID {
		return fmt.Errorf("restore-preview run_id %q does not match backup plan run_id %q", preview.RunID, plan.RunID)
	}
	if preview.TaskID != plan.TaskID {
		return fmt.Errorf("restore-preview task_id %q does not match backup plan task_id %q", preview.TaskID, plan.TaskID)
	}
	if preview.Domain != plan.Domain {
		return fmt.Errorf("restore-preview domain %q does not match backup plan domain %q", preview.Domain, plan.Domain)
	}
	if !sameFilesystemPath(preview.BackupRoot, plan.BackupRoot) {
		return fmt.Errorf("restore-preview backup_root %q does not match backup plan backup_root %q", preview.BackupRoot, plan.BackupRoot)
	}
	return nil
}

func validateRestorePreviewMatchesCurrent(currentPreview RestorePreview, loadedPreview RestorePreview) error {
	currentHash, err := restorePreviewCompareSHA256(currentPreview)
	if err != nil {
		return err
	}
	loadedHash, err := restorePreviewCompareSHA256(loadedPreview)
	if err != nil {
		return err
	}
	if loadedHash != currentHash {
		return fmt.Errorf("restore-preview diverged from current restore dry-run")
	}
	return nil
}

func restorePreviewCompareSHA256(preview RestorePreview) (string, error) {
	preview.CreatedAt = preview.CreatedAt.UTC()
	return canonicalJSONSHA256(preview)
}

func validateRestoreExecutionItem(backupRoot string, index int, item RestorePreviewItem) error {
	restoreItem := RestoreItem{
		TargetPath: item.TargetPath,
		BackupPath: item.BackupPath,
		Exists:     item.Exists,
		Operation:  item.Operation,
		SHA256:     cloneStringPointer(item.SHA256),
	}
	if err := validateRestoreItemShape(backupRoot, index, restoreItem); err != nil {
		return err
	}
	if item.Exists {
		if item.Action != RestorePreviewActionRestoreFromBackup {
			return fmt.Errorf("restore preview item[%d] action %q does not match existing target", index, item.Action)
		}
		if item.BackupSHA256 == nil || strings.TrimSpace(*item.BackupSHA256) == "" {
			return fmt.Errorf("restore preview item[%d] backup_sha256 is required for target_path %q", index, item.TargetPath)
		}
		currentBackupSHA256, err := fileSHA256(item.BackupPath)
		if err != nil {
			return fmt.Errorf("restore preview item[%d] backup_path %q cannot be hashed: %w", index, item.BackupPath, err)
		}
		if currentBackupSHA256 != *item.BackupSHA256 {
			return fmt.Errorf("restore preview item[%d] backup_path %q sha256 does not match restore-preview", index, item.BackupPath)
		}
		targetInfo, err := os.Stat(item.TargetPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("restore preview item[%d] stat target_path %q: %w", index, item.TargetPath, err)
		}
		if err == nil && targetInfo.IsDir() {
			return fmt.Errorf("restore preview item[%d] target_path %q is a directory", index, item.TargetPath)
		}
		return nil
	}

	if item.Action != RestorePreviewActionRemoveIfExists {
		return fmt.Errorf("restore preview item[%d] action %q does not match missing target", index, item.Action)
	}
	targetInfo, err := os.Stat(item.TargetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("restore preview item[%d] stat target_path %q: %w", index, item.TargetPath, err)
	}
	if targetInfo.IsDir() {
		return fmt.Errorf("restore preview item[%d] target_path %q is a directory", index, item.TargetPath)
	}
	return nil
}

func copyRestoreFile(sourcePath string, destinationPath string) (int64, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("open backup_path %q: %w", sourcePath, err)
	}
	defer source.Close()

	destinationDir := filepath.Dir(destinationPath)
	if destinationDir != "." {
		if err := os.MkdirAll(destinationDir, 0o755); err != nil {
			return 0, fmt.Errorf("create target directory %q: %w", destinationDir, err)
		}
	}
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open target_path %q: %w", destinationPath, err)
	}
	defer destination.Close()

	written, err := io.Copy(destination, source)
	if err != nil {
		return written, fmt.Errorf("copy backup_path %q to target_path %q: %w", sourcePath, destinationPath, err)
	}
	return written, nil
}
