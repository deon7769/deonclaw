package retrievalcontext_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validProviderCallExecutorConfigYAML = `provider_call_executor:
  enabled: false
  provider: codex
  require_confirm_flag: true
  require_execution_bundle: true
  require_chain_audit: true
  require_approval: true
  require_dispatch_config: true
  allow_provider_call: false
  allow_network: false
  allow_worker_execution: false
  allow_prompt_injection: false
  blocked_reason: implementation_not_enabled
  max_total_chars: 6000
  max_payload_bytes: 120000
`

func writeProviderCallExecutorConfig(t *testing.T, content string) string {
	t.Helper()
	const path = "provider-call-executor.yaml"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func TestProviderCallExecutorConfigValidateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	path := writeProviderCallExecutorConfig(t, validProviderCallExecutorConfigYAML)

	cfg, err := retrievalcontext.LoadProviderCallExecutorConfig(path)
	if err != nil {
		t.Fatalf("LoadProviderCallExecutorConfig() error = %v", err)
	}
	if err := retrievalcontext.ValidateProviderCallExecutorConfig(cfg); err != nil {
		t.Fatalf("ValidateProviderCallExecutorConfig() error = %v", err)
	}

	result, err := retrievalcontext.ProviderCallExecutorConfigValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderCallExecutorConfigValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
	if result.Enabled || result.AllowProviderCall || result.AllowNetwork || result.AllowWorkerExecution || result.AllowPromptInjection {
		t.Fatalf("result = %#v, want disabled allow flags", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q, want %q", result.BlockedReason, retrievalcontext.ProviderCallExecutorBlockedReason)
	}

	var textBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderCallExecutorConfigValidateText(result, &textBuf); err != nil {
		t.Fatalf("WriteProviderCallExecutorConfigValidateText() error = %v", err)
	}
	if strings.Contains(textBuf.String(), "text_excerpt") || strings.Contains(textBuf.String(), "alpha text") {
		t.Fatal("executor config validate text leaked preview content")
	}
}

func TestProviderCallExecutorConfigValidateRejectsEnabled(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validProviderCallExecutorConfigYAML, "enabled: false", "enabled: true", 1)
	path := writeProviderCallExecutorConfig(t, content)

	cfg, err := retrievalcontext.LoadProviderCallExecutorConfig(path)
	if err != nil {
		t.Fatalf("LoadProviderCallExecutorConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderCallExecutorConfigValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderCallExecutorConfigValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestProviderCallExecutorConfigValidateRejectsAllowProviderCall(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := strings.Replace(validProviderCallExecutorConfigYAML, "allow_provider_call: false", "allow_provider_call: true", 1)
	path := writeProviderCallExecutorConfig(t, content)

	cfg, err := retrievalcontext.LoadProviderCallExecutorConfig(path)
	if err != nil {
		t.Fatalf("LoadProviderCallExecutorConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderCallExecutorConfigValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderCallExecutorConfigValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}
