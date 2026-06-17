package runner

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/mcpconfig"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

const (
	mcpToolProposalArtifactName          = "mcp-tool-call-proposal.json"
	mcpToolProposalLintArtifactName      = "mcp-tool-call-proposal-lint.json"
	mcpToolProposalPreflightArtifactName = "mcp-tool-call-preflight.json"

	mcpToolProposalStatusNone            = "none"
	mcpToolProposalStatusValid           = "valid"
	mcpToolProposalStatusInvalid         = "invalid"
	mcpToolProposalStatusPreflightFailed = "preflight_failed"
	mcpToolProposalStatusPreflightPassed = "preflight_passed"
)

var refusedMCPApprovalArtifactNames = []string{
	"mcp-tool-call-approval.json",
	"mcp-call-approval.json",
}

type mcpToolProposalCheck struct {
	Status         string
	ProposalID     string
	ProposalSHA256 string
	LintJSON       []byte
	PreflightJSON  []byte
	Violations     []string
	Warnings       []string
}

func checkMCPToolProposal(result *workers.RunResult, policySpec tasks.MCPProposalPolicySpec) (mcpToolProposalCheck, error) {
	check := mcpToolProposalCheck{Status: mcpToolProposalStatusNone}
	if result == nil {
		return check, nil
	}

	proposalJSON, hasProposal := workerArtifactContent(result.Artifacts, mcpToolProposalArtifactName)
	if hasProposal {
		check.ProposalSHA256 = sha256Hex(proposalJSON)
	}
	if approvalName, ok := findRefusedMCPApprovalArtifact(result); ok {
		check.Status = mcpToolProposalStatusInvalid
		check.Violations = []string{fmt.Sprintf("worker emitted MCP approval artifact %q; workers cannot approve MCP calls", approvalName)}
		check.LintJSON = mustMCPToolProposalLintJSON(check, "invalid")
		return check, fmt.Errorf("MCP proposal approval artifact refused: %s", approvalName)
	}
	if !hasProposal {
		return check, nil
	}

	proposal, err := mcpapproval.ParseProposalJSON(proposalJSON)
	if err != nil {
		check.Status = mcpToolProposalStatusInvalid
		check.Violations = []string{err.Error()}
		check.LintJSON = mustMCPToolProposalLintJSON(check, "invalid")
		return check, fmt.Errorf("MCP tool proposal invalid: %w", err)
	}
	check.ProposalID = proposal.ID
	if err := proposal.Validate(); err != nil {
		check.Status = mcpToolProposalStatusInvalid
		check.Violations = []string{err.Error()}
		check.LintJSON = mustMCPToolProposalLintJSON(check, "invalid")
		return check, fmt.Errorf("MCP tool proposal invalid: %w", err)
	}

	if !usesMCPProposalPolicy(policySpec) {
		check.Status = mcpToolProposalStatusValid
		check.Warnings = []string{"skipped_policy: mcp_proposal_policy is not configured; proposal was not preflighted"}
		check.LintJSON = mustMCPToolProposalLintJSON(check, "skipped_policy")
		return check, nil
	}

	validationOpts, err := loadMCPProposalValidationOptions(policySpec)
	if err != nil {
		check.Status = mcpToolProposalStatusPreflightFailed
		check.Violations = []string{err.Error()}
		check.LintJSON = mustMCPToolProposalLintJSON(check, mcpapproval.PreflightStatusFailed)
		check.PreflightJSON = mustMCPToolProposalPreflightFailureJSON(proposal.ID, err.Error())
		if policySpec.RequirePreflight {
			return check, fmt.Errorf("MCP proposal preflight failed: %w", err)
		}
		return check, nil
	}

	lint := mcpapproval.LintProposal(proposal, validationOpts)
	check.Violations = append([]string(nil), lint.Violations...)
	check.Warnings = append([]string(nil), lint.Warnings...)
	lintJSON, err := json.MarshalIndent(lint, "", "  ")
	if err != nil {
		return mcpToolProposalCheck{}, err
	}
	check.LintJSON = append(lintJSON, '\n')

	preflight := mcpapproval.BuildPreflight(proposal, validationOpts)
	preflightJSON, err := preflight.JSON()
	if err != nil {
		return mcpToolProposalCheck{}, err
	}
	check.PreflightJSON = preflightJSON
	if preflight.Status == mcpapproval.PreflightStatusPassed {
		check.Status = mcpToolProposalStatusPreflightPassed
		return check, nil
	}

	check.Status = mcpToolProposalStatusPreflightFailed
	check.Violations = append([]string(nil), preflight.Failures...)
	if policySpec.RequirePreflight {
		return check, fmt.Errorf("MCP proposal preflight failed")
	}
	return check, nil
}

func loadMCPProposalValidationOptions(policySpec tasks.MCPProposalPolicySpec) (mcpapproval.ValidationOptions, error) {
	cfg, err := mcpconfig.Load(policySpec.Config)
	if err != nil {
		return mcpapproval.ValidationOptions{}, err
	}
	policy, err := mcpsmoke.LoadCallPolicy(policySpec.Policy)
	if err != nil {
		return mcpapproval.ValidationOptions{}, err
	}

	var runtimeCfg *runtimeconfig.Config
	if strings.TrimSpace(policySpec.RuntimeConfig) != "" {
		cfg, err := runtimeconfig.Load(policySpec.RuntimeConfig)
		if err != nil {
			return mcpapproval.ValidationOptions{}, err
		}
		runtimeCfg = &cfg
	}
	return mcpapproval.ValidationOptions{
		Config:        cfg,
		Policy:        policy,
		RuntimeConfig: runtimeCfg,
	}, nil
}

func findRefusedMCPApprovalArtifact(result *workers.RunResult) (string, bool) {
	if result == nil {
		return "", false
	}
	for _, name := range refusedMCPApprovalArtifactNames {
		if _, ok := workerArtifactContent(result.Artifacts, name); ok {
			return name, true
		}
	}
	return "", false
}

func usesMCPProposalPolicy(policySpec tasks.MCPProposalPolicySpec) bool {
	return strings.TrimSpace(policySpec.Config) != "" ||
		strings.TrimSpace(policySpec.Policy) != "" ||
		strings.TrimSpace(policySpec.RuntimeConfig) != "" ||
		policySpec.RequirePreflight
}

func mustMCPToolProposalLintJSON(check mcpToolProposalCheck, status string) []byte {
	data, err := json.MarshalIndent(struct {
		ProposalID     string   `json:"proposal_id,omitempty"`
		Status         string   `json:"status"`
		ProposalSHA256 string   `json:"proposal_sha256,omitempty"`
		Violations     []string `json:"violations"`
		Warnings       []string `json:"warnings"`
	}{
		ProposalID:     check.ProposalID,
		Status:         status,
		ProposalSHA256: check.ProposalSHA256,
		Violations:     check.Violations,
		Warnings:       check.Warnings,
	}, "", "  ")
	if err != nil {
		return []byte("{\"status\":\"invalid\",\"violations\":[\"marshal lint json failed\"]}\n")
	}
	return append(data, '\n')
}

func mustMCPToolProposalPreflightFailureJSON(proposalID string, failure string) []byte {
	data, err := json.MarshalIndent(mcpapproval.MCPToolCallPreflight{
		ProposalID: proposalID,
		Status:     mcpapproval.PreflightStatusFailed,
		Warnings:   []string{},
		Failures:   []string{failure},
	}, "", "  ")
	if err != nil {
		return []byte("{\"status\":\"failed\",\"failures\":[\"marshal preflight json failed\"]}\n")
	}
	return append(data, '\n')
}
