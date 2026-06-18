package retrievalcontext_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestValidateInjectionPolicyOKForExample(t *testing.T) {
	root := repoRoot(t)
	cfg, err := retrievalcontext.LoadInjectionPolicy(filepath.Join(root, "configs/examples/retrieval-injection-policy.yaml"))
	if err != nil {
		t.Fatalf("LoadInjectionPolicy() error = %v", err)
	}
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err != nil {
		t.Fatalf("ValidateInjectionPolicy() error = %v", err)
	}
}

func TestValidateInjectionPolicyFailsWhenModeNotPlanOnly(t *testing.T) {
	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Mode = "execute"
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err == nil || !strings.Contains(err.Error(), "plan_only") {
		t.Fatalf("error = %v, want plan_only failure", err)
	}
}

func TestValidateInjectionPolicyFailsWhenSafetyFlagFalse(t *testing.T) {
	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Safety.ForbidActiveSearch = false
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err == nil || !strings.Contains(err.Error(), "forbid_active_search") {
		t.Fatalf("error = %v, want safety failure", err)
	}
}

func TestValidateInjectionPolicyFailsOnBlockedPaths(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*retrievalcontext.InjectionPolicyConfig)
	}{
		{
			name: "absolute approval path",
			mut: func(cfg *retrievalcontext.InjectionPolicyConfig) {
				cfg.RetrievalInjectionPolicy.Approval.ApprovalPath = "/tmp/approval.json"
			},
		},
		{
			name: "parent traversal bundle path",
			mut: func(cfg *retrievalcontext.InjectionPolicyConfig) {
				cfg.RetrievalInjectionPolicy.Bundle.Path = "../bundle.json"
			},
		},
		{
			name: "secrets materialized path",
			mut: func(cfg *retrievalcontext.InjectionPolicyConfig) {
				cfg.RetrievalInjectionPolicy.Materialized.Path = "secrets/materialized.json"
			},
		},
		{
			name: "dotenv request path",
			mut: func(cfg *retrievalcontext.InjectionPolicyConfig) {
				cfg.RetrievalInjectionPolicy.Approval.RequestPath = ".env/request.json"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validInjectionPolicyConfig()
			tc.mut(&cfg)
			if err := retrievalcontext.ValidateInjectionPolicy(cfg); err == nil {
				t.Fatal("expected blocked path error")
			}
		})
	}
}

func TestValidateInjectionPolicyFailsOnInvalidLimits(t *testing.T) {
	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Limits.MaxTotalChars = 10
	cfg.RetrievalInjectionPolicy.Limits.MaxCharsPerChunk = 100
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err == nil || !strings.Contains(err.Error(), "max_total_chars") {
		t.Fatalf("error = %v, want limits failure", err)
	}
}

func TestValidateInjectionPolicyFailsWhenTextExcerptInYAML(t *testing.T) {
	data := []byte("retrieval_injection_policy:\n  mode: plan_only\n  text_excerpt: leaked\n")
	_, err := retrievalcontext.ParseInjectionPolicy(data)
	if err == nil || !strings.Contains(err.Error(), "text_excerpt") {
		t.Fatalf("error = %v, want text_excerpt failure", err)
	}
}

func TestInjectionPolicyPlanWouldInjectFalse(t *testing.T) {
	cfg := validInjectionPolicyConfig()
	result, err := retrievalcontext.InjectionPolicyPlan(cfg)
	if err != nil {
		t.Fatalf("InjectionPolicyPlan() error = %v", err)
	}
	if result.WouldInject || result.Reason != retrievalcontext.InjectionPolicyReasonSchemaOnly {
		t.Fatalf("result = %#v, want schema-only non-injection plan", result)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok without artifact probes", result.Status)
	}
}

func TestInjectionPolicyPlanWarnsWhenApprovalRunnerInjectionNotAllowed(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Approval.ApprovalPath = "approval.json"
	cfg.RetrievalInjectionPolicy.Approval.RequestPath = "approval-request.json"
	cfg.RetrievalInjectionPolicy.Bundle.Path = "bundle.json"
	cfg.RetrievalInjectionPolicy.Materialized.Path = "materialized.json"

	result, err := retrievalcontext.InjectionPolicyPlan(cfg)
	if err != nil {
		t.Fatalf("InjectionPolicyPlan() error = %v", err)
	}
	if result.WouldInject {
		t.Fatal("would_inject must be false")
	}
	if result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	if !strings.Contains(strings.Join(result.Warnings, "; "), "runner_injection_allowed is false") {
		t.Fatalf("warnings = %#v", result.Warnings)
	}
}

func TestInjectionPolicyPlanOutputHasNoTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	setupGovernanceArtifacts(t)

	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Approval.ApprovalPath = "approval.json"
	cfg.RetrievalInjectionPolicy.Approval.RequestPath = "approval-request.json"
	cfg.RetrievalInjectionPolicy.Bundle.Path = "bundle.json"
	cfg.RetrievalInjectionPolicy.Materialized.Path = "materialized.json"

	result, err := retrievalcontext.InjectionPolicyPlan(cfg)
	if err != nil {
		t.Fatalf("InjectionPolicyPlan() error = %v", err)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteInjectionPolicyPlanText(result, &textBuf); err != nil {
		t.Fatalf("WriteInjectionPolicyPlanText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("plan text leaked materialized content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteInjectionPolicyPlanJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteInjectionPolicyPlanJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") {
		t.Fatal("plan json contains text_excerpt")
	}
}

func validInjectionPolicyConfig() retrievalcontext.InjectionPolicyConfig {
	return retrievalcontext.InjectionPolicyConfig{
		RetrievalInjectionPolicy: retrievalcontext.InjectionPolicy{
			Mode: retrievalcontext.InjectionPolicyModePlanOnly,
			Approval: retrievalcontext.InjectionPolicyApproval{
				Required:                      true,
				RequireRunnerInjectionAllowed: true,
				ApprovalPath:                  "artifacts/run/retrieval-context-approval.json",
				RequestPath:                   "artifacts/run/retrieval-context-approval-request.json",
			},
			Bundle: retrievalcontext.InjectionPolicyBundle{
				Path: "artifacts/run/retrieval-context-bundle.json",
			},
			Materialized: retrievalcontext.InjectionPolicyMaterialized{
				Path: "artifacts/run/retrieval-context-materialized.json",
			},
			Limits: retrievalcontext.InjectionPolicyLimits{
				MaxTotalChars:    6000,
				MaxCharsPerChunk: 1200,
				MaxChunks:        8,
			},
			Prompt: retrievalcontext.InjectionPolicyPrompt{
				SectionTitle:      "Governed materialized retrieval context",
				IncludeMetadata:   true,
				IncludeSourcePath: true,
				IncludeHashes:     true,
			},
			Safety: retrievalcontext.InjectionPolicySafety{
				RequireGovernanceReportOK: true,
				ForbidActiveSearch:        true,
				ForbidProviderCalls:       true,
				ForbidSourceFileReads:     true,
				ForbidMemoryApply:         true,
			},
		},
	}
}

func TestValidateInjectionPolicyFailsWhenApprovalNotRequired(t *testing.T) {
	cfg := validInjectionPolicyConfig()
	cfg.RetrievalInjectionPolicy.Approval.Required = false
	if err := retrievalcontext.ValidateInjectionPolicy(cfg); err == nil || !strings.Contains(err.Error(), "approval.required") {
		t.Fatalf("error = %v, want approval.required failure", err)
	}
}

func TestInjectionPolicyPlanFromExampleYAMLFile(t *testing.T) {
	root := repoRoot(t)
	cfg, err := retrievalcontext.LoadInjectionPolicy(filepath.Join(root, "configs/examples/retrieval-injection-policy.yaml"))
	if err != nil {
		t.Fatalf("LoadInjectionPolicy() error = %v", err)
	}
	result, err := retrievalcontext.InjectionPolicyPlan(cfg)
	if err != nil {
		t.Fatalf("InjectionPolicyPlan() error = %v", err)
	}
	if result.WouldInject {
		t.Fatal("would_inject must be false")
	}
}

func TestLoadInjectionPolicyRejectsMissingFile(t *testing.T) {
	_, err := retrievalcontext.LoadInjectionPolicy(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("expected missing file error")
	}
	_ = os.ErrNotExist
}
