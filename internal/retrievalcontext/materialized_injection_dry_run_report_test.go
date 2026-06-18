package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedInjectionDryRunReportOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if result.WorkerExecution || result.PromptChangedInRealRunner {
		t.Fatalf("result = %#v, want no worker execution or prompt change", result)
	}
	if !result.PromptSectionRendered || !result.ContainsText || !result.PreviewOnly {
		t.Fatalf("result = %#v, want rendered preview-only metadata", result)
	}
	if result.PromptOutputSHA256 == "" || result.MaterializedSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionDryRunReportText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionDryRunReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionDryRunReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionDryRunReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("report json leaked preview content")
	}
}

func TestMaterializedInjectionDryRunReportFailsOnPromptOutputSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)
	if err := os.WriteFile(promptOutputPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionDryRunReportFailsWhenWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)
	dryRun := readJSONFile(t, dryRunPath)
	dryRun["worker_execution"] = true
	writeJSONFile(t, dryRunPath, dryRun)

	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionDryRunReportFailsWhenPromptChangedInRealRunnerTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)
	dryRun := readJSONFile(t, dryRunPath)
	dryRun["prompt_changed_in_real_runner"] = true
	writeJSONFile(t, dryRunPath, dryRun)

	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_changed_in_real_runner must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionDryRunReportFailsWhenPromptOutputMissingDryRunNotice(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)
	data, err := os.ReadFile(promptOutputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := strings.Replace(string(data), retrievalcontext.MaterializedInjectionDryRunNotice, "missing notice", 1)
	if err := os.WriteFile(promptOutputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "missing dry-run notice") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionDryRunReportFailsWhenPromptOutputMissingTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)
	if err := os.WriteFile(promptOutputPath, []byte(retrievalcontext.MaterializedInjectionDryRunNotice+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	dryRun := readJSONFile(t, dryRunPath)
	dryRun["prompt_output_sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
	writeJSONFile(t, dryRunPath, dryRun)

	result, err := retrievalcontext.MaterializedInjectionDryRunReport(retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: promptOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRunReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "missing text_excerpt") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionDryRunReportRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, promptOutputPath := setupMaterializedInjectionDryRunReportArtifacts(t)

	opts := retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       "/tmp/materialized-injection-dry-run.json",
		PromptOutputPath: promptOutputPath,
	}
	_, err := retrievalcontext.MaterializedInjectionDryRunReport(opts)
	if err == nil {
		t.Fatal("expected blocked dry-run path error")
	}

	opts = retrievalcontext.MaterializedInjectionDryRunReportOptions{
		DryRunPath:       dryRunPath,
		PromptOutputPath: "/tmp/materialized-injection-prompt-section.md",
	}
	_, err = retrievalcontext.MaterializedInjectionDryRunReport(opts)
	if err == nil {
		t.Fatal("expected blocked prompt output path error")
	}
}

func setupMaterializedInjectionDryRunReportArtifacts(t *testing.T) (string, string) {
	t.Helper()
	taskPath, preflightPath, previewPath := setupMaterializedInjectionDryRunArtifacts(t)
	const dryRunPath = "materialized-injection-dry-run.json"
	const promptOutputPath = "materialized-injection-prompt-section.md"
	if _, err := retrievalcontext.MaterializedInjectionDryRun(retrievalcontext.MaterializedInjectionDryRunOptions{
		TaskPath:                         taskPath,
		PreflightPath:                    preflightPath,
		PromptPreviewPath:                previewPath,
		OutputPath:                       dryRunPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
	}); err != nil {
		t.Fatalf("MaterializedInjectionDryRun() error = %v", err)
	}
	return dryRunPath, promptOutputPath
}
