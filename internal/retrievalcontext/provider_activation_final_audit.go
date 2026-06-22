package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

var providerActivationCIExpectedCommands = []string{
	"gofmt -w .",
	"git diff --check",
	"go test ./...",
	"make provider-call-chain-smoke",
}

type ProviderActivationFinalAuditOptions struct {
	ActivationReleasePackagePath     string
	ActivationReleaseGatePath        string
	OperatorReviewBundlePath         string
	KillSwitchPlanPath               string
	ActivationPolicyPlanPath         string
	ActivationReadinessAuditPath     string
	RealCallProposalPath             string
	CredentialPolicyPlanPath         string
	ResponseChangeProposalReportPath string
	ActivationApprovalPath           string
	ActivationRehearsalPath          string
	ExecutionSimulationReportPath    string
}

type ProviderActivationFinalAuditResult struct {
	Status                     string   `json:"status"`
	FinalAuditReady            bool     `json:"final_audit_ready"`
	KillSwitchActive           bool     `json:"kill_switch_active"`
	OperatorReviewRequired     bool     `json:"operator_review_required"`
	RealActivationSupportedNow bool     `json:"real_activation_supported_now"`
	ActivationAllowedNow       bool     `json:"activation_allowed_now"`
	ProviderCall               bool     `json:"provider_call"`
	NetworkCall                bool     `json:"network_call"`
	SecretValuesRead           bool     `json:"secret_values_read"`
	TransportCalled            bool     `json:"transport_called"`
	SentToProvider             bool     `json:"sent_to_provider"`
	ReceivedFromProvider       bool     `json:"received_from_provider"`
	WorkspaceModified          bool     `json:"workspace_modified"`
	DiffApplied                bool     `json:"diff_applied"`
	CommitCreated              bool     `json:"commit_created"`
	PRCreated                  bool     `json:"pr_created"`
	WorkerExecution            bool     `json:"worker_execution"`
	PromptInjectionRealRunner  bool     `json:"prompt_injection_real_runner"`
	BlockedReason              string   `json:"blocked_reason"`
	Warnings                   []string `json:"warnings,omitempty"`
	Failures                   []string `json:"failures,omitempty"`
}

type ProviderActivationCIReportOptions struct {
	ProviderActivationFinalAuditOptions
}

type ProviderActivationCIReportResult struct {
	Status                     string   `json:"status"`
	CIObservabilityReady       bool     `json:"ci_observability_ready"`
	FinalAuditReady            bool     `json:"final_audit_ready"`
	KillSwitchActive           bool     `json:"kill_switch_active"`
	OperatorReviewRequired     bool     `json:"operator_review_required"`
	RealActivationSupportedNow bool     `json:"real_activation_supported_now"`
	ActivationAllowedNow       bool     `json:"activation_allowed_now"`
	ProviderCall               bool     `json:"provider_call"`
	NetworkCall                bool     `json:"network_call"`
	SecretValuesRead           bool     `json:"secret_values_read"`
	TransportCalled            bool     `json:"transport_called"`
	SentToProvider             bool     `json:"sent_to_provider"`
	ReceivedFromProvider       bool     `json:"received_from_provider"`
	WorkspaceModified          bool     `json:"workspace_modified"`
	DiffApplied                bool     `json:"diff_applied"`
	CommitCreated              bool     `json:"commit_created"`
	PRCreated                  bool     `json:"pr_created"`
	WorkerExecution            bool     `json:"worker_execution"`
	PromptInjectionRealRunner  bool     `json:"prompt_injection_real_runner"`
	BlockedReason              string   `json:"blocked_reason"`
	ExpectedLocalCommands      []string `json:"expected_local_commands,omitempty"`
	ExpectedPRChecklist        []string `json:"expected_pr_checklist,omitempty"`
	Warnings                   []string `json:"warnings,omitempty"`
	Failures                   []string `json:"failures,omitempty"`
}

var providerActivationExpectedPRChecklist = []string{
	"make provider-call-chain-smoke passes on PR branch",
	"activation_allowed_now remains false in all metadata artifacts",
	"kill_switch_active remains true",
	"operator_review_required remains true and operator_approved_now remains false",
	"metadata anti-leak checks pass on JSON and stdout",
	"do not merge until CI smoke is green",
}

func ProviderActivationFinalAudit(opts ProviderActivationFinalAuditOptions) (ProviderActivationFinalAuditResult, error) {
	failures := []string{}

	killSwitch, _, err := LoadProviderActivationKillSwitchPlan(opts.KillSwitchPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if !killSwitch.KillSwitchValidated || !killSwitch.KillSwitchPlanReady {
			failures = append(failures, "kill switch plan must be validated and ready")
		}
		if !killSwitch.GlobalDisabled || !killSwitch.ProviderCallBlocked || !killSwitch.NetworkBlocked || !killSwitch.SecretReadBlocked || !killSwitch.TransportBlocked || !killSwitch.WorkspaceWriteBlocked {
			failures = append(failures, "kill switch must keep all block flags true")
		}
		if killSwitch.ActivationAllowedNow {
			failures = append(failures, "kill switch activation_allowed_now must be false")
		}
	}

	operatorReview, _, err := LoadProviderActivationOperatorReviewBundle(opts.OperatorReviewBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if !operatorReview.OperatorReviewBundleReady {
			failures = append(failures, "operator review bundle operator_review_bundle_ready must be true")
		}
		if !operatorReview.OperatorReviewRequired {
			failures = append(failures, "operator review bundle operator_review_required must be true")
		}
		if operatorReview.OperatorApprovedNow {
			failures = append(failures, "operator review bundle operator_approved_now must be false")
		}
		if operatorReview.ActivationAllowedNow || operatorReview.ProviderCall || operatorReview.NetworkCall || operatorReview.SecretValuesRead || operatorReview.TransportCalled || operatorReview.WorkspaceModified {
			failures = append(failures, "operator review bundle must keep execution flags blocked")
		}
	}

	gateData, err := readArtifactBytesNoTextExcerpt("activation release gate", opts.ActivationReleaseGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		var gate ProviderActivationReleaseGateResult
		if err := json.Unmarshal(gateData, &gate); err != nil {
			failures = append(failures, fmt.Sprintf("parse activation release gate: %v", err))
		} else {
			if !gate.ActivationGateReady {
				failures = append(failures, "activation release gate activation_gate_ready must be true")
			}
			if gate.ActivationAllowedNow || gate.RealActivationSupportedNow {
				failures = append(failures, "activation release gate must keep activation blocked")
			}
			if gate.ProviderCall || gate.NetworkCall || gate.SecretValuesRead || gate.TransportCalled || gate.WorkspaceModified {
				failures = append(failures, "activation release gate must keep execution flags blocked")
			}
		}
	}

	pkg, _, err := LoadProviderActivationReleasePackage(opts.ActivationReleasePackagePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if pkg.ActivationAllowedNow || pkg.RealActivationSupportedNow {
		failures = append(failures, "activation release package must keep activation blocked")
	}

	policyPlan, _, err := LoadProviderActivationPolicyPlan(opts.ActivationPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if policyPlan.ActivationAllowedNow {
		failures = append(failures, "activation policy plan activation_allowed_now must be false")
	}

	readinessAudit, _, err := LoadProviderActivationReadinessAudit(opts.ActivationReadinessAuditPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if readinessAudit.ActivationAllowedNow || readinessAudit.RealProviderCallSupportedNow {
		failures = append(failures, "activation readiness audit must keep activation blocked")
	}

	result := blockedProviderActivationFinalAuditResult(failures)
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.FinalAuditReady = true
		result.KillSwitchActive = killSwitch.KillSwitchValidated && killSwitch.GlobalDisabled
	}
	return result, nil
}

func blockedProviderActivationFinalAuditResult(failures []string) ProviderActivationFinalAuditResult {
	return ProviderActivationFinalAuditResult{
		Status:                     lancedbpolicy.StatusOK,
		KillSwitchActive:           false,
		OperatorReviewRequired:     true,
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
}

func providerActivationFinalAuditResultFromAudit(audit ProviderActivationFinalAuditResult) ProviderActivationCIReportResult {
	return ProviderActivationCIReportResult{
		Status:                     audit.Status,
		FinalAuditReady:            audit.FinalAuditReady,
		KillSwitchActive:           audit.KillSwitchActive,
		OperatorReviewRequired:     audit.OperatorReviewRequired,
		RealActivationSupportedNow: audit.RealActivationSupportedNow,
		ActivationAllowedNow:       audit.ActivationAllowedNow,
		ProviderCall:               audit.ProviderCall,
		NetworkCall:                audit.NetworkCall,
		SecretValuesRead:           audit.SecretValuesRead,
		TransportCalled:            audit.TransportCalled,
		SentToProvider:             audit.SentToProvider,
		ReceivedFromProvider:       audit.ReceivedFromProvider,
		WorkspaceModified:          audit.WorkspaceModified,
		DiffApplied:                audit.DiffApplied,
		CommitCreated:              audit.CommitCreated,
		PRCreated:                  audit.PRCreated,
		WorkerExecution:            audit.WorkerExecution,
		PromptInjectionRealRunner:  audit.PromptInjectionRealRunner,
		BlockedReason:              audit.BlockedReason,
		Failures:                   append([]string(nil), audit.Failures...),
	}
}

func ProviderActivationCIReport(opts ProviderActivationCIReportOptions) (ProviderActivationCIReportResult, error) {
	audit, err := ProviderActivationFinalAudit(opts.ProviderActivationFinalAuditOptions)
	if err != nil {
		return ProviderActivationCIReportResult{}, err
	}
	result := providerActivationFinalAuditResultFromAudit(audit)
	result.ExpectedLocalCommands = append([]string(nil), providerActivationCIExpectedCommands...)
	result.ExpectedPRChecklist = append([]string(nil), providerActivationExpectedPRChecklist...)
	if audit.Status == lancedbpolicy.StatusFailed {
		return result, nil
	}
	result.CIObservabilityReady = true
	return result, nil
}

func WriteProviderActivationFinalAuditJSON(result ProviderActivationFinalAuditResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation final audit must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}

func WriteProviderActivationFinalAuditText(result ProviderActivationFinalAuditResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_final_audit:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  final_audit_ready: %t\n  kill_switch_active: %t\n  operator_review_required: %t\n  activation_allowed_now: %t\n  blocked_reason: %s\n",
		result.Status, result.FinalAuditReady, result.KillSwitchActive, result.OperatorReviewRequired, result.ActivationAllowedNow, result.BlockedReason)
	return err
}

func WriteProviderActivationCIReportJSON(result ProviderActivationCIReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation ci report must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}

func WriteProviderActivationCIReportText(result ProviderActivationCIReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_ci_report:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  ci_observability_ready: %t\n  final_audit_ready: %t\n  kill_switch_active: %t\n  activation_allowed_now: %t\n  blocked_reason: %s\n",
		result.Status, result.CIObservabilityReady, result.FinalAuditReady, result.KillSwitchActive, result.ActivationAllowedNow, result.BlockedReason)
	return err
}
