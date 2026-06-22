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

var providerActivationOperatorReviewChecklist = []string{
	"policy_still_disabled",
	"no_secret_reads",
	"no_network",
	"no_transport",
	"no_workspace_modifications",
	"fake_response_only",
	"activation_not_allowed_now",
}

type ProviderActivationOperatorReviewBundleOptions struct {
	ActivationReleasePackagePath     string
	ActivationReleaseGatePath        string
	ActivationPolicyPlanPath         string
	ActivationApprovalPath           string
	ActivationRehearsalPath          string
	ActivationReadinessAuditPath     string
	RealCallProposalPath             string
	ExecutionSimulationReportPath    string
	CredentialPolicyPlanPath         string
	ResponseChangeProposalReportPath string
	OutputPath                       string
}

type ProviderActivationOperatorReviewBundleResult struct {
	Status                         string   `json:"status"`
	OperatorReviewBundleReady      bool     `json:"operator_review_bundle_ready"`
	OperatorReviewRequired         bool     `json:"operator_review_required"`
	OperatorApprovedNow            bool     `json:"operator_approved_now"`
	ActivationAllowedNow           bool     `json:"activation_allowed_now"`
	ProviderCall                   bool     `json:"provider_call"`
	NetworkCall                    bool     `json:"network_call"`
	SecretValuesRead               bool     `json:"secret_values_read"`
	TransportCalled                bool     `json:"transport_called"`
	WorkspaceModified              bool     `json:"workspace_modified"`
	BlockedReason                  string   `json:"blocked_reason"`
	ReviewChecklist                []string `json:"review_checklist,omitempty"`
	ActivationPolicyPlanSHA256     string   `json:"activation_policy_plan_sha256,omitempty"`
	ActivationApprovalSHA256       string   `json:"activation_approval_sha256,omitempty"`
	ActivationReleasePackageSHA256 string   `json:"activation_release_package_sha256,omitempty"`
	RealCallProposalSHA256         string   `json:"real_call_proposal_sha256,omitempty"`
	CredentialPolicyPlanSHA256     string   `json:"credential_policy_plan_sha256,omitempty"`
	ProviderPayloadSHA256          string   `json:"provider_payload_sha256,omitempty"`
	Warnings                       []string `json:"warnings,omitempty"`
	Failures                       []string `json:"failures,omitempty"`
}

type ProviderActivationOperatorReviewReportOptions struct {
	OperatorReviewBundlePath         string
	ActivationReleasePackagePath     string
	ActivationReleaseGatePath        string
	ActivationPolicyPlanPath         string
	ActivationApprovalPath           string
	ActivationRehearsalPath          string
	ActivationReadinessAuditPath     string
	RealCallProposalPath             string
	ExecutionSimulationReportPath    string
	CredentialPolicyPlanPath         string
	ResponseChangeProposalReportPath string
}

func ProviderActivationOperatorReviewBundle(opts ProviderActivationOperatorReviewBundleOptions) (ProviderActivationOperatorReviewBundleResult, error) {
	chain, failures, err := loadProviderActivationReleaseChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationApprovalPath,
		opts.ActivationRehearsalPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.ExecutionSimulationReportPath,
		opts.CredentialPolicyPlanPath,
		opts.ResponseChangeProposalReportPath,
	)
	if err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}
	if err := validateRelativeSafePath("activation release package path", opts.ActivationReleasePackagePath); err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}

	pkg, pkgData, err := LoadProviderActivationReleasePackage(opts.ActivationReleasePackagePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		reconcileProviderExecutorHash("operator review release package", pkg.ActivationPolicyPlanSHA256, chain.activationPolicyPlanSHA256, &failures)
		reconcileProviderExecutorHash("operator review release package", pkg.ActivationApprovalSHA256, chain.activationApprovalSHA256, &failures)
		reconcileProviderExecutorHash("operator review release package", pkg.RealCallProposalSHA256, chain.realCallProposalSHA256, &failures)
		reconcileProviderExecutorHash("operator review release package", pkg.CredentialPolicyPlanSHA256, chain.credentialPolicyPlanSHA256, &failures)
		reconcileProviderExecutorHash("operator review release package", pkg.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
		if !pkg.ActivationReleasePackageReady {
			failures = append(failures, "activation release package activation_release_package_ready must be true")
		}
		if pkg.ActivationAllowedNow || pkg.RealActivationSupportedNow || pkg.ProviderCall || pkg.NetworkCall || pkg.SecretValuesRead || pkg.TransportCalled || pkg.WorkspaceModified {
			failures = append(failures, "activation release package must keep execution flags blocked")
		}
	}

	if opts.ActivationReleaseGatePath != "" {
		if err := validateRelativeSafePath("activation release gate path", opts.ActivationReleaseGatePath); err != nil {
			return ProviderActivationOperatorReviewBundleResult{}, err
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
				if gate.ActivationAllowedNow || gate.RealActivationSupportedNow || gate.ProviderCall || gate.NetworkCall || gate.SecretValuesRead || gate.TransportCalled || gate.WorkspaceModified {
					failures = append(failures, "activation release gate must keep execution flags blocked")
				}
			}
		}
	} else {
		gate, err := ProviderActivationReleaseGate(ProviderActivationReleaseGateOptions{
			ActivationPolicyPlanPath:         opts.ActivationPolicyPlanPath,
			ActivationApprovalPath:           opts.ActivationApprovalPath,
			ActivationRehearsalPath:          opts.ActivationRehearsalPath,
			ActivationReadinessAuditPath:     opts.ActivationReadinessAuditPath,
			RealCallProposalPath:             opts.RealCallProposalPath,
			ExecutionSimulationReportPath:    opts.ExecutionSimulationReportPath,
			CredentialPolicyPlanPath:         opts.CredentialPolicyPlanPath,
			ResponseChangeProposalReportPath: opts.ResponseChangeProposalReportPath,
			ActivationReleasePackagePath:     opts.ActivationReleasePackagePath,
		})
		if err != nil {
			return ProviderActivationOperatorReviewBundleResult{}, err
		}
		if gate.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, gate.Failures...)
		}
	}

	approvalData, err := readArtifactBytesNoTextExcerpt("activation approval", opts.ActivationApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		var approval ProviderActivationApproval
		if err := json.Unmarshal(approvalData, &approval); err != nil {
			failures = append(failures, fmt.Sprintf("parse activation approval: %v", err))
		} else if !approval.ActivationAuthorizedForFuture {
			failures = append(failures, "activation approval activation_authorized_for_future must be true")
		}
		if sha256Hex(approvalData) != chain.activationApprovalSHA256 {
			failures = append(failures, "activation approval hash mismatch")
		}
	}

	policyPlanData, err := readArtifactBytesNoTextExcerpt("activation policy plan", opts.ActivationPolicyPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if sha256Hex(policyPlanData) != chain.activationPolicyPlanSHA256 {
		failures = append(failures, "activation policy plan hash mismatch")
	}

	result := blockedProviderActivationOperatorReviewBundleResult(chain, pkgData, failures)
	if len(failures) == 0 {
		result.OperatorReviewBundleReady = true
	}
	if err := writeProviderActivationOperatorReviewBundleJSON(opts.OutputPath, result); err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}
	return result, nil
}

func ProviderActivationOperatorReviewReport(opts ProviderActivationOperatorReviewReportOptions) (ProviderActivationOperatorReviewBundleResult, error) {
	if err := validateRelativeSafePath("operator review bundle path", opts.OperatorReviewBundlePath); err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}
	bundle, bundleData, err := LoadProviderActivationOperatorReviewBundle(opts.OperatorReviewBundlePath)
	if err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}
	chain, failures, err := loadProviderActivationReleaseChain(
		opts.ActivationPolicyPlanPath,
		opts.ActivationApprovalPath,
		opts.ActivationRehearsalPath,
		opts.ActivationReadinessAuditPath,
		opts.RealCallProposalPath,
		opts.ExecutionSimulationReportPath,
		opts.CredentialPolicyPlanPath,
		opts.ResponseChangeProposalReportPath,
	)
	if err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, err
	}
	reconcileProviderExecutorHash("operator review bundle", bundle.ActivationPolicyPlanSHA256, chain.activationPolicyPlanSHA256, &failures)
	reconcileProviderExecutorHash("operator review bundle", bundle.ActivationApprovalSHA256, chain.activationApprovalSHA256, &failures)
	reconcileProviderExecutorHash("operator review bundle", bundle.RealCallProposalSHA256, chain.realCallProposalSHA256, &failures)
	reconcileProviderExecutorHash("operator review bundle", bundle.CredentialPolicyPlanSHA256, chain.credentialPolicyPlanSHA256, &failures)
	reconcileProviderExecutorHash("operator review bundle", bundle.ProviderPayloadSHA256, chain.providerPayloadSHA256, &failures)
	if !bundle.OperatorReviewBundleReady {
		failures = append(failures, "operator review bundle operator_review_bundle_ready must be true")
	}
	if !bundle.OperatorReviewRequired {
		failures = append(failures, "operator review bundle operator_review_required must be true")
	}
	if bundle.OperatorApprovedNow {
		failures = append(failures, "operator review bundle operator_approved_now must be false")
	}
	if bundle.ActivationAllowedNow || bundle.ProviderCall || bundle.NetworkCall || bundle.SecretValuesRead || bundle.TransportCalled || bundle.WorkspaceModified {
		failures = append(failures, "operator review bundle must keep execution flags blocked")
	}
	if sha256Hex(bundleData) == "" {
		failures = append(failures, "operator review bundle hash missing")
	}
	if bundle.ActivationReleasePackageSHA256 != "" {
		pkgData, err := readArtifactBytesNoTextExcerpt("activation release package", opts.ActivationReleasePackagePath)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			reconcileProviderExecutorHash("operator review bundle release package", bundle.ActivationReleasePackageSHA256, sha256Hex(pkgData), &failures)
		}
	}

	result := bundle
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		result.OperatorReviewBundleReady = false
	} else {
		result.Status = lancedbpolicy.StatusOK
		result.OperatorReviewBundleReady = true
	}
	return result, nil
}

func blockedProviderActivationOperatorReviewBundleResult(chain providerActivationReleaseChain, pkgData []byte, failures []string) ProviderActivationOperatorReviewBundleResult {
	result := ProviderActivationOperatorReviewBundleResult{
		Status:                         lancedbpolicy.StatusOK,
		OperatorReviewRequired:         true,
		OperatorApprovedNow:            false,
		ActivationAllowedNow:           false,
		ProviderCall:                   false,
		NetworkCall:                    false,
		SecretValuesRead:               false,
		TransportCalled:                false,
		WorkspaceModified:              false,
		BlockedReason:                  ProviderCallExecutorBlockedReason,
		ReviewChecklist:                append([]string(nil), providerActivationOperatorReviewChecklist...),
		ActivationPolicyPlanSHA256:     chain.activationPolicyPlanSHA256,
		ActivationApprovalSHA256:       chain.activationApprovalSHA256,
		ActivationReleasePackageSHA256: sha256Hex(pkgData),
		RealCallProposalSHA256:         chain.realCallProposalSHA256,
		CredentialPolicyPlanSHA256:     chain.credentialPolicyPlanSHA256,
		ProviderPayloadSHA256:          chain.providerPayloadSHA256,
		Failures:                       failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result
}

func LoadProviderActivationOperatorReviewBundle(path string) (ProviderActivationOperatorReviewBundleResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider activation operator review bundle", path)
	if err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, nil, err
	}
	var result ProviderActivationOperatorReviewBundleResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderActivationOperatorReviewBundleResult{}, nil, fmt.Errorf("parse provider activation operator review bundle json: %w", err)
	}
	return result, data, nil
}

func writeProviderActivationOperatorReviewBundleJSON(path string, result ProviderActivationOperatorReviewBundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation operator review bundle output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation operator review bundle json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation operator review bundle must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderActivationOperatorReviewBundleJSON(result ProviderActivationOperatorReviewBundleResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderActivationOperatorReviewBundleText(result ProviderActivationOperatorReviewBundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_operator_review_bundle:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  operator_review_bundle_ready: %t\n  operator_review_required: %t\n  operator_approved_now: %t\n  activation_allowed_now: %t\n  blocked_reason: %s\n",
		result.Status, result.OperatorReviewBundleReady, result.OperatorReviewRequired, result.OperatorApprovedNow, result.ActivationAllowedNow, result.BlockedReason)
	return err
}

func WriteProviderActivationOperatorReviewReportJSON(result ProviderActivationOperatorReviewBundleResult, out io.Writer) error {
	return WriteProviderActivationOperatorReviewBundleJSON(result, out)
}

func WriteProviderActivationOperatorReviewReportText(result ProviderActivationOperatorReviewBundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_operator_review_report:"); err != nil {
		return err
	}
	return WriteProviderActivationOperatorReviewBundleText(result, out)
}
