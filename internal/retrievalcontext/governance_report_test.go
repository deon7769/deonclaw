package retrievalcontext_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestGovernanceReportOKWithValidChain(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || result.CanInjectNow {
		t.Fatalf("result = %#v, want ok governance report with injection disabled", result)
	}
	if result.RequiredFutureFlag != retrievalcontext.RequiredFutureInjectFlag {
		t.Fatalf("required_future_flag = %q", result.RequiredFutureFlag)
	}
	if len(result.Stages) != 6 {
		t.Fatalf("stages = %#v, want 6 stages", result.Stages)
	}

	var stdout bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportText(result, &stdout); err != nil {
		t.Fatalf("WriteGovernanceReportText() error = %v", err)
	}
	if strings.Contains(stdout.String(), "alpha text") || strings.Contains(stdout.String(), "text_excerpt") {
		t.Fatal("governance report text leaked materialized content")
	}

	var buf bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportJSON(result, &buf); err != nil {
		t.Fatalf("WriteGovernanceReportJSON() error = %v", err)
	}
	if strings.Contains(buf.String(), "text_excerpt") || strings.Contains(buf.String(), "alpha text") {
		t.Fatal("governance report json leaked materialized content")
	}
}

func TestGovernanceReportWarningPropagatedFromMaterializedChain(t *testing.T) {
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

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected propagated warnings")
	}
}

func TestGovernanceReportFailsOnHashMismatches(t *testing.T) {
	cases := []struct {
		name    string
		tamper  func(t *testing.T)
		wantErr string
	}{
		{
			name: "materialized source retrieval sha",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "materialized.json")
				payload["source_retrieval_context_sha256"] = "wrong-sha"
				writeJSONFile(t, "materialized.json", payload)
			},
			wantErr: "materialized source_retrieval_context_sha256 mismatch",
		},
		{
			name: "bundle retrieval sha",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "bundle.json")
				payload["retrieval_context_sha256"] = "wrong-sha"
				writeJSONFile(t, "bundle.json", payload)
			},
			wantErr: "bundle retrieval_context_sha256 mismatch",
		},
		{
			name: "bundle materialized sha",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "bundle.json")
				payload["materialized_sha256"] = "wrong-sha"
				writeJSONFile(t, "bundle.json", payload)
			},
			wantErr: "bundle materialized_sha256 mismatch",
		},
		{
			name: "request bundle sha",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "approval-request.json")
				payload["bundle_sha256"] = "wrong-sha"
				writeJSONFile(t, "approval-request.json", payload)
			},
			wantErr: "request bundle_sha256 mismatch",
		},
		{
			name: "approval request sha",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "approval.json")
				payload["request_sha256"] = "wrong-sha"
				writeJSONFile(t, "approval.json", payload)
			},
			wantErr: "approval request_sha256 mismatch",
		},
		{
			name: "approval bundle sha vs request",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "approval.json")
				payload["bundle_sha256"] = "wrong-sha"
				writeJSONFile(t, "approval.json", payload)
			},
			wantErr: "approval bundle_sha256 mismatch with request",
		},
		{
			name: "approval materialized sha vs request",
			tamper: func(t *testing.T) {
				payload := readJSONFile(t, "approval.json")
				payload["materialized_sha256"] = "wrong-sha"
				writeJSONFile(t, "approval.json", payload)
			},
			wantErr: "approval materialized_sha256 mismatch with request",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			chdir(t, dir)
			setupGovernanceArtifacts(t)
			tc.tamper(t)

			result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
			if err != nil {
				t.Fatalf("GovernanceReport() error = %v", err)
			}
			if result.Status != lancedbpolicy.StatusFailed {
				t.Fatalf("status = %q, want failed", result.Status)
			}
			joined := strings.Join(result.Failures, "; ")
			if !strings.Contains(joined, tc.wantErr) {
				t.Fatalf("failures = %q, want %q", joined, tc.wantErr)
			}
		})
	}
}

func TestGovernanceReportFailsWhenInjectionPlanCanInjectNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	payload := readJSONFile(t, "injection-plan.json")
	payload["can_inject_now"] = true
	writeJSONFile(t, "injection-plan.json", payload)

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "can_inject_now must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestGovernanceReportFailsWhenInjectionPlanReasonDiverges(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	payload := readJSONFile(t, "injection-plan.json")
	payload["reason"] = "unexpected_reason"
	writeJSONFile(t, "injection-plan.json", payload)

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "injection_plan reason") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestGovernanceReportRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	opts := governanceReportOpts()
	opts.RetrievalContextPath = "/tmp/retrieval-context.json"
	_, err := retrievalcontext.GovernanceReport(opts)
	if err == nil {
		t.Fatal("expected blocked retrieval context path error")
	}
}

func setupGovernanceArtifacts(t *testing.T) {
	t.Helper()
	setupApprovalArtifacts(t)
	if _, err := retrievalcontext.InjectionPlan(injectionPlanOpts()); err != nil {
		t.Fatalf("InjectionPlan() error = %v", err)
	}
}

func governanceReportOpts() retrievalcontext.GovernanceReportOptions {
	return retrievalcontext.GovernanceReportOptions{
		RetrievalContextPath: "retrieval-context.json",
		MaterializedPath:     "materialized.json",
		BundlePath:           "bundle.json",
		RequestPath:          "approval-request.json",
		ApprovalPath:         "approval.json",
		InjectionPlanPath:    "injection-plan.json",
	}
}

func TestGovernanceReportJSONHasNoTextExcerptField(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	result, err := retrievalcontext.GovernanceReport(governanceReportOpts())
	if err != nil {
		t.Fatalf("GovernanceReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteGovernanceReportJSON(result, &buf); err != nil {
		t.Fatalf("WriteGovernanceReportJSON() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	encoded, _ := json.Marshal(payload)
	if strings.Contains(string(encoded), "text_excerpt") {
		t.Fatal("governance report json contains text_excerpt")
	}
}
