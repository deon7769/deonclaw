package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validExecutionEnableConfigYAML = `materialized_injection_execution_enable:
  enabled: false
  require_confirm_flag: true
  require_readiness_report: true
  require_execution_gate: true
  require_assembly_report: true
  allow_provider_call: false
  allow_worker_execution: false
  allow_prompt_injection: false
  max_total_chars: 6000
  blocked_reason: implementation_not_enabled
`

func writeExecutionEnableConfig(t *testing.T, content string) string {
	t.Helper()
	const path = "materialized-injection-execution-enable.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestMaterializedInjectionExecutionEnableValidateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeExecutionEnableConfig(t, validExecutionEnableConfigYAML)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	if err := retrievalcontext.ValidateMaterializedInjectionExecutionEnable(cfg); err != nil {
		t.Fatalf("ValidateMaterializedInjectionExecutionEnable() error = %v", err)
	}

	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Enabled || result.AllowProviderCall || result.AllowWorkerExecution || result.AllowPromptInjection {
		t.Fatalf("result = %#v, want disabled enable flags", result)
	}
	if result.MaxTotalChars <= 0 {
		t.Fatalf("result = %#v, want max_total_chars > 0", result)
	}
	if result.BlockedReason != retrievalcontext.MaterializedInjectionExecutionEnableBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.MaterializedInjectionExecutionEnableBlockedReason)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionExecutionEnableValidateText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionExecutionEnableValidateText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("execution enable validate text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedInjectionExecutionEnableValidateJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedInjectionExecutionEnableValidateJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("execution enable validate json leaked preview content")
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "enabled: false", "enabled: true", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "enabled must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenAllowProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "allow_provider_call: false", "allow_provider_call: true", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenAllowWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "allow_worker_execution: false", "allow_worker_execution: true", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenAllowPromptInjectionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "allow_prompt_injection: false", "allow_prompt_injection: true", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "allow_prompt_injection must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenRequireConfirmFlagFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "require_confirm_flag: true", "require_confirm_flag: false", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "require_confirm_flag must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenBlockedReasonDifferent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "blocked_reason: implementation_not_enabled", "blocked_reason: something_else", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "blocked_reason") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedInjectionExecutionEnableValidateRejectsConfigWithTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := validExecutionEnableConfigYAML + "\n# text_excerpt placeholder\n"
	path := writeExecutionEnableConfig(t, content)

	_, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err == nil || !strings.Contains(err.Error(), "materialized preview text") {
		t.Fatalf("error = %v, want materialized preview text rejection", err)
	}
}

func TestMaterializedInjectionExecutionEnableValidateFailsWhenMaxTotalCharsZero(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validExecutionEnableConfigYAML, "max_total_chars: 6000", "max_total_chars: 0", 1)
	path := writeExecutionEnableConfig(t, content)

	cfg, err := retrievalcontext.LoadMaterializedInjectionExecutionEnable(path)
	if err != nil {
		t.Fatalf("LoadMaterializedInjectionExecutionEnable() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedInjectionExecutionEnableValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedInjectionExecutionEnableValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if !strings.Contains(strings.Join(result.Failures, "; "), "max_total_chars must be > 0") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}
