package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestInjectionGovernanceBundleOKWithValidChain(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)

	result, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts())
	if err != nil {
		t.Fatalf("InjectionGovernanceBundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK && result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("result = %#v, want ok or warning", result)
	}
	if result.ContainsText || result.RunnerExecution || result.ExecutionSupportedNow {
		t.Fatalf("result = %#v, want metadata-only bundle", result)
	}
	if !result.InjectionAuthorizedForFuture {
		t.Fatal("injection_authorized_for_future must be true")
	}
	if result.MaterializedSHA256 == "" || result.PolicySHA256 == "" {
		t.Fatalf("result = %#v, want populated hashes", result)
	}

	bundleData, err := os.ReadFile("injection-governance-bundle.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(bundleData), "text_excerpt") || strings.Contains(string(bundleData), "alpha text") {
		t.Fatal("bundle json leaked materialized text")
	}
	summaryData, err := os.ReadFile("injection-governance-bundle.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(summaryData), "text_excerpt") || strings.Contains(string(summaryData), "alpha text") {
		t.Fatal("bundle summary leaked materialized text")
	}
}

func TestInjectionGovernanceBundleFailsOnPromptPreviewReportFailed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)

	report := readJSONFile(t, "prompt-preview-report.json")
	report["status"] = lancedbpolicy.StatusFailed
	writeJSONFile(t, "prompt-preview-report.json", report)

	_, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts())
	if err == nil || !strings.Contains(err.Error(), "prompt preview report status") {
		t.Fatalf("error = %v, want prompt preview report status failure", err)
	}
}

func TestInjectionGovernanceBundleFailsOnApprovalRequestSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)
	if err := os.WriteFile("injection-approval-request.json", []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts())
	if err == nil || !strings.Contains(err.Error(), "request_sha256 mismatch") {
		t.Fatalf("error = %v, want request_sha256 mismatch", err)
	}
}

func TestInjectionGovernanceBundleFailsOnPolicyHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)

	approval := readJSONFile(t, "injection-approval.json")
	approval["policy_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, "injection-approval.json", approval)

	_, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts())
	if err == nil || !strings.Contains(err.Error(), "policy_sha256 mismatch") {
		t.Fatalf("error = %v, want policy_sha256 mismatch", err)
	}
}

func TestInjectionGovernanceBundleFailsOnPromptPreviewHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)

	report := readJSONFile(t, "prompt-preview-report.json")
	hashes, _ := report["hashes"].(map[string]any)
	hashes["prompt_preview_sha256"] = strings.Repeat("0", 64)
	report["hashes"] = hashes
	writeJSONFile(t, "prompt-preview-report.json", report)

	_, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts())
	if err == nil || !strings.Contains(err.Error(), "prompt_preview_sha256 mismatch") {
		t.Fatalf("error = %v, want prompt_preview_sha256 mismatch", err)
	}
}

func TestInjectionGovernanceBundleFailsWhenWouldExecuteRunnerTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)

	plan := readJSONFile(t, "injection-execution-plan.json")
	plan["would_execute_runner"] = true
	writeJSONFile(t, "injection-execution-plan.json", plan)

	_, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts())
	if err == nil || !strings.Contains(err.Error(), "would_execute_runner must be false") {
		t.Fatalf("error = %v, want would_execute_runner failure", err)
	}
}

func TestInjectionGovernanceBundleRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionGovernanceBundleArtifacts(t)

	opts := injectionGovernanceBundleOpts()
	opts.OutputPath = "/tmp/injection-governance-bundle.json"
	_, err := retrievalcontext.InjectionGovernanceBundle(opts)
	if err == nil {
		t.Fatal("expected blocked output path error")
	}
}

func setupInjectionGovernanceBundleArtifacts(t *testing.T) {
	t.Helper()
	setupPromptPreviewReportArtifacts(t)

	report, err := retrievalcontext.PromptPreviewReport(promptPreviewReportOpts())
	if err != nil {
		t.Fatalf("PromptPreviewReport() error = %v", err)
	}
	var reportBuf bytes.Buffer
	if err := retrievalcontext.WritePromptPreviewReportJSON(report, &reportBuf); err != nil {
		t.Fatalf("WritePromptPreviewReportJSON() error = %v", err)
	}
	if err := os.WriteFile("prompt-preview-report.json", reportBuf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func injectionGovernanceBundleOpts() retrievalcontext.InjectionGovernanceBundleOptions {
	return retrievalcontext.InjectionGovernanceBundleOptions{
		GovernanceReportPath:         "governance-report.json",
		PolicyPath:                   "injection-policy.yaml",
		InjectionApprovalRequestPath: "injection-approval-request.json",
		InjectionApprovalPath:        "injection-approval.json",
		ExecutionPlanPath:            "injection-execution-plan.json",
		PromptPreviewManifestPath:    "prompt-preview.json",
		PromptPreviewReportPath:      "prompt-preview-report.json",
		OutputPath:                   "injection-governance-bundle.json",
		SummaryPath:                  "injection-governance-bundle.md",
	}
}
