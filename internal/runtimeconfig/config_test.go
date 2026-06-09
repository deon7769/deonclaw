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
	cases := []string{"/", "/home", "~/.ssh", ".env", "config/.env", "secrets", "config/secrets"}
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
