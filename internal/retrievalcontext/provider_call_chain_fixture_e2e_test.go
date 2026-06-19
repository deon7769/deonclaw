package retrievalcontext_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type providerCallChainFixture struct {
	fullProviderCallChain
	executionBundlePath string
	executorConfigPath  string
}

func setupProviderCallChainFixture(t *testing.T) providerCallChainFixture {
	t.Helper()
	chain := setupFullProviderCallChain(t)

	const executionBundlePath = "provider-call-execution-bundle.json"
	if _, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath:   chain.dispatchPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessPath,
		ApprovalPath:         chain.approvalPath,
		OutputPath:           executionBundlePath,
	}); err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}

	const executorConfigPath = "provider-call-executor.yaml"
	if err := os.WriteFile(executorConfigPath, []byte(validProviderCallExecutorConfigYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(executor config) error = %v", err)
	}

	return providerCallChainFixture{
		fullProviderCallChain: chain,
		executionBundlePath:   executionBundlePath,
		executorConfigPath:    executorConfigPath,
	}
}

func runProviderCallChainContinuityAudit(t *testing.T, chain providerCallChainFixture) retrievalcontext.ProviderCallChainContinuityAuditResult {
	t.Helper()
	result, err := retrievalcontext.ProviderCallChainContinuityAudit(retrievalcontext.ProviderCallChainContinuityAuditOptions{
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
	})
	if err != nil {
		t.Fatalf("ProviderCallChainContinuityAudit() error = %v", err)
	}
	return result
}

func assertProviderCallChainAuditReady(t *testing.T, result retrievalcontext.ProviderCallChainContinuityAuditResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("audit status = %q, want ok or warning; failures=%#v", result.Status, result.Failures)
	}
	if !result.ChainContinuityReady {
		t.Fatalf("chain_continuity_ready = false, want true")
	}
	if !result.ProducerCommandsActive {
		t.Fatalf("producer_commands_active = false, want true")
	}
	if !result.LoadersReconciled {
		t.Fatalf("loaders_reconciled = false, want true")
	}
	if result.ProviderCall || result.NetworkCall || result.WorkerExecution || result.SentToProvider {
		t.Fatalf("audit = %#v, want all execution flags false", result)
	}
	if result.ProviderPayloadSHA256 == "" {
		t.Fatal("provider_payload_sha256 must be set on audit result")
	}
}

func assertProviderCallChainBlockedFlags(t *testing.T, label string, providerCall, networkCall, workerExecution, sentToProvider, promptInjectionRealRunner bool) {
	t.Helper()
	if providerCall || networkCall || workerExecution || sentToProvider || promptInjectionRealRunner {
		t.Fatalf("%s flags provider_call=%t network_call=%t worker_execution=%t sent_to_provider=%t prompt_injection_real_runner=%t, want all false",
			label, providerCall, networkCall, workerExecution, sentToProvider, promptInjectionRealRunner)
	}
}

func assertProviderCallChainHashCoherence(t *testing.T, chain providerCallChainFixture) {
	t.Helper()
	payloadData, err := os.ReadFile(chain.payloadOutputPath)
	if err != nil {
		t.Fatalf("ReadFile(payload output) error = %v", err)
	}
	sum := sha256.Sum256(payloadData)
	want := hex.EncodeToString(sum[:])
	if want != chain.payloadSHA256 {
		t.Fatalf("payload file sha256 = %q, want %q", want, chain.payloadSHA256)
	}

	type hashCarrier struct {
		ProviderPayloadSHA256 string `json:"provider_payload_sha256"`
	}
	paths := []string{
		chain.dryRunPath,
		chain.payloadReportPath,
		chain.gatePath,
		chain.readinessPath,
		chain.approvalRequestPath,
		chain.approvalPath,
		chain.executionBundlePath,
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", path, err)
		}
		var carrier hashCarrier
		if err := json.Unmarshal(data, &carrier); err != nil {
			t.Fatalf("Unmarshal(%q) error = %v", path, err)
		}
		if carrier.ProviderPayloadSHA256 == "" {
			continue
		}
		if carrier.ProviderPayloadSHA256 != chain.payloadSHA256 {
			t.Fatalf("%s provider_payload_sha256 = %q, want %q", path, carrier.ProviderPayloadSHA256, chain.payloadSHA256)
		}
	}
}

func assertProviderCallChainMetadataNoTextLeak(t *testing.T, chain providerCallChainFixture, auditJSON []byte) {
	t.Helper()
	metadataPaths := []string{
		chain.runPlanPath,
		chain.dryRunPath,
		chain.payloadReportPath,
		chain.gatePath,
		chain.readinessPath,
		chain.approvalRequestPath,
		chain.approvalPath,
		chain.executionBundlePath,
	}
	for _, path := range metadataPaths {
		assertNoTextExcerpt(t, path)
	}
	if strings.Contains(string(auditJSON), "text_excerpt") || strings.Contains(string(auditJSON), "alpha text") {
		t.Fatal("chain audit json must not contain materialized preview text")
	}
	payloadData, err := os.ReadFile(chain.payloadOutputPath)
	if err != nil {
		t.Fatalf("ReadFile(payload output) error = %v", err)
	}
	if !strings.Contains(string(payloadData), "text_excerpt") {
		t.Fatal("payload output must contain text_excerpt for fixture validation")
	}
}

func TestProviderCallChainFixtureSmokeE2E(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)

	dryRun, _, err := retrievalcontext.LoadMaterializedProviderPayload(chain.dryRunPath)
	if err != nil {
		t.Fatalf("LoadMaterializedProviderPayload() error = %v", err)
	}
	assertProviderCallChainBlockedFlags(t, "payload dry-run", dryRun.ProviderCall, dryRun.NetworkCall, dryRun.WorkerExecution, dryRun.SentToProvider, dryRun.PromptInjectionRealRunner)

	report, _, err := retrievalcontext.LoadMaterializedProviderPayloadReport(chain.payloadReportPath)
	if err != nil {
		t.Fatalf("LoadMaterializedProviderPayloadReport() error = %v", err)
	}
	assertProviderCallChainBlockedFlags(t, "payload report", report.ProviderCall, report.NetworkCall, report.WorkerExecution, report.SentToProvider, report.PromptInjectionRealRunner)
	if !report.ProviderPayloadValidated {
		t.Fatal("provider_payload_validated must be true")
	}

	bundleData, err := os.ReadFile(chain.executionBundlePath)
	if err != nil {
		t.Fatalf("ReadFile(execution bundle) error = %v", err)
	}
	var bundle retrievalcontext.ProviderCallExecutionBundleResult
	if err := json.Unmarshal(bundleData, &bundle); err != nil {
		t.Fatalf("Unmarshal(execution bundle) error = %v", err)
	}
	if !bundle.ProviderCallAuthorizedForFuture {
		t.Fatal("provider_call_authorized_for_future must be true on execution bundle")
	}
	assertProviderCallChainBlockedFlags(t, "execution bundle", bundle.ProviderCall, bundle.NetworkCall, bundle.WorkerExecution, bundle.SentToProvider, false)

	assertProviderCallChainHashCoherence(t, chain)

	result := runProviderCallChainContinuityAudit(t, chain)
	assertProviderCallChainAuditReady(t, result)

	var auditBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallChainContinuityAuditJSON(result, &auditBuf); err != nil {
		t.Fatalf("WriteProviderCallChainContinuityAuditJSON() error = %v", err)
	}
	if err := os.WriteFile("provider-call-chain-audit.json", auditBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(audit) error = %v", err)
	}
	assertProviderCallChainMetadataNoTextLeak(t, chain, auditBuf.Bytes())

	cfg, err := retrievalcontext.LoadProviderCallExecutorConfig(chain.executorConfigPath)
	if err != nil {
		t.Fatalf("LoadProviderCallExecutorConfig() error = %v", err)
	}
	configResult, err := retrievalcontext.ProviderCallExecutorConfigValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderCallExecutorConfigValidate() error = %v", err)
	}
	if configResult.Status != lancedbpolicy.StatusOK {
		t.Fatalf("executor config status = %q, want ok", configResult.Status)
	}

	validateResult, err := retrievalcontext.ProviderCallExecutorValidate(providerCallExecutorOpts(chain))
	if err != nil {
		t.Fatalf("ProviderCallExecutorValidate() error = %v", err)
	}
	assertProviderCallExecutorBlocked(t, validateResult)

	const planPath = "provider-call-executor-plan.json"
	planResult, err := retrievalcontext.ProviderCallExecutorPlan(retrievalcontext.ProviderCallExecutorPlanOptions{
		ProviderCallExecutorOptions: providerCallExecutorOpts(chain),
		OutputPath:                  planPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorPlan() error = %v", err)
	}
	assertProviderCallExecutorBlocked(t, planResult)
	assertNoTextExcerpt(t, planPath)

	const dryRunPath = "provider-call-executor-dry-run.json"
	dryRunResult, err := retrievalcontext.ProviderCallExecutorDryRun(retrievalcontext.ProviderCallExecutorDryRunOptions{
		ProviderCallExecutorOptions: providerCallExecutorDryRunOpts(chain),
		OutputPath:                  dryRunPath,
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutorDryRun() error = %v", err)
	}
	assertProviderCallExecutorDryRunBlocked(t, dryRunResult)
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

	runProviderActivationReadinessChain(t, chain)
}

func TestProviderCallChainFixtureCLISmokeE2E(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	chdir(t, dir)

	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	if err := os.Rename(dispatchPath, "materialized-provider-dispatch.yaml"); err != nil {
		t.Fatalf("Rename(dispatch) error = %v", err)
	}
	dispatchPath = "materialized-provider-dispatch.yaml"

	deonctl := buildDeonctlBinaryAt(t, root)

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "materialized-provider-dispatch", "validate",
		"--config", dispatchPath,
		"--output-format", "json",
	})
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "materialized-provider-payload-dry-run",
		"--dispatch-config", dispatchPath,
		"--provider-run-plan", runPlanPath,
		"--assembled-output", assembledPath,
		"--output", "materialized-provider-payload-dry-run.json",
		"--payload-output", "materialized-provider-payload.md",
		"--confirm-inject-materialized-context",
		"--output-format", "json",
	})

	var reportStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "materialized-provider-payload-report",
		"--payload-dry-run", "materialized-provider-payload-dry-run.json",
		"--payload-output", "materialized-provider-payload.md",
		"--output-format", "json",
	}, &reportStdout)
	if err := os.WriteFile("materialized-provider-payload-report.json", reportStdout.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(payload report) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "materialized-provider-call-gate",
		"--dispatch-config", dispatchPath,
		"--payload-report", "materialized-provider-payload-report.json",
		"--payload-output", "materialized-provider-payload.md",
		"--confirm-inject-materialized-context",
		"--output", "materialized-provider-call-gate.json",
		"--output-format", "json",
	})

	var readinessStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "materialized-provider-call-readiness-report",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--payload-report", "materialized-provider-payload-report.json",
		"--output-format", "json",
	}, &readinessStdout)
	if err := os.WriteFile("materialized-provider-call-readiness-report.json", readinessStdout.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(readiness report) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-approval", "new",
		"--readiness-report", "materialized-provider-call-readiness-report.json",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--payload-report", "materialized-provider-payload-report.json",
		"--output", "provider-call-approval-request.json",
	})

	var payloadReport struct {
		ProviderPayloadSHA256 string `json:"provider_payload_sha256"`
	}
	if err := json.Unmarshal(reportStdout.Bytes(), &payloadReport); err != nil {
		t.Fatalf("Unmarshal(payload report) error = %v", err)
	}
	if payloadReport.ProviderPayloadSHA256 == "" {
		t.Fatal("payload report provider_payload_sha256 is required")
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-approval", "approve",
		"--request", "provider-call-approval-request.json",
		"--output", "provider-call-approval.json",
		"--confirm-provider-payload-sha256", payloadReport.ProviderPayloadSHA256,
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-execution-bundle",
		"--dispatch-config", dispatchPath,
		"--payload-report", "materialized-provider-payload-report.json",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--readiness-report", "materialized-provider-call-readiness-report.json",
		"--approval", "provider-call-approval.json",
		"--output", "provider-call-execution-bundle.json",
		"--output-format", "json",
	})

	var auditStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-chain-audit",
		"--dispatch-config", dispatchPath,
		"--provider-run-plan", runPlanPath,
		"--payload-dry-run", "materialized-provider-payload-dry-run.json",
		"--payload-output", "materialized-provider-payload.md",
		"--payload-report", "materialized-provider-payload-report.json",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--readiness-report", "materialized-provider-call-readiness-report.json",
		"--approval-request", "provider-call-approval-request.json",
		"--approval", "provider-call-approval.json",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--output-format", "json",
	}, &auditStdout)

	var audit retrievalcontext.ProviderCallChainContinuityAuditResult
	if err := json.Unmarshal(auditStdout.Bytes(), &audit); err != nil {
		t.Fatalf("Unmarshal(audit) error = %v", err)
	}
	assertProviderCallChainAuditReady(t, audit)

	chain := providerCallChainFixture{
		fullProviderCallChain: fullProviderCallChain{
			dispatchPath:        dispatchPath,
			runPlanPath:         runPlanPath,
			dryRunPath:          "materialized-provider-payload-dry-run.json",
			payloadOutputPath:   "materialized-provider-payload.md",
			payloadReportPath:   "materialized-provider-payload-report.json",
			gatePath:            "materialized-provider-call-gate.json",
			readinessPath:       "materialized-provider-call-readiness-report.json",
			approvalRequestPath: "provider-call-approval-request.json",
			approvalPath:        "provider-call-approval.json",
			payloadSHA256:       payloadReport.ProviderPayloadSHA256,
		},
		executionBundlePath: "provider-call-execution-bundle.json",
	}
	assertProviderCallChainHashCoherence(t, chain)
	assertProviderCallChainMetadataNoTextLeak(t, chain, auditStdout.Bytes())

	if err := os.WriteFile("provider-call-executor.yaml", []byte(validProviderCallExecutorConfigYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(executor config) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor", "config", "validate",
		"--config", "provider-call-executor.yaml",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor", "validate",
		"--executor-config", "provider-call-executor.yaml",
		"--dispatch-config", dispatchPath,
		"--provider-run-plan", runPlanPath,
		"--payload-dry-run", "materialized-provider-payload-dry-run.json",
		"--payload-output", "materialized-provider-payload.md",
		"--payload-report", "materialized-provider-payload-report.json",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--readiness-report", "materialized-provider-call-readiness-report.json",
		"--approval-request", "provider-call-approval-request.json",
		"--approval", "provider-call-approval.json",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--output-format", "json",
	})

	var executorStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor", "plan",
		"--executor-config", "provider-call-executor.yaml",
		"--dispatch-config", dispatchPath,
		"--provider-run-plan", runPlanPath,
		"--payload-dry-run", "materialized-provider-payload-dry-run.json",
		"--payload-output", "materialized-provider-payload.md",
		"--payload-report", "materialized-provider-payload-report.json",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--readiness-report", "materialized-provider-call-readiness-report.json",
		"--approval-request", "provider-call-approval-request.json",
		"--approval", "provider-call-approval.json",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--output", "provider-call-executor-plan.json",
		"--output-format", "json",
	}, &executorStdout)

	var executor retrievalcontext.ProviderCallExecutorResult
	if err := json.Unmarshal(executorStdout.Bytes(), &executor); err != nil {
		t.Fatalf("Unmarshal(executor plan) error = %v", err)
	}
	assertProviderCallExecutorBlocked(t, executor)
	assertNoTextExcerpt(t, "provider-call-executor-plan.json")

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor", "dry-run",
		"--executor-config", "provider-call-executor.yaml",
		"--dispatch-config", dispatchPath,
		"--provider-run-plan", runPlanPath,
		"--payload-dry-run", "materialized-provider-payload-dry-run.json",
		"--payload-report", "materialized-provider-payload-report.json",
		"--provider-call-gate", "materialized-provider-call-gate.json",
		"--readiness-report", "materialized-provider-call-readiness-report.json",
		"--approval-request", "provider-call-approval-request.json",
		"--approval", "provider-call-approval.json",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--output", "provider-call-executor-dry-run.json",
		"--output-format", "json",
	})

	var dryRunReportStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor", "dry-run-report",
		"--dry-run", "provider-call-executor-dry-run.json",
		"--executor-config", "provider-call-executor.yaml",
		"--output-format", "json",
	}, &dryRunReportStdout)

	var dryRunReport retrievalcontext.ProviderCallExecutorDryRunReportResult
	if err := json.Unmarshal(dryRunReportStdout.Bytes(), &dryRunReport); err != nil {
		t.Fatalf("Unmarshal(dry-run report) error = %v", err)
	}
	if !dryRunReport.ExecutorDryRunValidated {
		t.Fatalf("executor_dry_run_validated = false, failures=%#v", dryRunReport.Failures)
	}
	assertNoTextExcerpt(t, "provider-call-executor-dry-run.json")

	var dryRunReportBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallExecutorDryRunReportJSON(dryRunReport, &dryRunReportBuf); err != nil {
		t.Fatalf("WriteProviderCallExecutorDryRunReportJSON() error = %v", err)
	}
	if err := os.WriteFile("provider-call-executor-dry-run-report.json", dryRunReportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(dry-run report) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor", "preflight",
		"--executor-config", "provider-call-executor.yaml",
		"--dry-run", "provider-call-executor-dry-run.json",
		"--dry-run-report", "provider-call-executor-dry-run-report.json",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--approval", "provider-call-approval.json",
		"--output", "provider-call-executor-preflight.json",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor-dispatch-approval", "new",
		"--executor-config", "provider-call-executor.yaml",
		"--dry-run", "provider-call-executor-dry-run.json",
		"--dry-run-report", "provider-call-executor-dry-run-report.json",
		"--preflight", "provider-call-executor-preflight.json",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--approval", "provider-call-approval.json",
		"--output", "provider-call-executor-dispatch-approval-request.json",
	})

	var preflight retrievalcontext.ProviderCallExecutorPreflightResult
	preflightData, err := os.ReadFile("provider-call-executor-preflight.json")
	if err != nil {
		t.Fatalf("ReadFile(preflight) error = %v", err)
	}
	if err := json.Unmarshal(preflightData, &preflight); err != nil {
		t.Fatalf("Unmarshal(preflight) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor-dispatch-approval", "approve",
		"--request", "provider-call-executor-dispatch-approval-request.json",
		"--output", "provider-call-executor-dispatch-approval.json",
		"--confirm-executor-config-sha256", preflight.ExecutorConfigSHA256,
		"--confirm-execution-bundle-sha256", preflight.ExecutionBundleSHA256,
		"--confirm-provider-payload-sha256", preflight.ProviderPayloadSHA256,
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-call-executor-dispatch-approval", "inspect",
		"--approval", "provider-call-executor-dispatch-approval.json",
		"--request", "provider-call-executor-dispatch-approval-request.json",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-transport-plan",
		"--executor-config", "provider-call-executor.yaml",
		"--executor-preflight", "provider-call-executor-preflight.json",
		"--dispatch-approval", "provider-call-executor-dispatch-approval.json",
		"--output", "provider-transport-plan.json",
		"--output-format", "json",
	})

	var releaseBundleStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-executor-release-bundle",
		"--executor-config", "provider-call-executor.yaml",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--dry-run", "provider-call-executor-dry-run.json",
		"--dry-run-report", "provider-call-executor-dry-run-report.json",
		"--executor-preflight", "provider-call-executor-preflight.json",
		"--dispatch-approval", "provider-call-executor-dispatch-approval.json",
		"--transport-plan", "provider-transport-plan.json",
		"--output", "provider-executor-release-bundle.json",
		"--output-format", "json",
	}, &releaseBundleStdout)

	var releaseGateStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-executor-release-gate",
		"--executor-config", "provider-call-executor.yaml",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--dry-run", "provider-call-executor-dry-run.json",
		"--dry-run-report", "provider-call-executor-dry-run-report.json",
		"--executor-preflight", "provider-call-executor-preflight.json",
		"--dispatch-approval", "provider-call-executor-dispatch-approval.json",
		"--transport-plan", "provider-transport-plan.json",
		"--release-bundle", "provider-executor-release-bundle.json",
		"--output-format", "json",
	}, &releaseGateStdout)

	var releaseGate retrievalcontext.ProviderExecutorReleaseGateResult
	if err := json.Unmarshal(releaseGateStdout.Bytes(), &releaseGate); err != nil {
		t.Fatalf("Unmarshal(release gate) error = %v", err)
	}
	assertProviderExecutorReleaseGateBlocked(t, releaseGate)
	if err := os.WriteFile("provider-executor-release-gate.json", releaseGateStdout.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(release gate) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-request-envelope", "dry-run",
		"--executor-config", "provider-call-executor.yaml",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--executor-preflight", "provider-call-executor-preflight.json",
		"--dispatch-approval", "provider-call-executor-dispatch-approval.json",
		"--transport-plan", "provider-transport-plan.json",
		"--release-bundle", "provider-executor-release-bundle.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--output", "provider-request-envelope.json",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-request-envelope", "report",
		"--request-envelope", "provider-request-envelope.json",
		"--executor-config", "provider-call-executor.yaml",
		"--execution-bundle", "provider-call-execution-bundle.json",
		"--executor-preflight", "provider-call-executor-preflight.json",
		"--dispatch-approval", "provider-call-executor-dispatch-approval.json",
		"--transport-plan", "provider-transport-plan.json",
		"--release-bundle", "provider-executor-release-bundle.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--output-format", "json",
	})

	if err := os.WriteFile("provider-adapters.yaml", []byte(validProviderAdaptersConfigYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(provider adapters) error = %v", err)
	}

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-adapter-registry", "validate",
		"--config", "provider-adapters.yaml",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-adapter-plan",
		"--config", "provider-adapters.yaml",
		"--request-envelope", "provider-request-envelope.json",
		"--output", "provider-adapter-plan.json",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-response-fixture", "generate",
		"--request-envelope", "provider-request-envelope.json",
		"--adapter-plan", "provider-adapter-plan.json",
		"--output", "provider-response-fixture.json",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-response-fixture", "inspect",
		"--fixture", "provider-response-fixture.json",
		"--request-envelope", "provider-request-envelope.json",
		"--adapter-plan", "provider-adapter-plan.json",
		"--output-format", "json",
	})

	var simulationBundleStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-execution-simulation-bundle",
		"--request-envelope", "provider-request-envelope.json",
		"--adapter-plan", "provider-adapter-plan.json",
		"--response-fixture", "provider-response-fixture.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--executor-config", "provider-call-executor.yaml",
		"--output", "provider-execution-simulation-bundle.json",
		"--output-format", "json",
	}, &simulationBundleStdout)

	var simulationReportStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-execution-simulation-report",
		"--simulation-bundle", "provider-execution-simulation-bundle.json",
		"--request-envelope", "provider-request-envelope.json",
		"--adapter-plan", "provider-adapter-plan.json",
		"--response-fixture", "provider-response-fixture.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--executor-config", "provider-call-executor.yaml",
		"--output-format", "json",
	}, &simulationReportStdout)

	var simulationReport retrievalcontext.ProviderExecutionSimulationBundleResult
	if err := json.Unmarshal(simulationReportStdout.Bytes(), &simulationReport); err != nil {
		t.Fatalf("Unmarshal(simulation report) error = %v", err)
	}
	assertProviderExecutionSimulationBundleBlocked(t, simulationReport)

	if err := os.WriteFile("provider-execution-simulation-report.json", simulationReportStdout.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile(simulation report) error = %v", err)
	}

	if err := os.WriteFile("provider-credential-policy.yaml", []byte(validProviderCredentialPolicyConfigYAML), 0o644); err != nil {
		t.Fatalf("WriteFile(credential policy) error = %v", err)
	}

	var credentialValidateStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-credential-policy", "validate",
		"--config", "provider-credential-policy.yaml",
		"--output-format", "json",
	}, &credentialValidateStdout)

	var credentialValidate retrievalcontext.ProviderCredentialPolicyValidateResult
	if err := json.Unmarshal(credentialValidateStdout.Bytes(), &credentialValidate); err != nil {
		t.Fatalf("Unmarshal(credential validate) error = %v", err)
	}
	assertProviderCredentialPolicyBlocked(t, credentialValidate)

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-credential-policy", "plan",
		"--config", "provider-credential-policy.yaml",
		"--output", "provider-credential-policy-plan.json",
		"--output-format", "json",
	})

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-real-call-proposal", "new",
		"--execution-simulation-report", "provider-execution-simulation-report.json",
		"--request-envelope", "provider-request-envelope.json",
		"--adapter-plan", "provider-adapter-plan.json",
		"--credential-policy-plan", "provider-credential-policy-plan.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--output", "provider-real-call-proposal.json",
	})

	var realCallInspectStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-real-call-proposal", "inspect",
		"--proposal", "provider-real-call-proposal.json",
		"--execution-simulation-report", "provider-execution-simulation-report.json",
		"--request-envelope", "provider-request-envelope.json",
		"--adapter-plan", "provider-adapter-plan.json",
		"--credential-policy-plan", "provider-credential-policy-plan.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--output-format", "json",
	}, &realCallInspectStdout)

	var realCallInspect retrievalcontext.ProviderRealCallProposalInspectResult
	if err := json.Unmarshal(realCallInspectStdout.Bytes(), &realCallInspect); err != nil {
		t.Fatalf("Unmarshal(real call inspect) error = %v", err)
	}
	assertProviderRealCallProposalInspectBlocked(t, realCallInspect)

	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-response-change-proposal",
		"--response-fixture", "provider-response-fixture.json",
		"--execution-simulation-report", "provider-execution-simulation-report.json",
		"--real-call-proposal", "provider-real-call-proposal.json",
		"--output", "provider-response-change-proposal.json",
		"--output-format", "json",
	})

	var changeReportStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-response-change-proposal-report",
		"--change-proposal", "provider-response-change-proposal.json",
		"--response-fixture", "provider-response-fixture.json",
		"--execution-simulation-report", "provider-execution-simulation-report.json",
		"--real-call-proposal", "provider-real-call-proposal.json",
		"--output-format", "json",
	}, &changeReportStdout)

	var changeReport retrievalcontext.ProviderResponseChangeProposalResult
	if err := json.Unmarshal(changeReportStdout.Bytes(), &changeReport); err != nil {
		t.Fatalf("Unmarshal(change report) error = %v", err)
	}
	assertProviderResponseChangeProposalBlocked(t, changeReport)

	var activationAuditStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-activation-readiness-audit",
		"--credential-policy-plan", "provider-credential-policy-plan.json",
		"--real-call-proposal", "provider-real-call-proposal.json",
		"--change-proposal", "provider-response-change-proposal.json",
		"--execution-simulation-report", "provider-execution-simulation-report.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--output-format", "json",
	}, &activationAuditStdout)

	var activationAudit retrievalcontext.ProviderActivationReadinessAuditResult
	if err := json.Unmarshal(activationAuditStdout.Bytes(), &activationAudit); err != nil {
		t.Fatalf("Unmarshal(activation audit) error = %v", err)
	}
	assertProviderActivationReadinessBlocked(t, activationAudit)

	var activationReportStdout bytes.Buffer
	mustDeonctlOK(t, deonctl, []string{
		"worker", "codex", "provider-activation-readiness-report",
		"--credential-policy-plan", "provider-credential-policy-plan.json",
		"--real-call-proposal", "provider-real-call-proposal.json",
		"--change-proposal", "provider-response-change-proposal.json",
		"--execution-simulation-report", "provider-execution-simulation-report.json",
		"--release-gate", "provider-executor-release-gate.json",
		"--output-format", "json",
	}, &activationReportStdout)

	var activationReport retrievalcontext.ProviderActivationReadinessAuditResult
	if err := json.Unmarshal(activationReportStdout.Bytes(), &activationReport); err != nil {
		t.Fatalf("Unmarshal(activation report) error = %v", err)
	}
	assertProviderActivationReadinessBlocked(t, activationReport)

	sprintPaths := []string{
		"provider-call-executor-dry-run-report.json",
		"provider-call-executor-preflight.json",
		"provider-call-executor-dispatch-approval-request.json",
		"provider-call-executor-dispatch-approval.json",
		"provider-transport-plan.json",
		"provider-executor-release-bundle.json",
		"provider-executor-release-gate.json",
		"provider-request-envelope.json",
		"provider-adapter-plan.json",
		"provider-response-fixture.json",
		"provider-execution-simulation-bundle.json",
		"provider-execution-simulation-report.json",
		"provider-credential-policy-plan.json",
		"provider-real-call-proposal.json",
		"provider-response-change-proposal.json",
	}
	for _, path := range sprintPaths {
		assertNoTextExcerpt(t, path)
	}
	if strings.Contains(releaseGateStdout.String(), "text_excerpt") || strings.Contains(releaseBundleStdout.String(), "alpha text") {
		t.Fatal("release gate/bundle stdout must not contain materialized preview text")
	}
	if strings.Contains(simulationBundleStdout.String(), "text_excerpt") || strings.Contains(simulationReportStdout.String(), "alpha text") {
		t.Fatal("simulation stdout must not contain materialized preview text")
	}
}

func buildDeonctlBinaryAt(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "deonctl")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/deonctl")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build deonctl: %v\n%s", err, out)
	}
	return bin
}

func mustDeonctlOK(t *testing.T, deonctl string, args []string, stdoutCaptures ...*bytes.Buffer) {
	t.Helper()
	cmd := exec.Command(deonctl, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if len(stdoutCaptures) > 0 {
		cmd.Stdout = stdoutCaptures[0]
	} else {
		var discard bytes.Buffer
		cmd.Stdout = &discard
	}
	if err := cmd.Run(); err != nil {
		t.Fatalf("deonctl %v failed: %v\nstderr=%q", args, err, stderr.String())
	}
}
