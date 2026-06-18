package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedPromptAssemblyReportOKWithValidAssembly(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.AssembledPromptValidated {
		t.Fatalf("result = %#v, want assembled_prompt_validated", result)
	}
	if result.WorkerExecution || result.SentToWorker || result.PromptChangedInRealRunner {
		t.Fatalf("result = %#v, want no worker execution", result)
	}
	if !result.ContainsText || !result.PreviewOnly {
		t.Fatalf("result = %#v, want preview-only metadata", result)
	}
	if result.AssembledOutputSHA256 == "" || result.MaterializedSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedPromptAssemblyReportText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedPromptAssemblyReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("assembly report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedPromptAssemblyReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedPromptAssemblyReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("assembly report json leaked preview content")
	}
}

func TestMaterializedPromptAssemblyReportFailsOnAssembledHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)
	if err := os.WriteFile(assembledOutputPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "assembled_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyReportFailsWhenWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)
	assembly := readJSONFile(t, assemblyPath)
	assembly["worker_execution"] = true
	writeJSONFile(t, assemblyPath, assembly)

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyReportFailsWhenSentToWorkerTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)
	assembly := readJSONFile(t, assemblyPath)
	assembly["sent_to_worker"] = true
	writeJSONFile(t, assemblyPath, assembly)

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "sent_to_worker must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyReportFailsWhenPromptChangedInRealRunnerTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)
	assembly := readJSONFile(t, assemblyPath)
	assembly["prompt_changed_in_real_runner"] = true
	writeJSONFile(t, assemblyPath, assembly)

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "prompt_changed_in_real_runner must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyReportFailsWhenAssembledOutputMissingDryRunNotice(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)
	data, err := os.ReadFile(assembledOutputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := strings.Replace(string(data), retrievalcontext.MaterializedPromptAssemblyDryRunNotice, "notice removed", 1)
	if err := os.WriteFile(assembledOutputPath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	assembly := readJSONFile(t, assemblyPath)
	assembly["assembled_output_sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
	writeJSONFile(t, assemblyPath, assembly)

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "missing dry-run notice") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyReportFailsWhenAssembledOutputMissingTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)
	if err := os.WriteFile(assembledOutputPath, []byte(retrievalcontext.MaterializedPromptAssemblyDryRunNotice+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	assembly := readJSONFile(t, assemblyPath)
	assembly["assembled_output_sha256"] = "0000000000000000000000000000000000000000000000000000000000000000"
	writeJSONFile(t, assemblyPath, assembly)

	result, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "missing text_excerpt") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedPromptAssemblyReportRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	assemblyPath, assembledOutputPath := setupMaterializedPromptAssemblyReportArtifacts(t)

	_, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  "/tmp/assembly.json",
		AssembledOutputPath: assembledOutputPath,
	})
	if err == nil {
		t.Fatal("expected blocked assembly dry-run path error")
	}

	_, err = retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyPath,
		AssembledOutputPath: "/tmp/assembled.md",
	})
	if err == nil {
		t.Fatal("expected blocked assembled output path error")
	}
}

func setupMaterializedPromptAssemblyReportArtifacts(t *testing.T) (string, string) {
	t.Helper()
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	const assemblyPath = "materialized-prompt-assembly-dry-run.json"
	const assembledOutputPath = "materialized-prompt-assembly.md"
	if _, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       assemblyPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
	}); err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	return assemblyPath, assembledOutputPath
}
