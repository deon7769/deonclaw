package retrievalcontext_test

import (
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestValidateTaskMaterializedInjectionOKWithValidBundle(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	bundlePath := writeValidGovernanceBundleForTask(t)

	result, err := retrievalcontext.ValidateTaskMaterializedInjection(&tasks.MaterializedInjectionSpec{
		Enabled:            false,
		GovernanceBundle:   bundlePath,
		PromptPreview:      "retrieval-context-prompt-preview.md",
		RequireConfirmFlag: true,
		MaxTotalChars:      6000,
	})
	if err != nil {
		t.Fatalf("ValidateTaskMaterializedInjection() error = %v", err)
	}
	if !result.Declared || result.Enabled || result.SupportedNow {
		t.Fatalf("result = %#v, want declared disabled unsupported", result)
	}
	if result.GovernanceBundleSHA256 == "" {
		t.Fatal("governance_bundle_sha256 is required")
	}
}

func TestValidateTaskMaterializedInjectionFailsWhenEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	bundlePath := writeValidGovernanceBundleForTask(t)

	_, err := retrievalcontext.ValidateTaskMaterializedInjection(&tasks.MaterializedInjectionSpec{
		Enabled:            true,
		GovernanceBundle:   bundlePath,
		PromptPreview:      "retrieval-context-prompt-preview.md",
		RequireConfirmFlag: true,
		MaxTotalChars:      6000,
	})
	if err == nil || !strings.Contains(err.Error(), tasks.MaterializedInjectionNotSupportedYet) {
		t.Fatalf("error = %v, want not supported yet", err)
	}
}

func TestValidateTaskMaterializedInjectionFailsOnInvalidBundle(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("bad-bundle.json", []byte(`{"status":"failed","contains_text":true,"runner_execution":true,"injection_authorized_for_future":false,"execution_supported_now":true}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := retrievalcontext.ValidateTaskMaterializedInjection(&tasks.MaterializedInjectionSpec{
		Enabled:            false,
		GovernanceBundle:   "bad-bundle.json",
		PromptPreview:      "retrieval-context-prompt-preview.md",
		RequireConfirmFlag: true,
		MaxTotalChars:      6000,
	})
	if err == nil || !strings.Contains(err.Error(), "contains_text must be false") {
		t.Fatalf("error = %v, want invalid bundle failure", err)
	}
}

func writeValidGovernanceBundleForTask(t *testing.T) string {
	t.Helper()
	setupInjectionGovernanceBundleArtifacts(t)
	if _, err := retrievalcontext.InjectionGovernanceBundle(injectionGovernanceBundleOpts()); err != nil {
		t.Fatalf("InjectionGovernanceBundle() error = %v", err)
	}
	return "injection-governance-bundle.json"
}
