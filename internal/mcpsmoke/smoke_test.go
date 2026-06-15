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
	if !strings.Contains(output, `"id":2`) || !strings.Contains(output, `"tools":[]`) {
		t.Fatalf("stdout = %q, want empty tools/list response", output)
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
