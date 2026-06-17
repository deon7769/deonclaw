package mcpapproval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/mcpconfig"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
)

func TestNewProposalGeneratesValidJSON(t *testing.T) {
	proposal, err := NewProposal(NewProposalOptions{
		CreatedAt:         time.Date(2026, 6, 16, 21, 0, 0, 0, time.UTC),
		Server:            "filesystem-readonly",
		Tool:              "fs.read",
		Arguments:         []byte(`{"path":"."}`),
		Reason:            "Read-only smoke.",
		RequestedBy:       "davi",
		PolicyPath:        "configs/examples/mcp-call-policy.yaml",
		Runtime:           mcpsmoke.RuntimeDocker,
		RuntimeConfigPath: "configs/examples/runtime.yaml",
		Workspace:         ".",
	})
	if err != nil {
		t.Fatalf("NewProposal() error = %v", err)
	}
	if proposal.Status != ProposalStatusProposed || proposal.Source.Type != SourceTypeManual {
		t.Fatalf("proposal = %#v, want proposed/manual", proposal)
	}
	if proposal.ArgumentsSHA256 == "" {
		t.Fatalf("proposal arguments hash is empty")
	}
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	if !strings.Contains(string(data), `"arguments_sha256"`) || !strings.Contains(string(data), `"status": "proposed"`) {
		t.Fatalf("proposal JSON = %s, want schema fields", data)
	}
}

func TestLintProposalPassesForReadonlyAllowlisted(t *testing.T) {
	proposal := testProposal(t, []byte(`{"text":"hello"}`))

	result := LintProposal(proposal, ValidationOptions{
		Config: testMCPConfig(nil, []string{mcpconfig.CapabilityRead}),
		Policy: testCallPolicy(65536, 1048576),
		RuntimeConfig: &runtimeconfig.Config{Runtime: runtimeconfig.Runtime{
			Mode:   runtimeconfig.ModeDocker,
			Docker: runtimeconfig.DockerConfig{Image: "deonclaw-runner:latest", Workdir: "/workspace", Mounts: []runtimeconfig.MountSpec{{Source: ".", Target: "/workspace", Mode: runtimeconfig.MountModeReadWrite}}},
		}},
	})
	if result.Status != PreflightStatusPassed || len(result.Violations) != 0 {
		t.Fatalf("lint = %#v, want passed", result)
	}
}

func TestLintProposalRejectsInvalidArguments(t *testing.T) {
	proposal := testProposal(t, []byte(`{"text":"hello"}`))
	proposal.Arguments = []byte(`"not-object"`)
	proposal.ArgumentsSHA256 = ArgumentsSHA256(proposal.Arguments)

	result := LintProposal(proposal, ValidationOptions{
		Config: testMCPConfig(nil, []string{mcpconfig.CapabilityRead}),
		Policy: testCallPolicy(65536, 1048576),
	})
	if result.Status != PreflightStatusFailed || len(result.Violations) == 0 {
		t.Fatalf("lint = %#v, want invalid arguments failure", result)
	}
}

func TestPreflightFailsForWriteExec(t *testing.T) {
	proposal := testProposal(t, []byte(`{"text":"hello"}`))

	preflight := BuildPreflight(proposal, ValidationOptions{
		Config: testMCPConfig(nil, []string{mcpconfig.CapabilityWrite}),
		Policy: testCallPolicy(65536, 1048576),
	})
	if preflight.Status != PreflightStatusFailed || preflight.Checks.NoWriteExec {
		t.Fatalf("preflight = %#v, want write/exec failure", preflight)
	}
}

func TestPreflightFailsForMissingEnv(t *testing.T) {
	unsetEnvForApprovalTest(t, "MCP_TOKEN")
	proposal := testProposal(t, []byte(`{"text":"hello"}`))

	preflight := BuildPreflight(proposal, ValidationOptions{
		Config: testMCPConfig([]string{"MCP_TOKEN"}, []string{mcpconfig.CapabilityRead}),
		Policy: testCallPolicy(65536, 1048576),
	})
	if preflight.Status != PreflightStatusFailed || preflight.Checks.EnvAvailable {
		t.Fatalf("preflight = %#v, want missing env failure", preflight)
	}
}

func TestBuildApprovalRequiresConfirmReadOnlyAndHashes(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("mcp_call_policy:\n  allow_real_readonly: true\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}
	proposal := testProposal(t, []byte(`{"text":"hello"}`))

	_, err := BuildApproval(proposal, NewApprovalOptions{
		Decision:   ApprovalDecisionApproved,
		Reason:     "Approve read-only call.",
		PolicyPath: policyPath,
	})
	if err == nil || !strings.Contains(err.Error(), "confirm_read_only") {
		t.Fatalf("BuildApproval() error = %v, want confirm_read_only", err)
	}

	approval, err := BuildApproval(proposal, NewApprovalOptions{
		Decision:        ApprovalDecisionApproved,
		ApprovedBy:      "davi",
		Reason:          "Approve read-only call.",
		PolicyPath:      policyPath,
		ConfirmReadOnly: true,
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	policySHA256, err := PolicySHA256FromFile(policyPath)
	if err != nil {
		t.Fatalf("PolicySHA256FromFile() error = %v", err)
	}
	if approval.ArgumentsSHA256 != proposal.ArgumentsSHA256 || approval.PolicySHA256 != policySHA256 {
		t.Fatalf("approval = %#v, want proposal/policy hashes", approval)
	}
}

func TestValidateExecutionRejectsChangedHashes(t *testing.T) {
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(policyPath, []byte("mcp_call_policy:\n  allow_real_readonly: true\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(policy) error = %v", err)
	}
	proposal := testProposal(t, []byte(`{"text":"hello"}`))
	approval, err := BuildApproval(proposal, NewApprovalOptions{
		Decision:        ApprovalDecisionApproved,
		Reason:          "Approve.",
		PolicyPath:      policyPath,
		ConfirmReadOnly: true,
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	proposal.Arguments = []byte(`{"text":"changed"}`)
	proposal.ArgumentsSHA256 = ArgumentsSHA256(proposal.Arguments)

	err = ValidateExecution(proposal, approval, policyPath, MCPToolCallPreflight{Status: PreflightStatusPassed})
	if err == nil || !strings.Contains(err.Error(), "arguments_sha256") {
		t.Fatalf("ValidateExecution() error = %v, want arguments hash mismatch", err)
	}
}

func testProposal(t *testing.T, arguments []byte) MCPToolCallProposal {
	t.Helper()
	proposal, err := NewProposal(NewProposalOptions{
		CreatedAt:         time.Date(2026, 6, 16, 21, 0, 0, 0, time.UTC),
		Server:            "filesystem-readonly",
		Tool:              mcpsmoke.FakeEchoToolName,
		Arguments:         arguments,
		Reason:            "Read-only call.",
		PolicyPath:        "policy.yaml",
		Runtime:           mcpsmoke.RuntimeDocker,
		RuntimeConfigPath: "runtime.yaml",
		Workspace:         ".",
	})
	if err != nil {
		t.Fatalf("NewProposal() error = %v", err)
	}
	return proposal
}

func testMCPConfig(env []string, capabilities []string) mcpconfig.Config {
	return mcpconfig.Config{MCP: mcpconfig.MCPConfig{Servers: map[string]mcpconfig.ServerConfig{
		"filesystem-readonly": {
			Command:      "fake-mcp",
			Enabled:      false,
			TestOnly:     false,
			Protocol:     mcpconfig.ProtocolStdio,
			Trust:        mcpconfig.TrustLocal,
			Capabilities: capabilities,
			Env:          mcpconfig.ServerEnv{Passthrough: env},
		},
	}}}
}

func testCallPolicy(maxArgumentsBytes int, maxResponseBytes int) mcpsmoke.CallPolicy {
	return mcpsmoke.CallPolicy{MCPCallPolicy: mcpsmoke.MCPCallPolicy{
		AllowRealReadonly:    true,
		RequireDockerForReal: true,
		MaxToolCalls:         1,
		AllowedServers:       []string{"filesystem-readonly"},
		AllowedTools:         []string{mcpsmoke.FakeEchoToolName},
		AllowedCapabilities:  []string{mcpconfig.CapabilityRead},
		MaxArgumentsBytes:    maxArgumentsBytes,
		MaxResponseBytes:     maxResponseBytes,
	}}
}

func unsetEnvForApprovalTest(t *testing.T, name string) {
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
