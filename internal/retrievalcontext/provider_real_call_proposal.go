package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderRealCallProposalNewOptions struct {
	SimulationReportPath     string
	RequestEnvelopePath      string
	AdapterPlanPath          string
	CredentialPolicyPlanPath string
	ReleaseGatePath          string
	OutputPath               string
}

type ProviderRealCallProposal struct {
	Status                     string    `json:"status"`
	CreatedAt                  time.Time `json:"created_at"`
	RealCallProposalReady      bool      `json:"real_call_proposal_ready"`
	WouldCallProviderIfEnabled bool      `json:"would_call_provider_if_enabled"`
	ProviderCallAllowedNow     bool      `json:"provider_call_allowed_now"`
	TransportCalled            bool      `json:"transport_called"`
	SentToProvider             bool      `json:"sent_to_provider"`
	SecretValuesRead           bool      `json:"secret_values_read"`
	ProviderCall               bool      `json:"provider_call"`
	NetworkCall                bool      `json:"network_call"`
	WorkerExecution            bool      `json:"worker_execution"`
	WorkspaceModified          bool      `json:"workspace_modified"`
	BlockedReason              string    `json:"blocked_reason"`
	SimulationReportSHA256     string    `json:"simulation_report_sha256"`
	RequestEnvelopeSHA256      string    `json:"request_envelope_sha256"`
	AdapterPlanSHA256          string    `json:"adapter_plan_sha256"`
	CredentialPolicyPlanSHA256 string    `json:"credential_policy_plan_sha256"`
	ReleaseGateSHA256          string    `json:"release_gate_sha256"`
	ProviderPayloadSHA256      string    `json:"provider_payload_sha256,omitempty"`
	Warnings                   []string  `json:"warnings,omitempty"`
	Failures                   []string  `json:"failures,omitempty"`
}

type ProviderRealCallProposalInspectOptions struct {
	SimulationReportPath     string
	RequestEnvelopePath      string
	AdapterPlanPath          string
	CredentialPolicyPlanPath string
	ReleaseGatePath          string
}

type ProviderRealCallProposalInspectResult struct {
	Status                     string   `json:"status"`
	RealCallProposalReady      bool     `json:"real_call_proposal_ready"`
	WouldCallProviderIfEnabled bool     `json:"would_call_provider_if_enabled"`
	ProviderCallAllowedNow     bool     `json:"provider_call_allowed_now"`
	TransportCalled            bool     `json:"transport_called"`
	SentToProvider             bool     `json:"sent_to_provider"`
	SecretValuesRead           bool     `json:"secret_values_read"`
	ProviderCall               bool     `json:"provider_call"`
	NetworkCall                bool     `json:"network_call"`
	Warnings                   []string `json:"warnings"`
	Failures                   []string `json:"failures,omitempty"`
}

func NewProviderRealCallProposal(opts ProviderRealCallProposalNewOptions) (ProviderRealCallProposal, error) {
	chain, failures, err := loadProviderRealCallProposalChain(opts.SimulationReportPath, opts.RequestEnvelopePath, opts.AdapterPlanPath, opts.CredentialPolicyPlanPath, opts.ReleaseGatePath)
	if err != nil {
		return ProviderRealCallProposal{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealCallProposal{}, err
	}

	result := ProviderRealCallProposal{
		Status:                     lancedbpolicy.StatusOK,
		CreatedAt:                  time.Now().UTC(),
		WouldCallProviderIfEnabled: true,
		ProviderCallAllowedNow:     false,
		TransportCalled:            false,
		SentToProvider:             false,
		SecretValuesRead:           false,
		ProviderCall:               false,
		NetworkCall:                false,
		WorkerExecution:            false,
		WorkspaceModified:          false,
		BlockedReason:              ProviderCallExecutorBlockedReason,
		SimulationReportSHA256:     chain.simulationReportSHA256,
		RequestEnvelopeSHA256:      chain.requestEnvelopeSHA256,
		AdapterPlanSHA256:          chain.adapterPlanSHA256,
		CredentialPolicyPlanSHA256: chain.credentialPolicyPlanSHA256,
		ReleaseGateSHA256:          chain.releaseGateSHA256,
		ProviderPayloadSHA256:      chain.providerPayloadSHA256,
		Failures:                   failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.RealCallProposalReady = true
	if err := writeProviderRealCallProposalJSON(opts.OutputPath, result); err != nil {
		return ProviderRealCallProposal{}, err
	}
	return result, nil
}

func InspectProviderRealCallProposal(path string, opts ProviderRealCallProposalInspectOptions) (ProviderRealCallProposalInspectResult, error) {
	if err := validateRelativeSafePath("real call proposal path", path); err != nil {
		return ProviderRealCallProposalInspectResult{}, err
	}
	data, err := readArtifactBytesNoTextExcerpt("real call proposal", path)
	if err != nil {
		return ProviderRealCallProposalInspectResult{}, err
	}
	proposal, err := ParseProviderRealCallProposalJSON(data)
	if err != nil {
		return ProviderRealCallProposalInspectResult{}, err
	}

	result := ProviderRealCallProposalInspectResult{
		Status:                     lancedbpolicy.StatusOK,
		RealCallProposalReady:      proposal.RealCallProposalReady,
		WouldCallProviderIfEnabled: proposal.WouldCallProviderIfEnabled,
		ProviderCallAllowedNow:     proposal.ProviderCallAllowedNow,
		TransportCalled:            proposal.TransportCalled,
		SentToProvider:             proposal.SentToProvider,
		SecretValuesRead:           proposal.SecretValuesRead,
		ProviderCall:               proposal.ProviderCall,
		NetworkCall:                proposal.NetworkCall,
		Warnings:                   append([]string(nil), proposal.Warnings...),
	}

	var failures []string
	if !proposal.RealCallProposalReady {
		failures = append(failures, "real_call_proposal_ready must be true")
	}
	if !proposal.WouldCallProviderIfEnabled || proposal.ProviderCallAllowedNow || proposal.SecretValuesRead {
		failures = append(failures, "real call proposal must keep execution and secret flags blocked")
	}
	if proposal.TransportCalled || proposal.SentToProvider || proposal.ProviderCall || proposal.NetworkCall {
		failures = append(failures, "real call proposal must keep transport flags blocked")
	}

	if opts.SimulationReportPath != "" {
		chain, chainFailures, err := loadProviderRealCallProposalChain(opts.SimulationReportPath, opts.RequestEnvelopePath, opts.AdapterPlanPath, opts.CredentialPolicyPlanPath, opts.ReleaseGatePath)
		if err != nil {
			return ProviderRealCallProposalInspectResult{}, err
		}
		failures = append(failures, chainFailures...)
		if proposal.SimulationReportSHA256 != chain.simulationReportSHA256 {
			failures = append(failures, "simulation_report_sha256 mismatch")
		}
		if proposal.RequestEnvelopeSHA256 != chain.requestEnvelopeSHA256 {
			failures = append(failures, "request_envelope_sha256 mismatch")
		}
	}

	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

type providerRealCallProposalChain struct {
	simulationReportSHA256     string
	requestEnvelopeSHA256      string
	adapterPlanSHA256          string
	credentialPolicyPlanSHA256 string
	releaseGateSHA256          string
	providerPayloadSHA256      string
}

func loadProviderRealCallProposalChain(simulationReportPath, requestEnvelopePath, adapterPlanPath, credentialPolicyPlanPath, releaseGatePath string) (providerRealCallProposalChain, []string, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"simulation report path", simulationReportPath},
		{"request envelope path", requestEnvelopePath},
		{"adapter plan path", adapterPlanPath},
		{"credential policy plan path", credentialPolicyPlanPath},
		{"release gate path", releaseGatePath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return providerRealCallProposalChain{}, nil, err
		}
	}

	var chain providerRealCallProposalChain
	var failures []string

	simulationData, err := readArtifactBytesNoTextExcerpt("simulation report", simulationReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.simulationReportSHA256 = sha256Hex(simulationData)
		var simulation ProviderExecutionSimulationBundleResult
		if err := json.Unmarshal(simulationData, &simulation); err != nil {
			failures = append(failures, err.Error())
		} else if !simulation.SimulationBundleReady {
			failures = append(failures, "simulation report simulation_bundle_ready must be true")
		} else if simulation.ExecutionResultAvailable || simulation.ProviderCall || simulation.SentToProvider {
			failures = append(failures, "simulation report must keep execution flags blocked")
		}
	}

	envelope, envelopeData, err := LoadProviderRequestEnvelope(requestEnvelopePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.requestEnvelopeSHA256 = sha256Hex(envelopeData)
		if !envelope.ProviderRequestReady {
			failures = append(failures, "request envelope provider_request_ready must be true")
		}
		chain.providerPayloadSHA256 = envelope.ProviderPayloadSHA256
	}

	adapterPlan, adapterData, err := LoadProviderAdapterPlan(adapterPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.adapterPlanSHA256 = sha256Hex(adapterData)
		if !adapterPlan.AdapterPlanReady {
			failures = append(failures, "adapter plan adapter_plan_ready must be true")
		}
	}

	credentialPlan, credentialData, err := LoadProviderCredentialPolicyPlan(credentialPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.credentialPolicyPlanSHA256 = sha256Hex(credentialData)
		if !credentialPlan.CredentialPolicyPlanReady || credentialPlan.SecretValuesRead {
			failures = append(failures, "credential policy plan must be ready without secret reads")
		}
	}

	releaseGate, releaseGateData, err := LoadProviderExecutorReleaseGate(releaseGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.releaseGateSHA256 = sha256Hex(releaseGateData)
		if !releaseGate.ActivationGateReady || releaseGate.ActivationAllowedNow {
			failures = append(failures, "release gate must keep activation blocked")
		}
	}

	return chain, failures, nil
}

func LoadProviderRealCallProposal(path string) (ProviderRealCallProposal, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("real call proposal", path)
	if err != nil {
		return ProviderRealCallProposal{}, nil, err
	}
	proposal, err := ParseProviderRealCallProposalJSON(data)
	if err != nil {
		return ProviderRealCallProposal{}, nil, err
	}
	return proposal, data, nil
}

func ParseProviderRealCallProposalJSON(data []byte) (ProviderRealCallProposal, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderRealCallProposal{}, fmt.Errorf("real call proposal must not contain materialized preview text")
	}
	var proposal ProviderRealCallProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		return ProviderRealCallProposal{}, fmt.Errorf("parse real call proposal json: %w", err)
	}
	return proposal, nil
}

func writeProviderRealCallProposalJSON(path string, proposal ProviderRealCallProposal) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create real call proposal output dir: %w", err)
	}
	data, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal real call proposal json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("real call proposal must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderRealCallProposalInspectJSON(result ProviderRealCallProposalInspectResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal real call proposal inspect json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderRealCallProposalInspectText(result ProviderRealCallProposalInspectResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_real_call_proposal_inspect:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"real_call_proposal_ready", fmt.Sprintf("%t", result.RealCallProposalReady)},
		{"would_call_provider_if_enabled", fmt.Sprintf("%t", result.WouldCallProviderIfEnabled)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
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
	return nil
}

func WriteProviderRealCallProposalNewText(proposal ProviderRealCallProposal, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_real_call_proposal_new: ok"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "real_call_proposal_ready: %t\nwould_call_provider_if_enabled: %t\nprovider_call_allowed_now: %t\nsecret_values_read: %t\n",
		proposal.RealCallProposalReady, proposal.WouldCallProviderIfEnabled, proposal.ProviderCallAllowedNow, proposal.SecretValuesRead)
	return err
}
