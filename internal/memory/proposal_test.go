package memory

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestMemoryProposalJSONValid(t *testing.T) {
	createdAt := time.Date(2026, 5, 19, 12, 30, 0, 0, time.UTC)
	proposal := NewProposal(NewProposalOptions{
		ProposalID: "mem-test-001",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: "memory/context/example.md",
		Operation:  OperationAppend,
		Reason:     "Capture stable workflow.",
		Evidence: []MemoryEvidence{
			{Type: "run_artifact", Path: "artifacts/run-001/summary.md"},
		},
		Patches: []MemoryPatch{
			{TargetPath: "memory/context/example.md", Operation: OperationAppend, Content: "New memory note."},
		},
		CreatedAt: createdAt,
	})

	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("JSON() produced invalid JSON: %s", data)
	}

	var decoded MemoryProposal
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.ProposalID != "mem-test-001" {
		t.Fatalf("proposal_id = %q, want mem-test-001", decoded.ProposalID)
	}
	if decoded.Status != StatusProposed {
		t.Fatalf("status = %q, want %q", decoded.Status, StatusProposed)
	}
	if decoded.Operation != OperationAppend {
		t.Fatalf("operation = %q, want %q", decoded.Operation, OperationAppend)
	}
	if len(decoded.Evidence) != 1 || decoded.Evidence[0].Path != "artifacts/run-001/summary.md" {
		t.Fatalf("evidence = %#v, want run artifact evidence", decoded.Evidence)
	}
	if len(decoded.Patches) != 1 || decoded.Patches[0].Content != "New memory note." {
		t.Fatalf("patches = %#v, want content patch", decoded.Patches)
	}
}

func TestMemoryProposalMarkdownValid(t *testing.T) {
	proposal := NewProposal(NewProposalOptions{
		ProposalID: "mem-test-002",
		RunID:      "run-002",
		TaskID:     "task-002",
		Domain:     "escalasoft",
		TargetPath: "memory/escalasoft/example.md",
		Operation:  OperationUpdate,
		Reason:     "Capture domain-specific finding.",
		Evidence: []MemoryEvidence{
			{Type: "run", RunID: "run-002"},
		},
		Patches: []MemoryPatch{
			{TargetPath: "memory/escalasoft/example.md", Operation: OperationUpdate, Content: "Updated note."},
		},
		CreatedAt: time.Date(2026, 5, 19, 12, 45, 0, 0, time.UTC),
	})

	markdown, err := proposal.Markdown()
	if err != nil {
		t.Fatalf("Markdown() error = %v", err)
	}
	output := string(markdown)
	for _, want := range []string{
		"# Memory Proposal mem-test-002",
		"proposal_id: mem-test-002",
		"run_id: run-002",
		"task_id: task-002",
		"domain: escalasoft",
		"target_path: memory/escalasoft/example.md",
		"operation: update",
		"status: proposed",
		"reason: Capture domain-specific finding.",
		"## Evidence",
		"type: run",
		"## Patches",
		"content: |",
		"  Updated note.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("Markdown() = %q, want %q", output, want)
		}
	}
}

func TestValidateRejectsInvalidOperation(t *testing.T) {
	proposal := NewProposal(NewProposalOptions{
		ProposalID: "mem-test-003",
		RunID:      "run-003",
		TaskID:     "task-003",
		Domain:     "general",
		TargetPath: "memory/context/example.md",
		Operation:  MemoryOperation("delete"),
		Reason:     "Invalid operation should fail.",
		CreatedAt:  time.Date(2026, 5, 19, 13, 0, 0, 0, time.UTC),
	})

	err := proposal.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid operation error")
	}
	if !strings.Contains(err.Error(), `operation "delete" is not supported`) {
		t.Fatalf("Validate() error = %q, want invalid operation", err.Error())
	}
}

func TestNewProposalDefaultsStatusProposed(t *testing.T) {
	proposal := NewProposal(NewProposalOptions{
		ProposalID: "mem-test-004",
		RunID:      "run-004",
		TaskID:     "task-004",
		Domain:     "general",
		TargetPath: "memory/context/example.md",
		Operation:  OperationCreate,
		Reason:     "Create initial note.",
		CreatedAt:  time.Date(2026, 5, 19, 13, 15, 0, 0, time.UTC),
	})

	if proposal.Status != StatusProposed {
		t.Fatalf("status = %q, want %q", proposal.Status, StatusProposed)
	}
}
