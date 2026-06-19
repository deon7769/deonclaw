package retrievalcontext_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validDispatchConfigYAML = `materialized_injection_dispatch:
  enabled: false
  allow_provider_call: false
  allow_network: false
  allow_worker_execution: false
  blocked_reason: implementation_not_enabled
`

func TestProviderCallApprovalRequestApproveInspectOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainArtifacts(t)

	request, err := retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath:  chain.readinessReportPath,
		ProviderCallGatePath: chain.gatePath,
		PayloadReportPath:    chain.payloadReportPath,
		OutputPath:           "provider-call-approval-request.json",
	})
	if err != nil {
		t.Fatalf("NewProviderCallApprovalRequest() error = %v", err)
	}
	if request.ProviderCallAllowedNow || request.SentToProvider || request.ContainsText {
		t.Fatalf("request = %#v, want metadata-only blocked flags", request)
	}

	approval, err := retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath:                "provider-call-approval-request.json",
		OutputPath:                 "provider-call-approval.json",
		ConfirmPayloadOutputSHA256: chain.payloadOutputSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderCall() error = %v", err)
	}
	if !approval.ProviderCallAuthorizedForFuture {
		t.Fatalf("approval = %#v, want provider_call_authorized_for_future", approval)
	}
	if approval.ProviderCallAllowedNow || approval.SentToProvider {
		t.Fatalf("approval = %#v, want provider call still blocked", approval)
	}
	if !approval.ConfirmPayloadOutputSHA256 {
		t.Fatalf("approval = %#v, want confirm_payload_output_sha256", approval)
	}

	inspect, err := retrievalcontext.InspectProviderCallApproval("provider-call-approval.json", retrievalcontext.InspectProviderCallApprovalOptions{
		RequestPath: "provider-call-approval-request.json",
	})
	if err != nil {
		t.Fatalf("InspectProviderCallApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect = %#v, want ok", inspect)
	}
	if inspect.ProviderCallAllowedNow || inspect.SentToProvider {
		t.Fatalf("inspect = %#v, want blocked provider call flags", inspect)
	}

	approvalData, err := os.ReadFile("provider-call-approval.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(approvalData), "text_excerpt") || strings.Contains(string(approvalData), "alpha text") {
		t.Fatal("approval artifact leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteInspectProviderCallApprovalText(inspect, &textBuf); err != nil {
		t.Fatalf("WriteInspectProviderCallApprovalText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("inspect text leaked preview content")
	}
}

func TestApproveProviderCallFailsWithoutConfirmHash(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainArtifacts(t)

	_, err := retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath:  chain.readinessReportPath,
		ProviderCallGatePath: chain.gatePath,
		PayloadReportPath:    chain.payloadReportPath,
		OutputPath:           "provider-call-approval-request.json",
	})
	if err != nil {
		t.Fatalf("NewProviderCallApprovalRequest() error = %v", err)
	}

	_, err = retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath: "provider-call-approval-request.json",
		OutputPath:  "provider-call-approval.json",
	})
	if err == nil || !strings.Contains(err.Error(), "confirm-payload-output-sha256") {
		t.Fatalf("ApproveProviderCall() error = %v, want confirm hash required", err)
	}
}

func TestApproveProviderCallFailsOnHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainArtifacts(t)

	_, err := retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath:  chain.readinessReportPath,
		ProviderCallGatePath: chain.gatePath,
		PayloadReportPath:    chain.payloadReportPath,
		OutputPath:           "provider-call-approval-request.json",
	})
	if err != nil {
		t.Fatalf("NewProviderCallApprovalRequest() error = %v", err)
	}

	_, err = retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath:                "provider-call-approval-request.json",
		OutputPath:                 "provider-call-approval.json",
		ConfirmPayloadOutputSHA256: "deadbeef",
	})
	if err == nil || !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("ApproveProviderCall() error = %v, want hash mismatch", err)
	}
}

func TestNewProviderCallApprovalRequestFailsWhenGateHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainArtifacts(t)

	gateData, err := os.ReadFile(chain.gatePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var gate retrievalcontext.ProviderCallGateResult
	if err := json.Unmarshal(gateData, &gate); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	gate.PayloadOutputSHA256 = "deadbeef"
	updated, err := json.MarshalIndent(gate, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(chain.gatePath, append(updated, '\n'), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath:  chain.readinessReportPath,
		ProviderCallGatePath: chain.gatePath,
		PayloadReportPath:    chain.payloadReportPath,
		OutputPath:           "provider-call-approval-request.json",
	})
	if err == nil || !strings.Contains(err.Error(), "payload_output_sha256 mismatch") {
		t.Fatalf("NewProviderCallApprovalRequest() error = %v, want mismatch", err)
	}
}

func TestNewProviderCallApprovalRequestRejectsPayloadReportWithTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainArtifacts(t)

	if err := os.WriteFile(chain.payloadReportPath, []byte(`{"status":"ok","text_excerpt":"leak"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath:  chain.readinessReportPath,
		ProviderCallGatePath: chain.gatePath,
		PayloadReportPath:    chain.payloadReportPath,
		OutputPath:           "provider-call-approval-request.json",
	})
	if err == nil || (!strings.Contains(err.Error(), "text_excerpt") && !strings.Contains(err.Error(), "materialized preview text")) {
		t.Fatalf("NewProviderCallApprovalRequest() error = %v, want text_excerpt rejection", err)
	}
}

type providerCallChainArtifacts struct {
	dispatchConfigPath    string
	payloadReportPath     string
	gatePath              string
	readinessReportPath   string
	payloadOutputPath     string
	payloadOutputSHA256   string
	providerRunPlanSHA256 string
}

func setupProviderCallChainArtifacts(t *testing.T) providerCallChainArtifacts {
	t.Helper()
	taskPath, enableConfigPath, gatePath, assemblyReportPath, assembledOutputPath := setupMaterializedInjectionProviderRunPlanArtifacts(t)

	providerRunPlan, err := retrievalcontext.MaterializedInjectionProviderRunPlan(retrievalcontext.MaterializedInjectionProviderRunPlanOptions{
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
	if !providerRunPlan.ProviderRunPlanReady {
		t.Fatalf("provider run plan = %#v, want ready", providerRunPlan)
	}

	providerRunPlanData, err := os.ReadFile("materialized-injection-provider-run-plan.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	providerRunPlanSHA := sha256HexBytes(providerRunPlanData)

	const payloadOutputPath = "materialized-injection-payload-output.md"
	payloadOutput := "# provider payload dry-run\n\nalpha text for payload only\n"
	if err := os.WriteFile(payloadOutputPath, []byte(payloadOutput), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	payloadOutputSHA := sha256HexBytes([]byte(payloadOutput))

	const payloadReportPath = "materialized-injection-payload-report.json"
	payloadReport := retrievalcontext.PayloadReportResult{
		Status:                lancedbpolicy.StatusOK,
		PayloadValidated:      true,
		ContainsText:          true,
		PreviewOnly:           true,
		WorkerExecution:       false,
		SentToProvider:        false,
		PayloadOutputSHA256:   payloadOutputSHA,
		MaterializedSHA256:    providerRunPlan.MaterializedSHA256,
		AssembledOutputSHA256: providerRunPlan.AssembledOutputSHA256,
		ProviderRunPlanSHA256: providerRunPlanSHA,
	}
	writeJSONArtifact(t, payloadReportPath, payloadReport)
	payloadReportData, err := os.ReadFile(payloadReportPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	payloadReportSHA := sha256HexBytes(payloadReportData)

	const providerCallGatePath = "materialized-injection-provider-call-gate.json"
	gate := retrievalcontext.ProviderCallGateResult{
		Status:                              lancedbpolicy.StatusOK,
		ProviderCallGateReady:               true,
		ProviderCallAllowedNow:              false,
		SentToProvider:                      false,
		ImplementationAllowsProviderCallNow: false,
		BlockedReason:                       retrievalcontext.MaterializedInjectionExecutionEnableBlockedReason,
		ProviderRunPlanSHA256:               providerRunPlanSHA,
		PayloadReportSHA256:                 payloadReportSHA,
		PayloadOutputSHA256:                 payloadOutputSHA,
		MaterializedSHA256:                  providerRunPlan.MaterializedSHA256,
		AssembledOutputSHA256:               providerRunPlan.AssembledOutputSHA256,
		ConfirmFlagUsed:                     true,
	}
	writeJSONArtifact(t, providerCallGatePath, gate)
	gateData, err := os.ReadFile(providerCallGatePath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	gateSHA := sha256HexBytes(gateData)

	const readinessReportPath = "materialized-injection-provider-call-readiness-report.json"
	readiness := retrievalcontext.ProviderCallReadinessReportResult{
		Status:                              lancedbpolicy.StatusOK,
		ProviderCallReadinessReady:          true,
		ProviderCallAllowedNow:              false,
		SentToProvider:                      false,
		ImplementationAllowsProviderCallNow: false,
		BlockedReason:                       retrievalcontext.MaterializedInjectionExecutionEnableBlockedReason,
		ProviderRunPlanReady:                true,
		PayloadReportValidated:              true,
		ProviderCallGateReady:               true,
		ProviderRunPlanSHA256:               providerRunPlanSHA,
		PayloadReportSHA256:                 payloadReportSHA,
		ProviderCallGateSHA256:              gateSHA,
		PayloadOutputSHA256:                 payloadOutputSHA,
		MaterializedSHA256:                  providerRunPlan.MaterializedSHA256,
		AssembledOutputSHA256:               providerRunPlan.AssembledOutputSHA256,
	}
	writeJSONArtifact(t, readinessReportPath, readiness)

	const dispatchConfigPath = "materialized-injection-dispatch.yaml"
	if err := os.WriteFile(dispatchConfigPath, []byte(validDispatchConfigYAML), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	return providerCallChainArtifacts{
		dispatchConfigPath:    dispatchConfigPath,
		payloadReportPath:     payloadReportPath,
		gatePath:              providerCallGatePath,
		readinessReportPath:   readinessReportPath,
		payloadOutputPath:     payloadOutputPath,
		payloadOutputSHA256:   payloadOutputSHA,
		providerRunPlanSHA256: providerRunPlanSHA,
	}
}

func writeJSONArtifact(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func sha256HexBytes(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
