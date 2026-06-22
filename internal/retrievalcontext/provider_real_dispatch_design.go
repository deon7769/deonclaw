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

const futureProviderRealDispatchCommand = "deonctl worker codex provider-real-dispatch execute"

var futureProviderRealDispatchConfirmFlags = []string{
	"--confirm-provider-executor-dispatch",
	"--confirm-provider-payload-sha256",
	"--confirm-secret-read-policy-sha256",
	"--confirm-activation-final-audit-sha256",
}

type ProviderRealDispatchDesignOptions struct {
	RealTransportImplementationPlanPath string
	SecretReadProposalPath              string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	ProviderRequestEnvelopePath         string
	ProviderRealCallProposalPath        string
	OutputPath                          string
}

type ProviderRealDispatchDesignResult struct {
	Status                                string    `json:"status"`
	CreatedAt                             time.Time `json:"created_at"`
	RealDispatchDesignReady               bool      `json:"real_dispatch_design_ready"`
	RealDispatchCommandAvailable          bool      `json:"real_dispatch_command_available"`
	ExecuteSubcommandRegistered           bool      `json:"execute_subcommand_registered"`
	ProviderCall                          bool      `json:"provider_call"`
	NetworkCall                           bool      `json:"network_call"`
	TransportCalled                       bool      `json:"transport_called"`
	SecretValuesRead                      bool      `json:"secret_values_read"`
	SentToProvider                        bool      `json:"sent_to_provider"`
	ReceivedFromProvider                  bool      `json:"received_from_provider"`
	WorkspaceModified                     bool      `json:"workspace_modified"`
	DiffApplied                           bool      `json:"diff_applied"`
	CommitCreated                         bool      `json:"commit_created"`
	PRCreated                             bool      `json:"pr_created"`
	WorkerExecution                       bool      `json:"worker_execution"`
	PromptInjectionRealRunner             bool      `json:"prompt_injection_real_runner"`
	BlockedReason                         string    `json:"blocked_reason"`
	FutureDispatchCommand                 string    `json:"future_dispatch_command,omitempty"`
	RequiredConfirmFlags                  []string  `json:"required_confirm_flags,omitempty"`
	RealTransportImplementationPlanSHA256 string    `json:"real_transport_implementation_plan_sha256,omitempty"`
	SecretReadProposalSHA256              string    `json:"secret_read_proposal_sha256,omitempty"`
	ActivationFinalAuditSHA256            string    `json:"activation_final_audit_sha256,omitempty"`
	ActivationCIReportSHA256              string    `json:"activation_ci_report_sha256,omitempty"`
	ProviderRequestEnvelopeSHA256         string    `json:"provider_request_envelope_sha256,omitempty"`
	ProviderRealCallProposalSHA256        string    `json:"provider_real_call_proposal_sha256,omitempty"`
	ProviderPayloadSHA256                 string    `json:"provider_payload_sha256,omitempty"`
	Warnings                              []string  `json:"warnings,omitempty"`
	Failures                              []string  `json:"failures,omitempty"`
}

type ProviderRealDispatchDesignReportOptions struct {
	RealDispatchDesignPath              string
	RealTransportImplementationPlanPath string
	SecretReadProposalPath              string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	ProviderRequestEnvelopePath         string
	ProviderRealCallProposalPath        string
}

type providerRealDispatchDesignChain struct {
	realTransportImplementationPlanSHA256 string
	secretReadProposalSHA256              string
	activationFinalAuditSHA256            string
	activationCIReportSHA256              string
	providerRequestEnvelopeSHA256         string
	providerRealCallProposalSHA256        string
	providerPayloadSHA256                 string
}

func ProviderRealDispatchDesign(opts ProviderRealDispatchDesignOptions) (ProviderRealDispatchDesignResult, error) {
	chain, failures, err := loadProviderRealDispatchDesignChain(
		opts.RealTransportImplementationPlanPath,
		opts.SecretReadProposalPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.ProviderRequestEnvelopePath,
		opts.ProviderRealCallProposalPath,
	)
	if err != nil {
		return ProviderRealDispatchDesignResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealDispatchDesignResult{}, err
	}

	result := blockedProviderRealDispatchDesign(chain, failures)
	if len(failures) == 0 {
		result.RealDispatchDesignReady = true
	}
	if err := writeProviderRealDispatchDesignJSON(opts.OutputPath, result); err != nil {
		return ProviderRealDispatchDesignResult{}, err
	}
	return result, nil
}

func ProviderRealDispatchDesignReport(opts ProviderRealDispatchDesignReportOptions) (ProviderRealDispatchDesignResult, error) {
	design, designData, err := LoadProviderRealDispatchDesign(opts.RealDispatchDesignPath)
	if err != nil {
		return ProviderRealDispatchDesignResult{}, err
	}
	chain, failures, err := loadProviderRealDispatchDesignChain(
		opts.RealTransportImplementationPlanPath,
		opts.SecretReadProposalPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.ProviderRequestEnvelopePath,
		opts.ProviderRealCallProposalPath,
	)
	if err != nil {
		return ProviderRealDispatchDesignResult{}, err
	}
	if !design.RealDispatchDesignReady {
		failures = append(failures, "real dispatch design real_dispatch_design_ready must be true")
	}
	if design.RealDispatchCommandAvailable || design.ExecuteSubcommandRegistered {
		failures = append(failures, "real dispatch design must keep execute command disabled")
	}
	if design.ProviderCall || design.NetworkCall || design.TransportCalled || design.SecretValuesRead || design.SentToProvider || design.ReceivedFromProvider || design.WorkspaceModified || design.DiffApplied || design.CommitCreated || design.PRCreated || design.WorkerExecution || design.PromptInjectionRealRunner {
		failures = append(failures, "real dispatch design must keep execution flags blocked")
	}
	if design.FutureDispatchCommand != futureProviderRealDispatchCommand {
		failures = append(failures, "future_dispatch_command mismatch")
	}
	reconcileProviderExecutorHash("real dispatch design", design.RealTransportImplementationPlanSHA256, chain.realTransportImplementationPlanSHA256, &failures)
	reconcileProviderExecutorHash("real dispatch design", design.SecretReadProposalSHA256, chain.secretReadProposalSHA256, &failures)
	reconcileProviderExecutorHash("real dispatch design", design.ActivationFinalAuditSHA256, chain.activationFinalAuditSHA256, &failures)
	reconcileProviderExecutorHash("real dispatch design", design.ActivationCIReportSHA256, chain.activationCIReportSHA256, &failures)
	reconcileProviderExecutorHash("real dispatch design", design.ProviderRequestEnvelopeSHA256, chain.providerRequestEnvelopeSHA256, &failures)
	reconcileProviderExecutorHash("real dispatch design", design.ProviderRealCallProposalSHA256, chain.providerRealCallProposalSHA256, &failures)
	reconcileProviderExecutorHash("real dispatch design", design.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
	if sha256Hex(designData) == "" {
		failures = append(failures, "real dispatch design hash missing")
	}

	result := blockedProviderRealDispatchDesign(chain, failures)
	if len(failures) == 0 {
		result.RealDispatchDesignReady = true
	}
	return result, nil
}

func loadProviderRealDispatchDesignChain(realTransportPlanPath, secretReadProposalPath, activationFinalAuditPath, activationCIReportPath, requestEnvelopePath, realCallProposalPath string) (providerRealDispatchDesignChain, []string, error) {
	var chain providerRealDispatchDesignChain
	failures := []string{}

	transportPlan, transportData, err := LoadProviderRealTransportImplementationPlan(realTransportPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.realTransportImplementationPlanSHA256 = sha256Hex(transportData)
		chain.providerPayloadSHA256 = transportPlan.ProviderPayloadSHA256
		if !transportPlan.RealTransportImplementationPlanReady {
			failures = append(failures, "real transport implementation plan real_transport_implementation_plan_ready must be true")
		}
		if transportPlan.RealTransportAvailableNow || transportPlan.TransportEnabled || transportPlan.TransportCalled || transportPlan.ProviderCall || transportPlan.NetworkCall || transportPlan.SecretValuesRead {
			failures = append(failures, "real transport implementation plan must keep execution flags blocked")
		}
	}

	proposal, proposalData, err := LoadProviderSecretReadProposal(secretReadProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.secretReadProposalSHA256 = sha256Hex(proposalData)
		if !proposal.SecretReadProposalReady {
			failures = append(failures, "secret read proposal secret_read_proposal_ready must be true")
		}
		if proposal.SecretReadAllowedNow || proposal.SecretValuesRead {
			failures = append(failures, "secret read proposal must keep secret reads blocked")
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
		if !finalAudit.FinalAuditReady || !finalAudit.KillSwitchActive {
			failures = append(failures, "activation final audit must be ready with kill switch active")
		}
		if finalAudit.ActivationAllowedNow || finalAudit.ProviderCall || finalAudit.NetworkCall || finalAudit.SecretValuesRead || finalAudit.TransportCalled {
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
	}

	envelope, envelopeData, err := LoadProviderRequestEnvelope(requestEnvelopePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.providerRequestEnvelopeSHA256 = sha256Hex(envelopeData)
		if !envelope.ProviderRequestReady {
			failures = append(failures, "provider request envelope provider_request_ready must be true")
		}
		if envelope.ProviderCall || envelope.NetworkCall || envelope.TransportCalled || envelope.SentToProvider {
			failures = append(failures, "provider request envelope must keep execution flags blocked")
		}
	}

	realCallProposal, realCallData, err := LoadProviderRealCallProposal(realCallProposalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.providerRealCallProposalSHA256 = sha256Hex(realCallData)
		if realCallProposal.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 == "" {
			chain.providerPayloadSHA256 = realCallProposal.ProviderPayloadSHA256
		}
		if !realCallProposal.RealCallProposalReady {
			failures = append(failures, "provider real call proposal real_call_proposal_ready must be true")
		}
		if realCallProposal.ProviderCallAllowedNow || realCallProposal.ProviderCall || realCallProposal.NetworkCall || realCallProposal.SecretValuesRead || realCallProposal.TransportCalled || realCallProposal.SentToProvider {
			failures = append(failures, "provider real call proposal must keep execution flags blocked")
		}
	}

	return chain, failures, nil
}

func blockedProviderRealDispatchDesign(chain providerRealDispatchDesignChain, failures []string) ProviderRealDispatchDesignResult {
	result := ProviderRealDispatchDesignResult{
		Status:                                lancedbpolicy.StatusOK,
		CreatedAt:                             time.Now().UTC(),
		RealDispatchCommandAvailable:          false,
		ExecuteSubcommandRegistered:           false,
		ProviderCall:                          false,
		NetworkCall:                           false,
		TransportCalled:                       false,
		SecretValuesRead:                      false,
		SentToProvider:                        false,
		ReceivedFromProvider:                  false,
		WorkspaceModified:                     false,
		DiffApplied:                           false,
		CommitCreated:                         false,
		PRCreated:                             false,
		WorkerExecution:                       false,
		PromptInjectionRealRunner:             false,
		BlockedReason:                         ProviderCallExecutorBlockedReason,
		FutureDispatchCommand:                 futureProviderRealDispatchCommand,
		RequiredConfirmFlags:                  append([]string(nil), futureProviderRealDispatchConfirmFlags...),
		RealTransportImplementationPlanSHA256: chain.realTransportImplementationPlanSHA256,
		SecretReadProposalSHA256:              chain.secretReadProposalSHA256,
		ActivationFinalAuditSHA256:            chain.activationFinalAuditSHA256,
		ActivationCIReportSHA256:              chain.activationCIReportSHA256,
		ProviderRequestEnvelopeSHA256:         chain.providerRequestEnvelopeSHA256,
		ProviderRealCallProposalSHA256:        chain.providerRealCallProposalSHA256,
		ProviderPayloadSHA256:                 chain.providerPayloadSHA256,
		Failures:                              failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderRealDispatchDesign(path string) (ProviderRealDispatchDesignResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider real dispatch design", path)
	if err != nil {
		return ProviderRealDispatchDesignResult{}, nil, err
	}
	var result ProviderRealDispatchDesignResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderRealDispatchDesignResult{}, nil, fmt.Errorf("parse provider real dispatch design json: %w", err)
	}
	return result, data, nil
}

func writeProviderRealDispatchDesignJSON(path string, result ProviderRealDispatchDesignResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider real dispatch design output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider real dispatch design json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider real dispatch design must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderRealDispatchDesignJSON(result ProviderRealDispatchDesignResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderRealDispatchDesignText(result ProviderRealDispatchDesignResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_real_dispatch_design:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  real_dispatch_design_ready: %t\n  execute_subcommand_registered: %t\n  future_dispatch_command: %s\n  blocked_reason: %s\n",
		result.Status, result.RealDispatchDesignReady, result.ExecuteSubcommandRegistered, result.FutureDispatchCommand, result.BlockedReason)
	return err
}
