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
  kimi:
    command: kimi-test
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
	if got := cfg.Command("kimi"); got != "kimi-test" {
		t.Fatalf("kimi command = %q, want kimi-test", got)
	}
}

func TestDefaultIncludesOpenCodeFallback(t *testing.T) {
	if got := Default().Command("opencode"); got != "opencode" {
		t.Fatalf("opencode fallback = %q, want opencode", got)
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
