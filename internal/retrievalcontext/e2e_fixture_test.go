package retrievalcontext_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestRetrievalContextGovernanceFixtureE2E(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	chdir(t, dir)
	copyRetrievalContextFixtureFrom(t, root, dir)

	const (
		retrievalContextPath = "retrieval-context.json"
		chunksPath           = "memory-index-chunks.jsonl"
		materializedPath     = "retrieval-context-materialized.json"
		materializedSummary  = "retrieval-context-materialized.md"
		bundlePath           = "retrieval-context-bundle.json"
		bundleSummary        = "retrieval-context-bundle.md"
		requestPath          = "retrieval-context-approval-request.json"
		approvalPath         = "retrieval-context-approval.json"
		injectionPlanPath    = "retrieval-context-injection-plan.json"
		injectionPlanSummary = "retrieval-context-injection-plan.md"
	)

	inspect, err := retrievalcontext.InspectArtifact(retrievalContextPath)
	if err != nil {
		t.Fatalf("InspectArtifact() error = %v", err)
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("inspect status = %q, want ok", inspect.Status)
	}

	materializeResult, err := retrievalcontext.Materialize(retrievalcontext.MaterializeOptions{
		RetrievalContextPath:    retrievalContextPath,
		ChunksPath:              chunksPath,
		OutputPath:              materializedPath,
		SummaryPath:             materializedSummary,
		MaxCharsPerChunk:        1200,
		MaxTotalChars:           6000,
		ConfirmIncludeChunkText: true,
	})
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if materializeResult.Status != lancedbpolicy.StatusOK {
		t.Fatalf("materialize status = %q, want ok", materializeResult.Status)
	}

	materializedReport, err := retrievalcontext.MaterializedReport(materializedPath)
	if err != nil {
		t.Fatalf("MaterializedReport() error = %v", err)
	}
	if materializedReport.Status != lancedbpolicy.StatusOK {
		t.Fatalf("materialized report status = %q, want ok", materializedReport.Status)
	}

	bundleResult, err := retrievalcontext.Bundle(retrievalcontext.BundleOptions{
		RetrievalContextPath: retrievalContextPath,
		MaterializedPath:     materializedPath,
		OutputPath:           bundlePath,
		SummaryPath:          bundleSummary,
	})
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
	if bundleResult.Status != lancedbpolicy.StatusOK {
		t.Fatalf("bundle status = %q, want ok", bundleResult.Status)
	}

	if _, err := retrievalcontext.NewApprovalRequest(retrievalcontext.NewApprovalRequestOptions{
		BundlePath: bundlePath,
		OutputPath: requestPath,
	}); err != nil {
		t.Fatalf("NewApprovalRequest() error = %v", err)
	}

	if _, err := retrievalcontext.ApproveMaterializedContext(retrievalcontext.ApproveMaterializedContextOptions{
		RequestPath:                       requestPath,
		OutputPath:                        approvalPath,
		ConfirmApproveMaterializedContext: true,
	}); err != nil {
		t.Fatalf("ApproveMaterializedContext() error = %v", err)
	}

	approvalInspect, err := retrievalcontext.InspectApproval(approvalPath, retrievalcontext.InspectApprovalOptions{
		RequestPath: requestPath,
	})
	if err != nil {
		t.Fatalf("InspectApproval() error = %v", err)
	}
	if approvalInspect.Status != lancedbpolicy.StatusOK {
		t.Fatalf("approval inspect status = %q, want ok", approvalInspect.Status)
	}

	injectionPlan, err := retrievalcontext.InjectionPlan(retrievalcontext.InjectionPlanOptions{
		ApprovalPath:     approvalPath,
		RequestPath:      requestPath,
		BundlePath:       bundlePath,
		MaterializedPath: materializedPath,
		OutputPath:       injectionPlanPath,
		SummaryPath:      injectionPlanSummary,
	})
	if err != nil {
		t.Fatalf("InjectionPlan() error = %v", err)
	}
	if injectionPlan.Status != lancedbpolicy.StatusOK || injectionPlan.CanInjectNow {
		t.Fatalf("injection plan = %#v, want ok with injection disabled", injectionPlan)
	}
	if injectionPlan.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q", injectionPlan.RequiredFutureFlag)
	}

	governance, err := retrievalcontext.GovernanceReport(retrievalcontext.GovernanceReportOptions{
		RetrievalContextPath: retrievalContextPath,
		MaterializedPath:     materializedPath,
		BundlePath:           bundlePath,
		RequestPath:          requestPath,
		ApprovalPath:         approvalPath,
		InjectionPlanPath:    injectionPlanPath,
	})
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	if governance.Status != lancedbpolicy.StatusOK {
		t.Fatalf("governance status = %q, want ok", governance.Status)
	}
	if governance.CanInjectNow {
		t.Fatal("governance can_inject_now must be false")
	}
	if governance.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("governance required_future_flag = %q", governance.RequiredFutureFlag)
	}

	assertNoTextExcerpt(t, retrievalContextPath)
	assertNoTextExcerpt(t, bundlePath)
	assertNoTextExcerpt(t, bundleSummary)
	assertNoTextExcerpt(t, requestPath)
	assertNoTextExcerpt(t, approvalPath)
	assertNoTextExcerpt(t, injectionPlanPath)
	assertNoTextExcerpt(t, injectionPlanSummary)

	materializedData, err := os.ReadFile(materializedPath)
	if err != nil {
		t.Fatalf("ReadFile(materialized) error = %v", err)
	}
	if !strings.Contains(string(materializedData), `"text_excerpt"`) {
		t.Fatal("materialized artifact should contain text_excerpt")
	}

	var governanceStdout bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportText(governance, &governanceStdout); err != nil {
		t.Fatalf("WriteGovernanceReportText() error = %v", err)
	}
	if strings.Contains(governanceStdout.String(), "alpha text") || strings.Contains(governanceStdout.String(), "text_excerpt") {
		t.Fatal("governance report text leaked materialized content")
	}

	assertFixtureInjectionPolicyE2E(t, root, dir, "retrieval-injection-policy.yaml")
}

func assertFixtureInjectionPolicyE2E(t *testing.T, root, dir, policyName string) {
	t.Helper()
	policyPath := filepath.Join(dir, policyName)
	copyFixtureFile(t, filepath.Join(root, "configs", "examples", "retrieval-context-fixture", policyName), policyPath)

	cfg, err := retrievalcontext.LoadInjectionPolicy(policyPath)
	if err != nil {
		t.Fatalf("LoadInjectionPolicy() error = %v", err)
	}
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err != nil {
		t.Fatalf("ValidateInjectionPolicy() error = %v", err)
	}

	plan, err := retrievalcontext.InjectionPolicyPlan(cfg)
	if err != nil {
		t.Fatalf("InjectionPolicyPlan() error = %v", err)
	}
	if plan.WouldInject {
		t.Fatal("would_inject must be false")
	}
	if plan.Reason != retrievalcontext.InjectionPolicyReasonSchemaOnly {
		t.Fatalf("reason = %q, want %q", plan.Reason, retrievalcontext.InjectionPolicyReasonSchemaOnly)
	}
	if plan.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", plan.Status)
	}
	if !strings.Contains(strings.Join(plan.Warnings, "; "), "runner_injection_allowed is false") {
		t.Fatalf("warnings = %#v, want runner_injection_allowed warning", plan.Warnings)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteInjectionPolicyPlanText(plan, &textBuf); err != nil {
		t.Fatalf("WriteInjectionPolicyPlanText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("injection policy plan text leaked materialized content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteInjectionPolicyPlanJSON(plan, &jsonBuf); err != nil {
		t.Fatalf("WriteInjectionPolicyPlanJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") {
		t.Fatal("injection policy plan json contains text_excerpt")
	}
}

func copyRetrievalContextFixtureFrom(t *testing.T, root, dstDir string) {
	t.Helper()
	srcDir := filepath.Join(root, "configs", "examples", "retrieval-context-fixture")
	for _, name := range []string{"retrieval-context.json", "memory-index-chunks.jsonl"} {
		copyFixtureFile(t, filepath.Join(srcDir, name), filepath.Join(dstDir, name))
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func copyFixtureFile(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("Open(%q) error = %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("Create(%q) error = %v", dst, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("Copy(%q -> %q) error = %v", src, dst, err)
	}
}

func assertNoTextExcerpt(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if strings.Contains(string(data), "text_excerpt") {
		t.Fatalf("%q must not contain text_excerpt", path)
	}
}
