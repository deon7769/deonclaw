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
	ApplyResultStatusUpdated  ApplyResultItemStatus = "updated"
)

type ApplyResultStatus string

const (
	ApplyResultStatusSucceeded     ApplyResultStatus = "succeeded"
	ApplyResultStatusFailed        ApplyResultStatus = "failed"
	ApplyResultStatusPartialFailed ApplyResultStatus = "partial_failed"
)

type ApplyResult struct {
	ProposalID string                 `json:"proposal_id"`
	ApprovalID string                 `json:"approval_id"`
	RunID      string                 `json:"run_id"`
	TaskID     string                 `json:"task_id"`
	Domain     string                 `json:"domain"`
	Status     ApplyResultStatus      `json:"status"`
	Items      []ApplyResultItem      `json:"items"`
	FailedItem *ApplyResultFailedItem `json:"failed_item,omitempty"`
	Error      string                 `json:"error,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

type ApplyResultItem struct {
	TargetPath   string                `json:"target_path"`
	Operation    MemoryOperation       `json:"operation"`
	Status       ApplyResultItemStatus `json:"status"`
	BytesWritten int64                 `json:"bytes_written"`
	SHA256       string                `json:"sha256"`
}

type ApplyResultFailedItem struct {
	TargetPath string          `json:"target_path"`
	Operation  MemoryOperation `json:"operation"`
	Error      string          `json:"error,omitempty"`
}

type ApplyExecutionError struct {
	Result ApplyResult
	Err    error
}

func (e *ApplyExecutionError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "apply execution failed"
}

func (e *ApplyExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type NewApplyExecuteOptions struct {
	CreatedAt                  time.Time
	atomicWriteOptionsByTarget map[string]atomicApplyWriteOptions
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
		Status:     ApplyResultStatusFailed,
		Items:      []ApplyResultItem{},
		CreatedAt:  createdAt.UTC(),
	}

	prepared, err := prepareApplyExecution(proposal, approval, policy, backupPlan, backupResult)
	if err != nil {
		return applyExecutionFailure(result, ApplyResultStatusFailed, nil, err)
	}

	return executeApplyStaged(prepared, opts, result)
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
	targetKey := applyPathKey(action.TargetPath)
	planItem := planByTarget[targetKey]
	switch action.Operation {
	case OperationCreate:
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
	case OperationUpdate:
		if !planItem.Exists {
			return fmt.Errorf("apply action[%d] update target_path %q requires backup plan exists=true", index, action.TargetPath)
		}
		info, err := os.Stat(action.TargetPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("apply action[%d] update target_path %q does not exist", index, action.TargetPath)
		}
		if err != nil {
			return fmt.Errorf("apply action[%d] stat target_path %q: %w", index, action.TargetPath, err)
		}
		if info.IsDir() {
			return fmt.Errorf("apply action[%d] target_path %q is a directory", index, action.TargetPath)
		}
	case OperationArchive:
		return fmt.Errorf("apply action[%d] operation not implemented: %s", index, action.Operation)
	default:
		return fmt.Errorf("apply action[%d] operation %q is not supported", index, action.Operation)
	}
	return nil
}

func executeApplyAction(action ApplyPreviewAction, planItem BackupItem) (int64, ApplyResultItemStatus, error) {
	staged, err := prepareApplyActionTemp(preparedApplyItem{action: action, planItem: planItem}, nil)
	if err != nil {
		return 0, "", err
	}
	defer func() {
		cleanupStagedApplyItems([]stagedApplyItem{staged})
	}()

	if err := validateStagedApplyTemp(staged); err != nil {
		return staged.bytesWritten, "", err
	}
	if err := validateStagedApplyTarget(staged); err != nil {
		return staged.bytesWritten, "", err
	}
	if err := renameStagedApplyItem(&staged); err != nil {
		return staged.bytesWritten, "", err
	}
	return staged.bytesWritten, staged.status, nil
}

func executeApplyStaged(prepared []preparedApplyItem, opts NewApplyExecuteOptions, result ApplyResult) (ApplyResult, error) {
	staged := make([]stagedApplyItem, 0, len(prepared))
	defer func() {
		cleanupStagedApplyItems(staged)
	}()

	for _, item := range prepared {
		stagedItem, err := prepareApplyActionTemp(item, opts.atomicWriteOptionsByTarget)
		if err != nil {
			return applyExecutionFailure(result, ApplyResultStatusFailed, failedApplyResultItemFromAction(item.action, err), err)
		}
		staged = append(staged, stagedItem)
	}
	for _, item := range staged {
		if err := validateStagedApplyTemp(item); err != nil {
			return applyExecutionFailure(result, ApplyResultStatusFailed, failedApplyResultItemFromAction(item.action, err), err)
		}
	}
	for _, item := range staged {
		if err := validateStagedApplyTarget(item); err != nil {
			return applyExecutionFailure(result, ApplyResultStatusFailed, failedApplyResultItemFromAction(item.action, err), err)
		}
	}

	result.Items = make([]ApplyResultItem, 0, len(staged))
	for index := range staged {
		if err := renameStagedApplyItem(&staged[index]); err != nil {
			status := ApplyResultStatusFailed
			if len(result.Items) > 0 {
				status = ApplyResultStatusPartialFailed
			}
			return applyExecutionFailure(result, status, failedApplyResultItemFromAction(staged[index].action, err), err)
		}
		sha256, err := fileSHA256(staged[index].action.TargetPath)
		if err != nil {
			result.Items = append(result.Items, applyResultItemFromStaged(staged[index], ""))
			return applyExecutionFailure(result, ApplyResultStatusPartialFailed, failedApplyResultItemFromAction(staged[index].action, err), err)
		}
		result.Items = append(result.Items, applyResultItemFromStaged(staged[index], sha256))
	}
	result.Status = ApplyResultStatusSucceeded
	return result, nil
}

func applyExecutionFailure(result ApplyResult, status ApplyResultStatus, failedItem *ApplyResultFailedItem, err error) (ApplyResult, error) {
	result.Status = status
	result.FailedItem = failedItem
	if err != nil {
		result.Error = err.Error()
		if result.FailedItem != nil {
			result.FailedItem.Error = err.Error()
		}
	}
	if result.Items == nil {
		result.Items = []ApplyResultItem{}
	}
	return result, &ApplyExecutionError{Result: result, Err: err}
}

func applyResultItemFromStaged(item stagedApplyItem, sha256 string) ApplyResultItem {
	return ApplyResultItem{
		TargetPath:   item.action.TargetPath,
		Operation:    item.action.Operation,
		Status:       item.status,
		BytesWritten: item.bytesWritten,
		SHA256:       sha256,
	}
}

func failedApplyResultItemFromAction(action ApplyPreviewAction, err error) *ApplyResultFailedItem {
	item := &ApplyResultFailedItem{
		TargetPath: action.TargetPath,
		Operation:  action.Operation,
	}
	if err != nil {
		item.Error = err.Error()
	}
	return item
}

type stagedApplyItem struct {
	action                ApplyPreviewAction
	planItem              BackupItem
	targetPath            string
	content               string
	tempPath              string
	mode                  atomicApplyMode
	expectedCurrentSHA256 string
	bytesWritten          int64
	status                ApplyResultItemStatus
	beforeRename          func() error
	rename                func(tempPath string, targetPath string) error
	renamed               bool
}

func prepareApplyActionTemp(item preparedApplyItem, hooksByTarget map[string]atomicApplyWriteOptions) (stagedApplyItem, error) {
	action := item.action
	planItem := item.planItem
	mode := atomicApplyMode("")
	content := action.Content
	status := ApplyResultItemStatus("")
	bytesWritten := int64(len(action.Content))
	expectedCurrentSHA256 := ""

	switch action.Operation {
	case OperationCreate:
		mode = atomicApplyModeCreate
		status = ApplyResultStatusCreated
	case OperationAppend:
		if !planItem.Exists {
			mode = atomicApplyModeCreate
			status = ApplyResultStatusAppended
			break
		}
		mode = atomicApplyModeReplace
		status = ApplyResultStatusAppended
		if planItem.SHA256 == nil || strings.TrimSpace(*planItem.SHA256) == "" {
			return stagedApplyItem{}, fmt.Errorf("backup item target_path %q missing sha256 for append", action.TargetPath)
		}
		expectedCurrentSHA256 = *planItem.SHA256
		currentContent, err := os.ReadFile(action.TargetPath)
		if err != nil {
			return stagedApplyItem{}, fmt.Errorf("read target_path %q for append: %w", action.TargetPath, err)
		}
		content = string(currentContent) + action.Content
	case OperationUpdate:
		if !planItem.Exists {
			return stagedApplyItem{}, fmt.Errorf("backup item target_path %q must exist for update", action.TargetPath)
		}
		mode = atomicApplyModeReplace
		status = ApplyResultStatusUpdated
		if planItem.SHA256 == nil || strings.TrimSpace(*planItem.SHA256) == "" {
			return stagedApplyItem{}, fmt.Errorf("backup item target_path %q missing sha256 for update", action.TargetPath)
		}
		expectedCurrentSHA256 = *planItem.SHA256
	default:
		return stagedApplyItem{}, fmt.Errorf("operation not implemented: %s", action.Operation)
	}

	writeOptions := atomicApplyWriteOptions{
		Mode:                  mode,
		ExpectedCurrentSHA256: expectedCurrentSHA256,
	}
	if hooksByTarget != nil {
		if hook, ok := hooksByTarget[applyPathKey(action.TargetPath)]; ok {
			writeOptions.WriteTemp = hook.WriteTemp
			writeOptions.AfterTempWrite = hook.AfterTempWrite
			writeOptions.BeforeRename = hook.BeforeRename
			writeOptions.Rename = hook.Rename
		}
	}

	staged, err := prepareAtomicApplyWrite(action.TargetPath, content, writeOptions)
	if err != nil {
		return stagedApplyItem{}, err
	}
	staged.action = action
	staged.planItem = planItem
	staged.status = status
	staged.bytesWritten = bytesWritten
	return staged, nil
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
	Rename                func(tempPath string, targetPath string) error
}

func writeApplyContentAtomically(targetPath string, content string, opts atomicApplyWriteOptions) (int64, error) {
	staged, err := prepareAtomicApplyWrite(targetPath, content, opts)
	if err != nil {
		return 0, err
	}
	defer func() {
		cleanupStagedApplyItems([]stagedApplyItem{staged})
	}()

	if err := validateStagedApplyTemp(staged); err != nil {
		return staged.bytesWritten, err
	}
	if err := validateStagedApplyTarget(staged); err != nil {
		return staged.bytesWritten, err
	}
	if err := renameStagedApplyItem(&staged); err != nil {
		return staged.bytesWritten, err
	}
	return staged.bytesWritten, nil
}

func prepareAtomicApplyWrite(targetPath string, content string, opts atomicApplyWriteOptions) (stagedApplyItem, error) {
	targetDir := filepath.Dir(targetPath)
	if targetDir != "." {
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			return stagedApplyItem{}, fmt.Errorf("create target directory for %q: %w", targetPath, err)
		}
	}

	tempFile, err := os.CreateTemp(targetDir, "."+filepath.Base(targetPath)+".apply-*")
	if err != nil {
		return stagedApplyItem{}, fmt.Errorf("create temporary apply file for target_path %q: %w", targetPath, err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return stagedApplyItem{}, fmt.Errorf("close temporary apply file %q: %w", tempPath, err)
	}

	writeTemp := opts.WriteTemp
	if writeTemp == nil {
		writeTemp = writeApplyTempContent
	}
	written, err := writeTemp(tempPath, content)
	if err != nil {
		_ = os.Remove(tempPath)
		return stagedApplyItem{}, fmt.Errorf("write temporary apply file %q: %w", tempPath, err)
	}
	if opts.AfterTempWrite != nil {
		if err := opts.AfterTempWrite(tempPath); err != nil {
			_ = os.Remove(tempPath)
			return stagedApplyItem{}, fmt.Errorf("after temporary apply file write %q: %w", tempPath, err)
		}
	}

	return stagedApplyItem{
		targetPath:            targetPath,
		content:               content,
		tempPath:              tempPath,
		mode:                  opts.Mode,
		expectedCurrentSHA256: opts.ExpectedCurrentSHA256,
		bytesWritten:          written,
		beforeRename:          opts.BeforeRename,
		rename:                opts.Rename,
	}, nil
}

func validateStagedApplyTemp(item stagedApplyItem) error {
	return validateApplyTempContent(item.tempPath, item.content)
}

func validateStagedApplyTarget(item stagedApplyItem) error {
	if item.beforeRename != nil {
		if err := item.beforeRename(); err != nil {
			return fmt.Errorf("before apply rename for target_path %q: %w", item.targetPath, err)
		}
	}
	switch item.mode {
	case atomicApplyModeCreate:
		exists, err := targetExists(item.targetPath)
		if err != nil {
			return fmt.Errorf("stat target_path %q before rename: %w", item.targetPath, err)
		}
		if exists {
			return fmt.Errorf("target_path %q already exists before rename", item.targetPath)
		}
	case atomicApplyModeReplace:
		if strings.TrimSpace(item.expectedCurrentSHA256) == "" {
			return fmt.Errorf("expected current sha256 is required for target_path %q", item.targetPath)
		}
		currentSHA256, err := fileSHA256(item.targetPath)
		if err != nil {
			return fmt.Errorf("hash target_path %q before rename: %w", item.targetPath, err)
		}
		if currentSHA256 != item.expectedCurrentSHA256 {
			return fmt.Errorf("target_path %q changed before rename", item.targetPath)
		}
	default:
		return fmt.Errorf("atomic apply mode %q is not supported", item.mode)
	}
	return nil
}

func renameStagedApplyItem(item *stagedApplyItem) error {
	rename := item.rename
	if rename == nil {
		rename = os.Rename
	}
	if err := rename(item.tempPath, item.targetPath); err != nil {
		return fmt.Errorf("rename temporary apply file %q to target_path %q: %w", item.tempPath, item.targetPath, err)
	}
	item.renamed = true
	return nil
}

func cleanupStagedApplyItems(items []stagedApplyItem) {
	for _, item := range items {
		if item.tempPath != "" && !item.renamed {
			_ = os.Remove(item.tempPath)
		}
	}
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
