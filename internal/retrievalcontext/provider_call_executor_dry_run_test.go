package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func providerCallExecutorDryRunOpts(chain providerCallChainFixture) retrievalcontext.ProviderCallExecutorOptions {
	return retrievalcontext.ProviderCallExecutorOptions{
		ExecutorConfigPath:   chain.executorConfigPath,
		DispatchConfigPath:   chain.dispatchPath,
		ProviderRunPlanPath:  chain.runPlanPath,
		PayloadDryRunPath:    chain.dryRunPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessPath,
		ApprovalRequestPath:  chain.approvalRequestPath,
		ApprovalPath:         chain.approvalPath,
		ExecutionBundlePath:  chain.executionBundlePath,
	}
}

func assertProviderCallExecutorDryRunBlocked(t *testing.T, result retrievalcontext.ProviderCallExecutorDryRunResult) {
	t.Helper()
	if !result.ExecutorDryRunReady {
		t.Fatalf("executor_dry_run_ready = false, want true; failures=%#v", result.Failures)
	}
	if !result.ExecutorPolicyValidated || !result.ExecutorConfigValidated {
		t.Fatalf("executor policy/config validation false: %#v", result)
	}
	if result.TransportCalled {
		t.Fatal("transport_called must be false")
	}
	if result.ContainsText {
		t.Fatal("contains_text must be false")
	}
	if !result.PreviewOnly {
		t.Fatal("preview_only must be true")
	}
	assertProviderCallChainBlockedFlags(t, "provider call executor dry-run", result.ProviderCall, result.NetworkCall, result.WorkerExecution, result.SentToProvider, result.PromptInjectionRealRunner)
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.ProviderCallExecutorBlockedReason)
	}
}

func TestProviderCallExecutorDryRunSuccess(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	const dryRunPath = "provider-call-executor-dry-run.json"
	result, err := retrievalcontext.ProviderCallExecutorDryRun(retrievalcontext.ProviderCallExecutorDryRunOptions{
		ProviderCallExecutorOptions: providerCallExecutorDryRunOpts(chain),
		OutputPath:                  dryRunPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorDryRun() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want ok or warning", result.Status)
	}
	assertProviderCallExecutorDryRunBlocked(t, result)
	assertNoTextExcerpt(t, dryRunPath)

	report, err := retrievalcontext.ProviderCallExecutorDryRunReport(retrievalcontext.ProviderCallExecutorDryRunReportOptions{
		DryRunPath:         dryRunPath,
		ExecutorConfigPath: chain.executorConfigPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorDryRunReport() error = %v", err)
	}
	if !report.ExecutorDryRunValidated {
		t.Fatalf("executor_dry_run_validated = false, failures=%#v", report.Failures)
	}
	if report.TransportCalled || report.ContainsText {
		t.Fatalf("report = %#v, want transport_called and contains_text false", report)
	}

	var buf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallExecutorDryRunReportJSON(report, &buf); err != nil {
		t.Fatalf("WriteProviderCallExecutorDryRunReportJSON() error = %v", err)
	}
	if bytes.Contains(buf.Bytes(), []byte("text_excerpt")) {
		t.Fatal("dry-run report json must not contain text_excerpt")
	}
}

func TestProviderCallExecutorDryRunFailsWhenPolicyDisabled(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	content := bytes.Replace([]byte(validProviderCallExecutorConfigYAML), []byte("enabled: false"), []byte("enabled: true"), 1)
	if err := os.WriteFile(chain.executorConfigPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutorDryRun(retrievalcontext.ProviderCallExecutorDryRunOptions{
		ProviderCallExecutorOptions: providerCallExecutorDryRunOpts(chain),
		OutputPath:                  "provider-call-executor-dry-run.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorDryRun() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.ExecutorDryRunReady {
		t.Fatal("executor_dry_run_ready must be false when policy invalid")
	}
}
