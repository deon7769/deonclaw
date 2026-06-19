package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedInjectionRunPlanOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.RunPlanReady {
		t.Fatalf("result = %#v, want run_plan_ready", result)
	}
	if result.WorkerExecutionPlanned || result.PromptInjectionPlanned || result.ImplementationAllowsExecutionNow {
		t.Fatalf("result = %#v, want execution/injection blocked", result)
	}
	if !result.ConfirmFlagUsed {
		t.Fatalf("result = %#v, want confirm_flag_used", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedInjectionRuntimeBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedInjectionRuntimeBlockedReason)
	}
	if result.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q, want %q", result.RequiredFutureFlag, retrievalcontext.RequiredFutureInjectFlag)
	}
	if result.MaterializedSHA256 == "" || result.AssembledOutputSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("materialized-injection-run-plan.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("run-plan output leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionRunPlanText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionRunPlanText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("run-plan text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionRunPlanJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionRunPlanJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("run-plan json leaked preview content")
	}
}

func TestMaterializedInjectionRunPlanRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)

	_, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:            taskPath,
		RuntimeConfigPath:   runtimeConfigPath,
		ExecutionGatePath:   gatePath,
		AssemblyReportPath:  assemblyReportPath,
		AssembledOutputPath: assembledOutputPath,
		OutputPath:          "materialized-injection-run-plan.json",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedInjectionRunPlanFailsWhenRuntimeEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)
	content, err := os.ReadFile(runtimeConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(runtimeConfigPath, []byte(strings.Replace(string(content), "enabled: false", "enabled: true", 1)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "enabled must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRunPlanFailsWhenRuntimeAllowWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)
	content, err := os.ReadFile(runtimeConfigPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if err := os.WriteFile(runtimeConfigPath, []byte(strings.Replace(string(content), "allow_worker_execution: false", "allow_worker_execution: true", 1)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRunPlanFailsWhenExecutionGateNotReady(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["execution_gate_ready"] = false
	writeJSONFile(t, gatePath, gate)

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "execution_gate_ready must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRunPlanFailsWhenAssemblyReportNotValidated(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)
	report := readJSONFile(t, assemblyReportPath)
	report["assembled_prompt_validated"] = false
	writeJSONFile(t, assemblyReportPath, report)

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "assembled_prompt_validated must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRunPlanFailsOnMaterializedSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)
	report := readJSONFile(t, assemblyReportPath)
	report["materialized_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, assemblyReportPath, report)

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "materialized_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRunPlanFailsOnAssembledOutputSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)
	if err := os.WriteFile(assembledOutputPath, []byte("tampered\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionRunPlan(retrievalcontext.MaterializedInjectionRunPlanOptions{
		TaskPath:                         taskPath,
		RuntimeConfigPath:                runtimeConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-injection-run-plan.json",
	})
	if err != nil {
		t.Fatalf("MaterializedInjectionRunPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "assembled_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRunPlanRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionRunPlanArtifacts(t)

	cases := []struct {
		name string
		opts retrievalcontext.MaterializedInjectionRunPlanOptions
	}{
		{
			name: "task path",
			opts: retrievalcontext.MaterializedInjectionRunPlanOptions{
				TaskPath: "/tmp/task.yaml", RuntimeConfigPath: runtimeConfigPath,
				ExecutionGatePath: gatePath, AssemblyReportPath: assemblyReportPath, AssembledOutputPath: assembledOutputPath,
				ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
			},
		},
		{
			name: "runtime config path",
			opts: retrievalcontext.MaterializedInjectionRunPlanOptions{
				TaskPath: taskPath, RuntimeConfigPath: "/tmp/runtime.yaml",
				ExecutionGatePath: gatePath, AssemblyReportPath: assemblyReportPath, AssembledOutputPath: assembledOutputPath,
				ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
			},
		},
		{
			name: "execution gate path",
			opts: retrievalcontext.MaterializedInjectionRunPlanOptions{
				TaskPath: taskPath, RuntimeConfigPath: runtimeConfigPath,
				ExecutionGatePath: "/tmp/gate.json", AssemblyReportPath: assemblyReportPath, AssembledOutputPath: assembledOutputPath,
				ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
			},
		},
		{
			name: "assembly report path",
			opts: retrievalcontext.MaterializedInjectionRunPlanOptions{
				TaskPath: taskPath, RuntimeConfigPath: runtimeConfigPath,
				ExecutionGatePath: gatePath, AssemblyReportPath: "/tmp/report.json", AssembledOutputPath: assembledOutputPath,
				ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
			},
		},
		{
			name: "assembled output path",
			opts: retrievalcontext.MaterializedInjectionRunPlanOptions{
				TaskPath: taskPath, RuntimeConfigPath: runtimeConfigPath,
				ExecutionGatePath: gatePath, AssemblyReportPath: assemblyReportPath, AssembledOutputPath: "/tmp/assembled.md",
				ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := retrievalcontext.MaterializedInjectionRunPlan(tc.opts)
			if err == nil {
				t.Fatalf("expected blocked path error for %s", tc.name)
			}
		})
	}
}

func setupMaterializedInjectionRunPlanArtifacts(t *testing.T) (string, string, string, string, string) {
	t.Helper()
	gatePath, promptOutputPath, basePromptPath := setupMaterializedPromptAssemblyArtifacts(t)
	const assemblyDryRunPath = "run-plan-materialized-prompt-assembly-dry-run.json"
	const assembledOutputPath = "run-plan-materialized-prompt-assembly.md"
	if _, err := retrievalcontext.MaterializedPromptAssembly(retrievalcontext.MaterializedPromptAssemblyOptions{
		ExecutionGatePath:                gatePath,
		BasePromptFixturePath:            basePromptPath,
		PromptOutputPath:                 promptOutputPath,
		OutputPath:                       assemblyDryRunPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
	}); err != nil {
		t.Fatalf("MaterializedPromptAssembly() error = %v", err)
	}
	const assemblyReportPath = "run-plan-materialized-prompt-assembly-report.json"
	report, err := retrievalcontext.MaterializedPromptAssemblyReport(retrievalcontext.MaterializedPromptAssemblyReportOptions{
		AssemblyDryRunPath:  assemblyDryRunPath,
		AssembledOutputPath: assembledOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedPromptAssemblyReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedPromptAssemblyReportJSON(report, &buf); err != nil {
		t.Fatalf("WriteMaterializedPromptAssemblyReportJSON() error = %v", err)
	}
	if err := os.WriteFile(assemblyReportPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	taskData, err := os.ReadFile("execution-gate-materialized-injection-task.yaml")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	const taskPath = "run-plan-materialized-injection-task.yaml"
	if err := os.WriteFile(taskPath, taskData, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	const runtimeConfigPath = "run-plan-materialized-injection-runtime.yaml"
	if err := os.WriteFile(runtimeConfigPath, []byte(validRuntimeConfigYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return taskPath, runtimeConfigPath, gatePath, assemblyReportPath, assembledOutputPath
}
