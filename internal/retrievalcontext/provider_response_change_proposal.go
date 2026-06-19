package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderResponseChangeProposalOptions struct {
	ResponseFixturePath  string
	SimulationReportPath string
	RealCallProposalPath string
	OutputPath           string
}

type ProviderResponseChangeProposalResult struct {
	Status                 string   `json:"status"`
	ChangeProposalReady    bool     `json:"change_proposal_ready"`
	ResponseSource         string   `json:"response_source"`
	WorkspaceModified      bool     `json:"workspace_modified"`
	DiffApplied            bool     `json:"diff_applied"`
	CommitCreated          bool     `json:"commit_created"`
	PRCreated              bool     `json:"pr_created"`
	WorkerExecution        bool     `json:"worker_execution"`
	ProviderCall           bool     `json:"provider_call"`
	NetworkCall            bool     `json:"network_call"`
	TransportCalled        bool     `json:"transport_called"`
	SentToProvider         bool     `json:"sent_to_provider"`
	BlockedReason          string   `json:"blocked_reason"`
	ResponseFixtureSHA256  string   `json:"response_fixture_sha256"`
	SimulationReportSHA256 string   `json:"simulation_report_sha256"`
	RealCallProposalSHA256 string   `json:"real_call_proposal_sha256"`
	ResponseFixtureID      string   `json:"response_fixture_id"`
	ProposedChangeSummary  string   `json:"proposed_change_summary"`
	ProposedFileIntent     string   `json:"proposed_file_intent"`
	Warnings               []string `json:"warnings,omitempty"`
	Failures               []string `json:"failures,omitempty"`
}

type ProviderResponseChangeProposalReportOptions struct {
	ChangeProposalPath   string
	ResponseFixturePath  string
	SimulationReportPath string
	RealCallProposalPath string
}

func ProviderResponseChangeProposal(opts ProviderResponseChangeProposalOptions) (ProviderResponseChangeProposalResult, error) {
	chain, failures, err := loadProviderResponseChangeProposalChain(opts.ResponseFixturePath, opts.SimulationReportPath, opts.RealCallProposalPath)
	if err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}

	result := ProviderResponseChangeProposalResult{
		Status:                 lancedbpolicy.StatusOK,
		ResponseSource:         ProviderResponseFixtureSource,
		WorkspaceModified:      false,
		DiffApplied:            false,
		CommitCreated:          false,
		PRCreated:              false,
		WorkerExecution:        false,
		ProviderCall:           false,
		NetworkCall:            false,
		TransportCalled:        false,
		SentToProvider:         false,
		BlockedReason:          ProviderCallExecutorBlockedReason,
		ResponseFixtureSHA256:  chain.responseFixtureSHA256,
		SimulationReportSHA256: chain.simulationReportSHA256,
		RealCallProposalSHA256: chain.realCallProposalSHA256,
		ResponseFixtureID:      chain.responseFixtureID,
		ProposedChangeSummary:  "metadata-only fixture change intent; no patch applied",
		ProposedFileIntent:     "would_update_workspace_files_after_provider_response",
		Failures:               failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.ChangeProposalReady = true
	if err := writeProviderResponseChangeProposalJSON(opts.OutputPath, result); err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}
	return result, nil
}

func ProviderResponseChangeProposalReport(opts ProviderResponseChangeProposalReportOptions) (ProviderResponseChangeProposalResult, error) {
	if err := validateRelativeSafePath("change proposal path", opts.ChangeProposalPath); err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}
	data, err := readArtifactBytesNoTextExcerpt("change proposal", opts.ChangeProposalPath)
	if err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}
	stored, err := ParseProviderResponseChangeProposalJSON(data)
	if err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}

	chain, failures, err := loadProviderResponseChangeProposalChain(opts.ResponseFixturePath, opts.SimulationReportPath, opts.RealCallProposalPath)
	if err != nil {
		return ProviderResponseChangeProposalResult{}, err
	}

	result := stored
	result.Failures = failures
	if !stored.ChangeProposalReady {
		failures = append(failures, "stored change_proposal_ready must be true")
	}
	if stored.WorkspaceModified || stored.DiffApplied || stored.CommitCreated || stored.PRCreated || stored.WorkerExecution {
		failures = append(failures, "stored change proposal must keep workspace flags blocked")
	}
	if stored.ResponseFixtureSHA256 != chain.responseFixtureSHA256 {
		failures = append(failures, "response_fixture_sha256 mismatch")
	}
	if stored.RealCallProposalSHA256 != chain.realCallProposalSHA256 {
		failures = append(failures, "real_call_proposal_sha256 mismatch")
	}
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.Status = lancedbpolicy.StatusOK
	}
	return result, nil
}

type providerResponseChangeProposalChain struct {
	responseFixtureSHA256  string
	simulationReportSHA256 string
	realCallProposalSHA256 string
	responseFixtureID      string
}

func loadProviderResponseChangeProposalChain(responseFixturePath, simulationReportPath, realCallProposalPath string) (providerResponseChangeProposalChain, []string, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"response fixture path", responseFixturePath},
		{"simulation report path", simulationReportPath},
		{"real call proposal path", realCallProposalPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return providerResponseChangeProposalChain{}, nil, err
		}
	}

	var chain providerResponseChangeProposalChain
	var failures []string

	fixture, fixtureData, err := LoadProviderResponseFixture(responseFixturePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.responseFixtureSHA256 = sha256Hex(fixtureData)
		chain.responseFixtureID = fixture.ResponseFixtureID
		if !fixture.ResponseFixtureReady || fixture.ResponseSource != ProviderResponseFixtureSource {
			failures = append(failures, "response fixture must be ready with fixture source")
		}
	}

	simulationData, err := readArtifactBytesNoTextExcerpt("simulation report", simulationReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.simulationReportSHA256 = sha256Hex(simulationData)
	}

	proposal, proposalData, err := LoadProviderRealCallProposal(realCallProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.realCallProposalSHA256 = sha256Hex(proposalData)
		if !proposal.RealCallProposalReady || proposal.ProviderCallAllowedNow {
			failures = append(failures, "real call proposal must be ready with provider_call_allowed_now false")
		}
	}

	return chain, failures, nil
}

func ParseProviderResponseChangeProposalJSON(data []byte) (ProviderResponseChangeProposalResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderResponseChangeProposalResult{}, fmt.Errorf("change proposal must not contain materialized preview text")
	}
	var result ProviderResponseChangeProposalResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderResponseChangeProposalResult{}, fmt.Errorf("parse change proposal json: %w", err)
	}
	return result, nil
}

func writeProviderResponseChangeProposalJSON(path string, result ProviderResponseChangeProposalResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create change proposal output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal change proposal json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("change proposal must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderResponseChangeProposalText(result ProviderResponseChangeProposalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_response_change_proposal:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"change_proposal_ready", fmt.Sprintf("%t", result.ChangeProposalReady)},
		{"response_source", result.ResponseSource},
		{"workspace_modified", fmt.Sprintf("%t", result.WorkspaceModified)},
		{"diff_applied", fmt.Sprintf("%t", result.DiffApplied)},
		{"commit_created", fmt.Sprintf("%t", result.CommitCreated)},
		{"pr_created", fmt.Sprintf("%t", result.PRCreated)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	return nil
}

func WriteProviderResponseChangeProposalJSON(result ProviderResponseChangeProposalResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal change proposal json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderResponseChangeProposalReportText(result ProviderResponseChangeProposalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_response_change_proposal_report:"); err != nil {
		return err
	}
	return WriteProviderResponseChangeProposalText(result, out)
}

func WriteProviderResponseChangeProposalReportJSON(result ProviderResponseChangeProposalResult, out io.Writer) error {
	return WriteProviderResponseChangeProposalJSON(result, out)
}
