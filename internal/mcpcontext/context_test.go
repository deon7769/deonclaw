package mcpcontext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestLoadDiscoveryAttachment(t *testing.T) {
	path := writeMCPContextTestFile(t, `{
  "tool_count": 2,
  "tool_names": ["fs.read", "fs.stat"],
  "tools": []
}`)
	ctx, err := Load(tasks.MCPContextSpec{Attachments: []tasks.MCPContextAttachment{{
		Name: "filesystem-discovery",
		Kind: KindDiscovery,
		Path: path,
	}}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ctx.Count() != 1 || ctx.Attachments[0].ToolCount != 2 || ctx.Attachments[0].SHA256 == "" {
		t.Fatalf("context = %#v, want discovery summary", ctx)
	}
	markdown := string(ctx.Markdown())
	if !strings.Contains(markdown, "# MCP Context Attachments") ||
		!strings.Contains(markdown, "tool_count: 2") ||
		!strings.Contains(markdown, "fs.read, fs.stat") {
		t.Fatalf("markdown = %q, want discovery summary", markdown)
	}
}

func TestLoadCallAttachmentRejectsInvalidBundleState(t *testing.T) {
	tests := []struct {
		name    string
		bundle  string
		wantErr string
	}{
		{
			name: "status failed",
			bundle: callBundleJSON(`"preflight_status":"passed",
  "status":"failed",
  "tool_calls":1`),
			wantErr: "status",
		},
		{
			name: "preflight failed",
			bundle: callBundleJSON(`"preflight_status":"failed",
  "status":"succeeded",
  "tool_calls":1`),
			wantErr: "preflight_status",
		},
		{
			name: "wrong tool calls",
			bundle: callBundleJSON(`"preflight_status":"passed",
  "status":"succeeded",
  "tool_calls":2`),
			wantErr: "tool_calls",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeMCPContextTestFile(t, tt.bundle)
			_, err := Load(tasks.MCPContextSpec{Attachments: []tasks.MCPContextAttachment{{
				Name: "filesystem-call",
				Kind: KindCall,
				Path: path,
			}}})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadCallAttachmentSummarizesWithoutRawResponse(t *testing.T) {
	path := writeMCPContextTestFile(t, callBundleJSON(`"preflight_status":"passed",
  "status":"succeeded",
  "tool_calls":1,
  "response_truncated":true`))
	ctx, err := Load(tasks.MCPContextSpec{Attachments: []tasks.MCPContextAttachment{{
		Name: "filesystem-call",
		Kind: KindCall,
		Path: path,
	}}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	markdown := string(ctx.Markdown())
	if !strings.Contains(markdown, "response_truncated: true") ||
		!strings.Contains(markdown, "mcp-call-response.json") {
		t.Fatalf("markdown = %q, want call summary", markdown)
	}
	if strings.Contains(markdown, "super-secret-value") || strings.Contains(markdown, "response_preview") {
		t.Fatalf("markdown leaked raw response detail: %q", markdown)
	}
}

func writeMCPContextTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "attachment.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func callBundleJSON(fields string) string {
	return `{
  "proposal_id":"proposal-001",
  "approval_sha256":"approval-hash",
  "proposal_sha256":"proposal-hash",
  "policy_sha256":"policy-hash",
  "server":"filesystem-readonly",
  "tool":"fs.read",
  "arguments_sha256":"args-hash",
  "runtime":"docker",
  ` + fields + `,
  "artifacts":{
    "mcp-call-smoke-summary.md":"artifacts/mcp-call-smoke-summary.md",
    "mcp-call-response.json":"artifacts/mcp-call-response.json"
  }
}`
}

func TestConstantsMatchMCPPackages(t *testing.T) {
	if mcpapproval.PreflightStatusPassed != "passed" || mcpsmoke.StatusSucceeded != "succeeded" {
		t.Fatalf("unexpected MCP status constants")
	}
}
