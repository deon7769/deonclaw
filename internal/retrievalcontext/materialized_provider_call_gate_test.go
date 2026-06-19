package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedProviderCallGateOKWithValidArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)

	result, err := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath:               dispatchPath,
		PayloadReportPath:                payloadReportPath,
		PayloadOutputPath:                payloadOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       "materialized-provider-call-gate.json",
	})
	if err != nil {
		t.Fatalf("MaterializedProviderCallGate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ProviderCallGateReady || !result.ProviderPayloadValidated || !result.PayloadReadyForFutureCall {
		t.Fatalf("result = %#v, want gate ready", result)
	}
	if result.ProviderCallAllowedNow || result.NetworkCallAllowedNow || result.WorkerExecutionAllowedNow || result.PromptInjectionAllowedNow || result.SentToProvider {
		t.Fatalf("result = %#v, want calls blocked", result)
	}
	if result.Provider != retrievalcontext.MaterializedProviderDispatchProvider {
		t.Fatalf("provider = %q, want codex", result.Provider)
	}
	if !result.ConfirmFlagUsed {
		t.Fatalf("result = %#v, want confirm_flag_used", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedProviderDispatchBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
	if result.MaterializedSHA256 == "" || result.ProviderPayloadSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("materialized-provider-call-gate.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("call gate output leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderCallGateText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderCallGateText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("call gate text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderCallGateJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderCallGateJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("call gate json leaked preview content")
	}
}

func TestMaterializedProviderCallGateRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)

	_, err := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		OutputPath: "out.json",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-inject-materialized-context") {
		t.Fatalf("error = %v, want missing confirm flag", err)
	}
}

func TestMaterializedProviderCallGateFailsWhenDispatchAllowProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	content, _ := os.ReadFile(dispatchPath)
	_ = os.WriteFile(dispatchPath, []byte(strings.Replace(string(content), "allow_provider_call: false", "allow_provider_call: true", 1)), 0o600)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "allow_provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallGateFailsWhenDispatchAllowNetworkTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	content, _ := os.ReadFile(dispatchPath)
	_ = os.WriteFile(dispatchPath, []byte(strings.Replace(string(content), "allow_network: false", "allow_network: true", 1)), 0o600)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "allow_network must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallGateFailsWhenPayloadReportFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["status"] = "failed"
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestMaterializedProviderCallGateFailsWhenProviderPayloadValidatedFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["provider_payload_validated"] = false
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_payload_validated must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallGateFailsWhenProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["provider_call"] = true
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallGateFailsWhenNetworkCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["network_call"] = true
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "network_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallGateFailsWhenSentToProviderTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["sent_to_provider"] = true
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "sent_to_provider must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallGateFailsOnProviderPayloadSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	_ = os.WriteFile(payloadOutputPath, []byte("tampered\n"), 0o644)

	result, _ := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: "out.json",
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_payload_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedProviderCallGateArtifacts(t *testing.T) (string, string, string) {
	t.Helper()
	dryRunPath, payloadOutputPath := setupMaterializedProviderPayloadReportArtifacts(t)
	report, err := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayloadReport() error = %v", err)
	}
	const payloadReportPath = "gate-materialized-provider-payload-report.json"
	var buf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderPayloadReportJSON(report, &buf); err != nil {
		t.Fatalf("WriteMaterializedProviderPayloadReportJSON() error = %v", err)
	}
	if err := os.WriteFile(payloadReportPath, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	const dispatchPath = "payload-materialized-provider-dispatch.yaml"
	return dispatchPath, payloadReportPath, payloadOutputPath
}
