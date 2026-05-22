package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const ApplyPreflightJSONArtifactName = "apply-preflight.json"

type ApplyPreflightStatus string

const (
	ApplyPreflightStatusOK     ApplyPreflightStatus = "preflight_ok"
	ApplyPreflightStatusFailed ApplyPreflightStatus = "failed"
)

type ApplyPreflight struct {
	ProposalID      string                `json:"proposal_id"`
	ApprovalID      string                `json:"approval_id"`
	RunID           string                `json:"run_id"`
	TaskID          string                `json:"task_id"`
	Domain          string                `json:"domain"`
	TargetPath      string                `json:"target_path"`
	Operation       MemoryOperation       `json:"operation"`
	Status          ApplyPreflightStatus  `json:"status"`
	Failures        []string              `json:"failures"`
	LintStatus      LintStatus            `json:"lint_status"`
	LintWarnings    []string              `json:"lint_warnings"`
	LintViolations  []string              `json:"lint_violations"`
	ApplyStatus     ApplyStatus           `json:"apply_status"`
	PatchCount      int                   `json:"patch_count"`
	PatchWarnings   []string              `json:"patch_warnings"`
	PatchViolations []ApplyPatchViolation `json:"patch_violations"`
	CreatedAt       time.Time             `json:"created_at"`
}

type NewApplyPreflightOptions struct {
	CreatedAt time.Time
}

func LoadApprovalFromFile(approvalPath string) (MemoryApproval, error) {
	data, err := os.ReadFile(approvalPath)
	if err != nil {
		return MemoryApproval{}, fmt.Errorf("read memory approval %q: %w", approvalPath, err)
	}
	return ParseApprovalJSON(data)
}

func ParseApprovalJSON(data []byte) (MemoryApproval, error) {
	var approval MemoryApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return MemoryApproval{}, fmt.Errorf("parse memory approval json: %w", err)
	}
	return approval, nil
}

func BuildApplyPreflight(proposal MemoryProposal, approval MemoryApproval, policy *MemoryPolicy, opts NewApplyPreflightOptions) ApplyPreflight {
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	lint := LintProposal(proposal, policy)
	applyPreview, _ := BuildApplyDryRunPreview(proposal, policy)
	preflight := ApplyPreflight{
		ProposalID:      proposal.ProposalID,
		ApprovalID:      approval.ApprovalID,
		RunID:           proposal.RunID,
		TaskID:          proposal.TaskID,
		Domain:          proposal.Domain,
		TargetPath:      proposal.TargetPath,
		Operation:       proposal.Operation,
		Status:          ApplyPreflightStatusOK,
		Failures:        []string{},
		LintStatus:      lint.Status,
		LintWarnings:    cloneApprovalStrings(lint.Warnings),
		LintViolations:  cloneApprovalStrings(lint.Violations),
		ApplyStatus:     applyPreview.Status,
		PatchCount:      applyPreview.PatchCount,
		PatchWarnings:   cloneApprovalStrings(applyPreview.PatchWarnings),
		PatchViolations: cloneApplyPatchViolations(applyPreview.PatchViolations),
		CreatedAt:       createdAt.UTC(),
	}

	if err := approval.Validate(); err != nil {
		preflight.addFailure(fmt.Sprintf("approval artifact invalid: %v", err))
	}
	if approval.ProposalID != proposal.ProposalID {
		preflight.addFailure(fmt.Sprintf("approval proposal_id %q does not match proposal_id %q", approval.ProposalID, proposal.ProposalID))
	}
	if approval.RunID != proposal.RunID {
		preflight.addFailure(fmt.Sprintf("approval run_id %q does not match proposal run_id %q", approval.RunID, proposal.RunID))
	}
	if approval.TaskID != proposal.TaskID {
		preflight.addFailure(fmt.Sprintf("approval task_id %q does not match proposal task_id %q", approval.TaskID, proposal.TaskID))
	}
	if approval.Domain != proposal.Domain {
		preflight.addFailure(fmt.Sprintf("approval domain %q does not match proposal domain %q", approval.Domain, proposal.Domain))
	}
	if approval.TargetPath != proposal.TargetPath {
		preflight.addFailure(fmt.Sprintf("approval target_path %q does not match proposal target_path %q", approval.TargetPath, proposal.TargetPath))
	}
	if approval.Operation != proposal.Operation {
		preflight.addFailure(fmt.Sprintf("approval operation %q does not match proposal operation %q", approval.Operation, proposal.Operation))
	}
	if approval.Decision != DecisionApproved {
		preflight.addFailure(fmt.Sprintf("approval decision must be %q, got %q", DecisionApproved, approval.Decision))
	}
	if approval.LintStatus != LintStatusOK {
		preflight.addFailure(fmt.Sprintf("approval lint_status must be %q, got %q", LintStatusOK, approval.LintStatus))
	}
	if approval.ApplyStatus != ApplyStatusDryRunOK {
		preflight.addFailure(fmt.Sprintf("approval apply_status must be %q, got %q", ApplyStatusDryRunOK, approval.ApplyStatus))
	}
	if len(approval.PatchViolations) > 0 {
		preflight.addFailure(fmt.Sprintf("approval patch_violations must be empty, got %d", len(approval.PatchViolations)))
	}
	if lint.Status != LintStatusOK {
		preflight.addFailure(fmt.Sprintf("current lint_status must be %q, got %q", LintStatusOK, lint.Status))
	}
	if applyPreview.Status != ApplyStatusDryRunOK {
		preflight.addFailure(fmt.Sprintf("current apply_status must be %q, got %q", ApplyStatusDryRunOK, applyPreview.Status))
	}
	if len(applyPreview.PatchViolations) > 0 {
		preflight.addFailure(fmt.Sprintf("current patch_violations must be empty, got %d", len(applyPreview.PatchViolations)))
	}
	if len(preflight.Failures) > 0 {
		preflight.Status = ApplyPreflightStatusFailed
	}
	return preflight
}

func (p *ApplyPreflight) addFailure(failure string) {
	p.Failures = append(p.Failures, failure)
}

func (p ApplyPreflight) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
