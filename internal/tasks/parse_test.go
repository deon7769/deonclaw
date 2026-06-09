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
  runtime: docker
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
	if task.Validation.Runtime != "docker" {
		t.Fatalf("validation.runtime = %q, want docker", task.Validation.Runtime)
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

func TestParseModelStrategy(t *testing.T) {
	task, err := Parse([]byte(`id: strategy-task-001
title: Strategy task
domain: general
worker: opencode
model_strategy:
  preferred:
    - " opencode-zai-glm-5-1 "
    - ""
  fallback:
    - " opencode-fast "
  require_tags:
    - " coding "
  fallback_policy:
    enabled: false
    max_attempts: 1
    retry_on:
      - " worker_failed "
      - ""
      - " validation_failed "
    never_retry_on:
      - " policy_failed "
      - " memory_policy_failed "
goal: Run with model strategy
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
  - strategy parsed
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if task.ModelStrategy == nil {
		t.Fatal("ModelStrategy = nil, want parsed strategy")
	}
	if len(task.ModelStrategy.Preferred) != 1 || task.ModelStrategy.Preferred[0] != "opencode-zai-glm-5-1" {
		t.Fatalf("preferred = %#v, want normalized opencode-zai-glm-5-1", task.ModelStrategy.Preferred)
	}
	if len(task.ModelStrategy.Fallback) != 1 || task.ModelStrategy.Fallback[0] != "opencode-fast" {
		t.Fatalf("fallback = %#v, want normalized opencode-fast", task.ModelStrategy.Fallback)
	}
	if len(task.ModelStrategy.RequireTags) != 1 || task.ModelStrategy.RequireTags[0] != "coding" {
		t.Fatalf("require_tags = %#v, want normalized coding", task.ModelStrategy.RequireTags)
	}
	if task.ModelStrategy.FallbackPolicy == nil {
		t.Fatal("fallback_policy = nil, want parsed policy")
	}
	if task.ModelStrategy.FallbackPolicy.Enabled {
		t.Fatalf("fallback_policy.enabled = true, want false")
	}
	if task.ModelStrategy.FallbackPolicy.MaxAttempts != 1 {
		t.Fatalf("fallback_policy.max_attempts = %d, want 1", task.ModelStrategy.FallbackPolicy.MaxAttempts)
	}
	if len(task.ModelStrategy.FallbackPolicy.RetryOn) != 2 || task.ModelStrategy.FallbackPolicy.RetryOn[0] != "worker_failed" || task.ModelStrategy.FallbackPolicy.RetryOn[1] != "validation_failed" {
		t.Fatalf("fallback_policy.retry_on = %#v, want normalized worker_failed/validation_failed", task.ModelStrategy.FallbackPolicy.RetryOn)
	}
	if len(task.ModelStrategy.FallbackPolicy.NeverRetryOn) != 2 || task.ModelStrategy.FallbackPolicy.NeverRetryOn[0] != "policy_failed" || task.ModelStrategy.FallbackPolicy.NeverRetryOn[1] != "memory_policy_failed" {
		t.Fatalf("fallback_policy.never_retry_on = %#v, want normalized never retry reasons", task.ModelStrategy.FallbackPolicy.NeverRetryOn)
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
