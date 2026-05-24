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
		bytesWritten, status, err := executeApplyAction(item.action, item.planItem)
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
	action   ApplyPreviewAction
	planItem BackupItem
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
		prepared = append(prepared, preparedApplyItem{
			action:   action,
			planItem: planByTarget[applyPathKey(action.TargetPath)],
		})
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
		if planItem.SHA256 == nil || strings.TrimSpace(*planItem.SHA256) == "" {
			return fmt.Errorf("backup result item[%d] backup plan sha256 is required for copied target_path %q", index, resultItem.TargetPath)
		}
		planSHA256 := *planItem.SHA256
		if resultItem.SHA256 == nil || *resultItem.SHA256 != planSHA256 {
			return fmt.Errorf("backup result item[%d] sha256 does not match backup plan for target_path %q", index, resultItem.TargetPath)
		}
		if resultItem.BackupSHA256 == nil || strings.TrimSpace(*resultItem.BackupSHA256) == "" {
			return fmt.Errorf("backup result item[%d] backup_sha256 is required for copied target_path %q", index, resultItem.TargetPath)
		}
		if *resultItem.BackupSHA256 != planSHA256 {
			return fmt.Errorf("backup result item[%d] backup_sha256 does not match backup plan for target_path %q", index, resultItem.TargetPath)
		}
		backupSHA256, err := fileSHA256(resultItem.BackupPath)
		if err != nil {
			return fmt.Errorf("backup result item[%d] backup_path %q cannot be hashed: %w", index, resultItem.BackupPath, err)
		}
		if backupSHA256 != planSHA256 {
			return fmt.Errorf("backup result item[%d] backup_path %q sha256 does not match backup plan", index, resultItem.BackupPath)
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

func executeApplyAction(action ApplyPreviewAction, planItem BackupItem) (int64, ApplyResultItemStatus, error) {
	switch action.Operation {
	case OperationCreate:
		written, err := writeApplyContentAtomically(action.TargetPath, action.Content, atomicApplyWriteOptions{
			Mode: atomicApplyModeCreate,
		})
		if err != nil {
			return written, "", err
		}
		return written, ApplyResultStatusCreated, nil
	case OperationAppend:
		if !planItem.Exists {
			written, err := writeApplyContentAtomically(action.TargetPath, action.Content, atomicApplyWriteOptions{
				Mode: atomicApplyModeCreate,
			})
			if err != nil {
				return written, "", err
			}
			return written, ApplyResultStatusAppended, nil
		}
		if planItem.SHA256 == nil || strings.TrimSpace(*planItem.SHA256) == "" {
			return 0, "", fmt.Errorf("backup item target_path %q missing sha256 for append", action.TargetPath)
		}
		currentContent, err := os.ReadFile(action.TargetPath)
		if err != nil {
			return 0, "", fmt.Errorf("read target_path %q for append: %w", action.TargetPath, err)
		}
		_, err = writeApplyContentAtomically(action.TargetPath, string(currentContent)+action.Content, atomicApplyWriteOptions{
			Mode:                  atomicApplyModeReplace,
			ExpectedCurrentSHA256: *planItem.SHA256,
		})
		if err != nil {
			return 0, "", err
		}
		return int64(len(action.Content)), ApplyResultStatusAppended, nil
	default:
		return 0, "", fmt.Errorf("operation not implemented: %s", action.Operation)
	}
}

type atomicApplyMode string

const (
	atomicApplyModeCreate  atomicApplyMode = "create"
	atomicApplyModeReplace atomicApplyMode = "replace"
)

type atomicApplyWriteOptions struct {
	Mode                  atomicApplyMode
	ExpectedCurrentSHA256 string
	WriteTemp             func(tempPath string, content string) (int64, error)
	AfterTempWrite        func(tempPath string) error
	BeforeRename          func() error
}

func writeApplyContentAtomically(targetPath string, content string, opts atomicApplyWriteOptions) (int64, error) {
	targetDir := filepath.Dir(targetPath)
	if targetDir != "." {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return 0, fmt.Errorf("create target directory for %q: %w", targetPath, err)
		}
	}

	tempFile, err := os.CreateTemp(targetDir, "."+filepath.Base(targetPath)+".apply-*")
	if err != nil {
		return 0, fmt.Errorf("create temporary apply file for target_path %q: %w", targetPath, err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return 0, fmt.Errorf("close temporary apply file %q: %w", tempPath, err)
	}

	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tempPath)
		}
	}()

	writeTemp := opts.WriteTemp
	if writeTemp == nil {
		writeTemp = writeApplyTempContent
	}
	written, err := writeTemp(tempPath, content)
	if err != nil {
		return written, fmt.Errorf("write temporary apply file %q: %w", tempPath, err)
	}
	if opts.AfterTempWrite != nil {
		if err := opts.AfterTempWrite(tempPath); err != nil {
			return written, fmt.Errorf("after temporary apply file write %q: %w", tempPath, err)
		}
	}
	if err := validateApplyTempContent(tempPath, content); err != nil {
		return written, err
	}

	if opts.BeforeRename != nil {
		if err := opts.BeforeRename(); err != nil {
			return written, fmt.Errorf("before apply rename for target_path %q: %w", targetPath, err)
		}
	}

	switch opts.Mode {
	case atomicApplyModeCreate:
		exists, err := targetExists(targetPath)
		if err != nil {
			return written, fmt.Errorf("stat target_path %q before rename: %w", targetPath, err)
		}
		if exists {
			return written, fmt.Errorf("target_path %q already exists before rename", targetPath)
		}
	case atomicApplyModeReplace:
		if strings.TrimSpace(opts.ExpectedCurrentSHA256) == "" {
			return written, fmt.Errorf("expected current sha256 is required for target_path %q", targetPath)
		}
		currentSHA256, err := fileSHA256(targetPath)
		if err != nil {
			return written, fmt.Errorf("hash target_path %q before rename: %w", targetPath, err)
		}
		if currentSHA256 != opts.ExpectedCurrentSHA256 {
			return written, fmt.Errorf("target_path %q changed before rename", targetPath)
		}
	default:
		return written, fmt.Errorf("atomic apply mode %q is not supported", opts.Mode)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		return written, fmt.Errorf("rename temporary apply file %q to target_path %q: %w", tempPath, targetPath, err)
	}
	renamed = true
	return int64(written), nil
}

func writeApplyTempContent(tempPath string, content string) (int64, error) {
	if err := os.WriteFile(tempPath, []byte(content), 0o600); err != nil {
		return 0, err
	}
	return int64(len(content)), nil
}

func validateApplyTempContent(tempPath string, content string) error {
	data, err := os.ReadFile(tempPath)
	if err != nil {
		return fmt.Errorf("read temporary apply file %q: %w", tempPath, err)
	}
	if string(data) != content {
		return fmt.Errorf("temporary apply file %q content does not match expected content", tempPath)
	}
	return nil
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
