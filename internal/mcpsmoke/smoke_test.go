package mcpsmoke

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/mcpconfig"
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
