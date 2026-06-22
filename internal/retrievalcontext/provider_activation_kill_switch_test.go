package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

const validProviderActivationKillSwitchYAML = `provider_activation_kill_switch:
  global_disabled: true
  block_provider_call: true
  block_network: true
  block_secret_read: true
  block_transport: true
  block_workspace_write: true
  block_diff_apply: true
  block_worker_execution: true
  block_prompt_injection: true
  blocked_reason: implementation_not_enabled
`

func TestAssertNoPreviewLeakInStringAllowsCleanMetadata(t *testing.T) {
	assertNoPreviewLeakInString(t, "clean", `{"status":"ok","blocked_reason":"implementation_not_enabled"}`)
}

func TestProviderActivationKillSwitchValidateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("provider-activation-kill-switch.yaml", []byte(validProviderActivationKillSwitchYAML), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderActivationKillSwitchConfig("provider-activation-kill-switch.yaml")
	if err != nil {
		t.Fatalf("LoadProviderActivationKillSwitchConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderActivationKillSwitchValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderActivationKillSwitchValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || !result.KillSwitchValidated || !result.GlobalDisabled {
		t.Fatalf("validate = %#v, want ok with kill switch active", result)
	}
	if result.ActivationAllowedNow || !result.ProviderCallBlocked || !result.SecretReadBlocked {
		t.Fatalf("validate flags = %#v, want all blocked", result)
	}
}

func TestProviderActivationKillSwitchRejectsBlockFalse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	content := bytes.Replace([]byte(validProviderActivationKillSwitchYAML), []byte("block_network: true"), []byte("block_network: false"), 1)
	if err := os.WriteFile("bad-kill-switch.yaml", content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	cfg, err := retrievalcontext.LoadProviderActivationKillSwitchConfig("bad-kill-switch.yaml")
	if err != nil {
		t.Fatalf("LoadProviderActivationKillSwitchConfig() error = %v", err)
	}
	result, err := retrievalcontext.ProviderActivationKillSwitchValidate(cfg)
	if err != nil {
		t.Fatalf("ProviderActivationKillSwitchValidate() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestProviderActivationKillSwitchPlanOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	if err := os.WriteFile("provider-activation-kill-switch.yaml", []byte(validProviderActivationKillSwitchYAML), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	plan, err := retrievalcontext.ProviderActivationKillSwitchPlan(retrievalcontext.ProviderActivationKillSwitchPlanOptions{
		ConfigPath: "provider-activation-kill-switch.yaml",
		OutputPath: "provider-activation-kill-switch-plan.json",
	})
	if err != nil {
		t.Fatalf("ProviderActivationKillSwitchPlan() error = %v", err)
	}
	if !plan.KillSwitchPlanReady || !plan.KillSwitchValidated {
		t.Fatalf("plan = %#v, want ready and validated", plan)
	}
	assertNoPreviewLeakInFile(t, "provider-activation-kill-switch-plan.json")
}
