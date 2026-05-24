package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ApplyResultJSONArtifactName = "apply-result.json"

type ApplyResultItemStatus string

const (
	ApplyResultStatusCreated  ApplyResultItemStatus = "created"
	ApplyResultStatusAppended ApplyResultItemStatus = "appended"
)

type ApplyResult struct {
	ProposalID string            `json:"proposal_id"`
	ApprovalID string            `json:"approval_id"`
	RunID      string            `json:"run_id"`
	TaskID     string            `json:"task_id"`
	Domain     string            `json:"domain"`
	Items      []ApplyResultItem `json:"items"`
	CreatedAt  time.Time         `json:"created_at"`
}

type ApplyResultItem struct {
	TargetPath   string                `json:"target_path"`
	Operation    MemoryOperation       `json:"operation"`
	Status       ApplyResultItemStatus `json:"status"`
	BytesWritten int64                 `json:"bytes_written"`
	SHA256       string                `json:"sha256"`
}

type NewApplyExecuteOptions struct {
	CreatedAt time.Time
}

func LoadBackupResultFromFile(resultPath string) (BackupResult, error) {
	data, err := os.ReadFile(resultPath)
	if err != nil {
		return BackupResult{}, fmt.Errorf("read backup result %q: %w", resultPath, err)
	}
	return ParseBackupResultJSON(data)
}

func ParseBackupResultJSON(data []byte) (BackupResult, error) {
	var result BackupResult
	if err := json.Unmarshal(data, &result); err != nil {
		return BackupResult{}, fmt.Errorf("parse backup result json: %w", err)
	}
	return result, nil
}

func ExecuteApply(proposal MemoryProposal, approval MemoryApproval, policy *MemoryPolicy, backupPlan BackupPlan, backupResult BackupResult, opts NewApplyExecuteOptions) (ApplyResult, error) {
	prepared, err := prepareApplyExecution(proposal, approval, policy, backupPlan, backupResult)
	if err != nil {
		return ApplyResult{}, err
	}

	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	result := ApplyResult{
		ProposalID: proposal.ProposalID,
		ApprovalID: approval.ApprovalID,
		RunID:      proposal.RunID,
		TaskID:     proposal.TaskID,
		Domain:     proposal.Domain,
		Items:      []ApplyResultItem{},
		CreatedAt:  createdAt.UTC(),
	}

	for _, item := range prepared {
		bytesWritten, status, err := executeApplyAction(item.action)
		if err != nil {
			return ApplyResult{}, err
		}
		sha256, err := fileSHA256(item.action.TargetPath)
		if err != nil {
			return ApplyResult{}, err
		}
		result.Items = append(result.Items, ApplyResultItem{
			TargetPath:   item.action.TargetPath,
			Operation:    item.action.Operation,
			Status:       status,
			BytesWritten: bytesWritten,
			SHA256:       sha256,
		})
	}
	return result, nil
}

func (r ApplyResult) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

type preparedApplyItem struct {
	action ApplyPreviewAction
}

func prepareApplyExecution(proposal MemoryProposal, approval MemoryApproval, policy *MemoryPolicy, backupPlan BackupPlan, backupResult BackupResult) ([]preparedApplyItem, error) {
	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{})
	if preflight.Status == ApplyPreflightStatusFailed {
		return nil, fmt.Errorf("apply preflight failed")
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		return nil, fmt.Errorf("build apply dry-run preview: %w", err)
	}
	if err := validateApplyBackupArtifacts(proposal, approval, backupPlan, backupResult); err != nil {
		return nil, err
	}

	planByTarget, err := validateApplyBackupPlanItems(backupPlan)
	if err != nil {
		return nil, err
	}
	if err := validateApplyBackupResultItems(backupPlan, backupResult); err != nil {
		return nil, err
	}

	prepared := make([]preparedApplyItem, 0, len(preview.Actions))
	createTargets := map[string]bool{}
	for index, action := range preview.Actions {
		if err := validateApplyAction(index, action, planByTarget, createTargets); err != nil {
			return nil, err
		}
		prepared = append(prepared, preparedApplyItem{action: action})
	}
	return prepared, nil
}

func validateApplyBackupArtifacts(proposal MemoryProposal, approval MemoryApproval, backupPlan BackupPlan, backupResult BackupResult) error {
	if backupPlan.ProposalID != proposal.ProposalID {
		return fmt.Errorf("backup plan proposal_id %q does not match proposal_id %q", backupPlan.ProposalID, proposal.ProposalID)
	}
	if backupPlan.ApprovalID != approval.ApprovalID {
		return fmt.Errorf("backup plan approval_id %q does not match approval_id %q", backupPlan.ApprovalID, approval.ApprovalID)
	}
	if backupResult.ProposalID != proposal.ProposalID {
		return fmt.Errorf("backup result proposal_id %q does not match proposal_id %q", backupResult.ProposalID, proposal.ProposalID)
	}
	if backupResult.ApprovalID != approval.ApprovalID {
		return fmt.Errorf("backup result approval_id %q does not match approval_id %q", backupResult.ApprovalID, approval.ApprovalID)
	}
	if backupPlan.RunID != proposal.RunID || backupResult.RunID != proposal.RunID {
		return fmt.Errorf("backup artifacts run_id must match proposal run_id %q", proposal.RunID)
	}
	if backupPlan.TaskID != proposal.TaskID || backupResult.TaskID != proposal.TaskID {
		return fmt.Errorf("backup artifacts task_id must match proposal task_id %q", proposal.TaskID)
	}
	if backupPlan.Domain != proposal.Domain || backupResult.Domain != proposal.Domain {
		return fmt.Errorf("backup artifacts domain must match proposal domain %q", proposal.Domain)
	}
	if !sameFilesystemPath(backupPlan.BackupRoot, backupResult.BackupRoot) {
		return fmt.Errorf("backup result backup_root %q does not match backup plan backup_root %q", backupResult.BackupRoot, backupPlan.BackupRoot)
	}
	return nil
}

func validateApplyBackupPlanItems(plan BackupPlan) (map[string]BackupItem, error) {
	backupRoot, err := cleanBackupRoot(plan.BackupRoot)
	if err != nil {
		return nil, err
	}
	byTarget := map[string]BackupItem{}
	for index, item := range plan.Items {
		if err := validateBackupMaterializeItem(backupRoot, index, item); err != nil {
			return nil, err
		}
		targetKey := applyPathKey(item.TargetPath)
		if _, exists := byTarget[targetKey]; exists {
			return nil, fmt.Errorf("backup item[%d] target_path %q is duplicated", index, item.TargetPath)
		}
		if item.Exists {
			if item.SHA256 == nil || strings.TrimSpace(*item.SHA256) == "" {
				return nil, fmt.Errorf("backup item[%d] missing sha256 for existing target_path %q", index, item.TargetPath)
			}
			currentSHA256, err := fileSHA256(item.TargetPath)
			if err != nil {
				return nil, fmt.Errorf("backup item[%d] target_path %q cannot be hashed: %w", index, item.TargetPath, err)
			}
			if currentSHA256 != *item.SHA256 {
				return nil, fmt.Errorf("backup item[%d] target_path %q changed since backup plan", index, item.TargetPath)
			}
		} else {
			exists, err := targetExists(item.TargetPath)
			if err != nil {
				return nil, fmt.Errorf("backup item[%d] stat target_path %q: %w", index, item.TargetPath, err)
			}
			if exists {
				return nil, fmt.Errorf("backup item[%d] target_path %q appeared since backup plan", index, item.TargetPath)
			}
		}
		byTarget[targetKey] = item
	}
	return byTarget, nil
}

func validateApplyBackupResultItems(plan BackupPlan, result BackupResult) error {
	planItems := map[string]BackupItem{}
	for _, item := range plan.Items {
		planItems[backupResultCoverageKey(item.TargetPath, item.BackupPath)] = item
	}
	seenResultItems := map[string]bool{}
	for index, item := range result.Items {
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
		if !pathWithinRoot(item.BackupPath, plan.BackupRoot) {
			return fmt.Errorf("backup result item[%d] backup_path %q escapes backup_root %q", index, item.BackupPath, plan.BackupRoot)
		}

		key := backupResultCoverageKey(item.TargetPath, item.BackupPath)
		planItem, ok := planItems[key]
		if !ok {
			return fmt.Errorf("backup result item[%d] target_path %q is not in backup plan", index, item.TargetPath)
		}
		if seenResultItems[key] {
			return fmt.Errorf("backup result item[%d] target_path %q is duplicated", index, item.TargetPath)
		}
		seenResultItems[key] = true
		if err := validateApplyBackupResultItem(index, planItem, item); err != nil {
			return err
		}
	}
	for index, item := range plan.Items {
		key := backupResultCoverageKey(item.TargetPath, item.BackupPath)
		if !seenResultItems[key] {
			return fmt.Errorf("backup result missing item for backup plan item[%d] target_path %q", index, item.TargetPath)
		}
	}
	return nil
}

func validateApplyBackupResultItem(index int, planItem BackupItem, resultItem BackupResultItem) error {
	if planItem.Exists != resultItem.Exists {
		return fmt.Errorf("backup result item[%d] exists %t does not match backup plan exists %t", index, resultItem.Exists, planItem.Exists)
	}
	if planItem.Operation != resultItem.Operation {
		return fmt.Errorf("backup result item[%d] operation %q does not match backup plan operation %q", index, resultItem.Operation, planItem.Operation)
	}
	if planItem.Exists {
		if resultItem.Status != BackupResultStatusCopied {
			return fmt.Errorf("backup result item[%d] backup was not copied for target_path %q", index, resultItem.TargetPath)
		}
		if resultItem.SHA256 == nil || planItem.SHA256 == nil || *resultItem.SHA256 != *planItem.SHA256 {
			return fmt.Errorf("backup result item[%d] sha256 does not match backup plan for target_path %q", index, resultItem.TargetPath)
		}
		if resultItem.BackupSHA256 == nil || strings.TrimSpace(*resultItem.BackupSHA256) == "" {
			return fmt.Errorf("backup result item[%d] backup_sha256 is required for copied target_path %q", index, resultItem.TargetPath)
		}
		backupSHA256, err := fileSHA256(resultItem.BackupPath)
		if err != nil {
			return fmt.Errorf("backup result item[%d] backup_path %q cannot be hashed: %w", index, resultItem.BackupPath, err)
		}
		if backupSHA256 != *resultItem.BackupSHA256 {
			return fmt.Errorf("backup result item[%d] backup_path %q sha256 does not match backup result", index, resultItem.BackupPath)
		}
		return nil
	}
	if resultItem.Status != BackupResultStatusSkippedMissing {
		return fmt.Errorf("backup result item[%d] status %q does not match missing target", index, resultItem.Status)
	}
	return nil
}

func validateApplyAction(index int, action ApplyPreviewAction, planByTarget map[string]BackupItem, createTargets map[string]bool) error {
	if strings.TrimSpace(action.TargetPath) == "" {
		return fmt.Errorf("apply action[%d] target_path is required", index)
	}
	if hasPathTraversal(action.TargetPath) {
		return fmt.Errorf("apply action[%d] target_path %q contains path traversal", index, action.TargetPath)
	}
	if _, ok := planByTarget[applyPathKey(action.TargetPath)]; !ok {
		return fmt.Errorf("apply action[%d] target_path %q is not covered by backup plan", index, action.TargetPath)
	}
	switch action.Operation {
	case OperationCreate:
		targetKey := applyPathKey(action.TargetPath)
		if createTargets[targetKey] {
			return fmt.Errorf("apply action[%d] duplicate create target_path %q", index, action.TargetPath)
		}
		createTargets[targetKey] = true
		exists, err := targetExists(action.TargetPath)
		if err != nil {
			return fmt.Errorf("apply action[%d] stat target_path %q: %w", index, action.TargetPath, err)
		}
		if exists {
			return fmt.Errorf("apply action[%d] create target_path %q already exists", index, action.TargetPath)
		}
	case OperationAppend:
		info, err := os.Stat(action.TargetPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("apply action[%d] stat target_path %q: %w", index, action.TargetPath, err)
		}
		if err == nil && info.IsDir() {
			return fmt.Errorf("apply action[%d] target_path %q is a directory", index, action.TargetPath)
		}
	case OperationUpdate, OperationArchive:
		return fmt.Errorf("apply action[%d] operation not implemented: %s", index, action.Operation)
	default:
		return fmt.Errorf("apply action[%d] operation %q is not supported", index, action.Operation)
	}
	return nil
}

func executeApplyAction(action ApplyPreviewAction) (int64, ApplyResultItemStatus, error) {
	if err := os.MkdirAll(filepath.Dir(action.TargetPath), 0o755); err != nil {
		return 0, "", fmt.Errorf("create target directory for %q: %w", action.TargetPath, err)
	}

	switch action.Operation {
	case OperationCreate:
		written, err := writeApplyContent(action.TargetPath, action.Content, os.O_CREATE|os.O_EXCL|os.O_WRONLY)
		if err != nil {
			return written, "", err
		}
		return written, ApplyResultStatusCreated, nil
	case OperationAppend:
		written, err := writeApplyContent(action.TargetPath, action.Content, os.O_CREATE|os.O_APPEND|os.O_WRONLY)
		if err != nil {
			return written, "", err
		}
		return written, ApplyResultStatusAppended, nil
	default:
		return 0, "", fmt.Errorf("operation not implemented: %s", action.Operation)
	}
}

func writeApplyContent(targetPath string, content string, flag int) (int64, error) {
	file, err := os.OpenFile(targetPath, flag, 0o600)
	if err != nil {
		return 0, fmt.Errorf("open target_path %q: %w", targetPath, err)
	}
	defer file.Close()

	written, err := file.WriteString(content)
	if err != nil {
		return int64(written), fmt.Errorf("write target_path %q: %w", targetPath, err)
	}
	return int64(written), nil
}

func targetExists(targetPath string) (bool, error) {
	_, err := os.Stat(targetPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func backupResultCoverageKey(targetPath string, backupPath string) string {
	return applyPathKey(targetPath) + "\x00" + applyPathKey(backupPath)
}

func applyPathKey(value string) string {
	value = filepath.Clean(strings.TrimSpace(value))
	abs, err := filepath.Abs(value)
	if err != nil {
		return filepath.ToSlash(value)
	}
	return filepath.ToSlash(abs)
}
