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

func TestMaterializedInjectionTaskReportOKWithFixtureTask(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	bundlePath := writeValidGovernanceBundleForTask(t)

	task := validMaterializedInjectionTask(bundlePath, "retrieval-context-prompt-preview.md")
	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		t.Fatalf("MaterializedInjectionTaskReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.MaterializedInjectionDeclared || result.MaterializedInjectionEnabled || result.MaterializedInjectionSupportedNow {
		t.Fatalf("result = %#v, want declared disabled unsupported", result)
	}
	if result.GovernanceBundleSHA256 == "" {
		t.Fatal("governance_bundle_sha256 is required")
	}
	if !result.PromptPreviewPathDeclared || result.PromptPreviewRead || result.RunnerPromptChanged {
		t.Fatalf("result = %#v, want preview declared but not read and runner unchanged", result)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionTaskReportText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionTaskReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionTaskReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionTaskReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("report json leaked preview content")
	}
}

func TestMaterializedInjectionTaskReportFailsWhenEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	bundlePath := writeValidGovernanceBundleForTask(t)

	task := validMaterializedInjectionTask(bundlePath, "retrieval-context-prompt-preview.md")
	task.RetrievalContext.MaterializedInjection.Enabled = true

	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		t.Fatalf("MaterializedInjectionTaskReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), tasks.MaterializedInjectionNotSupportedYet) {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionTaskReportFailsOnInvalidBundle(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("bad-bundle.json", []byte(`{"status":"failed","contains_text":true,"runner_execution":true,"injection_authorized_for_future":false,"execution_supported_now":true}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	task := validMaterializedInjectionTask("bad-bundle.json", "retrieval-context-prompt-preview.md")
	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		t.Fatalf("MaterializedInjectionTaskReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "contains_text must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionTaskReportFailsOnUnsafeGovernanceBundlePath(t *testing.T) {
	task := validMaterializedInjectionTask("/tmp/bundle.json", "retrieval-context-prompt-preview.md")
	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		t.Fatalf("MaterializedInjectionTaskReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "must be a relative path") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionTaskReportFailsOnUnsafePromptPreviewPath(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	bundlePath := writeValidGovernanceBundleForTask(t)

	task := validMaterializedInjectionTask(bundlePath, "secrets/preview.md")
	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		t.Fatalf("MaterializedInjectionTaskReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "blocked path") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionTaskReportDoesNotReadPromptPreview(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	bundlePath := writeValidGovernanceBundleForTask(t)

	task := validMaterializedInjectionTask(bundlePath, "missing-prompt-preview.md")
	result, err := retrievalcontext.MaterializedInjectionTaskReport(task)
	if err != nil {
		t.Fatalf("MaterializedInjectionTaskReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning without reading preview", result)
	}
	if result.PromptPreviewRead {
		t.Fatal("prompt_preview_read must be false")
	}
}

func validMaterializedInjectionTask(bundlePath, previewPath string) *tasks.Task {
	return &tasks.Task{
		ID:     "retrieval-context-materialized-injection-001",
		Title:  "Materialized injection declaration",
		Domain: "general",
		Worker: "codex",
		Goal:   "Declare future governed materialized injection without runner execution.",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{Scope: "none"},
		RetrievalContext: tasks.RetrievalContextSpec{
			MaterializedInjection: &tasks.MaterializedInjectionSpec{
				Enabled:            false,
				GovernanceBundle:   bundlePath,
				PromptPreview:      previewPath,
				RequireConfirmFlag: true,
				MaxTotalChars:      6000,
			},
		},
		ForbiddenPaths:  []string{"secrets/**"},
		ExpectedOutputs: []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{
			"Task declares materialized injection schema only",
		},
	}
}
