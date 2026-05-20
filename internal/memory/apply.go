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
	ProposalID     string               `json:"proposal_id"`
	Domain         string               `json:"domain"`
	TargetPath     string               `json:"target_path"`
	Operation      MemoryOperation      `json:"operation"`
	Status         ApplyStatus          `json:"status"`
	LintWarnings   []string             `json:"lint_warnings"`
	LintViolations []string             `json:"lint_violations,omitempty"`
	PatchCount     int                  `json:"patch_count"`
	PatchWarnings  []string             `json:"patch_warnings,omitempty"`
	Actions        []ApplyPreviewAction `json:"actions"`
}

type ApplyPreviewAction struct {
	TargetPath  string          `json:"target_path"`
	Operation   MemoryOperation `json:"operation"`
	Description string          `json:"description"`
	Content     string          `json:"content,omitempty"`
	Warning     string          `json:"warning,omitempty"`
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

	preview.Status = ApplyStatusDryRunOK
	preview.PatchCount = len(proposal.Patches)
	for _, patch := range proposal.Patches {
		action := buildApplyPreviewAction(proposal, patch)
		if strings.TrimSpace(patch.Content) == "" {
			warning := fmt.Sprintf("patch for %q has empty content", action.TargetPath)
			action.Warning = warning
			preview.PatchWarnings = append(preview.PatchWarnings, warning)
		}
		preview.Actions = append(preview.Actions, action)
	}

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
