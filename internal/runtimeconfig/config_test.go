package runtimeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAndValidateValidRuntimeConfig(t *testing.T) {
	path := writeRuntimeConfig(t, `runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    memory_limit: 2g
    cpus: "2"
    mounts:
      - source: .
        target: /workspace
        mode: rw
      - source: mysecondbrain
        target: /memory/mysecondbrain
        mode: ro
      - source: escalasoft_brain
        target: /memory/escalasoft_brain
        mode: ro
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	result, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", result.Warnings)
	}
}

func TestValidateRejectsInvalidMode(t *testing.T) {
	cfg := validDockerConfig()
	cfg.Runtime.Mode = "podman"

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid mode")
	}
	if !strings.Contains(err.Error(), `runtime.mode "podman" is not supported`) {
		t.Fatalf("error = %v, want invalid mode", err)
	}
}

func TestValidateRejectsDangerousMountSources(t *testing.T) {
	cases := []string{
		"/",
		"/home",
		"/home/davi",
		"/root",
		"/root/.config",
		"~",
		"~/workspace",
		"~/.ssh",
		".env",
		"config/.env",
		"secrets",
		"config/secrets",
		"/var/run/docker.sock",
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			cfg := validDockerConfig()
			cfg.Runtime.Docker.Mounts = []MountSpec{{Source: source, Target: "/workspace", Mode: "ro"}}

			_, err := Validate(cfg)
			if err == nil {
				t.Fatalf("Validate() error = nil, want rejection for %q", source)
			}
			if !strings.Contains(err.Error(), "is not allowed") {
				t.Fatalf("error = %v, want not allowed", err)
			}
		})
	}
}

func TestValidateRejectsDangerousMountTargets(t *testing.T) {
	cases := []string{
		"workspace",
		"/",
		"/root",
		"/root/.config",
		"/etc",
		"/etc/deonclaw",
		"/var/run/docker.sock",
	}
	for _, target := range cases {
		t.Run(target, func(t *testing.T) {
			cfg := validDockerConfig()
			cfg.Runtime.Docker.Mounts = []MountSpec{{Source: ".", Target: target, Mode: "ro"}}

			_, err := Validate(cfg)
			if err == nil {
				t.Fatalf("Validate() error = nil, want rejection for target %q", target)
			}
			if !strings.Contains(err.Error(), "target") || !strings.Contains(err.Error(), "is not allowed") {
				t.Fatalf("error = %v, want target not allowed", err)
			}
		})
	}
}

func TestValidateMountModes(t *testing.T) {
	for _, mode := range []string{"ro", "rw"} {
		t.Run(mode, func(t *testing.T) {
			cfg := validDockerConfig()
			cfg.Runtime.Docker.Mounts = []MountSpec{{Source: ".", Target: "/workspace", Mode: mode}}

			if _, err := Validate(cfg); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}

	cfg := validDockerConfig()
	cfg.Runtime.Docker.Mounts = []MountSpec{{Source: ".", Target: "/workspace", Mode: "write"}}
	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid mount mode")
	}
	if !strings.Contains(err.Error(), `runtime.docker.mounts[0].mode "write" is not supported`) {
		t.Fatalf("error = %v, want invalid mount mode", err)
	}
}

func TestValidateNetworkDefaultWarns(t *testing.T) {
	cfg := validDockerConfig()
	cfg.Runtime.Docker.Network = "default"

	result, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "network access") {
		t.Fatalf("warnings = %#v, want network access warning", result.Warnings)
	}
}

func TestValidateEnvPassthrough(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "secret")
	cfg := validDockerConfig()
	cfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY", "OPENAI_API_KEY"}

	result, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "OPENAI_API_KEY") {
		t.Fatalf("warnings = %#v, want missing OPENAI_API_KEY warning", result.Warnings)
	}
	if strings.Contains(strings.Join(result.Warnings, "\n"), "secret") {
		t.Fatalf("warnings leaked secret: %#v", result.Warnings)
	}
}

func TestValidateRejectsInvalidEnvPassthroughNames(t *testing.T) {
	cases := []string{
		"",
		"OPENAI API",
		"OPENAI_API_KEY=value",
		"OPENAI-API",
		"OPENAI/API",
		"OPENAI.API",
		"openai_api_key",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validDockerConfig()
			cfg.Runtime.Docker.Env.Passthrough = []string{name}

			_, err := Validate(cfg)
			if err == nil {
				t.Fatalf("Validate() error = nil, want invalid env name for %q", name)
			}
			if !strings.Contains(err.Error(), "runtime.docker.env.passthrough[0]") {
				t.Fatalf("error = %v, want env passthrough field", err)
			}
		})
	}
}

func TestValidateInvalidEnvPassthroughDoesNotEchoInlineValue(t *testing.T) {
	cfg := validDockerConfig()
	cfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY=super-secret-value"}

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid env name")
	}
	if strings.Contains(err.Error(), "super-secret-value") {
		t.Fatalf("error leaked inline value: %v", err)
	}
}

func TestPlanDockerBuildsCommand(t *testing.T) {
	plan, err := PlanDocker(validDockerConfig(), "/tmp/workspace")
	if err != nil {
		t.Fatalf("PlanDocker() error = %v", err)
	}

	wantParts := []string{
		"docker run --rm --network none",
		"--memory 2g",
		"--cpus 2",
		"--read-only",
		"-w /workspace",
		"-v .:/workspace:rw",
		"-v mysecondbrain:/memory/mysecondbrain:ro",
		"--label deonclaw.workspace=/tmp/workspace",
		"deonclaw-runner:latest",
	}
	for _, want := range wantParts {
		if !strings.Contains(plan.Display, want) {
			t.Fatalf("display = %q, want %q", plan.Display, want)
		}
	}
}

func TestPlanDockerIncludesEnvPassthroughNamesOnly(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "super-secret-value")
	cfg := validDockerConfig()
	cfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY"}

	plan, err := PlanDocker(cfg, "/tmp/workspace")
	if err != nil {
		t.Fatalf("PlanDocker() error = %v", err)
	}

	wantParts := []string{"-e ZAI_API_KEY", "deonclaw-runner:latest"}
	for _, want := range wantParts {
		if !strings.Contains(plan.Display, want) {
			t.Fatalf("display = %q, want %q", plan.Display, want)
		}
	}
	if strings.Contains(plan.Display, "super-secret-value") {
		t.Fatalf("display leaked secret: %q", plan.Display)
	}
}

func TestPlanDockerExecAppendsCommandAfterImage(t *testing.T) {
	plan, err := PlanDockerExec(validDockerConfig(), "/tmp/workspace", []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("PlanDockerExec() error = %v", err)
	}

	if len(plan.Command) < 3 {
		t.Fatalf("command = %#v, want docker plan plus exec command", plan.Command)
	}
	imageIndex := -1
	for i, part := range plan.Command {
		if part == "deonclaw-runner:latest" {
			imageIndex = i
			break
		}
	}
	if imageIndex < 0 {
		t.Fatalf("command = %#v, want image", plan.Command)
	}
	gotTail := plan.Command[imageIndex+1:]
	wantTail := []string{"echo", "hello"}
	if strings.Join(gotTail, "\x00") != strings.Join(wantTail, "\x00") {
		t.Fatalf("command tail = %#v, want %#v", gotTail, wantTail)
	}
	if strings.Contains(plan.Display, "sh -c") {
		t.Fatalf("display = %q, must not use implicit shell", plan.Display)
	}
}

func TestPlanDockerExecRejectsMissingEnvPassthrough(t *testing.T) {
	unsetEnvForRuntimeConfigTest(t, "ZAI_API_KEY")
	cfg := validDockerConfig()
	cfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY"}

	_, err := PlanDockerExec(cfg, "/tmp/workspace", []string{"echo", "hello"})
	if err == nil {
		t.Fatal("PlanDockerExec() error = nil, want missing env rejection")
	}
	if !strings.Contains(err.Error(), "ZAI_API_KEY") {
		t.Fatalf("error = %v, want missing env name", err)
	}
}

func TestPlanDockerExecAcceptsSetEnvPassthrough(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "super-secret-value")
	cfg := validDockerConfig()
	cfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY"}

	plan, err := PlanDockerExec(cfg, "/tmp/workspace", []string{"echo", "hello"})
	if err != nil {
		t.Fatalf("PlanDockerExec() error = %v", err)
	}
	if !strings.Contains(plan.Display, "-e ZAI_API_KEY") {
		t.Fatalf("display = %q, want env passthrough flag", plan.Display)
	}
	if strings.Contains(plan.Display, "super-secret-value") {
		t.Fatalf("display leaked secret: %q", plan.Display)
	}
}

func TestPlanDockerExecRejectsEmptyCommand(t *testing.T) {
	_, err := PlanDockerExec(validDockerConfig(), "/tmp/workspace", nil)
	if err == nil {
		t.Fatal("PlanDockerExec() error = nil, want empty command rejection")
	}
	if !strings.Contains(err.Error(), "command must not be empty") {
		t.Fatalf("error = %v, want empty command rejection", err)
	}
}

func TestPlanDockerRejectsLocalRuntime(t *testing.T) {
	cfg := validDockerConfig()
	cfg.Runtime.Mode = "local"

	_, err := PlanDocker(cfg, ".")
	if err == nil {
		t.Fatal("PlanDocker() error = nil, want local rejection")
	}
	if !strings.Contains(err.Error(), `runtime.mode "local" does not support docker-plan`) {
		t.Fatalf("error = %v, want local rejection", err)
	}
}

func validDockerConfig() Config {
	return Config{
		Runtime: Runtime{
			Mode: ModeDocker,
			Docker: DockerConfig{
				Image:        "deonclaw-runner:latest",
				Workdir:      "/workspace",
				Network:      "none",
				ReadOnlyRoot: true,
				MemoryLimit:  "2g",
				CPUs:         "2",
				Mounts: []MountSpec{
					{Source: ".", Target: "/workspace", Mode: "rw"},
					{Source: "mysecondbrain", Target: "/memory/mysecondbrain", Mode: "ro"},
				},
			},
		},
	}
}

func writeRuntimeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func unsetEnvForRuntimeConfigTest(t *testing.T, name string) {
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
