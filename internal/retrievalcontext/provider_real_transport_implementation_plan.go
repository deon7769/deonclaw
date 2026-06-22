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

const futureRealProviderTransportMode = "FutureRealProviderTransport"
const activeBlockedProviderTransportMode = "BlockedProviderTransport"

type ProviderRealTransportImplementationPlanOptions struct {
	SecretReadProposalPath   string
	ProviderAdapterPlanPath  string
	ActivationFinalAuditPath string
	OperatorReviewBundlePath string
	KillSwitchPlanPath       string
	OutputPath               string
}

type ProviderRealTransportImplementationPlanResult struct {
	Status                               string   `json:"status"`
	RealTransportImplementationPlanReady bool     `json:"real_transport_implementation_plan_ready"`
	RealTransportAvailableNow            bool     `json:"real_transport_available_now"`
	TransportEnabled                     bool     `json:"transport_enabled"`
	TransportCalled                      bool     `json:"transport_called"`
	ProviderCall                         bool     `json:"provider_call"`
	NetworkCall                          bool     `json:"network_call"`
	SecretValuesRead                     bool     `json:"secret_values_read"`
	BlockedReason                        string   `json:"blocked_reason"`
	ActiveTransportMode                  string   `json:"active_transport_mode"`
	FutureTransportMode                  string   `json:"future_transport_mode"`
	FutureTransportContract              []string `json:"future_transport_contract,omitempty"`
	SecretReadProposalSHA256             string   `json:"secret_read_proposal_sha256,omitempty"`
	ProviderAdapterPlanSHA256            string   `json:"provider_adapter_plan_sha256,omitempty"`
	ActivationFinalAuditSHA256           string   `json:"activation_final_audit_sha256,omitempty"`
	OperatorReviewBundleSHA256           string   `json:"operator_review_bundle_sha256,omitempty"`
	KillSwitchPlanSHA256                 string   `json:"kill_switch_plan_sha256,omitempty"`
	ProviderPayloadSHA256                string   `json:"provider_payload_sha256,omitempty"`
	Warnings                             []string `json:"warnings,omitempty"`
	Failures                             []string `json:"failures,omitempty"`
}

type ProviderRealTransportImplementationReportOptions struct {
	RealTransportImplementationPlanPath string
	SecretReadProposalPath              string
	ProviderAdapterPlanPath             string
	ActivationFinalAuditPath            string
	OperatorReviewBundlePath            string
	KillSwitchPlanPath                  string
}

var providerRealTransportFutureContract = []string{
	"future transport must implement ProviderTransport.Deliver without being called in this sprint",
	"future transport must read secrets only through credential policy allowed env var names",
	"future transport must honor kill_switch_active and activation_allowed_now gates",
	"BlockedProviderTransport remains the only active transport implementation",
	"no SDK import, no network dial, and no transport.Deliver invocation in this sprint",
}

type providerRealTransportImplementationChain struct {
	secretReadProposalSHA256   string
	providerAdapterPlanSHA256  string
	activationFinalAuditSHA256 string
	operatorReviewBundleSHA256 string
	killSwitchPlanSHA256       string
	providerPayloadSHA256      string
}

func ProviderRealTransportImplementationPlan(opts ProviderRealTransportImplementationPlanOptions) (ProviderRealTransportImplementationPlanResult, error) {
	chain, failures, err := loadProviderRealTransportImplementationChain(
		opts.SecretReadProposalPath,
		opts.ProviderAdapterPlanPath,
		opts.ActivationFinalAuditPath,
		opts.OperatorReviewBundlePath,
		opts.KillSwitchPlanPath,
	)
	if err != nil {
		return ProviderRealTransportImplementationPlanResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealTransportImplementationPlanResult{}, err
	}

	result := blockedProviderRealTransportImplementationPlan(chain, failures)
	if len(failures) == 0 {
		result.RealTransportImplementationPlanReady = true
	}
	if err := writeProviderRealTransportImplementationPlanJSON(opts.OutputPath, result); err != nil {
		return ProviderRealTransportImplementationPlanResult{}, err
	}
	return result, nil
}

func ProviderRealTransportImplementationReport(opts ProviderRealTransportImplementationReportOptions) (ProviderRealTransportImplementationPlanResult, error) {
	plan, planData, err := LoadProviderRealTransportImplementationPlan(opts.RealTransportImplementationPlanPath)
	if err != nil {
		return ProviderRealTransportImplementationPlanResult{}, err
	}
	chain, failures, err := loadProviderRealTransportImplementationChain(
		opts.SecretReadProposalPath,
		opts.ProviderAdapterPlanPath,
		opts.ActivationFinalAuditPath,
		opts.OperatorReviewBundlePath,
		opts.KillSwitchPlanPath,
	)
	if err != nil {
		return ProviderRealTransportImplementationPlanResult{}, err
	}
	if !plan.RealTransportImplementationPlanReady {
		failures = append(failures, "real transport implementation plan real_transport_implementation_plan_ready must be true")
	}
	if plan.RealTransportAvailableNow || plan.TransportEnabled || plan.TransportCalled || plan.ProviderCall || plan.NetworkCall || plan.SecretValuesRead {
		failures = append(failures, "real transport implementation plan must keep execution flags blocked")
	}
	if plan.ActiveTransportMode != activeBlockedProviderTransportMode {
		failures = append(failures, "active_transport_mode must remain BlockedProviderTransport")
	}
	reconcileProviderExecutorHash("real transport implementation plan", plan.SecretReadProposalSHA256, chain.secretReadProposalSHA256, &failures)
	reconcileProviderExecutorHash("real transport implementation plan", plan.ProviderAdapterPlanSHA256, chain.providerAdapterPlanSHA256, &failures)
	reconcileProviderExecutorHash("real transport implementation plan", plan.ActivationFinalAuditSHA256, chain.activationFinalAuditSHA256, &failures)
	reconcileProviderExecutorHash("real transport implementation plan", plan.OperatorReviewBundleSHA256, chain.operatorReviewBundleSHA256, &failures)
	reconcileProviderExecutorHash("real transport implementation plan", plan.KillSwitchPlanSHA256, chain.killSwitchPlanSHA256, &failures)
	reconcileProviderExecutorHash("real transport implementation plan", plan.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
	if sha256Hex(planData) == "" {
		failures = append(failures, "real transport implementation plan hash missing")
	}

	result := blockedProviderRealTransportImplementationPlan(chain, failures)
	if len(failures) == 0 {
		result.RealTransportImplementationPlanReady = true
	}
	return result, nil
}

func loadProviderRealTransportImplementationChain(secretReadProposalPath, providerAdapterPlanPath, activationFinalAuditPath, operatorReviewBundlePath, killSwitchPlanPath string) (providerRealTransportImplementationChain, []string, error) {
	var chain providerRealTransportImplementationChain
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
		if proposal.SecretReadAllowedNow || proposal.SecretValuesRead || proposal.ProviderCall || proposal.NetworkCall || proposal.TransportCalled {
			failures = append(failures, "secret read proposal must keep execution flags blocked")
		}
	}

	adapterPlan, adapterData, err := LoadProviderAdapterPlan(providerAdapterPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.providerAdapterPlanSHA256 = sha256Hex(adapterData)
		if !adapterPlan.AdapterPlanReady {
			failures = append(failures, "provider adapter plan adapter_plan_ready must be true")
		}
		if adapterPlan.ProviderCall || adapterPlan.NetworkCall || adapterPlan.TransportCalled || adapterPlan.SentToProvider {
			failures = append(failures, "provider adapter plan must keep execution flags blocked")
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

	bundle, bundleData, err := LoadProviderActivationOperatorReviewBundle(operatorReviewBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.operatorReviewBundleSHA256 = sha256Hex(bundleData)
		if !bundle.OperatorReviewBundleReady || !bundle.OperatorReviewRequired {
			failures = append(failures, "operator review bundle must be ready and required")
		}
		if bundle.OperatorApprovedNow || bundle.ActivationAllowedNow || bundle.ProviderCall || bundle.NetworkCall || bundle.SecretValuesRead || bundle.TransportCalled {
			failures = append(failures, "operator review bundle must keep execution flags blocked")
		}
	}

	killSwitchPlan, killSwitchData, err := LoadProviderActivationKillSwitchPlan(killSwitchPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.killSwitchPlanSHA256 = sha256Hex(killSwitchData)
		if !killSwitchPlan.KillSwitchPlanReady || !killSwitchPlan.GlobalDisabled {
			failures = append(failures, "kill switch plan must be ready with global_disabled true")
		}
	}

	return chain, failures, nil
}

func blockedProviderRealTransportImplementationPlan(chain providerRealTransportImplementationChain, failures []string) ProviderRealTransportImplementationPlanResult {
	result := ProviderRealTransportImplementationPlanResult{
		Status:                     lancedbpolicy.StatusOK,
		RealTransportAvailableNow:  false,
		TransportEnabled:           false,
		TransportCalled:            false,
		ProviderCall:               false,
		NetworkCall:                false,
		SecretValuesRead:           false,
		BlockedReason:              ProviderCallExecutorBlockedReason,
		ActiveTransportMode:        activeBlockedProviderTransportMode,
		FutureTransportMode:        futureRealProviderTransportMode,
		FutureTransportContract:    append([]string(nil), providerRealTransportFutureContract...),
		SecretReadProposalSHA256:   chain.secretReadProposalSHA256,
		ProviderAdapterPlanSHA256:  chain.providerAdapterPlanSHA256,
		ActivationFinalAuditSHA256: chain.activationFinalAuditSHA256,
		OperatorReviewBundleSHA256: chain.operatorReviewBundleSHA256,
		KillSwitchPlanSHA256:       chain.killSwitchPlanSHA256,
		ProviderPayloadSHA256:      chain.providerPayloadSHA256,
		Failures:                   failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderRealTransportImplementationPlan(path string) (ProviderRealTransportImplementationPlanResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider real transport implementation plan", path)
	if err != nil {
		return ProviderRealTransportImplementationPlanResult{}, nil, err
	}
	var result ProviderRealTransportImplementationPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderRealTransportImplementationPlanResult{}, nil, fmt.Errorf("parse provider real transport implementation plan json: %w", err)
	}
	return result, data, nil
}

func writeProviderRealTransportImplementationPlanJSON(path string, result ProviderRealTransportImplementationPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider real transport implementation plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider real transport implementation plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider real transport implementation plan must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderRealTransportImplementationPlanJSON(result ProviderRealTransportImplementationPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderRealTransportImplementationPlanText(result ProviderRealTransportImplementationPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_real_transport_implementation_plan:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  real_transport_implementation_plan_ready: %t\n  active_transport_mode: %s\n  blocked_reason: %s\n",
		result.Status, result.RealTransportImplementationPlanReady, result.ActiveTransportMode, result.BlockedReason)
	return err
}
