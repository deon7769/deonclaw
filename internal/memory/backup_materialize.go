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

const BackupResultJSONArtifactName = "backup-result.json"

type BackupResultItemStatus string

const (
	BackupResultStatusCopied         BackupResultItemStatus = "copied"
	BackupResultStatusSkippedMissing BackupResultItemStatus = "skipped_missing"
)

type BackupResult struct {
	ProposalID string             `json:"proposal_id"`
	ApprovalID string             `json:"approval_id"`
	RunID      string             `json:"run_id"`
	TaskID     string             `json:"task_id"`
	Domain     string             `json:"domain"`
	BackupRoot string             `json:"backup_root"`
	Items      []BackupResultItem `json:"items"`
	CreatedAt  time.Time          `json:"created_at"`
}

type BackupResultItem struct {
	TargetPath   string                 `json:"target_path"`
	BackupPath   string                 `json:"backup_path"`
	Exists       bool                   `json:"exists"`
	Operation    MemoryOperation        `json:"operation"`
	Status       BackupResultItemStatus `json:"status"`
	SHA256       *string                `json:"sha256,omitempty"`
	BackupSHA256 *string                `json:"backup_sha256,omitempty"`
	SizeBytes    *int64                 `json:"size_bytes,omitempty"`
	BytesCopied  int64                  `json:"bytes_copied"`
}

type NewBackupMaterializeOptions struct {
	CreatedAt time.Time
}

func LoadBackupPlanFromFile(planPath string) (BackupPlan, error) {
	data, err := os.ReadFile(planPath)
	if err != nil {
		return BackupPlan{}, fmt.Errorf("read backup plan %q: %w", planPath, err)
	}
	return ParseBackupPlanJSON(data)
}

func ParseBackupPlanJSON(data []byte) (BackupPlan, error) {
	var plan BackupPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return BackupPlan{}, fmt.Errorf("parse backup plan json: %w", err)
	}
	return plan, nil
}

func MaterializeBackup(plan BackupPlan, opts NewBackupMaterializeOptions) (BackupResult, error) {
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	prepared, err := prepareBackupMaterialization(plan)
	if err != nil {
		return BackupResult{}, err
	}

	result := BackupResult{
		ProposalID: plan.ProposalID,
		ApprovalID: plan.ApprovalID,
		RunID:      plan.RunID,
		TaskID:     plan.TaskID,
		Domain:     plan.Domain,
		BackupRoot: plan.BackupRoot,
		Items:      []BackupResultItem{},
		CreatedAt:  createdAt.UTC(),
	}

	for _, item := range prepared {
		resultItem := BackupResultItem{
			TargetPath:  item.planItem.TargetPath,
			BackupPath:  item.planItem.BackupPath,
			Exists:      item.planItem.Exists,
			Operation:   item.planItem.Operation,
			SHA256:      cloneStringPointer(item.planItem.SHA256),
			SizeBytes:   cloneInt64Pointer(item.planItem.SizeBytes),
			BytesCopied: 0,
		}
		if !item.planItem.Exists {
			resultItem.Status = BackupResultStatusSkippedMissing
			result.Items = append(result.Items, resultItem)
			continue
		}

		bytesCopied, err := copyFile(item.planItem.TargetPath, item.planItem.BackupPath)
		if err != nil {
			return BackupResult{}, err
		}
		backupSHA256, err := fileSHA256(item.planItem.BackupPath)
		if err != nil {
			return BackupResult{}, fmt.Errorf("hash backup_path %q: %w", item.planItem.BackupPath, err)
		}
		resultItem.Status = BackupResultStatusCopied
		resultItem.BackupSHA256 = &backupSHA256
		resultItem.BytesCopied = bytesCopied
		result.Items = append(result.Items, resultItem)
	}

	return result, nil
}

func (r BackupResult) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

type preparedBackupMaterializeItem struct {
	planItem BackupItem
}

func prepareBackupMaterialization(plan BackupPlan) ([]preparedBackupMaterializeItem, error) {
	backupRoot, err := cleanBackupRoot(plan.BackupRoot)
	if err != nil {
		return nil, err
	}
	prepared := make([]preparedBackupMaterializeItem, 0, len(plan.Items))
	for index, item := range plan.Items {
		if err := validateBackupMaterializeItem(backupRoot, index, item); err != nil {
			return nil, err
		}
		if item.Exists {
			currentSHA256, err := fileSHA256(item.TargetPath)
			if err != nil {
				return nil, fmt.Errorf("backup item[%d] target_path %q cannot be hashed: %w", index, item.TargetPath, err)
			}
			if item.SHA256 == nil || strings.TrimSpace(*item.SHA256) == "" {
				return nil, fmt.Errorf("backup item[%d] missing sha256 for existing target_path %q", index, item.TargetPath)
			}
			if currentSHA256 != *item.SHA256 {
				return nil, fmt.Errorf("backup item[%d] target_path %q changed since backup plan", index, item.TargetPath)
			}
		}
		prepared = append(prepared, preparedBackupMaterializeItem{planItem: item})
	}
	return prepared, nil
}

func validateBackupMaterializeItem(backupRoot string, index int, item BackupItem) error {
	if strings.TrimSpace(item.TargetPath) == "" {
		return fmt.Errorf("backup item[%d] target_path is required", index)
	}
	if hasPathTraversal(item.TargetPath) {
		return fmt.Errorf("backup item[%d] target_path %q contains path traversal", index, item.TargetPath)
	}
	if strings.TrimSpace(item.BackupPath) == "" {
		return fmt.Errorf("backup item[%d] backup_path is required", index)
	}
	if hasPathTraversal(item.BackupPath) {
		return fmt.Errorf("backup item[%d] backup_path %q contains path traversal", index, item.BackupPath)
	}
	if !pathWithinRoot(item.BackupPath, backupRoot) {
		return fmt.Errorf("backup item[%d] backup_path %q escapes backup_root %q", index, item.BackupPath, backupRoot)
	}
	if sameFilesystemPath(item.TargetPath, item.BackupPath) {
		return fmt.Errorf("backup item[%d] backup_path must not equal target_path %q", index, item.TargetPath)
	}
	return nil
}

func copyFile(sourcePath string, destinationPath string) (int64, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return 0, fmt.Errorf("open target_path %q: %w", sourcePath, err)
	}
	defer source.Close()

	destinationDir := filepath.Dir(destinationPath)
	if destinationDir != "." {
		if err := os.MkdirAll(destinationDir, 0o755); err != nil {
			return 0, fmt.Errorf("create backup directory %q: %w", destinationDir, err)
		}
	}
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open backup_path %q: %w", destinationPath, err)
	}
	defer destination.Close()

	written, err := io.Copy(destination, source)
	if err != nil {
		return written, fmt.Errorf("copy target_path %q to backup_path %q: %w", sourcePath, destinationPath, err)
	}
	return written, nil
}

func sameFilesystemPath(left string, right string) bool {
	leftAbs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(left)))
	if err != nil {
		return false
	}
	rightAbs, err := filepath.Abs(filepath.Clean(strings.TrimSpace(right)))
	if err != nil {
		return false
	}
	return leftAbs == rightAbs
}
