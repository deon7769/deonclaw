package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validProviderActivationPolicyYAML = `provider_activation_policy:
  enabled: false
  provider: codex
  require_activation_readiness_audit: true
  require_real_call_proposal: true
  require_credential_policy: true
  require_execution_simulation_report: true
  require_release_gate: true
  require_manual_operator_approval: true
  allow_provider_call: false
  allow_network: false
  allow_secret_read: false
  allow_transport: false
  allow_workspace_write: false
  allow_diff_apply: false
  allow_worker_execution: false
  allow_prompt_injection: false
  blocked_reason: implementation_not_enabled
  max_total_chars: 6000
  max_payload_bytes: 120000
`

func TestProviderActivationPolicyValidateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("provider-activation-policy.yaml", []byte(validProviderActivationPolicyYAML), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderActivationPolicyConfig("provider-activation-policy.yaml")
	if err != nil {
		t.Fatalf("LoadProviderActivationPolicyConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderActivationPolicyValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderActivationPolicyValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, failures=%#v", result.Status, result.Failures)
	}
	assertProviderActivationPolicyValidateBlocked(t, result)
}

func TestProviderActivationPolicyRejectEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := bytes.Replace([]byte(validProviderActivationPolicyYAML), []byte("enabled: false"), []byte("enabled: true"), 1)
	if err := os.WriteFile("bad-activation-policy.yaml", content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderActivationPolicyConfig("bad-activation-policy.yaml")
	if err != nil {
		t.Fatalf("LoadProviderActivationPolicyConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderActivationPolicyValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderActivationPolicyValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestProviderActivationPolicyRejectAllowProviderCallTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := bytes.Replace([]byte(validProviderActivationPolicyYAML), []byte("allow_provider_call: false"), []byte("allow_provider_call: true"), 1)
	if err := os.WriteFile("bad-activation-policy.yaml", content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderActivationPolicyConfig("bad-activation-policy.yaml")
	if err != nil {
		t.Fatalf("LoadProviderActivationPolicyConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderActivationPolicyValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderActivationPolicyValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func assertProviderActivationPolicyValidateBlocked(t *testing.T, result retrievalcontext.ProviderActivationPolicyValidateResult) {
	t.Helper()
	if !result.ActivationPolicyValidated {
		t.Fatalf("activation_policy_validated = false, failures=%#v", result.Failures)
	}
	if result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("activation policy validate flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
}

func assertProviderActivationPolicyPlanBlocked(t *testing.T, result retrievalcontext.ProviderActivationPolicyPlanResult) {
	t.Helper()
	if !result.ActivationPolicyPlanReady || !result.ActivationPolicyValidated {
		t.Fatalf("plan ready=%t validated=%t", result.ActivationPolicyPlanReady, result.ActivationPolicyValidated)
	}
	if result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("activation policy plan flags must stay blocked: %#v", result)
	}
}
