package memory

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ApplyStatus string

const (
	ApplyStatusDryRunOK ApplyStatus = "dry_run_ok"
	ApplyStatusFailed   ApplyStatus = "failed"
)

type ApplyPreview struct {
	ProposalID      string                `json:"proposal_id"`
	Domain          string                `json:"domain"`
	TargetPath      string                `json:"target_path"`
	Operation       MemoryOperation       `json:"operation"`
	Status          ApplyStatus           `json:"status"`
	LintWarnings    []string              `json:"lint_warnings"`
	LintViolations  []string              `json:"lint_violations,omitempty"`
	PatchCount      int                   `json:"patch_count"`
	PatchWarnings   []string              `json:"patch_warnings,omitempty"`
	PatchViolations []ApplyPatchViolation `json:"patch_violations,omitempty"`
	Actions         []ApplyPreviewAction  `json:"actions"`
}

type ApplyPatchViolation struct {
	PatchIndex int             `json:"patch_index"`
	TargetPath string          `json:"target_path"`
	Operation  MemoryOperation `json:"operation"`
	Violation  string          `json:"violation"`
}

type ApplyPreviewAction struct {
	TargetPath  string          `json:"target_path"`
	Operation   MemoryOperation `json:"operation"`
	Description string          `json:"description"`
	Content     string          `json:"content,omitempty"`
	Warning     string          `json:"warning,omitempty"`
	Warnings    []string        `json:"warnings,omitempty"`
	Violations  []string        `json:"violations,omitempty"`
}

func BuildApplyDryRunPreview(proposal MemoryProposal, policy *MemoryPolicy) (ApplyPreview, error) {
	preview := ApplyPreview{
		ProposalID: proposal.ProposalID,
		Domain:     proposal.Domain,
		TargetPath: proposal.TargetPath,
		Operation:  proposal.Operation,
		Status:     ApplyStatusFailed,
	}

	lint := LintProposal(proposal, policy)
	preview.LintWarnings = append(preview.LintWarnings, lint.Warnings...)
	preview.LintViolations = append(preview.LintViolations, lint.Violations...)
	if lint.Status == LintStatusFailed {
		return preview, fmt.Errorf("memory proposal lint failed")
	}

	preview.PatchCount = len(proposal.Patches)
	for index, patch := range proposal.Patches {
		action := buildApplyPreviewAction(proposal, patch)
		patchLint := lintApplyPatch(proposal, action, policy)
		for _, warning := range patchLint.Warnings {
			addApplyPatchWarning(&preview, &action, index, warning)
		}
		for _, violation := range patchLint.Violations {
			addApplyPatchViolation(&preview, &action, index, violation)
		}
		if strings.TrimSpace(patch.Content) == "" {
			warning := fmt.Sprintf("patch for %q has empty content", action.TargetPath)
			addApplyPatchWarning(&preview, &action, index, warning)
		}
		preview.Actions = append(preview.Actions, action)
	}

	if len(preview.PatchViolations) > 0 {
		return preview, fmt.Errorf("memory proposal patch lint failed")
	}

	preview.Status = ApplyStatusDryRunOK
	return preview, nil
}

func (p ApplyPreview) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func buildApplyPreviewAction(proposal MemoryProposal, patch MemoryPatch) ApplyPreviewAction {
	targetPath := strings.TrimSpace(patch.TargetPath)
	if targetPath == "" {
		targetPath = proposal.TargetPath
	}

	operation := patch.Operation
	if operation == "" {
		operation = proposal.Operation
	}

	return ApplyPreviewAction{
		TargetPath:  targetPath,
		Operation:   operation,
		Description: applyPreviewDescription(operation),
		Content:     patch.Content,
	}
}

func lintApplyPatch(proposal MemoryProposal, action ApplyPreviewAction, policy *MemoryPolicy) LintResult {
	patchProposal := proposal
	patchProposal.TargetPath = action.TargetPath
	patchProposal.Operation = action.Operation
	return LintProposal(patchProposal, policy)
}

func addApplyPatchWarning(preview *ApplyPreview, action *ApplyPreviewAction, patchIndex int, warning string) {
	formatted := fmt.Sprintf("patch[%d]: %s", patchIndex, warning)
	action.Warnings = append(action.Warnings, formatted)
	if action.Warning == "" {
		action.Warning = formatted
	}
	preview.PatchWarnings = append(preview.PatchWarnings, formatted)
}

func addApplyPatchViolation(preview *ApplyPreview, action *ApplyPreviewAction, patchIndex int, violation string) {
	formatted := fmt.Sprintf("patch[%d]: %s", patchIndex, violation)
	action.Violations = append(action.Violations, formatted)
	preview.PatchViolations = append(preview.PatchViolations, ApplyPatchViolation{
		PatchIndex: patchIndex,
		TargetPath: action.TargetPath,
		Operation:  action.Operation,
		Violation:  violation,
	})
}

func applyPreviewDescription(operation MemoryOperation) string {
	switch operation {
	case OperationAppend:
		return "would append content"
	case OperationCreate:
		return "would create file"
	case OperationUpdate:
		return "would replace or update file"
	case OperationArchive:
		return "would archive file"
	default:
		return "would process file"
	}
}
