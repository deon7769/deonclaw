package retrievalcontext_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func providerCallExecutorOpts(chain providerCallChainFixture) retrievalcontext.ProviderCallExecutorOptions {
	return retrievalcontext.ProviderCallExecutorOptions{
		ExecutorConfigPath:   chain.executorConfigPath,
		DispatchConfigPath:   chain.dispatchPath,
		ProviderRunPlanPath:  chain.runPlanPath,
		PayloadDryRunPath:    chain.dryRunPath,
		PayloadOutputPath:    chain.payloadOutputPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessPath,
		ApprovalRequestPath:  chain.approvalRequestPath,
		ApprovalPath:         chain.approvalPath,
		ExecutionBundlePath:  chain.executionBundlePath,
	}
}

func assertProviderCallExecutorBlocked(t *testing.T, result retrievalcontext.ProviderCallExecutorResult) {
	t.Helper()
	if !result.ExecutorPolicyValidated {
		t.Fatal("executor_policy_validated = false, want true")
	}
	if !result.ExecutorConfigValidated {
		t.Fatalf("executor_config_validated = false, want true; failures=%#v", result.Failures)
	}
	if result.ExecutionSupportedNow {
		t.Fatal("execution_supported_now = true, want false")
	}
	if !result.ProviderCallAuthorizedForFuture {
		t.Fatal("provider_call_authorized_for_future = false, want true")
	}
	if !result.ChainContinuityReady {
		t.Fatal("chain_continuity_ready = false, want true")
	}
	if result.ProviderCallAllowedNow || result.SentToProvider {
		t.Fatalf("provider_call_allowed_now=%t sent_to_provider=%t, want both false", result.ProviderCallAllowedNow, result.SentToProvider)
	}
	assertProviderCallChainBlockedFlags(t, "provider call executor", result.ProviderCall, result.NetworkCall, result.WorkerExecution, result.SentToProvider, result.PromptInjectionRealRunner)
	if result.BlockedReason != retrievalcontext.MaterializedProviderDispatchBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedProviderDispatchBlockedReason)
	}
	if result.ExecutionBundleSHA256 == "" || result.ApprovalSHA256 == "" || result.ProviderPayloadSHA256 == "" {
		t.Fatalf("executor hashes missing: bundle=%q approval=%q payload=%q", result.ExecutionBundleSHA256, result.ApprovalSHA256, result.ProviderPayloadSHA256)
	}
	if result.ExecutorConfigSHA256 == "" {
		t.Fatal("executor_config_sha256 must be set")
	}
}

func TestProviderCallExecutorValidateSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	result, err := retrievalcontext.ProviderCallExecutorValidate(providerCallExecutorOpts(chain))
	if err != nil {
		t.Fatalf("ProviderCallExecutorValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want ok or warning", result.Status)
	}
	assertProviderCallExecutorBlocked(t, result)
}

func TestProviderCallExecutorPlanSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	const planPath = "provider-call-executor-plan.json"
	result, err := retrievalcontext.ProviderCallExecutorPlan(retrievalcontext.ProviderCallExecutorPlanOptions{
		ProviderCallExecutorOptions: providerCallExecutorOpts(chain),
		OutputPath:                  planPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorPlan() error = %v", err)
	}
	assertProviderCallExecutorBlocked(t, result)
	if len(result.PlanSteps) == 0 {
		t.Fatal("plan_steps must be populated")
	}
	planData, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("ReadFile(plan) error = %v", err)
	}
	assertNoTextExcerpt(t, planPath)
	var buf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallExecutorJSON(result, &buf); err != nil {
		t.Fatalf("WriteProviderCallExecutorJSON() error = %v", err)
	}
	if bytes.Contains(planData, []byte("text_excerpt")) {
		t.Fatal("plan artifact must not contain text_excerpt")
	}
}

func TestProviderCallExecutorValidateFailsWhenBundleUnauthorized(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	bundleData, err := os.ReadFile(chain.executionBundlePath)
	if err != nil {
		t.Fatalf("ReadFile(bundle) error = %v", err)
	}
	patched := bytes.Replace(bundleData, []byte(`"provider_call_authorized_for_future": true`), []byte(`"provider_call_authorized_for_future": false`), 1)
	if err := os.WriteFile(chain.executionBundlePath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile(bundle) error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutorValidate(providerCallExecutorOpts(chain))
	if err != nil {
		t.Fatalf("ProviderCallExecutorValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.ExecutorConfigValidated {
		t.Fatal("executor_config_validated must be false when bundle unauthorized")
	}
}

func TestLoadProviderCallExecutionBundle(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	bundle, data, err := retrievalcontext.LoadProviderCallExecutionBundle(chain.executionBundlePath)
	if err != nil {
		t.Fatalf("LoadProviderCallExecutionBundle() error = %v", err)
	}
	if !bundle.ProviderCallAuthorizedForFuture {
		t.Fatal("bundle provider_call_authorized_for_future must be true")
	}
	if len(data) == 0 {
		t.Fatal("bundle raw bytes must be non-empty")
	}
}

func TestProviderCallExecutorValidateFailsWhenPolicyDisabled(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	content := strings.Replace(validProviderCallExecutorConfigYAML, "enabled: false", "enabled: true", 1)
	if err := os.WriteFile(chain.executorConfigPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(executor config) error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutorValidate(providerCallExecutorOpts(chain))
	if err != nil {
		t.Fatalf("ProviderCallExecutorValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.ExecutorConfigValidated || result.ExecutorPolicyValidated {
		t.Fatal("executor validation flags must be false when policy config is invalid")
	}
}

func TestBlockedProviderTransportNeverDelivers(t *testing.T) {
	var transport retrievalcontext.BlockedProviderTransport
	resp, err := transport.Deliver(context.Background(), retrievalcontext.ProviderTransportRequest{})
	if err == nil {
		t.Fatal("Deliver() error = nil, want ErrProviderTransportNotEnabled")
	}
	if resp.Delivered {
		t.Fatal("Delivered = true, want false")
	}
}
