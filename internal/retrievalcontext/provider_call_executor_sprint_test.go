package retrievalcontext_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type providerExecutorSprintArtifacts struct {
	dryRunPath           string
	dryRunReportPath     string
	preflightPath        string
	dispatchRequestPath  string
	dispatchApprovalPath string
	transportPlanPath    string
	releaseBundlePath    string
	releaseGatePath      string
}

func runProviderExecutorSprintChain(t *testing.T, chain providerCallChainFixture) providerExecutorSprintArtifacts {
	t.Helper()

	const dryRunPath = "provider-call-executor-dry-run.json"
	if _, err := retrievalcontext.ProviderCallExecutorDryRun(retrievalcontext.ProviderCallExecutorDryRunOptions{
		ProviderCallExecutorOptions: providerCallExecutorDryRunOpts(chain),
		OutputPath:                  dryRunPath,
	}); err != nil {
		t.Fatalf("ProviderCallExecutorDryRun() error = %v", err)
	}
	assertNoTextExcerpt(t, dryRunPath)

	dryRunReport, err := retrievalcontext.ProviderCallExecutorDryRunReport(retrievalcontext.ProviderCallExecutorDryRunReportOptions{
		DryRunPath:         dryRunPath,
		ExecutorConfigPath: chain.executorConfigPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorDryRunReport() error = %v", err)
	}
	if !dryRunReport.ExecutorDryRunValidated {
		t.Fatalf("executor_dry_run_validated = false, failures=%#v", dryRunReport.Failures)
	}

	const dryRunReportPath = "provider-call-executor-dry-run-report.json"
	var dryRunReportBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallExecutorDryRunReportJSON(dryRunReport, &dryRunReportBuf); err != nil {
		t.Fatalf("WriteProviderCallExecutorDryRunReportJSON() error = %v", err)
	}
	if err := os.WriteFile(dryRunReportPath, dryRunReportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(dry-run report) error = %v", err)
	}
	assertNoTextExcerpt(t, dryRunReportPath)

	const preflightPath = "provider-call-executor-preflight.json"
	preflight, err := retrievalcontext.ProviderCallExecutorPreflight(retrievalcontext.ProviderCallExecutorPreflightOptions{
		ExecutorConfigPath:  chain.executorConfigPath,
		DryRunPath:          dryRunPath,
		DryRunReportPath:    dryRunReportPath,
		ExecutionBundlePath: chain.executionBundlePath,
		ApprovalPath:        chain.approvalPath,
		OutputPath:          preflightPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorPreflight() error = %v", err)
	}
	assertProviderExecutorPreflightBlocked(t, preflight)
	assertNoTextExcerpt(t, preflightPath)

	const dispatchRequestPath = "provider-call-executor-dispatch-approval-request.json"
	request, err := retrievalcontext.NewProviderCallExecutorDispatchApprovalRequest(retrievalcontext.NewProviderCallExecutorDispatchApprovalRequestOptions{
		ExecutorConfigPath:  chain.executorConfigPath,
		DryRunPath:          dryRunPath,
		DryRunReportPath:    dryRunReportPath,
		PreflightPath:       preflightPath,
		ExecutionBundlePath: chain.executionBundlePath,
		ApprovalPath:        chain.approvalPath,
		OutputPath:          dispatchRequestPath,
	})
	if err != nil {
		t.Fatalf("NewProviderCallExecutorDispatchApprovalRequest() error = %v", err)
	}
	assertNoTextExcerpt(t, dispatchRequestPath)

	const dispatchApprovalPath = "provider-call-executor-dispatch-approval.json"
	dispatchApproval, err := retrievalcontext.ApproveProviderCallExecutorDispatch(retrievalcontext.ApproveProviderCallExecutorDispatchOptions{
		RequestPath:                  dispatchRequestPath,
		OutputPath:                   dispatchApprovalPath,
		ConfirmExecutorConfigSHA256:  request.ExecutorConfigSHA256,
		ConfirmExecutionBundleSHA256: request.ExecutionBundleSHA256,
		ConfirmProviderPayloadSHA256: request.ProviderPayloadSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderCallExecutorDispatch() error = %v", err)
	}
	assertProviderExecutorDispatchApprovalBlocked(t, dispatchApproval)
	assertNoTextExcerpt(t, dispatchApprovalPath)

	const transportPlanPath = "provider-transport-plan.json"
	transportPlan, err := retrievalcontext.ProviderTransportPlan(retrievalcontext.ProviderTransportPlanOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutorPreflightPath: preflightPath,
		DispatchApprovalPath:  dispatchApprovalPath,
		OutputPath:            transportPlanPath,
	})
	if err != nil {
		t.Fatalf("ProviderTransportPlan() error = %v", err)
	}
	assertProviderTransportPlanBlocked(t, transportPlan)
	assertNoTextExcerpt(t, transportPlanPath)

	const releaseBundlePath = "provider-executor-release-bundle.json"
	releaseBundle, err := retrievalcontext.ProviderExecutorReleaseBundle(retrievalcontext.ProviderExecutorReleaseBundleOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutionBundlePath:   chain.executionBundlePath,
		DryRunPath:            dryRunPath,
		DryRunReportPath:      dryRunReportPath,
		ExecutorPreflightPath: preflightPath,
		DispatchApprovalPath:  dispatchApprovalPath,
		TransportPlanPath:     transportPlanPath,
		OutputPath:            releaseBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderExecutorReleaseBundle() error = %v", err)
	}
	assertProviderExecutorReleaseBundleBlocked(t, releaseBundle)
	assertNoTextExcerpt(t, releaseBundlePath)

	releaseGate, err := retrievalcontext.ProviderExecutorReleaseGate(retrievalcontext.ProviderExecutorReleaseGateOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutionBundlePath:   chain.executionBundlePath,
		DryRunPath:            dryRunPath,
		DryRunReportPath:      dryRunReportPath,
		ExecutorPreflightPath: preflightPath,
		DispatchApprovalPath:  dispatchApprovalPath,
		TransportPlanPath:     transportPlanPath,
		ReleaseBundlePath:     releaseBundlePath,
	})
	if err != nil {
		t.Fatalf("ProviderExecutorReleaseGate() error = %v", err)
	}
	assertProviderExecutorReleaseGateBlocked(t, releaseGate)

	assertProviderExecutorSprintHashCoherence(t, chain, preflight, dispatchApproval, transportPlan, releaseBundle)

	return providerExecutorSprintArtifacts{
		dryRunPath:           dryRunPath,
		dryRunReportPath:     dryRunReportPath,
		preflightPath:        preflightPath,
		dispatchRequestPath:  dispatchRequestPath,
		dispatchApprovalPath: dispatchApprovalPath,
		transportPlanPath:    transportPlanPath,
		releaseBundlePath:    releaseBundlePath,
	}
}

func assertProviderExecutorPreflightBlocked(t *testing.T, result retrievalcontext.ProviderCallExecutorPreflightResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("preflight status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ExecutorPreflightReady {
		t.Fatal("executor_preflight_ready = false, want true")
	}
	if result.ExecutionAllowedNow || result.TransportCalled || result.ProviderCall || result.NetworkCall || result.WorkerExecution || result.SentToProvider || result.PromptInjectionRealRunner {
		t.Fatalf("preflight execution flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.ProviderCallExecutorBlockedReason)
	}
	if result.RequiredFutureConfirmFlag != retrievalcontext.RequiredFutureConfirmProviderExecutorDispatch {
		t.Fatalf("required_future_confirm_flag = %q", result.RequiredFutureConfirmFlag)
	}
}

func assertProviderExecutorDispatchApprovalBlocked(t *testing.T, approval retrievalcontext.ProviderCallExecutorDispatchApproval) {
	t.Helper()
	if !approval.DispatchAuthorizedForFuture {
		t.Fatal("dispatch_authorized_for_future = false, want true")
	}
	if approval.DispatchAllowedNow || approval.ProviderCall || approval.NetworkCall || approval.TransportCalled || approval.SentToProvider {
		t.Fatalf("dispatch approval execution flags must stay blocked: %#v", approval)
	}
}

func assertProviderTransportPlanBlocked(t *testing.T, result retrievalcontext.ProviderTransportPlanResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("transport plan status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.TransportPlanReady {
		t.Fatal("transport_plan_ready = false, want true")
	}
	if result.TransportEnabled || result.TransportCalled || result.ProviderCall || result.NetworkCall || result.SentToProvider {
		t.Fatalf("transport plan flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
	if result.TransportMode != "BlockedProviderTransport" {
		t.Fatalf("transport_mode = %q, want BlockedProviderTransport", result.TransportMode)
	}
}

func assertProviderExecutorReleaseBundleBlocked(t *testing.T, result retrievalcontext.ProviderExecutorReleaseBundleResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("release bundle status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ReleaseBundleReady {
		t.Fatal("release_bundle_ready = false, want true")
	}
	if result.ActivationAllowedNow || result.ExecutionSupportedNow || result.ProviderCall || result.NetworkCall || result.WorkerExecution || result.TransportCalled || result.SentToProvider {
		t.Fatalf("release bundle flags must stay blocked: %#v", result)
	}
}

func assertProviderExecutorReleaseGateBlocked(t *testing.T, result retrievalcontext.ProviderExecutorReleaseGateResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("release gate status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ActivationGateReady || !result.ReleaseBundleReady {
		t.Fatalf("activation_gate_ready=%t release_bundle_ready=%t, want both true", result.ActivationGateReady, result.ReleaseBundleReady)
	}
	if result.ActivationAllowedNow || result.ExecutionSupportedNow || result.ProviderCall || result.NetworkCall || result.WorkerExecution || result.TransportCalled || result.SentToProvider {
		t.Fatalf("release gate flags must stay blocked: %#v", result)
	}
}

func assertProviderExecutorSprintHashCoherence(t *testing.T, chain providerCallChainFixture, preflight retrievalcontext.ProviderCallExecutorPreflightResult, dispatch retrievalcontext.ProviderCallExecutorDispatchApproval, transport retrievalcontext.ProviderTransportPlanResult, bundle retrievalcontext.ProviderExecutorReleaseBundleResult) {
	t.Helper()
	if preflight.ProviderPayloadSHA256 != chain.payloadSHA256 {
		t.Fatalf("preflight provider_payload_sha256 = %q, want %q", preflight.ProviderPayloadSHA256, chain.payloadSHA256)
	}
	if dispatch.ProviderPayloadSHA256 != chain.payloadSHA256 {
		t.Fatalf("dispatch approval provider_payload_sha256 = %q, want %q", dispatch.ProviderPayloadSHA256, chain.payloadSHA256)
	}
	if bundle.ProviderPayloadSHA256 != chain.payloadSHA256 {
		t.Fatalf("release bundle provider_payload_sha256 = %q, want %q", bundle.ProviderPayloadSHA256, chain.payloadSHA256)
	}
	if transport.ExecutorConfigSHA256 != preflight.ExecutorConfigSHA256 {
		t.Fatal("transport plan executor_config_sha256 mismatch with preflight")
	}
	if dispatch.PreflightSHA256 != "" {
		preflightData, err := os.ReadFile("provider-call-executor-preflight.json")
		if err != nil {
			t.Fatalf("ReadFile(preflight) error = %v", err)
		}
		sum := sha256.Sum256(preflightData)
		if dispatch.PreflightSHA256 != hex.EncodeToString(sum[:]) {
			t.Fatal("dispatch approval preflight_sha256 mismatch")
		}
	}
}

func TestProviderCallExecutorPreflightSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	runProviderExecutorSprintChain(t, chain)
}

func TestProviderCallExecutorDispatchApprovalInspectSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderExecutorSprintChain(t, chain)

	inspect, err := retrievalcontext.InspectProviderCallExecutorDispatchApproval(artifacts.dispatchApprovalPath, retrievalcontext.InspectProviderCallExecutorDispatchApprovalOptions{
		RequestPath: artifacts.dispatchRequestPath,
	})
	if err != nil {
		t.Fatalf("InspectProviderCallExecutorDispatchApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect status = %q, failures=%#v", inspect.Status, inspect.Failures)
	}
	if !inspect.DispatchAuthorizedForFuture || inspect.DispatchAllowedNow {
		t.Fatalf("inspect dispatch flags invalid: %#v", inspect)
	}
}

func TestProviderTransportPlanFailsWhenDispatchUnauthorized(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderExecutorSprintChain(t, chain)

	approvalData, err := os.ReadFile(artifacts.dispatchApprovalPath)
	if err != nil {
		t.Fatalf("ReadFile(dispatch approval) error = %v", err)
	}
	patched := bytes.Replace(approvalData, []byte(`"dispatch_authorized_for_future": true`), []byte(`"dispatch_authorized_for_future": false`), 1)
	if err := os.WriteFile(artifacts.dispatchApprovalPath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile(dispatch approval) error = %v", err)
	}

	_, err = retrievalcontext.ProviderTransportPlan(retrievalcontext.ProviderTransportPlanOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutorPreflightPath: artifacts.preflightPath,
		DispatchApprovalPath:  artifacts.dispatchApprovalPath,
		OutputPath:            "provider-transport-plan-bad.json",
	})
	if err == nil {
		t.Fatal("ProviderTransportPlan() error = nil, want validation failure")
	}
}

func TestProviderExecutorReleaseGateFailsWhenBundleMissing(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderExecutorSprintChain(t, chain)

	_, err := retrievalcontext.ProviderExecutorReleaseGate(retrievalcontext.ProviderExecutorReleaseGateOptions{
		ExecutorConfigPath:    chain.executorConfigPath,
		ExecutionBundlePath:   chain.executionBundlePath,
		DryRunPath:            artifacts.dryRunPath,
		DryRunReportPath:      artifacts.dryRunReportPath,
		ExecutorPreflightPath: artifacts.preflightPath,
		DispatchApprovalPath:  artifacts.dispatchApprovalPath,
		TransportPlanPath:     artifacts.transportPlanPath,
		ReleaseBundlePath:     "missing-release-bundle.json",
	})
	if err == nil {
		t.Fatal("ProviderExecutorReleaseGate() error = nil, want read failure")
	}
}
