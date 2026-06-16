package mcpsmoke

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/mcpconfig"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
)

func TestFakeServerRespondsInitializeAndToolsList(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":3,"method":"shutdown"}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
		"",
	}, "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := RunFakeServer(context.Background(), strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunFakeServer() error = %v, stderr=%q", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, `"id":1`) || !strings.Contains(output, `"serverInfo"`) {
		t.Fatalf("stdout = %q, want initialize response", output)
	}
	if !strings.Contains(output, `"id":2`) || !strings.Contains(output, FakeEchoToolName) {
		t.Fatalf("stdout = %q, want fake echo tool in tools/list response", output)
	}
}

func TestFakeServerToolCallEchoReturnsPayload(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"deonclaw.fake.echo","arguments":{"text":"hello"}}}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
		"",
	}, "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := RunFakeServer(context.Background(), strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunFakeServer() error = %v, stderr=%q", err, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, `"id":1`) || !strings.Contains(output, `"content"`) || !strings.Contains(output, `"hello"`) {
		t.Fatalf("stdout = %q, want echo tool response", output)
	}
}

func TestFakeServerUnknownToolReturnsJSONRPCError(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"other.tool","arguments":{"text":"hello"}}}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
		"",
	}, "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	err := RunFakeServer(context.Background(), strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunFakeServer() error = %v, stderr=%q", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"error"`) || !strings.Contains(stdout.String(), `"unknown tool"`) {
		t.Fatalf("stdout = %q, want unknown tool JSON-RPC error", stdout.String())
	}
}

func TestSmokeLocalGeneratesArtifactsAndTranscript(t *testing.T) {
	t.Setenv("DEONCLAW_MCP_SMOKE_HELPER", "1")
	t.Setenv("MCP_TOKEN", "super-secret-value")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := mcpSmokeTestConfig(t, []string{"MCP_TOKEN"}, "fake")

	result, err := Smoke(context.Background(), Options{
		Config:         cfg,
		Server:         "fake-stdio",
		ArtifactsDir:   artifactsDir,
		Timeout:        3 * time.Second,
		Runtime:        RuntimeLocal,
		RuntimeConfig:  nil,
		Workspace:      ".",
		StartedAtClock: fixedSmokeClock,
	})
	if err != nil {
		t.Fatalf("Smoke() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.Server != "fake-stdio" || result.ToolCalls != 0 {
		t.Fatalf("result = %#v, want successful fake smoke without tool calls", result)
	}
	for _, name := range []string{"mcp-smoke-summary.md", "mcp-transcript.jsonl", "mcp-stdout.log", "mcp-stderr.log", "mcp-smoke-result.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := readSmokeArtifact(t, artifactsDir, "mcp-transcript.jsonl")
	assertTranscriptJSONLValid(t, transcript)
	if !strings.Contains(transcript, `"direction":"request"`) || !strings.Contains(transcript, `"direction":"response"`) || !strings.Contains(transcript, `"tools/list"`) {
		t.Fatalf("transcript = %q, want request/response tools/list", transcript)
	}
	allArtifacts := transcript + readSmokeArtifact(t, artifactsDir, "mcp-smoke-summary.md") + readSmokeArtifact(t, artifactsDir, "mcp-smoke-result.json")
	if strings.Contains(allArtifacts, "super-secret-value") {
		t.Fatalf("artifacts leaked env value: %q", allArtifacts)
	}
}

func TestSmokeRejectsServerWithoutTestOnly(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["fake-stdio"]
	server.TestOnly = false
	cfg.MCP.Servers["fake-stdio"] = server

	_, err := Smoke(context.Background(), Options{
		Config:       cfg,
		Server:       "fake-stdio",
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
	})
	if err == nil || !strings.Contains(err.Error(), "test_only=true") {
		t.Fatalf("Smoke() error = %v, want test_only rejection", err)
	}
}

func TestSmokeRejectsWriteExecCapability(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["fake-stdio"]
	server.Capabilities = []string{mcpconfig.CapabilityWrite}
	cfg.MCP.Servers["fake-stdio"] = server

	_, err := Smoke(context.Background(), Options{
		Config:       cfg,
		Server:       "fake-stdio",
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
	})
	if err == nil || !strings.Contains(err.Error(), "write or exec") {
		t.Fatalf("Smoke() error = %v, want write/exec rejection", err)
	}
}

func TestSmokeTimeoutFailsControlled(t *testing.T) {
	t.Setenv("DEONCLAW_MCP_SMOKE_HELPER", "1")
	cfg := mcpSmokeTestConfig(t, nil, "hang")

	_, err := Smoke(context.Background(), Options{
		Config:       cfg,
		Server:       "fake-stdio",
		ArtifactsDir: t.TempDir(),
		Timeout:      100 * time.Millisecond,
		Runtime:      RuntimeLocal,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Smoke() error = %v, want timeout", err)
	}
}

func TestSmokeDockerWithFakeDockerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installMCPFakeDocker(t, "fake", "docker fake stderr\n")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := mcpSmokeTestConfig(t, []string{"MCP_TOKEN"}, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)

	result, err := Smoke(context.Background(), Options{
		Config:         cfg,
		Server:         "fake-stdio",
		ArtifactsDir:   artifactsDir,
		Timeout:        3 * time.Second,
		Runtime:        RuntimeDocker,
		RuntimeConfig:  &runtimeCfg,
		Workspace:      ".",
		StartedAtClock: fixedSmokeClock,
	})
	if err != nil {
		t.Fatalf("Smoke() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.Runtime != RuntimeDocker || result.ToolCalls != 0 {
		t.Fatalf("result = %#v, want successful docker fake smoke without tool calls", result)
	}
	for _, name := range []string{"mcp-smoke-summary.md", "mcp-transcript.jsonl", "mcp-stdout.log", "mcp-stderr.log", "mcp-smoke-result.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := readSmokeArtifact(t, artifactsDir, "mcp-transcript.jsonl")
	assertTranscriptJSONLValid(t, transcript)
	if !strings.Contains(transcript, `"tools/list"`) {
		t.Fatalf("transcript = %q, want tools/list", transcript)
	}
	stdoutLog := readSmokeArtifact(t, artifactsDir, "mcp-stdout.log")
	if !strings.Contains(stdoutLog, `"serverInfo"`) {
		t.Fatalf("stdout log = %q, want JSON-RPC responses", stdoutLog)
	}
	stderrLog := readSmokeArtifact(t, artifactsDir, "mcp-stderr.log")
	if !strings.Contains(stderrLog, "docker fake stderr") {
		t.Fatalf("stderr log = %q, want docker stderr capture", stderrLog)
	}
	args := readDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOf(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want runtime image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}
	allArtifacts := transcript + stdoutLog + stderrLog + readSmokeArtifact(t, artifactsDir, "mcp-smoke-result.json")
	if strings.Contains(allArtifacts, "super-secret-value") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("secret leaked; args=%#v artifacts=%q", args, allArtifacts)
	}
}

func TestSmokeDockerMissingServerEnvFailsBeforeDocker(t *testing.T) {
	unsetEnvForSmokeTest(t, "MCP_TOKEN")
	argsPath := installMCPFakeDocker(t, "fake", "")
	cfg := mcpSmokeTestConfig(t, []string{"MCP_TOKEN"}, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)

	_, err := Smoke(context.Background(), Options{
		Config:        cfg,
		Server:        "fake-stdio",
		ArtifactsDir:  t.TempDir(),
		Timeout:       time.Second,
		Runtime:       RuntimeDocker,
		RuntimeConfig: &runtimeCfg,
		Workspace:     ".",
	})
	if err == nil || !strings.Contains(err.Error(), "MCP_TOKEN") {
		t.Fatalf("Smoke() error = %v, want missing MCP_TOKEN", err)
	}
	assertSmokeFileEmptyOrMissing(t, argsPath)
}

func TestSmokeDockerRejectsServerWithoutTestOnlyBeforeDocker(t *testing.T) {
	argsPath := installMCPFakeDocker(t, "fake", "")
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["fake-stdio"]
	server.TestOnly = false
	cfg.MCP.Servers["fake-stdio"] = server
	runtimeCfg := dockerSmokeRuntimeConfig(nil)

	_, err := Smoke(context.Background(), Options{
		Config:        cfg,
		Server:        "fake-stdio",
		ArtifactsDir:  t.TempDir(),
		Timeout:       time.Second,
		Runtime:       RuntimeDocker,
		RuntimeConfig: &runtimeCfg,
		Workspace:     ".",
	})
	if err == nil || !strings.Contains(err.Error(), "test_only=true") {
		t.Fatalf("Smoke() error = %v, want test_only rejection", err)
	}
	assertSmokeFileEmptyOrMissing(t, argsPath)
}

func TestSmokeDockerRejectsWriteExecCapabilityBeforeDocker(t *testing.T) {
	argsPath := installMCPFakeDocker(t, "fake", "")
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["fake-stdio"]
	server.Capabilities = []string{mcpconfig.CapabilityExec}
	cfg.MCP.Servers["fake-stdio"] = server
	runtimeCfg := dockerSmokeRuntimeConfig(nil)

	_, err := Smoke(context.Background(), Options{
		Config:        cfg,
		Server:        "fake-stdio",
		ArtifactsDir:  t.TempDir(),
		Timeout:       time.Second,
		Runtime:       RuntimeDocker,
		RuntimeConfig: &runtimeCfg,
		Workspace:     ".",
	})
	if err == nil || !strings.Contains(err.Error(), "write or exec") {
		t.Fatalf("Smoke() error = %v, want write/exec rejection", err)
	}
	assertSmokeFileEmptyOrMissing(t, argsPath)
}

func TestSmokeDockerTimeoutFailsControlled(t *testing.T) {
	_ = installMCPFakeDocker(t, "hang", "")
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)

	_, err := Smoke(context.Background(), Options{
		Config:        cfg,
		Server:        "fake-stdio",
		ArtifactsDir:  t.TempDir(),
		Timeout:       100 * time.Millisecond,
		Runtime:       RuntimeDocker,
		RuntimeConfig: &runtimeCfg,
		Workspace:     ".",
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Smoke() error = %v, want timeout", err)
	}
}

func TestToolSmokeLocalGeneratesArtifactsAndCallsToolOnce(t *testing.T) {
	t.Setenv("DEONCLAW_MCP_SMOKE_HELPER", "1")
	t.Setenv("MCP_TOKEN", "super-secret-value")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := mcpSmokeTestConfig(t, []string{"MCP_TOKEN"}, "fake")
	policy := testToolPolicy()

	result, err := ToolSmoke(context.Background(), ToolOptions{
		Config:         cfg,
		Server:         "fake-stdio",
		Tool:           FakeEchoToolName,
		Arguments:      []byte(`{"text":"hello"}`),
		ArtifactsDir:   artifactsDir,
		Timeout:        3 * time.Second,
		Runtime:        RuntimeLocal,
		Policy:         &policy,
		StartedAtClock: fixedSmokeClock,
	})
	if err != nil {
		t.Fatalf("ToolSmoke() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.Tool != FakeEchoToolName || result.ToolCalls != 1 {
		t.Fatalf("result = %#v, want one successful fake tool call", result)
	}
	for _, name := range []string{"mcp-tool-smoke-summary.md", "mcp-tool-transcript.jsonl", "mcp-tool-stdout.log", "mcp-tool-stderr.log", "mcp-tool-result.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := readSmokeArtifact(t, artifactsDir, "mcp-tool-transcript.jsonl")
	assertTranscriptJSONLValid(t, transcript)
	assertTranscriptRequestMethodCount(t, transcript, "tools/call", 1)
	if !strings.Contains(transcript, `"hello"`) {
		t.Fatalf("transcript = %q, want echo payload", transcript)
	}
	allArtifacts := transcript + readSmokeArtifact(t, artifactsDir, "mcp-tool-smoke-summary.md") + readSmokeArtifact(t, artifactsDir, "mcp-tool-result.json")
	if strings.Contains(allArtifacts, "super-secret-value") {
		t.Fatalf("artifacts leaked env value: %q", allArtifacts)
	}
}

func TestToolSmokeDockerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installMCPFakeDocker(t, "fake", "docker fake stderr\n")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := mcpSmokeTestConfig(t, []string{"MCP_TOKEN"}, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)
	policy := testToolPolicy()

	result, err := ToolSmoke(context.Background(), ToolOptions{
		Config:         cfg,
		Server:         "fake-stdio",
		Tool:           FakeEchoToolName,
		Arguments:      []byte(`{"text":"hello docker"}`),
		ArtifactsDir:   artifactsDir,
		Timeout:        3 * time.Second,
		Runtime:        RuntimeDocker,
		RuntimeConfig:  &runtimeCfg,
		Workspace:      ".",
		Policy:         &policy,
		StartedAtClock: fixedSmokeClock,
	})
	if err != nil {
		t.Fatalf("ToolSmoke() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.Runtime != RuntimeDocker || result.ToolCalls != 1 {
		t.Fatalf("result = %#v, want successful docker fake tool smoke", result)
	}
	transcript := readSmokeArtifact(t, artifactsDir, "mcp-tool-transcript.jsonl")
	assertTranscriptJSONLValid(t, transcript)
	assertTranscriptRequestMethodCount(t, transcript, "tools/call", 1)
	if !strings.Contains(readSmokeArtifact(t, artifactsDir, "mcp-tool-stderr.log"), "docker fake stderr") {
		t.Fatalf("stderr artifact missing docker stderr")
	}
	args := readDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOf(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want runtime image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want fake server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("docker args unsafe or leaked secret: %#v", args)
	}
}

func TestToolSmokeRejectsToolNotAllowlisted(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	policy := testToolPolicy()
	policy.MCPToolPolicy.AllowedTools = []string{"other.tool"}

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":"hello"}`),
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("ToolSmoke() error = %v, want tool allowlist rejection", err)
	}
}

func TestToolSmokeRejectsServerNotAllowlisted(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	policy := testToolPolicy()
	policy.MCPToolPolicy.AllowedServers = []string{"other-server"}

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":"hello"}`),
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "not allowlisted") {
		t.Fatalf("ToolSmoke() error = %v, want server allowlist rejection", err)
	}
}

func TestToolSmokeRejectsWriteExecCapability(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["fake-stdio"]
	server.Capabilities = []string{mcpconfig.CapabilityWrite}
	cfg.MCP.Servers["fake-stdio"] = server
	policy := testToolPolicy()

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":"hello"}`),
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "write or exec") {
		t.Fatalf("ToolSmoke() error = %v, want write/exec rejection", err)
	}
}

func TestToolSmokeRejectsServerWithoutTestOnly(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["fake-stdio"]
	server.TestOnly = false
	cfg.MCP.Servers["fake-stdio"] = server
	policy := testToolPolicy()

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":"hello"}`),
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "test_only=true") {
		t.Fatalf("ToolSmoke() error = %v, want test_only rejection", err)
	}
}

func TestToolSmokeRejectsInvalidArguments(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	policy := testToolPolicy()

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":`),
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Fatalf("ToolSmoke() error = %v, want invalid arguments rejection", err)
	}
}

func TestToolSmokeRejectsOversizedArguments(t *testing.T) {
	cfg := mcpSmokeTestConfig(t, nil, "fake")
	policy := testToolPolicy()

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":"` + strings.Repeat("x", MaxToolArgumentsBytes) + `"}`),
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("ToolSmoke() error = %v, want oversized arguments rejection", err)
	}
}

func TestToolSmokeTimeoutFailsControlled(t *testing.T) {
	t.Setenv("DEONCLAW_MCP_SMOKE_HELPER", "1")
	cfg := mcpSmokeTestConfig(t, nil, "hang")
	policy := testToolPolicy()

	_, err := ToolSmoke(context.Background(), ToolOptions{
		Config:       cfg,
		Server:       "fake-stdio",
		Tool:         FakeEchoToolName,
		Arguments:    []byte(`{"text":"hello"}`),
		ArtifactsDir: t.TempDir(),
		Timeout:      100 * time.Millisecond,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("ToolSmoke() error = %v, want timeout", err)
	}
}

func TestLoadExampleToolPolicy(t *testing.T) {
	policy, err := LoadToolPolicy(filepath.Join("..", "..", "configs", "examples", "mcp-tool-policy.yaml"))
	if err != nil {
		t.Fatalf("LoadToolPolicy() error = %v", err)
	}
	if err := ValidateToolPolicy(policy); err != nil {
		t.Fatalf("ValidateToolPolicy() error = %v", err)
	}
	if !stringSliceContainsSequence(policy.MCPToolPolicy.AllowedTools, []string{FakeEchoToolName}) {
		t.Fatalf("allowed tools = %#v, want fake echo", policy.MCPToolPolicy.AllowedTools)
	}
}

func TestToolPolicyRejectsWriteExecAllowedCapabilities(t *testing.T) {
	policy := testToolPolicy()
	policy.MCPToolPolicy.AllowedCapabilities = []string{mcpconfig.CapabilityRead, mcpconfig.CapabilityExec}

	err := ValidateToolPolicy(policy)
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("ValidateToolPolicy() error = %v, want write/exec rejection", err)
	}
}

func TestDiscoverRejectsServerWithoutPolicy(t *testing.T) {
	cfg := mcpDiscoveryTestConfig(t, nil, "fake")

	_, err := Discover(context.Background(), DiscoveryOptions{
		Config:       cfg,
		Server:       "filesystem-readonly",
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeDocker,
	})
	if err == nil || !strings.Contains(err.Error(), "--policy") {
		t.Fatalf("Discover() error = %v, want missing policy rejection", err)
	}
}

func TestDiscoverRejectsWriteExecCapability(t *testing.T) {
	cfg := mcpDiscoveryTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Capabilities = []string{mcpconfig.CapabilityExec}
	cfg.MCP.Servers["filesystem-readonly"] = server
	policy := testDiscoveryPolicy()

	_, err := Discover(context.Background(), DiscoveryOptions{
		Config:       cfg,
		Server:       "filesystem-readonly",
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeDocker,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "write or exec") {
		t.Fatalf("Discover() error = %v, want write/exec rejection", err)
	}
}

func TestDiscoverRejectsEnabledServer(t *testing.T) {
	cfg := mcpDiscoveryTestConfig(t, nil, "fake")
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Enabled = true
	cfg.MCP.Servers["filesystem-readonly"] = server
	policy := testDiscoveryPolicy()

	_, err := Discover(context.Background(), DiscoveryOptions{
		Config:       cfg,
		Server:       "filesystem-readonly",
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeDocker,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "enabled=false") {
		t.Fatalf("Discover() error = %v, want enabled=false rejection", err)
	}
}

func TestDiscoverRejectsRealServerLocalWhenDockerRequired(t *testing.T) {
	cfg := mcpDiscoveryTestConfig(t, nil, "fake")
	policy := testDiscoveryPolicy()

	_, err := Discover(context.Background(), DiscoveryOptions{
		Config:       cfg,
		Server:       "filesystem-readonly",
		ArtifactsDir: t.TempDir(),
		Timeout:      time.Second,
		Runtime:      RuntimeLocal,
		Policy:       &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "--runtime docker") {
		t.Fatalf("Discover() error = %v, want docker-required rejection", err)
	}
}

func TestDiscoverDockerWithFakeRealReadonlyServerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installMCPFakeDocker(t, "fake", "docker fake stderr\n")
	artifactsDir := filepath.Join(t.TempDir(), "artifacts")
	cfg := mcpDiscoveryTestConfig(t, []string{"MCP_TOKEN"}, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)
	policy := testDiscoveryPolicy()

	result, err := Discover(context.Background(), DiscoveryOptions{
		Config:         cfg,
		Server:         "filesystem-readonly",
		ArtifactsDir:   artifactsDir,
		Timeout:        3 * time.Second,
		Runtime:        RuntimeDocker,
		RuntimeConfig:  &runtimeCfg,
		Workspace:      ".",
		Policy:         &policy,
		StartedAtClock: fixedSmokeClock,
	})
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if result.Status != StatusSucceeded || result.Runtime != RuntimeDocker || result.TestOnly || result.ToolCalls != 0 || result.ToolCount != 1 {
		t.Fatalf("result = %#v, want successful real read-only discovery without tool calls", result)
	}
	for _, name := range []string{"mcp-discovery-summary.md", "mcp-discovery-transcript.jsonl", "mcp-discovery-stdout.log", "mcp-discovery-stderr.log", "mcp-discovery-result.json", "mcp-tools-list.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := readSmokeArtifact(t, artifactsDir, "mcp-discovery-transcript.jsonl")
	assertTranscriptJSONLValid(t, transcript)
	assertTranscriptRequestMethodCount(t, transcript, "initialize", 1)
	assertTranscriptRequestMethodCount(t, transcript, "tools/list", 1)
	assertTranscriptRequestMethodCount(t, transcript, "shutdown", 1)
	assertTranscriptRequestMethodCount(t, transcript, "exit", 1)
	assertTranscriptRequestMethodCount(t, transcript, "tools/call", 0)

	var toolsList ToolsListArtifact
	if err := json.Unmarshal([]byte(readSmokeArtifact(t, artifactsDir, "mcp-tools-list.json")), &toolsList); err != nil {
		t.Fatalf("mcp-tools-list.json invalid: %v", err)
	}
	if toolsList.ToolCount != 1 || !stringSliceContainsSequence(toolsList.ToolNames, []string{FakeEchoToolName}) || len(toolsList.Tools) != 1 || len(toolsList.Tools[0].InputSchema) == 0 {
		t.Fatalf("tools list = %#v, want fake echo metadata with schema", toolsList)
	}

	args := readDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOf(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want runtime image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}
	allArtifacts := transcript +
		readSmokeArtifact(t, artifactsDir, "mcp-discovery-summary.md") +
		readSmokeArtifact(t, artifactsDir, "mcp-discovery-result.json") +
		readSmokeArtifact(t, artifactsDir, "mcp-tools-list.json") +
		readSmokeArtifact(t, artifactsDir, "mcp-discovery-stdout.log") +
		readSmokeArtifact(t, artifactsDir, "mcp-discovery-stderr.log")
	if strings.Contains(allArtifacts, "super-secret-value") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("secret leaked; args=%#v artifacts=%q", args, allArtifacts)
	}
}

func TestDiscoverDockerMissingServerEnvFailsBeforeDocker(t *testing.T) {
	unsetEnvForSmokeTest(t, "MCP_TOKEN")
	argsPath := installMCPFakeDocker(t, "fake", "")
	cfg := mcpDiscoveryTestConfig(t, []string{"MCP_TOKEN"}, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)
	policy := testDiscoveryPolicy()

	_, err := Discover(context.Background(), DiscoveryOptions{
		Config:        cfg,
		Server:        "filesystem-readonly",
		ArtifactsDir:  t.TempDir(),
		Timeout:       time.Second,
		Runtime:       RuntimeDocker,
		RuntimeConfig: &runtimeCfg,
		Workspace:     ".",
		Policy:        &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "MCP_TOKEN") {
		t.Fatalf("Discover() error = %v, want missing MCP_TOKEN", err)
	}
	assertSmokeFileEmptyOrMissing(t, argsPath)
}

func TestDiscoverDockerTimeoutFailsControlled(t *testing.T) {
	_ = installMCPFakeDocker(t, "hang", "")
	cfg := mcpDiscoveryTestConfig(t, nil, "fake")
	runtimeCfg := dockerSmokeRuntimeConfig(nil)
	policy := testDiscoveryPolicy()

	_, err := Discover(context.Background(), DiscoveryOptions{
		Config:        cfg,
		Server:        "filesystem-readonly",
		ArtifactsDir:  t.TempDir(),
		Timeout:       100 * time.Millisecond,
		Runtime:       RuntimeDocker,
		RuntimeConfig: &runtimeCfg,
		Workspace:     ".",
		Policy:        &policy,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Discover() error = %v, want timeout", err)
	}
}

func TestLoadExampleDiscoveryPolicy(t *testing.T) {
	policy, err := LoadDiscoveryPolicy(filepath.Join("..", "..", "configs", "examples", "mcp-discovery-policy.yaml"))
	if err != nil {
		t.Fatalf("LoadDiscoveryPolicy() error = %v", err)
	}
	if err := ValidateDiscoveryPolicy(policy); err != nil {
		t.Fatalf("ValidateDiscoveryPolicy() error = %v", err)
	}
	if !stringSliceContainsSequence(policy.MCPDiscoveryPolicy.AllowedServers, []string{"filesystem-readonly"}) {
		t.Fatalf("allowed servers = %#v, want filesystem-readonly", policy.MCPDiscoveryPolicy.AllowedServers)
	}
}

func TestDiscoveryPolicyRejectsWriteExecAllowedCapabilities(t *testing.T) {
	policy := testDiscoveryPolicy()
	policy.MCPDiscoveryPolicy.AllowedCapabilities = []string{mcpconfig.CapabilityRead, mcpconfig.CapabilityWrite}

	err := ValidateDiscoveryPolicy(policy)
	if err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("ValidateDiscoveryPolicy() error = %v, want write/exec rejection", err)
	}
}

func TestMCPFakeServerHelperProcess(t *testing.T) {
	if os.Getenv("DEONCLAW_MCP_SMOKE_HELPER") != "1" {
		return
	}
	args := helperArgs()
	if len(args) > 0 && args[0] == "hang" {
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	if err := RunFakeServer(context.Background(), os.Stdin, os.Stdout, os.Stderr); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestMCPFakeDockerHelperProcess(t *testing.T) {
	if os.Getenv("DEONCLAW_MCP_FAKE_DOCKER_HELPER") != "1" {
		return
	}
	os.Exit(runMCPFakeDockerHelper())
}

func mcpSmokeTestConfig(t *testing.T, env []string, mode string) mcpconfig.Config {
	t.Helper()
	return mcpconfig.Config{
		MCP: mcpconfig.MCPConfig{
			Servers: map[string]mcpconfig.ServerConfig{
				"fake-stdio": {
					Command:      os.Args[0],
					Args:         []string{"-test.run=TestMCPFakeServerHelperProcess", "--", mode},
					Enabled:      false,
					TestOnly:     true,
					Protocol:     mcpconfig.ProtocolStdio,
					Trust:        mcpconfig.TrustLocal,
					Capabilities: []string{mcpconfig.CapabilityRead},
					Env:          mcpconfig.ServerEnv{Passthrough: env},
				},
			},
		},
	}
}

func mcpDiscoveryTestConfig(t *testing.T, env []string, mode string) mcpconfig.Config {
	t.Helper()
	return mcpconfig.Config{
		MCP: mcpconfig.MCPConfig{
			Servers: map[string]mcpconfig.ServerConfig{
				"filesystem-readonly": {
					Command:      os.Args[0],
					Args:         []string{"-test.run=TestMCPFakeServerHelperProcess", "--", mode},
					Enabled:      false,
					TestOnly:     false,
					Protocol:     mcpconfig.ProtocolStdio,
					Trust:        mcpconfig.TrustLocal,
					Capabilities: []string{mcpconfig.CapabilityRead},
					Env:          mcpconfig.ServerEnv{Passthrough: env},
				},
			},
		},
	}
}

func dockerSmokeRuntimeConfig(env []string) runtimeconfig.Config {
	return runtimeconfig.Config{
		Runtime: runtimeconfig.Runtime{
			Mode: runtimeconfig.ModeDocker,
			Docker: runtimeconfig.DockerConfig{
				Image:        "deonclaw-runner:latest",
				Workdir:      "/workspace",
				Network:      runtimeconfig.NetworkNone,
				ReadOnlyRoot: true,
				MemoryLimit:  "2g",
				CPUs:         "2",
				Env:          runtimeconfig.DockerEnv{Passthrough: env},
				Mounts: []runtimeconfig.MountSpec{
					{Source: ".", Target: "/workspace", Mode: runtimeconfig.MountModeReadWrite},
					{Source: "mysecondbrain", Target: "/memory/mysecondbrain", Mode: runtimeconfig.MountModeReadOnly},
				},
			},
		},
	}
}

func fixedSmokeClock() time.Time {
	return time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
}

func readSmokeArtifact(t *testing.T, dir string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", name, err)
	}
	return string(data)
}

func assertTranscriptJSONLValid(t *testing.T, content string) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineCount := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		lineCount++
		var decoded struct {
			Direction string          `json:"direction"`
			Method    string          `json:"method"`
			ID        json.RawMessage `json:"id"`
			Timestamp string          `json:"timestamp"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("transcript line is not valid JSON: %q error=%v", line, err)
		}
		if decoded.Direction == "" || decoded.Method == "" || decoded.Timestamp == "" || len(decoded.Payload) == 0 {
			t.Fatalf("transcript line missing fields: %#v", decoded)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan transcript error = %v", err)
	}
	if lineCount == 0 {
		t.Fatalf("transcript had no JSONL lines")
	}
}

func assertTranscriptRequestMethodCount(t *testing.T, content string, method string, want int) {
	t.Helper()
	scanner := bufio.NewScanner(strings.NewReader(content))
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var decoded struct {
			Direction string `json:"direction"`
			Method    string `json:"method"`
		}
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("transcript line is not valid JSON: %q error=%v", line, err)
		}
		if decoded.Direction == "request" && decoded.Method == method {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan transcript error = %v", err)
	}
	if count != want {
		t.Fatalf("request method %s count = %d, want %d in %q", method, count, want, content)
	}
}

func testToolPolicy() ToolPolicy {
	return ToolPolicy{
		MCPToolPolicy: MCPToolPolicy{
			AllowTestOnly:       true,
			MaxToolCalls:        1,
			AllowedServers:      []string{"fake-stdio"},
			AllowedTools:        []string{FakeEchoToolName},
			AllowedCapabilities: []string{mcpconfig.CapabilityRead},
		},
	}
}

func testDiscoveryPolicy() DiscoveryPolicy {
	return DiscoveryPolicy{
		MCPDiscoveryPolicy: MCPDiscoveryPolicy{
			AllowRealReadonly:    true,
			MaxToolCalls:         0,
			AllowedServers:       []string{"filesystem-readonly"},
			AllowedCapabilities:  []string{mcpconfig.CapabilityRead},
			RequireDockerForReal: true,
		},
	}
}

func helperArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1:]
		}
	}
	return nil
}

func installMCPFakeDocker(t *testing.T, mode string, stderr string) string {
	t.Helper()
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "docker.args")
	t.Setenv("DEONCLAW_MCP_FAKE_DOCKER_ARGS_PATH", argsPath)
	t.Setenv("DEONCLAW_MCP_FAKE_DOCKER_MODE", mode)
	t.Setenv("DEONCLAW_MCP_FAKE_DOCKER_STDERR", stderr)

	path := filepath.Join(tempDir, "docker")
	if stdruntime.GOOS == "windows" {
		path += ".bat"
	}
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("Abs(test binary) error = %v", err)
	}
	var content string
	if stdruntime.GOOS == "windows" {
		content = fmt.Sprintf("@echo off\r\nset DEONCLAW_MCP_FAKE_DOCKER_HELPER=1\r\n\"%s\" -test.run=TestMCPFakeDockerHelperProcess -- %%*\r\nexit /b %%ERRORLEVEL%%\r\n", testBinary)
	} else {
		content = "#!/bin/sh\nDEONCLAW_MCP_FAKE_DOCKER_HELPER=1 exec " + smokeShellQuote(testBinary) + " -test.run=TestMCPFakeDockerHelperProcess -- \"$@\"\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(fake docker) error = %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func runMCPFakeDockerHelper() int {
	if argsPath := os.Getenv("DEONCLAW_MCP_FAKE_DOCKER_ARGS_PATH"); argsPath != "" {
		content := strings.Join(helperArgs(), "\n")
		if content != "" {
			content += "\n"
		}
		_ = os.WriteFile(argsPath, []byte(content), 0o600)
	}
	if stderr := os.Getenv("DEONCLAW_MCP_FAKE_DOCKER_STDERR"); stderr != "" {
		_, _ = fmt.Fprint(os.Stderr, stderr)
	}
	switch os.Getenv("DEONCLAW_MCP_FAKE_DOCKER_MODE") {
	case "hang":
		time.Sleep(10 * time.Second)
		return 0
	default:
		if err := RunFakeServer(context.Background(), os.Stdin, os.Stdout, os.Stderr); err != nil {
			return 1
		}
		return 0
	}
}

func readDockerArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func stringSliceContainsSequence(values []string, want []string) bool {
	if len(want) == 0 {
		return true
	}
	if len(want) > len(values) {
		return false
	}
	for i := 0; i <= len(values)-len(want); i++ {
		matched := true
		for j := range want {
			if values[i+j] != want[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func indexOf(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

func assertSmokeFileEmptyOrMissing(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if len(data) != 0 {
		t.Fatalf("%s = %q, want empty or missing", path, string(data))
	}
}

func unsetEnvForSmokeTest(t *testing.T, name string) {
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

func smokeShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
