package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedProviderPayloadDryRunOKWithFixtureArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)

	result, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:               dispatchPath,
		ProviderRunPlanPath:              runPlanPath,
		AssembledOutputPath:              assembledPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-provider-payload-dry-run.json",
		PayloadOutputPath:                "materialized-provider-payload.md",
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayload() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ProviderPayloadRendered {
		t.Fatalf("result = %#v, want provider_payload_rendered", result)
	}
	if result.ProviderCall || result.NetworkCall || result.SentToProvider || result.WorkerExecution || result.PromptInjectionRealRunner {
		t.Fatalf("result = %#v, want provider/network/worker blocked", result)
	}
	if result.Provider != retrievalcontext.MaterializedProviderDispatchProvider {
		t.Fatalf("provider = %q, want codex", result.Provider)
	}

	outputData, err := os.ReadFile("materialized-provider-payload-dry-run.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("payload dry-run json leaked preview content")
	}

	payloadData, err := os.ReadFile("materialized-provider-payload.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(payloadData), retrievalcontext.ProviderPayloadDryRunNotice) {
		t.Fatal("payload output missing dry-run notice")
	}
	if !strings.Contains(string(payloadData), "text_excerpt") {
		t.Fatal("payload output missing text_excerpt")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderPayloadText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderPayloadText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("payload dry-run text leaked preview content")
	}
}

func TestMaterializedProviderPayloadDoesNotCallProvider(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)

	result, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:               dispatchPath,
		ProviderRunPlanPath:              runPlanPath,
		AssembledOutputPath:              assembledPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-provider-payload-dry-run.json",
		PayloadOutputPath:                "materialized-provider-payload.md",
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayload() error = %v", err)
	}
	if result.ProviderCall || result.NetworkCall || result.SentToProvider {
		t.Fatalf("result = %#v, provider must not be called", result)
	}
}

func TestMaterializedProviderPayloadRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)

	_, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:  dispatchPath,
		ProviderRunPlanPath: runPlanPath,
		AssembledOutputPath: assembledPath,
		OutputPath:          "materialized-provider-payload-dry-run.json",
		PayloadOutputPath:   "materialized-provider-payload.md",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedProviderPayloadFailsWhenDispatchConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	content, _ := os.ReadFile(dispatchPath)
	_ = os.WriteFile(dispatchPath, []byte(strings.Replace(string(content), "enabled: false", "enabled: true", 1)), 0o600)

	result, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:               dispatchPath,
		ProviderRunPlanPath:              runPlanPath,
		AssembledOutputPath:              assembledPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-provider-payload-dry-run.json",
		PayloadOutputPath:                "materialized-provider-payload.md",
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayload() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "enabled must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadFailsWhenProviderRunPlanFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	plan := readJSONFile(t, runPlanPath)
	plan["status"] = "failed"
	writeJSONFile(t, runPlanPath, plan)

	result, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:               dispatchPath,
		ProviderRunPlanPath:              runPlanPath,
		AssembledOutputPath:              assembledPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-provider-payload-dry-run.json",
		PayloadOutputPath:                "materialized-provider-payload.md",
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayload() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestMaterializedProviderPayloadFailsWhenProviderCallAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	plan := readJSONFile(t, runPlanPath)
	plan["provider_call_allowed_now"] = true
	writeJSONFile(t, runPlanPath, plan)

	result, _ := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath: dispatchPath, ProviderRunPlanPath: runPlanPath, AssembledOutputPath: assembledPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json", PayloadOutputPath: "out.md",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_call_allowed_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadFailsWhenSentToProviderTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	plan := readJSONFile(t, runPlanPath)
	plan["sent_to_provider"] = true
	writeJSONFile(t, runPlanPath, plan)

	result, _ := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath: dispatchPath, ProviderRunPlanPath: runPlanPath, AssembledOutputPath: assembledPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json", PayloadOutputPath: "out.md",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "sent_to_provider must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadFailsOnAssembledOutputSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	_ = os.WriteFile(assembledPath, []byte("tampered\n"), 0o644)

	result, _ := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath: dispatchPath, ProviderRunPlanPath: runPlanPath, AssembledOutputPath: assembledPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json", PayloadOutputPath: "out.md",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "assembled_output_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadFailsWhenPayloadBytesExceedMax(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	content, _ := os.ReadFile(dispatchPath)
	_ = os.WriteFile(dispatchPath, []byte(strings.Replace(string(content), "max_payload_bytes: 120000", "max_payload_bytes: 10", 1)), 0o600)

	result, _ := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath: dispatchPath, ProviderRunPlanPath: runPlanPath, AssembledOutputPath: assembledPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json", PayloadOutputPath: "out.md",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "exceed max_payload_bytes") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedProviderPayloadArtifacts(t *testing.T) (string, string, string) {
	t.Helper()
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)

	const dispatchPath = "payload-materialized-provider-dispatch.yaml"
	if err := os.WriteFile(dispatchPath, []byte(validDispatchConfigYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	const runPlanPath = "payload-materialized-provider-run-plan.json"
	if _, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
		TaskPath:                         taskPath,
		EnableConfigPath:                 enableConfigPath,
		ExecutionGatePath:                gatePath,
		AssemblyReportPath:               assemblyReportPath,
		AssembledOutputPath:              assembledOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       runPlanPath,
	}); err != nil {
		t.Fatalf("MaterializedInjectionProviderRunPlan() error = %v", err)
	}

	return dispatchPath, runPlanPath, assembledOutputPath
}
