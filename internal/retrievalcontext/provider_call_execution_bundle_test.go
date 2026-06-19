package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestProviderCallExecutionBundleOK(t *testing.T) {
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
		ConfirmPayloadOutputSHA256: chain.payloadOutputSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderCall() error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath:   chain.dispatchConfigPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessReportPath,
		ApprovalPath:         "provider-call-approval.json",
		OutputPath:           "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if !result.ProviderCallAuthorizedForFuture {
		t.Fatalf("result = %#v, want provider_call_authorized_for_future", result)
	}
	if result.ProviderCallAllowedNow || result.SentToProvider || result.NetworkAllowed || result.RunnerExecution {
		t.Fatalf("result = %#v, want blocked execution flags", result)
	}
	if result.ContainsText {
		t.Fatalf("result = %#v, want contains_text false", result)
	}
	if result.DispatchConfigSHA256 == "" || result.ApprovalSHA256 == "" || result.PayloadOutputSHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	outputData, err := os.ReadFile("provider-call-execution-bundle.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(outputData), "text_excerpt") || strings.Contains(string(outputData), "alpha text") {
		t.Fatal("execution bundle leaked preview content")
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallExecutionBundleText(result, &textBuf); err != nil {
		t.Fatalf("WriteProviderCallExecutionBundleText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("execution bundle text leaked preview content")
	}
}

func TestProviderCallExecutionBundleFailsWhenDispatchAllowsNetwork(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainArtifacts(t)

	badDispatch := strings.Replace(validDispatchConfigYAML, "allow_network: false", "allow_network: true", 1)
	if err := os.WriteFile(chain.dispatchConfigPath, []byte(badDispatch), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

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
		ConfirmPayloadOutputSHA256: chain.payloadOutputSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderCall() error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath:   chain.dispatchConfigPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessReportPath,
		ApprovalPath:         "provider-call-approval.json",
		OutputPath:           "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("result = %#v, want failed", result)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_network") {
		t.Fatalf("failures = %#v, want allow_network failure", result.Failures)
	}
}

func TestProviderCallExecutionBundleFailsWhenApprovalHashMismatch(t *testing.T) {
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
	approval, err := retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath:                "provider-call-approval-request.json",
		OutputPath:                 "provider-call-approval.json",
		ConfirmPayloadOutputSHA256: chain.payloadOutputSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderCall() error = %v", err)
	}
	approval.PayloadOutputSHA256 = "deadbeef"
	writeJSONArtifact(t, "provider-call-approval.json", approval)

	result, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath:   chain.dispatchConfigPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessReportPath,
		ApprovalPath:         "provider-call-approval.json",
		OutputPath:           "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("result = %#v, want failed", result)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "payload_output_sha256 mismatch") {
		t.Fatalf("failures = %#v, want payload_output_sha256 mismatch", result.Failures)
	}
}

func TestProviderCallExecutionBundleDoesNotReadPayloadOutput(t *testing.T) {
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
		ConfirmPayloadOutputSHA256: chain.payloadOutputSHA256,
	})
	if err != nil {
		t.Fatalf("ApproveProviderCall() error = %v", err)
	}
	if err := os.Remove(chain.payloadOutputPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath:   chain.dispatchConfigPath,
		PayloadReportPath:    chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath:  chain.readinessReportPath,
		ApprovalPath:         "provider-call-approval.json",
		OutputPath:           "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok without payload-output file", result)
	}
}
