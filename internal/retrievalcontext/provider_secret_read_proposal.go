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

type ProviderSecretReadProposalNewOptions struct {
	CredentialPolicyPlanPath  string
	ActivationFinalAuditPath  string
	ActivationCIReportPath    string
	ActivationReleaseGatePath string
	OutputPath                string
}

type ProviderSecretReadProposal struct {
	Status                      string    `json:"status"`
	CreatedAt                   time.Time `json:"created_at"`
	SecretReadProposalReady     bool      `json:"secret_read_proposal_ready"`
	SecretReadAllowedNow        bool      `json:"secret_read_allowed_now"`
	SecretValuesRead            bool      `json:"secret_values_read"`
	ProviderCall                bool      `json:"provider_call"`
	NetworkCall                 bool      `json:"network_call"`
	TransportCalled             bool      `json:"transport_called"`
	ActivationAllowedNow        bool      `json:"activation_allowed_now"`
	BlockedReason               string    `json:"blocked_reason"`
	AllowedEnvVarNames          []string  `json:"allowed_env_var_names,omitempty"`
	FutureEnvVarNames           []string  `json:"future_env_var_names,omitempty"`
	CredentialPolicyPlanSHA256  string    `json:"credential_policy_plan_sha256,omitempty"`
	ActivationFinalAuditSHA256  string    `json:"activation_final_audit_sha256,omitempty"`
	ActivationCIReportSHA256    string    `json:"activation_ci_report_sha256,omitempty"`
	ActivationReleaseGateSHA256 string    `json:"activation_release_gate_sha256,omitempty"`
	ProviderPayloadSHA256       string    `json:"provider_payload_sha256,omitempty"`
	Warnings                    []string  `json:"warnings,omitempty"`
	Failures                    []string  `json:"failures,omitempty"`
}

type ProviderSecretReadProposalInspectOptions struct {
	CredentialPolicyPlanPath  string
	ActivationFinalAuditPath  string
	ActivationCIReportPath    string
	ActivationReleaseGatePath string
}

type ProviderSecretReadProposalInspectResult struct {
	Status                      string   `json:"status"`
	SecretReadProposalReady     bool     `json:"secret_read_proposal_ready"`
	SecretReadAllowedNow        bool     `json:"secret_read_allowed_now"`
	SecretValuesRead            bool     `json:"secret_values_read"`
	ProviderCall                bool     `json:"provider_call"`
	NetworkCall                 bool     `json:"network_call"`
	TransportCalled             bool     `json:"transport_called"`
	ActivationAllowedNow        bool     `json:"activation_allowed_now"`
	BlockedReason               string   `json:"blocked_reason"`
	CredentialPolicyPlanSHA256  string   `json:"credential_policy_plan_sha256,omitempty"`
	ActivationFinalAuditSHA256  string   `json:"activation_final_audit_sha256,omitempty"`
	ActivationCIReportSHA256    string   `json:"activation_ci_report_sha256,omitempty"`
	ActivationReleaseGateSHA256 string   `json:"activation_release_gate_sha256,omitempty"`
	ProviderPayloadSHA256       string   `json:"provider_payload_sha256,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
	Failures                    []string `json:"failures,omitempty"`
}

type providerSecretReadProposalChain struct {
	credentialPolicyPlanSHA256  string
	activationFinalAuditSHA256  string
	activationCIReportSHA256    string
	activationReleaseGateSHA256 string
	providerPayloadSHA256       string
	allowedEnvVarNames          []string
}

func NewProviderSecretReadProposal(opts ProviderSecretReadProposalNewOptions) (ProviderSecretReadProposal, error) {
	chain, failures, err := loadProviderSecretReadProposalChain(
		opts.CredentialPolicyPlanPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.ActivationReleaseGatePath,
	)
	if err != nil {
		return ProviderSecretReadProposal{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderSecretReadProposal{}, err
	}

	result := blockedProviderSecretReadProposal(chain, failures)
	if len(failures) == 0 {
		result.SecretReadProposalReady = true
	}
	if err := writeProviderSecretReadProposalJSON(opts.OutputPath, result); err != nil {
		return ProviderSecretReadProposal{}, err
	}
	return result, nil
}

func InspectProviderSecretReadProposal(path string, opts ProviderSecretReadProposalInspectOptions) (ProviderSecretReadProposalInspectResult, error) {
	if err := validateRelativeSafePath("secret read proposal path", path); err != nil {
		return ProviderSecretReadProposalInspectResult{}, err
	}
	proposal, proposalData, err := LoadProviderSecretReadProposal(path)
	if err != nil {
		return ProviderSecretReadProposalInspectResult{}, err
	}
	chain, failures, err := loadProviderSecretReadProposalChain(
		opts.CredentialPolicyPlanPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.ActivationReleaseGatePath,
	)
	if err != nil {
		return ProviderSecretReadProposalInspectResult{}, err
	}
	if !proposal.SecretReadProposalReady {
		failures = append(failures, "secret read proposal secret_read_proposal_ready must be true")
	}
	if proposal.SecretReadAllowedNow || proposal.SecretValuesRead {
		failures = append(failures, "secret read proposal must keep secret reads blocked")
	}
	reconcileProviderExecutorHash("secret read proposal", proposal.CredentialPolicyPlanSHA256, chain.credentialPolicyPlanSHA256, &failures)
	reconcileProviderExecutorHash("secret read proposal", proposal.ActivationFinalAuditSHA256, chain.activationFinalAuditSHA256, &failures)
	reconcileProviderExecutorHash("secret read proposal", proposal.ActivationCIReportSHA256, chain.activationCIReportSHA256, &failures)
	reconcileProviderExecutorHash("secret read proposal", proposal.ActivationReleaseGateSHA256, chain.activationReleaseGateSHA256, &failures)
	reconcileProviderExecutorHash("secret read proposal", proposal.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
	if sha256Hex(proposalData) == "" {
		failures = append(failures, "secret read proposal hash missing")
	}

	result := ProviderSecretReadProposalInspectResult{
		Status:                      lancedbpolicy.StatusOK,
		SecretReadProposalReady:     proposal.SecretReadProposalReady,
		SecretReadAllowedNow:        false,
		SecretValuesRead:            false,
		ProviderCall:                false,
		NetworkCall:                 false,
		TransportCalled:             false,
		ActivationAllowedNow:        false,
		BlockedReason:               ProviderCallExecutorBlockedReason,
		CredentialPolicyPlanSHA256:  chain.credentialPolicyPlanSHA256,
		ActivationFinalAuditSHA256:  chain.activationFinalAuditSHA256,
		ActivationCIReportSHA256:    chain.activationCIReportSHA256,
		ActivationReleaseGateSHA256: chain.activationReleaseGateSHA256,
		ProviderPayloadSHA256:       chain.providerPayloadSHA256,
		Failures:                    failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		result.SecretReadProposalReady = false
	}
	return result, nil
}

func loadProviderSecretReadProposalChain(credentialPolicyPlanPath, activationFinalAuditPath, activationCIReportPath, activationReleaseGatePath string) (providerSecretReadProposalChain, []string, error) {
	var chain providerSecretReadProposalChain
	failures := []string{}

	credentialPlan, credentialData, err := LoadProviderCredentialPolicyPlan(credentialPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.credentialPolicyPlanSHA256 = sha256Hex(credentialData)
		chain.allowedEnvVarNames = append([]string(nil), credentialPlan.AllowedEnvVarNames...)
		if !credentialPlan.CredentialPolicyPlanReady || !credentialPlan.CredentialPolicyValidated {
			failures = append(failures, "credential policy plan must be ready and validated")
		}
		if credentialPlan.SecretValuesRead || credentialPlan.ProviderCall || credentialPlan.NetworkCall {
			failures = append(failures, "credential policy plan must keep execution flags blocked")
		}
	}

	finalAudit, finalAuditData, err := LoadProviderActivationFinalAudit(activationFinalAuditPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationFinalAuditSHA256 = sha256Hex(finalAuditData)
		chain.providerPayloadSHA256 = finalAudit.ProviderPayloadSHA256
		if !finalAudit.FinalAuditReady || !finalAudit.KillSwitchActive {
			failures = append(failures, "activation final audit must be ready with kill switch active")
		}
		if finalAudit.ActivationAllowedNow || finalAudit.SecretValuesRead || finalAudit.ProviderCall || finalAudit.NetworkCall || finalAudit.TransportCalled {
			failures = append(failures, "activation final audit must keep execution flags blocked")
		}
	}

	ciReport, ciReportData, err := LoadProviderActivationCIReport(activationCIReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationCIReportSHA256 = sha256Hex(ciReportData)
		if !ciReport.CIObservabilityReady || !ciReport.FinalAuditReady {
			failures = append(failures, "activation ci report must be observability ready")
		}
		if ciReport.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 == "" {
			chain.providerPayloadSHA256 = ciReport.ProviderPayloadSHA256
		}
		if ciReport.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 != "" && ciReport.ProviderPayloadSHA256 != chain.providerPayloadSHA256 {
			failures = append(failures, "activation ci report provider_payload_sha256 mismatch with final audit")
		}
	}

	gate, gateData, err := LoadProviderActivationReleaseGate(activationReleaseGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationReleaseGateSHA256 = sha256Hex(gateData)
		if !gate.ActivationGateReady {
			failures = append(failures, "activation release gate activation_gate_ready must be true")
		}
		if gate.ActivationAllowedNow || gate.RealActivationSupportedNow || gate.ProviderCall || gate.NetworkCall || gate.SecretValuesRead || gate.TransportCalled {
			failures = append(failures, "activation release gate must keep execution flags blocked")
		}
	}

	return chain, failures, nil
}

func blockedProviderSecretReadProposal(chain providerSecretReadProposalChain, failures []string) ProviderSecretReadProposal {
	futureNames := append([]string(nil), chain.allowedEnvVarNames...)
	result := ProviderSecretReadProposal{
		Status:                      lancedbpolicy.StatusOK,
		CreatedAt:                   time.Now().UTC(),
		SecretReadAllowedNow:        false,
		SecretValuesRead:            false,
		ProviderCall:                false,
		NetworkCall:                 false,
		TransportCalled:             false,
		ActivationAllowedNow:        false,
		BlockedReason:               ProviderCallExecutorBlockedReason,
		AllowedEnvVarNames:          append([]string(nil), chain.allowedEnvVarNames...),
		FutureEnvVarNames:           futureNames,
		CredentialPolicyPlanSHA256:  chain.credentialPolicyPlanSHA256,
		ActivationFinalAuditSHA256:  chain.activationFinalAuditSHA256,
		ActivationCIReportSHA256:    chain.activationCIReportSHA256,
		ActivationReleaseGateSHA256: chain.activationReleaseGateSHA256,
		ProviderPayloadSHA256:       chain.providerPayloadSHA256,
		Failures:                    failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderSecretReadProposal(path string) (ProviderSecretReadProposal, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider secret read proposal", path)
	if err != nil {
		return ProviderSecretReadProposal{}, nil, err
	}
	var result ProviderSecretReadProposal
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderSecretReadProposal{}, nil, fmt.Errorf("parse provider secret read proposal json: %w", err)
	}
	return result, data, nil
}

func writeProviderSecretReadProposalJSON(path string, result ProviderSecretReadProposal) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider secret read proposal output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider secret read proposal json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider secret read proposal must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderSecretReadProposalJSON(result ProviderSecretReadProposal, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderSecretReadProposalInspectJSON(result ProviderSecretReadProposalInspectResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}
