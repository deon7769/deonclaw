package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validRuntimeConfigYAML = `materialized_injection_runtime:
  enabled: false
  require_confirm_flag: true
  require_execution_gate: true
  require_assembly_report: true
  allow_worker_execution: false
  allow_prompt_injection: false
  max_total_chars: 6000
  blocked_reason: implementation_not_enabled
`

func writeRuntimeConfig(t *testing.T, content string) string {
	t.Helper()
	const path = "materialized-injection-runtime.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestMaterializedInjectionRuntimeValidateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeRuntimeConfig(t, validRuntimeConfigYAML)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	if err := retrievalcontext.ValidateMaterializedInjectionRuntime(cfg); err != nil {
		t.Fatalf("ValidateMaterializedInjectionRuntime() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Enabled || result.AllowWorkerExecution || result.AllowPromptInjection {
		t.Fatalf("result = %#v, want disabled runtime flags", result)
	}
	if !result.RequireConfirmFlag || !result.RequireExecutionGate || !result.RequireAssemblyReport {
		t.Fatalf("result = %#v, want require flags true", result)
	}
	if result.MaxTotalChars <= 0 {
		t.Fatalf("result = %#v, want max_total_chars > 0", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedInjectionRuntimeBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedInjectionRuntimeBlockedReason)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionRuntimeValidateText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionRuntimeValidateText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("runtime validate text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionRuntimeValidateJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionRuntimeValidateJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("runtime validate json leaked preview content")
	}
}

func TestMaterializedInjectionRuntimeValidateFailsWhenEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validRuntimeConfigYAML, "enabled: false", "enabled: true", 1)
	path := writeRuntimeConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "enabled must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRuntimeValidateFailsWhenAllowWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validRuntimeConfigYAML, "allow_worker_execution: false", "allow_worker_execution: true", 1)
	path := writeRuntimeConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRuntimeValidateFailsWhenAllowPromptInjectionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validRuntimeConfigYAML, "allow_prompt_injection: false", "allow_prompt_injection: true", 1)
	path := writeRuntimeConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_prompt_injection must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRuntimeValidateFailsWhenRequireConfirmFlagFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validRuntimeConfigYAML, "require_confirm_flag: true", "require_confirm_flag: false", 1)
	path := writeRuntimeConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "require_confirm_flag must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRuntimeValidateFailsWhenBlockedReasonDifferent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validRuntimeConfigYAML, "blocked_reason: implementation_not_enabled", "blocked_reason: something_else", 1)
	path := writeRuntimeConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "blocked_reason") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionRuntimeValidateRejectsConfigWithTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := validRuntimeConfigYAML + "\n# text_excerpt placeholder\n"
	path := writeRuntimeConfig(t, content)

	_, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err == nil || !strings.Contains(err.Error(), "materialized preview text") {
		t.Fatalf("error = %v, want materialized preview text rejection", err)
	}
}

func TestMaterializedInjectionRuntimeValidateFailsWhenMaxTotalCharsZero(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validRuntimeConfigYAML, "max_total_chars: 6000", "max_total_chars: 0", 1)
	path := writeRuntimeConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionRuntime(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionRuntime() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionRuntimeValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionRuntimeValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "max_total_chars must be > 0") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}
