package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"gopkg.in/yaml.v3"
)

func TestNewInjectionApprovalRequestOKWithGovernanceAndPolicy(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)

	request, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts())
	if err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}
	if request.Status != retrievalcontext.RequestStatusPending || request.ContainsText {
		t.Fatalf("request = %#v, want pending without text", request)
	}
	if !request.RequestedRunnerInjectionAllowed {
		t.Fatal("requested_runner_injection_allowed must be true")
	}
	raw, err := os.ReadFile("injection-approval-request.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), `"text_excerpt"`) {
		t.Fatal("injection approval request contains text_excerpt")
	}
}

func TestNewInjectionApprovalRequestFailsWhenGovernanceFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)

	payload := readJSONFile(t, "governance-report.json")
	payload["status"] = lancedbpolicy.StatusFailed
	writeJSONFile(t, "governance-report.json", payload)

	_, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts())
	if err == nil || !strings.Contains(err.Error(), "governance report status") {
		t.Fatalf("error = %v, want governance failed rejection", err)
	}
}

func TestNewInjectionApprovalRequestAcceptsGovernanceWarning(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	longText := strings.Repeat("x", 40)
	payload := validRetrievalArtifactMap(t)
	attachments := payload["attachments"].([]map[string]any)
	hits := attachments[0]["hits"].([]map[string]any)
	hits[0]["text_sha256"] = textSHA(longText)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, payload))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", longText, "sha-source")})
	opts := materializeOpts()
	opts.MaxCharsPerChunk = 10
	if _, err := retrievalcontext.Materialize(opts); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if _, err := retrievalcontext.Bundle(bundleOpts()); err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	}); err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}
	if _, err := retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath:                       "approval-request.json",
		OutputPath:                        "approval.json",
		ConfirmApproveMaterializedContext: true,
	}); err != nil {
		t.Fatalf("ApproveMaterializedContext() error = %v", err)
	}
	if _, err := retrievalcontext.InjectionPlan(injectionPlanOpts()); err != nil {
		t.Fatalf("InjectionPlan() error = %v", err)
	}
	writeInjectionPolicyYAML(t, "injection-policy.yaml")

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportJSON(result, &buf); err != nil {
		t.Fatalf("WriteGovernanceReportJSON() error = %v", err)
	}
	if err := os.WriteFile("governance-report.json", buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	request, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts())
	if err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}
	if len(request.Warnings) == 0 {
		t.Fatal("expected warnings from warning governance chain")
	}
}

func TestApproveRunnerInjectionRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)
	if _, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts()); err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}

	_, err := retrievalcontext.ApproveRunnerInjection(retrievalcontext.ApproveRunnerInjectionOptions{
		RequestPath: "injection-approval-request.json",
		OutputPath:  "injection-approval.json",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-allow-runner-injection") {
		t.Fatalf("error = %v, want confirm flag required", err)
	}
}

func TestApproveRunnerInjectionSetsRunnerInjectionAllowedTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)
	if _, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts()); err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}

	approval, err := retrievalcontext.ApproveRunnerInjection(retrievalcontext.ApproveRunnerInjectionOptions{
		RequestPath:                 "injection-approval-request.json",
		OutputPath:                  "injection-approval.json",
		ConfirmAllowRunnerInjection: true,
		ApprovedAt:                  time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("ApproveRunnerInjection() error = %v", err)
	}
	if !approval.RunnerInjectionAllowed || approval.AllowedUse != retrievalcontext.AllowedUseRunnerInjectionPolicyOnly {
		t.Fatalf("approval = %#v, want runner injection authorized", approval)
	}
	if !approval.ConfirmAllowRunnerInjection {
		t.Fatal("confirm_allow_runner_injection must be true")
	}

	raw, err := os.ReadFile("injection-approval.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), `"text_excerpt"`) {
		t.Fatal("runner injection approval contains text_excerpt")
	}

	oldApproval, err := retrievalcontext.LoadMaterializedContextApproval("approval.json")
	if err != nil {
		t.Fatalf("LoadMaterializedContextApproval() error = %v", err)
	}
	if oldApproval.RunnerInjectionAllowed || oldApproval.AllowedUse != retrievalcontext.AllowedUseManualReviewOnly {
		t.Fatalf("materialized approval must remain manual_review_only: %#v", oldApproval)
	}
}

func TestInspectInjectionApprovalOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)
	if _, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts()); err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}
	if _, err := retrievalcontext.ApproveRunnerInjection(retrievalcontext.ApproveRunnerInjectionOptions{
		RequestPath:                 "injection-approval-request.json",
		OutputPath:                  "injection-approval.json",
		ConfirmAllowRunnerInjection: true,
	}); err != nil {
		t.Fatalf("ApproveRunnerInjection() error = %v", err)
	}

	result, err := retrievalcontext.InspectInjectionApproval("injection-approval.json", retrievalcontext.InspectInjectionApprovalOptions{
		RequestPath: "injection-approval-request.json",
	})
	if err != nil {
		t.Fatalf("InspectInjectionApproval() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || !result.RunnerInjectionAllowed {
		t.Fatalf("result = %#v, want ok inspect with runner injection allowed", result)
	}

	var stdout bytes.Buffer
	if err := retrievalcontext.WriteInspectInjectionApprovalText(result, &stdout); err != nil {
		t.Fatalf("WriteInspectInjectionApprovalText() error = %v", err)
	}
	if strings.Contains(stdout.String(), "text_excerpt") {
		t.Fatal("inspect text contains text_excerpt")
	}
}

func TestInspectInjectionApprovalFailsOnRequestSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)
	if _, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts()); err != nil {
		t.Fatalf("NewInjectionApprovalRequest() error = %v", err)
	}
	if _, err := retrievalcontext.ApproveRunnerInjection(retrievalcontext.ApproveRunnerInjectionOptions{
		RequestPath:                 "injection-approval-request.json",
		OutputPath:                  "injection-approval.json",
		ConfirmAllowRunnerInjection: true,
	}); err != nil {
		t.Fatalf("ApproveRunnerInjection() error = %v", err)
	}
	if err := os.WriteFile("tampered-request.json", []byte(`{"status":"pending"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := retrievalcontext.InspectInjectionApproval("injection-approval.json", retrievalcontext.InspectInjectionApprovalOptions{
		RequestPath: "tampered-request.json",
	})
	if err != nil {
		t.Fatalf("InspectInjectionApproval() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "request_sha256 mismatch") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestNewInjectionApprovalRequestRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)

	opts := injectionApprovalRequestOpts()
	opts.GovernanceReportPath = "/tmp/governance-report.json"
	_, err := retrievalcontext.NewInjectionApprovalRequest(opts)
	if err == nil {
		t.Fatal("expected blocked governance report path error")
	}
}

func writeInjectionApprovalTestArtifacts(t *testing.T) {
	t.Helper()
	setupGovernanceArtifacts(t)
	writeInjectionPolicyYAML(t, "injection-policy.yaml")

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportJSON(result, &buf); err != nil {
		t.Fatalf("WriteGovernanceReportJSON() error = %v", err)
	}
	if err := os.WriteFile("governance-report.json", buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func injectionApprovalRequestOpts() retrievalcontext.NewInjectionApprovalRequestOptions {
	return retrievalcontext.NewInjectionApprovalRequestOptions{
		GovernanceReportPath: "governance-report.json",
		PolicyPath:           "injection-policy.yaml",
		OutputPath:           "injection-approval-request.json",
	}
}

func writeInjectionPolicyYAML(t *testing.T, path string) {
	t.Helper()
	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Approval.ApprovalPath = "approval.json"
	cfg.RetrievalInjectionPolicy.Approval.RequestPath = "approval-request.json"
	cfg.RetrievalInjectionPolicy.Bundle.Path = "bundle.json"
	cfg.RetrievalInjectionPolicy.Materialized.Path = "materialized.json"
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func TestNewInjectionApprovalRequestFailsWhenPolicyPlanFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)

	bundlePayload := readJSONFile(t, "bundle.json")
	bundlePayload["status"] = lancedbpolicy.StatusFailed
	writeJSONFile(t, "bundle.json", bundlePayload)

	_, err := retrievalcontext.NewInjectionApprovalRequest(injectionApprovalRequestOpts())
	if err == nil || !strings.Contains(err.Error(), "injection policy plan status") {
		t.Fatalf("error = %v, want policy plan failed rejection", err)
	}
}

func TestLoadGovernanceReportFromFile(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeInjectionApprovalTestArtifacts(t)

	report, _, err := retrievalcontext.LoadGovernanceReport("governance-report.json")
	if err != nil {
		t.Fatalf("LoadGovernanceReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", report.Status)
	}
}

func TestRunnerInjectionApprovalRejectsTextExcerptField(t *testing.T) {
	data := []byte(`{"approved":true,"text_excerpt":"leak"}` + "\n")
	result, err := retrievalcontext.InspectInjectionApprovalBytes(data, retrievalcontext.InspectInjectionApprovalOptions{})
	if err != nil {
		t.Fatalf("InspectInjectionApprovalBytes() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}
