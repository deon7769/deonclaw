package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type fullProviderCallChain struct {
	dispatchPath        string
	runPlanPath         string
	dryRunPath          string
	payloadOutputPath   string
	payloadReportPath   string
	gatePath            string
	readinessPath       string
	approvalRequestPath string
	approvalPath        string
	payloadSHA256       string
}

func setupFullProviderCallChain(t *testing.T) fullProviderCallChain {
	t.Helper()
	dispatchPath, runPlanPath, assembledPath := setupMaterializedProviderPayloadArtifacts(t)

	const dryRunPath = "materialized-provider-payload-dry-run.json"
	const payloadOutputPath = "materialized-provider-payload.md"
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

	report, err := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: dryRunPath, PayloadOutputPath: payloadOutputPath,
	})
	if err != nil {
		t.Fatalf("MaterializedProviderPayloadReport() error = %v", err)
	}
	const payloadReportPath = "materialized-provider-payload-report.json"
	var reportBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderPayloadReportJSON(report, &reportBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderPayloadReportJSON() error = %v", err)
	}
	if err := os.WriteFile(payloadReportPath, reportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	const gatePath = "materialized-provider-call-gate.json"
	if _, err := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath: dispatchPath, PayloadReportPath: payloadReportPath, PayloadOutputPath: payloadOutputPath,
		ConfirmInjectMaterializedContext: true, OutputPath: gatePath,
	}); err != nil {
		t.Fatalf("MaterializedProviderCallGate() error = %v", err)
	}

	readiness, err := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
	})
	if err != nil {
		t.Fatalf("MaterializedProviderCallReadinessReport() error = %v", err)
	}
	const readinessPath = "materialized-provider-call-readiness-report.json"
	var readinessBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderCallReadinessReportJSON(readiness, &readinessBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderCallReadinessReportJSON() error = %v", err)
	}
	if err := os.WriteFile(readinessPath, readinessBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	const approvalRequestPath = "provider-call-approval-request.json"
	if _, err := retrievalcontext.NewProviderCallApprovalRequest(retrievalcontext.NewProviderCallApprovalRequestOptions{
		ReadinessReportPath: readinessPath, ProviderCallGatePath: gatePath, PayloadReportPath: payloadReportPath,
		OutputPath: approvalRequestPath,
	}); err != nil {
		t.Fatalf("NewProviderCallApprovalRequest() error = %v", err)
	}
	const approvalPath = "provider-call-approval.json"
	if _, err := retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath: approvalRequestPath, OutputPath: approvalPath, ConfirmProviderPayloadSHA256: report.ProviderPayloadSHA256,
	}); err != nil {
		t.Fatalf("ApproveProviderCall() error = %v", err)
	}

	return fullProviderCallChain{
		dispatchPath: dispatchPath, runPlanPath: runPlanPath, dryRunPath: dryRunPath,
		payloadOutputPath: payloadOutputPath, payloadReportPath: payloadReportPath, gatePath: gatePath,
		readinessPath: readinessPath, approvalRequestPath: approvalRequestPath, approvalPath: approvalPath,
		payloadSHA256: report.ProviderPayloadSHA256,
	}
}

func TestProviderCallApprovalRequestApproveInspectOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupFullProviderCallChain(t)

	inspect, err := retrievalcontext.InspectProviderCallApproval(chain.approvalPath, retrievalcontext.InspectProviderCallApprovalOptions{
		RequestPath: chain.approvalRequestPath,
	})
	if err != nil {
		t.Fatalf("InspectProviderCallApproval() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect = %#v, want ok", inspect)
	}
	if inspect.ProviderCallAllowedNow || inspect.NetworkCallAllowedNow || inspect.WorkerExecutionAllowedNow || inspect.SentToProvider {
		t.Fatalf("inspect = %#v, want blocked flags", inspect)
	}
}

func TestApproveProviderCallFailsOnHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupFullProviderCallChain(t)

	_, err := retrievalcontext.ApproveProviderCall(retrievalcontext.ApproveProviderCallOptions{
		RequestPath: chain.approvalRequestPath, OutputPath: "bad-approval.json", ConfirmProviderPayloadSHA256: "deadbeef",
	})
	if err == nil {
		t.Fatal("ApproveProviderCall() want error")
	}
}

func TestProviderCallExecutionBundleOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupFullProviderCallChain(t)

	result, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath: chain.dispatchPath, PayloadReportPath: chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath, ReadinessReportPath: chain.readinessPath,
		ApprovalPath: chain.approvalPath, OutputPath: "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}
	if !result.ProviderCallAuthorizedForFuture || result.ProviderCall || result.NetworkCall || result.WorkerExecution || result.SentToProvider {
		t.Fatalf("result = %#v, want authorized future only", result)
	}
}

func TestProviderCallChainContinuityAuditOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupFullProviderCallChain(t)

	_, err := retrievalcontext.ProviderCallExecutionBundle(retrievalcontext.ProviderCallExecutionBundleOptions{
		DispatchConfigPath: chain.dispatchPath, PayloadReportPath: chain.payloadReportPath,
		ProviderCallGatePath: chain.gatePath, ReadinessReportPath: chain.readinessPath,
		ApprovalPath: chain.approvalPath, OutputPath: "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallExecutionBundle() error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallChainContinuityAudit(retrievalcontext.ProviderCallChainContinuityAuditOptions{
		DispatchConfigPath: chain.dispatchPath, ProviderRunPlanPath: chain.runPlanPath,
		PayloadDryRunPath: chain.dryRunPath, PayloadOutputPath: chain.payloadOutputPath,
		PayloadReportPath: chain.payloadReportPath, ProviderCallGatePath: chain.gatePath,
		ReadinessReportPath: chain.readinessPath, ApprovalRequestPath: chain.approvalRequestPath,
		ApprovalPath: chain.approvalPath, ExecutionBundlePath: "provider-call-execution-bundle.json",
	})
	if err != nil {
		t.Fatalf("ProviderCallChainContinuityAudit() error = %v", err)
	}
	if !result.ChainContinuityReady || !result.LoadersReconciled {
		t.Fatalf("result = %#v, want continuity ready", result)
	}
	if result.ProviderCall || result.NetworkCall || result.WorkerExecution || result.SentToProvider {
		t.Fatalf("result = %#v, want blocked flags", result)
	}
}
