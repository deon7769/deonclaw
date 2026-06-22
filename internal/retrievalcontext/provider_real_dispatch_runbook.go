package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

var providerRealDispatchRunbookFutureSteps = []string{
	"verify_external_approval",
	"verify_kill_switch_policy",
	"verify_secret_policy_names_only",
	"verify_transport_plan",
	"verify_dispatch_design",
	"require_human_operator_review",
	"blocked_before_real_execution",
}

var providerRealDispatchRunbookOperationalChecklist = []string{
	"external_approval_authorized_for_future",
	"kill_switch_active",
	"operator_review_required",
	"secret_values_read_false",
	"execute_subcommand_not_registered",
	"transport_not_called",
	"provider_call_false",
}

var providerRealDispatchRunbookRollbackPlan = []string{
	"disable_kill_switch_override_if_enabled",
	"revoke_external_approval_if_compromised",
	"re_run_activation_final_audit",
	"re_run_provider_call_chain_smoke",
	"do_not_apply_workspace_diff_without_review",
}

type ProviderRealDispatchRunbookOptions struct {
	ExternalApprovalPath                string
	ExternalApprovalRequestPath         string
	DesignReviewGatePath                string
	RealDispatchDesignPath              string
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	KillSwitchPlanPath                  string
	OutputPath                          string
}

type ProviderRealDispatchRunbookResult struct {
	Status                   string   `json:"status"`
	RunbookReady             bool     `json:"runbook_ready"`
	RunbookMetadataOnly      bool     `json:"runbook_metadata_only"`
	ScriptGenerated          bool     `json:"script_generated"`
	ExecuteCommandGenerated  bool     `json:"execute_command_generated"`
	ProviderCall             bool     `json:"provider_call"`
	NetworkCall              bool     `json:"network_call"`
	SecretValuesRead         bool     `json:"secret_values_read"`
	TransportCalled          bool     `json:"transport_called"`
	WorkspaceModified        bool     `json:"workspace_modified"`
	BlockedReason            string   `json:"blocked_reason"`
	FutureSteps              []string `json:"future_steps,omitempty"`
	OperationalChecklist     []string `json:"operational_checklist,omitempty"`
	RollbackPlan             []string `json:"rollback_plan,omitempty"`
	ExternalApprovalSHA256   string   `json:"external_approval_sha256,omitempty"`
	DesignReviewGateSHA256   string   `json:"design_review_gate_sha256,omitempty"`
	RealDispatchDesignSHA256 string   `json:"real_dispatch_design_sha256,omitempty"`
	ProviderPayloadSHA256    string   `json:"provider_payload_sha256,omitempty"`
	Warnings                 []string `json:"warnings,omitempty"`
	Failures                 []string `json:"failures,omitempty"`
}

type ProviderRealDispatchRunbookReportOptions struct {
	RunbookPath                         string
	ExternalApprovalPath                string
	ExternalApprovalRequestPath         string
	DesignReviewGatePath                string
	RealDispatchDesignPath              string
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	KillSwitchPlanPath                  string
}

type providerRealDispatchRunbookChain struct {
	externalApprovalSHA256                string
	externalApprovalRequestSHA256         string
	designReviewGateSHA256                string
	realDispatchDesignSHA256              string
	secretReadProposalSHA256              string
	realTransportImplementationPlanSHA256 string
	activationFinalAuditSHA256            string
	activationCIReportSHA256              string
	killSwitchPlanSHA256                  string
	providerPayloadSHA256                 string
}

func ProviderRealDispatchRunbook(opts ProviderRealDispatchRunbookOptions) (ProviderRealDispatchRunbookResult, error) {
	chain, failures, err := loadProviderRealDispatchRunbookChain(
		opts.ExternalApprovalPath, opts.ExternalApprovalRequestPath, opts.DesignReviewGatePath,
		opts.RealDispatchDesignPath, opts.SecretReadProposalPath, opts.RealTransportImplementationPlanPath,
		opts.ActivationFinalAuditPath, opts.ActivationCIReportPath, opts.KillSwitchPlanPath,
	)
	if err != nil {
		return ProviderRealDispatchRunbookResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealDispatchRunbookResult{}, err
	}

	result := blockedProviderRealDispatchRunbook(chain, failures)
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.RunbookReady = true
	if err := writeProviderRealDispatchRunbookJSON(opts.OutputPath, result); err != nil {
		return ProviderRealDispatchRunbookResult{}, err
	}
	return result, nil
}

func ProviderRealDispatchRunbookReport(opts ProviderRealDispatchRunbookReportOptions) (ProviderRealDispatchRunbookResult, error) {
	stored, _, err := LoadProviderRealDispatchRunbook(opts.RunbookPath)
	if err != nil {
		return ProviderRealDispatchRunbookResult{}, err
	}
	chain, failures, err := loadProviderRealDispatchRunbookChain(
		opts.ExternalApprovalPath, opts.ExternalApprovalRequestPath, opts.DesignReviewGatePath,
		opts.RealDispatchDesignPath, opts.SecretReadProposalPath, opts.RealTransportImplementationPlanPath,
		opts.ActivationFinalAuditPath, opts.ActivationCIReportPath, opts.KillSwitchPlanPath,
	)
	if err != nil {
		return ProviderRealDispatchRunbookResult{}, err
	}

	result := blockedProviderRealDispatchRunbook(chain, failures)
	result.RunbookReady = stored.RunbookReady
	result.FutureSteps = append([]string(nil), stored.FutureSteps...)
	result.OperationalChecklist = append([]string(nil), stored.OperationalChecklist...)
	result.RollbackPlan = append([]string(nil), stored.RollbackPlan...)

	reconcileProviderExecutorHash("runbook report", stored.ExternalApprovalSHA256, chain.externalApprovalSHA256, &failures)
	reconcileProviderExecutorHash("runbook report", stored.DesignReviewGateSHA256, chain.designReviewGateSHA256, &failures)
	reconcileProviderExecutorHash("runbook report", stored.RealDispatchDesignSHA256, chain.realDispatchDesignSHA256, &failures)
	reconcileProviderExecutorHash("runbook report", stored.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)

	if stored.ScriptGenerated || stored.ExecuteCommandGenerated {
		failures = append(failures, "runbook must not generate scripts or execute commands")
	}
	if !stored.RunbookMetadataOnly || !stored.RunbookReady {
		failures = append(failures, "runbook must be metadata-only and ready")
	}

	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = failures
	}
	return result, nil
}

func loadProviderRealDispatchRunbookChain(
	externalApprovalPath, externalApprovalRequestPath, designReviewGatePath, realDispatchDesignPath,
	secretReadProposalPath, realTransportImplementationPlanPath, activationFinalAuditPath,
	activationCIReportPath, killSwitchPlanPath string,
) (providerRealDispatchRunbookChain, []string, error) {
	chain := providerRealDispatchRunbookChain{}
	var failures []string

	approval, approvalData, err := LoadProviderRealDispatchExternalApproval(externalApprovalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.externalApprovalSHA256 = sha256Hex(approvalData)
	if !approval.ExternalApprovalAuthorizedForFuture || approval.ExternalApprovalAllowedNow || approval.RealDispatchAllowedNow {
		failures = append(failures, "external approval must authorize future only")
	}
	if approval.ProviderPayloadSHA256 != "" {
		chain.providerPayloadSHA256 = approval.ProviderPayloadSHA256
	}

	requestData, err := readArtifactBytesNoTextExcerpt("external approval request", externalApprovalRequestPath)
	if err != nil {
		return chain, nil, err
	}
	chain.externalApprovalRequestSHA256 = sha256Hex(requestData)

	gateData, err := readArtifactBytesNoTextExcerpt("design review gate", designReviewGatePath)
	if err != nil {
		return chain, nil, err
	}
	gate, err := ParseProviderRealActivationDesignReviewGateJSON(gateData)
	if err != nil {
		return chain, nil, err
	}
	chain.designReviewGateSHA256 = sha256Hex(gateData)
	if !gate.RealActivationDesignGateReady {
		failures = append(failures, "design review gate must be ready")
	}
	if gate.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 == "" {
		chain.providerPayloadSHA256 = gate.ProviderPayloadSHA256
	}

	dispatchDesign, dispatchData, err := LoadProviderRealDispatchDesign(realDispatchDesignPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realDispatchDesignSHA256 = sha256Hex(dispatchData)
	if !dispatchDesign.RealDispatchDesignReady || dispatchDesign.ExecuteSubcommandRegistered {
		failures = append(failures, "real dispatch design must be ready without execute subcommand")
	}

	proposal, proposalData, err := LoadProviderSecretReadProposal(secretReadProposalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.secretReadProposalSHA256 = sha256Hex(proposalData)
	if !proposal.SecretReadProposalReady || proposal.SecretValuesRead {
		failures = append(failures, "secret read proposal must be ready without secret reads")
	}

	transportPlan, transportData, err := LoadProviderRealTransportImplementationPlan(realTransportImplementationPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realTransportImplementationPlanSHA256 = sha256Hex(transportData)
	if !transportPlan.RealTransportImplementationPlanReady {
		failures = append(failures, "real transport implementation plan must be ready")
	}

	finalAudit, finalAuditData, err := LoadProviderActivationFinalAudit(activationFinalAuditPath)
	if err != nil {
		return chain, nil, err
	}
	chain.activationFinalAuditSHA256 = sha256Hex(finalAuditData)
	if !finalAudit.FinalAuditReady {
		failures = append(failures, "activation final audit must be ready")
	}

	ciReport, ciReportData, err := LoadProviderActivationCIReport(activationCIReportPath)
	if err != nil {
		return chain, nil, err
	}
	chain.activationCIReportSHA256 = sha256Hex(ciReportData)
	if !ciReport.CIObservabilityReady {
		failures = append(failures, "activation ci report must be observability ready")
	}

	killSwitchPlan, killSwitchData, err := LoadProviderActivationKillSwitchPlan(killSwitchPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.killSwitchPlanSHA256 = sha256Hex(killSwitchData)
	if !killSwitchPlan.KillSwitchPlanReady || !killSwitchPlan.GlobalDisabled {
		failures = append(failures, "kill switch plan must be ready with global_disabled true")
	}

	return chain, failures, nil
}

func blockedProviderRealDispatchRunbook(chain providerRealDispatchRunbookChain, failures []string) ProviderRealDispatchRunbookResult {
	result := ProviderRealDispatchRunbookResult{
		Status:                   lancedbpolicy.StatusOK,
		RunbookMetadataOnly:      true,
		ScriptGenerated:          false,
		ExecuteCommandGenerated:  false,
		ProviderCall:             false,
		NetworkCall:              false,
		SecretValuesRead:         false,
		TransportCalled:          false,
		WorkspaceModified:        false,
		BlockedReason:            ProviderCallExecutorBlockedReason,
		FutureSteps:              append([]string(nil), providerRealDispatchRunbookFutureSteps...),
		OperationalChecklist:     append([]string(nil), providerRealDispatchRunbookOperationalChecklist...),
		RollbackPlan:             append([]string(nil), providerRealDispatchRunbookRollbackPlan...),
		ExternalApprovalSHA256:   chain.externalApprovalSHA256,
		DesignReviewGateSHA256:   chain.designReviewGateSHA256,
		RealDispatchDesignSHA256: chain.realDispatchDesignSHA256,
		ProviderPayloadSHA256:    chain.providerPayloadSHA256,
		Failures:                 failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderRealDispatchRunbook(path string) (ProviderRealDispatchRunbookResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("real dispatch runbook", path)
	if err != nil {
		return ProviderRealDispatchRunbookResult{}, nil, err
	}
	var result ProviderRealDispatchRunbookResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderRealDispatchRunbookResult{}, nil, fmt.Errorf("parse real dispatch runbook json: %w", err)
	}
	return result, data, nil
}

func writeProviderRealDispatchRunbookJSON(path string, result ProviderRealDispatchRunbookResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	body := string(data)
	if strings.Contains(body, "text_excerpt") || strings.Contains(body, "alpha text") {
		return fmt.Errorf("real dispatch runbook must not contain materialized preview text")
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func WriteProviderRealDispatchRunbookJSON(result ProviderRealDispatchRunbookResult, out io.Writer) error {
	return json.NewEncoder(out).Encode(result)
}

func WriteProviderRealDispatchRunbookText(result ProviderRealDispatchRunbookResult, out io.Writer) error {
	_, err := fmt.Fprintf(out, "status: %s\nrunbook_ready: %t\nrunbook_metadata_only: %t\nscript_generated: %t\nexecute_command_generated: %t\nblocked_reason: %s\n",
		result.Status, result.RunbookReady, result.RunbookMetadataOnly, result.ScriptGenerated, result.ExecuteCommandGenerated, result.BlockedReason)
	return err
}
