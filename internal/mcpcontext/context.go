package mcpcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const (
	KindDiscovery = "discovery"
	KindCall      = "call"
)

type Context struct {
	Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
	Name              string            `json:"name"`
	Kind              string            `json:"kind"`
	Path              string            `json:"path"`
	SHA256            string            `json:"sha256"`
	Summary           string            `json:"summary"`
	ToolCount         int               `json:"tool_count,omitempty"`
	ToolNames         []string          `json:"tool_names,omitempty"`
	ProposalID        string            `json:"proposal_id,omitempty"`
	Server            string            `json:"server,omitempty"`
	Tool              string            `json:"tool,omitempty"`
	Status            string            `json:"status,omitempty"`
	PreflightStatus   string            `json:"preflight_status,omitempty"`
	ToolCalls         int               `json:"tool_calls,omitempty"`
	ResponseTruncated bool              `json:"response_truncated,omitempty"`
	Artifacts         map[string]string `json:"artifacts,omitempty"`
}

func Load(spec tasks.MCPContextSpec) (Context, error) {
	ctx := Context{Attachments: []Attachment{}}
	for i, input := range spec.Attachments {
		attachment, err := loadAttachment(input)
		if err != nil {
			return Context{}, fmt.Errorf("mcp_context.attachments[%d]: %w", i, err)
		}
		ctx.Attachments = append(ctx.Attachments, attachment)
	}
	return ctx, nil
}

func (c Context) Markdown() []byte {
	if len(c.Attachments) == 0 {
		return nil
	}

	var output strings.Builder
	output.WriteString("# MCP Context Attachments\n\n")
	for _, attachment := range c.Attachments {
		output.WriteString("- name: ")
		output.WriteString(attachment.Name)
		output.WriteByte('\n')
		output.WriteString("  kind: ")
		output.WriteString(attachment.Kind)
		output.WriteByte('\n')
		output.WriteString("  path: ")
		output.WriteString(attachment.Path)
		output.WriteByte('\n')
		output.WriteString("  sha256: ")
		output.WriteString(attachment.SHA256)
		output.WriteByte('\n')
		output.WriteString("  summary: ")
		output.WriteString(attachment.Summary)
		output.WriteByte('\n')

		switch attachment.Kind {
		case KindDiscovery:
			output.WriteString("  tool_count: ")
			output.WriteString(fmt.Sprintf("%d", attachment.ToolCount))
			output.WriteByte('\n')
			output.WriteString("  tool_names: ")
			if len(attachment.ToolNames) == 0 {
				output.WriteString("none")
			} else {
				output.WriteString(strings.Join(attachment.ToolNames, ", "))
			}
			output.WriteByte('\n')
		case KindCall:
			output.WriteString("  proposal_id: ")
			output.WriteString(attachment.ProposalID)
			output.WriteByte('\n')
			output.WriteString("  server: ")
			output.WriteString(attachment.Server)
			output.WriteByte('\n')
			output.WriteString("  tool: ")
			output.WriteString(attachment.Tool)
			output.WriteByte('\n')
			output.WriteString("  status: ")
			output.WriteString(attachment.Status)
			output.WriteByte('\n')
			output.WriteString("  preflight_status: ")
			output.WriteString(attachment.PreflightStatus)
			output.WriteByte('\n')
			output.WriteString("  tool_calls: ")
			output.WriteString(fmt.Sprintf("%d", attachment.ToolCalls))
			output.WriteByte('\n')
			output.WriteString("  response_truncated: ")
			output.WriteString(fmt.Sprintf("%t", attachment.ResponseTruncated))
			output.WriteByte('\n')
			if len(attachment.Artifacts) > 0 {
				output.WriteString("  artifacts:\n")
				for _, name := range sortedArtifactNames(attachment.Artifacts) {
					output.WriteString("    - ")
					output.WriteString(name)
					output.WriteString(": ")
					output.WriteString(attachment.Artifacts[name])
					output.WriteByte('\n')
				}
			}
		}
	}
	return []byte(output.String())
}

func (c Context) Count() int {
	return len(c.Attachments)
}

func loadAttachment(input tasks.MCPContextAttachment) (Attachment, error) {
	name := strings.TrimSpace(input.Name)
	kind := strings.TrimSpace(input.Kind)
	path := strings.TrimSpace(input.Path)
	if name == "" {
		return Attachment{}, errors.New("name is required")
	}
	if path == "" {
		return Attachment{}, errors.New("path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Attachment{}, fmt.Errorf("read %q: %w", path, err)
	}
	hash := sha256.Sum256(data)
	base := Attachment{
		Name:   name,
		Kind:   kind,
		Path:   path,
		SHA256: hex.EncodeToString(hash[:]),
	}
	switch kind {
	case KindDiscovery:
		return loadDiscoveryAttachment(base, data)
	case KindCall:
		return loadCallAttachment(base, data)
	default:
		return Attachment{}, fmt.Errorf("kind %q is not supported", kind)
	}
}

func loadDiscoveryAttachment(base Attachment, data []byte) (Attachment, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Attachment{}, fmt.Errorf("parse discovery attachment json: %w", err)
	}
	if _, ok := raw["tool_count"]; !ok {
		return Attachment{}, errors.New("discovery attachment missing tool_count")
	}
	if _, ok := raw["tool_names"]; !ok {
		return Attachment{}, errors.New("discovery attachment missing tool_names")
	}
	var artifact mcpsmoke.ToolsListArtifact
	if err := json.Unmarshal(data, &artifact); err != nil {
		return Attachment{}, fmt.Errorf("parse discovery attachment fields: %w", err)
	}
	if artifact.ToolCount < 0 {
		return Attachment{}, errors.New("discovery attachment tool_count must not be negative")
	}
	if artifact.ToolNames == nil {
		return Attachment{}, errors.New("discovery attachment tool_names is required")
	}
	base.ToolCount = artifact.ToolCount
	base.ToolNames = append([]string(nil), artifact.ToolNames...)
	base.Summary = fmt.Sprintf("discovery listed %d MCP tools", artifact.ToolCount)
	return base, nil
}

func loadCallAttachment(base Attachment, data []byte) (Attachment, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Attachment{}, fmt.Errorf("parse call attachment json: %w", err)
	}
	for _, field := range []string{"proposal_id", "approval_sha256", "proposal_sha256", "policy_sha256"} {
		if len(raw[field]) == 0 {
			return Attachment{}, fmt.Errorf("call attachment missing %s", field)
		}
	}
	var bundle mcpapproval.MCPToolCallExecutionBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return Attachment{}, fmt.Errorf("parse call execution bundle: %w", err)
	}
	if strings.TrimSpace(bundle.ProposalID) == "" ||
		strings.TrimSpace(bundle.ApprovalSHA256) == "" ||
		strings.TrimSpace(bundle.ProposalSHA256) == "" ||
		strings.TrimSpace(bundle.PolicySHA256) == "" {
		return Attachment{}, errors.New("call attachment requires non-empty proposal and approval hashes")
	}
	if bundle.PreflightStatus != mcpapproval.PreflightStatusPassed {
		return Attachment{}, fmt.Errorf("call attachment preflight_status must be %q", mcpapproval.PreflightStatusPassed)
	}
	if bundle.Status != mcpsmoke.StatusSucceeded {
		return Attachment{}, fmt.Errorf("call attachment status must be %q", mcpsmoke.StatusSucceeded)
	}
	if bundle.ToolCalls != 1 {
		return Attachment{}, errors.New("call attachment tool_calls must equal 1")
	}
	base.ProposalID = bundle.ProposalID
	base.Server = bundle.Server
	base.Tool = bundle.Tool
	base.Status = bundle.Status
	base.PreflightStatus = bundle.PreflightStatus
	base.ToolCalls = bundle.ToolCalls
	base.ResponseTruncated = bundle.ResponseTruncated
	base.Artifacts = copyStringMap(bundle.Artifacts)
	base.Summary = fmt.Sprintf("approved read-only call %s/%s succeeded with %d tool call", bundle.Server, bundle.Tool, bundle.ToolCalls)
	return base, nil
}

func sortedArtifactNames(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func copyStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	copied := make(map[string]string, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}
