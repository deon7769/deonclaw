package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type providerRealDispatchDesignArtifacts struct {
	providerActivationHardeningArtifacts
	secretReadProposalPath              string
	realTransportImplementationPlanPath string
	realDispatchDesignPath              string
	designReviewPackagePath             string
}

func runProviderRealDispatchDesignChain(t *testing.T, chain providerCallChainFixture) providerRealDispatchDesignArtifacts {
	t.Helper()
	hardening := runProviderActivationHardeningChain(t, chain)

	const secretReadProposalPath = "provider-secret-read-proposal.json"
	proposal, err := retrievalcontext.NewProviderSecretReadProposal(retrievalcontext.ProviderSecretReadProposalNewOptions{
		CredentialPolicyPlanPath:  hardening.credentialPolicyPlanPath,
		ActivationFinalAuditPath:  hardening.activationFinalAuditPath,
		ActivationCIReportPath:    hardening.activationCIReportPath,
		ActivationReleaseGatePath: hardening.activationReleaseGatePath,
		OutputPath:                secretReadProposalPath,
	})
	if err != nil {
		t.Fatalf("NewProviderSecretReadProposal() error = %v", err)
	}
	assertProviderSecretReadProposalBlocked(t, proposal)
	assertNoPreviewLeakInFile(t, secretReadProposalPath)

	inspect, err := retrievalcontext.InspectProviderSecretReadProposal(secretReadProposalPath, retrievalcontext.ProviderSecretReadProposalInspectOptions{
		CredentialPolicyPlanPath:  hardening.credentialPolicyPlanPath,
		ActivationFinalAuditPath:  hardening.activationFinalAuditPath,
		ActivationCIReportPath:    hardening.activationCIReportPath,
		ActivationReleaseGatePath: hardening.activationReleaseGatePath,
	})
	if err != nil {
		t.Fatalf("InspectProviderSecretReadProposal() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK || !inspect.SecretReadProposalReady {
		t.Fatalf("inspect = %#v", inspect)
	}

	const realTransportPlanPath = "provider-real-transport-implementation-plan.json"
	transportPlan, err := retrievalcontext.ProviderRealTransportImplementationPlan(retrievalcontext.ProviderRealTransportImplementationPlanOptions{
		SecretReadProposalPath:   secretReadProposalPath,
		ProviderAdapterPlanPath:  hardening.adapterPlanPath,
		ActivationFinalAuditPath: hardening.activationFinalAuditPath,
		OperatorReviewBundlePath: hardening.operatorReviewBundlePath,
		KillSwitchPlanPath:       hardening.killSwitchPlanPath,
		OutputPath:               realTransportPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealTransportImplementationPlan() error = %v", err)
	}
	assertProviderRealTransportImplementationPlanBlocked(t, transportPlan)
	assertNoPreviewLeakInFile(t, realTransportPlanPath)

	transportReport, err := retrievalcontext.ProviderRealTransportImplementationReport(retrievalcontext.ProviderRealTransportImplementationReportOptions{
		RealTransportImplementationPlanPath: realTransportPlanPath,
		SecretReadProposalPath:              secretReadProposalPath,
		ProviderAdapterPlanPath:             hardening.adapterPlanPath,
		ActivationFinalAuditPath:            hardening.activationFinalAuditPath,
		OperatorReviewBundlePath:            hardening.operatorReviewBundlePath,
		KillSwitchPlanPath:                  hardening.killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealTransportImplementationReport() error = %v", err)
	}
	assertProviderRealTransportImplementationPlanBlocked(t, transportReport)

	const realDispatchDesignPath = "provider-real-dispatch-design.json"
	dispatchDesign, err := retrievalcontext.ProviderRealDispatchDesign(retrievalcontext.ProviderRealDispatchDesignOptions{
		RealTransportImplementationPlanPath: realTransportPlanPath,
		SecretReadProposalPath:              secretReadProposalPath,
		ActivationFinalAuditPath:            hardening.activationFinalAuditPath,
		ActivationCIReportPath:              hardening.activationCIReportPath,
		ProviderRequestEnvelopePath:         hardening.requestEnvelopePath,
		ProviderRealCallProposalPath:        hardening.realCallProposalPath,
		OutputPath:                          realDispatchDesignPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchDesign() error = %v", err)
	}
	assertProviderRealDispatchDesignBlocked(t, dispatchDesign)
	assertNoPreviewLeakInFile(t, realDispatchDesignPath)

	dispatchReport, err := retrievalcontext.ProviderRealDispatchDesignReport(retrievalcontext.ProviderRealDispatchDesignReportOptions{
		RealDispatchDesignPath:              realDispatchDesignPath,
		RealTransportImplementationPlanPath: realTransportPlanPath,
		SecretReadProposalPath:              secretReadProposalPath,
		ActivationFinalAuditPath:            hardening.activationFinalAuditPath,
		ActivationCIReportPath:              hardening.activationCIReportPath,
		ProviderRequestEnvelopePath:         hardening.requestEnvelopePath,
		ProviderRealCallProposalPath:        hardening.realCallProposalPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchDesignReport() error = %v", err)
	}
	assertProviderRealDispatchDesignBlocked(t, dispatchReport)

	const designReviewPackagePath = "provider-real-activation-design-review-package.json"
	reviewPackage, err := retrievalcontext.ProviderRealActivationDesignReviewPackage(retrievalcontext.ProviderRealActivationDesignReviewPackageOptions{
		SecretReadProposalPath:              secretReadProposalPath,
		RealTransportImplementationPlanPath: realTransportPlanPath,
		RealDispatchDesignPath:              realDispatchDesignPath,
		ActivationFinalAuditPath:            hardening.activationFinalAuditPath,
		ActivationCIReportPath:              hardening.activationCIReportPath,
		KillSwitchPlanPath:                  hardening.killSwitchPlanPath,
		OperatorReviewBundlePath:            hardening.operatorReviewBundlePath,
		OutputPath:                          designReviewPackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealActivationDesignReviewPackage() error = %v", err)
	}
	assertProviderRealActivationDesignReviewBlocked(t, reviewPackage)
	assertNoPreviewLeakInFile(t, designReviewPackagePath)

	gate, err := retrievalcontext.ProviderRealActivationDesignReviewGate(retrievalcontext.ProviderRealActivationDesignReviewGateOptions{
		DesignReviewPackagePath:             designReviewPackagePath,
		SecretReadProposalPath:              secretReadProposalPath,
		RealTransportImplementationPlanPath: realTransportPlanPath,
		RealDispatchDesignPath:              realDispatchDesignPath,
		ActivationFinalAuditPath:            hardening.activationFinalAuditPath,
		ActivationCIReportPath:              hardening.activationCIReportPath,
		KillSwitchPlanPath:                  hardening.killSwitchPlanPath,
		OperatorReviewBundlePath:            hardening.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealActivationDesignReviewGate() error = %v", err)
	}
	assertProviderRealActivationDesignReviewGateBlocked(t, gate)

	var gateBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderRealActivationDesignReviewGateJSON(gate, &gateBuf); err != nil {
		t.Fatalf("WriteProviderRealActivationDesignReviewGateJSON() error = %v", err)
	}
	assertNoPreviewLeakInString(t, "design review gate stdout", gateBuf.String())

	return providerRealDispatchDesignArtifacts{
		providerActivationHardeningArtifacts: hardening,
		secretReadProposalPath:               secretReadProposalPath,
		realTransportImplementationPlanPath:  realTransportPlanPath,
		realDispatchDesignPath:               realDispatchDesignPath,
		designReviewPackagePath:              designReviewPackagePath,
	}
}

func assertProviderSecretReadProposalBlocked(t *testing.T, result retrievalcontext.ProviderSecretReadProposal) {
	t.Helper()
	if !result.SecretReadProposalReady {
		t.Fatalf("secret read proposal = %#v", result)
	}
	if result.SecretReadAllowedNow || result.SecretValuesRead || result.ProviderCall || result.NetworkCall || result.TransportCalled || result.ActivationAllowedNow {
		t.Fatalf("secret read proposal flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
	if len(result.FutureEnvVarNames) == 0 {
		t.Fatal("future_env_var_names must be declared")
	}
}

func assertProviderRealTransportImplementationPlanBlocked(t *testing.T, result retrievalcontext.ProviderRealTransportImplementationPlanResult) {
	t.Helper()
	if !result.RealTransportImplementationPlanReady {
		t.Fatalf("transport plan = %#v", result)
	}
	if result.RealTransportAvailableNow || result.TransportEnabled || result.TransportCalled || result.ProviderCall || result.NetworkCall || result.SecretValuesRead {
		t.Fatalf("transport plan flags must stay blocked: %#v", result)
	}
	if result.ActiveTransportMode != "BlockedProviderTransport" {
		t.Fatalf("active_transport_mode = %q", result.ActiveTransportMode)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
}

func assertProviderRealDispatchDesignBlocked(t *testing.T, result retrievalcontext.ProviderRealDispatchDesignResult) {
	t.Helper()
	if !result.RealDispatchDesignReady {
		t.Fatalf("dispatch design = %#v", result)
	}
	if result.RealDispatchCommandAvailable || result.ExecuteSubcommandRegistered {
		t.Fatalf("dispatch design must keep execute disabled: %#v", result)
	}
	if result.ProviderCall || result.NetworkCall || result.TransportCalled || result.SecretValuesRead || result.SentToProvider || result.ReceivedFromProvider || result.WorkspaceModified || result.DiffApplied || result.CommitCreated || result.PRCreated || result.WorkerExecution || result.PromptInjectionRealRunner {
		t.Fatalf("dispatch design flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
}

func assertProviderRealActivationDesignReviewBlocked(t *testing.T, result retrievalcontext.ProviderRealActivationDesignReviewPackageResult) {
	t.Helper()
	if !result.RealActivationDesignReviewReady {
		t.Fatalf("design review package = %#v", result)
	}
	if !result.KillSwitchActive || !result.OperatorReviewRequired {
		t.Fatalf("design review package governance = %#v", result)
	}
	assertProviderRealActivationDesignExecutionFlagsBlocked(t, "design review package", result.RealDispatchSupportedNow, result.ActivationAllowedNow, result.SecretValuesRead, result.ProviderCall, result.NetworkCall, result.TransportCalled, result.SentToProvider, result.ReceivedFromProvider, result.WorkspaceModified, result.DiffApplied, result.CommitCreated, result.PRCreated, result.WorkerExecution, result.PromptInjectionRealRunner)
}

func assertProviderRealActivationDesignReviewGateBlocked(t *testing.T, result retrievalcontext.ProviderRealActivationDesignReviewGateResult) {
	t.Helper()
	if !result.RealActivationDesignReviewReady || !result.RealActivationDesignGateReady {
		t.Fatalf("design review gate = %#v", result)
	}
	if !result.KillSwitchActive || !result.OperatorReviewRequired {
		t.Fatalf("design review gate governance = %#v", result)
	}
	assertProviderRealActivationDesignExecutionFlagsBlocked(t, "design review gate", result.RealDispatchSupportedNow, result.ActivationAllowedNow, result.SecretValuesRead, result.ProviderCall, result.NetworkCall, result.TransportCalled, result.SentToProvider, result.ReceivedFromProvider, result.WorkspaceModified, result.DiffApplied, result.CommitCreated, result.PRCreated, result.WorkerExecution, result.PromptInjectionRealRunner)
}

func assertProviderRealActivationDesignExecutionFlagsBlocked(t *testing.T, label string, realDispatchSupportedNow, activationAllowedNow, secretValuesRead, providerCall, networkCall, transportCalled, sentToProvider, receivedFromProvider, workspaceModified, diffApplied, commitCreated, prCreated, workerExecution, promptInjectionRealRunner bool) {
	t.Helper()
	if realDispatchSupportedNow || activationAllowedNow || secretValuesRead || providerCall || networkCall || transportCalled || sentToProvider || receivedFromProvider || workspaceModified || diffApplied || commitCreated || prCreated || workerExecution || promptInjectionRealRunner {
		t.Fatalf("%s execution flags must stay blocked: real_dispatch_supported_now=%t activation_allowed_now=%t secret_values_read=%t provider_call=%t network_call=%t transport_called=%t sent_to_provider=%t received_from_provider=%t workspace_modified=%t diff_applied=%t commit_created=%t pr_created=%t worker_execution=%t prompt_injection_real_runner=%t",
			label, realDispatchSupportedNow, activationAllowedNow, secretValuesRead, providerCall, networkCall, transportCalled, sentToProvider, receivedFromProvider, workspaceModified, diffApplied, commitCreated, prCreated, workerExecution, promptInjectionRealRunner)
	}
}

func TestProviderRealDispatchDesignChainOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	runProviderRealDispatchDesignChain(t, chain)
}

func TestProviderSecretReadProposalInspectFailsWhenActivationReleaseGateSHA256Mismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.secretReadProposalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"activation_release_gate_sha256": "`), []byte(`"activation_release_gate_sha256": "mismatch-`), 1)
	if err := os.WriteFile(artifacts.secretReadProposalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	inspect, err := retrievalcontext.InspectProviderSecretReadProposal(artifacts.secretReadProposalPath, retrievalcontext.ProviderSecretReadProposalInspectOptions{
		CredentialPolicyPlanPath:  artifacts.credentialPolicyPlanPath,
		ActivationFinalAuditPath:  artifacts.activationFinalAuditPath,
		ActivationCIReportPath:    artifacts.activationCIReportPath,
		ActivationReleaseGatePath: artifacts.activationReleaseGatePath,
	})
	if err != nil {
		t.Fatalf("InspectProviderSecretReadProposal() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("inspect = %#v, want failed on activation_release_gate_sha256 mismatch", inspect)
	}
}

func TestProviderSecretReadProposalFailsWhenSecretReadAllowedNow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.secretReadProposalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"secret_read_allowed_now": false`), []byte(`"secret_read_allowed_now": true`), 1)
	if err := os.WriteFile(artifacts.secretReadProposalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.ProviderRealTransportImplementationPlan(retrievalcontext.ProviderRealTransportImplementationPlanOptions{
		SecretReadProposalPath:   artifacts.secretReadProposalPath,
		ProviderAdapterPlanPath:  artifacts.adapterPlanPath,
		ActivationFinalAuditPath: artifacts.activationFinalAuditPath,
		OperatorReviewBundlePath: artifacts.operatorReviewBundlePath,
		KillSwitchPlanPath:       artifacts.killSwitchPlanPath,
		OutputPath:               "transport-plan-should-fail.json",
	})
	if err != nil {
		t.Fatalf("ProviderRealTransportImplementationPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed || result.RealTransportImplementationPlanReady {
		t.Fatalf("transport plan = %#v, want failed when secret_read_allowed_now is true", result)
	}
}

func TestProviderSecretReadProposalFailsWhenSecretValuesRead(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.secretReadProposalPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"secret_values_read": false`), []byte(`"secret_values_read": true`), 1)
	if err := os.WriteFile(artifacts.secretReadProposalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	inspect, err := retrievalcontext.InspectProviderSecretReadProposal(artifacts.secretReadProposalPath, retrievalcontext.ProviderSecretReadProposalInspectOptions{
		CredentialPolicyPlanPath:  artifacts.credentialPolicyPlanPath,
		ActivationFinalAuditPath:  artifacts.activationFinalAuditPath,
		ActivationCIReportPath:    artifacts.activationCIReportPath,
		ActivationReleaseGatePath: artifacts.activationReleaseGatePath,
	})
	if err != nil {
		t.Fatalf("InspectProviderSecretReadProposal() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusFailed {
		t.Fatal("inspect must fail when secret_values_read is true")
	}
}

func TestProviderRealTransportPlanFailsWhenTransportCalled(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.realTransportImplementationPlanPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"transport_called": false`), []byte(`"transport_called": true`), 1)
	if err := os.WriteFile(artifacts.realTransportImplementationPlanPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := retrievalcontext.ProviderRealTransportImplementationReport(retrievalcontext.ProviderRealTransportImplementationReportOptions{
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		ProviderAdapterPlanPath:             artifacts.adapterPlanPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealTransportImplementationReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatal("report must fail when transport_called is true")
	}
}

func TestProviderRealDispatchDesignFailsWhenExecuteSubcommandRegistered(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.realDispatchDesignPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"execute_subcommand_registered": false`), []byte(`"execute_subcommand_registered": true`), 1)
	if err := os.WriteFile(artifacts.realDispatchDesignPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := retrievalcontext.ProviderRealDispatchDesignReport(retrievalcontext.ProviderRealDispatchDesignReportOptions{
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		ProviderRequestEnvelopePath:         artifacts.requestEnvelopePath,
		ProviderRealCallProposalPath:        artifacts.realCallProposalPath,
	})
	if err != nil {
		t.Fatalf("ProviderRealDispatchDesignReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatal("report must fail when execute_subcommand_registered is true")
	}
}

func TestProviderRealActivationDesignReviewGateFailsWhenKillSwitchInactive(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.killSwitchPlanPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"global_disabled": true`), []byte(`"global_disabled": false`), 1)
	if err := os.WriteFile(artifacts.killSwitchPlanPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	gate, err := retrievalcontext.ProviderRealActivationDesignReviewGate(retrievalcontext.ProviderRealActivationDesignReviewGateOptions{
		DesignReviewPackagePath:             artifacts.designReviewPackagePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealActivationDesignReviewGate() error = %v", err)
	}
	if gate.Status != lancedbpolicy.StatusFailed || gate.RealActivationDesignGateReady {
		t.Fatalf("gate = %#v, want failed when kill switch inactive", gate)
	}
}

func TestProviderRealActivationDesignReviewGateFailsWhenActivationAllowedNow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderRealDispatchDesignChain(t, chain)

	data, err := os.ReadFile(artifacts.designReviewPackagePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"activation_allowed_now": false`), []byte(`"activation_allowed_now": true`), 1)
	if err := os.WriteFile(artifacts.designReviewPackagePath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	gate, err := retrievalcontext.ProviderRealActivationDesignReviewGate(retrievalcontext.ProviderRealActivationDesignReviewGateOptions{
		DesignReviewPackagePath:             artifacts.designReviewPackagePath,
		SecretReadProposalPath:              artifacts.secretReadProposalPath,
		RealTransportImplementationPlanPath: artifacts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              artifacts.realDispatchDesignPath,
		ActivationFinalAuditPath:            artifacts.activationFinalAuditPath,
		ActivationCIReportPath:              artifacts.activationCIReportPath,
		KillSwitchPlanPath:                  artifacts.killSwitchPlanPath,
		OperatorReviewBundlePath:            artifacts.operatorReviewBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderRealActivationDesignReviewGate() error = %v", err)
	}
	if gate.Status != lancedbpolicy.StatusFailed {
		t.Fatal("gate must fail when activation_allowed_now is true")
	}
}
