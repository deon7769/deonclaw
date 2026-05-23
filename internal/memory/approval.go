package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const ApprovalJSONArtifactName = "memory-approval.json"

type ApprovalDecision string

const (
	DecisionApproved ApprovalDecision = "approved"
	DecisionRejected ApprovalDecision = "rejected"
)

type MemoryApproval struct {
	ApprovalID         string                `json:"approval_id"`
	ProposalID         string                `json:"proposal_id"`
	ProposalSHA256     string                `json:"proposal_sha256"`
	ApplyPreviewSHA256 string                `json:"apply_preview_sha256"`
	RunID              string                `json:"run_id"`
	TaskID             string                `json:"task_id"`
	Domain             string                `json:"domain"`
	TargetPath         string                `json:"target_path"`
	Operation          MemoryOperation       `json:"operation"`
	Reviewer           string                `json:"reviewer"`
	Decision           ApprovalDecision      `json:"decision"`
	Reason             string                `json:"reason"`
	LintStatus         LintStatus            `json:"lint_status"`
	LintWarnings       []string              `json:"lint_warnings"`
	LintViolations     []string              `json:"lint_violations"`
	ApplyStatus        ApplyStatus           `json:"apply_status"`
	PatchCount         int                   `json:"patch_count"`
	PatchWarnings      []string              `json:"patch_warnings"`
	PatchViolations    []ApplyPatchViolation `json:"patch_violations"`
	CreatedAt          time.Time             `json:"created_at"`
}

type NewApprovalOptions struct {
	ApprovalID string
	Reviewer   string
	Decision   ApprovalDecision
	Reason     string
	CreatedAt  time.Time
}

func BuildApproval(proposal MemoryProposal, policy *MemoryPolicy, opts NewApprovalOptions) (MemoryApproval, error) {
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	approvalID := strings.TrimSpace(opts.ApprovalID)
	if approvalID == "" {
		approvalID = "mem-approval-" + createdAt.UTC().Format("20060102T150405.000000000Z")
	}

	lint := LintProposal(proposal, policy)
	proposalSHA256, err := canonicalProposalSHA256(proposal)
	if err != nil {
		return MemoryApproval{}, fmt.Errorf("hash memory proposal: %w", err)
	}

	applyPreview, _ := BuildApplyDryRunPreview(proposal, policy)
	applyPreviewSHA256, err := canonicalApplyPreviewSHA256(applyPreview)
	if err != nil {
		return MemoryApproval{}, fmt.Errorf("hash apply preview: %w", err)
	}

	approval := MemoryApproval{
		ApprovalID:         approvalID,
		ProposalID:         proposal.ProposalID,
		ProposalSHA256:     proposalSHA256,
		ApplyPreviewSHA256: applyPreviewSHA256,
		RunID:              proposal.RunID,
		TaskID:             proposal.TaskID,
		Domain:             proposal.Domain,
		TargetPath:         proposal.TargetPath,
		Operation:          proposal.Operation,
		Reviewer:           strings.TrimSpace(opts.Reviewer),
		Decision:           ApprovalDecision(strings.TrimSpace(string(opts.Decision))),
		Reason:             strings.TrimSpace(opts.Reason),
		LintStatus:         lint.Status,
		LintWarnings:       cloneApprovalStrings(lint.Warnings),
		LintViolations:     cloneApprovalStrings(lint.Violations),
		ApplyStatus:        applyPreview.Status,
		PatchCount:         applyPreview.PatchCount,
		PatchWarnings:      cloneApprovalStrings(applyPreview.PatchWarnings),
		PatchViolations:    cloneApplyPatchViolations(applyPreview.PatchViolations),
		CreatedAt:          createdAt.UTC(),
	}

	if err := approval.Validate(); err != nil {
		return MemoryApproval{}, err
	}
	if approval.Decision == DecisionApproved && lint.Status != LintStatusOK {
		return MemoryApproval{}, fmt.Errorf("approved decision requires proposal lint status ok, got %s", lint.Status)
	}
	if approval.Decision == DecisionApproved && (approval.ApplyStatus != ApplyStatusDryRunOK || len(approval.PatchViolations) > 0) {
		return MemoryApproval{}, fmt.Errorf("approved decision requires apply_status dry_run_ok with no patch violations, got %s with %d patch violations", approval.ApplyStatus, len(approval.PatchViolations))
	}
	return approval, nil
}

func (a MemoryApproval) Validate() error {
	var errs []error
	requireNonEmpty := func(field string, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}

	requireNonEmpty("approval_id", a.ApprovalID)
	requireNonEmpty("proposal_id", a.ProposalID)
	requireNonEmpty("proposal_sha256", a.ProposalSHA256)
	requireNonEmpty("apply_preview_sha256", a.ApplyPreviewSHA256)
	requireNonEmpty("run_id", a.RunID)
	requireNonEmpty("task_id", a.TaskID)
	requireNonEmpty("domain", a.Domain)
	requireNonEmpty("target_path", a.TargetPath)
	requireNonEmpty("operation", string(a.Operation))
	requireNonEmpty("reviewer", a.Reviewer)
	requireNonEmpty("reason", a.Reason)
	if !validApprovalDecision(a.Decision) {
		errs = append(errs, fmt.Errorf("decision %q is not supported", a.Decision))
	}
	if !validLintStatus(a.LintStatus) {
		errs = append(errs, fmt.Errorf("lint_status %q is not supported", a.LintStatus))
	}
	if !validApplyStatus(a.ApplyStatus) {
		errs = append(errs, fmt.Errorf("apply_status %q is not supported", a.ApplyStatus))
	}
	if a.PatchCount < 0 {
		errs = append(errs, fmt.Errorf("patch_count cannot be negative"))
	}
	if a.CreatedAt.IsZero() {
		errs = append(errs, fmt.Errorf("created_at is required"))
	}
	if len(errs) > 0 {
		return joinApprovalErrors(errs)
	}
	return nil
}

func (a MemoryApproval) JSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validApprovalDecision(decision ApprovalDecision) bool {
	switch decision {
	case DecisionApproved, DecisionRejected:
		return true
	default:
		return false
	}
}

func validLintStatus(status LintStatus) bool {
	return status == LintStatusOK || status == LintStatusFailed
}

func validApplyStatus(status ApplyStatus) bool {
	return status == ApplyStatusDryRunOK || status == ApplyStatusFailed
}

func cloneApprovalStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return append([]string{}, values...)
}

func cloneApplyPatchViolations(values []ApplyPatchViolation) []ApplyPatchViolation {
	if values == nil {
		return []ApplyPatchViolation{}
	}
	return append([]ApplyPatchViolation{}, values...)
}

func joinApprovalErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	var parts []string
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return errors.New(strings.Join(parts, "; "))
}
