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
	ProposalID  string        `json:"proposal_id"`
	ApprovalID  string        `json:"approval_id"`
	RunID       string        `json:"run_id"`
	TaskID      string        `json:"task_id"`
	Domain      string        `json:"domain"`
	BackupRoot  string        `json:"backup_root"`
	Entries     []BackupEntry `json:"entries"`
	RestorePlan RestorePlan   `json:"restore_plan"`
	CreatedAt   time.Time     `json:"created_at"`
}

type BackupEntry struct {
	TargetPath string          `json:"target_path"`
	Exists     bool            `json:"exists"`
	BackupPath string          `json:"backup_path"`
	SHA256     *string         `json:"sha256,omitempty"`
	SizeBytes  *int64          `json:"size_bytes,omitempty"`
	Operation  MemoryOperation `json:"operation"`
}

type RestorePlan struct {
	ProposalID string         `json:"proposal_id"`
	ApprovalID string         `json:"approval_id"`
	Entries    []RestoreEntry `json:"entries"`
}

type RestoreEntry struct {
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
	backupRoot, err := cleanBackupRoot(opts.BackupRoot)
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
		Entries:    []BackupEntry{},
		CreatedAt:  createdAt.UTC(),
	}
	restore := RestorePlan{
		ProposalID: proposal.ProposalID,
		ApprovalID: approval.ApprovalID,
		Entries:    []RestoreEntry{},
	}

	for _, action := range preview.Actions {
		entry, err := buildBackupEntry(backupRoot, action.TargetPath, action.Operation)
		if err != nil {
			return BackupPlan{}, err
		}
		plan.Entries = append(plan.Entries, entry)
		restore.Entries = append(restore.Entries, RestoreEntry{
			TargetPath: entry.TargetPath,
			BackupPath: entry.BackupPath,
			Exists:     entry.Exists,
			SHA256:     cloneStringPointer(entry.SHA256),
			SizeBytes:  cloneInt64Pointer(entry.SizeBytes),
			Operation:  entry.Operation,
		})
	}
	plan.RestorePlan = restore
	return plan, nil
}

func (p BackupPlan) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
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

func buildBackupEntry(backupRoot string, targetPath string, operation MemoryOperation) (BackupEntry, error) {
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		return BackupEntry{}, fmt.Errorf("target_path is required")
	}
	if hasPathTraversal(targetPath) {
		return BackupEntry{}, fmt.Errorf("target_path %q contains path traversal", targetPath)
	}

	backupPath, err := backupPathForTarget(backupRoot, targetPath)
	if err != nil {
		return BackupEntry{}, err
	}

	entry := BackupEntry{
		TargetPath: targetPath,
		Exists:     false,
		BackupPath: backupPath,
		Operation:  operation,
	}

	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return entry, nil
		}
		return BackupEntry{}, fmt.Errorf("stat target_path %q: %w", targetPath, err)
	}
	if info.IsDir() {
		return BackupEntry{}, fmt.Errorf("target_path %q is a directory", targetPath)
	}

	hash, err := fileSHA256(targetPath)
	if err != nil {
		return BackupEntry{}, err
	}
	size := info.Size()
	entry.Exists = true
	entry.SHA256 = &hash
	entry.SizeBytes = &size
	return entry, nil
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
