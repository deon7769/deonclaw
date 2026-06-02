package workerconfig

import (
	"os"
	"path/filepath"
	"testing"
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
