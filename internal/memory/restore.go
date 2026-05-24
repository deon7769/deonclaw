package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const RestorePreviewJSONArtifactName = "restore-preview.json"

type RestorePreviewStatus string

const RestorePreviewStatusDryRunOK RestorePreviewStatus = "dry_run_ok"

type RestorePreviewAction string

const (
	RestorePreviewActionRestoreFromBackup RestorePreviewAction = "restore_from_backup"
	RestorePreviewActionRemoveIfExists    RestorePreviewAction = "remove_if_exists"
)

type RestorePreview struct {
	ProposalID string               `json:"proposal_id"`
	ApprovalID string               `json:"approval_id"`
	RunID      string               `json:"run_id"`
	TaskID     string               `json:"task_id"`
	Domain     string               `json:"domain"`
	BackupRoot string               `json:"backup_root"`
	Status     RestorePreviewStatus `json:"status"`
	Items      []RestorePreviewItem `json:"items"`
	CreatedAt  time.Time            `json:"created_at"`
}

type RestorePreviewItem struct {
	TargetPath   string               `json:"target_path"`
	BackupPath   string               `json:"backup_path,omitempty"`
	Exists       bool                 `json:"exists"`
	Operation    MemoryOperation      `json:"operation"`
	Action       RestorePreviewAction `json:"action"`
	SHA256       *string              `json:"sha256,omitempty"`
	BackupSHA256 *string              `json:"backup_sha256,omitempty"`
}

type NewRestorePreviewOptions struct {
	CreatedAt time.Time
}

func BuildRestorePreview(plan BackupPlan, result BackupResult, opts NewRestorePreviewOptions) (RestorePreview, error) {
	if err := validateRestoreBackupArtifacts(plan, result); err != nil {
		return RestorePreview{}, err
	}
	if err := validateRestorePlan(plan); err != nil {
		return RestorePreview{}, err
	}

	resultByItem := map[string]BackupResultItem{}
	for index, item := range result.Items {
		if err := validateRestoreBackupResultItemShape(plan.BackupRoot, index, item); err != nil {
			return RestorePreview{}, err
		}
		key := backupResultCoverageKey(item.TargetPath, item.BackupPath)
		if _, exists := resultByItem[key]; exists {
			return RestorePreview{}, fmt.Errorf("backup result item[%d] target_path %q is duplicated", index, item.TargetPath)
		}
		resultByItem[key] = item
	}

	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	preview := RestorePreview{
		ProposalID: plan.ProposalID,
		ApprovalID: plan.ApprovalID,
		RunID:      plan.RunID,
		TaskID:     plan.TaskID,
		Domain:     plan.Domain,
		BackupRoot: plan.BackupRoot,
		Status:     RestorePreviewStatusDryRunOK,
		Items:      []RestorePreviewItem{},
		CreatedAt:  createdAt.UTC(),
	}

	for index, item := range plan.RestorePlan.Items {
		resultItem, ok := resultByItem[backupResultCoverageKey(item.TargetPath, item.BackupPath)]
		if !ok {
			return RestorePreview{}, fmt.Errorf("restore item[%d] target_path %q missing backup result item", index, item.TargetPath)
		}
		if err := validateRestoreItemAgainstResult(index, item, resultItem); err != nil {
			return RestorePreview{}, err
		}

		action := RestorePreviewActionRemoveIfExists
		if item.Exists {
			action = RestorePreviewActionRestoreFromBackup
		}
		preview.Items = append(preview.Items, RestorePreviewItem{
			TargetPath:   item.TargetPath,
			BackupPath:   item.BackupPath,
			Exists:       item.Exists,
			Operation:    item.Operation,
			Action:       action,
			SHA256:       cloneStringPointer(item.SHA256),
			BackupSHA256: cloneStringPointer(resultItem.BackupSHA256),
		})
	}

	return preview, nil
}

func (p RestorePreview) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validateRestoreBackupArtifacts(plan BackupPlan, result BackupResult) error {
	if result.ProposalID != plan.ProposalID {
		return fmt.Errorf("backup result proposal_id %q does not match backup plan proposal_id %q", result.ProposalID, plan.ProposalID)
	}
	if result.ApprovalID != plan.ApprovalID {
		return fmt.Errorf("backup result approval_id %q does not match backup plan approval_id %q", result.ApprovalID, plan.ApprovalID)
	}
	if result.RunID != plan.RunID {
		return fmt.Errorf("backup result run_id %q does not match backup plan run_id %q", result.RunID, plan.RunID)
	}
	if result.TaskID != plan.TaskID {
		return fmt.Errorf("backup result task_id %q does not match backup plan task_id %q", result.TaskID, plan.TaskID)
	}
	if result.Domain != plan.Domain {
		return fmt.Errorf("backup result domain %q does not match backup plan domain %q", result.Domain, plan.Domain)
	}
	if !sameFilesystemPath(result.BackupRoot, plan.BackupRoot) {
		return fmt.Errorf("backup result backup_root %q does not match backup plan backup_root %q", result.BackupRoot, plan.BackupRoot)
	}
	return nil
}

func validateRestorePlan(plan BackupPlan) error {
	if strings.TrimSpace(plan.RestorePlan.ProposalID) == "" {
		return fmt.Errorf("restore_plan proposal_id is required")
	}
	if strings.TrimSpace(plan.RestorePlan.ApprovalID) == "" {
		return fmt.Errorf("restore_plan approval_id is required")
	}
	if plan.RestorePlan.ProposalID != plan.ProposalID {
		return fmt.Errorf("restore_plan proposal_id %q does not match backup plan proposal_id %q", plan.RestorePlan.ProposalID, plan.ProposalID)
	}
	if plan.RestorePlan.ApprovalID != plan.ApprovalID {
		return fmt.Errorf("restore_plan approval_id %q does not match backup plan approval_id %q", plan.RestorePlan.ApprovalID, plan.ApprovalID)
	}
	if len(plan.RestorePlan.Items) == 0 && len(plan.Items) > 0 {
		return fmt.Errorf("restore_plan items are required")
	}
	if len(plan.RestorePlan.Items) != len(plan.Items) {
		return fmt.Errorf("restore_plan items count %d does not match backup plan items count %d", len(plan.RestorePlan.Items), len(plan.Items))
	}
	seen := map[string]bool{}
	for index, item := range plan.RestorePlan.Items {
		if err := validateRestoreItemShape(plan.BackupRoot, index, item); err != nil {
			return err
		}
		key := backupResultCoverageKey(item.TargetPath, item.BackupPath)
		if seen[key] {
			return fmt.Errorf("restore item[%d] target_path %q is duplicated", index, item.TargetPath)
		}
		seen[key] = true
	}
	return nil
}

func validateRestoreItemShape(backupRoot string, index int, item RestoreItem) error {
	if strings.TrimSpace(item.TargetPath) == "" {
		return fmt.Errorf("restore item[%d] target_path is required", index)
	}
	if hasPathTraversal(item.TargetPath) {
		return fmt.Errorf("restore item[%d] target_path %q contains path traversal", index, item.TargetPath)
	}
	if strings.TrimSpace(item.BackupPath) == "" {
		return fmt.Errorf("restore item[%d] backup_path is required", index)
	}
	if hasPathTraversal(item.BackupPath) {
		return fmt.Errorf("restore item[%d] backup_path %q contains path traversal", index, item.BackupPath)
	}
	if !pathWithinRoot(item.BackupPath, backupRoot) {
		return fmt.Errorf("restore item[%d] backup_path %q escapes backup_root %q", index, item.BackupPath, backupRoot)
	}
	if sameFilesystemPath(item.TargetPath, item.BackupPath) {
		return fmt.Errorf("restore item[%d] backup_path must not equal target_path %q", index, item.TargetPath)
	}
	return nil
}

func validateRestoreBackupResultItemShape(backupRoot string, index int, item BackupResultItem) error {
	if strings.TrimSpace(item.TargetPath) == "" {
		return fmt.Errorf("backup result item[%d] target_path is required", index)
	}
	if hasPathTraversal(item.TargetPath) {
		return fmt.Errorf("backup result item[%d] target_path %q contains path traversal", index, item.TargetPath)
	}
	if strings.TrimSpace(item.BackupPath) == "" {
		return fmt.Errorf("backup result item[%d] backup_path is required", index)
	}
	if hasPathTraversal(item.BackupPath) {
		return fmt.Errorf("backup result item[%d] backup_path %q contains path traversal", index, item.BackupPath)
	}
	if !pathWithinRoot(item.BackupPath, backupRoot) {
		return fmt.Errorf("backup result item[%d] backup_path %q escapes backup_root %q", index, item.BackupPath, backupRoot)
	}
	return nil
}

func validateRestoreItemAgainstResult(index int, restoreItem RestoreItem, resultItem BackupResultItem) error {
	if restoreItem.Exists != resultItem.Exists {
		return fmt.Errorf("restore item[%d] exists %t does not match backup result exists %t", index, restoreItem.Exists, resultItem.Exists)
	}
	if restoreItem.Operation != resultItem.Operation {
		return fmt.Errorf("restore item[%d] operation %q does not match backup result operation %q", index, restoreItem.Operation, resultItem.Operation)
	}
	if restoreItem.Exists {
		if resultItem.Status != BackupResultStatusCopied {
			return fmt.Errorf("restore item[%d] backup was not copied for target_path %q", index, restoreItem.TargetPath)
		}
		if resultItem.BackupSHA256 == nil || strings.TrimSpace(*resultItem.BackupSHA256) == "" {
			return fmt.Errorf("restore item[%d] backup_sha256 is required for target_path %q", index, restoreItem.TargetPath)
		}
		backupSHA256, err := fileSHA256(resultItem.BackupPath)
		if err != nil {
			return fmt.Errorf("restore item[%d] backup_path %q cannot be hashed: %w", index, resultItem.BackupPath, err)
		}
		if backupSHA256 != *resultItem.BackupSHA256 {
			return fmt.Errorf("restore item[%d] backup_path %q sha256 does not match backup result", index, resultItem.BackupPath)
		}
		return nil
	}
	if resultItem.Status != BackupResultStatusSkippedMissing {
		return fmt.Errorf("restore item[%d] status %q does not match missing target", index, resultItem.Status)
	}
	return nil
}
