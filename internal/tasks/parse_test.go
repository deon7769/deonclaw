package tasks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	task, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if task.ID != "test-task-001" {
		t.Errorf("ID = %q, want test-task-001", task.ID)
	}
	if task.Worker != "codex" {
		t.Errorf("Worker = %q, want codex", task.Worker)
	}
	if task.Mode != "read_only" {
		t.Errorf("Mode = %q, want read_only", task.Mode)
	}
}

func TestParseValidationCommands(t *testing.T) {
	task, err := Parse([]byte(`id: validation-task-001
title: Validation task
domain: general
worker: codex
goal: Run validation
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
validation:
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
      timeout_seconds: 300
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - validation runs
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(task.Validation.Commands) != 1 {
		t.Fatalf("len(validation.commands) = %d, want 1", len(task.Validation.Commands))
	}
	command := task.Validation.Commands[0]
	if command.Name != "go-test" || command.Command != "go" || command.TimeoutSeconds != 300 {
		t.Fatalf("validation command = %#v, want go-test/go/300", command)
	}
	if len(command.Args) != 2 || command.Args[0] != "test" || command.Args[1] != "./..." {
		t.Fatalf("validation args = %#v, want test ./...", command.Args)
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestParseModelProfile(t *testing.T) {
	task, err := Parse([]byte(`id: profile-task-001
title: Profile task
domain: general
worker: opencode
model_profile: " opencode-zai-glm-5-1 "
goal: Run with model profile
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - profile parsed
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if task.ModelProfile != "opencode-zai-glm-5-1" {
		t.Fatalf("ModelProfile = %q, want opencode-zai-glm-5-1", task.ModelProfile)
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadCodexSmokeExample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "tasks", "codex-smoke.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example task not found at %s", path)
	}

	task, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if task.ID != "codex-smoke-001" {
		t.Errorf("ID = %q, want codex-smoke-001", task.ID)
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
