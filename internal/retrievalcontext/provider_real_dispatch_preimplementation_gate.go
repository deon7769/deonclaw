package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderRealDispatchPreimplementationGateOptions struct {
	ExternalApprovalPath                string
	RunbookPath                         string
	RiskRegisterPath                    string
	DesignReviewGatePath                string
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	RealDispatchDesignPath              string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	KillSwitchPlanPath                  string
	OperatorReviewBundlePath            string
}

type ProviderRealDispatchPreimplementationGateResult struct {
	Status                      string   `json:"status"`
	PreimplementationGateReady  bool     `json:"preimplementation_gate_ready"`
	RealDispatchSupportedNow    bool     `json:"real_dispatch_supported_now"`
	RealDispatchAllowedNow      bool     `json:"real_dispatch_allowed_now"`
	ExecuteSubcommandRegistered bool     `json:"execute_subcommand_registered"`
	SecretValuesRead            bool     `json:"secret_values_read"`
	ProviderCall                bool     `json:"provider_call"`
	NetworkCall                 bool     `json:"network_call"`
	TransportCalled             bool     `json:"transport_called"`
	SentToProvider              bool     `json:"sent_to_provider"`
	ReceivedFromProvider        bool     `json:"received_from_provider"`
	WorkspaceModified           bool     `json:"workspace_modified"`
	DiffApplied                 bool     `json:"diff_applied"`
	CommitCreated               bool     `json:"commit_created"`
	PRCreated                   bool     `json:"pr_created"`
	WorkerExecution             bool     `json:"worker_execution"`
	PromptInjectionRealRunner   bool     `json:"prompt_injection_real_runner"`
	ActivationAllowedNow        bool     `json:"activation_allowed_now"`
	BlockedReason               string   `json:"blocked_reason"`
	KillSwitchActive            bool     `json:"kill_switch_active"`
	OperatorReviewRequired      bool     `json:"operator_review_required"`
	ExternalApprovalSHA256      string   `json:"external_approval_sha256,omitempty"`
	RunbookSHA256               string   `json:"runbook_sha256,omitempty"`
	RiskRegisterSHA256          string   `json:"risk_register_sha256,omitempty"`
	DesignReviewGateSHA256      string   `json:"design_review_gate_sha256,omitempty"`
	ProviderPayloadSHA256       string   `json:"provider_payload_sha256,omitempty"`
	Warnings                    []string `json:"warnings,omitempty"`
	Failures                    []string `json:"failures,omitempty"`
}

type providerRealDispatchPreimplementationChain struct {
	externalApprovalSHA256                string
	runbookSHA256                         string
	riskRegisterSHA256                    string
	designReviewGateSHA256                string
	secretReadProposalSHA256              string
	realTransportImplementationPlanSHA256 string
	realDispatchDesignSHA256              string
	activationFinalAuditSHA256            string
	activationCIReportSHA256              string
	killSwitchPlanSHA256                  string
	operatorReviewBundleSHA256            string
	providerPayloadSHA256                 string
	killSwitchActive                      bool
	operatorReviewRequired                bool
}

func ProviderRealDispatchPreimplementationGate(opts ProviderRealDispatchPreimplementationGateOptions) (ProviderRealDispatchPreimplementationGateResult, error) {
	chain, failures, err := loadProviderRealDispatchPreimplementationChain(opts)
	if err != nil {
		return ProviderRealDispatchPreimplementationGateResult{}, err
	}
	result := blockedProviderRealDispatchPreimplementationGate(chain, failures)
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.PreimplementationGateReady = true
	return result, nil
}

func ProviderRealDispatchPreimplementationReport(opts ProviderRealDispatchPreimplementationGateOptions) (ProviderRealDispatchPreimplementationGateResult, error) {
	return ProviderRealDispatchPreimplementationGate(opts)
}

func loadProviderRealDispatchPreimplementationChain(opts ProviderRealDispatchPreimplementationGateOptions) (providerRealDispatchPreimplementationChain, []string, error) {
	chain := providerRealDispatchPreimplementationChain{killSwitchActive: true, operatorReviewRequired: true}
	var failures []string

	for _, check := range []struct {
		field string
		path  string
	}{
		{"external approval path", opts.ExternalApprovalPath},
		{"runbook path", opts.RunbookPath},
		{"risk register path", opts.RiskRegisterPath},
		{"design review gate path", opts.DesignReviewGatePath},
		{"secret read proposal path", opts.SecretReadProposalPath},
		{"real transport implementation plan path", opts.RealTransportImplementationPlanPath},
		{"real dispatch design path", opts.RealDispatchDesignPath},
		{"activation final audit path", opts.ActivationFinalAuditPath},
		{"activation ci report path", opts.ActivationCIReportPath},
		{"kill switch plan path", opts.KillSwitchPlanPath},
		{"operator review bundle path", opts.OperatorReviewBundlePath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return chain, nil, err
		}
	}

	approval, approvalData, err := LoadProviderRealDispatchExternalApproval(opts.ExternalApprovalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.externalApprovalSHA256 = sha256Hex(approvalData)
	if !approval.ExternalApprovalAuthorizedForFuture {
		failures = append(failures, "external approval must be authorized for future")
	}
	if approval.ExternalApprovalAllowedNow || approval.RealDispatchAllowedNow {
		failures = append(failures, "external approval must not be allowed now")
	}
	if approval.ExecuteSubcommandRegistered || approval.ProviderCall || approval.NetworkCall || approval.SecretValuesRead || approval.TransportCalled || approval.SentToProvider || approval.WorkspaceModified {
		failures = append(failures, "external approval execution flags must stay blocked")
	}
	if approval.ProviderPayloadSHA256 != "" {
		chain.providerPayloadSHA256 = approval.ProviderPayloadSHA256
	}

	runbook, runbookData, err := LoadProviderRealDispatchRunbook(opts.RunbookPath)
	if err != nil {
		return chain, nil, err
	}
	chain.runbookSHA256 = sha256Hex(runbookData)
	if !runbook.RunbookReady || !runbook.RunbookMetadataOnly || runbook.ScriptGenerated || runbook.ExecuteCommandGenerated {
		failures = append(failures, "runbook must be metadata-only and ready")
	}
	reconcileProviderExecutorHash("preimplementation gate", runbook.ExternalApprovalSHA256, chain.externalApprovalSHA256, &failures)

	riskRegister, riskData, err := LoadProviderRealDispatchRiskRegister(opts.RiskRegisterPath)
	if err != nil {
		return chain, nil, err
	}
	chain.riskRegisterSHA256 = sha256Hex(riskData)
	if !riskRegister.RiskRegisterReady || !riskRegister.RiskReviewRequired || !riskRegister.RisksBlocked {
		failures = append(failures, "risk register must be ready with risks blocked")
	}
	for _, risk := range riskRegister.Risks {
		if risk.CurrentStatus == "active_execution" {
			failures = append(failures, fmt.Sprintf("risk %q must not be active_execution", risk.ID))
		}
	}
	reconcileProviderExecutorHash("preimplementation gate", riskRegister.ExternalApprovalSHA256, chain.externalApprovalSHA256, &failures)

	gateData, err := readArtifactBytesNoTextExcerpt("design review gate", opts.DesignReviewGatePath)
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
	if gate.RealDispatchSupportedNow {
		failures = append(failures, "design review gate must keep real_dispatch_supported_now false")
	}
	if gate.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 == "" {
		chain.providerPayloadSHA256 = gate.ProviderPayloadSHA256
	}

	proposal, proposalData, err := LoadProviderSecretReadProposal(opts.SecretReadProposalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.secretReadProposalSHA256 = sha256Hex(proposalData)
	if proposal.SecretValuesRead {
		failures = append(failures, "secret_values_read must be false")
	}

	transportPlan, transportData, err := LoadProviderRealTransportImplementationPlan(opts.RealTransportImplementationPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realTransportImplementationPlanSHA256 = sha256Hex(transportData)
	if transportPlan.TransportCalled || transportPlan.ProviderCall || transportPlan.NetworkCall {
		failures = append(failures, "transport plan execution flags must stay blocked")
	}

	dispatchDesign, dispatchData, err := LoadProviderRealDispatchDesign(opts.RealDispatchDesignPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realDispatchDesignSHA256 = sha256Hex(dispatchData)
	if dispatchDesign.ExecuteSubcommandRegistered || dispatchDesign.RealDispatchCommandAvailable {
		failures = append(failures, "execute subcommand must not be registered")
	}

	finalAudit, finalAuditData, err := LoadProviderActivationFinalAudit(opts.ActivationFinalAuditPath)
	if err != nil {
		return chain, nil, err
	}
	chain.activationFinalAuditSHA256 = sha256Hex(finalAuditData)
	if !finalAudit.FinalAuditReady {
		failures = append(failures, "activation final audit must be ready")
	}
	if !finalAudit.KillSwitchActive {
		chain.killSwitchActive = false
		failures = append(failures, "kill_switch_active must be true")
	}
	if !finalAudit.OperatorReviewRequired {
		chain.operatorReviewRequired = false
		failures = append(failures, "operator_review_required must be true")
	}

	ciReport, ciReportData, err := LoadProviderActivationCIReport(opts.ActivationCIReportPath)
	if err != nil {
		return chain, nil, err
	}
	chain.activationCIReportSHA256 = sha256Hex(ciReportData)
	if !ciReport.CIObservabilityReady {
		failures = append(failures, "activation ci report must be observability ready")
	}

	killSwitchPlan, killSwitchData, err := LoadProviderActivationKillSwitchPlan(opts.KillSwitchPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.killSwitchPlanSHA256 = sha256Hex(killSwitchData)
	if !killSwitchPlan.KillSwitchPlanReady || !killSwitchPlan.GlobalDisabled {
		chain.killSwitchActive = false
		failures = append(failures, "kill switch plan must be ready with global_disabled true")
	}

	operatorBundle, operatorData, err := LoadProviderActivationOperatorReviewBundle(opts.OperatorReviewBundlePath)
	if err != nil {
		return chain, nil, err
	}
	chain.operatorReviewBundleSHA256 = sha256Hex(operatorData)
	if !operatorBundle.OperatorReviewBundleReady || !operatorBundle.OperatorReviewRequired {
		chain.operatorReviewRequired = false
		failures = append(failures, "operator review bundle must be ready and required")
	}

	return chain, failures, nil
}

func blockedProviderRealDispatchPreimplementationGate(chain providerRealDispatchPreimplementationChain, failures []string) ProviderRealDispatchPreimplementationGateResult {
	result := ProviderRealDispatchPreimplementationGateResult{
		Status:                      lancedbpolicy.StatusOK,
		RealDispatchSupportedNow:    false,
		RealDispatchAllowedNow:      false,
		ExecuteSubcommandRegistered: false,
		SecretValuesRead:            false,
		ProviderCall:                false,
		NetworkCall:                 false,
		TransportCalled:             false,
		SentToProvider:              false,
		ReceivedFromProvider:        false,
		WorkspaceModified:           false,
		DiffApplied:                 false,
		CommitCreated:               false,
		PRCreated:                   false,
		WorkerExecution:             false,
		PromptInjectionRealRunner:   false,
		ActivationAllowedNow:        false,
		BlockedReason:               ProviderCallExecutorBlockedReason,
		KillSwitchActive:            chain.killSwitchActive,
		OperatorReviewRequired:      chain.operatorReviewRequired,
		ExternalApprovalSHA256:      chain.externalApprovalSHA256,
		RunbookSHA256:               chain.runbookSHA256,
		RiskRegisterSHA256:          chain.riskRegisterSHA256,
		DesignReviewGateSHA256:      chain.designReviewGateSHA256,
		ProviderPayloadSHA256:       chain.providerPayloadSHA256,
		Failures:                    failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func WriteProviderRealDispatchPreimplementationGateJSON(result ProviderRealDispatchPreimplementationGateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	body := string(data)
	if strings.Contains(body, "text_excerpt") || strings.Contains(body, "alpha text") {
		return fmt.Errorf("preimplementation gate output must not contain materialized preview text")
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderRealDispatchPreimplementationGateText(result ProviderRealDispatchPreimplementationGateResult, out io.Writer) error {
	_, err := fmt.Fprintf(out, "status: %s\npreimplementation_gate_ready: %t\nreal_dispatch_supported_now: %t\nreal_dispatch_allowed_now: %t\nexecute_subcommand_registered: %t\nkill_switch_active: %t\noperator_review_required: %t\nblocked_reason: %s\n",
		result.Status, result.PreimplementationGateReady, result.RealDispatchSupportedNow, result.RealDispatchAllowedNow, result.ExecuteSubcommandRegistered, result.KillSwitchActive, result.OperatorReviewRequired, result.BlockedReason)
	return err
}
