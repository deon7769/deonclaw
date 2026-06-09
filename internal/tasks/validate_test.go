package tasks

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateValidFixture(t *testing.T) {
	task, err := LoadFromFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateMissingID(t *testing.T) {
	task, err := LoadFromFile(filepath.Join("testdata", "missing-id.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "id is required") {
		t.Fatalf("error = %v, want id is required", err)
	}
}

func TestValidateBadMode(t *testing.T) {
	task, err := LoadFromFile(filepath.Join("testdata", "bad-mode.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	err = Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), `mode "invalid_mode" is not supported`) {
		t.Fatalf("error = %v, want unsupported mode", err)
	}
}

func TestValidateRequiresDefinitionOfDone(t *testing.T) {
	task := validTask()
	task.DefinitionOfDone = nil

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "definition_of_done must not be empty") {
		t.Fatalf("error = %v, want definition_of_done requirement", err)
	}
}

func TestValidateRequiresAllowedPathsForWorkspaceWrite(t *testing.T) {
	task := validTask()
	task.Mode = "workspace_write"
	task.AllowedPaths = nil

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "allowed_paths must not be empty for workspace_write mode") {
		t.Fatalf("error = %v, want workspace_write allowed_paths requirement", err)
	}
}

func TestValidateAllowsEmptyAllowedPathsForReadOnly(t *testing.T) {
	task := validTask()
	task.Mode = "read_only"
	task.AllowedPaths = nil

	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidValidationCommand(t *testing.T) {
	task := validTask()
	task.Validation.Commands = []ValidationCommand{
		{Name: "go-test", TimeoutSeconds: -1},
	}

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "validation.commands[0].command is required") {
		t.Fatalf("error = %v, want command requirement", err)
	}
	if !strings.Contains(err.Error(), "validation.commands[0].timeout_seconds must not be negative") {
		t.Fatalf("error = %v, want timeout requirement", err)
	}
}

func TestValidateRejectsModelProfileAndModelStrategy(t *testing.T) {
	task := validTask()
	task.ModelProfile = "opencode-zai-glm-5-1"
	task.ModelStrategy = &ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
	}

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "model_profile and model_strategy are mutually exclusive") {
		t.Fatalf("error = %v, want mutual exclusion error", err)
	}
}

func TestValidateRejectsEmptyModelStrategyPreferred(t *testing.T) {
	task := validTask()
	task.ModelStrategy = &ModelStrategy{}

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "model_strategy.preferred must not be empty") {
		t.Fatalf("error = %v, want preferred requirement", err)
	}
}

func TestValidateAcceptsModelStrategy(t *testing.T) {
	task := validTask()
	task.Worker = "opencode"
	task.ModelStrategy = &ModelStrategy{
		Preferred:   []string{"opencode-zai-glm-5-1"},
		Fallback:    []string{"opencode-fast"},
		RequireTags: []string{"coding"},
		FallbackPolicy: &FallbackPolicy{
			Enabled:      false,
			MaxAttempts:  1,
			RetryOn:      []string{"worker_failed", "validation_failed"},
			NeverRetryOn: []string{"policy_failed", "memory_policy_failed"},
		},
	}

	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateAcceptsDefaultDisabledFallbackPolicy(t *testing.T) {
	task := validTask()
	task.Worker = "opencode"
	task.ModelStrategy = &ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		FallbackPolicy: &FallbackPolicy{
			Enabled: false,
		},
	}

	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsEnabledFallbackPolicyWithoutPositiveAttempts(t *testing.T) {
	task := validTask()
	task.Worker = "opencode"
	task.ModelStrategy = &ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		FallbackPolicy: &FallbackPolicy{
			Enabled:     true,
			MaxAttempts: 0,
		},
	}

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "model_strategy.fallback_policy.max_attempts must be greater than zero") {
		t.Fatalf("error = %v, want max_attempts error", err)
	}

	task.ModelStrategy.FallbackPolicy.MaxAttempts = -1
	err = Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error for negative max_attempts, got nil")
	}
	if !strings.Contains(err.Error(), "model_strategy.fallback_policy.max_attempts must be greater than zero") {
		t.Fatalf("error = %v, want max_attempts error", err)
	}
}

func TestValidateRejectsPolicyFailedInFallbackRetryOn(t *testing.T) {
	task := validTask()
	task.Worker = "opencode"
	task.ModelStrategy = &ModelStrategy{
		Preferred: []string{"opencode-zai-glm-5-1"},
		FallbackPolicy: &FallbackPolicy{
			Enabled:     false,
			MaxAttempts: 1,
			RetryOn:     []string{"policy_failed"},
		},
	}

	err := Validate(task)
	if err == nil {
		t.Fatal("Validate() expected error, got nil")
	}
	if !strings.Contains(err.Error(), `model_strategy.fallback_policy.retry_on[0] "policy_failed" is not supported`) {
		t.Fatalf("error = %v, want policy_failed retry_on rejection", err)
	}
}

func validTask() *Task {
	return &Task{
		ID:     "valid-task-001",
		Title:  "Valid task",
		Domain: "general",
		Worker: "codex",
		Goal:   "Validate task policy",
		Mode:   "read_only",
		Workspace: WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: MemorySpec{
			Scope: "none",
		},
		AllowedPaths:     nil,
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"task validates"},
	}
}
