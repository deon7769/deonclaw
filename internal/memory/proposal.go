package memory

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const ProposalJSONArtifactName = "memory-proposal.json"
const ProposalMarkdownArtifactName = "memory-proposal.md"

type MemoryOperation string

const (
	OperationCreate  MemoryOperation = "create"
	OperationUpdate  MemoryOperation = "update"
	OperationAppend  MemoryOperation = "append"
	OperationArchive MemoryOperation = "archive"
)

type MemoryProposalStatus string

const StatusProposed MemoryProposalStatus = "proposed"

type MemoryProposal struct {
	ProposalID string               `json:"proposal_id"`
	RunID      string               `json:"run_id"`
	TaskID     string               `json:"task_id"`
	Domain     string               `json:"domain"`
	TargetPath string               `json:"target_path"`
	Operation  MemoryOperation      `json:"operation"`
	Status     MemoryProposalStatus `json:"status"`
	Reason     string               `json:"reason"`
	Evidence   []MemoryEvidence     `json:"evidence"`
	CreatedAt  time.Time            `json:"created_at"`
	Patches    []MemoryPatch        `json:"patches"`
}

type MemoryPatch struct {
	TargetPath  string          `json:"target_path"`
	Operation   MemoryOperation `json:"operation"`
	ArchivePath string          `json:"archive_path,omitempty"`
	Content     string          `json:"content,omitempty"`
}

type MemoryEvidence struct {
	Type        string `json:"type"`
	Path        string `json:"path,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Description string `json:"description,omitempty"`
}

type NewProposalOptions struct {
	ProposalID string
	RunID      string
	TaskID     string
	Domain     string
	TargetPath string
	Operation  MemoryOperation
	Reason     string
	Evidence   []MemoryEvidence
	CreatedAt  time.Time
	Patches    []MemoryPatch
}

func NewProposal(opts NewProposalOptions) MemoryProposal {
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	proposalID := strings.TrimSpace(opts.ProposalID)
	if proposalID == "" {
		proposalID = "mem-" + createdAt.UTC().Format("20060102T150405.000000000Z")
	}

	evidence := opts.Evidence
	if evidence == nil {
		evidence = []MemoryEvidence{}
	}
	patches := opts.Patches
	if patches == nil {
		patches = []MemoryPatch{
			{
				TargetPath: opts.TargetPath,
				Operation:  opts.Operation,
			},
		}
	}

	return MemoryProposal{
		ProposalID: proposalID,
		RunID:      opts.RunID,
		TaskID:     opts.TaskID,
		Domain:     opts.Domain,
		TargetPath: opts.TargetPath,
		Operation:  opts.Operation,
		Status:     StatusProposed,
		Reason:     opts.Reason,
		Evidence:   evidence,
		CreatedAt:  createdAt.UTC(),
		Patches:    patches,
	}
}

func (p MemoryProposal) Validate() error {
	var errs []error
	requireNonEmpty := func(field string, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}

	requireNonEmpty("proposal_id", p.ProposalID)
	requireNonEmpty("run_id", p.RunID)
	requireNonEmpty("task_id", p.TaskID)
	requireNonEmpty("domain", p.Domain)
	requireNonEmpty("target_path", p.TargetPath)
	requireNonEmpty("reason", p.Reason)
	if !validOperation(p.Operation) {
		errs = append(errs, fmt.Errorf("operation %q is not supported", p.Operation))
	}
	if p.Status == "" {
		errs = append(errs, fmt.Errorf("status is required"))
	} else if p.Status != StatusProposed {
		errs = append(errs, fmt.Errorf("status %q is not supported", p.Status))
	}
	if p.CreatedAt.IsZero() {
		errs = append(errs, fmt.Errorf("created_at is required"))
	}
	if len(errs) > 0 {
		return joinErrors(errs)
	}
	return nil
}

func (p MemoryProposal) JSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (p MemoryProposal) Markdown() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	var output strings.Builder
	output.WriteString("# Memory Proposal ")
	output.WriteString(p.ProposalID)
	output.WriteString("\n\n")

	writeMarkdownField(&output, "proposal_id", p.ProposalID)
	writeMarkdownField(&output, "run_id", p.RunID)
	writeMarkdownField(&output, "task_id", p.TaskID)
	writeMarkdownField(&output, "domain", p.Domain)
	writeMarkdownField(&output, "target_path", p.TargetPath)
	writeMarkdownField(&output, "operation", string(p.Operation))
	writeMarkdownField(&output, "status", string(p.Status))
	writeMarkdownField(&output, "reason", p.Reason)
	writeMarkdownField(&output, "created_at", p.CreatedAt.UTC().Format(time.RFC3339Nano))
	output.WriteByte('\n')

	output.WriteString("## Evidence\n")
	if len(p.Evidence) == 0 {
		output.WriteString("- none\n")
	} else {
		for _, evidence := range p.Evidence {
			output.WriteString("- type: ")
			output.WriteString(evidence.Type)
			output.WriteByte('\n')
			if evidence.Path != "" {
				output.WriteString("  path: ")
				output.WriteString(evidence.Path)
				output.WriteByte('\n')
			}
			if evidence.RunID != "" {
				output.WriteString("  run_id: ")
				output.WriteString(evidence.RunID)
				output.WriteByte('\n')
			}
			if evidence.Description != "" {
				output.WriteString("  description: ")
				output.WriteString(evidence.Description)
				output.WriteByte('\n')
			}
		}
	}
	output.WriteByte('\n')

	output.WriteString("## Patches\n")
	if len(p.Patches) == 0 {
		output.WriteString("- none\n")
	} else {
		for _, patch := range p.Patches {
			output.WriteString("- target_path: ")
			output.WriteString(patch.TargetPath)
			output.WriteByte('\n')
			output.WriteString("  operation: ")
			output.WriteString(string(patch.Operation))
			output.WriteByte('\n')
			if patch.ArchivePath != "" {
				output.WriteString("  archive_path: ")
				output.WriteString(patch.ArchivePath)
				output.WriteByte('\n')
			}
			if patch.Content != "" {
				output.WriteString("  content: |\n")
				writeIndented(&output, patch.Content, "    ")
			}
		}
	}

	return []byte(output.String()), nil
}

func validOperation(operation MemoryOperation) bool {
	switch operation {
	case OperationCreate, OperationUpdate, OperationAppend, OperationArchive:
		return true
	default:
		return false
	}
}

func writeMarkdownField(output *strings.Builder, key string, value string) {
	output.WriteString(key)
	output.WriteString(": ")
	output.WriteString(value)
	output.WriteByte('\n')
}

func writeIndented(output *strings.Builder, value string, prefix string) {
	lines := strings.Split(value, "\n")
	for i, line := range lines {
		if i == len(lines)-1 && line == "" {
			continue
		}
		output.WriteString(prefix)
		output.WriteString(line)
		output.WriteByte('\n')
	}
}

func joinErrors(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	var parts []string
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return errors.New(strings.Join(parts, "; "))
}
