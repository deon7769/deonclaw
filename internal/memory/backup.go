package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const BackupPlanJSONArtifactName = "backup-plan.json"

type BackupPlan struct {
	ProposalID  string       `json:"proposal_id"`
	ApprovalID  string       `json:"approval_id"`
	RunID       string       `json:"run_id"`
	TaskID      string       `json:"task_id"`
	Domain      string       `json:"domain"`
	BackupRoot  string       `json:"backup_root"`
	Items       []BackupItem `json:"items"`
	RestorePlan RestorePlan  `json:"restore_plan"`
	CreatedAt   time.Time    `json:"created_at"`
}

type BackupItem struct {
	TargetPath string          `json:"target_path"`
	Exists     bool            `json:"exists"`
	BackupPath string          `json:"backup_path"`
	SHA256     *string         `json:"sha256,omitempty"`
	SizeBytes  *int64          `json:"size_bytes,omitempty"`
	Operation  MemoryOperation `json:"operation"`
}

type RestorePlan struct {
	ProposalID string        `json:"proposal_id"`
	ApprovalID string        `json:"approval_id"`
	Items      []RestoreItem `json:"items"`
}

type RestoreItem struct {
	TargetPath string          `json:"target_path"`
	BackupPath string          `json:"backup_path"`
	Exists     bool            `json:"exists"`
	SHA256     *string         `json:"sha256,omitempty"`
	SizeBytes  *int64          `json:"size_bytes,omitempty"`
	Operation  MemoryOperation `json:"operation"`
}

type NewBackupPlanOptions struct {
	BackupRoot string
	CreatedAt  time.Time
}

func BuildBackupPlan(proposal MemoryProposal, approval MemoryApproval, policy *MemoryPolicy, opts NewBackupPlanOptions) (BackupPlan, error) {
	backupRoot := opts.BackupRoot
	if strings.TrimSpace(backupRoot) == "" {
		backupRoot = defaultBackupRoot(proposal.ProposalID)
	}
	backupRoot, err := cleanBackupRoot(backupRoot)
	if err != nil {
		return BackupPlan{}, err
	}

	preflight := BuildApplyPreflight(proposal, approval, policy, NewApplyPreflightOptions{
		CreatedAt: opts.CreatedAt,
	})
	if preflight.Status == ApplyPreflightStatusFailed {
		return BackupPlan{}, fmt.Errorf("apply preflight failed")
	}

	preview, err := BuildApplyDryRunPreview(proposal, policy)
	if err != nil {
		return BackupPlan{}, fmt.Errorf("build apply dry-run preview: %w", err)
	}

	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	plan := BackupPlan{
		ProposalID: proposal.ProposalID,
		ApprovalID: approval.ApprovalID,
		RunID:      proposal.RunID,
		TaskID:     proposal.TaskID,
		Domain:     proposal.Domain,
		BackupRoot: backupRoot,
		Items:      []BackupItem{},
		CreatedAt:  createdAt.UTC(),
	}
	restore := RestorePlan{
		ProposalID: proposal.ProposalID,
		ApprovalID: approval.ApprovalID,
		Items:      []RestoreItem{},
	}

	seenTargets := map[string]bool{}
	for _, action := range preview.Actions {
		targets := backupTargetsForAction(action)
		for _, target := range targets {
			targetKey := backupTargetKey(target.path)
			if seenTargets[targetKey] {
				continue
			}
			seenTargets[targetKey] = true

			item, err := buildBackupItem(backupRoot, target.path, target.operation)
			if err != nil {
				return BackupPlan{}, err
			}
			if target.mustExist && !item.Exists {
				return BackupPlan{}, fmt.Errorf("target_path %q must exist for archive backup plan", target.path)
			}
			if target.mustBeMissing && item.Exists {
				return BackupPlan{}, fmt.Errorf("archive_path %q must not exist for archive backup plan", target.path)
			}
			plan.Items = append(plan.Items, item)
			restore.Items = append(restore.Items, RestoreItem{
				TargetPath: item.TargetPath,
				BackupPath: item.BackupPath,
				Exists:     item.Exists,
				SHA256:     cloneStringPointer(item.SHA256),
				SizeBytes:  cloneInt64Pointer(item.SizeBytes),
				Operation:  item.Operation,
			})
		}
	}
	plan.RestorePlan = restore
	return plan, nil
}

type backupPlanTarget struct {
	path          string
	operation     MemoryOperation
	mustExist     bool
	mustBeMissing bool
}

func backupTargetsForAction(action ApplyPreviewAction) []backupPlanTarget {
	if action.Operation != OperationArchive {
		return []backupPlanTarget{{path: action.TargetPath, operation: action.Operation}}
	}
	return []backupPlanTarget{
		{path: action.TargetPath, operation: action.Operation, mustExist: true},
		{path: action.ArchivePath, operation: action.Operation, mustBeMissing: true},
	}
}

func (p BackupPlan) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func defaultBackupRoot(proposalID string) string {
	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" || hasPathTraversal(proposalID) {
		proposalID = "unknown-proposal"
	}
	proposalID = strings.NewReplacer(
		"\\", "-",
		"/", "-",
		":", "-",
	).Replace(proposalID)
	return filepath.Join(".deonclaw", "memory-backups", proposalID)
}

func cleanBackupRoot(backupRoot string) (string, error) {
	backupRoot = strings.TrimSpace(backupRoot)
	if backupRoot == "" {
		return "", fmt.Errorf("backup_root is required")
	}
	if hasPathTraversal(backupRoot) {
		return "", fmt.Errorf("backup_root %q contains path traversal", backupRoot)
	}
	cleaned := filepath.Clean(backupRoot)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("backup_root is required")
	}
	return cleaned, nil
}

func buildBackupItem(backupRoot string, targetPath string, operation MemoryOperation) (BackupItem, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return BackupItem{}, fmt.Errorf("target_path is required")
	}
	if hasPathTraversal(targetPath) {
		return BackupItem{}, fmt.Errorf("target_path %q contains path traversal", targetPath)
	}

	backupPath, err := backupPathForTarget(backupRoot, targetPath)
	if err != nil {
		return BackupItem{}, err
	}

	item := BackupItem{
		TargetPath: targetPath,
		Exists:     false,
		BackupPath: backupPath,
		Operation:  operation,
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return item, nil
		}
		return BackupItem{}, fmt.Errorf("stat target_path %q: %w", targetPath, err)
	}
	if info.IsDir() {
		return BackupItem{}, fmt.Errorf("target_path %q is a directory", targetPath)
	}

	hash, err := fileSHA256(targetPath)
	if err != nil {
		return BackupItem{}, err
	}
	size := info.Size()
	item.Exists = true
	item.SHA256 = &hash
	item.SizeBytes = &size
	return item, nil
}

func backupTargetKey(targetPath string) string {
	return filepath.Clean(strings.TrimSpace(targetPath))
}

func backupPathForTarget(backupRoot string, targetPath string) (string, error) {
	relativeTarget, err := backupRelativeTarget(targetPath)
	if err != nil {
		return "", err
	}
	backupPath := filepath.Join(backupRoot, relativeTarget)
	if !pathWithinRoot(backupPath, backupRoot) {
		return "", fmt.Errorf("backup path %q escapes backup_root %q", backupPath, backupRoot)
	}
	return backupPath, nil
}

func backupRelativeTarget(targetPath string) (string, error) {
	if hasPathTraversal(targetPath) {
		return "", fmt.Errorf("target_path %q contains path traversal", targetPath)
	}
	cleaned := filepath.ToSlash(filepath.Clean(targetPath))
	cleaned = strings.TrimPrefix(cleaned, filepath.VolumeName(cleaned))
	cleaned = strings.TrimLeft(cleaned, "/")
	if cleaned == "" || cleaned == "." {
		return "", fmt.Errorf("target_path %q cannot be mapped to backup_path", targetPath)
	}
	return filepath.FromSlash(cleaned), nil
}

func pathWithinRoot(candidate string, root string) bool {
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func hasPathTraversal(value string) bool {
	value = strings.TrimSpace(filepath.ToSlash(value))
	if value == "" {
		return false
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open target_path %q: %w", path, err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash target_path %q: %w", path, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
