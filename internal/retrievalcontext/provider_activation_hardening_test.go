package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type providerActivationHardeningArtifacts struct {
	providerActivationControlArtifacts
	activationReleaseGatePath string
	operatorReviewBundlePath  string
	killSwitchConfigPath      string
	killSwitchPlanPath        string
}

func runProviderActivationHardeningChain(t *testing.T, chain providerCallChainFixture) providerActivationHardeningArtifacts {
	t.Helper()
	control := runProviderActivationControlPlaneChain(t, chain)

	gate, err := retrievalcontext.ProviderActivationReleaseGate(retrievalcontext.ProviderActivationReleaseGateOptions{
		ActivationPolicyPlanPath:         control.activationPolicyPlanPath,
		ActivationApprovalPath:           control.activationApprovalPath,
		ActivationRehearsalPath:          control.activationRehearsalPath,
		ActivationReadinessAuditPath:     control.activationReadinessAuditPath,
		RealCallProposalPath:             control.realCallProposalPath,
		ExecutionSimulationReportPath:    control.simulationReportPath,
		CredentialPolicyPlanPath:         control.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: control.changeProposalReportPath,
		ActivationReleasePackagePath:     control.activationReleasePackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReleaseGate() error = %v", err)
	}
	assertProviderActivationReleaseGateBlocked(t, gate)

	const activationReleaseGatePath = "provider-activation-release-gate.json"
	var gateBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderActivationReleaseGateJSON(gate, &gateBuf); err != nil {
		t.Fatalf("WriteProviderActivationReleaseGateJSON() error = %v", err)
	}
	if err := os.WriteFile(activationReleaseGatePath, gateBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(release gate) error = %v", err)
	}
	assertNoPreviewLeakInFile(t, activationReleaseGatePath)
	assertNoPreviewLeakInString(t, "release gate stdout", gateBuf.String())

	const operatorReviewBundlePath = "provider-activation-operator-review-bundle.json"
	bundle, err := retrievalcontext.ProviderActivationOperatorReviewBundle(retrievalcontext.ProviderActivationOperatorReviewBundleOptions{
		ActivationReleasePackagePath:     control.activationReleasePackagePath,
		ActivationReleaseGatePath:        activationReleaseGatePath,
		ActivationPolicyPlanPath:         control.activationPolicyPlanPath,
		ActivationApprovalPath:           control.activationApprovalPath,
		ActivationRehearsalPath:          control.activationRehearsalPath,
		ActivationReadinessAuditPath:     control.activationReadinessAuditPath,
		RealCallProposalPath:             control.realCallProposalPath,
		ExecutionSimulationReportPath:    control.simulationReportPath,
		CredentialPolicyPlanPath:         control.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: control.changeProposalReportPath,
		OutputPath:                       operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationOperatorReviewBundle() error = %v", err)
	}
	assertProviderActivationOperatorReviewBlocked(t, bundle)
	assertNoPreviewLeakInFile(t, operatorReviewBundlePath)

	report, err := retrievalcontext.ProviderActivationOperatorReviewReport(retrievalcontext.ProviderActivationOperatorReviewReportOptions{
		OperatorReviewBundlePath:         operatorReviewBundlePath,
		ActivationReleasePackagePath:     control.activationReleasePackagePath,
		ActivationReleaseGatePath:        activationReleaseGatePath,
		ActivationPolicyPlanPath:         control.activationPolicyPlanPath,
		ActivationApprovalPath:           control.activationApprovalPath,
		ActivationRehearsalPath:          control.activationRehearsalPath,
		ActivationReadinessAuditPath:     control.activationReadinessAuditPath,
		RealCallProposalPath:             control.realCallProposalPath,
		ExecutionSimulationReportPath:    control.simulationReportPath,
		CredentialPolicyPlanPath:         control.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: control.changeProposalReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationOperatorReviewReport() error = %v", err)
	}
	assertProviderActivationOperatorReviewBlocked(t, report)

	const killSwitchConfigPath = "provider-activation-kill-switch.yaml"
	if err := os.WriteFile(killSwitchConfigPath, []byte(validProviderActivationKillSwitchYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(kill switch config) error = %v", err)
	}
	const killSwitchPlanPath = "provider-activation-kill-switch-plan.json"
	killSwitchPlan, err := retrievalcontext.ProviderActivationKillSwitchPlan(retrievalcontext.ProviderActivationKillSwitchPlanOptions{
		ConfigPath: killSwitchConfigPath,
		OutputPath: killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationKillSwitchPlan() error = %v", err)
	}
	if !killSwitchPlan.KillSwitchPlanReady {
		t.Fatalf("kill switch plan = %#v", killSwitchPlan)
	}
	assertNoPreviewLeakInFile(t, killSwitchPlanPath)

	audit, err := retrievalcontext.ProviderActivationFinalAudit(retrievalcontext.ProviderActivationFinalAuditOptions{
		ActivationReleasePackagePath:     control.activationReleasePackagePath,
		ActivationReleaseGatePath:        activationReleaseGatePath,
		OperatorReviewBundlePath:         operatorReviewBundlePath,
		KillSwitchPlanPath:               killSwitchPlanPath,
		ActivationPolicyPlanPath:         control.activationPolicyPlanPath,
		ActivationReadinessAuditPath:     control.activationReadinessAuditPath,
		RealCallProposalPath:             control.realCallProposalPath,
		CredentialPolicyPlanPath:         control.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: control.changeProposalReportPath,
		ActivationApprovalPath:           control.activationApprovalPath,
		ActivationRehearsalPath:          control.activationRehearsalPath,
		ExecutionSimulationReportPath:    control.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationFinalAudit() error = %v", err)
	}
	assertProviderActivationFinalAuditBlocked(t, audit)

	ciReport, err := retrievalcontext.ProviderActivationCIReport(retrievalcontext.ProviderActivationCIReportOptions{
		ProviderActivationFinalAuditOptions: retrievalcontext.ProviderActivationFinalAuditOptions{
			ActivationReleasePackagePath:     control.activationReleasePackagePath,
			ActivationReleaseGatePath:        activationReleaseGatePath,
			OperatorReviewBundlePath:         operatorReviewBundlePath,
			KillSwitchPlanPath:               killSwitchPlanPath,
			ActivationPolicyPlanPath:         control.activationPolicyPlanPath,
			ActivationReadinessAuditPath:     control.activationReadinessAuditPath,
			RealCallProposalPath:             control.realCallProposalPath,
			CredentialPolicyPlanPath:         control.credentialPolicyPlanPath,
			ResponseChangeProposalReportPath: control.changeProposalReportPath,
			ActivationApprovalPath:           control.activationApprovalPath,
			ActivationRehearsalPath:          control.activationRehearsalPath,
			ExecutionSimulationReportPath:    control.simulationReportPath,
		},
	})
	if err != nil {
		t.Fatalf("ProviderActivationCIReport() error = %v", err)
	}
	assertProviderActivationCIReportBlocked(t, ciReport)

	return providerActivationHardeningArtifacts{
		providerActivationControlArtifacts: control,
		activationReleaseGatePath:          activationReleaseGatePath,
		operatorReviewBundlePath:           operatorReviewBundlePath,
		killSwitchConfigPath:               killSwitchConfigPath,
		killSwitchPlanPath:                 killSwitchPlanPath,
	}
}

func assertProviderActivationOperatorReviewBlocked(t *testing.T, result retrievalcontext.ProviderActivationOperatorReviewBundleResult) {
	t.Helper()
	if !result.OperatorReviewBundleReady || !result.OperatorReviewRequired {
		t.Fatalf("operator review ready=%t required=%t failures=%#v", result.OperatorReviewBundleReady, result.OperatorReviewRequired, result.Failures)
	}
	if result.OperatorApprovedNow || result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("operator review flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
}

func assertProviderActivationFinalAuditBlocked(t *testing.T, result retrievalcontext.ProviderActivationFinalAuditResult) {
	t.Helper()
	if !result.FinalAuditReady || !result.KillSwitchActive || !result.OperatorReviewRequired {
		t.Fatalf("final audit = %#v, want ready with kill switch and operator review", result)
	}
	if result.RealActivationSupportedNow || result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("final audit flags must stay blocked: %#v", result)
	}
}

func assertProviderActivationCIReportBlocked(t *testing.T, result retrievalcontext.ProviderActivationCIReportResult) {
	t.Helper()
	if !result.CIObservabilityReady || !result.FinalAuditReady || !result.KillSwitchActive {
		t.Fatalf("ci report = %#v, want observability ready", result)
	}
	if result.RealActivationSupportedNow || result.ActivationAllowedNow {
		t.Fatalf("ci report activation flags must stay blocked: %#v", result)
	}
	if len(result.ExpectedLocalCommands) == 0 || len(result.ExpectedPRChecklist) == 0 {
		t.Fatal("ci report must include expected commands and PR checklist")
	}
}

func TestProviderActivationOperatorReviewBundleOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	runProviderActivationHardeningChain(t, chain)
}

func TestProviderActivationFinalAuditFailsWhenKillSwitchInactive(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationHardeningChain(t, chain)

	data, err := os.ReadFile(artifacts.killSwitchPlanPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"global_disabled": true`), []byte(`"global_disabled": false`), 1)
	if err := os.WriteFile(artifacts.killSwitchPlanPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	audit, err := retrievalcontext.ProviderActivationFinalAudit(retrievalcontext.ProviderActivationFinalAuditOptions{
		ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
		ActivationReleaseGatePath:        artifacts.activationReleaseGatePath,
		OperatorReviewBundlePath:         artifacts.operatorReviewBundlePath,
		KillSwitchPlanPath:               artifacts.killSwitchPlanPath,
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationFinalAudit() error = %v", err)
	}
	if audit.Status != lancedbpolicy.StatusFailed || audit.FinalAuditReady {
		t.Fatalf("audit = %#v, want failed when kill switch inactive", audit)
	}
}

func TestProviderActivationFinalAuditFailsWhenOperatorApprovedNow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationHardeningChain(t, chain)

	data, err := os.ReadFile(artifacts.operatorReviewBundlePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"operator_approved_now": false`), []byte(`"operator_approved_now": true`), 1)
	if err := os.WriteFile(artifacts.operatorReviewBundlePath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	audit, err := retrievalcontext.ProviderActivationFinalAudit(retrievalcontext.ProviderActivationFinalAuditOptions{
		ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
		ActivationReleaseGatePath:        artifacts.activationReleaseGatePath,
		OperatorReviewBundlePath:         artifacts.operatorReviewBundlePath,
		KillSwitchPlanPath:               artifacts.killSwitchPlanPath,
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationFinalAudit() error = %v", err)
	}
	if audit.Status != lancedbpolicy.StatusFailed {
		t.Fatal("audit status must be failed when operator_approved_now is true")
	}
}

func TestProviderActivationFinalAuditFailsWhenActivationAllowedNow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationHardeningChain(t, chain)

	data, err := os.ReadFile(artifacts.activationReleaseGatePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"activation_allowed_now": false`), []byte(`"activation_allowed_now": true`), 1)
	if err := os.WriteFile(artifacts.activationReleaseGatePath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	audit, err := retrievalcontext.ProviderActivationFinalAudit(retrievalcontext.ProviderActivationFinalAuditOptions{
		ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
		ActivationReleaseGatePath:        artifacts.activationReleaseGatePath,
		OperatorReviewBundlePath:         artifacts.operatorReviewBundlePath,
		KillSwitchPlanPath:               artifacts.killSwitchPlanPath,
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationFinalAudit() error = %v", err)
	}
	if audit.Status != lancedbpolicy.StatusFailed {
		t.Fatal("audit status must be failed when activation_allowed_now is true")
	}
}
