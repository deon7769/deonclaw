package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderActivationReadinessAuditOptions struct {
	CredentialPolicyPlanPath string
	RealCallProposalPath     string
	ChangeProposalPath       string
	SimulationReportPath     string
	ReleaseGatePath          string
}

type ProviderActivationReadinessAuditResult struct {
	Status                       string   `json:"status"`
	ActivationReadinessReady     bool     `json:"activation_readiness_ready"`
	RealProviderCallSupportedNow bool     `json:"real_provider_call_supported_now"`
	ActivationAllowedNow         bool     `json:"activation_allowed_now"`
	ProviderCall                 bool     `json:"provider_call"`
	NetworkCall                  bool     `json:"network_call"`
	TransportCalled              bool     `json:"transport_called"`
	SecretValuesRead             bool     `json:"secret_values_read"`
	WorkspaceModified            bool     `json:"workspace_modified"`
	ExecutionSupportedNow        bool     `json:"execution_supported_now"`
	PromptInjectionRealRunner    bool     `json:"prompt_injection_real_runner"`
	BlockedReason                string   `json:"blocked_reason"`
	CredentialPolicyPlanSHA256   string   `json:"credential_policy_plan_sha256"`
	RealCallProposalSHA256       string   `json:"real_call_proposal_sha256"`
	ChangeProposalSHA256         string   `json:"change_proposal_sha256"`
	SimulationReportSHA256       string   `json:"simulation_report_sha256"`
	ReleaseGateSHA256            string   `json:"release_gate_sha256"`
	Warnings                     []string `json:"warnings,omitempty"`
	Failures                     []string `json:"failures,omitempty"`
}

func ProviderActivationReadinessAudit(opts ProviderActivationReadinessAuditOptions) (ProviderActivationReadinessAuditResult, error) {
	chain, failures, err := loadProviderActivationReadinessChain(opts.CredentialPolicyPlanPath, opts.RealCallProposalPath, opts.ChangeProposalPath, opts.SimulationReportPath, opts.ReleaseGatePath)
	if err != nil {
		return ProviderActivationReadinessAuditResult{}, err
	}

	result := ProviderActivationReadinessAuditResult{
		Status:                       lancedbpolicy.StatusOK,
		RealProviderCallSupportedNow: false,
		ActivationAllowedNow:         false,
		ProviderCall:                 false,
		NetworkCall:                  false,
		TransportCalled:              false,
		SecretValuesRead:             false,
		WorkspaceModified:            false,
		ExecutionSupportedNow:        false,
		PromptInjectionRealRunner:    false,
		BlockedReason:                ProviderCallExecutorBlockedReason,
		CredentialPolicyPlanSHA256:   chain.credentialPolicyPlanSHA256,
		RealCallProposalSHA256:       chain.realCallProposalSHA256,
		ChangeProposalSHA256:         chain.changeProposalSHA256,
		SimulationReportSHA256:       chain.simulationReportSHA256,
		ReleaseGateSHA256:            chain.releaseGateSHA256,
		Failures:                     failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.ActivationReadinessReady = true
	}
	return result, nil
}

func LoadProviderActivationReadinessAudit(path string) (ProviderActivationReadinessAuditResult, []byte, error) {
	if err := validateRelativeSafePath("provider activation readiness audit path", path); err != nil {
		return ProviderActivationReadinessAuditResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("provider activation readiness audit", path)
	if err != nil {
		return ProviderActivationReadinessAuditResult{}, nil, err
	}
	var result ProviderActivationReadinessAuditResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderActivationReadinessAuditResult{}, nil, fmt.Errorf("parse provider activation readiness audit json: %w", err)
	}
	return result, data, nil
}

func ProviderActivationReadinessReport(opts ProviderActivationReadinessAuditOptions) (ProviderActivationReadinessAuditResult, error) {
	return ProviderActivationReadinessAudit(opts)
}

type providerActivationReadinessChain struct {
	credentialPolicyPlanSHA256 string
	realCallProposalSHA256     string
	changeProposalSHA256       string
	simulationReportSHA256     string
	releaseGateSHA256          string
}

func loadProviderActivationReadinessChain(credentialPolicyPlanPath, realCallProposalPath, changeProposalPath, simulationReportPath, releaseGatePath string) (providerActivationReadinessChain, []string, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"credential policy plan path", credentialPolicyPlanPath},
		{"real call proposal path", realCallProposalPath},
		{"change proposal path", changeProposalPath},
		{"simulation report path", simulationReportPath},
		{"release gate path", releaseGatePath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return providerActivationReadinessChain{}, nil, err
		}
	}

	var chain providerActivationReadinessChain
	var failures []string

	credentialPlan, credentialData, err := LoadProviderCredentialPolicyPlan(credentialPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.credentialPolicyPlanSHA256 = sha256Hex(credentialData)
		if !credentialPlan.CredentialPolicyPlanReady || credentialPlan.SecretValuesRead || credentialPlan.CredentialCheckEnabled {
			failures = append(failures, "credential policy plan must be ready with checks disabled and no secret reads")
		}
	}

	proposal, proposalData, err := LoadProviderRealCallProposal(realCallProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.realCallProposalSHA256 = sha256Hex(proposalData)
		if !proposal.RealCallProposalReady || !proposal.WouldCallProviderIfEnabled || proposal.ProviderCallAllowedNow || proposal.SecretValuesRead {
			failures = append(failures, "real call proposal readiness invalid")
		}
		if proposal.TransportCalled || proposal.SentToProvider || proposal.ProviderCall {
			failures = append(failures, "real call proposal must keep transport flags blocked")
		}
	}

	changeData, err := readArtifactBytesNoTextExcerpt("change proposal", changeProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.changeProposalSHA256 = sha256Hex(changeData)
		change, err := ParseProviderResponseChangeProposalJSON(changeData)
		if err != nil {
			failures = append(failures, err.Error())
		} else if !change.ChangeProposalReady || change.WorkspaceModified || change.DiffApplied || change.WorkerExecution {
			failures = append(failures, "change proposal must be ready without workspace modification")
		}
	}

	simulationData, err := readArtifactBytesNoTextExcerpt("simulation report", simulationReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.simulationReportSHA256 = sha256Hex(simulationData)
		var simulation ProviderExecutionSimulationBundleResult
		if err := json.Unmarshal(simulationData, &simulation); err != nil {
			failures = append(failures, err.Error())
		} else if !simulation.SimulationBundleReady || simulation.ExecutionResultAvailable {
			failures = append(failures, "simulation report must be ready without execution result")
		}
	}

	releaseGate, releaseGateData, err := LoadProviderExecutorReleaseGate(releaseGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.releaseGateSHA256 = sha256Hex(releaseGateData)
		if !releaseGate.ActivationGateReady || releaseGate.ActivationAllowedNow || releaseGate.ExecutionSupportedNow {
			failures = append(failures, "release gate must keep activation blocked")
		}
	}

	return chain, failures, nil
}

func WriteProviderActivationReadinessAuditText(result ProviderActivationReadinessAuditResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_readiness_audit:"); err != nil {
		return err
	}
	return writeProviderActivationReadinessLines(result, out)
}

func WriteProviderActivationReadinessReportText(result ProviderActivationReadinessAuditResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_readiness_report:"); err != nil {
		return err
	}
	return writeProviderActivationReadinessLines(result, out)
}

func writeProviderActivationReadinessLines(result ProviderActivationReadinessAuditResult, out io.Writer) error {
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"activation_readiness_ready", fmt.Sprintf("%t", result.ActivationReadinessReady)},
		{"real_provider_call_supported_now", fmt.Sprintf("%t", result.RealProviderCallSupportedNow)},
		{"activation_allowed_now", fmt.Sprintf("%t", result.ActivationAllowedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"workspace_modified", fmt.Sprintf("%t", result.WorkspaceModified)},
		{"blocked_reason", result.BlockedReason},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	text := fmt.Sprintf("%+v", result)
	if strings.Contains(text, "text_excerpt") || strings.Contains(text, "alpha text") {
		return fmt.Errorf("activation readiness text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderActivationReadinessAuditJSON(result ProviderActivationReadinessAuditResult, out io.Writer) error {
	return writeProviderActivationReadinessJSON(result, out)
}

func WriteProviderActivationReadinessReportJSON(result ProviderActivationReadinessAuditResult, out io.Writer) error {
	return writeProviderActivationReadinessJSON(result, out)
}

func writeProviderActivationReadinessJSON(result ProviderActivationReadinessAuditResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal activation readiness json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("activation readiness json must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}
