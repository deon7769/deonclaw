package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestMaterializedInjectionPreflightOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, reportPath := setupMaterializedInjectionPreflightArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                taskPath,
		PromptPreviewReportPath: reportPath,
		OutputPath:              "materialized-injection-preflight.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionPreflight() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if result.WorkerExecutionAllowed || result.PromptInjectionAllowedNow {
		t.Fatalf("result = %#v, want execution and injection disabled", result)
	}
	if !result.MaterializedInjectionDeclared || result.GovernanceBundleSHA256 == "" || result.PromptPreviewReportSHA256 == "" || result.MaterializedSHA256 == "" {
		t.Fatalf("result = %#v, want populated preflight metadata", result)
	}
	if result.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q", result.RequiredFutureFlag)
	}

	outputData, err := os.ReadFile("materialized-injection-preflight.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("preflight output leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionPreflightText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionPreflightText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("preflight text leaked preview content")
	}
}

func TestMaterializedInjectionPreflightFailsWhenTaskEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, reportPath := setupMaterializedInjectionPreflightArtifacts(t)
	taskData, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := strings.Replace(string(taskData), "enabled: false", "enabled: true", 1)
	if err := os.WriteFile(taskPath, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                taskPath,
		PromptPreviewReportPath: reportPath,
		OutputPath:              "materialized-injection-preflight.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionPreflight() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), tasks.MaterializedInjectionNotSupportedYet) {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionPreflightFailsOnInvalidBundle(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, reportPath := setupMaterializedInjectionPreflightArtifacts(t)
	if err := os.WriteFile("injection-governance-bundle.json", []byte(`{"status":"failed","contains_text":true,"runner_execution":true,"injection_authorized_for_future":false,"execution_supported_now":true}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                taskPath,
		PromptPreviewReportPath: reportPath,
		OutputPath:              "materialized-injection-preflight.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionPreflight() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "contains_text must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionPreflightFailsOnPromptPreviewReportFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, reportPath := setupMaterializedInjectionPreflightArtifacts(t)
	report := readJSONFile(t, reportPath)
	report["status"] = lancedbpolicy.StatusFailed
	writeJSONFile(t, reportPath, report)

	result, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                taskPath,
		PromptPreviewReportPath: reportPath,
		OutputPath:              "materialized-injection-preflight.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionPreflight() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt preview report status") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionPreflightFailsOnMaterializedSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, reportPath := setupMaterializedInjectionPreflightArtifacts(t)
	report := readJSONFile(t, reportPath)
	hashes, _ := report["hashes"].(map[string]any)
	hashes["materialized_sha256"] = strings.Repeat("0", 64)
	report["hashes"] = hashes
	writeJSONFile(t, reportPath, report)

	result, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                taskPath,
		PromptPreviewReportPath: reportPath,
		OutputPath:              "materialized-injection-preflight.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionPreflight() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "materialized_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedInjectionPreflightArtifacts(t *testing.T) (string, string) {
	t.Helper()
	setupInjectionGovernanceBundleArtifacts(t)
	if _, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts()); err != nil {
		t.Fatalf("InjectionGovernanceBundle() error = %v", err)
	}

	report, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	const reportPath = "retrieval-context-prompt-preview-report.json"
	var reportBuf bytes.Buffer
	if err := retrievalcontext.WritePromptPreviewReportJSON(report, &reportBuf); err != nil {
		t.Fatalf("WritePromptPreviewReportJSON() error = %v", err)
	}
	if err := os.WriteFile(reportPath, reportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	task := validMaterializedInjectionTask("injection-governance-bundle.json", "retrieval-context-prompt-preview.md")
	const taskPath = "materialized-injection-task.yaml"
	taskYAML := `id: retrieval-context-materialized-injection-001
title: "Materialized injection declaration"
domain: general
worker: codex
goal: "Declare future governed materialized injection without runner execution."
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
retrieval_context:
  materialized_injection:
    enabled: false
    governance_bundle: ` + task.RetrievalContext.MaterializedInjection.GovernanceBundle + `
    prompt_preview: ` + task.RetrievalContext.MaterializedInjection.PromptPreview + `
    require_confirm_flag: true
    max_total_chars: 6000
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - Task declares materialized injection schema only
`
	if err := os.WriteFile(taskPath, []byte(taskYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return taskPath, reportPath
}
