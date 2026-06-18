package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const basePromptFixtureContent = "# Worker prompt fixture\n\nThis is a dry-run base worker prompt fixture.\nIt is not sent to Codex or OpenCode.\n"

func TestMaterializedPromptAssemblyOKWithValidGate(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.AssembledPromptRendered {
		t.Fatalf("result = %#v, want assembled_prompt_rendered", result)
	}
	if result.WorkerExecution || result.SentToWorker || result.PromptChangedInRealRunner {
		t.Fatalf("result = %#v, want no worker execution or prompt change", result)
	}
	if !result.ContainsText || !result.PreviewOnly || !result.ConfirmFlagUsed {
		t.Fatalf("result = %#v, want preview-only metadata", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedInjectionExecutionGateBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedInjectionExecutionGateBlockedReason)
	}
	if result.BasePromptSHA256 == "" || result.PromptOutputSHA256 == "" || result.AssembledOutputSHA256 == "" || result.MaterializedSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("materialized-prompt-assembly-dry-run.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("assembly dry-run json leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedPromptAssemblyText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedPromptAssemblyText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("assembly dry-run text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedPromptAssemblyJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedPromptAssemblyJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("assembly dry-run json leaked preview content")
	}

	assembledData, err := os.ReadFile("materialized-prompt-assembly.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	assembledText := string(assembledData)
	if !strings.Contains(assembledText, retrievalcontext.MaterializedPromptAssemblyDryRunNotice) {
		t.Fatal("assembled output missing dry-run notice")
	}
	if !strings.Contains(assembledText, "text_excerpt") {
		t.Fatal("assembled output missing text_excerpt")
	}
}

func TestMaterializedPromptAssemblyRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)

	_, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:     gatePath,
		BasePromptFixturePath: basePromptPath,
		PromptOutputPath:      promptOutputPath,
		OutputPath:            "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:   "materialized-prompt-assembly.md",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedPromptAssemblyFailsWhenGateFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["status"] = lancedbpolicy.StatusFailed
	gate["failures"] = []string{"forced failure"}
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.AssembledPromptRendered {
		t.Fatal("assembled_prompt_rendered must be false when gate failed")
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "execution gate status must be ok or warning") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyFailsWhenGateReadyFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["execution_gate_ready"] = false
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "execution_gate_ready must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyFailsWhenImplementationAllowsExecutionNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["implementation_allows_execution_now"] = true
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "implementation_allows_execution_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyFailsWhenWorkerExecutionAllowedTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["worker_execution_allowed"] = true
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "worker_execution_allowed must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyFailsWhenPromptInjectionAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["prompt_injection_allowed_now"] = true
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_injection_allowed_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyFailsOnPromptOutputSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	if err := os.WriteFile(promptOutputPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
	if _, err := os.Stat("materialized-prompt-assembly.md"); err == nil {
		t.Fatal("assembled output must not be written when prompt output hash mismatches")
	}
}

func TestMaterializedPromptAssemblyRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)

	_, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                "/tmp/gate.json",
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err == nil {
		t.Fatal("expected blocked execution gate path error")
	}

	_, err = retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            "/tmp/base.md",
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err == nil {
		t.Fatal("expected blocked base prompt path error")
	}

	_, err = retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 "/tmp/prompt-section.md",
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "materialized-prompt-assembly.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err == nil {
		t.Fatal("expected blocked prompt output path error")
	}

	_, err = retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       "materialized-prompt-assembly-dry-run.json",
		AssembledOutputPath:              "/tmp/assembled.md",
		ConfirmInjectMaterializedContext: true,
	})
	if err == nil {
		t.Fatal("expected blocked assembled output path error")
	}
}

func setupMaterializedPromptAssemblyArtifacts(t *testing.T) (string, string, string) {
	t.Helper()
	taskPath, readinessPath, promptOutputPath := setupMaterializedInjectionExecutionGateArtifacts(t)
	const gatePath = "retrieval-context-materialized-injection-execution-gate.json"
	if _, err := retrievalcontext.MaterializedInjectionExecutionGate(retrievalcontext.MaterializedInjectionExecutionGateOptions{
		TaskPath:                         taskPath,
		ReadinessReportPath:              readinessPath,
		PromptOutputPath:                 promptOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       gatePath,
	}); err != nil {
		t.Fatalf("MaterializedInjectionExecutionGate() error = %v", err)
	}

	const basePromptPath = "base-worker-prompt-fixture.md"
	if err := os.WriteFile(basePromptPath, []byte(basePromptFixtureContent), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return gatePath, promptOutputPath, basePromptPath
}
