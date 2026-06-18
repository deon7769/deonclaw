package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedInjectionDryRunOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, preflightPath, previewPath := setupMaterializedInjectionDryRunArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionDryRun(retrievalcontext.MaterializedInjectionDryRunOptions{
		TaskPath:                         taskPath,
		PreflightPath:                    preflightPath,
		PromptPreviewPath:                previewPath,
		OutputPath:                       "materialized-injection-dry-run.json",
		PromptOutputPath:                 "materialized-injection-prompt-section.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRun() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if result.WorkerExecution || result.PromptChangedInRealRunner {
		t.Fatalf("result = %#v, want no worker execution or prompt change", result)
	}
	if !result.PromptSectionRendered || !result.ContainsText || !result.PreviewOnly || !result.ConfirmFlagUsed {
		t.Fatalf("result = %#v, want rendered preview-only dry-run metadata", result)
	}
	if result.PromptOutputSHA256 == "" || result.MaterializedSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("materialized-injection-dry-run.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("dry-run output leaked preview content")
	}

	promptOutput, err := os.ReadFile("materialized-injection-prompt-section.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	promptText := string(promptOutput)
	if !strings.Contains(promptText, "text_excerpt:") || !strings.Contains(promptText, "alpha text") {
		t.Fatal("prompt output must contain rendered text_excerpt content")
	}
	if !strings.Contains(promptText, retrievalcontext.MaterializedInjectionDryRunNotice) {
		t.Fatal("prompt output must contain dry-run notice")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionDryRunText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionDryRunText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("dry-run text leaked preview content")
	}
}

func TestMaterializedInjectionDryRunRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, preflightPath, previewPath := setupMaterializedInjectionDryRunArtifacts(t)

	_, err := retrievalcontext.MaterializedInjectionDryRun(retrievalcontext.MaterializedInjectionDryRunOptions{
		TaskPath:          taskPath,
		PreflightPath:     preflightPath,
		PromptPreviewPath: previewPath,
		OutputPath:        "materialized-injection-dry-run.json",
		PromptOutputPath:  "materialized-injection-prompt-section.md",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedInjectionDryRunFailsWhenPreflightFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, preflightPath, previewPath := setupMaterializedInjectionDryRunArtifacts(t)
	preflight := readJSONFile(t, preflightPath)
	preflight["status"] = lancedbpolicy.StatusFailed
	preflight["failures"] = []string{"forced failure"}
	writeJSONFile(t, preflightPath, preflight)

	result, err := retrievalcontext.MaterializedInjectionDryRun(retrievalcontext.MaterializedInjectionDryRunOptions{
		TaskPath:                         taskPath,
		PreflightPath:                    preflightPath,
		PromptPreviewPath:                previewPath,
		OutputPath:                       "materialized-injection-dry-run.json",
		PromptOutputPath:                 "materialized-injection-prompt-section.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRun() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "preflight status") {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if _, err := os.Stat("materialized-injection-prompt-section.md"); err == nil {
		t.Fatal("prompt output must not be written when dry-run fails")
	}
}

func TestMaterializedInjectionDryRunFailsWhenPromptPreviewMissing(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, preflightPath, previewPath := setupMaterializedInjectionDryRunArtifacts(t)
	if err := os.Remove(previewPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionDryRun(retrievalcontext.MaterializedInjectionDryRunOptions{
		TaskPath:                         taskPath,
		PreflightPath:                    preflightPath,
		PromptPreviewPath:                previewPath,
		OutputPath:                       "materialized-injection-dry-run.json",
		PromptOutputPath:                 "materialized-injection-prompt-section.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionDryRun() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "read prompt preview") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedInjectionDryRunArtifacts(t *testing.T) (string, string, string) {
	t.Helper()
	taskPath, reportPath := setupMaterializedInjectionPreflightArtifacts(t)

	previewData, err := os.ReadFile("prompt-preview.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	const previewPath = "retrieval-context-prompt-preview.md"
	if err := os.WriteFile(previewPath, previewData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	const preflightPath = "retrieval-context-materialized-injection-preflight.json"
	if _, err := retrievalcontext.MaterializedInjectionPreflight(retrievalcontext.MaterializedInjectionPreflightOptions{
		TaskPath:                taskPath,
		PromptPreviewReportPath: reportPath,
		OutputPath:              preflightPath,
	}); err != nil {
		t.Fatalf("MaterializedInjectionPreflight() error = %v", err)
	}
	return taskPath, preflightPath, previewPath
}
