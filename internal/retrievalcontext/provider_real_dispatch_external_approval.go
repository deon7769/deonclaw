package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const AllowedUseProviderRealDispatchExternalApprovalOnly = "provider_real_dispatch_external_approval_only"

type ProviderRealDispatchExternalApprovalRequest struct {
	Status                                string    `json:"status"`
	CreatedAt                             time.Time `json:"created_at"`
	DesignReviewPackageSHA256             string    `json:"design_review_package_sha256"`
	DesignReviewGateSHA256                string    `json:"design_review_gate_sha256"`
	SecretReadProposalSHA256              string    `json:"secret_read_proposal_sha256"`
	RealDispatchDesignSHA256              string    `json:"real_dispatch_design_sha256"`
	RealTransportImplementationPlanSHA256 string    `json:"real_transport_implementation_plan_sha256"`
	ActivationFinalAuditSHA256            string    `json:"activation_final_audit_sha256"`
	ActivationCIReportSHA256              string    `json:"activation_ci_report_sha256"`
	KillSwitchPlanSHA256                  string    `json:"kill_switch_plan_sha256"`
	OperatorReviewBundleSHA256            string    `json:"operator_review_bundle_sha256"`
	ProviderPayloadSHA256                 string    `json:"provider_payload_sha256,omitempty"`
	RequestedExternalApprovalAuthorized   bool      `json:"requested_external_approval_authorized"`
	ExternalApprovalAllowedNow            bool      `json:"external_approval_allowed_now"`
	RealDispatchAllowedNow                bool      `json:"real_dispatch_allowed_now"`
	ExecuteSubcommandRegistered           bool      `json:"execute_subcommand_registered"`
	ProviderCall                          bool      `json:"provider_call"`
	NetworkCall                           bool      `json:"network_call"`
	SecretValuesRead                      bool      `json:"secret_values_read"`
	TransportCalled                       bool      `json:"transport_called"`
	SentToProvider                        bool      `json:"sent_to_provider"`
	WorkspaceModified                     bool      `json:"workspace_modified"`
	BlockedReason                         string    `json:"blocked_reason"`
	Warnings                              []string  `json:"warnings,omitempty"`
}

type ProviderRealDispatchExternalApproval struct {
	Approved                            bool      `json:"approved"`
	ApprovedAt                          time.Time `json:"approved_at"`
	RequestSHA256                       string    `json:"request_sha256"`
	DesignReviewPackageSHA256           string    `json:"design_review_package_sha256"`
	DesignReviewGateSHA256              string    `json:"design_review_gate_sha256"`
	SecretReadProposalSHA256            string    `json:"secret_read_proposal_sha256"`
	RealDispatchDesignSHA256            string    `json:"real_dispatch_design_sha256"`
	ProviderPayloadSHA256               string    `json:"provider_payload_sha256,omitempty"`
	AllowedUse                          string    `json:"allowed_use"`
	ExternalApprovalAuthorizedForFuture bool      `json:"external_approval_authorized_for_future"`
	ExternalApprovalAllowedNow          bool      `json:"external_approval_allowed_now"`
	RealDispatchAllowedNow              bool      `json:"real_dispatch_allowed_now"`
	ExecuteSubcommandRegistered         bool      `json:"execute_subcommand_registered"`
	ProviderCall                        bool      `json:"provider_call"`
	NetworkCall                         bool      `json:"network_call"`
	SecretValuesRead                    bool      `json:"secret_values_read"`
	TransportCalled                     bool      `json:"transport_called"`
	SentToProvider                      bool      `json:"sent_to_provider"`
	WorkspaceModified                   bool      `json:"workspace_modified"`
	BlockedReason                       string    `json:"blocked_reason"`
	ConfirmDesignReviewPackageSHA256    bool      `json:"confirm_design_review_package_sha256"`
	ConfirmDesignReviewGateSHA256       bool      `json:"confirm_design_review_gate_sha256"`
	ConfirmSecretReadProposalSHA256     bool      `json:"confirm_secret_read_proposal_sha256"`
	ConfirmRealDispatchDesignSHA256     bool      `json:"confirm_real_dispatch_design_sha256"`
	ConfirmProviderPayloadSHA256        bool      `json:"confirm_provider_payload_sha256"`
	Warnings                            []string  `json:"warnings,omitempty"`
}

type NewProviderRealDispatchExternalApprovalRequestOptions struct {
	DesignReviewPackagePath             string
	DesignReviewGatePath                string
	SecretReadProposalPath              string
	RealDispatchDesignPath              string
	RealTransportImplementationPlanPath string
	ActivationFinalAuditPath            string
	ActivationCIReportPath              string
	KillSwitchPlanPath                  string
	OperatorReviewBundlePath            string
	OutputPath                          string
}

type ApproveProviderRealDispatchExternalApprovalOptions struct {
	RequestPath                      string
	OutputPath                       string
	ConfirmDesignReviewPackageSHA256 string
	ConfirmDesignReviewGateSHA256    string
	ConfirmSecretReadProposalSHA256  string
	ConfirmRealDispatchDesignSHA256  string
	ConfirmProviderPayloadSHA256     string
	ApprovedAt                       time.Time
}

type InspectProviderRealDispatchExternalApprovalOptions struct {
	RequestPath          string
	DesignReviewGatePath string
}

type InspectProviderRealDispatchExternalApprovalResult struct {
	Status                              string   `json:"status"`
	Approved                            bool     `json:"approved"`
	ExternalApprovalAuthorizedForFuture bool     `json:"external_approval_authorized_for_future"`
	ExternalApprovalAllowedNow          bool     `json:"external_approval_allowed_now"`
	RealDispatchAllowedNow              bool     `json:"real_dispatch_allowed_now"`
	ExecuteSubcommandRegistered         bool     `json:"execute_subcommand_registered"`
	ProviderCall                        bool     `json:"provider_call"`
	NetworkCall                         bool     `json:"network_call"`
	SecretValuesRead                    bool     `json:"secret_values_read"`
	TransportCalled                     bool     `json:"transport_called"`
	SentToProvider                      bool     `json:"sent_to_provider"`
	WorkspaceModified                   bool     `json:"workspace_modified"`
	BlockedReason                       string   `json:"blocked_reason"`
	ProviderPayloadSHA256               string   `json:"provider_payload_sha256,omitempty"`
	Warnings                            []string `json:"warnings"`
	Failures                            []string `json:"failures,omitempty"`
}

type providerRealDispatchExternalApprovalChain struct {
	designReviewPackageSHA256             string
	designReviewGateSHA256                string
	secretReadProposalSHA256              string
	realDispatchDesignSHA256              string
	realTransportImplementationPlanSHA256 string
	activationFinalAuditSHA256            string
	activationCIReportSHA256              string
	killSwitchPlanSHA256                  string
	operatorReviewBundleSHA256            string
	providerPayloadSHA256                 string
	killSwitchActive                      bool
	operatorReviewRequired                bool
	designGateReady                       bool
}

func NewProviderRealDispatchExternalApprovalRequest(opts NewProviderRealDispatchExternalApprovalRequestOptions) (ProviderRealDispatchExternalApprovalRequest, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"design review package path", opts.DesignReviewPackagePath},
		{"design review gate path", opts.DesignReviewGatePath},
		{"secret read proposal path", opts.SecretReadProposalPath},
		{"real dispatch design path", opts.RealDispatchDesignPath},
		{"real transport implementation plan path", opts.RealTransportImplementationPlanPath},
		{"activation final audit path", opts.ActivationFinalAuditPath},
		{"activation ci report path", opts.ActivationCIReportPath},
		{"kill switch plan path", opts.KillSwitchPlanPath},
		{"operator review bundle path", opts.OperatorReviewBundlePath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderRealDispatchExternalApprovalRequest{}, err
		}
	}

	chain, failures, err := loadProviderRealDispatchExternalApprovalChain(
		opts.DesignReviewPackagePath,
		opts.DesignReviewGatePath,
		opts.SecretReadProposalPath,
		opts.RealDispatchDesignPath,
		opts.RealTransportImplementationPlanPath,
		opts.ActivationFinalAuditPath,
		opts.ActivationCIReportPath,
		opts.KillSwitchPlanPath,
		opts.OperatorReviewBundlePath,
	)
	if err != nil {
		return ProviderRealDispatchExternalApprovalRequest{}, err
	}
	if len(failures) > 0 {
		return ProviderRealDispatchExternalApprovalRequest{}, fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	request := ProviderRealDispatchExternalApprovalRequest{
		Status:                                RequestStatusPending,
		CreatedAt:                             time.Now().UTC(),
		DesignReviewPackageSHA256:             chain.designReviewPackageSHA256,
		DesignReviewGateSHA256:                chain.designReviewGateSHA256,
		SecretReadProposalSHA256:              chain.secretReadProposalSHA256,
		RealDispatchDesignSHA256:              chain.realDispatchDesignSHA256,
		RealTransportImplementationPlanSHA256: chain.realTransportImplementationPlanSHA256,
		ActivationFinalAuditSHA256:            chain.activationFinalAuditSHA256,
		ActivationCIReportSHA256:              chain.activationCIReportSHA256,
		KillSwitchPlanSHA256:                  chain.killSwitchPlanSHA256,
		OperatorReviewBundleSHA256:            chain.operatorReviewBundleSHA256,
		ProviderPayloadSHA256:                 chain.providerPayloadSHA256,
		RequestedExternalApprovalAuthorized:   true,
		ExternalApprovalAllowedNow:            false,
		RealDispatchAllowedNow:                false,
		ExecuteSubcommandRegistered:           false,
		ProviderCall:                          false,
		NetworkCall:                           false,
		SecretValuesRead:                      false,
		TransportCalled:                       false,
		SentToProvider:                        false,
		WorkspaceModified:                     false,
		BlockedReason:                         ProviderCallExecutorBlockedReason,
	}
	if err := request.Validate(); err != nil {
		return ProviderRealDispatchExternalApprovalRequest{}, err
	}
	if err := writeProviderRealDispatchExternalApprovalRequestJSON(opts.OutputPath, request); err != nil {
		return ProviderRealDispatchExternalApprovalRequest{}, err
	}
	return request, nil
}

func ApproveProviderRealDispatchExternalApproval(opts ApproveProviderRealDispatchExternalApprovalOptions) (ProviderRealDispatchExternalApproval, error) {
	for flag, value := range map[string]string{
		"--confirm-design-review-package-sha256": opts.ConfirmDesignReviewPackageSHA256,
		"--confirm-design-review-gate-sha256":    opts.ConfirmDesignReviewGateSHA256,
		"--confirm-secret-read-proposal-sha256":  opts.ConfirmSecretReadProposalSHA256,
		"--confirm-real-dispatch-design-sha256":  opts.ConfirmRealDispatchDesignSHA256,
		"--confirm-provider-payload-sha256":      opts.ConfirmProviderPayloadSHA256,
	} {
		if strings.TrimSpace(value) == "" {
			return ProviderRealDispatchExternalApproval{}, fmt.Errorf("%s is required", flag)
		}
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}

	requestData, err := readArtifactBytesNoTextExcerpt("external approval request", opts.RequestPath)
	if err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}
	request, err := ParseProviderRealDispatchExternalApprovalRequestJSON(requestData)
	if err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}
	if err := request.Validate(); err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}
	if request.Status != RequestStatusPending {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("external approval request status %q must be pending", request.Status)
	}
	if request.DesignReviewPackageSHA256 != opts.ConfirmDesignReviewPackageSHA256 {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("confirm design review package sha256 mismatch")
	}
	if request.DesignReviewGateSHA256 != opts.ConfirmDesignReviewGateSHA256 {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("confirm design review gate sha256 mismatch")
	}
	if request.SecretReadProposalSHA256 != opts.ConfirmSecretReadProposalSHA256 {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("confirm secret read proposal sha256 mismatch")
	}
	if request.RealDispatchDesignSHA256 != opts.ConfirmRealDispatchDesignSHA256 {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("confirm real dispatch design sha256 mismatch")
	}
	if request.ProviderPayloadSHA256 != opts.ConfirmProviderPayloadSHA256 {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("confirm provider payload sha256 mismatch")
	}

	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}

	approval := ProviderRealDispatchExternalApproval{
		Approved:                            true,
		ApprovedAt:                          approvedAt.UTC(),
		RequestSHA256:                       sha256Hex(requestData),
		DesignReviewPackageSHA256:           request.DesignReviewPackageSHA256,
		DesignReviewGateSHA256:              request.DesignReviewGateSHA256,
		SecretReadProposalSHA256:            request.SecretReadProposalSHA256,
		RealDispatchDesignSHA256:            request.RealDispatchDesignSHA256,
		ProviderPayloadSHA256:               request.ProviderPayloadSHA256,
		AllowedUse:                          AllowedUseProviderRealDispatchExternalApprovalOnly,
		ExternalApprovalAuthorizedForFuture: true,
		ExternalApprovalAllowedNow:          false,
		RealDispatchAllowedNow:              false,
		ExecuteSubcommandRegistered:         false,
		ProviderCall:                        false,
		NetworkCall:                         false,
		SecretValuesRead:                    false,
		TransportCalled:                     false,
		SentToProvider:                      false,
		WorkspaceModified:                   false,
		BlockedReason:                       ProviderCallExecutorBlockedReason,
		ConfirmDesignReviewPackageSHA256:    true,
		ConfirmDesignReviewGateSHA256:       true,
		ConfirmSecretReadProposalSHA256:     true,
		ConfirmRealDispatchDesignSHA256:     true,
		ConfirmProviderPayloadSHA256:        true,
	}
	if err := approval.Validate(); err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}
	if err := writeProviderRealDispatchExternalApprovalJSON(opts.OutputPath, approval); err != nil {
		return ProviderRealDispatchExternalApproval{}, err
	}
	return approval, nil
}

func InspectProviderRealDispatchExternalApproval(path string, opts InspectProviderRealDispatchExternalApprovalOptions) (InspectProviderRealDispatchExternalApprovalResult, error) {
	if err := validateRelativeSafePath("external approval path", path); err != nil {
		return InspectProviderRealDispatchExternalApprovalResult{}, err
	}
	data, err := readArtifactBytesNoTextExcerpt("external approval", path)
	if err != nil {
		return InspectProviderRealDispatchExternalApprovalResult{}, err
	}
	approval, err := ParseProviderRealDispatchExternalApprovalJSON(data)
	if err != nil {
		return InspectProviderRealDispatchExternalApprovalResult{}, err
	}

	result := InspectProviderRealDispatchExternalApprovalResult{
		Status:                              lancedbpolicy.StatusOK,
		Approved:                            approval.Approved,
		ExternalApprovalAuthorizedForFuture: approval.ExternalApprovalAuthorizedForFuture,
		ExternalApprovalAllowedNow:          approval.ExternalApprovalAllowedNow,
		RealDispatchAllowedNow:              approval.RealDispatchAllowedNow,
		ExecuteSubcommandRegistered:         approval.ExecuteSubcommandRegistered,
		ProviderCall:                        approval.ProviderCall,
		NetworkCall:                         approval.NetworkCall,
		SecretValuesRead:                    approval.SecretValuesRead,
		TransportCalled:                     approval.TransportCalled,
		SentToProvider:                      approval.SentToProvider,
		WorkspaceModified:                   approval.WorkspaceModified,
		BlockedReason:                       approval.BlockedReason,
		ProviderPayloadSHA256:               approval.ProviderPayloadSHA256,
		Warnings:                            append([]string(nil), approval.Warnings...),
	}

	var failures []string
	if err := approval.Validate(); err != nil {
		failures = append(failures, err.Error())
	}
	if !approval.ExternalApprovalAuthorizedForFuture || approval.ExternalApprovalAllowedNow || approval.RealDispatchAllowedNow {
		failures = append(failures, "external approval must authorize future only")
	}
	if approval.ExecuteSubcommandRegistered || approval.ProviderCall || approval.NetworkCall || approval.SecretValuesRead || approval.TransportCalled || approval.SentToProvider || approval.WorkspaceModified {
		failures = append(failures, "external approval execution flags must stay blocked")
	}

	if opts.RequestPath != "" {
		requestData, err := readArtifactBytesNoTextExcerpt("external approval request", opts.RequestPath)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			request, err := ParseProviderRealDispatchExternalApprovalRequestJSON(requestData)
			if err != nil {
				failures = append(failures, err.Error())
			} else {
				requestSHA := sha256Hex(requestData)
				if approval.RequestSHA256 != requestSHA {
					failures = append(failures, "request_sha256 mismatch")
				}
				if approval.DesignReviewPackageSHA256 != request.DesignReviewPackageSHA256 {
					failures = append(failures, "design_review_package_sha256 mismatch with request")
				}
				if approval.DesignReviewGateSHA256 != request.DesignReviewGateSHA256 {
					failures = append(failures, "design_review_gate_sha256 mismatch with request")
				}
			}
		}
	}

	if opts.DesignReviewGatePath != "" {
		gateData, err := readArtifactBytesNoTextExcerpt("design review gate", opts.DesignReviewGatePath)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			gateSHA := sha256Hex(gateData)
			if approval.DesignReviewGateSHA256 != gateSHA {
				failures = append(failures, "design_review_gate_sha256 mismatch with gate artifact")
			}
			gate, err := ParseProviderRealActivationDesignReviewGateJSON(gateData)
			if err != nil {
				failures = append(failures, err.Error())
			} else if !gate.RealActivationDesignGateReady {
				failures = append(failures, "design review gate must be ready")
			}
		}
	}

	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = failures
	}
	return result, nil
}

func loadProviderRealDispatchExternalApprovalChain(
	designReviewPackagePath, designReviewGatePath, secretReadProposalPath, realDispatchDesignPath,
	realTransportImplementationPlanPath, activationFinalAuditPath, activationCIReportPath,
	killSwitchPlanPath, operatorReviewBundlePath string,
) (providerRealDispatchExternalApprovalChain, []string, error) {
	chain := providerRealDispatchExternalApprovalChain{killSwitchActive: true, operatorReviewRequired: true, designGateReady: true}
	var failures []string

	pkg, pkgData, err := LoadProviderRealActivationDesignReviewPackage(designReviewPackagePath)
	if err != nil {
		return chain, nil, err
	}
	chain.designReviewPackageSHA256 = sha256Hex(pkgData)
	if !pkg.RealActivationDesignReviewReady {
		failures = append(failures, "design review package must be ready")
	}
	if pkg.ProviderPayloadSHA256 != "" {
		chain.providerPayloadSHA256 = pkg.ProviderPayloadSHA256
	}

	proposal, proposalData, err := LoadProviderSecretReadProposal(secretReadProposalPath)
	if err != nil {
		return chain, nil, err
	}
	chain.secretReadProposalSHA256 = sha256Hex(proposalData)
	reconcileProviderExecutorHash("external approval chain", pkg.SecretReadProposalSHA256, chain.secretReadProposalSHA256, &failures)

	transportPlan, transportData, err := LoadProviderRealTransportImplementationPlan(realTransportImplementationPlanPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realTransportImplementationPlanSHA256 = sha256Hex(transportData)
	reconcileProviderExecutorHash("external approval chain", pkg.RealTransportImplementationPlanSHA256, chain.realTransportImplementationPlanSHA256, &failures)

	dispatchDesign, dispatchData, err := LoadProviderRealDispatchDesign(realDispatchDesignPath)
	if err != nil {
		return chain, nil, err
	}
	chain.realDispatchDesignSHA256 = sha256Hex(dispatchData)
	reconcileProviderExecutorHash("external approval chain", pkg.RealDispatchDesignSHA256, chain.realDispatchDesignSHA256, &failures)
	if !dispatchDesign.RealDispatchDesignReady || dispatchDesign.ExecuteSubcommandRegistered {
		failures = append(failures, "real dispatch design must be ready without execute subcommand")
	}
	if !transportPlan.RealTransportImplementationPlanReady || transportPlan.TransportCalled {
		failures = append(failures, "real transport implementation plan must be ready and blocked")
	}
	if !proposal.SecretReadProposalReady || proposal.SecretValuesRead {
		failures = append(failures, "secret read proposal must be ready without secret reads")
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
		chain.designGateReady = false
	}
	if gate.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 == "" {
		chain.providerPayloadSHA256 = gate.ProviderPayloadSHA256
	}
	if !gate.KillSwitchActive {
		chain.killSwitchActive = false
		failures = append(failures, "design review gate kill_switch_active must be true")
	}
	if !gate.OperatorReviewRequired {
		chain.operatorReviewRequired = false
		failures = append(failures, "design review gate operator_review_required must be true")
	}
	if gate.RealDispatchSupportedNow {
		failures = append(failures, "design review gate must keep real dispatch blocked")
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
		chain.killSwitchActive = false
	}

	operatorBundle, operatorData, err := LoadProviderActivationOperatorReviewBundle(operatorReviewBundlePath)
	if err != nil {
		return chain, nil, err
	}
	chain.operatorReviewBundleSHA256 = sha256Hex(operatorData)
	if !operatorBundle.OperatorReviewBundleReady || !operatorBundle.OperatorReviewRequired {
		failures = append(failures, "operator review bundle must be ready and required")
		chain.operatorReviewRequired = false
	}

	return chain, failures, nil
}

func (r ProviderRealDispatchExternalApprovalRequest) Validate() error {
	if r.Status != RequestStatusPending {
		return fmt.Errorf("status %q must be pending", r.Status)
	}
	if !r.RequestedExternalApprovalAuthorized {
		return fmt.Errorf("requested_external_approval_authorized must be true")
	}
	if r.ExternalApprovalAllowedNow || r.RealDispatchAllowedNow || r.ExecuteSubcommandRegistered {
		return fmt.Errorf("external approval request must keep execution blocked")
	}
	if r.ProviderCall || r.NetworkCall || r.SecretValuesRead || r.TransportCalled || r.SentToProvider || r.WorkspaceModified {
		return fmt.Errorf("external approval request execution flags must stay false")
	}
	if r.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("blocked_reason %q must be %q", r.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	return nil
}

func (a ProviderRealDispatchExternalApproval) Validate() error {
	if !a.Approved {
		return fmt.Errorf("approved must be true")
	}
	if a.AllowedUse != AllowedUseProviderRealDispatchExternalApprovalOnly {
		return fmt.Errorf("allowed_use %q must be %q", a.AllowedUse, AllowedUseProviderRealDispatchExternalApprovalOnly)
	}
	if !a.ExternalApprovalAuthorizedForFuture || a.ExternalApprovalAllowedNow || a.RealDispatchAllowedNow {
		return fmt.Errorf("external approval must authorize future only")
	}
	if a.ExecuteSubcommandRegistered || a.ProviderCall || a.NetworkCall || a.SecretValuesRead || a.TransportCalled || a.SentToProvider || a.WorkspaceModified {
		return fmt.Errorf("external approval execution flags must stay false")
	}
	if !a.ConfirmDesignReviewPackageSHA256 || !a.ConfirmDesignReviewGateSHA256 || !a.ConfirmSecretReadProposalSHA256 || !a.ConfirmRealDispatchDesignSHA256 || !a.ConfirmProviderPayloadSHA256 {
		return fmt.Errorf("all confirm flags must be true")
	}
	if a.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("blocked_reason %q must be %q", a.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	return nil
}

func ParseProviderRealDispatchExternalApprovalRequestJSON(data []byte) (ProviderRealDispatchExternalApprovalRequest, error) {
	var request ProviderRealDispatchExternalApprovalRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return ProviderRealDispatchExternalApprovalRequest{}, fmt.Errorf("parse external approval request json: %w", err)
	}
	return request, nil
}

func ParseProviderRealDispatchExternalApprovalJSON(data []byte) (ProviderRealDispatchExternalApproval, error) {
	var approval ProviderRealDispatchExternalApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return ProviderRealDispatchExternalApproval{}, fmt.Errorf("parse external approval json: %w", err)
	}
	return approval, nil
}

func LoadProviderRealDispatchExternalApproval(path string) (ProviderRealDispatchExternalApproval, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("external approval", path)
	if err != nil {
		return ProviderRealDispatchExternalApproval{}, nil, err
	}
	approval, err := ParseProviderRealDispatchExternalApprovalJSON(data)
	if err != nil {
		return ProviderRealDispatchExternalApproval{}, nil, err
	}
	return approval, data, nil
}

func writeProviderRealDispatchExternalApprovalRequestJSON(path string, request ProviderRealDispatchExternalApprovalRequest) error {
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeProviderRealDispatchExternalApprovalJSON(path string, approval ProviderRealDispatchExternalApproval) error {
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func WriteProviderRealDispatchExternalApprovalInspectJSON(result InspectProviderRealDispatchExternalApprovalResult, out io.Writer) error {
	return json.NewEncoder(out).Encode(result)
}

func WriteProviderRealDispatchExternalApprovalInspectText(result InspectProviderRealDispatchExternalApprovalResult, out io.Writer) error {
	_, err := fmt.Fprintf(out, "status: %s\napproved: %t\nexternal_approval_authorized_for_future: %t\nexternal_approval_allowed_now: %t\nreal_dispatch_allowed_now: %t\nexecute_subcommand_registered: %t\nblocked_reason: %s\n",
		result.Status, result.Approved, result.ExternalApprovalAuthorizedForFuture, result.ExternalApprovalAllowedNow, result.RealDispatchAllowedNow, result.ExecuteSubcommandRegistered, result.BlockedReason)
	return err
}

func WriteProviderRealDispatchExternalApprovalNewText(request ProviderRealDispatchExternalApprovalRequest, out io.Writer) error {
	_, err := fmt.Fprintf(out, "status: %s\nrequested_external_approval_authorized: %t\nexternal_approval_allowed_now: %t\nblocked_reason: %s\n",
		request.Status, request.RequestedExternalApprovalAuthorized, request.ExternalApprovalAllowedNow, request.BlockedReason)
	return err
}

func WriteProviderRealDispatchExternalApprovalApproveText(approval ProviderRealDispatchExternalApproval, out io.Writer) error {
	_, err := fmt.Fprintf(out, "approved: %t\nexternal_approval_authorized_for_future: %t\nexternal_approval_allowed_now: %t\nreal_dispatch_allowed_now: %t\nblocked_reason: %s\n",
		approval.Approved, approval.ExternalApprovalAuthorizedForFuture, approval.ExternalApprovalAllowedNow, approval.RealDispatchAllowedNow, approval.BlockedReason)
	return err
}
