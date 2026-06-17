package mcpproposalqueue

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
)

const (
	ProposalArtifactName          = "mcp-tool-call-proposal.json"
	ProposalLintArtifactName      = "mcp-tool-call-proposal-lint.json"
	ProposalPreflightArtifactName = "mcp-tool-call-preflight.json"

	StatusValid           = "valid"
	StatusInvalid         = "invalid"
	StatusPreflightPassed = "preflight_passed"
	StatusPreflightFailed = "preflight_failed"
	StatusRefused         = "refused"
)

var workerApprovalArtifactNames = []string{
	"mcp-tool-call-approval.json",
	"mcp-call-approval.json",
}

type ListOptions struct {
	Status string
	Worker string
	TaskID string
}

type ListEntry struct {
	RunID           string `json:"run_id"`
	TaskID          string `json:"task_id"`
	Worker          string `json:"worker"`
	ProposalStatus  string `json:"proposal_status"`
	ProposalSHA256  string `json:"proposal_sha256,omitempty"`
	ProposalID      string `json:"proposal_id,omitempty"`
	Server          string `json:"server,omitempty"`
	Tool            string `json:"tool,omitempty"`
	PreflightStatus string `json:"preflight_status,omitempty"`
	ArtifactPath    string `json:"artifact_path,omitempty"`
	ApprovalRefused bool   `json:"approval_refused,omitempty"`
}

type ListResult struct {
	Proposals []ListEntry `json:"proposals"`
}

type ShowDetail struct {
	RunID           string   `json:"run_id"`
	TaskID          string   `json:"task_id"`
	Worker          string   `json:"worker"`
	ProposalStatus  string   `json:"proposal_status"`
	ProposalID      string   `json:"proposal_id,omitempty"`
	Server          string   `json:"server,omitempty"`
	Tool            string   `json:"tool,omitempty"`
	ArgumentsSHA256 string   `json:"arguments_sha256,omitempty"`
	ProposalSHA256  string   `json:"proposal_sha256,omitempty"`
	PolicyPath      string   `json:"policy_path,omitempty"`
	ConfigPath      string   `json:"config_path,omitempty"`
	Runtime         string   `json:"runtime,omitempty"`
	RuntimeConfig   string   `json:"runtime_config_path,omitempty"`
	Workspace       string   `json:"workspace,omitempty"`
	PreflightStatus string   `json:"preflight_status,omitempty"`
	Violations      []string `json:"violations,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	PreflightFails  []string `json:"preflight_failures,omitempty"`
	ApprovalRefused bool     `json:"approval_refused,omitempty"`
	ApprovalName    string   `json:"approval_artifact_name,omitempty"`
	ArtifactPath    string   `json:"artifact_path,omitempty"`
	LintPath        string   `json:"lint_artifact_path,omitempty"`
	PreflightPath   string   `json:"preflight_artifact_path,omitempty"`
	Suggested       []string `json:"suggested_commands,omitempty"`
}

func List(ctx context.Context, db store.Store, opts ListOptions) (ListResult, error) {
	runRecords, err := db.ListRuns(ctx)
	if err != nil {
		return ListResult{}, err
	}

	var entries []ListEntry
	for _, runRecord := range runRecords {
		if opts.Worker != "" && runRecord.Worker != opts.Worker {
			continue
		}
		if opts.TaskID != "" && runRecord.TaskID != opts.TaskID {
			continue
		}

		entry, ok, err := buildListEntry(ctx, db, runRecord)
		if err != nil {
			return ListResult{}, err
		}
		if !ok {
			continue
		}
		if opts.Status != "" && entry.ProposalStatus != opts.Status {
			continue
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].RunID == entries[j].RunID {
			return entries[i].ProposalID < entries[j].ProposalID
		}
		return entries[i].RunID < entries[j].RunID
	})
	return ListResult{Proposals: entries}, nil
}

func Show(ctx context.Context, db store.Store, runID string) (ShowDetail, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return ShowDetail{}, fmt.Errorf("run id is required")
	}
	runRecord, err := db.Run(ctx, runID)
	if err != nil {
		return ShowDetail{}, err
	}

	runArtifacts, err := db.ArtifactsByRun(ctx, runID)
	if err != nil {
		return ShowDetail{}, err
	}

	detail := ShowDetail{
		RunID:  runRecord.ID,
		TaskID: runRecord.TaskID,
		Worker: runRecord.Worker,
	}
	artifactPaths := indexArtifactPaths(runArtifacts)

	if approvalName, refused := findWorkerApprovalArtifact(runArtifacts); refused {
		detail.ApprovalRefused = true
		detail.ApprovalName = approvalName
		detail.ProposalStatus = StatusRefused
		detail.Violations = []string{
			fmt.Sprintf("worker emitted MCP approval artifact %q; workers cannot approve MCP calls", approvalName),
		}
	}

	proposalPath, hasProposal := artifactPaths[ProposalArtifactName]
	if hasProposal {
		detail.ArtifactPath = proposalPath
		proposal, proposalSHA256, err := loadProposalFile(proposalPath)
		if err != nil {
			if detail.ProposalStatus == "" {
				detail.ProposalStatus = StatusInvalid
			}
			detail.Violations = append(detail.Violations, err.Error())
		} else {
			detail.ProposalID = proposal.ID
			detail.Server = proposal.Server
			detail.Tool = proposal.Tool
			detail.ArgumentsSHA256 = proposal.ArgumentsSHA256
			detail.ProposalSHA256 = proposalSHA256
			detail.PolicyPath = proposal.PolicyPath
			detail.ConfigPath = proposal.ConfigPath
			detail.Runtime = proposal.Runtime
			detail.RuntimeConfig = proposal.RuntimeConfigPath
			detail.Workspace = proposal.Workspace
			if detail.ProposalStatus == "" {
				detail.ProposalStatus = deriveProposalStatus(runArtifacts, artifactPaths)
			}
			detail.Suggested = suggestedCommands(proposalPath, proposal)
		}
	} else if detail.ProposalStatus == StatusRefused {
		// approval-only refusal without a proposal artifact
	} else {
		return ShowDetail{}, fmt.Errorf("run %q has no MCP tool call proposal", runID)
	}

	if lintPath, ok := artifactPaths[ProposalLintArtifactName]; ok {
		detail.LintPath = lintPath
		violations, warnings, err := loadLintArtifact(lintPath)
		if err != nil {
			detail.Violations = append(detail.Violations, err.Error())
		} else {
			if len(violations) > 0 {
				detail.Violations = appendUnique(detail.Violations, violations...)
			}
			if len(warnings) > 0 {
				detail.Warnings = appendUnique(detail.Warnings, warnings...)
			}
		}
	}

	if preflightPath, ok := artifactPaths[ProposalPreflightArtifactName]; ok {
		detail.PreflightPath = preflightPath
		preflightStatus, failures, warnings, err := loadPreflightArtifact(preflightPath)
		if err != nil {
			detail.PreflightFails = append(detail.PreflightFails, err.Error())
		} else {
			detail.PreflightStatus = preflightStatus
			if len(failures) > 0 {
				detail.PreflightFails = appendUnique(detail.PreflightFails, failures...)
			}
			if len(warnings) > 0 {
				detail.Warnings = appendUnique(detail.Warnings, warnings...)
			}
		}
	}

	if detail.ApprovalRefused {
		detail.ProposalStatus = StatusRefused
	}

	return detail, nil
}

func Export(ctx context.Context, db store.Store, runID string, outputPath string) error {
	runID = strings.TrimSpace(runID)
	outputPath = strings.TrimSpace(outputPath)
	if runID == "" {
		return fmt.Errorf("run id is required")
	}
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}

	runArtifacts, err := db.ArtifactsByRun(ctx, runID)
	if err != nil {
		return err
	}
	artifactPaths := indexArtifactPaths(runArtifacts)
	proposalPath, ok := artifactPaths[ProposalArtifactName]
	if !ok {
		return fmt.Errorf("run %q has no MCP tool call proposal artifact", runID)
	}
	content, err := os.ReadFile(proposalPath)
	if err != nil {
		return fmt.Errorf("read proposal artifact %q: %w", proposalPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		return fmt.Errorf("write proposal export %q: %w", outputPath, err)
	}
	return nil
}

func WriteListText(result ListResult, out io.Writer) error {
	if len(result.Proposals) == 0 {
		_, err := fmt.Fprintln(out, "mcp_proposals: (empty)")
		return err
	}
	if _, err := fmt.Fprintln(out, "mcp_proposals:"); err != nil {
		return err
	}
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "run_id\ttask_id\tworker\tproposal_status\tproposal_sha256\tproposal_id\tserver\ttool\tpreflight_status\tartifact_path"); err != nil {
		return err
	}
	for _, entry := range result.Proposals {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.RunID,
			entry.TaskID,
			entry.Worker,
			entry.ProposalStatus,
			entry.ProposalSHA256,
			entry.ProposalID,
			entry.Server,
			entry.Tool,
			entry.PreflightStatus,
			entry.ArtifactPath,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

func WriteListJSON(result ListResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func WriteShowText(detail ShowDetail, out io.Writer) error {
	lines := []struct {
		name  string
		value string
	}{
		{"run_id", detail.RunID},
		{"task_id", detail.TaskID},
		{"worker", detail.Worker},
		{"proposal_status", detail.ProposalStatus},
		{"proposal_id", detail.ProposalID},
		{"server", detail.Server},
		{"tool", detail.Tool},
		{"arguments_sha256", detail.ArgumentsSHA256},
		{"proposal_sha256", detail.ProposalSHA256},
		{"policy_path", detail.PolicyPath},
		{"config_path", detail.ConfigPath},
		{"runtime", detail.Runtime},
		{"runtime_config_path", detail.RuntimeConfig},
		{"workspace", detail.Workspace},
		{"preflight_status", detail.PreflightStatus},
		{"artifact_path", detail.ArtifactPath},
		{"lint_artifact_path", detail.LintPath},
		{"preflight_artifact_path", detail.PreflightPath},
	}
	for _, line := range lines {
		if strings.TrimSpace(line.value) == "" {
			continue
		}
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.name, line.value); err != nil {
			return err
		}
	}
	if detail.ApprovalRefused {
		if _, err := fmt.Fprintf(out, "approval_refused: true\n"); err != nil {
			return err
		}
		if detail.ApprovalName != "" {
			if _, err := fmt.Fprintf(out, "approval_artifact_name: %s\n", detail.ApprovalName); err != nil {
				return err
			}
		}
	}
	for _, label := range []struct {
		name  string
		items []string
	}{
		{"violation", detail.Violations},
		{"warning", detail.Warnings},
		{"preflight_failure", detail.PreflightFails},
	} {
		for _, item := range label.items {
			if _, err := fmt.Fprintf(out, "%s: %s\n", label.name, item); err != nil {
				return err
			}
		}
	}
	if len(detail.Suggested) > 0 {
		if _, err := fmt.Fprintln(out, "suggested_commands:"); err != nil {
			return err
		}
		for _, command := range detail.Suggested {
			if _, err := fmt.Fprintf(out, "  %s\n", command); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteShowJSON(detail ShowDetail, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(detail)
}

func ValidateListStatus(status string) error {
	if status == "" {
		return nil
	}
	switch status {
	case StatusValid, StatusInvalid, StatusPreflightPassed, StatusPreflightFailed, StatusRefused:
		return nil
	default:
		return fmt.Errorf("unsupported --status %q", status)
	}
}

func buildListEntry(ctx context.Context, db store.Store, runRecord runs.Run) (ListEntry, bool, error) {
	runArtifacts, err := db.ArtifactsByRun(ctx, runRecord.ID)
	if err != nil {
		return ListEntry{}, false, err
	}
	artifactPaths := indexArtifactPaths(runArtifacts)

	approvalName, approvalRefused := findWorkerApprovalArtifact(runArtifacts)
	proposalPath, hasProposal := artifactPaths[ProposalArtifactName]
	if !hasProposal && !approvalRefused {
		return ListEntry{}, false, nil
	}

	entry := ListEntry{
		RunID:           runRecord.ID,
		TaskID:          runRecord.TaskID,
		Worker:          runRecord.Worker,
		ApprovalRefused: approvalRefused,
		ArtifactPath:    proposalPath,
	}
	if approvalRefused {
		entry.ProposalStatus = StatusRefused
	}

	if hasProposal {
		proposal, proposalSHA256, err := loadProposalFile(proposalPath)
		if err != nil {
			entry.ProposalStatus = StatusInvalid
			entry.ProposalSHA256 = proposalSHA256
		} else {
			entry.ProposalID = proposal.ID
			entry.Server = proposal.Server
			entry.Tool = proposal.Tool
			entry.ProposalSHA256 = proposalSHA256
			if !approvalRefused {
				entry.ProposalStatus = deriveProposalStatus(runArtifacts, artifactPaths)
			}
		}
	} else if approvalRefused {
		entry.ProposalStatus = StatusRefused
		_ = approvalName
	}

	if preflightPath, ok := artifactPaths[ProposalPreflightArtifactName]; ok {
		preflightStatus, _, _, err := loadPreflightArtifact(preflightPath)
		if err == nil {
			entry.PreflightStatus = preflightStatus
		}
	}

	if entry.ProposalStatus == "" {
		entry.ProposalStatus = traceProposalStatus(runArtifacts)
	}
	if entry.ProposalStatus == "" && approvalRefused {
		entry.ProposalStatus = StatusRefused
	}

	return entry, true, nil
}

func deriveProposalStatus(runArtifacts []artifacts.Artifact, artifactPaths map[string]string) string {
	if preflightPath, ok := artifactPaths[ProposalPreflightArtifactName]; ok {
		status, _, _, err := loadPreflightArtifact(preflightPath)
		if err == nil {
			switch status {
			case mcpapproval.PreflightStatusPassed:
				return StatusPreflightPassed
			case mcpapproval.PreflightStatusFailed:
				return StatusPreflightFailed
			}
		}
	}
	if lintPath, ok := artifactPaths[ProposalLintArtifactName]; ok {
		violations, _, err := loadLintArtifact(lintPath)
		if err == nil {
			if len(violations) > 0 {
				return StatusInvalid
			}
		}
		var lint struct {
			Status string `json:"status"`
		}
		if content, err := os.ReadFile(lintPath); err == nil {
			if json.Unmarshal(content, &lint) == nil {
				switch lint.Status {
				case "skipped_policy":
					return StatusValid
				case mcpapproval.PreflightStatusFailed, "invalid":
					return StatusInvalid
				case mcpapproval.PreflightStatusPassed:
					return StatusValid
				}
			}
		}
	}
	if status := traceProposalStatus(runArtifacts); status != "" {
		return status
	}
	return StatusValid
}

func traceProposalStatus(runArtifacts []artifacts.Artifact) string {
	tracePath, ok := artifactPathByName(runArtifacts, "execution-trace.json")
	if !ok {
		return ""
	}
	content, err := os.ReadFile(tracePath)
	if err != nil {
		return ""
	}
	var trace struct {
		MCPToolProposalStatus string `json:"mcp_tool_proposal_status"`
		MCPToolProposalSHA256 string `json:"mcp_tool_proposal_sha256"`
	}
	if err := json.Unmarshal(content, &trace); err != nil {
		return ""
	}
	switch trace.MCPToolProposalStatus {
	case "valid":
		return StatusValid
	case "invalid":
		return StatusInvalid
	case "preflight_passed":
		return StatusPreflightPassed
	case "preflight_failed":
		return StatusPreflightFailed
	default:
		return ""
	}
}

func loadProposalFile(path string) (mcpapproval.MCPToolCallProposal, string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return mcpapproval.MCPToolCallProposal{}, "", fmt.Errorf("read proposal artifact %q: %w", path, err)
	}
	sha := fileContentSHA256(content)
	proposal, err := mcpapproval.ParseProposalJSON(content)
	if err != nil {
		return mcpapproval.MCPToolCallProposal{}, sha, err
	}
	return proposal, sha, nil
}

func loadLintArtifact(path string) ([]string, []string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read lint artifact %q: %w", path, err)
	}
	var lint mcpapproval.LintResult
	if err := json.Unmarshal(content, &lint); err != nil {
		return nil, nil, fmt.Errorf("parse lint artifact %q: %w", path, err)
	}
	return lint.Violations, lint.Warnings, nil
}

func loadPreflightArtifact(path string) (string, []string, []string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil, fmt.Errorf("read preflight artifact %q: %w", path, err)
	}
	var preflight mcpapproval.MCPToolCallPreflight
	if err := json.Unmarshal(content, &preflight); err != nil {
		return "", nil, nil, fmt.Errorf("parse preflight artifact %q: %w", path, err)
	}
	return preflight.Status, preflight.Failures, preflight.Warnings, nil
}

func suggestedCommands(proposalPath string, proposal mcpapproval.MCPToolCallProposal) []string {
	approvalOutput := strings.TrimSuffix(proposalPath, filepath.Ext(proposalPath)) + "-approval.json"
	approve := fmt.Sprintf(
		"deonctl mcp proposal approve --proposal %s --policy %s --decision approved --reason \"<review reason>\" --output %s --confirm-read-only",
		proposalPath,
		proposal.PolicyPath,
		approvalOutput,
	)
	execute := fmt.Sprintf(
		"deonctl mcp proposal execute --proposal %s --approval %s --config %s --policy %s --artifacts-dir <artifacts-dir> --confirm-execute",
		proposalPath,
		approvalOutput,
		proposal.ConfigPath,
		proposal.PolicyPath,
	)
	if strings.TrimSpace(proposal.ConfigPath) == "" {
		execute = fmt.Sprintf(
			"deonctl mcp proposal execute --proposal %s --approval %s --policy %s --artifacts-dir <artifacts-dir> --confirm-execute",
			proposalPath,
			approvalOutput,
			proposal.PolicyPath,
		)
	}
	return []string{approve, execute}
}

func findWorkerApprovalArtifact(runArtifacts []artifacts.Artifact) (string, bool) {
	for _, name := range workerApprovalArtifactNames {
		if _, ok := artifactPathByName(runArtifacts, name); ok {
			return name, true
		}
	}
	return "", false
}

func indexArtifactPaths(runArtifacts []artifacts.Artifact) map[string]string {
	paths := make(map[string]string, len(runArtifacts))
	for _, artifact := range runArtifacts {
		name := artifactBaseName(artifact.Path)
		if name == "" {
			continue
		}
		paths[name] = artifact.Path
	}
	return paths
}

func artifactPathByName(runArtifacts []artifacts.Artifact, name string) (string, bool) {
	for _, artifact := range runArtifacts {
		if artifactBaseName(artifact.Path) == name {
			return artifact.Path, true
		}
	}
	return "", false
}

func artifactBaseName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func fileContentSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func appendUnique(dst []string, items ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, item := range dst {
		seen[item] = struct{}{}
	}
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		dst = append(dst, item)
	}
	return dst
}
