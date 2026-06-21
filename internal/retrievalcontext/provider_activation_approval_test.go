package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type providerActivationControlPrerequisites struct {
	providerActivationReadinessArtifacts
	activationPolicyConfigPath   string
	activationPolicyPlanPath     string
	activationReadinessAuditPath string
	changeProposalReportPath     string
}

func runProviderActivationControlPlanePrerequisites(t *testing.T, chain providerCallChainFixture) providerActivationControlPrerequisites {
	t.Helper()
	readiness := runProviderActivationReadinessChain(t, chain)

	const activationReadinessAuditPath = "provider-activation-readiness-audit.json"
	audit, err := retrievalcontext.ProviderActivationReadinessAudit(retrievalcontext.ProviderActivationReadinessAuditOptions{
		CredentialPolicyPlanPath: readiness.credentialPolicyPlanPath,
		RealCallProposalPath:     readiness.realCallProposalPath,
		ChangeProposalPath:       readiness.changeProposalPath,
		SimulationReportPath:     readiness.simulationReportPath,
		ReleaseGatePath:          readiness.releaseGatePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReadinessAudit() error = %v", err)
	}
	assertProviderActivationReadinessBlocked(t, audit)
	var auditBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderActivationReadinessAuditJSON(audit, &auditBuf); err != nil {
		t.Fatalf("WriteProviderActivationReadinessAuditJSON() error = %v", err)
	}
	if err := os.WriteFile(activationReadinessAuditPath, auditBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(activation readiness audit) error = %v", err)
	}
	assertNoTextExcerpt(t, activationReadinessAuditPath)

	const changeProposalReportPath = "provider-response-change-proposal-report.json"
	changeReport, err := retrievalcontext.ProviderResponseChangeProposalReport(retrievalcontext.ProviderResponseChangeProposalReportOptions{
		ChangeProposalPath:   readiness.changeProposalPath,
		ResponseFixturePath:  readiness.responseFixturePath,
		SimulationReportPath: readiness.simulationReportPath,
		RealCallProposalPath: readiness.realCallProposalPath,
	})
	if err != nil {
		t.Fatalf("ProviderResponseChangeProposalReport() error = %v", err)
	}
	assertProviderResponseChangeProposalBlocked(t, changeReport)
	var changeReportBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderResponseChangeProposalReportJSON(changeReport, &changeReportBuf); err != nil {
		t.Fatalf("WriteProviderResponseChangeProposalReportJSON() error = %v", err)
	}
	if err := os.WriteFile(changeProposalReportPath, changeReportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(change proposal report) error = %v", err)
	}
	assertNoTextExcerpt(t, changeProposalReportPath)

	const activationPolicyConfigPath = "provider-activation-policy.yaml"
	if err := os.WriteFile(activationPolicyConfigPath, []byte(validProviderActivationPolicyYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(activation policy config) error = %v", err)
	}

	const activationPolicyPlanPath = "provider-activation-policy-plan.json"
	plan, err := retrievalcontext.ProviderActivationPolicyPlan(retrievalcontext.ProviderActivationPolicyPlanOptions{
		ConfigPath: activationPolicyConfigPath, OutputPath: activationPolicyPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationPolicyPlan() error = %v", err)
	}
	assertProviderActivationPolicyPlanBlocked(t, plan)
	assertNoTextExcerpt(t, activationPolicyPlanPath)

	return providerActivationControlPrerequisites{
		providerActivationReadinessArtifacts: readiness,
		activationPolicyConfigPath:           activationPolicyConfigPath,
		activationPolicyPlanPath:             activationPolicyPlanPath,
		activationReadinessAuditPath:         activationReadinessAuditPath,
		changeProposalReportPath:             changeProposalReportPath,
	}
}

func TestProviderActivationApprovalOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	prereqs := runProviderActivationControlPlanePrerequisites(t, chain)

	const approvalRequestPath = "provider-activation-approval-request.json"
	request, err := retrievalcontext.NewProviderActivationApprovalRequest(retrievalcontext.NewProviderActivationApprovalRequestOptions{
		ActivationPolicyPlanPath:      prereqs.activationPolicyPlanPath,
		ActivationReadinessAuditPath:  prereqs.activationReadinessAuditPath,
		RealCallProposalPath:          prereqs.realCallProposalPath,
		CredentialPolicyPlanPath:      prereqs.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: prereqs.simulationReportPath,
		OutputPath:                    approvalRequestPath,
	})
	if err != nil {
		t.Fatalf("NewProviderActivationApprovalRequest() error = %v", err)
	}
	if request.Status != lancedbpolicy.StatusOK || !request.RequestedActivationAuthorized {
		t.Fatalf("request = %#v, want ok authorized request", request)
	}
	assertNoTextExcerpt(t, approvalRequestPath)

	const approvalPath = "provider-activation-approval.json"
	approval, err := retrievalcontext.ApproveProviderActivation(retrievalcontext.ApproveProviderActivationOptions{
		RequestPath:                   approvalRequestPath,
		OutputPath:                    approvalPath,
		ConfirmActivationPolicySHA256: request.ActivationPolicyPlanSHA256,
		ConfirmReadinessAuditSHA256:   request.ReadinessAuditSHA256,
		ConfirmRealCallProposalSHA256: request.RealCallProposalSHA256,
		ConfirmProviderPayloadSHA256:  request.ProviderPayloadSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderActivation() error = %v", err)
	}
	assertProviderActivationApprovalBlocked(t, approval)
	assertNoTextExcerpt(t, approvalPath)

	inspect, err := retrievalcontext.InspectProviderActivationApproval(approvalPath, retrievalcontext.InspectProviderActivationApprovalOptions{
		RequestPath: approvalRequestPath,
	})
	if err != nil {
		t.Fatalf("InspectProviderActivationApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect = %#v, want ok", inspect)
	}
}

func TestApproveProviderActivationFailsWithoutEachConfirmHash(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	prereqs := runProviderActivationControlPlanePrerequisites(t, chain)

	const approvalRequestPath = "provider-activation-approval-request.json"
	request, err := retrievalcontext.NewProviderActivationApprovalRequest(retrievalcontext.NewProviderActivationApprovalRequestOptions{
		ActivationPolicyPlanPath:      prereqs.activationPolicyPlanPath,
		ActivationReadinessAuditPath:  prereqs.activationReadinessAuditPath,
		RealCallProposalPath:          prereqs.realCallProposalPath,
		CredentialPolicyPlanPath:      prereqs.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: prereqs.simulationReportPath,
		OutputPath:                    approvalRequestPath,
	})
	if err != nil {
		t.Fatalf("NewProviderActivationApprovalRequest() error = %v", err)
	}

	tests := []struct {
		name string
		opts retrievalcontext.ApproveProviderActivationOptions
		want string
	}{
		{
			name: "missing activation policy sha256",
			opts: retrievalcontext.ApproveProviderActivationOptions{
				RequestPath:                   approvalRequestPath,
				OutputPath:                    "bad-approval.json",
				ConfirmReadinessAuditSHA256:   request.ReadinessAuditSHA256,
				ConfirmRealCallProposalSHA256: request.RealCallProposalSHA256,
				ConfirmProviderPayloadSHA256:  request.ProviderPayloadSHA256,
			},
			want: "--confirm-activation-policy-sha256 is required",
		},
		{
			name: "missing readiness audit sha256",
			opts: retrievalcontext.ApproveProviderActivationOptions{
				RequestPath:                   approvalRequestPath,
				OutputPath:                    "bad-approval.json",
				ConfirmActivationPolicySHA256: request.ActivationPolicyPlanSHA256,
				ConfirmRealCallProposalSHA256: request.RealCallProposalSHA256,
				ConfirmProviderPayloadSHA256:  request.ProviderPayloadSHA256,
			},
			want: "--confirm-readiness-audit-sha256 is required",
		},
		{
			name: "missing real call proposal sha256",
			opts: retrievalcontext.ApproveProviderActivationOptions{
				RequestPath:                   approvalRequestPath,
				OutputPath:                    "bad-approval.json",
				ConfirmActivationPolicySHA256: request.ActivationPolicyPlanSHA256,
				ConfirmReadinessAuditSHA256:   request.ReadinessAuditSHA256,
				ConfirmProviderPayloadSHA256:  request.ProviderPayloadSHA256,
			},
			want: "--confirm-real-call-proposal-sha256 is required",
		},
		{
			name: "missing provider payload sha256",
			opts: retrievalcontext.ApproveProviderActivationOptions{
				RequestPath:                   approvalRequestPath,
				OutputPath:                    "bad-approval.json",
				ConfirmActivationPolicySHA256: request.ActivationPolicyPlanSHA256,
				ConfirmReadinessAuditSHA256:   request.ReadinessAuditSHA256,
				ConfirmRealCallProposalSHA256: request.RealCallProposalSHA256,
			},
			want: "--confirm-provider-payload-sha256 is required",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := retrievalcontext.ApproveProviderActivation(tc.opts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestInspectProviderActivationApprovalDetectsHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	prereqs := runProviderActivationControlPlanePrerequisites(t, chain)

	const approvalRequestPath = "provider-activation-approval-request.json"
	request, err := retrievalcontext.NewProviderActivationApprovalRequest(retrievalcontext.NewProviderActivationApprovalRequestOptions{
		ActivationPolicyPlanPath:      prereqs.activationPolicyPlanPath,
		ActivationReadinessAuditPath:  prereqs.activationReadinessAuditPath,
		RealCallProposalPath:          prereqs.realCallProposalPath,
		CredentialPolicyPlanPath:      prereqs.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: prereqs.simulationReportPath,
		OutputPath:                    approvalRequestPath,
	})
	if err != nil {
		t.Fatalf("NewProviderActivationApprovalRequest() error = %v", err)
	}

	const approvalPath = "provider-activation-approval.json"
	if _, err := retrievalcontext.ApproveProviderActivation(retrievalcontext.ApproveProviderActivationOptions{
		RequestPath:                   approvalRequestPath,
		OutputPath:                    approvalPath,
		ConfirmActivationPolicySHA256: request.ActivationPolicyPlanSHA256,
		ConfirmReadinessAuditSHA256:   request.ReadinessAuditSHA256,
		ConfirmRealCallProposalSHA256: request.RealCallProposalSHA256,
		ConfirmProviderPayloadSHA256:  request.ProviderPayloadSHA256,
	}); err != nil {
		t.Fatalf("ApproveProviderActivation() error = %v", err)
	}

	approvalData, err := os.ReadFile(approvalPath)
	if err != nil {
		t.Fatalf("ReadFile(approval) error = %v", err)
	}
	patched := bytes.Replace(approvalData, []byte(request.ProviderPayloadSHA256), []byte("deadbeef"), 1)
	if err := os.WriteFile(approvalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile(approval) error = %v", err)
	}

	inspect, err := retrievalcontext.InspectProviderActivationApproval(approvalPath, retrievalcontext.InspectProviderActivationApprovalOptions{
		RequestPath: approvalRequestPath,
	})
	if err != nil {
		t.Fatalf("InspectProviderActivationApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("inspect status = %q, want failed", inspect.Status)
	}
}

func assertProviderActivationApprovalBlocked(t *testing.T, approval retrievalcontext.ProviderActivationApproval) {
	t.Helper()
	if !approval.Approved || !approval.ActivationAuthorizedForFuture {
		t.Fatalf("approval = %#v, want approved for future activation", approval)
	}
	if approval.ActivationAllowedNow || approval.ProviderCallAllowedNow || approval.ProviderCall || approval.NetworkCall || approval.TransportCalled || approval.SentToProvider || approval.SecretValuesRead || approval.WorkspaceModified {
		t.Fatalf("activation approval flags must stay blocked: %#v", approval)
	}
	if approval.AllowedUse != retrievalcontext.AllowedUseProviderActivationPolicyOnly {
		t.Fatalf("allowed_use = %q", approval.AllowedUse)
	}
}
