package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const providerRealDispatchRiskStatusBlocked = "blocked"

type ProviderRealDispatchRiskEntry struct {
	ID            string `json:"id"`
	Severity      string `json:"severity"`
	Likelihood    string `json:"likelihood"`
	Mitigation    string `json:"mitigation"`
	RequiredGuard string `json:"required_guard"`
	CurrentStatus string `json:"current_status"`
}

var providerRealDispatchDefaultRisks = []ProviderRealDispatchRiskEntry{
	{ID: "secret_leak", Severity: "critical", Likelihood: "low", Mitigation: "names-only credential policy; no secret reads in metadata chain", RequiredGuard: "secret_values_read_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "unintended_network_call", Severity: "critical", Likelihood: "low", Mitigation: "blocked transport and network flags on all artifacts", RequiredGuard: "network_call_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "provider_payload_sent_accidentally", Severity: "critical", Likelihood: "low", Mitigation: "provider call gate and external approval required before any future dispatch", RequiredGuard: "sent_to_provider_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "transport_deliver_invoked", Severity: "critical", Likelihood: "low", Mitigation: "BlockedProviderTransport only; transport_called must stay false", RequiredGuard: "transport_called_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "workspace_modified", Severity: "high", Likelihood: "low", Mitigation: "no diff application in design or approval chain", RequiredGuard: "workspace_modified_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "diff_applied", Severity: "high", Likelihood: "low", Mitigation: "response change proposals remain fixture-only", RequiredGuard: "diff_applied_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "commit_or_pr_created", Severity: "high", Likelihood: "low", Mitigation: "no automated commit/PR in provider dispatch chain", RequiredGuard: "commit_created_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "prompt_injection_real_runner_changed", Severity: "high", Likelihood: "low", Mitigation: "materialized injection remains blocked in activation chain", RequiredGuard: "prompt_injection_real_runner_false", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "kill_switch_disabled", Severity: "critical", Likelihood: "low", Mitigation: "kill switch plan must keep global_disabled true", RequiredGuard: "kill_switch_active", CurrentStatus: providerRealDispatchRiskStatusBlocked},
	{ID: "external_approval_mismatch", Severity: "high", Likelihood: "low", Mitigation: "external approval confirm hashes and inspect reconciliation", RequiredGuard: "external_approval_hash_coherence", CurrentStatus: providerRealDispatchRiskStatusBlocked},
}

type ProviderRealDispatchRiskRegisterOptions struct {
	RunbookPath                         string
	ExternalApprovalPath                string
	DesignReviewGatePath                string
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	RealDispatchDesignPath              string
	ActivationFinalAuditPath            string
	KillSwitchPlanPath                  string
	OutputPath                          string
}

type ProviderRealDispatchRiskRegisterResult struct {
	Status                 string                          `json:"status"`
	RiskRegisterReady      bool                            `json:"risk_register_ready"`
	RiskReviewRequired     bool                            `json:"risk_review_required"`
	RisksBlocked           bool                            `json:"risks_blocked"`
	ProviderCall           bool                            `json:"provider_call"`
	NetworkCall            bool                            `json:"network_call"`
	SecretValuesRead       bool                            `json:"secret_values_read"`
	TransportCalled        bool                            `json:"transport_called"`
	WorkspaceModified      bool                            `json:"workspace_modified"`
	BlockedReason          string                          `json:"blocked_reason"`
	Risks                  []ProviderRealDispatchRiskEntry `json:"risks,omitempty"`
	RunbookSHA256          string                          `json:"runbook_sha256,omitempty"`
	ExternalApprovalSHA256 string                          `json:"external_approval_sha256,omitempty"`
	DesignReviewGateSHA256 string                          `json:"design_review_gate_sha256,omitempty"`
	ProviderPayloadSHA256  string                          `json:"provider_payload_sha256,omitempty"`
	Warnings               []string                        `json:"warnings,omitempty"`
	Failures               []string                        `json:"failures,omitempty"`
}

type ProviderRealDispatchRiskReportOptions struct {
	RiskRegisterPath                    string
	RunbookPath                         string
	ExternalApprovalPath                string
	DesignReviewGatePath                string
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	RealDispatchDesignPath              string
	ActivationFinalAuditPath            string
	KillSwitchPlanPath                  string
}

func ProviderRealDispatchRiskRegister(opts ProviderRealDispatchRiskRegisterOptions) (ProviderRealDispatchRiskRegisterResult, error) {
	chain, failures, err := loadProviderRealDispatchRiskChain(
		opts.RunbookPath, opts.ExternalApprovalPath, opts.DesignReviewGatePath,
		opts.SecretReadProposalPath, opts.RealTransportImplementationPlanPath, opts.RealDispatchDesignPath,
		opts.ActivationFinalAuditPath, opts.KillSwitchPlanPath,
	)
	if err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, err
	}

	result := blockedProviderRealDispatchRiskRegister(chain, failures)
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.RiskRegisterReady = true
	if err := writeProviderRealDispatchRiskRegisterJSON(opts.OutputPath, result); err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, err
	}
	return result, nil
}

func ProviderRealDispatchRiskReport(opts ProviderRealDispatchRiskReportOptions) (ProviderRealDispatchRiskRegisterResult, error) {
	stored, _, err := LoadProviderRealDispatchRiskRegister(opts.RiskRegisterPath)
	if err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, err
	}
	chain, failures, err := loadProviderRealDispatchRiskChain(
		opts.RunbookPath, opts.ExternalApprovalPath, opts.DesignReviewGatePath,
		opts.SecretReadProposalPath, opts.RealTransportImplementationPlanPath, opts.RealDispatchDesignPath,
		opts.ActivationFinalAuditPath, opts.KillSwitchPlanPath,
	)
	if err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, err
	}

	result := blockedProviderRealDispatchRiskRegister(chain, failures)
	result.RiskRegisterReady = stored.RiskRegisterReady
	result.Risks = append([]ProviderRealDispatchRiskEntry(nil), stored.Risks...)

	reconcileProviderExecutorHash("risk report", stored.RunbookSHA256, chain.runbookSHA256, &failures)
	reconcileProviderExecutorHash("risk report", stored.ExternalApprovalSHA256, chain.externalApprovalSHA256, &failures)
	reconcileProviderExecutorHash("risk report", stored.DesignReviewGateSHA256, chain.designReviewGateSHA256, &failures)
	reconcileProviderExecutorHash("risk report", stored.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)

	for _, risk := range stored.Risks {
		if risk.CurrentStatus == "active_execution" {
			failures = append(failures, fmt.Sprintf("risk %q must not be active_execution", risk.ID))
		}
	}
	if !stored.RiskReviewRequired || !stored.RisksBlocked {
		failures = append(failures, "risk register must require review with risks blocked")
	}

	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = failures
	}
	return result, nil
}

type providerRealDispatchRiskChain struct {
	runbookSHA256                         string
	externalApprovalSHA256                string
	designReviewGateSHA256                string
	secretReadProposalSHA256              string
	realTransportImplementationPlanSHA256 string
	realDispatchDesignSHA256              string
	activationFinalAuditSHA256            string
	killSwitchPlanSHA256                  string
	providerPayloadSHA256                 string
	killSwitchActive                      bool
}

func loadProviderRealDispatchRiskChain(
	runbookPath, externalApprovalPath, designReviewGatePath, secretReadProposalPath,
	realTransportImplementationPlanPath, realDispatchDesignPath, activationFinalAuditPath, killSwitchPlanPath string,
) (providerRealDispatchRiskChain, []string, error) {
	chain := providerRealDispatchRiskChain{killSwitchActive: true}
	var failures []string

	runbook, runbookData, err := LoadProviderRealDispatchRunbook(runbookPath)
	if err != nil {
		return chain, nil, err
	}
	chain.runbookSHA256 = sha256Hex(runbookData)
	if !runbook.RunbookReady || !runbook.RunbookMetadataOnly || runbook.ScriptGenerated || runbook.ExecuteCommandGenerated {
		failures = append(failures, "runbook must be metadata-only and ready")
	}
	if runbook.ProviderPayloadSHA256 != "" {
		chain.providerPayloadSHA256 = runbook.ProviderPayloadSHA256
	}

	approval, approvalData, err := LoadProviderRealDispatchExternalApproval(externalApprovalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.externalApprovalSHA256 = sha256Hex(approvalData)
	if !approval.ExternalApprovalAuthorizedForFuture || approval.ExternalApprovalAllowedNow {
		failures = append(failures, "external approval must authorize future only")
	}

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

	proposal, proposalData, err := LoadProviderSecretReadProposal(secretReadProposalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.secretReadProposalSHA256 = sha256Hex(proposalData)
	if proposal.SecretValuesRead {
		failures = append(failures, "secret read proposal must not read secrets")
	}

	transportPlan, transportData, err := LoadProviderRealTransportImplementationPlan(realTransportImplementationPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realTransportImplementationPlanSHA256 = sha256Hex(transportData)
	if transportPlan.TransportCalled {
		failures = append(failures, "transport plan must keep transport_called false")
	}

	dispatchDesign, dispatchData, err := LoadProviderRealDispatchDesign(realDispatchDesignPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realDispatchDesignSHA256 = sha256Hex(dispatchData)
	if dispatchDesign.ExecuteSubcommandRegistered {
		failures = append(failures, "real dispatch design must not register execute subcommand")
	}

	finalAudit, finalAuditData, err := LoadProviderActivationFinalAudit(activationFinalAuditPath)
	if err != nil {
		return chain, nil, err
	}
	chain.activationFinalAuditSHA256 = sha256Hex(finalAuditData)
	if !finalAudit.FinalAuditReady {
		failures = append(failures, "activation final audit must be ready")
	}

	killSwitchPlan, killSwitchData, err := LoadProviderActivationKillSwitchPlan(killSwitchPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.killSwitchPlanSHA256 = sha256Hex(killSwitchData)
	if !killSwitchPlan.GlobalDisabled {
		chain.killSwitchActive = false
		failures = append(failures, "kill switch plan global_disabled must be true")
	}

	return chain, failures, nil
}

func blockedProviderRealDispatchRiskRegister(chain providerRealDispatchRiskChain, failures []string) ProviderRealDispatchRiskRegisterResult {
	risks := append([]ProviderRealDispatchRiskEntry(nil), providerRealDispatchDefaultRisks...)
	result := ProviderRealDispatchRiskRegisterResult{
		Status:                 lancedbpolicy.StatusOK,
		RiskReviewRequired:     true,
		RisksBlocked:           true,
		ProviderCall:           false,
		NetworkCall:            false,
		SecretValuesRead:       false,
		TransportCalled:        false,
		WorkspaceModified:      false,
		BlockedReason:          ProviderCallExecutorBlockedReason,
		Risks:                  risks,
		RunbookSHA256:          chain.runbookSHA256,
		ExternalApprovalSHA256: chain.externalApprovalSHA256,
		DesignReviewGateSHA256: chain.designReviewGateSHA256,
		ProviderPayloadSHA256:  chain.providerPayloadSHA256,
		Failures:               failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderRealDispatchRiskRegister(path string) (ProviderRealDispatchRiskRegisterResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("real dispatch risk register", path)
	if err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, nil, err
	}
	var result ProviderRealDispatchRiskRegisterResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderRealDispatchRiskRegisterResult{}, nil, fmt.Errorf("parse real dispatch risk register json: %w", err)
	}
	return result, data, nil
}

func writeProviderRealDispatchRiskRegisterJSON(path string, result ProviderRealDispatchRiskRegisterResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	body := string(data)
	if strings.Contains(body, "text_excerpt") || strings.Contains(body, "alpha text") {
		return fmt.Errorf("real dispatch risk register must not contain materialized preview text")
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func WriteProviderRealDispatchRiskRegisterJSON(result ProviderRealDispatchRiskRegisterResult, out io.Writer) error {
	return json.NewEncoder(out).Encode(result)
}

func WriteProviderRealDispatchRiskReportJSON(result ProviderRealDispatchRiskRegisterResult, out io.Writer) error {
	return json.NewEncoder(out).Encode(result)
}

func WriteProviderRealDispatchRiskRegisterText(result ProviderRealDispatchRiskRegisterResult, out io.Writer) error {
	_, err := fmt.Fprintf(out, "status: %s\nrisk_register_ready: %t\nrisk_review_required: %t\nrisks_blocked: %t\nblocked_reason: %s\n",
		result.Status, result.RiskRegisterReady, result.RiskReviewRequired, result.RisksBlocked, result.BlockedReason)
	return err
}
