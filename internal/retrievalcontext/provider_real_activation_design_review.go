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

type ProviderRealActivationDesignReviewPackageOptions struct {
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	RealDispatchDesignPath              string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	KillSwitchPlanPath                  string
	OperatorReviewBundlePath            string
	OutputPath                          string
}

type ProviderRealActivationDesignReviewPackageResult struct {
	Status                                string   `json:"status"`
	RealActivationDesignReviewReady       bool     `json:"real_activation_design_review_ready"`
	RealDispatchSupportedNow              bool     `json:"real_dispatch_supported_now"`
	ActivationAllowedNow                  bool     `json:"activation_allowed_now"`
	SecretValuesRead                      bool     `json:"secret_values_read"`
	ProviderCall                          bool     `json:"provider_call"`
	NetworkCall                           bool     `json:"network_call"`
	TransportCalled                       bool     `json:"transport_called"`
	SentToProvider                        bool     `json:"sent_to_provider"`
	ReceivedFromProvider                  bool     `json:"received_from_provider"`
	WorkspaceModified                     bool     `json:"workspace_modified"`
	DiffApplied                           bool     `json:"diff_applied"`
	CommitCreated                         bool     `json:"commit_created"`
	PRCreated                             bool     `json:"pr_created"`
	WorkerExecution                       bool     `json:"worker_execution"`
	PromptInjectionRealRunner             bool     `json:"prompt_injection_real_runner"`
	BlockedReason                         string   `json:"blocked_reason"`
	KillSwitchActive                      bool     `json:"kill_switch_active"`
	OperatorReviewRequired                bool     `json:"operator_review_required"`
	SecretReadProposalSHA256              string   `json:"secret_read_proposal_sha256,omitempty"`
	RealTransportImplementationPlanSHA256 string   `json:"real_transport_implementation_plan_sha256,omitempty"`
	RealDispatchDesignSHA256              string   `json:"real_dispatch_design_sha256,omitempty"`
	ActivationFinalAuditSHA256            string   `json:"activation_final_audit_sha256,omitempty"`
	ActivationCIReportSHA256              string   `json:"activation_ci_report_sha256,omitempty"`
	KillSwitchPlanSHA256                  string   `json:"kill_switch_plan_sha256,omitempty"`
	OperatorReviewBundleSHA256            string   `json:"operator_review_bundle_sha256,omitempty"`
	ProviderPayloadSHA256                 string   `json:"provider_payload_sha256,omitempty"`
	ReviewChecklist                       []string `json:"review_checklist,omitempty"`
	Warnings                              []string `json:"warnings,omitempty"`
	Failures                              []string `json:"failures,omitempty"`
}

type ProviderRealActivationDesignReviewGateOptions struct {
	DesignReviewPackagePath             string
	SecretReadProposalPath              string
	RealTransportImplementationPlanPath string
	RealDispatchDesignPath              string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	KillSwitchPlanPath                  string
	OperatorReviewBundlePath            string
}

type ProviderRealActivationDesignReviewGateResult struct {
	Status                                string   `json:"status"`
	RealActivationDesignReviewReady       bool     `json:"real_activation_design_review_ready"`
	RealActivationDesignGateReady         bool     `json:"real_activation_design_gate_ready"`
	RealDispatchSupportedNow              bool     `json:"real_dispatch_supported_now"`
	ActivationAllowedNow                  bool     `json:"activation_allowed_now"`
	SecretValuesRead                      bool     `json:"secret_values_read"`
	ProviderCall                          bool     `json:"provider_call"`
	NetworkCall                           bool     `json:"network_call"`
	TransportCalled                       bool     `json:"transport_called"`
	SentToProvider                        bool     `json:"sent_to_provider"`
	ReceivedFromProvider                  bool     `json:"received_from_provider"`
	WorkspaceModified                     bool     `json:"workspace_modified"`
	DiffApplied                           bool     `json:"diff_applied"`
	CommitCreated                         bool     `json:"commit_created"`
	PRCreated                             bool     `json:"pr_created"`
	WorkerExecution                       bool     `json:"worker_execution"`
	PromptInjectionRealRunner             bool     `json:"prompt_injection_real_runner"`
	BlockedReason                         string   `json:"blocked_reason"`
	KillSwitchActive                      bool     `json:"kill_switch_active"`
	OperatorReviewRequired                bool     `json:"operator_review_required"`
	SecretReadProposalSHA256              string   `json:"secret_read_proposal_sha256,omitempty"`
	RealTransportImplementationPlanSHA256 string   `json:"real_transport_implementation_plan_sha256,omitempty"`
	RealDispatchDesignSHA256              string   `json:"real_dispatch_design_sha256,omitempty"`
	ActivationFinalAuditSHA256            string   `json:"activation_final_audit_sha256,omitempty"`
	ActivationCIReportSHA256              string   `json:"activation_ci_report_sha256,omitempty"`
	ProviderPayloadSHA256                 string   `json:"provider_payload_sha256,omitempty"`
	Warnings                              []string `json:"warnings,omitempty"`
	Failures                              []string `json:"failures,omitempty"`
}

var providerRealActivationDesignReviewChecklist = []string{
	"secret_read_proposal_ready",
	"real_transport_implementation_plan_ready",
	"real_dispatch_design_ready",
	"no_secret_values_read",
	"no_provider_call",
	"no_network_call",
	"no_transport_called",
	"kill_switch_active",
	"operator_review_required",
	"execute_subcommand_not_registered",
	"activation_not_allowed_now",
}

type providerRealActivationDesignReviewChain struct {
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

func ProviderRealActivationDesignReviewPackage(opts ProviderRealActivationDesignReviewPackageOptions) (ProviderRealActivationDesignReviewPackageResult, error) {
	chain, failures, err := loadProviderRealActivationDesignReviewChain(
		opts.SecretReadProposalPath,
		opts.RealTransportImplementationPlanPath,
		opts.RealDispatchDesignPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.KillSwitchPlanPath,
		opts.OperatorReviewBundlePath,
	)
	if err != nil {
		return ProviderRealActivationDesignReviewPackageResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealActivationDesignReviewPackageResult{}, err
	}

	result := blockedProviderRealActivationDesignReviewPackage(chain, failures)
	if len(failures) == 0 {
		result.RealActivationDesignReviewReady = true
	}
	if err := writeProviderRealActivationDesignReviewPackageJSON(opts.OutputPath, result); err != nil {
		return ProviderRealActivationDesignReviewPackageResult{}, err
	}
	return result, nil
}

func ProviderRealActivationDesignReviewGate(opts ProviderRealActivationDesignReviewGateOptions) (ProviderRealActivationDesignReviewGateResult, error) {
	pkg, pkgData, err := LoadProviderRealActivationDesignReviewPackage(opts.DesignReviewPackagePath)
	if err != nil {
		return ProviderRealActivationDesignReviewGateResult{}, err
	}
	chain, failures, err := loadProviderRealActivationDesignReviewChain(
		opts.SecretReadProposalPath,
		opts.RealTransportImplementationPlanPath,
		opts.RealDispatchDesignPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.KillSwitchPlanPath,
		opts.OperatorReviewBundlePath,
	)
	if err != nil {
		return ProviderRealActivationDesignReviewGateResult{}, err
	}
	if !pkg.RealActivationDesignReviewReady {
		failures = append(failures, "design review package real_activation_design_review_ready must be true")
	}
	if pkg.RealDispatchSupportedNow || pkg.ActivationAllowedNow || pkg.SecretValuesRead || pkg.ProviderCall || pkg.NetworkCall || pkg.TransportCalled || pkg.SentToProvider || pkg.ReceivedFromProvider || pkg.WorkspaceModified || pkg.DiffApplied || pkg.CommitCreated || pkg.PRCreated || pkg.WorkerExecution || pkg.PromptInjectionRealRunner {
		failures = append(failures, "design review package must keep execution flags blocked")
	}
	if !pkg.KillSwitchActive || !pkg.OperatorReviewRequired {
		failures = append(failures, "design review package must require kill switch and operator review")
	}
	reconcileProviderExecutorHash("design review gate", pkg.SecretReadProposalSHA256, chain.secretReadProposalSHA256, &failures)
	reconcileProviderExecutorHash("design review gate", pkg.RealTransportImplementationPlanSHA256, chain.realTransportImplementationPlanSHA256, &failures)
	reconcileProviderExecutorHash("design review gate", pkg.RealDispatchDesignSHA256, chain.realDispatchDesignSHA256, &failures)
	reconcileProviderExecutorHash("design review gate", pkg.ActivationFinalAuditSHA256, chain.activationFinalAuditSHA256, &failures)
	reconcileProviderExecutorHash("design review gate", pkg.ActivationCIReportSHA256, chain.activationCIReportSHA256, &failures)
	reconcileProviderExecutorHash("design review gate", pkg.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
	if sha256Hex(pkgData) == "" {
		failures = append(failures, "design review package hash missing")
	}

	result := blockedProviderRealActivationDesignReviewGate(chain, failures)
	if len(failures) == 0 {
		result.RealActivationDesignReviewReady = true
		result.RealActivationDesignGateReady = true
	}
	return result, nil
}

func loadProviderRealActivationDesignReviewChain(secretReadProposalPath, realTransportPlanPath, realDispatchDesignPath, activationFinalAuditPath, activationCIReportPath, killSwitchPlanPath, operatorReviewBundlePath string) (providerRealActivationDesignReviewChain, []string, error) {
	var chain providerRealActivationDesignReviewChain
	failures := []string{}

	proposal, proposalData, err := LoadProviderSecretReadProposal(secretReadProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.secretReadProposalSHA256 = sha256Hex(proposalData)
		chain.providerPayloadSHA256 = proposal.ProviderPayloadSHA256
		if !proposal.SecretReadProposalReady {
			failures = append(failures, "secret read proposal secret_read_proposal_ready must be true")
		}
		if proposal.SecretReadAllowedNow || proposal.SecretValuesRead {
			failures = append(failures, "secret read proposal must keep secret reads blocked")
		}
	}

	transportPlan, transportData, err := LoadProviderRealTransportImplementationPlan(realTransportPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.realTransportImplementationPlanSHA256 = sha256Hex(transportData)
		if !transportPlan.RealTransportImplementationPlanReady {
			failures = append(failures, "real transport implementation plan real_transport_implementation_plan_ready must be true")
		}
		if transportPlan.RealTransportAvailableNow || transportPlan.TransportEnabled || transportPlan.TransportCalled {
			failures = append(failures, "real transport implementation plan must keep transport blocked")
		}
	}

	dispatchDesign, dispatchData, err := LoadProviderRealDispatchDesign(realDispatchDesignPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.realDispatchDesignSHA256 = sha256Hex(dispatchData)
		if !dispatchDesign.RealDispatchDesignReady {
			failures = append(failures, "real dispatch design real_dispatch_design_ready must be true")
		}
		if dispatchDesign.ExecuteSubcommandRegistered || dispatchDesign.RealDispatchCommandAvailable {
			failures = append(failures, "real dispatch design must keep execute command disabled")
		}
		if dispatchDesign.ProviderCall || dispatchDesign.NetworkCall || dispatchDesign.TransportCalled || dispatchDesign.SecretValuesRead || dispatchDesign.SentToProvider {
			failures = append(failures, "real dispatch design must keep execution flags blocked")
		}
	}

	finalAudit, finalAuditData, err := LoadProviderActivationFinalAudit(activationFinalAuditPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.activationFinalAuditSHA256 = sha256Hex(finalAuditData)
		if finalAudit.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 == "" {
			chain.providerPayloadSHA256 = finalAudit.ProviderPayloadSHA256
		}
		if !finalAudit.FinalAuditReady {
			failures = append(failures, "activation final audit final_audit_ready must be true")
		}
		if finalAudit.ActivationAllowedNow || finalAudit.RealActivationSupportedNow || finalAudit.ProviderCall || finalAudit.NetworkCall || finalAudit.SecretValuesRead || finalAudit.TransportCalled {
			failures = append(failures, "activation final audit must keep execution flags blocked")
		}
		chain.killSwitchActive = finalAudit.KillSwitchActive
		chain.operatorReviewRequired = finalAudit.OperatorReviewRequired
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
		if !ciReport.KillSwitchActive {
			failures = append(failures, "activation ci report kill_switch_active must be true")
		}
		if !ciReport.OperatorReviewRequired {
			failures = append(failures, "activation ci report operator_review_required must be true")
		}
	}

	killSwitchPlan, killSwitchData, err := LoadProviderActivationKillSwitchPlan(killSwitchPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.killSwitchPlanSHA256 = sha256Hex(killSwitchData)
		if !killSwitchPlan.KillSwitchPlanReady || !killSwitchPlan.GlobalDisabled {
			failures = append(failures, "kill switch plan must be ready with global_disabled true")
			chain.killSwitchActive = false
		}
	}

	bundle, bundleData, err := LoadProviderActivationOperatorReviewBundle(operatorReviewBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.operatorReviewBundleSHA256 = sha256Hex(bundleData)
		if !bundle.OperatorReviewBundleReady || !bundle.OperatorReviewRequired {
			failures = append(failures, "operator review bundle must be ready and required")
			chain.operatorReviewRequired = false
		}
		if bundle.OperatorApprovedNow || bundle.ActivationAllowedNow {
			failures = append(failures, "operator review bundle must keep approval blocked")
		}
	}

	return chain, failures, nil
}

func blockedProviderRealActivationDesignReviewPackage(chain providerRealActivationDesignReviewChain, failures []string) ProviderRealActivationDesignReviewPackageResult {
	result := ProviderRealActivationDesignReviewPackageResult{
		Status:                                lancedbpolicy.StatusOK,
		RealDispatchSupportedNow:              false,
		ActivationAllowedNow:                  false,
		SecretValuesRead:                      false,
		ProviderCall:                          false,
		NetworkCall:                           false,
		TransportCalled:                       false,
		SentToProvider:                        false,
		ReceivedFromProvider:                  false,
		WorkspaceModified:                     false,
		DiffApplied:                           false,
		CommitCreated:                         false,
		PRCreated:                             false,
		WorkerExecution:                       false,
		PromptInjectionRealRunner:             false,
		BlockedReason:                         ProviderCallExecutorBlockedReason,
		KillSwitchActive:                      chain.killSwitchActive,
		OperatorReviewRequired:                chain.operatorReviewRequired,
		SecretReadProposalSHA256:              chain.secretReadProposalSHA256,
		RealTransportImplementationPlanSHA256: chain.realTransportImplementationPlanSHA256,
		RealDispatchDesignSHA256:              chain.realDispatchDesignSHA256,
		ActivationFinalAuditSHA256:            chain.activationFinalAuditSHA256,
		ActivationCIReportSHA256:              chain.activationCIReportSHA256,
		KillSwitchPlanSHA256:                  chain.killSwitchPlanSHA256,
		OperatorReviewBundleSHA256:            chain.operatorReviewBundleSHA256,
		ProviderPayloadSHA256:                 chain.providerPayloadSHA256,
		ReviewChecklist:                       append([]string(nil), providerRealActivationDesignReviewChecklist...),
		Failures:                              failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func blockedProviderRealActivationDesignReviewGate(chain providerRealActivationDesignReviewChain, failures []string) ProviderRealActivationDesignReviewGateResult {
	result := ProviderRealActivationDesignReviewGateResult{
		Status:                                lancedbpolicy.StatusOK,
		RealDispatchSupportedNow:              false,
		ActivationAllowedNow:                  false,
		SecretValuesRead:                      false,
		ProviderCall:                          false,
		NetworkCall:                           false,
		TransportCalled:                       false,
		SentToProvider:                        false,
		ReceivedFromProvider:                  false,
		WorkspaceModified:                     false,
		DiffApplied:                           false,
		CommitCreated:                         false,
		PRCreated:                             false,
		WorkerExecution:                       false,
		PromptInjectionRealRunner:             false,
		BlockedReason:                         ProviderCallExecutorBlockedReason,
		KillSwitchActive:                      chain.killSwitchActive,
		OperatorReviewRequired:                chain.operatorReviewRequired,
		SecretReadProposalSHA256:              chain.secretReadProposalSHA256,
		RealTransportImplementationPlanSHA256: chain.realTransportImplementationPlanSHA256,
		RealDispatchDesignSHA256:              chain.realDispatchDesignSHA256,
		ActivationFinalAuditSHA256:            chain.activationFinalAuditSHA256,
		ActivationCIReportSHA256:              chain.activationCIReportSHA256,
		ProviderPayloadSHA256:                 chain.providerPayloadSHA256,
		Failures:                              failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderRealActivationDesignReviewPackage(path string) (ProviderRealActivationDesignReviewPackageResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider real activation design review package", path)
	if err != nil {
		return ProviderRealActivationDesignReviewPackageResult{}, nil, err
	}
	var result ProviderRealActivationDesignReviewPackageResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderRealActivationDesignReviewPackageResult{}, nil, fmt.Errorf("parse provider real activation design review package json: %w", err)
	}
	return result, data, nil
}

func writeProviderRealActivationDesignReviewPackageJSON(path string, result ProviderRealActivationDesignReviewPackageResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider real activation design review package output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider real activation design review package json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider real activation design review package must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderRealActivationDesignReviewPackageJSON(result ProviderRealActivationDesignReviewPackageResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderRealActivationDesignReviewGateJSON(result ProviderRealActivationDesignReviewGateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderRealActivationDesignReviewGateText(result ProviderRealActivationDesignReviewGateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_real_activation_design_review_gate:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  real_activation_design_gate_ready: %t\n  kill_switch_active: %t\n  operator_review_required: %t\n  blocked_reason: %s\n",
		result.Status, result.RealActivationDesignGateReady, result.KillSwitchActive, result.OperatorReviewRequired, result.BlockedReason)
	return err
}
