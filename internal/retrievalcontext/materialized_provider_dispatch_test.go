package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validDispatchConfigYAML = `materialized_provider_dispatch:
  enabled: false
  provider: codex
  require_confirm_flag: true
  require_provider_run_plan: true
  require_assembled_prompt: true
  allow_provider_call: false
  allow_network: false
  allow_worker_execution: false
  allow_prompt_injection: false
  max_total_chars: 6000
  max_payload_bytes: 120000
  blocked_reason: implementation_not_enabled
`

func writeDispatchConfig(t *testing.T, content string) string {
	t.Helper()
	const path = "materialized-provider-dispatch.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestMaterializedProviderDispatchValidateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, validDispatchConfigYAML)

	cfg, err := retrievalcontext.LoadMaterializedProviderDispatch(path)
	if err != nil {
		t.Fatalf("LoadMaterializedProviderDispatch() error = %v", err)
	}
	result, err := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if err != nil {
		t.Fatalf("MaterializedProviderDispatchValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Enabled || result.AllowProviderCall || result.AllowNetwork || result.AllowWorkerExecution || result.AllowPromptInjection {
		t.Fatalf("result = %#v, want disabled dispatch flags", result)
	}
	if result.Provider != retrievalcontext.MaterializedProviderDispatchProvider {
		t.Fatalf("provider = %q, want codex", result.Provider)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderDispatchValidateText(result, &textBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderDispatchValidateText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("dispatch validate text leaked preview content")
	}

	var jsonBuf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedProviderDispatchValidateJSON(result, &jsonBuf); err != nil {
		t.Fatalf("WriteMaterializedProviderDispatchValidateJSON() error = %v", err)
	}
	if strings.Contains(jsonBuf.String(), "text_excerpt") || strings.Contains(jsonBuf.String(), "alpha text") {
		t.Fatal("dispatch validate json leaked preview content")
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "enabled: false", "enabled: true", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "enabled must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenProviderNotCodex(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "provider: codex", "provider: opencode", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "provider") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenAllowProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "allow_provider_call: false", "allow_provider_call: true", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "allow_provider_call must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenAllowNetworkTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "allow_network: false", "allow_network: true", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "allow_network must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenAllowWorkerExecutionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "allow_worker_execution: false", "allow_worker_execution: true", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "allow_worker_execution must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenAllowPromptInjectionTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "allow_prompt_injection: false", "allow_prompt_injection: true", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "allow_prompt_injection must be false") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenRequireConfirmFlagFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "require_confirm_flag: true", "require_confirm_flag: false", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "require_confirm_flag must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenRequireProviderRunPlanFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "require_provider_run_plan: true", "require_provider_run_plan: false", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "require_provider_run_plan must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenRequireAssembledPromptFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "require_assembled_prompt: true", "require_assembled_prompt: false", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "require_assembled_prompt must be true") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenBlockedReasonDifferent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "blocked_reason: implementation_not_enabled", "blocked_reason: other", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "blocked_reason") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenMaxTotalCharsZero(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "max_total_chars: 6000", "max_total_chars: 0", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "max_total_chars must be > 0") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateFailsWhenMaxPayloadBytesZero(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeDispatchConfig(t, strings.Replace(validDispatchConfigYAML, "max_payload_bytes: 120000", "max_payload_bytes: 0", 1))
	cfg, _ := retrievalcontext.LoadMaterializedProviderDispatch(path)
	result, _ := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if result.Status != lancedbpolicy.StatusFailed || !strings.Contains(strings.Join(result.Failures, "; "), "max_payload_bytes must be > 0") {
		t.Fatalf("failures = %#v", result.Failures)
	}
}

func TestMaterializedProviderDispatchValidateRejectsConfigWithTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_, err := retrievalcontext.LoadMaterializedProviderDispatch(writeDispatchConfig(t, validDispatchConfigYAML+"\n# text_excerpt\n"))
	if err == nil || !strings.Contains(err.Error(), "materialized preview text") {
		t.Fatalf("error = %v, want materialized preview text rejection", err)
	}
}
