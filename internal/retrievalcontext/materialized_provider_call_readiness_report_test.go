package retrievalcontext_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedProviderCallReadinessReportOKWithValidArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)

	result, err := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath,
		PayloadReportPath:    payloadReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedProviderCallReadinessReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ProviderCallReadinessReady || !result.PayloadReadyForFutureCall {
		t.Fatalf("result = %#v, want readiness ready", result)
	}
	if result.ProviderCallAllowedNow || result.NetworkCallAllowedNow || result.WorkerExecutionAllowedNow || result.PromptInjectionAllowedNow || result.SentToProvider {
		t.Fatalf("result = %#v, want calls blocked", result)
	}
	if result.Provider != retrievalcontext.MaterializedProviderDispatchProvider {
		t.Fatalf("provider = %q, want codex", result.Provider)
	}
	if result.BlockedReason != retrievalcontext.MaterializedProviderDispatchBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
	if result.MaterializedSHA256 == "" || result.ProviderPayloadSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderCallReadinessReportText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderCallReadinessReportText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("readiness report text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderCallReadinessReportJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderCallReadinessReportJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("readiness report json leaked preview content")
	}
}

func TestMaterializedProviderCallReadinessReportFailsWhenGateFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["status"] = "failed"
	writeJSONFile(t, gatePath, gate)

	result, _ := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestMaterializedProviderCallReadinessReportFailsWhenProviderCallAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["provider_call_allowed_now"] = true
	writeJSONFile(t, gatePath, gate)

	result, _ := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_call_allowed_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallReadinessReportFailsWhenNetworkCallAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["network_call_allowed_now"] = true
	writeJSONFile(t, gatePath, gate)

	result, _ := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "network_call_allowed_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallReadinessReportFailsWhenSentToProviderTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)
	gate := readJSONFile(t, gatePath)
	gate["sent_to_provider"] = true
	writeJSONFile(t, gatePath, gate)

	result, _ := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "sent_to_provider must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallReadinessReportFailsWhenPayloadReportProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["provider_call"] = true
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderCallReadinessReportFailsOnPayloadHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	gatePath, payloadReportPath := setupMaterializedProviderCallReadinessReportArtifacts(t)
	report := readJSONFile(t, payloadReportPath)
	report["provider_payload_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, payloadReportPath, report)

	result, _ := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider_payload_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func setupMaterializedProviderCallReadinessReportArtifacts(t *testing.T) (string, string) {
	t.Helper()
	dispatchPath, payloadReportPath, payloadOutputPath := setupMaterializedProviderCallGateArtifacts(t)
	const gatePath = "readiness-materialized-provider-call-gate.json"
	if _, err := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath:               dispatchPath,
		PayloadReportPath:                payloadReportPath,
		PayloadOutputPath:                payloadOutputPath,
		ConfirmInjectMaterializedContext: true,
		OutputPath:                       gatePath,
	}); err != nil {
		t.Fatalf("MaterializedProviderCallGate() error = %v", err)
	}
	return gatePath, payloadReportPath
}
