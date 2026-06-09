package workerconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestLoadWorkersConfigCommands(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workers.yaml")
	content := []byte(`workers:
  codex:
    command: codex-test
  opencode:
    command: opencode-test
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.Command("codex"); got != "codex-test" {
		t.Fatalf("codex command = %q, want codex-test", got)
	}
	if got := cfg.Command("opencode"); got != "opencode-test" {
		t.Fatalf("opencode command = %q, want opencode-test", got)
	}
}

func TestDefaultIncludesOpenCodeFallback(t *testing.T) {
	if got := Default().Command("opencode"); got != "opencode" {
		t.Fatalf("opencode fallback = %q, want opencode", got)
	}
}

func TestKnownWorkersExcludeFutureInspirationOnlyWorkers(t *testing.T) {
	if got := Default().Command("kimi"); got != "" {
		t.Fatalf("kimi fallback = %q, want no operational fallback", got)
	}
	for _, worker := range KnownWorkers() {
		if worker == "kimi" {
			t.Fatalf("KnownWorkers() = %#v, want no kimi", KnownWorkers())
		}
	}
}

func TestLoadWorkersConfigProviderModelEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workers.yaml")
	content := []byte(`workers:
  opencode:
    command: opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	worker := cfg.Worker("opencode")
	if worker.Command != "opencode" {
		t.Fatalf("command = %q, want opencode", worker.Command)
	}
	if worker.Provider != "z_ai_glm" {
		t.Fatalf("provider = %q, want z_ai_glm", worker.Provider)
	}
	if worker.Model != "glm-5.1" {
		t.Fatalf("model = %q, want glm-5.1", worker.Model)
	}
	if got := worker.Env["ZAI_API_KEY"]; got != "required" {
		t.Fatalf("ZAI_API_KEY marker = %q, want required", got)
	}
}

func TestLoadWorkersConfigModelProfiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workers.yaml")
	content := []byte(`workers:
  opencode:
    command: opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
      - general
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	profile, err := cfg.ResolveModelProfile("opencode", "opencode-zai-glm-5-1")
	if err != nil {
		t.Fatalf("ResolveModelProfile() error = %v", err)
	}
	if profile.Worker != "opencode" || profile.Provider != "z-ai" || profile.Model != "glm-5.1" || profile.ModelArg != "z-ai/glm-5.1" {
		t.Fatalf("profile = %#v, want opencode z-ai glm-5.1", profile)
	}
	if len(profile.Tags) != 2 || profile.Tags[0] != "coding" || profile.Tags[1] != "general" {
		t.Fatalf("tags = %#v, want coding/general", profile.Tags)
	}
	if got := profile.Env["ZAI_API_KEY"]; got != "required" {
		t.Fatalf("profile ZAI_API_KEY marker = %q, want required", got)
	}
}

func TestResolveModelProfileRejectsMissingAndWorkerMismatch(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"opencode-zai-glm-5-1": {
			Worker: "opencode",
			Model:  "glm-5.1",
		},
	}}

	if _, err := cfg.ResolveModelProfile("opencode", "missing-profile"); err == nil {
		t.Fatal("ResolveModelProfile(missing) error = nil, want error")
	}
	if _, err := cfg.ResolveModelProfile("codex", "opencode-zai-glm-5-1"); err == nil {
		t.Fatal("ResolveModelProfile(worker mismatch) error = nil, want error")
	}
	if _, err := cfg.ResolveModelProfile("opencode", ""); err != nil {
		t.Fatalf("ResolveModelProfile(empty) error = %v, want nil", err)
	}
}

func TestResolveModelStrategyValidatesProfilesAndTags(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"opencode-zai-glm-5-1": {
			Worker: "opencode",
			Model:  "glm-5.1",
			Tags:   []string{"coding", "general"},
		},
		"opencode-fast": {
			Worker: "opencode",
			Model:  "fast",
			Tags:   []string{"coding"},
		},
	}}

	resolved, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred:   []string{"opencode-zai-glm-5-1"},
		Fallback:    []string{"opencode-fast"},
		RequireTags: []string{"coding"},
		FallbackPolicy: &tasks.FallbackPolicy{
			Enabled:      false,
			MaxAttempts:  1,
			RetryOn:      []string{"worker_failed", "validation_failed"},
			NeverRetryOn: []string{"policy_failed", "memory_policy_failed"},
		},
	})
	if err != nil {
		t.Fatalf("ResolveModelStrategy() error = %v", err)
	}
	if len(resolved.Preferred) != 1 || resolved.Preferred[0].Name != "opencode-zai-glm-5-1" {
		t.Fatalf("preferred = %#v, want resolved opencode-zai-glm-5-1", resolved.Preferred)
	}
	if len(resolved.Fallback) != 1 || resolved.Fallback[0].Name != "opencode-fast" {
		t.Fatalf("fallback = %#v, want resolved opencode-fast", resolved.Fallback)
	}
	if len(resolved.RequireTags) != 1 || resolved.RequireTags[0] != "coding" {
		t.Fatalf("require_tags = %#v, want coding", resolved.RequireTags)
	}
	if resolved.FallbackPolicy.Enabled || resolved.FallbackPolicy.MaxAttempts != 1 {
		t.Fatalf("fallback_policy = %#v, want disabled max_attempts 1", resolved.FallbackPolicy)
	}
	planned, ok := resolved.PlannedModelProfile()
	if !ok || planned.Name != "opencode-zai-glm-5-1" {
		t.Fatalf("planned = %#v ok=%t, want first preferred profile", planned, ok)
	}
}

func TestResolveModelStrategyDefaultFallbackPolicyDisabled(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"opencode-zai-glm-5-1": {Worker: "opencode"},
	}}

	resolved, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
	})
	if err != nil {
		t.Fatalf("ResolveModelStrategy() error = %v", err)
	}
	if resolved.FallbackPolicy.Enabled {
		t.Fatalf("fallback_policy.enabled = true, want default disabled")
	}
	if resolved.FallbackPolicy.MaxAttempts != 1 {
		t.Fatalf("fallback_policy.max_attempts = %d, want default 1", resolved.FallbackPolicy.MaxAttempts)
	}
	if len(resolved.FallbackPolicy.RetryOn) != 2 || resolved.FallbackPolicy.RetryOn[0] != "worker_failed" || resolved.FallbackPolicy.RetryOn[1] != "validation_failed" {
		t.Fatalf("fallback_policy.retry_on = %#v, want default worker_failed/validation_failed", resolved.FallbackPolicy.RetryOn)
	}
	if len(resolved.FallbackPolicy.NeverRetryOn) != 2 || resolved.FallbackPolicy.NeverRetryOn[0] != "policy_failed" || resolved.FallbackPolicy.NeverRetryOn[1] != "memory_policy_failed" {
		t.Fatalf("fallback_policy.never_retry_on = %#v, want default policy blocks", resolved.FallbackPolicy.NeverRetryOn)
	}
}

func TestResolveModelStrategyRejectsInvalidFallbackPolicy(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"opencode-zai-glm-5-1": {Worker: "opencode"},
	}}

	_, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		FallbackPolicy: &tasks.FallbackPolicy{
			Enabled:     true,
			MaxAttempts: 0,
		},
	})
	if err == nil {
		t.Fatal("ResolveModelStrategy() error = nil, want max_attempts error")
	}
	if !strings.Contains(err.Error(), "model_strategy.fallback_policy.max_attempts must be greater than zero") {
		t.Fatalf("error = %v, want max_attempts error", err)
	}

	_, err = cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		FallbackPolicy: &tasks.FallbackPolicy{
			RetryOn: []string{"policy_failed"},
		},
	})
	if err == nil {
		t.Fatal("ResolveModelStrategy() error = nil, want policy_failed retry_on error")
	}
	if !strings.Contains(err.Error(), `model_strategy.fallback_policy.retry_on[0] "policy_failed" is not supported`) {
		t.Fatalf("error = %v, want policy_failed retry_on error", err)
	}
}

func TestResolveModelStrategyRejectsMissingProfile(t *testing.T) {
	cfg := Config{}

	_, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred: []string{"missing-profile"},
	})
	if err == nil {
		t.Fatal("ResolveModelStrategy() error = nil, want missing profile error")
	}
	if !strings.Contains(err.Error(), `model_strategy.preferred[0] profile "missing-profile" not found`) {
		t.Fatalf("error = %v, want missing profile error", err)
	}
}

func TestResolveModelStrategyRejectsMissingRequiredTag(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"opencode-zai-glm-5-1": {
			Worker: "opencode",
			Tags:   []string{"general"},
		},
	}}

	_, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred:   []string{"opencode-zai-glm-5-1"},
		RequireTags: []string{"coding"},
	})
	if err == nil {
		t.Fatal("ResolveModelStrategy() error = nil, want missing tag error")
	}
	if !strings.Contains(err.Error(), `model_strategy.preferred[0] profile "opencode-zai-glm-5-1" missing required tag "coding"`) {
		t.Fatalf("error = %v, want missing tag error", err)
	}
}

func TestResolveModelStrategyRejectsWorkerMismatch(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"codex-default": {
			Worker: "codex",
			Tags:   []string{"coding"},
		},
	}}

	_, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred:   []string{"codex-default"},
		RequireTags: []string{"coding"},
	})
	if err == nil {
		t.Fatal("ResolveModelStrategy() error = nil, want worker mismatch error")
	}
	if !strings.Contains(err.Error(), `model_strategy.preferred[0] profile "codex-default" worker "codex" does not match task worker "opencode"; automatic worker switching is not implemented`) {
		t.Fatalf("error = %v, want worker mismatch error", err)
	}
}

func TestResolveModelStrategyRejectsFallbackWorkerMismatch(t *testing.T) {
	cfg := Config{ModelProfiles: map[string]ModelProfile{
		"opencode-zai-glm-5-1": {Worker: "opencode"},
		"codex-default":        {Worker: "codex"},
	}}

	_, err := cfg.ResolveModelStrategy("opencode", &tasks.ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		Fallback:  []string{"codex-default"},
	})
	if err == nil {
		t.Fatal("ResolveModelStrategy() error = nil, want fallback worker mismatch error")
	}
	if !strings.Contains(err.Error(), `model_strategy.fallback[0] profile "codex-default" worker "codex" does not match task worker "opencode"; automatic worker switching is not implemented`) {
		t.Fatalf("error = %v, want fallback worker mismatch error", err)
	}
}

func TestFallbackEnvRequirementChecksForReportsMissingFallbackEnv(t *testing.T) {
	unsetEnvForTest(t, "FALLBACK_API_KEY")
	unsetEnvForTest(t, "OPENCODE_API_KEY")
	cfg := Config{
		Workers: map[string]Worker{
			"opencode": {Env: map[string]string{"OPENCODE_API_KEY": "required"}},
		},
		ModelProfiles: map[string]ModelProfile{
			"opencode-zai-glm-5-1": {
				Worker: "opencode",
			},
			"opencode-fallback": {
				Worker: "opencode",
				Env:    map[string]string{"FALLBACK_API_KEY": "required"},
			},
		},
	}

	checks, err := cfg.FallbackEnvRequirementChecksFor("opencode", &tasks.ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		Fallback:  []string{"opencode-fallback"},
		FallbackPolicy: &tasks.FallbackPolicy{
			Enabled:     false,
			MaxAttempts: 1,
		},
	})
	if err != nil {
		t.Fatalf("FallbackEnvRequirementChecksFor() error = %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("checks = %#v, want worker and fallback env requirements", checks)
	}
	if checks[0].Name != "FALLBACK_API_KEY" || checks[0].State != EnvStateMissing || checks[1].Name != "OPENCODE_API_KEY" || checks[1].State != EnvStateMissing {
		t.Fatalf("checks = %#v, want sorted missing worker and fallback env requirements", checks)
	}
}

func TestEnvRequirementChecksForMergesWorkerAndModelProfile(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	unsetEnvForTest(t, "OPENAI_API_KEY")
	cfg := Config{
		Workers: map[string]Worker{
			"opencode": {Env: map[string]string{"OPENAI_API_KEY": "required"}},
		},
		ModelProfiles: map[string]ModelProfile{
			"opencode-zai-glm-5-1": {
				Worker: "opencode",
				Env:    map[string]string{"ZAI_API_KEY": "required"},
			},
		},
	}

	checks, err := cfg.EnvRequirementChecksFor("opencode", "opencode-zai-glm-5-1")
	if err != nil {
		t.Fatalf("EnvRequirementChecksFor() error = %v", err)
	}
	if len(checks) != 2 {
		t.Fatalf("checks = %#v, want two merged checks", checks)
	}
	if checks[0].Name != "OPENAI_API_KEY" || checks[0].State != EnvStateMissing || checks[1].Name != "ZAI_API_KEY" || checks[1].State != EnvStateMissing {
		t.Fatalf("checks = %#v, want sorted missing checks", checks)
	}

	t.Setenv("ZAI_API_KEY", "dummy")
	checks, err = cfg.EnvRequirementChecksFor("opencode", "opencode-zai-glm-5-1")
	if err != nil {
		t.Fatalf("EnvRequirementChecksFor() error = %v", err)
	}
	if checks[1].Name != "ZAI_API_KEY" || checks[1].State != EnvStateSetMasked {
		t.Fatalf("checks = %#v, want profile env set_masked", checks)
	}
}

func TestEnvRequirementChecksForTaskWithoutProfileKeepsWorkerBehavior(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	cfg := Config{Workers: map[string]Worker{
		"opencode": {Env: map[string]string{"ZAI_API_KEY": "required"}},
	}}

	checks, err := cfg.EnvRequirementChecksFor("opencode", "")
	if err != nil {
		t.Fatalf("EnvRequirementChecksFor() error = %v", err)
	}
	if len(checks) != 1 || checks[0].Name != "ZAI_API_KEY" || checks[0].State != EnvStateMissing {
		t.Fatalf("checks = %#v, want worker env requirement", checks)
	}
}

func TestEnvRequirementChecksReportMissingAndSetMasked(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	cfg := Config{Workers: map[string]Worker{
		"opencode": {Env: map[string]string{"ZAI_API_KEY": "required"}},
	}}

	checks := cfg.EnvRequirementChecks("opencode")
	if len(checks) != 1 {
		t.Fatalf("checks = %#v, want one check", checks)
	}
	if checks[0].Name != "ZAI_API_KEY" || checks[0].Requirement != EnvRequirementRequired || checks[0].State != EnvStateMissing {
		t.Fatalf("check = %#v, want missing required ZAI_API_KEY", checks[0])
	}
	if err := cfg.ValidateRequiredEnv("opencode"); err == nil {
		t.Fatal("ValidateRequiredEnv() error = nil, want missing env error")
	}

	t.Setenv("ZAI_API_KEY", "")
	checks = cfg.EnvRequirementChecks("opencode")
	if len(checks) != 1 || checks[0].State != EnvStateSetMasked {
		t.Fatalf("checks = %#v, want set_masked", checks)
	}
	if err := cfg.ValidateRequiredEnv("opencode"); err != nil {
		t.Fatalf("ValidateRequiredEnv() error = %v, want nil", err)
	}
}

func unsetEnvForTest(t *testing.T, name string) {
	t.Helper()
	oldValue, hadOldValue := os.LookupEnv(name)
	if err := os.Unsetenv(name); err != nil {
		t.Fatalf("Unsetenv(%s) error = %v", name, err)
	}
	t.Cleanup(func() {
		if hadOldValue {
			_ = os.Setenv(name, oldValue)
		} else {
			_ = os.Unsetenv(name)
		}
	})
}
