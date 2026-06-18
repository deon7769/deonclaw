package retrievalcontext_test

import (
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestInjectionPlanOKWithValidApprovalButCannotInjectNow(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	result, err := retrievalcontext.InjectionPlan(injectionPlanOpts())
	if err != nil {
		t.Fatalf("InjectionPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || result.CanInjectNow {
		t.Fatalf("result = %#v, want ok plan with injection disabled", result)
	}
	if result.Reason != retrievalcontext.InjectionPlanReasonRunnerInjectionNotAllowed {
		t.Fatalf("reason = %q", result.Reason)
	}
	if result.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q", result.RequiredFutureFlag)
	}

	raw, err := os.ReadFile("injection-plan.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), `"text_excerpt"`) {
		t.Fatal("injection plan json contains text_excerpt")
	}
	summary, err := os.ReadFile("injection-plan.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(summary), "alpha text") || strings.Contains(string(summary), "text_excerpt") {
		t.Fatal("injection plan summary leaked materialized text")
	}
}

func TestInjectionPlanFailsWhenApprovalRequestSHADiverges(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	payload := readJSONFile(t, "approval-request.json")
	payload["included_chunk_count"] = 99
	writeJSONFile(t, "approval-request.json", payload)

	_, err := retrievalcontext.InjectionPlan(injectionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "approval inspect status") {
		t.Fatalf("error = %v, want approval inspect failure", err)
	}
}

func TestInjectionPlanFailsWhenBundleHashDiverges(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	payload := readJSONFile(t, "bundle.json")
	payload["included_chunk_count"] = 99
	writeJSONFile(t, "bundle.json", payload)

	_, err := retrievalcontext.InjectionPlan(injectionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "bundle_sha256 mismatch") {
		t.Fatalf("error = %v, want bundle hash mismatch", err)
	}
}

func TestInjectionPlanFailsWhenMaterializedHashDiverges(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	payload := readJSONFile(t, "materialized.json")
	payload["total_chars_included"] = 99
	writeJSONFile(t, "materialized.json", payload)

	_, err := retrievalcontext.InjectionPlan(injectionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "materialized_sha256 mismatch") {
		t.Fatalf("error = %v, want materialized hash mismatch", err)
	}
}

func TestInjectionPlanFailsWhenApprovalNotApproved(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	payload := readJSONFile(t, "approval.json")
	payload["approved"] = false
	writeJSONFile(t, "approval.json", payload)

	_, err := retrievalcontext.InjectionPlan(injectionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "approval inspect status") {
		t.Fatalf("error = %v, want approval inspect failure", err)
	}
}

func TestInjectionPlanFailsWhenRunnerInjectionAllowedTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	payload := readJSONFile(t, "approval.json")
	payload["runner_injection_allowed"] = true
	writeJSONFile(t, "approval.json", payload)

	_, err := retrievalcontext.InjectionPlan(injectionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "runner_injection") {
		t.Fatalf("error = %v, want runner injection failure", err)
	}
}

func TestInjectionPlanRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupApprovalArtifacts(t)

	opts := injectionPlanOpts()
	opts.ApprovalPath = "/tmp/approval.json"
	_, err := retrievalcontext.InjectionPlan(opts)
	if err == nil {
		t.Fatal("expected blocked approval path error")
	}
}

func setupApprovalArtifacts(t *testing.T) {
	t.Helper()
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
}

func injectionPlanOpts() retrievalcontext.InjectionPlanOptions {
	return retrievalcontext.InjectionPlanOptions{
		ApprovalPath:     "approval.json",
		RequestPath:      "approval-request.json",
		BundlePath:       "bundle.json",
		MaterializedPath: "materialized.json",
		OutputPath:       "injection-plan.json",
		SummaryPath:      "injection-plan.md",
	}
}
