package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedInjectionExecutionGateOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ExecutionGateReady {
		t.Fatalf("result = %#v, want execution_gate_ready", result)
	}
	if result.ImplementationAllowsExecutionNow || result.WorkerExecutionAllowed || result.PromptInjectionAllowedNow {
		t.Fatalf("result = %#v, want execution/injection blocked", result)
	}
	if !result.ConfirmFlagUsed {
		t.Fatalf("result = %#v, want confirm_flag_used", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedInjectionExecutionGateBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedInjectionExecutionGateBlockedReason)
	}
	if result.MaterializedSHA256 == "" || result.PromptOutputSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("materialized-injection-execution-gate.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("execution gate output leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionExecutionGateText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionExecutionGateText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("execution gate text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionExecutionGateJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionExecutionGateJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("execution gate json leaked preview content")
	}
}

func TestMaterializedInjectionExecutionGateRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)

	_, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:            taskPath,
		ReadinessReportPath: readinessPath,
		PromptOutputPath:    promptOutputPath,
		OutputPath:          "materialized-injection-execution-gate.json",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedInjectionExecutionGateFailsWhenReadinessFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	readiness := readJSONFile(t, readinessPath)
	readiness["status"] = lancedbpolicy.StatusFailed
	readiness["failures"] = []string{"forced failure"}
	writeJSONFile(t, readinessPath, readiness)

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.ExecutionGateReady {
		t.Fatal("execution_gate_ready must be false when readiness failed")
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "readiness status must be ok or warning") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionGateFailsWhenGovernanceReadyFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	readiness := readJSONFile(t, readinessPath)
	readiness["governance_ready_for_future_execution"] = false
	writeJSONFile(t, readinessPath, readiness)

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "governance_ready_for_future_execution must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionGateFailsWhenExecutionAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	readiness := readJSONFile(t, readinessPath)
	readiness["execution_allowed_now"] = true
	writeJSONFile(t, readinessPath, readiness)

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "execution_allowed_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionGateFailsWhenPromptInjectionAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	readiness := readJSONFile(t, readinessPath)
	readiness["prompt_injection_allowed_now"] = true
	writeJSONFile(t, readinessPath, readiness)

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_injection_allowed_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionGateFailsWhenWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	readiness := readJSONFile(t, readinessPath)
	readiness["worker_execution"] = true
	writeJSONFile(t, readinessPath, readiness)

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionGateFailsOnPromptOutputSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	if err := os.WriteFile(promptOutputPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionGateRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)

	_, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         "/tmp/task.yaml",
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err == nil {
		t.Fatal("expected blocked task path error")
	}

	_, err = retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              "/tmp/readiness.json",
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err == nil {
		t.Fatal("expected blocked readiness path error")
	}

	_, err = retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 "/tmp/prompt-section.md",
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-execution-gate.json",
	})
	if err == nil {
		t.Fatal("expected blocked prompt output path error")
	}
}

func setupMaterializedInjectionExecutionGateArtifacts(t *testing.T) (string, string, string) {
	t.Helper()
	preflightPath, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)
	readiness, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    preflightPath,
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionReadinessReport() error = %v", err)
	}
	const readinessPath = "retrieval-context-materialized-injection-readiness-report.json"
	var buf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionReadinessReportJSON(readiness, &buf); err != nil {
		t.Fatalf("WriteMaterializedInjectionReadinessReportJSON() error = %v", err)
	}
	if err := os.WriteFile(readinessPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	taskData, err := os.ReadFile("materialized-injection-task.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	taskPath := "execution-gate-materialized-injection-task.yaml"
	if err := os.WriteFile(taskPath, taskData, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	const promptOutputPath = "retrieval-context-materialized-injection-prompt-section.md"
	if _, err := os.Stat(promptOutputPath); err != nil {
		data, err := os.ReadFile("materialized-injection-prompt-section.md")
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		if err := os.WriteFile(promptOutputPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	return taskPath, readinessPath, promptOutputPath
}
