package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedProviderPayloadReportOKWithValidPayload(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)

	result, err := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath,
		PayloadOutputPath: payloadOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayloadReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ProviderPayloadValidated {
		t.Fatalf("result = %#v, want provider_payload_validated", result)
	}
	if result.ProviderCall || result.NetworkCall || result.SentToProvider || result.WorkerExecution || result.PromptInjectionRealRunner {
		t.Fatalf("result = %#v, want execution blocked", result)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderPayloadReportText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderPayloadReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("payload report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderPayloadReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderPayloadReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("payload report json leaked preview content")
	}
}

func TestMaterializedProviderPayloadReportFailsOnPayloadHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	_ = os.WriteFile(payloadOutputPath, []byte("tampered\n"), 0o644)

	result, err := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath,
		PayloadOutputPath: payloadOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayloadReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_payload_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	payload := readJSONFile(t, dryRunPath)
	payload["provider_call"] = true
	writeJSONFile(t, dryRunPath, payload)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenNetworkCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	payload := readJSONFile(t, dryRunPath)
	payload["network_call"] = true
	writeJSONFile(t, dryRunPath, payload)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "network_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	payload := readJSONFile(t, dryRunPath)
	payload["worker_execution"] = true
	writeJSONFile(t, dryRunPath, payload)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenSentToProviderTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	payload := readJSONFile(t, dryRunPath)
	payload["sent_to_provider"] = true
	writeJSONFile(t, dryRunPath, payload)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "sent_to_provider must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenPromptInjectionRealRunnerTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	payload := readJSONFile(t, dryRunPath)
	payload["prompt_injection_real_runner"] = true
	writeJSONFile(t, dryRunPath, payload)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "prompt_injection_real_runner must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenPayloadOutputMissingDryRunNotice(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	data, _ := os.ReadFile(payloadOutputPath)
	_ = os.WriteFile(payloadOutputPath, []byte(strings.ReplaceAll(string(data), retrievalcontext.ProviderPayloadDryRunNotice, "no notice")), 0o644)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "missing dry-run notice") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderPayloadReportFailsWhenPayloadOutputMissingTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	data, _ := os.ReadFile(payloadOutputPath)
	_ = os.WriteFile(payloadOutputPath, []byte(strings.ReplaceAll(string(data), "text_excerpt", "no_excerpt")), 0o644)

	result, _ := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "missing text_excerpt") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedProviderPayloadReportArtifacts(t *testing.T) (string, string) {
	t.Helper()
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)
	const dryRunPath = "report-materialized-provider-payload-dry-run.json"
	const payloadOutputPath = "report-materialized-provider-payload.md"
	if _, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:               dispatchPath,
		ProviderRunPlanPath:              runPlanPath,
		AssembledOutputPath:              assembledPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       dryRunPath,
		PayloadOutputPath:                payloadOutputPath,
	}); err != nil {
		t.Fatalf("MaterializedProviderPayload() error = %v", err)
	}
	return dryRunPath, payloadOutputPath
}
