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
)

func TestNewApprovalRequestOKWithBundleOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)

	request, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	})
	if err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}
	if request.Status != retrievalcontext.RequestStatusPending || request.ContainsText {
		t.Fatalf("request = %#v, want pending without text", request)
	}
	raw, err := os.ReadFile("approval-request.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), `"text_excerpt"`) {
		t.Fatal("approval request contains text_excerpt")
	}
}

func TestNewApprovalRequestOKWithBundleWarning(t *testing.T) {
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

	request, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	})
	if err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}
	if len(request.Warnings) == 0 {
		t.Fatal("expected warnings from warning bundle")
	}
}

func TestNewApprovalRequestFailsWithBundleFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if _, err := retrievalcontext.Bundle(bundleOpts()); err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
	bundlePayload := readJSONFile(t, "bundle.json")
	bundlePayload["status"] = lancedbpolicy.StatusFailed
	writeJSONFile(t, "bundle.json", bundlePayload)

	_, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	})
	if err == nil || !strings.Contains(err.Error(), "bundle status") {
		t.Fatalf("error = %v, want bundle failed", err)
	}
}

func TestApproveRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)
	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	}); err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}

	_, err := retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath: "approval-request.json",
		OutputPath:  "approval.json",
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-approve-materialized-context") {
		t.Fatalf("error = %v, want confirm flag required", err)
	}
}

func TestApproveDoesNotContainTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)
	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	}); err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}

	approval, err := retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath:                       "approval-request.json",
		OutputPath:                        "approval.json",
		ConfirmApproveMaterializedContext: true,
	})
	if err != nil {
		t.Fatalf("ApproveMaterializedContext() error = %v", err)
	}
	if !approval.Approved || approval.RunnerInjectionAllowed {
		t.Fatalf("approval = %#v, want approved without runner injection", approval)
	}
	raw, err := os.ReadFile("approval.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), `"text_excerpt"`) {
		t.Fatal("approval contains text_excerpt")
	}
}

func TestInspectApprovalOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)
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

	result, err := retrievalcontext.InspectApproval("approval.json", retrievalcontext.InspectApprovalOptions{
		RequestPath: "approval-request.json",
	})
	if err != nil {
		t.Fatalf("InspectApproval() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || !result.Approved {
		t.Fatalf("result = %#v, want ok approved", result)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteInspectApprovalText(result, &buf); err != nil {
		t.Fatalf("WriteInspectApprovalText() error = %v", err)
	}
	if strings.Contains(buf.String(), "alpha text") || strings.Contains(buf.String(), "text_excerpt") {
		t.Fatal("inspect text leaked materialized content")
	}
}

func TestInspectApprovalFailsOnRequestSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)
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

	payload := readJSONFile(t, "approval-request.json")
	payload["included_chunk_count"] = 99
	writeJSONFile(t, "approval-request.json", payload)

	result, err := retrievalcontext.InspectApproval("approval.json", retrievalcontext.InspectApprovalOptions{
		RequestPath: "approval-request.json",
	})
	if err != nil {
		t.Fatalf("InspectApproval() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestApprovalPathHardening(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)

	_, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "/tmp/bundle.json",
		OutputPath: "approval-request.json",
	})
	if err == nil {
		t.Fatal("expected blocked bundle path error")
	}

	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "secrets/request.json",
	}); err == nil {
		t.Fatal("expected blocked output path error")
	}

	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	}); err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}

	_, err = retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath:                       "../approval-request.json",
		OutputPath:                        "approval.json",
		ConfirmApproveMaterializedContext: true,
	})
	if err == nil {
		t.Fatal("expected blocked request path error")
	}
}

func setupBundleArtifacts(t *testing.T) {
	t.Helper()
	writeRetrievalArtifactFile(t, ".", mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, ".", []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if _, err := retrievalcontext.Bundle(bundleOpts()); err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
}

func TestNewApprovalRequestUsesFixedApprovedAtInApprove(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupBundleArtifacts(t)
	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: "bundle.json",
		OutputPath: "approval-request.json",
	}); err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}
	fixed := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	approval, err := retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath:                       "approval-request.json",
		OutputPath:                        "approval.json",
		ConfirmApproveMaterializedContext: true,
		ApprovedAt:                        fixed,
	})
	if err != nil {
		t.Fatalf("ApproveMaterializedContext() error = %v", err)
	}
	if !approval.ApprovedAt.Equal(fixed) {
		t.Fatalf("approved_at = %v, want %v", approval.ApprovedAt, fixed)
	}
}
