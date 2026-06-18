package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestPromptPreviewReportOKWithValidPreview(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)

	result, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("result = %#v, want ok report", result)
	}
	if !result.PreviewOnly || result.RunnerExecution {
		t.Fatalf("result = %#v, want preview-only non-runner report", result)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WritePromptPreviewReportText(result, &textBuf); err != nil {
		t.Fatalf("WritePromptPreviewReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "alpha text") || strings.Contains(textBuf.String(), "text_excerpt:") {
		t.Fatal("report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WritePromptPreviewReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WritePromptPreviewReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "alpha text") || strings.Contains(jsonBuf.String(), "text_excerpt") {
		t.Fatal("report json leaked preview content")
	}
}

func TestPromptPreviewReportFailsOnManifestHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)

	payload := readJSONFile(t, "prompt-preview.json")
	payload["prompt_preview_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, "prompt-preview.json", payload)

	result, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_preview_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestPromptPreviewReportFailsWhenManifestContainsTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)
	if err := os.WriteFile("prompt-preview.json", []byte(`{"text_excerpt":"leak"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err == nil || !strings.Contains(err.Error(), "text_excerpt") {
		t.Fatalf("error = %v, want manifest text_excerpt rejection", err)
	}
}

func TestPromptPreviewReportFailsWhenRunnerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)

	payload := readJSONFile(t, "prompt-preview.json")
	payload["runner_execution"] = true
	writeJSONFile(t, "prompt-preview.json", payload)

	result, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "runner_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestPromptPreviewReportFailsWhenExecutionPlanReasonInvalid(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)

	payload := readJSONFile(t, "injection-execution-plan.json")
	payload["reason"] = "unexpected_reason"
	writeJSONFile(t, "injection-execution-plan.json", payload)

	result, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "execution_plan_only") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestPromptPreviewReportFailsWhenPreviewMissingNotice(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)
	if err := os.WriteFile("prompt-preview.md", []byte("## title\n- text_excerpt: alpha text\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "derived context notice") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestPromptPreviewReportFailsWhenPreviewMissingTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)
	if err := os.WriteFile("prompt-preview.md", []byte("## Governed materialized retrieval context\n\nDerived governed retrieval context preview. Not canonical memory.\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "preview missing text_excerpt") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestPromptPreviewReportRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewReportArtifacts(t)

	opts := promptPreviewReportOpts()
	opts.PreviewPath = "/tmp/prompt-preview.md"
	_, err := retrievalcontext.PromptPreviewReport(opts)
	if err == nil {
		t.Fatal("expected blocked preview path error")
	}
}

func setupPromptPreviewReportArtifacts(t *testing.T) {
	t.Helper()
	setupPromptPreviewArtifacts(t)
	if _, err := retrievalcontext.PromptPreview(promptPreviewOpts()); err != nil {
		t.Fatalf("PromptPreview() error = %v", err)
	}
}

func promptPreviewReportOpts() retrievalcontext.PromptPreviewReportOptions {
	return retrievalcontext.PromptPreviewReportOptions{
		PreviewPath:       "prompt-preview.md",
		ManifestPath:      "prompt-preview.json",
		ExecutionPlanPath: "injection-execution-plan.json",
	}
}
