package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validProviderAdaptersConfigYAML = `provider_adapters:
  enabled: false
  blocked_reason: implementation_not_enabled
  adapters:
    codex:
      enabled: false
      provider: codex
      adapter_available_for_future: true
      adapter_enabled_now: false
      transport_enabled: false
      allow_provider_call: false
      allow_network: false
      blocked_reason: implementation_not_enabled
`

type providerExecutionSimulationArtifacts struct {
	providerExecutorSprintArtifacts
	releaseGatePath      string
	requestEnvelopePath  string
	adapterConfigPath    string
	adapterPlanPath      string
	responseFixturePath  string
	simulationBundlePath string
}

func runProviderExecutionSimulationChain(t *testing.T, chain providerCallChainFixture) providerExecutionSimulationArtifacts {
	t.Helper()
	sprint := runProviderExecutorSprintChainWithReleaseGate(t, chain)

	const adapterConfigPath = "provider-adapters.yaml"
	if err := os.WriteFile(adapterConfigPath, []byte(validProviderAdaptersConfigYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(adapter config) error = %v", err)
	}

	const requestEnvelopePath = "provider-request-envelope.json"
	envelope, err := retrievalcontext.ProviderRequestEnvelopeDryRun(retrievalcontext.ProviderRequestEnvelopeDryRunOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutionBundlePath:   chain.executionBundlePath,
		ExecutorPreflightPath: sprint.preflightPath,
		DispatchApprovalPath:  sprint.dispatchApprovalPath,
		TransportPlanPath:     sprint.transportPlanPath,
		ReleaseBundlePath:     sprint.releaseBundlePath,
		ReleaseGatePath:       sprint.releaseGatePath,
		OutputPath:            requestEnvelopePath,
	})
	if err != nil {
		t.Fatalf("ProviderRequestEnvelopeDryRun() error = %v", err)
	}
	assertProviderRequestEnvelopeBlocked(t, envelope)
	assertNoTextExcerpt(t, requestEnvelopePath)

	report, err := retrievalcontext.ProviderRequestEnvelopeReport(retrievalcontext.ProviderRequestEnvelopeReportOptions{
		RequestEnvelopePath: requestEnvelopePath, ExecutorConfigPath: chain.executorConfigPath,
		ExecutionBundlePath: chain.executionBundlePath, ExecutorPreflightPath: sprint.preflightPath,
		DispatchApprovalPath: sprint.dispatchApprovalPath, TransportPlanPath: sprint.transportPlanPath,
		ReleaseBundlePath: sprint.releaseBundlePath, ReleaseGatePath: sprint.releaseGatePath,
	})
	if err != nil {
		t.Fatalf("ProviderRequestEnvelopeReport() error = %v", err)
	}
	if !report.ProviderRequestReady {
		t.Fatalf("envelope report provider_request_ready = false, failures=%#v", report.Failures)
	}

	cfg, err := retrievalcontext.LoadProviderAdaptersConfig(adapterConfigPath)
	if err != nil {
		t.Fatalf("LoadProviderAdaptersConfig() error = %v", err)
	}
	validateResult, err := retrievalcontext.ProviderAdapterRegistryValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderAdapterRegistryValidate() error = %v", err)
	}
	if !validateResult.AdapterRegistryValidated {
		t.Fatalf("adapter_registry_validated = false, failures=%#v", validateResult.Failures)
	}

	const adapterPlanPath = "provider-adapter-plan.json"
	adapterPlan, err := retrievalcontext.ProviderAdapterPlan(retrievalcontext.ProviderAdapterPlanOptions{
		ConfigPath: adapterConfigPath, RequestEnvelopePath: requestEnvelopePath, OutputPath: adapterPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderAdapterPlan() error = %v", err)
	}
	assertProviderAdapterPlanBlocked(t, adapterPlan)
	assertNoTextExcerpt(t, adapterPlanPath)

	const responseFixturePath = "provider-response-fixture.json"
	responseFixture, err := retrievalcontext.ProviderResponseFixtureGenerate(retrievalcontext.ProviderResponseFixtureGenerateOptions{
		RequestEnvelopePath: requestEnvelopePath, AdapterPlanPath: adapterPlanPath, OutputPath: responseFixturePath,
	})
	if err != nil {
		t.Fatalf("ProviderResponseFixtureGenerate() error = %v", err)
	}
	assertProviderResponseFixtureBlocked(t, responseFixture)
	assertNoTextExcerpt(t, responseFixturePath)

	inspect, err := retrievalcontext.ProviderResponseFixtureInspect(responseFixturePath, retrievalcontext.ProviderResponseFixtureInspectOptions{
		RequestEnvelopePath: requestEnvelopePath, AdapterPlanPath: adapterPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderResponseFixtureInspect() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("response fixture inspect status = %q, failures=%#v", inspect.Status, inspect.Failures)
	}

	const simulationBundlePath = "provider-execution-simulation-bundle.json"
	simulationBundle, err := retrievalcontext.ProviderExecutionSimulationBundle(retrievalcontext.ProviderExecutionSimulationBundleOptions{
		RequestEnvelopePath: requestEnvelopePath, AdapterPlanPath: adapterPlanPath,
		ResponseFixturePath: responseFixturePath, ReleaseGatePath: sprint.releaseGatePath,
		ExecutorConfigPath: chain.executorConfigPath, OutputPath: simulationBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderExecutionSimulationBundle() error = %v", err)
	}
	assertProviderExecutionSimulationBundleBlocked(t, simulationBundle)
	assertNoTextExcerpt(t, simulationBundlePath)

	simulationReport, err := retrievalcontext.ProviderExecutionSimulationReport(retrievalcontext.ProviderExecutionSimulationReportOptions{
		SimulationBundlePath: simulationBundlePath, RequestEnvelopePath: requestEnvelopePath,
		AdapterPlanPath: adapterPlanPath, ResponseFixturePath: responseFixturePath,
		ReleaseGatePath: sprint.releaseGatePath, ExecutorConfigPath: chain.executorConfigPath,
	})
	if err != nil {
		t.Fatalf("ProviderExecutionSimulationReport() error = %v", err)
	}
	assertProviderExecutionSimulationBundleBlocked(t, simulationReport)

	return providerExecutionSimulationArtifacts{
		providerExecutorSprintArtifacts: sprint,
		releaseGatePath:                 sprint.releaseGatePath,
		requestEnvelopePath:             requestEnvelopePath,
		adapterConfigPath:               adapterConfigPath,
		adapterPlanPath:                 adapterPlanPath,
		responseFixturePath:             responseFixturePath,
		simulationBundlePath:            simulationBundlePath,
	}
}

func runProviderExecutorSprintChainWithReleaseGate(t *testing.T, chain providerCallChainFixture) providerExecutorSprintArtifacts {
	t.Helper()
	sprint := runProviderExecutorSprintChain(t, chain)

	releaseGate, err := retrievalcontext.ProviderExecutorReleaseGate(retrievalcontext.ProviderExecutorReleaseGateOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutionBundlePath:   chain.executionBundlePath,
		DryRunPath:            sprint.dryRunPath,
		DryRunReportPath:      sprint.dryRunReportPath,
		ExecutorPreflightPath: sprint.preflightPath,
		DispatchApprovalPath:  sprint.dispatchApprovalPath,
		TransportPlanPath:     sprint.transportPlanPath,
		ReleaseBundlePath:     sprint.releaseBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderExecutorReleaseGate() error = %v", err)
	}
	assertProviderExecutorReleaseGateBlocked(t, releaseGate)

	const releaseGatePath = "provider-executor-release-gate.json"
	var releaseGateBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderExecutorReleaseGateJSON(releaseGate, &releaseGateBuf); err != nil {
		t.Fatalf("WriteProviderExecutorReleaseGateJSON() error = %v", err)
	}
	if err := os.WriteFile(releaseGatePath, releaseGateBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(release gate) error = %v", err)
	}
	assertNoTextExcerpt(t, releaseGatePath)

	sprint.releaseGatePath = releaseGatePath
	return sprint
}

func assertProviderRequestEnvelopeBlocked(t *testing.T, result retrievalcontext.ProviderRequestEnvelopeResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("envelope status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ProviderRequestReady {
		t.Fatal("provider_request_ready = false, want true")
	}
	if result.RequestContainsText || result.SentToProvider || result.ProviderCall || result.NetworkCall || result.TransportCalled || result.WorkerExecution {
		t.Fatalf("envelope flags must stay blocked: %#v", result)
	}
}

func assertProviderAdapterPlanBlocked(t *testing.T, result retrievalcontext.ProviderAdapterPlanResult) {
	t.Helper()
	if !result.AdapterPlanReady || !result.AdapterRegistryValidated {
		t.Fatalf("adapter plan ready=%t registry=%t, failures=%#v", result.AdapterPlanReady, result.AdapterRegistryValidated, result.Failures)
	}
	if !result.AdapterAvailableForFuture || result.AdapterEnabledNow || result.TransportEnabled || result.ProviderCall || result.NetworkCall {
		t.Fatalf("adapter plan flags must stay blocked: %#v", result)
	}
	if result.Provider != retrievalcontext.ProviderCallExecutorProvider {
		t.Fatalf("provider = %q", result.Provider)
	}
}

func assertProviderResponseFixtureBlocked(t *testing.T, result retrievalcontext.ProviderResponseFixtureResult) {
	t.Helper()
	if !result.ResponseFixtureReady {
		t.Fatal("response_fixture_ready = false, want true")
	}
	if result.ResponseSource != retrievalcontext.ProviderResponseFixtureSource {
		t.Fatalf("response_source = %q", result.ResponseSource)
	}
	if result.ProviderCall || result.NetworkCall || result.TransportCalled || result.SentToProvider || result.ReceivedFromProvider || result.WorkerExecution {
		t.Fatalf("response fixture flags must stay blocked: %#v", result)
	}
}

func assertProviderExecutionSimulationBundleBlocked(t *testing.T, result retrievalcontext.ProviderExecutionSimulationBundleResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("simulation status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.SimulationBundleReady || !result.SimulatedResponseReady {
		t.Fatalf("simulation_bundle_ready=%t simulated_response_ready=%t", result.SimulationBundleReady, result.SimulatedResponseReady)
	}
	if result.ExecutionResultAvailable || result.ProviderCall || result.NetworkCall || result.TransportCalled || result.SentToProvider || result.ReceivedFromProvider || result.WorkerExecution {
		t.Fatalf("simulation flags must stay blocked: %#v", result)
	}
}

func TestProviderRequestEnvelopeDryRunSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	runProviderExecutionSimulationChain(t, chain)
}

func TestProviderAdapterRegistryValidateFailsWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := bytes.Replace([]byte(validProviderAdaptersConfigYAML), []byte("enabled: false"), []byte("enabled: true"), 1)
	path := "provider-adapters-bad.yaml"
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderAdaptersConfig(path)
	if err != nil {
		t.Fatalf("LoadProviderAdaptersConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderAdapterRegistryValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderAdapterRegistryValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestProviderResponseFixtureInspectFailsOnTamperedID(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderExecutionSimulationChain(t, chain)

	data, err := os.ReadFile(artifacts.responseFixturePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	patched := bytes.Replace(data, []byte(`"response_fixture_id": "fixture-`), []byte(`"response_fixture_id": "tampered-`), 1)
	if err := os.WriteFile(artifacts.responseFixturePath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	inspect, err := retrievalcontext.ProviderResponseFixtureInspect(artifacts.responseFixturePath, retrievalcontext.ProviderResponseFixtureInspectOptions{
		RequestEnvelopePath: artifacts.requestEnvelopePath, AdapterPlanPath: artifacts.adapterPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderResponseFixtureInspect() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusFailed {
		t.Fatal("inspect status must be failed after tamper")
	}
}
