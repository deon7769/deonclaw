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

const (
	AllowedUseProviderActivationPolicyOnly = "provider_activation_policy_only"
)

type ProviderActivationApprovalRequest struct {
	Status                        string    `json:"status"`
	CreatedAt                     time.Time `json:"created_at"`
	ActivationPolicyPlanSHA256    string    `json:"activation_policy_plan_sha256"`
	ReadinessAuditSHA256          string    `json:"readiness_audit_sha256"`
	RealCallProposalSHA256        string    `json:"real_call_proposal_sha256"`
	CredentialPolicyPlanSHA256    string    `json:"credential_policy_plan_sha256"`
	SimulationReportSHA256        string    `json:"simulation_report_sha256"`
	ProviderPayloadSHA256         string    `json:"provider_payload_sha256"`
	RequestedActivationAuthorized bool      `json:"requested_activation_authorized"`
	ActivationAllowedNow          bool      `json:"activation_allowed_now"`
	ProviderCallAllowedNow        bool      `json:"provider_call_allowed_now"`
	ProviderCall                  bool      `json:"provider_call"`
	NetworkCall                   bool      `json:"network_call"`
	TransportCalled               bool      `json:"transport_called"`
	SentToProvider                bool      `json:"sent_to_provider"`
	SecretValuesRead              bool      `json:"secret_values_read"`
	WorkspaceModified             bool      `json:"workspace_modified"`
	ContainsText                  bool      `json:"contains_text"`
	BlockedReason                 string    `json:"blocked_reason"`
	Warnings                      []string  `json:"warnings,omitempty"`
}

type ProviderActivationApproval struct {
	Approved                      bool      `json:"approved"`
	ApprovedAt                    time.Time `json:"approved_at"`
	RequestSHA256                 string    `json:"request_sha256"`
	ActivationPolicyPlanSHA256    string    `json:"activation_policy_plan_sha256"`
	ReadinessAuditSHA256          string    `json:"readiness_audit_sha256"`
	RealCallProposalSHA256        string    `json:"real_call_proposal_sha256"`
	CredentialPolicyPlanSHA256    string    `json:"credential_policy_plan_sha256"`
	SimulationReportSHA256        string    `json:"simulation_report_sha256"`
	ProviderPayloadSHA256         string    `json:"provider_payload_sha256"`
	AllowedUse                    string    `json:"allowed_use"`
	ActivationAuthorizedForFuture bool      `json:"activation_authorized_for_future"`
	ActivationAllowedNow          bool      `json:"activation_allowed_now"`
	ProviderCallAllowedNow        bool      `json:"provider_call_allowed_now"`
	ProviderCall                  bool      `json:"provider_call"`
	NetworkCall                   bool      `json:"network_call"`
	TransportCalled               bool      `json:"transport_called"`
	SentToProvider                bool      `json:"sent_to_provider"`
	SecretValuesRead              bool      `json:"secret_values_read"`
	WorkspaceModified             bool      `json:"workspace_modified"`
	ConfirmActivationPolicySHA256 bool      `json:"confirm_activation_policy_sha256"`
	ConfirmReadinessAuditSHA256   bool      `json:"confirm_readiness_audit_sha256"`
	ConfirmRealCallProposalSHA256 bool      `json:"confirm_real_call_proposal_sha256"`
	ConfirmProviderPayloadSHA256  bool      `json:"confirm_provider_payload_sha256"`
	BlockedReason                 string    `json:"blocked_reason"`
	Warnings                      []string  `json:"warnings,omitempty"`
}

type NewProviderActivationApprovalRequestOptions struct {
	ActivationPolicyPlanPath      string
	ActivationReadinessAuditPath  string
	RealCallProposalPath          string
	CredentialPolicyPlanPath      string
	ExecutionSimulationReportPath string
	OutputPath                    string
}

type ApproveProviderActivationOptions struct {
	RequestPath                   string
	OutputPath                    string
	ConfirmActivationPolicySHA256 string
	ConfirmReadinessAuditSHA256   string
	ConfirmRealCallProposalSHA256 string
	ConfirmProviderPayloadSHA256  string
	ApprovedAt                    time.Time
}

type InspectProviderActivationApprovalOptions struct {
	RequestPath string
}

type InspectProviderActivationApprovalResult struct {
	Status                        string   `json:"status"`
	Approved                      bool     `json:"approved"`
	ActivationAuthorizedForFuture bool     `json:"activation_authorized_for_future"`
	ActivationAllowedNow          bool     `json:"activation_allowed_now"`
	ProviderCallAllowedNow        bool     `json:"provider_call_allowed_now"`
	ProviderCall                  bool     `json:"provider_call"`
	NetworkCall                   bool     `json:"network_call"`
	TransportCalled               bool     `json:"transport_called"`
	SentToProvider                bool     `json:"sent_to_provider"`
	SecretValuesRead              bool     `json:"secret_values_read"`
	WorkspaceModified             bool     `json:"workspace_modified"`
	AllowedUse                    string   `json:"allowed_use"`
	ProviderPayloadSHA256         string   `json:"provider_payload_sha256"`
	BlockedReason                 string   `json:"blocked_reason"`
	Warnings                      []string `json:"warnings"`
	Failures                      []string `json:"failures,omitempty"`
}

func NewProviderActivationApprovalRequest(opts NewProviderActivationApprovalRequestOptions) (ProviderActivationApprovalRequest, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"activation policy plan path", opts.ActivationPolicyPlanPath},
		{"activation readiness audit path", opts.ActivationReadinessAuditPath},
		{"real call proposal path", opts.RealCallProposalPath},
		{"credential policy plan path", opts.CredentialPolicyPlanPath},
		{"execution simulation report path", opts.ExecutionSimulationReportPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderActivationApprovalRequest{}, err
		}
	}

	chain, failures, err := loadProviderActivationApprovalChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.CredentialPolicyPlanPath,
		opts.ExecutionSimulationReportPath,
	)
	if err != nil {
		return ProviderActivationApprovalRequest{}, err
	}

	request := ProviderActivationApprovalRequest{
		Status:                        lancedbpolicy.StatusOK,
		CreatedAt:                     time.Now().UTC(),
		ActivationPolicyPlanSHA256:    chain.activationPolicyPlanSHA256,
		ReadinessAuditSHA256:          chain.readinessAuditSHA256,
		RealCallProposalSHA256:        chain.realCallProposalSHA256,
		CredentialPolicyPlanSHA256:    chain.credentialPolicyPlanSHA256,
		SimulationReportSHA256:        chain.simulationReportSHA256,
		ProviderPayloadSHA256:         chain.providerPayloadSHA256,
		RequestedActivationAuthorized: true,
		ActivationAllowedNow:          false,
		ProviderCallAllowedNow:        false,
		ProviderCall:                  false,
		NetworkCall:                   false,
		TransportCalled:               false,
		SentToProvider:                false,
		SecretValuesRead:              false,
		WorkspaceModified:             false,
		ContainsText:                  false,
		BlockedReason:                 ProviderCallExecutorBlockedReason,
	}
	if len(failures) > 0 {
		request.Status = lancedbpolicy.StatusFailed
		request.RequestedActivationAuthorized = false
	}
	if err := writeProviderActivationApprovalRequestJSON(opts.OutputPath, request); err != nil {
		return ProviderActivationApprovalRequest{}, err
	}
	if len(failures) > 0 {
		return request, fmt.Errorf("provider activation approval request validation failed: %s", strings.Join(failures, "; "))
	}
	return request, nil
}

func ApproveProviderActivation(opts ApproveProviderActivationOptions) (ProviderActivationApproval, error) {
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return ProviderActivationApproval{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderActivationApproval{}, err
	}
	request, requestData, err := LoadProviderActivationApprovalRequest(opts.RequestPath)
	if err != nil {
		return ProviderActivationApproval{}, err
	}
	if request.Status == lancedbpolicy.StatusFailed || !request.RequestedActivationAuthorized {
		return ProviderActivationApproval{}, fmt.Errorf("provider activation approval request is not valid for approval")
	}
	if opts.ConfirmActivationPolicySHA256 == "" {
		return ProviderActivationApproval{}, fmt.Errorf("--confirm-activation-policy-sha256 is required")
	}
	if opts.ConfirmReadinessAuditSHA256 == "" {
		return ProviderActivationApproval{}, fmt.Errorf("--confirm-readiness-audit-sha256 is required")
	}
	if opts.ConfirmRealCallProposalSHA256 == "" {
		return ProviderActivationApproval{}, fmt.Errorf("--confirm-real-call-proposal-sha256 is required")
	}
	if opts.ConfirmProviderPayloadSHA256 == "" {
		return ProviderActivationApproval{}, fmt.Errorf("--confirm-provider-payload-sha256 is required")
	}
	if opts.ConfirmActivationPolicySHA256 != request.ActivationPolicyPlanSHA256 {
		return ProviderActivationApproval{}, fmt.Errorf("confirm activation policy sha256 mismatch")
	}
	if opts.ConfirmReadinessAuditSHA256 != request.ReadinessAuditSHA256 {
		return ProviderActivationApproval{}, fmt.Errorf("confirm readiness audit sha256 mismatch")
	}
	if opts.ConfirmRealCallProposalSHA256 != request.RealCallProposalSHA256 {
		return ProviderActivationApproval{}, fmt.Errorf("confirm real call proposal sha256 mismatch")
	}
	if opts.ConfirmProviderPayloadSHA256 != request.ProviderPayloadSHA256 {
		return ProviderActivationApproval{}, fmt.Errorf("confirm provider payload sha256 mismatch")
	}

	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}
	approval := ProviderActivationApproval{
		Approved:                      true,
		ApprovedAt:                    approvedAt,
		RequestSHA256:                 sha256Hex(requestData),
		ActivationPolicyPlanSHA256:    request.ActivationPolicyPlanSHA256,
		ReadinessAuditSHA256:          request.ReadinessAuditSHA256,
		RealCallProposalSHA256:        request.RealCallProposalSHA256,
		CredentialPolicyPlanSHA256:    request.CredentialPolicyPlanSHA256,
		SimulationReportSHA256:        request.SimulationReportSHA256,
		ProviderPayloadSHA256:         request.ProviderPayloadSHA256,
		AllowedUse:                    AllowedUseProviderActivationPolicyOnly,
		ActivationAuthorizedForFuture: true,
		ActivationAllowedNow:          false,
		ProviderCallAllowedNow:        false,
		ProviderCall:                  false,
		NetworkCall:                   false,
		TransportCalled:               false,
		SentToProvider:                false,
		SecretValuesRead:              false,
		WorkspaceModified:             false,
		ConfirmActivationPolicySHA256: true,
		ConfirmReadinessAuditSHA256:   true,
		ConfirmRealCallProposalSHA256: true,
		ConfirmProviderPayloadSHA256:  true,
		BlockedReason:                 ProviderCallExecutorBlockedReason,
	}
	if err := writeProviderActivationApprovalJSON(opts.OutputPath, approval); err != nil {
		return ProviderActivationApproval{}, err
	}
	return approval, nil
}

func InspectProviderActivationApproval(approvalPath string, opts InspectProviderActivationApprovalOptions) (InspectProviderActivationApprovalResult, error) {
	if err := validateRelativeSafePath("approval path", approvalPath); err != nil {
		return InspectProviderActivationApprovalResult{}, err
	}
	approval, approvalData, err := LoadProviderActivationApproval(approvalPath)
	if err != nil {
		return InspectProviderActivationApprovalResult{}, err
	}
	result := InspectProviderActivationApprovalResult{
		Status:                        lancedbpolicy.StatusOK,
		Approved:                      approval.Approved,
		ActivationAuthorizedForFuture: approval.ActivationAuthorizedForFuture,
		ActivationAllowedNow:          approval.ActivationAllowedNow,
		ProviderCallAllowedNow:        approval.ProviderCallAllowedNow,
		ProviderCall:                  approval.ProviderCall,
		NetworkCall:                   approval.NetworkCall,
		TransportCalled:               approval.TransportCalled,
		SentToProvider:                approval.SentToProvider,
		SecretValuesRead:              approval.SecretValuesRead,
		WorkspaceModified:             approval.WorkspaceModified,
		AllowedUse:                    approval.AllowedUse,
		ProviderPayloadSHA256:         approval.ProviderPayloadSHA256,
		BlockedReason:                 approval.BlockedReason,
	}
	var failures []string
	if strings.Contains(string(approvalData), "text_excerpt") || strings.Contains(string(approvalData), "alpha text") {
		failures = append(failures, "approval must not contain materialized preview text")
	}
	if !approval.Approved {
		failures = append(failures, "approval.approved must be true")
	}
	if !approval.ActivationAuthorizedForFuture {
		failures = append(failures, "approval.activation_authorized_for_future must be true")
	}
	if approval.ActivationAllowedNow || approval.ProviderCallAllowedNow || approval.ProviderCall || approval.NetworkCall || approval.TransportCalled || approval.SentToProvider || approval.SecretValuesRead || approval.WorkspaceModified {
		failures = append(failures, "approval must keep execution flags blocked")
	}
	if opts.RequestPath != "" {
		if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
			return InspectProviderActivationApprovalResult{}, err
		}
		request, _, err := LoadProviderActivationApprovalRequest(opts.RequestPath)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			if request.ActivationPolicyPlanSHA256 != approval.ActivationPolicyPlanSHA256 {
				failures = append(failures, "activation policy plan sha256 mismatch between request and approval")
			}
			if request.ReadinessAuditSHA256 != approval.ReadinessAuditSHA256 {
				failures = append(failures, "readiness audit sha256 mismatch between request and approval")
			}
			if request.RealCallProposalSHA256 != approval.RealCallProposalSHA256 {
				failures = append(failures, "real call proposal sha256 mismatch between request and approval")
			}
			if request.ProviderPayloadSHA256 != approval.ProviderPayloadSHA256 {
				failures = append(failures, "provider payload sha256 mismatch between request and approval")
			}
		}
	}
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

type providerActivationApprovalChain struct {
	activationPolicyPlanSHA256 string
	readinessAuditSHA256       string
	realCallProposalSHA256     string
	credentialPolicyPlanSHA256 string
	simulationReportSHA256     string
	providerPayloadSHA256      string
}

func loadProviderActivationApprovalChain(activationPolicyPlanPath, activationReadinessAuditPath, realCallProposalPath, credentialPolicyPlanPath, executionSimulationReportPath string) (providerActivationApprovalChain, []string, error) {
	var chain providerActivationApprovalChain
	var failures []string

	policyPlan, policyData, err := LoadProviderActivationPolicyPlan(activationPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationPolicyPlanSHA256 = sha256Hex(policyData)
		if !policyPlan.ActivationPolicyPlanReady || !policyPlan.ActivationPolicyValidated {
			failures = append(failures, "activation policy plan must be ready and validated")
		}
		if policyPlan.ActivationAllowedNow || policyPlan.ProviderCall || policyPlan.NetworkCall || policyPlan.SecretValuesRead || policyPlan.TransportCalled || policyPlan.WorkspaceModified {
			failures = append(failures, "activation policy plan must keep execution flags blocked")
		}
	}

	audit, auditData, err := LoadProviderActivationReadinessAudit(activationReadinessAuditPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.readinessAuditSHA256 = sha256Hex(auditData)
		if !audit.ActivationReadinessReady {
			failures = append(failures, "activation readiness audit activation_readiness_ready must be true")
		}
		if audit.ActivationAllowedNow || audit.ProviderCall || audit.NetworkCall || audit.TransportCalled || audit.SecretValuesRead || audit.WorkspaceModified {
			failures = append(failures, "activation readiness audit must keep execution flags blocked")
		}
	}

	proposal, proposalData, err := LoadProviderRealCallProposal(realCallProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.realCallProposalSHA256 = sha256Hex(proposalData)
		chain.providerPayloadSHA256 = proposal.ProviderPayloadSHA256
		if !proposal.RealCallProposalReady {
			failures = append(failures, "real call proposal real_call_proposal_ready must be true")
		}
		if proposal.ProviderCallAllowedNow || proposal.ProviderCall || proposal.NetworkCall || proposal.TransportCalled || proposal.SentToProvider || proposal.SecretValuesRead || proposal.WorkspaceModified {
			failures = append(failures, "real call proposal must keep execution flags blocked")
		}
	}

	credentialPlan, credentialData, err := LoadProviderCredentialPolicyPlan(credentialPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.credentialPolicyPlanSHA256 = sha256Hex(credentialData)
		if !credentialPlan.CredentialPolicyPlanReady || credentialPlan.SecretValuesRead {
			failures = append(failures, "credential policy plan must be ready with no secret reads")
		}
	}

	simulationData, err := readArtifactBytesNoTextExcerpt("execution simulation report", executionSimulationReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.simulationReportSHA256 = sha256Hex(simulationData)
		var simulation ProviderExecutionSimulationBundleResult
		if err := json.Unmarshal(simulationData, &simulation); err != nil {
			failures = append(failures, fmt.Sprintf("parse execution simulation report: %v", err))
		} else {
			if !simulation.SimulationBundleReady {
				failures = append(failures, "execution simulation report simulation_bundle_ready must be true")
			}
			if simulation.ProviderCall || simulation.NetworkCall || simulation.TransportCalled || simulation.SentToProvider || simulation.ReceivedFromProvider || simulation.WorkerExecution || simulation.ActivationAllowedNow {
				failures = append(failures, "execution simulation report must keep execution flags blocked")
			}
			if chain.providerPayloadSHA256 != "" && simulation.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 != simulation.ProviderPayloadSHA256 {
				failures = append(failures, "provider_payload_sha256 mismatch between real call proposal and simulation report")
			}
			if chain.providerPayloadSHA256 == "" {
				chain.providerPayloadSHA256 = simulation.ProviderPayloadSHA256
			}
		}
	}

	if chain.providerPayloadSHA256 == "" {
		failures = append(failures, "provider_payload_sha256 is required")
	}
	return chain, failures, nil
}

func LoadProviderActivationApprovalRequest(path string) (ProviderActivationApprovalRequest, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider activation approval request", path)
	if err != nil {
		return ProviderActivationApprovalRequest{}, nil, err
	}
	var request ProviderActivationApprovalRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return ProviderActivationApprovalRequest{}, nil, fmt.Errorf("parse provider activation approval request json: %w", err)
	}
	return request, data, nil
}

func LoadProviderActivationApproval(path string) (ProviderActivationApproval, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider activation approval", path)
	if err != nil {
		return ProviderActivationApproval{}, nil, err
	}
	var approval ProviderActivationApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return ProviderActivationApproval{}, nil, fmt.Errorf("parse provider activation approval json: %w", err)
	}
	return approval, data, nil
}

func writeProviderActivationApprovalRequestJSON(path string, request ProviderActivationApprovalRequest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation approval request output dir: %w", err)
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation approval request json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation approval request must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func writeProviderActivationApprovalJSON(path string, approval ProviderActivationApproval) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation approval output dir: %w", err)
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation approval json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation approval must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderActivationApprovalInspectText(result InspectProviderActivationApprovalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_approval_inspect:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"approved", fmt.Sprintf("%t", result.Approved)},
		{"activation_authorized_for_future", fmt.Sprintf("%t", result.ActivationAuthorizedForFuture)},
		{"activation_allowed_now", fmt.Sprintf("%t", result.ActivationAllowedNow)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"workspace_modified", fmt.Sprintf("%t", result.WorkspaceModified)},
		{"allowed_use", result.AllowedUse},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
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
		return fmt.Errorf("provider activation approval inspect text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderActivationApprovalInspectJSON(result InspectProviderActivationApprovalResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation approval inspect json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation approval inspect json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
