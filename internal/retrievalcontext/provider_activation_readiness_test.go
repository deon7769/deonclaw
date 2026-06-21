package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validProviderCredentialPolicyConfigYAML = `provider_credential_policy:
  enabled: false
  provider: codex
  credential_check_enabled: false
  allow_secret_read: false
  allow_network: false
  blocked_reason: implementation_not_enabled
  allowed_env_var_names:
    - CODEX_API_KEY
    - OPENAI_API_KEY
`

type providerActivationReadinessArtifacts struct {
	providerExecutionSimulationArtifacts
	credentialConfigPath     string
	credentialPolicyPlanPath string
	realCallProposalPath     string
	changeProposalPath       string
	simulationReportPath     string
}

type providerActivationControlArtifacts struct {
	providerActivationControlPrerequisites
	activationApprovalRequestPath string
	activationApprovalPath        string
	activationRehearsalPath       string
	activationReleasePackagePath  string
}

func runProviderActivationControlPlaneChain(t *testing.T, chain providerCallChainFixture) providerActivationControlArtifacts {
	t.Helper()
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
	if request.Status == lancedbpolicy.StatusFailed || !request.RequestedActivationAuthorized {
		t.Fatalf("NewProviderActivationApprovalRequest() invalid request: %#v", request)
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
		t.Fatalf("inspect status = %q, failures=%#v", inspect.Status, inspect.Failures)
	}

	const rehearsalPath = "provider-activation-rehearsal.json"
	rehearsal, err := retrievalcontext.ProviderActivationRehearsal(retrievalcontext.ProviderActivationRehearsalOptions{
		ActivationPolicyPlanPath:      prereqs.activationPolicyPlanPath,
		ActivationApprovalPath:        approvalPath,
		ActivationReadinessAuditPath:  prereqs.activationReadinessAuditPath,
		RealCallProposalPath:          prereqs.realCallProposalPath,
		CredentialPolicyPlanPath:      prereqs.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: prereqs.simulationReportPath,
		OutputPath:                    rehearsalPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationRehearsal() error = %v", err)
	}
	assertProviderActivationRehearsalBlocked(t, rehearsal)
	assertNoTextExcerpt(t, rehearsalPath)

	rehearsalReport, err := retrievalcontext.ProviderActivationRehearsalReport(retrievalcontext.ProviderActivationRehearsalReportOptions{
		RehearsalPath:                 rehearsalPath,
		ActivationPolicyPlanPath:      prereqs.activationPolicyPlanPath,
		ActivationApprovalPath:        approvalPath,
		ActivationReadinessAuditPath:  prereqs.activationReadinessAuditPath,
		RealCallProposalPath:          prereqs.realCallProposalPath,
		CredentialPolicyPlanPath:      prereqs.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: prereqs.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationRehearsalReport() error = %v", err)
	}
	assertProviderActivationRehearsalBlocked(t, rehearsalReport)

	const releasePackagePath = "provider-activation-release-package.json"
	pkg, err := retrievalcontext.ProviderActivationReleasePackage(retrievalcontext.ProviderActivationReleasePackageOptions{
		ActivationPolicyPlanPath:         prereqs.activationPolicyPlanPath,
		ActivationApprovalPath:           approvalPath,
		ActivationRehearsalPath:          rehearsalPath,
		ActivationReadinessAuditPath:     prereqs.activationReadinessAuditPath,
		RealCallProposalPath:             prereqs.realCallProposalPath,
		ExecutionSimulationReportPath:    prereqs.simulationReportPath,
		CredentialPolicyPlanPath:         prereqs.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: prereqs.changeProposalReportPath,
		OutputPath:                       releasePackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReleasePackage() error = %v", err)
	}
	assertProviderActivationReleasePackageBlocked(t, pkg)
	assertNoTextExcerpt(t, releasePackagePath)

	gate, err := retrievalcontext.ProviderActivationReleaseGate(retrievalcontext.ProviderActivationReleaseGateOptions{
		ActivationPolicyPlanPath:         prereqs.activationPolicyPlanPath,
		ActivationApprovalPath:           approvalPath,
		ActivationRehearsalPath:          rehearsalPath,
		ActivationReadinessAuditPath:     prereqs.activationReadinessAuditPath,
		RealCallProposalPath:             prereqs.realCallProposalPath,
		ExecutionSimulationReportPath:    prereqs.simulationReportPath,
		CredentialPolicyPlanPath:         prereqs.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: prereqs.changeProposalReportPath,
		ActivationReleasePackagePath:     releasePackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReleaseGate() error = %v", err)
	}
	assertProviderActivationReleaseGateBlocked(t, gate)

	return providerActivationControlArtifacts{
		providerActivationControlPrerequisites: prereqs,
		activationApprovalRequestPath:          approvalRequestPath,
		activationApprovalPath:                 approvalPath,
		activationRehearsalPath:                rehearsalPath,
		activationReleasePackagePath:           releasePackagePath,
	}
}

func runProviderActivationReadinessChain(t *testing.T, chain providerCallChainFixture) providerActivationReadinessArtifacts {
	t.Helper()
	simulation := runProviderExecutionSimulationChain(t, chain)

	const simulationReportPath = "provider-execution-simulation-report.json"
	var simulationReportBuf bytes.Buffer
	simulationReport, err := retrievalcontext.ProviderExecutionSimulationReport(retrievalcontext.ProviderExecutionSimulationReportOptions{
		SimulationBundlePath: simulation.simulationBundlePath, RequestEnvelopePath: simulation.requestEnvelopePath,
		AdapterPlanPath: simulation.adapterPlanPath, ResponseFixturePath: simulation.responseFixturePath,
		ReleaseGatePath: simulation.releaseGatePath, ExecutorConfigPath: chain.executorConfigPath,
	})
	if err != nil {
		t.Fatalf("ProviderExecutionSimulationReport() error = %v", err)
	}
	if err := retrievalcontext.WriteProviderExecutionSimulationReportJSON(simulationReport, &simulationReportBuf); err != nil {
		t.Fatalf("WriteProviderExecutionSimulationReportJSON() error = %v", err)
	}
	if err := os.WriteFile(simulationReportPath, simulationReportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(simulation report) error = %v", err)
	}
	assertNoTextExcerpt(t, simulationReportPath)

	const credentialConfigPath = "provider-credential-policy.yaml"
	if err := os.WriteFile(credentialConfigPath, []byte(validProviderCredentialPolicyConfigYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(credential config) error = %v", err)
	}

	cfg, err := retrievalcontext.LoadProviderCredentialPolicyConfig(credentialConfigPath)
	if err != nil {
		t.Fatalf("LoadProviderCredentialPolicyConfig() error = %v", err)
	}
	validateResult, err := retrievalcontext.ProviderCredentialPolicyValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderCredentialPolicyValidate() error = %v", err)
	}
	assertProviderCredentialPolicyBlocked(t, validateResult)

	const credentialPolicyPlanPath = "provider-credential-policy-plan.json"
	credentialPlan, err := retrievalcontext.ProviderCredentialPolicyPlan(retrievalcontext.ProviderCredentialPolicyPlanOptions{
		ConfigPath: credentialConfigPath, OutputPath: credentialPolicyPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderCredentialPolicyPlan() error = %v", err)
	}
	assertProviderCredentialPolicyPlanBlocked(t, credentialPlan)
	assertNoTextExcerpt(t, credentialPolicyPlanPath)

	const realCallProposalPath = "provider-real-call-proposal.json"
	realCallProposal, err := retrievalcontext.NewProviderRealCallProposal(retrievalcontext.ProviderRealCallProposalNewOptions{
		SimulationReportPath: simulationReportPath, RequestEnvelopePath: simulation.requestEnvelopePath,
		AdapterPlanPath: simulation.adapterPlanPath, CredentialPolicyPlanPath: credentialPolicyPlanPath,
		ReleaseGatePath: simulation.releaseGatePath, OutputPath: realCallProposalPath,
	})
	if err != nil {
		t.Fatalf("NewProviderRealCallProposal() error = %v", err)
	}
	if realCallProposal.Status == lancedbpolicy.StatusFailed {
		t.Fatalf("NewProviderRealCallProposal() status failed, failures=%#v", realCallProposal.Failures)
	}
	assertProviderRealCallProposalBlocked(t, realCallProposal)
	assertNoTextExcerpt(t, realCallProposalPath)

	inspect, err := retrievalcontext.InspectProviderRealCallProposal(realCallProposalPath, retrievalcontext.ProviderRealCallProposalInspectOptions{
		SimulationReportPath: simulationReportPath, RequestEnvelopePath: simulation.requestEnvelopePath,
		AdapterPlanPath: simulation.adapterPlanPath, CredentialPolicyPlanPath: credentialPolicyPlanPath,
		ReleaseGatePath: simulation.releaseGatePath,
	})
	if err != nil {
		t.Fatalf("InspectProviderRealCallProposal() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect status = %q, failures=%#v", inspect.Status, inspect.Failures)
	}

	const changeProposalPath = "provider-response-change-proposal.json"
	changeProposal, err := retrievalcontext.ProviderResponseChangeProposal(retrievalcontext.ProviderResponseChangeProposalOptions{
		ResponseFixturePath: simulation.responseFixturePath, SimulationReportPath: simulationReportPath,
		RealCallProposalPath: realCallProposalPath, OutputPath: changeProposalPath,
	})
	if err != nil {
		t.Fatalf("ProviderResponseChangeProposal() error = %v", err)
	}
	assertProviderResponseChangeProposalBlocked(t, changeProposal)
	assertNoTextExcerpt(t, changeProposalPath)

	changeReport, err := retrievalcontext.ProviderResponseChangeProposalReport(retrievalcontext.ProviderResponseChangeProposalReportOptions{
		ChangeProposalPath: changeProposalPath, ResponseFixturePath: simulation.responseFixturePath,
		SimulationReportPath: simulationReportPath, RealCallProposalPath: realCallProposalPath,
	})
	if err != nil {
		t.Fatalf("ProviderResponseChangeProposalReport() error = %v", err)
	}
	assertProviderResponseChangeProposalBlocked(t, changeReport)

	audit, err := retrievalcontext.ProviderActivationReadinessAudit(retrievalcontext.ProviderActivationReadinessAuditOptions{
		CredentialPolicyPlanPath: credentialPolicyPlanPath, RealCallProposalPath: realCallProposalPath,
		ChangeProposalPath: changeProposalPath, SimulationReportPath: simulationReportPath,
		ReleaseGatePath: simulation.releaseGatePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReadinessAudit() error = %v", err)
	}
	assertProviderActivationReadinessBlocked(t, audit)

	report, err := retrievalcontext.ProviderActivationReadinessReport(retrievalcontext.ProviderActivationReadinessAuditOptions{
		CredentialPolicyPlanPath: credentialPolicyPlanPath, RealCallProposalPath: realCallProposalPath,
		ChangeProposalPath: changeProposalPath, SimulationReportPath: simulationReportPath,
		ReleaseGatePath: simulation.releaseGatePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReadinessReport() error = %v", err)
	}
	assertProviderActivationReadinessBlocked(t, report)

	return providerActivationReadinessArtifacts{
		providerExecutionSimulationArtifacts: simulation,
		credentialConfigPath:                 credentialConfigPath,
		credentialPolicyPlanPath:             credentialPolicyPlanPath,
		realCallProposalPath:                 realCallProposalPath,
		changeProposalPath:                   changeProposalPath,
		simulationReportPath:                 simulationReportPath,
	}
}

func assertProviderCredentialPolicyBlocked(t *testing.T, result retrievalcontext.ProviderCredentialPolicyValidateResult) {
	t.Helper()
	if !result.CredentialPolicyValidated {
		t.Fatalf("credential_policy_validated = false, failures=%#v", result.Failures)
	}
	if result.CredentialCheckEnabled || result.SecretValuesRead || result.ProviderCall || result.NetworkCall {
		t.Fatalf("credential policy validate flags must stay blocked: %#v", result)
	}
}

func assertProviderCredentialPolicyPlanBlocked(t *testing.T, result retrievalcontext.ProviderCredentialPolicyPlanResult) {
	t.Helper()
	if !result.CredentialPolicyPlanReady || !result.CredentialPolicyValidated {
		t.Fatalf("credential policy plan ready=%t validated=%t", result.CredentialPolicyPlanReady, result.CredentialPolicyValidated)
	}
	if result.SecretValuesRead || result.ProviderCall || result.NetworkCall {
		t.Fatalf("credential policy plan flags must stay blocked: %#v", result)
	}
}

func assertProviderRealCallProposalBlocked(t *testing.T, proposal retrievalcontext.ProviderRealCallProposal) {
	t.Helper()
	if !proposal.RealCallProposalReady || !proposal.WouldCallProviderIfEnabled {
		t.Fatalf("real call proposal ready=%t would_call=%t", proposal.RealCallProposalReady, proposal.WouldCallProviderIfEnabled)
	}
	if proposal.ProviderCallAllowedNow || proposal.SecretValuesRead || proposal.TransportCalled || proposal.SentToProvider {
		t.Fatalf("real call proposal flags must stay blocked: %#v", proposal)
	}
}

func assertProviderRealCallProposalInspectBlocked(t *testing.T, result retrievalcontext.ProviderRealCallProposalInspectResult) {
	t.Helper()
	if !result.RealCallProposalReady || !result.WouldCallProviderIfEnabled {
		t.Fatalf("real call inspect ready=%t would_call=%t", result.RealCallProposalReady, result.WouldCallProviderIfEnabled)
	}
	if result.ProviderCallAllowedNow || result.SecretValuesRead || result.TransportCalled || result.SentToProvider {
		t.Fatalf("real call inspect flags must stay blocked: %#v", result)
	}
}

func assertProviderResponseChangeProposalBlocked(t *testing.T, result retrievalcontext.ProviderResponseChangeProposalResult) {
	t.Helper()
	if !result.ChangeProposalReady {
		t.Fatal("change_proposal_ready = false, want true")
	}
	if result.WorkspaceModified || result.DiffApplied || result.CommitCreated || result.PRCreated || result.WorkerExecution {
		t.Fatalf("change proposal flags must stay blocked: %#v", result)
	}
	if result.ResponseSource != retrievalcontext.ProviderResponseFixtureSource {
		t.Fatalf("response_source = %q", result.ResponseSource)
	}
}

func assertProviderActivationReadinessBlocked(t *testing.T, result retrievalcontext.ProviderActivationReadinessAuditResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("activation readiness status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ActivationReadinessReady {
		t.Fatal("activation_readiness_ready = false, want true")
	}
	if result.RealProviderCallSupportedNow || result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.TransportCalled || result.SecretValuesRead || result.WorkspaceModified {
		t.Fatalf("activation readiness flags must stay blocked: %#v", result)
	}
}

func TestProviderActivationReadinessChainSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	runProviderActivationReadinessChain(t, chain)
}

func TestProviderCredentialPolicyValidateFailsWhenSecretReadAllowed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := bytes.Replace([]byte(validProviderCredentialPolicyConfigYAML), []byte("allow_secret_read: false"), []byte("allow_secret_read: true"), 1)
	if err := os.WriteFile("bad-credential-policy.yaml", content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderCredentialPolicyConfig("bad-credential-policy.yaml")
	if err != nil {
		t.Fatalf("LoadProviderCredentialPolicyConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderCredentialPolicyValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderCredentialPolicyValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestProviderActivationReadinessAuditFailsWhenWorkspaceModified(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationReadinessChain(t, chain)

	data, err := os.ReadFile(artifacts.changeProposalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"workspace_modified": false`), []byte(`"workspace_modified": true`), 1)
	if err := os.WriteFile(artifacts.changeProposalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	audit, err := retrievalcontext.ProviderActivationReadinessAudit(retrievalcontext.ProviderActivationReadinessAuditOptions{
		CredentialPolicyPlanPath: artifacts.credentialPolicyPlanPath, RealCallProposalPath: artifacts.realCallProposalPath,
		ChangeProposalPath: artifacts.changeProposalPath, SimulationReportPath: artifacts.simulationReportPath,
		ReleaseGatePath: artifacts.releaseGatePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReadinessAudit() error = %v", err)
	}
	if audit.Status != lancedbpolicy.StatusFailed {
		t.Fatal("audit status must be failed after workspace_modified tamper")
	}
}
