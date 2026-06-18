package tasks

import (
	"strings"
	"testing"
)

func TestValidateMaterializedInjectionEnabledFalseOK(t *testing.T) {
	task := validTask()
	task.RetrievalContext.MaterializedInjection = &MaterializedInjectionSpec{
		Enabled:            false,
		GovernanceBundle:   "artifacts/run-1/retrieval-context-injection-governance-bundle.json",
		PromptPreview:      "artifacts/run-1/retrieval-context-prompt-preview.md",
		RequireConfirmFlag: true,
		MaxTotalChars:      6000,
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateMaterializedInjectionEnabledTrueFails(t *testing.T) {
	task := validTask()
	task.RetrievalContext.MaterializedInjection = &MaterializedInjectionSpec{
		Enabled:            true,
		GovernanceBundle:   "artifacts/run-1/retrieval-context-injection-governance-bundle.json",
		PromptPreview:      "artifacts/run-1/retrieval-context-prompt-preview.md",
		RequireConfirmFlag: true,
		MaxTotalChars:      6000,
	}
	err := Validate(task)
	if err == nil || !strings.Contains(err.Error(), MaterializedInjectionNotSupportedYet) {
		t.Fatalf("error = %v, want not supported yet", err)
	}
}

func TestValidateMaterializedInjectionRejectsBlockedPaths(t *testing.T) {
	task := validTask()
	task.RetrievalContext.MaterializedInjection = &MaterializedInjectionSpec{
		Enabled:            false,
		GovernanceBundle:   "/tmp/bundle.json",
		PromptPreview:      "artifacts/run-1/retrieval-context-prompt-preview.md",
		RequireConfirmFlag: true,
		MaxTotalChars:      6000,
	}
	err := Validate(task)
	if err == nil || !strings.Contains(err.Error(), "must be a relative path") {
		t.Fatalf("error = %v, want relative path rejection", err)
	}

	task.RetrievalContext.MaterializedInjection.GovernanceBundle = "artifacts/../bundle.json"
	err = Validate(task)
	if err == nil || !strings.Contains(err.Error(), "must not contain ..") {
		t.Fatalf("error = %v, want .. rejection", err)
	}
}

func TestValidateMaterializedInjectionRequiresConfirmFlag(t *testing.T) {
	task := validTask()
	task.RetrievalContext.MaterializedInjection = &MaterializedInjectionSpec{
		Enabled:            false,
		GovernanceBundle:   "artifacts/run-1/retrieval-context-injection-governance-bundle.json",
		PromptPreview:      "artifacts/run-1/retrieval-context-prompt-preview.md",
		RequireConfirmFlag: false,
		MaxTotalChars:      6000,
	}
	err := Validate(task)
	if err == nil || !strings.Contains(err.Error(), "require_confirm_flag must be true") {
		t.Fatalf("error = %v, want require_confirm_flag failure", err)
	}
}

func TestParseMaterializedInjectionFromYAML(t *testing.T) {
	task, err := Parse([]byte(`id: task-1
title: "Materialized injection declaration"
domain: general
worker: codex
goal: "Declare future materialized injection"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
retrieval_context:
  materialized_injection:
    enabled: false
    governance_bundle: artifacts/run-1/retrieval-context-injection-governance-bundle.json
    prompt_preview: artifacts/run-1/retrieval-context-prompt-preview.md
    require_confirm_flag: true
    max_total_chars: 6000
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - schema declared only
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if task.RetrievalContext.MaterializedInjection == nil {
		t.Fatal("materialized_injection must be parsed")
	}
	if task.RetrievalContext.MaterializedInjection.Enabled {
		t.Fatal("enabled must be false")
	}
}
