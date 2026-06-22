package insights

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	ApplyStatusOK      = "ok"
	ApplyStatusBlocked = "blocked"
	ApplyStatusFailed  = "failed"

	ApplyModeDryRun               = "dry_run"
	ApplyModeDocumentationPreview = "documentation_preview"
)

type ApplyDryRunResult struct {
	ProposalID           string `json:"proposal_id"`
	ProposalType         string `json:"proposal_type"`
	Status               string `json:"status"`
	WouldApply           bool   `json:"would_apply"`
	ApplyMode            string `json:"apply_mode"`
	Target               string `json:"target"`
	PreviewPath          string `json:"preview_path,omitempty"`
	BlockedReason        string `json:"blocked_reason,omitempty"`
	ApprovalRequired     bool   `json:"approval_required"`
	ApprovalBound        bool   `json:"approval_bound"`
	ApprovalDecision     string `json:"approval_decision,omitempty"`
	EvidenceBundleSHA256 string `json:"evidence_bundle_sha256"`
	CreatedAt            string `json:"created_at"`
}

type ApplyExecuteResult struct {
	ApplyDryRunResult
	Executed          bool   `json:"executed"`
	AppliedArtifact   string `json:"applied_artifact,omitempty"`
	ProposalStatus    string `json:"proposal_status"`
	ReversiblePreview bool   `json:"reversible_preview"`
}

type ApplyOptions struct {
	Proposal LearningProposal
	Approval LearningProposalApproval
	Now      time.Time
}

type ApplyExecuteOptions struct {
	ApplyOptions
	PreviewOutputPath string
	ConfirmApply      bool
}

func ApplyDryRun(opts ApplyOptions) (ApplyDryRunResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	result := ApplyDryRunResult{
		ProposalID:           opts.Proposal.ProposalID,
		ProposalType:         opts.Proposal.Type,
		Status:               ApplyStatusBlocked,
		ApplyMode:            ApplyModeDryRun,
		Target:               opts.Proposal.Target,
		ApprovalRequired:     opts.Proposal.ApprovalRequired,
		EvidenceBundleSHA256: opts.Proposal.EvidenceBundleSHA256,
		CreatedAt:            now.Format(time.RFC3339Nano),
	}

	if err := ValidateProposal(opts.Proposal); err != nil {
		result.BlockedReason = err.Error()
		return result, nil
	}

	if opts.Proposal.ApprovalRequired {
		if strings.TrimSpace(opts.Approval.ApprovalID) == "" {
			result.BlockedReason = "approval artifact is required"
			return result, nil
		}
		if err := ValidateApprovalAgainstProposal(opts.Approval, opts.Proposal); err != nil {
			result.BlockedReason = err.Error()
			return result, nil
		}
		result.ApprovalBound = true
		result.ApprovalDecision = opts.Approval.Decision
		if opts.Approval.Decision != ApprovalDecisionApproved {
			result.BlockedReason = "approval decision is not approved"
			return result, nil
		}
	}

	switch opts.Proposal.Type {
	case ProposalTypeDocumentation, ProposalTypeFollowUpTask, ProposalTypeEvalCase, ProposalTypeTechnicalDebt:
		result.Status = ApplyStatusOK
		result.WouldApply = true
		result.ApplyMode = ApplyModeDocumentationPreview
	case ProposalTypeSkillPatch, ProposalTypeSkillCreate:
		if strings.TrimSpace(opts.Proposal.PatchPath) == "" {
			result.BlockedReason = "skill proposals require patch_path before apply"
			return result, nil
		}
		result.Status = ApplyStatusOK
		result.WouldApply = false
		result.BlockedReason = "skill apply is not enabled in this sprint; patch_path recorded for future registry integration"
	case ProposalTypeMemoryAdd, ProposalTypeMemoryReplace:
		result.BlockedReason = "memory proposals must use memory proposal workflow, not direct insight apply"
	case ProposalTypeRule, ProposalTypeWorkflow, ProposalTypeRoutingPolicy, ProposalTypeBudgetPolicy:
		result.BlockedReason = fmt.Sprintf("proposal type %q requires dedicated policy workflow", opts.Proposal.Type)
	default:
		result.BlockedReason = fmt.Sprintf("proposal type %q is not supported for apply", opts.Proposal.Type)
	}

	return result, nil
}

func ApplyExecute(opts ApplyExecuteOptions) (ApplyExecuteResult, error) {
	dryRun, err := ApplyDryRun(opts.ApplyOptions)
	if err != nil {
		return ApplyExecuteResult{}, err
	}

	result := ApplyExecuteResult{
		ApplyDryRunResult: dryRun,
		Executed:          false,
		ProposalStatus:    opts.Proposal.Status,
		ReversiblePreview: true,
	}

	if !opts.ConfirmApply {
		result.BlockedReason = "apply execute requires --confirm-apply"
		result.Status = ApplyStatusBlocked
		result.WouldApply = false
		return result, nil
	}
	if dryRun.Status != ApplyStatusOK || !dryRun.WouldApply {
		result.BlockedReason = dryRun.BlockedReason
		return result, nil
	}

	previewPath := strings.TrimSpace(opts.PreviewOutputPath)
	if previewPath == "" {
		return ApplyExecuteResult{}, errors.New("preview output path is required for apply execute")
	}

	content := buildApplyPreviewMarkdown(opts.Proposal, opts.Approval)
	if err := os.MkdirAll(filepath.Dir(previewPath), 0o755); err != nil {
		return ApplyExecuteResult{}, fmt.Errorf("create preview dir: %w", err)
	}
	if err := os.WriteFile(previewPath, []byte(content), 0o644); err != nil {
		return ApplyExecuteResult{}, fmt.Errorf("write apply preview %q: %w", previewPath, err)
	}

	result.Executed = true
	result.AppliedArtifact = previewPath
	result.PreviewPath = previewPath
	result.ProposalStatus = ProposalStatusApplied
	result.ApplyMode = ApplyModeDocumentationPreview
	return result, nil
}

func buildApplyPreviewMarkdown(proposal LearningProposal, approval LearningProposalApproval) string {
	var builder strings.Builder
	builder.WriteString("# Learning proposal apply preview\n\n")
	builder.WriteString("This artifact is a governed preview only. It does not mutate canonical targets.\n\n")
	fmt.Fprintf(&builder, "- proposal_id: %s\n", proposal.ProposalID)
	fmt.Fprintf(&builder, "- type: %s\n", proposal.Type)
	fmt.Fprintf(&builder, "- target: %s\n", proposal.Target)
	fmt.Fprintf(&builder, "- evidence_bundle_sha256: %s\n", proposal.EvidenceBundleSHA256)
	if strings.TrimSpace(approval.ApprovalID) != "" {
		fmt.Fprintf(&builder, "- approval_id: %s\n", approval.ApprovalID)
		fmt.Fprintf(&builder, "- reviewer: %s\n", approval.Reviewer)
	}
	builder.WriteString("\n## Reason\n\n")
	builder.WriteString(proposal.Reason)
	builder.WriteString("\n\n## Proposed change\n\n")
	builder.WriteString(proposal.ProposedChangeSummary)
	builder.WriteString("\n")
	return builder.String()
}

func WriteApplyDryRunJSON(result ApplyDryRunResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal apply dry-run: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write apply dry-run %q: %w", path, err)
	}
	return nil
}

func WriteApplyExecuteJSON(result ApplyExecuteResult, path string) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal apply result: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write apply result %q: %w", path, err)
	}
	return nil
}

func ReadApplyDryRunJSON(path string) (ApplyDryRunResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ApplyDryRunResult{}, fmt.Errorf("read apply dry-run %q: %w", path, err)
	}
	var result ApplyDryRunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ApplyDryRunResult{}, fmt.Errorf("parse apply dry-run %q: %w", path, err)
	}
	return result, nil
}
