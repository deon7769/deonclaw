package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/memory"
	"github.com/deon7769/deonclaw/internal/workers"
)

const (
	memoryProposalStatusMissing    = "missing"
	memoryProposalStatusNotChecked = "not_checked"
	memoryProposalStatusOK         = "ok"
	memoryProposalStatusFailed     = "failed"
)

type memoryProposalCheck struct {
	Status     string
	ProposalID string
	Violations []string
	Warnings   []string
	LintJSON   []byte
}

func checkMemoryProposal(runDir string, result *workers.RunResult, memoryPolicyPath string) (memoryProposalCheck, error) {
	check := memoryProposalCheck{Status: memoryProposalStatusNotChecked}
	if strings.TrimSpace(memoryPolicyPath) == "" {
		return check, nil
	}

	proposalJSON, ok, err := findMemoryProposalJSON(runDir, result)
	if err != nil {
		return memoryProposalCheck{}, err
	}
	if !ok {
		return memoryProposalCheck{Status: memoryProposalStatusMissing}, nil
	}

	policy, err := memory.LoadPolicyFromFile(memoryPolicyPath)
	if err != nil {
		return memoryProposalCheck{}, err
	}

	proposal, err := memory.ParseProposalJSON(proposalJSON)
	if err != nil {
		check = memoryProposalCheck{
			Status:     memoryProposalStatusFailed,
			Violations: []string{err.Error()},
		}
		return withMemoryProposalLintJSON(check)
	}

	lintResult := memory.LintProposal(proposal, policy)
	check = memoryProposalCheck{
		Status:     string(lintResult.Status),
		ProposalID: lintResult.ProposalID,
		Violations: lintResult.Violations,
		Warnings:   lintResult.Warnings,
	}
	return withMemoryProposalLintJSON(check)
}

func findMemoryProposalJSON(runDir string, result *workers.RunResult) ([]byte, bool, error) {
	if result != nil {
		if content, ok := workerArtifactContent(result.Artifacts, memory.ProposalJSONArtifactName); ok {
			return content, true, nil
		}
	}

	path := filepath.Join(runDir, memory.ProposalJSONArtifactName)
	content, err := os.ReadFile(path)
	if err == nil {
		return content, true, nil
	}
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("read memory proposal artifact %q: %w", path, err)
}

func withMemoryProposalLintJSON(check memoryProposalCheck) (memoryProposalCheck, error) {
	data, err := json.MarshalIndent(struct {
		ProposalID string   `json:"proposal_id"`
		Status     string   `json:"status"`
		Violations []string `json:"violations"`
		Warnings   []string `json:"warnings"`
	}{
		ProposalID: check.ProposalID,
		Status:     check.Status,
		Violations: check.Violations,
		Warnings:   check.Warnings,
	}, "", "  ")
	if err != nil {
		return memoryProposalCheck{}, err
	}
	check.LintJSON = append(data, '\n')
	return check, nil
}
