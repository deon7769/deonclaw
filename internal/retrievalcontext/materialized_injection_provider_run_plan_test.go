package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedInjectionProviderRunPlanOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ProviderRunPlanReady {
		t.Fatalf("result = %#v, want provider_run_plan_ready", result)
	}
	if result.ProviderCallAllowedNow || result.WorkerExecutionAllowedNow || result.PromptInjectionAllowedNow {
		t.Fatalf("result = %#v, want provider/worker/injection blocked", result)
	}
	if !result.WouldUseAssembledPrompt || !result.AssembledPromptValidated {
		t.Fatalf("result = %#v, want would_use_assembled_prompt and assembled_prompt_validated", result)
	}
	if result.SentToProvider {
		t.Fatalf("result = %#v, want sent_to_provider false", result)
	}
	if !result.ConfirmFlagUsed {
		t.Fatalf("result = %#v, want confirm_flag_used", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedInjectionExecutionEnableBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedInjectionExecutionEnableBlockedReason)
	}
	if result.MaterializedSHA256 == "" || result.AssembledOutputSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("materialized-injection-provider-run-plan.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("provider run-plan output leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionProviderRunPlanText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionProviderRunPlanText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("provider run-plan text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionProviderRunPlanJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionProviderRunPlanJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("provider run-plan json leaked preview content")
	}
}

func TestMaterializedInjectionProviderRunPlanDoesNotCallProvider(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.SentToProvider || result.ProviderCallAllowedNow {
		t.Fatalf("result = %#v, provider call must remain blocked", result)
	}
}

func TestMaterializedInjectionProviderRunPlanRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)

	_, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:            taskPath,
		EnableConfigPath:    enableConfigPath,
		ExecutionGatePath:   gatePath,
		AssemblyReportPath:  assemblyReportPath,
		AssembledOutputPath: assembledOutputPath,
		OutputPath:          "materialized-injection-provider-run-plan.json",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedInjectionProviderRunPlanFailsWhenEnableConfigEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)
	content, err := os.ReadFile(enableConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(enableConfigPath, []byte(strings.Replace(string(content), "enabled: false", "enabled: true", 1)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "enabled must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionProviderRunPlanFailsWhenEnableConfigAllowProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)
	content, err := os.ReadFile(enableConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(enableConfigPath, []byte(strings.Replace(string(content), "allow_provider_call: false", "allow_provider_call: true", 1)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionProviderRunPlanFailsWhenExecutionGateNotReady(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["execution_gate_ready"] = false
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "execution_gate_ready must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionProviderRunPlanFailsWhenAssemblyReportNotValidated(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)
	report := readJSONFile(t, assemblyReportPath)
	report["assembled_prompt_validated"] = false
	writeJSONFile(t, assemblyReportPath, report)

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "assembled_prompt_validated must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionProviderRunPlanFailsOnAssembledOutputSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)
	if err := os.WriteFile(assembledOutputPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-provider-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "assembled_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedInjectionProviderRunPlanArtifacts(t *testing.T) (string, string, string, string, string) {
	t.Helper()
	taskPath, _, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)

	const enableConfigPath = "provider-run-plan-materialized-injection-execution-enable.yaml"
	if err := os.WriteFile(enableConfigPath, []byte(validExecutionEnableConfigYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath
}
