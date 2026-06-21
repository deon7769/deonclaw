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

var providerActivationRehearsalFutureSteps = []string{
	"validate_activation_policy",
	"inspect_activation_approval",
	"verify_credential_policy_names_only",
	"verify_real_call_proposal",
	"verify_readiness_audit",
	"blocked_before_real_dispatch",
}

type ProviderActivationRehearsalOptions struct {
	ActivationPolicyPlanPath      string
	ActivationApprovalPath        string
	ActivationReadinessAuditPath  string
	RealCallProposalPath          string
	CredentialPolicyPlanPath      string
	ExecutionSimulationReportPath string
	OutputPath                    string
}

type ProviderActivationRehearsalResult struct {
	Status                      string   `json:"status"`
	ActivationRehearsalReady    bool     `json:"activation_rehearsal_ready"`
	ActivationSequenceValidated bool     `json:"activation_sequence_validated"`
	ActivationAllowedNow        bool     `json:"activation_allowed_now"`
	ProviderCall                bool     `json:"provider_call"`
	NetworkCall                 bool     `json:"network_call"`
	SecretValuesRead            bool     `json:"secret_values_read"`
	TransportCalled             bool     `json:"transport_called"`
	WorkspaceModified           bool     `json:"workspace_modified"`
	BlockedReason               string   `json:"blocked_reason"`
	FutureSteps                 []string `json:"future_steps,omitempty"`
	ActivationPolicyPlanSHA256  string   `json:"activation_policy_plan_sha256,omitempty"`
	ActivationApprovalSHA256    string   `json:"activation_approval_sha256,omitempty"`
	ReadinessAuditSHA256        string   `json:"readiness_audit_sha256,omitempty"`
	RealCallProposalSHA256      string   `json:"real_call_proposal_sha256,omitempty"`
	CredentialPolicyPlanSHA256  string   `json:"credential_policy_plan_sha256,omitempty"`
	SimulationReportSHA256      string   `json:"simulation_report_sha256,omitempty"`
	ProviderPayloadSHA256       string   `json:"provider_payload_sha256,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
	Failures                    []string `json:"failures,omitempty"`
}

type ProviderActivationRehearsalReportOptions struct {
	RehearsalPath                 string
	ActivationPolicyPlanPath      string
	ActivationApprovalPath        string
	ActivationReadinessAuditPath  string
	RealCallProposalPath          string
	CredentialPolicyPlanPath      string
	ExecutionSimulationReportPath string
}

func ProviderActivationRehearsal(opts ProviderActivationRehearsalOptions) (ProviderActivationRehearsalResult, error) {
	chain, failures, err := loadProviderActivationRehearsalChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationApprovalPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.CredentialPolicyPlanPath,
		opts.ExecutionSimulationReportPath,
	)
	if err != nil {
		return ProviderActivationRehearsalResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderActivationRehearsalResult{}, err
	}

	result := ProviderActivationRehearsalResult{
		Status:                     lancedbpolicy.StatusOK,
		ActivationAllowedNow:       false,
		ProviderCall:               false,
		NetworkCall:                false,
		SecretValuesRead:           false,
		TransportCalled:            false,
		WorkspaceModified:          false,
		BlockedReason:              ProviderCallExecutorBlockedReason,
		FutureSteps:                append([]string(nil), providerActivationRehearsalFutureSteps...),
		ActivationPolicyPlanSHA256: chain.activationPolicyPlanSHA256,
		ActivationApprovalSHA256:   chain.activationApprovalSHA256,
		ReadinessAuditSHA256:       chain.readinessAuditSHA256,
		RealCallProposalSHA256:     chain.realCallProposalSHA256,
		CredentialPolicyPlanSHA256: chain.credentialPolicyPlanSHA256,
		SimulationReportSHA256:     chain.simulationReportSHA256,
		ProviderPayloadSHA256:      chain.providerPayloadSHA256,
		Failures:                   failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.ActivationRehearsalReady = true
		result.ActivationSequenceValidated = true
	}
	if err := writeProviderActivationRehearsalJSON(opts.OutputPath, result); err != nil {
		return ProviderActivationRehearsalResult{}, err
	}
	return result, nil
}

func ProviderActivationRehearsalReport(opts ProviderActivationRehearsalReportOptions) (ProviderActivationRehearsalResult, error) {
	if err := validateRelativeSafePath("rehearsal path", opts.RehearsalPath); err != nil {
		return ProviderActivationRehearsalResult{}, err
	}
	stored, _, err := LoadProviderActivationRehearsal(opts.RehearsalPath)
	if err != nil {
		return ProviderActivationRehearsalResult{}, err
	}
	chain, failures, err := loadProviderActivationRehearsalChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationApprovalPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.CredentialPolicyPlanPath,
		opts.ExecutionSimulationReportPath,
	)
	if err != nil {
		return ProviderActivationRehearsalResult{}, err
	}
	if !stored.ActivationRehearsalReady {
		failures = append(failures, "stored rehearsal activation_rehearsal_ready must be true")
	}
	reconcileProviderExecutorHash("rehearsal", stored.ActivationPolicyPlanSHA256, chain.activationPolicyPlanSHA256, &failures)
	reconcileProviderExecutorHash("rehearsal", stored.ActivationApprovalSHA256, chain.activationApprovalSHA256, &failures)
	reconcileProviderExecutorHash("rehearsal", stored.ReadinessAuditSHA256, chain.readinessAuditSHA256, &failures)
	reconcileProviderExecutorHash("rehearsal", stored.RealCallProposalSHA256, chain.realCallProposalSHA256, &failures)
	reconcileProviderExecutorHash("rehearsal", stored.CredentialPolicyPlanSHA256, chain.credentialPolicyPlanSHA256, &failures)
	reconcileProviderExecutorHash("rehearsal", stored.SimulationReportSHA256, chain.simulationReportSHA256, &failures)
	if stored.ProviderCall || stored.NetworkCall || stored.SecretValuesRead || stored.TransportCalled || stored.WorkspaceModified || stored.ActivationAllowedNow {
		failures = append(failures, "stored rehearsal must keep execution flags blocked")
	}

	result := ProviderActivationRehearsalResult{
		Status:                     lancedbpolicy.StatusOK,
		ActivationAllowedNow:       false,
		ProviderCall:               false,
		NetworkCall:                false,
		SecretValuesRead:           false,
		TransportCalled:            false,
		WorkspaceModified:          false,
		BlockedReason:              ProviderCallExecutorBlockedReason,
		FutureSteps:                append([]string(nil), stored.FutureSteps...),
		ActivationPolicyPlanSHA256: chain.activationPolicyPlanSHA256,
		ActivationApprovalSHA256:   chain.activationApprovalSHA256,
		ReadinessAuditSHA256:       chain.readinessAuditSHA256,
		RealCallProposalSHA256:     chain.realCallProposalSHA256,
		CredentialPolicyPlanSHA256: chain.credentialPolicyPlanSHA256,
		SimulationReportSHA256:     chain.simulationReportSHA256,
		ProviderPayloadSHA256:      chain.providerPayloadSHA256,
		Failures:                   failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.ActivationRehearsalReady = true
		result.ActivationSequenceValidated = true
	}
	return result, nil
}

type providerActivationRehearsalChain struct {
	activationPolicyPlanSHA256 string
	activationApprovalSHA256   string
	readinessAuditSHA256       string
	realCallProposalSHA256     string
	credentialPolicyPlanSHA256 string
	simulationReportSHA256     string
	providerPayloadSHA256      string
}

func loadProviderActivationRehearsalChain(activationPolicyPlanPath, activationApprovalPath, activationReadinessAuditPath, realCallProposalPath, credentialPolicyPlanPath, executionSimulationReportPath string) (providerActivationRehearsalChain, []string, error) {
	approvalChain, failures, err := loadProviderActivationApprovalChain(
		activationPolicyPlanPath,
		activationReadinessAuditPath,
		realCallProposalPath,
		credentialPolicyPlanPath,
		executionSimulationReportPath,
	)
	if err != nil {
		return providerActivationRehearsalChain{}, nil, err
	}
	var chain providerActivationRehearsalChain
	chain.activationPolicyPlanSHA256 = approvalChain.activationPolicyPlanSHA256
	chain.readinessAuditSHA256 = approvalChain.readinessAuditSHA256
	chain.realCallProposalSHA256 = approvalChain.realCallProposalSHA256
	chain.credentialPolicyPlanSHA256 = approvalChain.credentialPolicyPlanSHA256
	chain.simulationReportSHA256 = approvalChain.simulationReportSHA256
	chain.providerPayloadSHA256 = approvalChain.providerPayloadSHA256

	if err := validateRelativeSafePath("activation approval path", activationApprovalPath); err != nil {
		return providerActivationRehearsalChain{}, nil, err
	}
	approval, approvalData, err := LoadProviderActivationApproval(activationApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationApprovalSHA256 = sha256Hex(approvalData)
		if !approval.Approved || !approval.ActivationAuthorizedForFuture {
			failures = append(failures, "activation approval must be approved for future activation")
		}
		if approval.ActivationAllowedNow || approval.ProviderCallAllowedNow || approval.ProviderCall || approval.NetworkCall || approval.TransportCalled || approval.SentToProvider || approval.SecretValuesRead || approval.WorkspaceModified {
			failures = append(failures, "activation approval must keep execution flags blocked")
		}
		reconcileProviderExecutorHash("activation approval", approval.ActivationPolicyPlanSHA256, chain.activationPolicyPlanSHA256, &failures)
		reconcileProviderExecutorHash("activation approval", approval.ReadinessAuditSHA256, chain.readinessAuditSHA256, &failures)
		reconcileProviderExecutorHash("activation approval", approval.RealCallProposalSHA256, chain.realCallProposalSHA256, &failures)
		reconcileProviderExecutorHash("activation approval", approval.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
	}
	return chain, failures, nil
}

func LoadProviderActivationRehearsal(path string) (ProviderActivationRehearsalResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider activation rehearsal", path)
	if err != nil {
		return ProviderActivationRehearsalResult{}, nil, err
	}
	var result ProviderActivationRehearsalResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderActivationRehearsalResult{}, nil, fmt.Errorf("parse provider activation rehearsal json: %w", err)
	}
	return result, data, nil
}

func writeProviderActivationRehearsalJSON(path string, result ProviderActivationRehearsalResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation rehearsal output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation rehearsal json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation rehearsal must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderActivationRehearsalText(result ProviderActivationRehearsalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_rehearsal:"); err != nil {
		return err
	}
	return writeProviderActivationRehearsalLines(result, out)
}

func WriteProviderActivationRehearsalReportText(result ProviderActivationRehearsalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_rehearsal_report:"); err != nil {
		return err
	}
	return writeProviderActivationRehearsalLines(result, out)
}

func WriteProviderActivationRehearsalJSON(result ProviderActivationRehearsalResult, out io.Writer) error {
	return writeProviderActivationRehearsalOutJSON(result, out)
}

func WriteProviderActivationRehearsalReportJSON(result ProviderActivationRehearsalResult, out io.Writer) error {
	return writeProviderActivationRehearsalOutJSON(result, out)
}

func writeProviderActivationRehearsalLines(result ProviderActivationRehearsalResult, out io.Writer) error {
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"activation_rehearsal_ready", fmt.Sprintf("%t", result.ActivationRehearsalReady)},
		{"activation_sequence_validated", fmt.Sprintf("%t", result.ActivationSequenceValidated)},
		{"activation_allowed_now", fmt.Sprintf("%t", result.ActivationAllowedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"workspace_modified", fmt.Sprintf("%t", result.WorkspaceModified)},
		{"blocked_reason", result.BlockedReason},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.FutureSteps) > 0 {
		if _, err := fmt.Fprintln(out, "\nfuture_steps:"); err != nil {
			return err
		}
		for _, step := range result.FutureSteps {
			if _, err := fmt.Fprintf(out, "- %s\n", step); err != nil {
				return err
			}
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
		return fmt.Errorf("provider activation rehearsal text must not contain materialized preview text")
	}
	return nil
}

func writeProviderActivationRehearsalOutJSON(result ProviderActivationRehearsalResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation rehearsal json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation rehearsal json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
