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

type ProviderActivationReleasePackageOptions struct {
	ActivationPolicyPlanPath         string
	ActivationApprovalPath           string
	ActivationRehearsalPath          string
	ActivationReadinessAuditPath     string
	RealCallProposalPath             string
	ExecutionSimulationReportPath    string
	CredentialPolicyPlanPath         string
	ResponseChangeProposalReportPath string
	OutputPath                       string
}

type ProviderActivationReleasePackageResult struct {
	Status                        string   `json:"status"`
	ActivationReleasePackageReady bool     `json:"activation_release_package_ready"`
	RealActivationSupportedNow    bool     `json:"real_activation_supported_now"`
	ActivationAllowedNow          bool     `json:"activation_allowed_now"`
	ProviderCall                  bool     `json:"provider_call"`
	NetworkCall                   bool     `json:"network_call"`
	SecretValuesRead              bool     `json:"secret_values_read"`
	TransportCalled               bool     `json:"transport_called"`
	SentToProvider                bool     `json:"sent_to_provider"`
	ReceivedFromProvider          bool     `json:"received_from_provider"`
	WorkspaceModified             bool     `json:"workspace_modified"`
	DiffApplied                   bool     `json:"diff_applied"`
	CommitCreated                 bool     `json:"commit_created"`
	PRCreated                     bool     `json:"pr_created"`
	WorkerExecution               bool     `json:"worker_execution"`
	PromptInjectionRealRunner     bool     `json:"prompt_injection_real_runner"`
	BlockedReason                 string   `json:"blocked_reason"`
	ActivationPolicyPlanSHA256    string   `json:"activation_policy_plan_sha256,omitempty"`
	ActivationApprovalSHA256      string   `json:"activation_approval_sha256,omitempty"`
	ActivationRehearsalSHA256     string   `json:"activation_rehearsal_sha256,omitempty"`
	ReadinessAuditSHA256          string   `json:"readiness_audit_sha256,omitempty"`
	RealCallProposalSHA256        string   `json:"real_call_proposal_sha256,omitempty"`
	SimulationReportSHA256        string   `json:"simulation_report_sha256,omitempty"`
	CredentialPolicyPlanSHA256    string   `json:"credential_policy_plan_sha256,omitempty"`
	ChangeProposalReportSHA256    string   `json:"change_proposal_report_sha256,omitempty"`
	ProviderPayloadSHA256         string   `json:"provider_payload_sha256,omitempty"`
	Warnings                      []string `json:"warnings,omitempty"`
	Failures                      []string `json:"failures,omitempty"`
}

type ProviderActivationReleaseGateOptions struct {
	ActivationPolicyPlanPath         string
	ActivationApprovalPath           string
	ActivationRehearsalPath          string
	ActivationReadinessAuditPath     string
	RealCallProposalPath             string
	ExecutionSimulationReportPath    string
	CredentialPolicyPlanPath         string
	ResponseChangeProposalReportPath string
	ActivationReleasePackagePath     string
}

type ProviderActivationReleaseGateResult struct {
	Status                        string   `json:"status"`
	ActivationGateReady           bool     `json:"activation_gate_ready"`
	ActivationReleasePackageReady bool     `json:"activation_release_package_ready"`
	RealActivationSupportedNow    bool     `json:"real_activation_supported_now"`
	ActivationAllowedNow          bool     `json:"activation_allowed_now"`
	ProviderCall                  bool     `json:"provider_call"`
	NetworkCall                   bool     `json:"network_call"`
	SecretValuesRead              bool     `json:"secret_values_read"`
	TransportCalled               bool     `json:"transport_called"`
	SentToProvider                bool     `json:"sent_to_provider"`
	ReceivedFromProvider          bool     `json:"received_from_provider"`
	WorkspaceModified             bool     `json:"workspace_modified"`
	DiffApplied                   bool     `json:"diff_applied"`
	CommitCreated                 bool     `json:"commit_created"`
	PRCreated                     bool     `json:"pr_created"`
	WorkerExecution               bool     `json:"worker_execution"`
	PromptInjectionRealRunner     bool     `json:"prompt_injection_real_runner"`
	BlockedReason                 string   `json:"blocked_reason"`
	Warnings                      []string `json:"warnings,omitempty"`
	Failures                      []string `json:"failures,omitempty"`
}

func ProviderActivationReleasePackage(opts ProviderActivationReleasePackageOptions) (ProviderActivationReleasePackageResult, error) {
	chain, failures, err := loadProviderActivationReleaseChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationApprovalPath,
		opts.ActivationRehearsalPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.ExecutionSimulationReportPath,
		opts.CredentialPolicyPlanPath,
		opts.ResponseChangeProposalReportPath,
	)
	if err != nil {
		return ProviderActivationReleasePackageResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderActivationReleasePackageResult{}, err
	}

	result := blockedProviderActivationReleasePackageResult(chain, failures)
	if len(failures) == 0 {
		result.ActivationReleasePackageReady = true
	}
	if err := writeProviderActivationReleasePackageJSON(opts.OutputPath, result); err != nil {
		return ProviderActivationReleasePackageResult{}, err
	}
	return result, nil
}

func ProviderActivationReleaseGate(opts ProviderActivationReleaseGateOptions) (ProviderActivationReleaseGateResult, error) {
	chain, failures, err := loadProviderActivationReleaseChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationApprovalPath,
		opts.ActivationRehearsalPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.ExecutionSimulationReportPath,
		opts.CredentialPolicyPlanPath,
		opts.ResponseChangeProposalReportPath,
	)
	if err != nil {
		return ProviderActivationReleaseGateResult{}, err
	}
	if err := validateRelativeSafePath("activation release package path", opts.ActivationReleasePackagePath); err != nil {
		return ProviderActivationReleaseGateResult{}, err
	}
	pkg, pkgData, err := LoadProviderActivationReleasePackage(opts.ActivationReleasePackagePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if !pkg.ActivationReleasePackageReady {
			failures = append(failures, "activation release package activation_release_package_ready must be true")
		}
		reconcileProviderExecutorHash("release package", pkg.ActivationPolicyPlanSHA256, chain.activationPolicyPlanSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.ActivationApprovalSHA256, chain.activationApprovalSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.ActivationRehearsalSHA256, chain.activationRehearsalSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.ReadinessAuditSHA256, chain.readinessAuditSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.RealCallProposalSHA256, chain.realCallProposalSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.SimulationReportSHA256, chain.simulationReportSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.CredentialPolicyPlanSHA256, chain.credentialPolicyPlanSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.ChangeProposalReportSHA256, chain.changeProposalReportSHA256, &failures)
		reconcileProviderExecutorHash("release package", pkg.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
		if pkg.ActivationAllowedNow || pkg.RealActivationSupportedNow || pkg.ProviderCall || pkg.NetworkCall || pkg.TransportCalled || pkg.SentToProvider || pkg.SecretValuesRead || pkg.WorkspaceModified {
			failures = append(failures, "activation release package must keep execution flags blocked")
		}
		if sha256Hex(pkgData) == "" {
			failures = append(failures, "activation release package hash missing")
		}
	}

	result := ProviderActivationReleaseGateResult{
		Status:                     lancedbpolicy.StatusOK,
		RealActivationSupportedNow: false,
		ActivationAllowedNow:       false,
		ProviderCall:               false,
		NetworkCall:                false,
		SecretValuesRead:           false,
		TransportCalled:            false,
		SentToProvider:             false,
		ReceivedFromProvider:       false,
		WorkspaceModified:          false,
		DiffApplied:                false,
		CommitCreated:              false,
		PRCreated:                  false,
		WorkerExecution:            false,
		PromptInjectionRealRunner:  false,
		BlockedReason:              ProviderCallExecutorBlockedReason,
		Failures:                   failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.ActivationGateReady = true
		result.ActivationReleasePackageReady = true
	}
	return result, nil
}

type providerActivationReleaseChain struct {
	activationPolicyPlanSHA256 string
	activationApprovalSHA256   string
	activationRehearsalSHA256  string
	readinessAuditSHA256       string
	realCallProposalSHA256     string
	simulationReportSHA256     string
	credentialPolicyPlanSHA256 string
	changeProposalReportSHA256 string
	providerPayloadSHA256      string
}

func loadProviderActivationReleaseChain(activationPolicyPlanPath, activationApprovalPath, activationRehearsalPath, activationReadinessAuditPath, realCallProposalPath, executionSimulationReportPath, credentialPolicyPlanPath, responseChangeProposalReportPath string) (providerActivationReleaseChain, []string, error) {
	rehearsalChain, failures, err := loadProviderActivationRehearsalChain(
		activationPolicyPlanPath,
		activationApprovalPath,
		activationReadinessAuditPath,
		realCallProposalPath,
		credentialPolicyPlanPath,
		executionSimulationReportPath,
	)
	if err != nil {
		return providerActivationReleaseChain{}, nil, err
	}
	var chain providerActivationReleaseChain
	chain.activationPolicyPlanSHA256 = rehearsalChain.activationPolicyPlanSHA256
	chain.activationApprovalSHA256 = rehearsalChain.activationApprovalSHA256
	chain.readinessAuditSHA256 = rehearsalChain.readinessAuditSHA256
	chain.realCallProposalSHA256 = rehearsalChain.realCallProposalSHA256
	chain.credentialPolicyPlanSHA256 = rehearsalChain.credentialPolicyPlanSHA256
	chain.simulationReportSHA256 = rehearsalChain.simulationReportSHA256
	chain.providerPayloadSHA256 = rehearsalChain.providerPayloadSHA256

	if err := validateRelativeSafePath("activation rehearsal path", activationRehearsalPath); err != nil {
		return providerActivationReleaseChain{}, nil, err
	}
	rehearsal, rehearsalData, err := LoadProviderActivationRehearsal(activationRehearsalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationRehearsalSHA256 = sha256Hex(rehearsalData)
		if !rehearsal.ActivationRehearsalReady || !rehearsal.ActivationSequenceValidated {
			failures = append(failures, "activation rehearsal must be ready with sequence validated")
		}
		if rehearsal.ActivationAllowedNow || rehearsal.ProviderCall || rehearsal.NetworkCall || rehearsal.SecretValuesRead || rehearsal.TransportCalled || rehearsal.WorkspaceModified {
			failures = append(failures, "activation rehearsal must keep execution flags blocked")
		}
	}

	if err := validateRelativeSafePath("response change proposal report path", responseChangeProposalReportPath); err != nil {
		return providerActivationReleaseChain{}, nil, err
	}
	changeReportData, err := readArtifactBytesNoTextExcerpt("response change proposal report", responseChangeProposalReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.changeProposalReportSHA256 = sha256Hex(changeReportData)
		var changeReport ProviderResponseChangeProposalResult
		if err := json.Unmarshal(changeReportData, &changeReport); err != nil {
			failures = append(failures, fmt.Sprintf("parse response change proposal report: %v", err))
		} else {
			if !changeReport.ChangeProposalReady {
				failures = append(failures, "response change proposal report change_proposal_ready must be true")
			}
			if changeReport.WorkspaceModified || changeReport.DiffApplied || changeReport.CommitCreated || changeReport.PRCreated || changeReport.ProviderCall || changeReport.NetworkCall || changeReport.TransportCalled || changeReport.SentToProvider {
				failures = append(failures, "response change proposal report must keep execution flags blocked")
			}
		}
	}
	return chain, failures, nil
}

func blockedProviderActivationReleasePackageResult(chain providerActivationReleaseChain, failures []string) ProviderActivationReleasePackageResult {
	result := ProviderActivationReleasePackageResult{
		Status:                     lancedbpolicy.StatusOK,
		RealActivationSupportedNow: false,
		ActivationAllowedNow:       false,
		ProviderCall:               false,
		NetworkCall:                false,
		SecretValuesRead:           false,
		TransportCalled:            false,
		SentToProvider:             false,
		ReceivedFromProvider:       false,
		WorkspaceModified:          false,
		DiffApplied:                false,
		CommitCreated:              false,
		PRCreated:                  false,
		WorkerExecution:            false,
		PromptInjectionRealRunner:  false,
		BlockedReason:              ProviderCallExecutorBlockedReason,
		ActivationPolicyPlanSHA256: chain.activationPolicyPlanSHA256,
		ActivationApprovalSHA256:   chain.activationApprovalSHA256,
		ActivationRehearsalSHA256:  chain.activationRehearsalSHA256,
		ReadinessAuditSHA256:       chain.readinessAuditSHA256,
		RealCallProposalSHA256:     chain.realCallProposalSHA256,
		SimulationReportSHA256:     chain.simulationReportSHA256,
		CredentialPolicyPlanSHA256: chain.credentialPolicyPlanSHA256,
		ChangeProposalReportSHA256: chain.changeProposalReportSHA256,
		ProviderPayloadSHA256:      chain.providerPayloadSHA256,
		Failures:                   failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderActivationReleasePackage(path string) (ProviderActivationReleasePackageResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider activation release package", path)
	if err != nil {
		return ProviderActivationReleasePackageResult{}, nil, err
	}
	var result ProviderActivationReleasePackageResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderActivationReleasePackageResult{}, nil, fmt.Errorf("parse provider activation release package json: %w", err)
	}
	return result, data, nil
}

func writeProviderActivationReleasePackageJSON(path string, result ProviderActivationReleasePackageResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation release package output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation release package json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation release package must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderActivationReleasePackageText(result ProviderActivationReleasePackageResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_release_package:"); err != nil {
		return err
	}
	return writeProviderActivationReleaseLines(result.ActivationReleasePackageReady, result.RealActivationSupportedNow, result.ActivationAllowedNow, result.Status, result.BlockedReason, result.Failures, out)
}

func WriteProviderActivationReleaseGateText(result ProviderActivationReleaseGateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_release_gate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"activation_gate_ready", fmt.Sprintf("%t", result.ActivationGateReady)},
		{"activation_release_package_ready", fmt.Sprintf("%t", result.ActivationReleasePackageReady)},
		{"real_activation_supported_now", fmt.Sprintf("%t", result.RealActivationSupportedNow)},
		{"activation_allowed_now", fmt.Sprintf("%t", result.ActivationAllowedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"received_from_provider", fmt.Sprintf("%t", result.ReceivedFromProvider)},
		{"workspace_modified", fmt.Sprintf("%t", result.WorkspaceModified)},
		{"diff_applied", fmt.Sprintf("%t", result.DiffApplied)},
		{"commit_created", fmt.Sprintf("%t", result.CommitCreated)},
		{"pr_created", fmt.Sprintf("%t", result.PRCreated)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
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
		return fmt.Errorf("provider activation release gate text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderActivationReleasePackageJSON(result ProviderActivationReleasePackageResult, out io.Writer) error {
	return writeProviderActivationReleasePackageOutJSON(result, out)
}

func WriteProviderActivationReleaseGateJSON(result ProviderActivationReleaseGateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation release gate json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation release gate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}

func writeProviderActivationReleaseLines(packageReady, realSupported, allowedNow bool, status, blockedReason string, failures []string, out io.Writer) error {
	lines := []struct {
		label string
		value string
	}{
		{"status", status},
		{"activation_release_package_ready", fmt.Sprintf("%t", packageReady)},
		{"real_activation_supported_now", fmt.Sprintf("%t", realSupported)},
		{"activation_allowed_now", fmt.Sprintf("%t", allowedNow)},
		{"provider_call", "false"},
		{"network_call", "false"},
		{"secret_values_read", "false"},
		{"transport_called", "false"},
		{"sent_to_provider", "false"},
		{"received_from_provider", "false"},
		{"workspace_modified", "false"},
		{"diff_applied", "false"},
		{"commit_created", "false"},
		{"pr_created", "false"},
		{"worker_execution", "false"},
		{"blocked_reason", blockedReason},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeProviderActivationReleasePackageOutJSON(result ProviderActivationReleasePackageResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation release package json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation release package json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
