package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type providerRealDispatchExternalApprovalArtifacts struct {
	providerRealDispatchDesignArtifacts
	externalApprovalRequestPath string
	externalApprovalPath        string
	runbookPath                 string
	riskRegisterPath            string
}

func runProviderRealDispatchExternalApprovalChain(t *testing.T, chain providerCallChainFixture) providerRealDispatchExternalApprovalArtifacts {
	t.Helper()
	design := runProviderRealDispatchDesignChain(t, chain)

	const externalApprovalRequestPath = "provider-real-dispatch-external-approval-request.json"
	request, err := retrievalcontext.NewProviderRealDispatchExternalApprovalRequest(retrievalcontext.NewProviderRealDispatchExternalApprovalRequestOptions{
		DesignReviewPackagePath:             design.designReviewPackagePath,
		DesignReviewGatePath:                design.designReviewGatePath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		ActivationCIReportPath:              design.activationCIReportPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
		OperatorReviewBundlePath:            design.operatorReviewBundlePath,
		OutputPath:                          externalApprovalRequestPath,
	})
	if err != nil {
		t.Fatalf("NewProviderRealDispatchExternalApprovalRequest() error = %v", err)
	}
	assertProviderRealDispatchExternalApprovalRequestBlocked(t, request)
	assertNoPreviewLeakInFile(t, externalApprovalRequestPath)

	const externalApprovalPath = "provider-real-dispatch-external-approval.json"
	approval, err := retrievalcontext.ApproveProviderRealDispatchExternalApproval(retrievalcontext.ApproveProviderRealDispatchExternalApprovalOptions{
		RequestPath:                      externalApprovalRequestPath,
		OutputPath:                       externalApprovalPath,
		ConfirmDesignReviewPackageSHA256: request.DesignReviewPackageSHA256,
		ConfirmDesignReviewGateSHA256:    request.DesignReviewGateSHA256,
		ConfirmSecretReadProposalSHA256:  request.SecretReadProposalSHA256,
		ConfirmRealDispatchDesignSHA256:  request.RealDispatchDesignSHA256,
		ConfirmProviderPayloadSHA256:     request.ProviderPayloadSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderRealDispatchExternalApproval() error = %v", err)
	}
	assertProviderRealDispatchExternalApprovalBlocked(t, approval)
	assertNoPreviewLeakInFile(t, externalApprovalPath)

	inspect, err := retrievalcontext.InspectProviderRealDispatchExternalApproval(externalApprovalPath, retrievalcontext.InspectProviderRealDispatchExternalApprovalOptions{
		RequestPath:          externalApprovalRequestPath,
		DesignReviewGatePath: design.designReviewGatePath,
	})
	if err != nil {
		t.Fatalf("InspectProviderRealDispatchExternalApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect status = %q failures=%#v", inspect.Status, inspect.Failures)
	}

	const runbookPath = "provider-real-dispatch-runbook.json"
	runbook, err := retrievalcontext.ProviderRealDispatchRunbook(retrievalcontext.ProviderRealDispatchRunbookOptions{
		ExternalApprovalPath:                externalApprovalPath,
		ExternalApprovalRequestPath:         externalApprovalRequestPath,
		DesignReviewGatePath:                design.designReviewGatePath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		ActivationCIReportPath:              design.activationCIReportPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
		OutputPath:                          runbookPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchRunbook() error = %v", err)
	}
	assertProviderRealDispatchRunbookBlocked(t, runbook)
	assertNoPreviewLeakInFile(t, runbookPath)

	runbookReport, err := retrievalcontext.ProviderRealDispatchRunbookReport(retrievalcontext.ProviderRealDispatchRunbookReportOptions{
		RunbookPath:                         runbookPath,
		ExternalApprovalPath:                externalApprovalPath,
		ExternalApprovalRequestPath:         externalApprovalRequestPath,
		DesignReviewGatePath:                design.designReviewGatePath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		ActivationCIReportPath:              design.activationCIReportPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchRunbookReport() error = %v", err)
	}
	assertProviderRealDispatchRunbookBlocked(t, runbookReport)

	const riskRegisterPath = "provider-real-dispatch-risk-register.json"
	riskRegister, err := retrievalcontext.ProviderRealDispatchRiskRegister(retrievalcontext.ProviderRealDispatchRiskRegisterOptions{
		RunbookPath:                         runbookPath,
		ExternalApprovalPath:                externalApprovalPath,
		DesignReviewGatePath:                design.designReviewGatePath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
		OutputPath:                          riskRegisterPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchRiskRegister() error = %v", err)
	}
	assertProviderRealDispatchRiskRegisterBlocked(t, riskRegister)
	assertNoPreviewLeakInFile(t, riskRegisterPath)

	riskReport, err := retrievalcontext.ProviderRealDispatchRiskReport(retrievalcontext.ProviderRealDispatchRiskReportOptions{
		RiskRegisterPath:                    riskRegisterPath,
		RunbookPath:                         runbookPath,
		ExternalApprovalPath:                externalApprovalPath,
		DesignReviewGatePath:                design.designReviewGatePath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchRiskReport() error = %v", err)
	}
	assertProviderRealDispatchRiskRegisterBlocked(t, riskReport)

	gate, err := retrievalcontext.ProviderRealDispatchPreimplementationGate(retrievalcontext.ProviderRealDispatchPreimplementationGateOptions{
		ExternalApprovalPath:                externalApprovalPath,
		RunbookPath:                         runbookPath,
		RiskRegisterPath:                    riskRegisterPath,
		DesignReviewGatePath:                design.designReviewGatePath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		ActivationCIReportPath:              design.activationCIReportPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
		OperatorReviewBundlePath:            design.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchPreimplementationGate() error = %v", err)
	}
	assertProviderRealDispatchPreimplementationGateBlocked(t, gate)

	report, err := retrievalcontext.ProviderRealDispatchPreimplementationReport(retrievalcontext.ProviderRealDispatchPreimplementationGateOptions{
		ExternalApprovalPath:                externalApprovalPath,
		RunbookPath:                         runbookPath,
		RiskRegisterPath:                    riskRegisterPath,
		DesignReviewGatePath:                design.designReviewGatePath,
		SecretReadProposalPath:              design.secretReadProposalPath,
		RealTransportImplementationPlanPath: design.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              design.realDispatchDesignPath,
		ActivationFinalAuditPath:            design.activationFinalAuditPath,
		ActivationCIReportPath:              design.activationCIReportPath,
		KillSwitchPlanPath:                  design.killSwitchPlanPath,
		OperatorReviewBundlePath:            design.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchPreimplementationReport() error = %v", err)
	}
	assertProviderRealDispatchPreimplementationGateBlocked(t, report)

	return providerRealDispatchExternalApprovalArtifacts{
		providerRealDispatchDesignArtifacts: design,
		externalApprovalRequestPath:         externalApprovalRequestPath,
		externalApprovalPath:                externalApprovalPath,
		runbookPath:                         runbookPath,
		riskRegisterPath:                    riskRegisterPath,
	}
}

func assertProviderRealDispatchExternalApprovalRequestBlocked(t *testing.T, request retrievalcontext.ProviderRealDispatchExternalApprovalRequest) {
	t.Helper()
	if request.Status != retrievalcontext.RequestStatusPending || !request.RequestedExternalApprovalAuthorized {
		t.Fatalf("external approval request = %#v", request)
	}
	if request.ExternalApprovalAllowedNow || request.RealDispatchAllowedNow || request.ExecuteSubcommandRegistered || request.SecretValuesRead || request.ProviderCall || request.NetworkCall || request.TransportCalled || request.WorkspaceModified {
		t.Fatalf("external approval request flags must stay blocked: %#v", request)
	}
}

func assertProviderRealDispatchExternalApprovalBlocked(t *testing.T, approval retrievalcontext.ProviderRealDispatchExternalApproval) {
	t.Helper()
	if !approval.Approved || !approval.ExternalApprovalAuthorizedForFuture {
		t.Fatalf("external approval = %#v", approval)
	}
	if approval.ExternalApprovalAllowedNow || approval.RealDispatchAllowedNow || approval.ExecuteSubcommandRegistered || approval.SecretValuesRead || approval.ProviderCall || approval.NetworkCall || approval.TransportCalled || approval.WorkspaceModified {
		t.Fatalf("external approval flags must stay blocked: %#v", approval)
	}
}

func assertProviderRealDispatchRunbookBlocked(t *testing.T, result retrievalcontext.ProviderRealDispatchRunbookResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK || !result.RunbookReady || !result.RunbookMetadataOnly {
		t.Fatalf("runbook = %#v", result)
	}
	if result.ScriptGenerated || result.ExecuteCommandGenerated || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("runbook flags must stay blocked: %#v", result)
	}
}

func assertProviderRealDispatchRiskRegisterBlocked(t *testing.T, result retrievalcontext.ProviderRealDispatchRiskRegisterResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK || !result.RiskRegisterReady || !result.RiskReviewRequired || !result.RisksBlocked {
		t.Fatalf("risk register = %#v", result)
	}
	for _, risk := range result.Risks {
		if risk.CurrentStatus == "active_execution" {
			t.Fatalf("risk %q must not be active_execution", risk.ID)
		}
	}
	if result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("risk register flags must stay blocked: %#v", result)
	}
}

func assertProviderRealDispatchPreimplementationGateBlocked(t *testing.T, result retrievalcontext.ProviderRealDispatchPreimplementationGateResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK || !result.PreimplementationGateReady {
		t.Fatalf("preimplementation gate = %#v", result)
	}
	if !result.KillSwitchActive || !result.OperatorReviewRequired {
		t.Fatalf("preimplementation gate governance = %#v", result)
	}
	if result.RealDispatchSupportedNow || result.RealDispatchAllowedNow || result.ExecuteSubcommandRegistered || result.SecretValuesRead || result.ProviderCall || result.NetworkCall || result.TransportCalled || result.WorkspaceModified || result.WorkerExecution {
		t.Fatalf("preimplementation gate flags must stay blocked: %#v", result)
	}
}

func TestProviderRealDispatchExternalApprovalChainOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	runProviderRealDispatchExternalApprovalChain(t, chain)
}

func TestProviderRealDispatchExternalApprovalApproveRequiresConfirmHashes(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	request, err := retrievalcontext.NewProviderRealDispatchExternalApprovalRequest(retrievalcontext.NewProviderRealDispatchExternalApprovalRequestOptions{
		DesignReviewPackagePath:             artifacts.designReviewPackagePath,
		DesignReviewGatePath:                artifacts.designReviewGatePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
		OutputPath:                          "bad-request.json",
	})
	if err != nil {
		t.Fatalf("NewProviderRealDispatchExternalApprovalRequest() error = %v", err)
	}

	_, err = retrievalcontext.ApproveProviderRealDispatchExternalApproval(retrievalcontext.ApproveProviderRealDispatchExternalApprovalOptions{
		RequestPath:                      "bad-request.json",
		OutputPath:                       "bad-approval.json",
		ConfirmDesignReviewPackageSHA256: request.DesignReviewPackageSHA256,
		ConfirmDesignReviewGateSHA256:    request.DesignReviewGateSHA256,
		ConfirmSecretReadProposalSHA256:  request.SecretReadProposalSHA256,
		ConfirmRealDispatchDesignSHA256:  "mismatch",
		ConfirmProviderPayloadSHA256:     request.ProviderPayloadSHA256,
	})
	if err == nil {
		t.Fatal("approve must fail on confirm hash mismatch")
	}
}

func TestProviderRealDispatchExternalApprovalInspectDetectsHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	data, err := os.ReadFile(artifacts.externalApprovalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"design_review_gate_sha256": "`), []byte(`"design_review_gate_sha256": "mismatch-`), 1)
	if err := os.WriteFile(artifacts.externalApprovalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	inspect, err := retrievalcontext.InspectProviderRealDispatchExternalApproval(artifacts.externalApprovalPath, retrievalcontext.InspectProviderRealDispatchExternalApprovalOptions{
		RequestPath:          artifacts.externalApprovalRequestPath,
		DesignReviewGatePath: artifacts.designReviewGatePath,
	})
	if err != nil {
		t.Fatalf("InspectProviderRealDispatchExternalApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusFailed {
		t.Fatal("inspect must fail after gate hash tamper")
	}
}

func TestProviderRealDispatchRunbookFailsWhenExecuteCommandGenerated(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	data, err := os.ReadFile(artifacts.runbookPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"execute_command_generated": false`), []byte(`"execute_command_generated": true`), 1)
	if err := os.WriteFile(artifacts.runbookPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := retrievalcontext.ProviderRealDispatchRunbookReport(retrievalcontext.ProviderRealDispatchRunbookReportOptions{
		RunbookPath:                         artifacts.runbookPath,
		ExternalApprovalPath:                artifacts.externalApprovalPath,
		ExternalApprovalRequestPath:         artifacts.externalApprovalRequestPath,
		DesignReviewGatePath:                artifacts.designReviewGatePath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchRunbookReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatal("runbook report must fail when execute_command_generated true")
	}
}

func TestProviderRealDispatchRiskRegisterFailsWhenRiskActiveExecution(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	data, err := os.ReadFile(artifacts.riskRegisterPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"current_status": "blocked"`), []byte(`"current_status": "active_execution"`), 1)
	if err := os.WriteFile(artifacts.riskRegisterPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := retrievalcontext.ProviderRealDispatchRiskReport(retrievalcontext.ProviderRealDispatchRiskReportOptions{
		RiskRegisterPath:                    artifacts.riskRegisterPath,
		RunbookPath:                         artifacts.runbookPath,
		ExternalApprovalPath:                artifacts.externalApprovalPath,
		DesignReviewGatePath:                artifacts.designReviewGatePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchRiskReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatal("risk report must fail when risk active_execution")
	}
}

func TestProviderRealDispatchPreimplementationGateFailsWhenKillSwitchInactive(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	data, err := os.ReadFile(artifacts.activationFinalAuditPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"kill_switch_active": true`), []byte(`"kill_switch_active": false`), 1)
	if err := os.WriteFile(artifacts.activationFinalAuditPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	gate, err := retrievalcontext.ProviderRealDispatchPreimplementationGate(retrievalcontext.ProviderRealDispatchPreimplementationGateOptions{
		ExternalApprovalPath:                artifacts.externalApprovalPath,
		RunbookPath:                         artifacts.runbookPath,
		RiskRegisterPath:                    artifacts.riskRegisterPath,
		DesignReviewGatePath:                artifacts.designReviewGatePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchPreimplementationGate() error = %v", err)
	}
	if gate.Status != lancedbpolicy.StatusFailed {
		t.Fatal("preimplementation gate must fail when kill_switch_active false")
	}
}

func TestProviderRealDispatchPreimplementationGateFailsWhenExecuteRegistered(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	data, err := os.ReadFile(artifacts.realDispatchDesignPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"execute_subcommand_registered": false`), []byte(`"execute_subcommand_registered": true`), 1)
	if err := os.WriteFile(artifacts.realDispatchDesignPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	gate, err := retrievalcontext.ProviderRealDispatchPreimplementationGate(retrievalcontext.ProviderRealDispatchPreimplementationGateOptions{
		ExternalApprovalPath:                artifacts.externalApprovalPath,
		RunbookPath:                         artifacts.runbookPath,
		RiskRegisterPath:                    artifacts.riskRegisterPath,
		DesignReviewGatePath:                artifacts.designReviewGatePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchPreimplementationGate() error = %v", err)
	}
	if gate.Status != lancedbpolicy.StatusFailed {
		t.Fatal("preimplementation gate must fail when execute_subcommand_registered true")
	}
}

func TestProviderRealDispatchPreimplementationGateFailsWhenProviderCall(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchExternalApprovalChain(t, chain)

	data, err := os.ReadFile(artifacts.externalApprovalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"provider_call": false`), []byte(`"provider_call": true`), 1)
	if err := os.WriteFile(artifacts.externalApprovalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	gate, err := retrievalcontext.ProviderRealDispatchPreimplementationGate(retrievalcontext.ProviderRealDispatchPreimplementationGateOptions{
		ExternalApprovalPath:                artifacts.externalApprovalPath,
		RunbookPath:                         artifacts.runbookPath,
		RiskRegisterPath:                    artifacts.riskRegisterPath,
		DesignReviewGatePath:                artifacts.designReviewGatePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchPreimplementationGate() error = %v", err)
	}
	if gate.Status != lancedbpolicy.StatusFailed {
		t.Fatal("preimplementation gate must fail when provider_call true")
	}
}
