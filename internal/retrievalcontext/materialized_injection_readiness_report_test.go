package retrievalcontext_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedInjectionReadinessReportOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	preflightPath, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    preflightPath,
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionReadinessReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.GovernanceReadyForFutureExecution {
		t.Fatalf("result = %#v, want governance ready", result)
	}
	if result.ExecutionAllowedNow || result.PromptInjectionAllowedNow || result.WorkerExecution {
		t.Fatalf("result = %#v, want execution disabled", result)
	}
	if result.MaterializedSHA256 == "" || result.PromptOutputSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}
	if result.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q", result.RequiredFutureFlag)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionReadinessReportText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionReadinessReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("readiness report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionReadinessReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionReadinessReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("readiness report json leaked preview content")
	}
}

func TestMaterializedInjectionReadinessReportFailsWhenPreflightWorkerExecutionAllowedTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	preflightPath, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)
	preflight := readJSONFile(t, preflightPath)
	preflight["worker_execution_allowed"] = true
	writeJSONFile(t, preflightPath, preflight)

	result, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    preflightPath,
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionReadinessReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.GovernanceReadyForFutureExecution {
		t.Fatal("governance_ready_for_future_execution must be false when failed")
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "worker_execution_allowed must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionReadinessReportFailsWhenDryRunWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	preflightPath, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)
	dryRun := readJSONFile(t, dryRunPath)
	dryRun["worker_execution"] = true
	writeJSONFile(t, dryRunPath, dryRun)

	result, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    preflightPath,
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionReadinessReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "dry-run worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionReadinessReportFailsOnDryRunReportHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	preflightPath, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)
	report := readJSONFile(t, dryRunReportPath)
	report["prompt_output_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, dryRunReportPath, report)

	result, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    preflightPath,
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionReadinessReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionReadinessReportFailsOnMaterializedSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	preflightPath, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)
	dryRun := readJSONFile(t, dryRunPath)
	dryRun["materialized_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, dryRunPath, dryRun)

	result, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    preflightPath,
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionReadinessReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "materialized_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionReadinessReportRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_, dryRunPath, dryRunReportPath := setupMaterializedInjectionReadinessReportArtifacts(t)

	_, err := retrievalcontext.MaterializedInjectionReadinessReport(retrievalcontext.MaterializedInjectionReadinessReportOptions{
		PreflightPath:    "/tmp/preflight.json",
		DryRunPath:       dryRunPath,
		DryRunReportPath: dryRunReportPath,
	})
	if err == nil {
		t.Fatal("expected blocked preflight path error")
	}
}

func setupMaterializedInjectionReadinessReportArtifacts(t *testing.T) (string, string, string) {
	t.Helper()
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)
	const preflightPath = "retrieval-context-materialized-injection-preflight.json"
	const dryRunReportPath = "materialized-injection-dry-run-report.json"

	report, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(dryRunReportPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return preflightPath, dryRunPath, dryRunReportPath
}
