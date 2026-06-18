package retrievalcontext_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestPromptPreviewOKWithValidExecutionPlan(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewArtifacts(t)

	result, err := retrievalcontext.PromptPreview(promptPreviewOpts())
	if err != nil {
		t.Fatalf("PromptPreview() error = %v", err)
	}
	if !result.Manifest.ContainsText || !result.Manifest.PreviewOnly || result.Manifest.RunnerExecution {
		t.Fatalf("manifest = %#v, want preview-only metadata", result.Manifest)
	}

	preview, err := os.ReadFile("prompt-preview.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	previewText := string(preview)
	if !strings.Contains(previewText, "Governed materialized retrieval context") {
		t.Fatal("preview missing section title")
	}
	if !strings.Contains(previewText, "text_excerpt:") || !strings.Contains(previewText, "alpha text") {
		t.Fatal("preview missing text_excerpt content")
	}
	if !strings.Contains(previewText, retrievalcontext.PromptPreviewDerivedContextNotice) {
		t.Fatal("preview missing derived context notice")
	}

	manifestData, err := os.ReadFile("prompt-preview.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(manifestData), "text_excerpt") {
		t.Fatal("manifest must not contain text_excerpt")
	}
	var manifest retrievalcontext.PromptPreviewManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !manifest.ContainsText || !manifest.PreviewOnly || manifest.RunnerExecution {
		t.Fatalf("manifest = %#v", manifest)
	}
}

func TestPromptPreviewRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewArtifacts(t)

	opts := promptPreviewOpts()
	opts.ConfirmRenderMaterializedContext = false
	_, err := retrievalcontext.PromptPreview(opts)
	if err == nil || !strings.Contains(err.Error(), "--confirm-render-materialized-context") {
		t.Fatalf("error = %v, want confirm flag required", err)
	}
}

func TestPromptPreviewFailsWhenExecutionPlanReasonInvalid(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewArtifacts(t)

	payload := readJSONFile(t, "injection-execution-plan.json")
	payload["reason"] = "unexpected_reason"
	writeJSONFile(t, "injection-execution-plan.json", payload)

	_, err := retrievalcontext.PromptPreview(promptPreviewOpts())
	if err == nil || !strings.Contains(err.Error(), "execution_plan_only") {
		t.Fatalf("error = %v, want execution plan reason failure", err)
	}
}

func TestPromptPreviewFailsOnMaterializedHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewArtifacts(t)

	payload := readJSONFile(t, "injection-execution-plan.json")
	payload["materialized_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, "injection-execution-plan.json", payload)

	_, err := retrievalcontext.PromptPreview(promptPreviewOpts())
	if err == nil || !strings.Contains(err.Error(), "materialized_sha256 mismatch") {
		t.Fatalf("error = %v, want materialized hash mismatch", err)
	}
}

func TestPromptPreviewFailsWhenCapsExceeded(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)
	writeInjectionPolicyYAMLWithLimits(t, "injection-policy.yaml", 6000, 1200, 8)
	writeGovernanceReportFile(t, "governance-report.json")
	if _, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts()); err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}
	if _, err := retrievalcontext.ApproveRunnerInjection(retrievalcontext.ApproveRunnerInjectionOptions{
		RequestPath:                 "injection-approval-request.json",
		OutputPath:                  "injection-approval.json",
		ConfirmAllowRunnerInjection: true,
	}); err != nil {
		t.Fatalf("ApproveRunnerInjection() error = %v", err)
	}
	if _, err := retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts()); err != nil {
		t.Fatalf("InjectionExecutionPlan() error = %v", err)
	}

	cfg, err := retrievalcontext.LoadInjectionPolicy("injection-policy.yaml")
	if err != nil {
		t.Fatalf("LoadInjectionPolicy() error = %v", err)
	}
	cfg.RetrievalInjectionPolicy.Limits.MaxTotalChars = 1
	cfg.RetrievalInjectionPolicy.Limits.MaxCharsPerChunk = 1
	writeInjectionPolicyYAMLFromConfig(t, "injection-policy.yaml", cfg)

	_, err = retrievalcontext.PromptPreview(promptPreviewOpts())
	if err == nil || !strings.Contains(err.Error(), "exceeds policy max_total_chars") {
		t.Fatalf("error = %v, want policy cap failure", err)
	}
}

func TestPromptPreviewRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupPromptPreviewArtifacts(t)

	opts := promptPreviewOpts()
	opts.PolicyPath = "/tmp/policy.yaml"
	_, err := retrievalcontext.PromptPreview(opts)
	if err == nil {
		t.Fatal("expected blocked policy path error")
	}
}

func setupPromptPreviewArtifacts(t *testing.T) {
	t.Helper()
	setupInjectionExecutionPlanArtifacts(t)
	if _, err := retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts()); err != nil {
		t.Fatalf("InjectionExecutionPlan() error = %v", err)
	}
}

func promptPreviewOpts() retrievalcontext.PromptPreviewOptions {
	return retrievalcontext.PromptPreviewOptions{
		ExecutionPlanPath:                "injection-execution-plan.json",
		PolicyPath:                       "injection-policy.yaml",
		MaterializedPath:                 "materialized.json",
		OutputPath:                       "prompt-preview.md",
		ManifestPath:                     "prompt-preview.json",
		ConfirmRenderMaterializedContext: true,
	}
}
