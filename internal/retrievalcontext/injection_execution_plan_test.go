package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"gopkg.in/yaml.v3"
)

func TestInjectionExecutionPlanOKWithValidChain(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionExecutionPlanArtifacts(t)

	result, err := retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts())
	if err != nil {
		t.Fatalf("InjectionExecutionPlan() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning from policy plan probe", result.Status)
	}
	if result.WouldExecuteRunner || result.ExecutionSupportedNow {
		t.Fatalf("result = %#v, want no runner execution", result)
	}
	if !result.WouldInjectMaterializedContext {
		t.Fatal("would_inject_materialized_context must be true")
	}
	if result.Reason != retrievalcontext.InjectionExecutionPlanReasonExecutionPlanOnly {
		t.Fatalf("reason = %q", result.Reason)
	}
	if result.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q", result.RequiredFutureFlag)
	}

	planJSON, err := os.ReadFile("injection-execution-plan.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(planJSON), `"text_excerpt"`) {
		t.Fatal("execution plan json contains text_excerpt")
	}
	summary, err := os.ReadFile("injection-execution-plan.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(summary), "text_excerpt") || strings.Contains(string(summary), "alpha text") {
		t.Fatal("execution plan summary leaked materialized text")
	}
}

func TestInjectionExecutionPlanFailsWhenRunnerInjectionNotAllowed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionExecutionPlanArtifacts(t)

	payload := readJSONFile(t, "injection-approval.json")
	payload["runner_injection_allowed"] = false
	writeJSONFile(t, "injection-approval.json", payload)

	_, err := retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "runner_injection_allowed must be true") {
		t.Fatalf("error = %v, want runner injection not allowed failure", err)
	}
}

func TestInjectionExecutionPlanFailsOnRequestSHAMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionExecutionPlanArtifacts(t)
	if err := os.WriteFile("tampered-request.json", []byte(`{"status":"pending"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	opts := injectionExecutionPlanOpts()
	opts.InjectionApprovalRequestPath = "tampered-request.json"
	_, err := retrievalcontext.InjectionExecutionPlan(opts)
	if err == nil || !strings.Contains(err.Error(), "request_sha256 mismatch") {
		t.Fatalf("error = %v, want request sha mismatch", err)
	}
}

func TestInjectionExecutionPlanFailsOnPolicyHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionExecutionPlanArtifacts(t)

	cfg, err := retrievalcontext.LoadInjectionPolicy("injection-policy.yaml")
	if err != nil {
		t.Fatalf("LoadInjectionPolicy() error = %v", err)
	}
	cfg.RetrievalInjectionPolicy.Prompt.SectionTitle = "Tampered section title"
	writeInjectionPolicyYAMLFromConfig(t, "injection-policy.yaml", cfg)

	_, err = retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "policy_sha256 mismatch") {
		t.Fatalf("error = %v, want policy hash mismatch", err)
	}
}

func TestInjectionExecutionPlanFailsOnMaterializedHashMismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionExecutionPlanArtifacts(t)

	payload := readJSONFile(t, "injection-approval.json")
	payload["materialized_sha256"] = strings.Repeat("0", 64)
	writeJSONFile(t, "injection-approval.json", payload)

	_, err := retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "materialized_sha256 mismatch") {
		t.Fatalf("error = %v, want materialized hash mismatch", err)
	}
}

func TestInjectionExecutionPlanFailsWhenMaterializedCharsExceedPolicyCaps(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)
	writeInjectionPolicyYAMLWithLimits(t, "injection-policy.yaml", 1, 1, 8)
	writeGovernanceReportFile(t, "governance-report.json")
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

	_, err := retrievalcontext.InjectionExecutionPlan(injectionExecutionPlanOpts())
	if err == nil || !strings.Contains(err.Error(), "exceeds policy max_total_chars") {
		t.Fatalf("error = %v, want policy char cap failure", err)
	}
}

func TestInjectionExecutionPlanRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupInjectionExecutionPlanArtifacts(t)

	opts := injectionExecutionPlanOpts()
	opts.PolicyPath = "/tmp/policy.yaml"
	_, err := retrievalcontext.InjectionExecutionPlan(opts)
	if err == nil {
		t.Fatal("expected blocked policy path error")
	}
}

func setupInjectionExecutionPlanArtifacts(t *testing.T) {
	t.Helper()
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
}

func injectionExecutionPlanOpts() retrievalcontext.InjectionExecutionPlanOptions {
	return retrievalcontext.InjectionExecutionPlanOptions{
		PolicyPath:                   "injection-policy.yaml",
		GovernanceReportPath:         "governance-report.json",
		InjectionApprovalPath:        "injection-approval.json",
		InjectionApprovalRequestPath: "injection-approval-request.json",
		MaterializedPath:             "materialized.json",
		OutputPath:                   "injection-execution-plan.json",
		SummaryPath:                  "injection-execution-plan.md",
	}
}

func writeInjectionPolicyYAMLWithLimits(t *testing.T, path string, maxTotal, maxPerChunk, maxChunks int) {
	t.Helper()
	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Approval.ApprovalPath = "approval.json"
	cfg.RetrievalInjectionPolicy.Approval.RequestPath = "approval-request.json"
	cfg.RetrievalInjectionPolicy.Bundle.Path = "bundle.json"
	cfg.RetrievalInjectionPolicy.Materialized.Path = "materialized.json"
	cfg.RetrievalInjectionPolicy.Limits.MaxTotalChars = maxTotal
	cfg.RetrievalInjectionPolicy.Limits.MaxCharsPerChunk = maxPerChunk
	cfg.RetrievalInjectionPolicy.Limits.MaxChunks = maxChunks
	writeInjectionPolicyYAMLFromConfig(t, path, cfg)
}

func writeGovernanceReportFile(t *testing.T, path string) {
	t.Helper()
	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportJSON(result, &buf); err != nil {
		t.Fatalf("WriteGovernanceReportJSON() error = %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func writeInjectionPolicyYAMLFromConfig(t *testing.T, path string, cfg retrievalcontext.InjectionPolicyConfig) {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("yaml.Marshal() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
