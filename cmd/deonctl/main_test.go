package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/memory"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deonclaw ") {
		t.Fatalf("stdout = %q, want version output", stdout.String())
	}
}

func TestRunDoctorText(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deonctl:") || !strings.Contains(stdout.String(), "worker codex:") {
		t.Fatalf("stdout = %q, want doctor text", stdout.String())
	}
}

func TestRunDoctorJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"doctor", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
}

func TestRunWorkersDoctorWithWorkerFilterAndConfig(t *testing.T) {
	tempDir := t.TempDir()
	writeCLIPathFakeExecutable(t, tempDir, "fake-codex", 0)
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	workersConfigPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(workersConfigPath, []byte("workers:\n  codex:\n    command: fake-codex\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(workers config) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"workers", "doctor", "--worker", "codex", "--workers-config", workersConfigPath, "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	var decoded struct {
		Workers []struct {
			Name              string `json:"name"`
			ConfiguredCommand string `json:"configured_command"`
			Available         bool   `json:"available"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if len(decoded.Workers) != 1 || decoded.Workers[0].Name != "codex" || decoded.Workers[0].ConfiguredCommand != "fake-codex" || !decoded.Workers[0].Available {
		t.Fatalf("workers = %#v, want fake codex available", decoded.Workers)
	}
}

func TestRunWorkersDoctorJSONIncludesProviderModel(t *testing.T) {
	tempDir := t.TempDir()
	writeCLIPathFakeExecutable(t, tempDir, "fake-opencode", 0)
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	workersConfigPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(workersConfigPath, []byte(`workers:
  opencode:
    command: fake-opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
`), 0o600); err != nil {
		t.Fatalf("WriteFile(workers config) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"workers", "doctor", "--worker", "opencode", "--workers-config", workersConfigPath, "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	var decoded struct {
		Workers []struct {
			Name     string `json:"name"`
			Provider string `json:"provider"`
			Model    string `json:"model"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if len(decoded.Workers) != 1 || decoded.Workers[0].Name != "opencode" || decoded.Workers[0].Provider != "z_ai_glm" || decoded.Workers[0].Model != "glm-5.1" {
		t.Fatalf("workers = %#v, want opencode provider/model", decoded.Workers)
	}
}

func TestRunWorkersDoctorJSONReportsMissingRequiredEnv(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	workersConfigPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: opencode
    env:
      ZAI_API_KEY: required
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"workers", "doctor", "--worker", "opencode", "--workers-config", workersConfigPath, "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	var decoded struct {
		Workers []struct {
			EnvRequiredOK   bool `json:"env_required_ok"`
			EnvRequirements []struct {
				Name        string `json:"name"`
				Requirement string `json:"requirement"`
				State       string `json:"state"`
			} `json:"env_requirements"`
		} `json:"workers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if len(decoded.Workers) != 1 || decoded.Workers[0].EnvRequiredOK || len(decoded.Workers[0].EnvRequirements) != 1 || decoded.Workers[0].EnvRequirements[0].State != "missing" {
		t.Fatalf("workers = %#v, want missing required env", decoded.Workers)
	}
}

func TestRunWorkersDoctorJSONReportsSetMaskedRequiredEnv(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	workersConfigPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: opencode
    env:
      ZAI_API_KEY: required
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"workers", "doctor", "--worker", "opencode", "--workers-config", workersConfigPath, "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, `"env_required_ok": true`) || !strings.Contains(text, `"state": "set_masked"`) {
		t.Fatalf("stdout = %q, want set_masked env requirement", text)
	}
	if strings.Contains(text, "zai-real-secret") {
		t.Fatalf("stdout leaked secret: %q", text)
	}
}

func TestRunWorkersDoctorJSONListsModelProfilesWhenRequested(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	workersConfigPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
      - general
`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"workers", "doctor", "--worker", "opencode", "--workers-config", workersConfigPath, "--profiles", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	var decoded struct {
		ModelProfiles []struct {
			Name          string   `json:"name"`
			Worker        string   `json:"worker"`
			Provider      string   `json:"provider"`
			Model         string   `json:"model"`
			ModelArg      string   `json:"model_arg"`
			Tags          []string `json:"tags"`
			EnvRequiredOK bool     `json:"env_required_ok"`
		} `json:"model_profiles"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if len(decoded.ModelProfiles) != 1 || decoded.ModelProfiles[0].Name != "opencode-zai-glm-5-1" || decoded.ModelProfiles[0].Worker != "opencode" || decoded.ModelProfiles[0].Provider != "z-ai" || decoded.ModelProfiles[0].Model != "glm-5.1" || decoded.ModelProfiles[0].ModelArg != "z-ai/glm-5.1" || decoded.ModelProfiles[0].EnvRequiredOK {
		t.Fatalf("model_profiles = %#v, want missing opencode z-ai profile", decoded.ModelProfiles)
	}
}

func TestRunConfigEnvMasksSecrets(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-real-secret")
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"config", "env"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	text := stdout.String()
	if !strings.Contains(text, "OPENAI_API_KEY=set(masked)") {
		t.Fatalf("stdout = %q, want masked key", text)
	}
	if !strings.Contains(text, "ZAI_API_KEY=set(masked)") {
		t.Fatalf("stdout = %q, want masked ZAI_API_KEY", text)
	}
	if strings.Contains(text, "sk-real-secret") || strings.Contains(text, "zai-real-secret") {
		t.Fatalf("stdout leaked secret: %q", text)
	}
}

func TestRunTaskValidate(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"task", "validate", taskPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "task worker-mismatch-001: valid") {
		t.Fatalf("stdout = %q, want task valid output", stdout.String())
	}
}

func TestRunDomainsValidate(t *testing.T) {
	configPath := writeDomainsConfigFile(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "domains config valid: 2 domains") {
		t.Fatalf("stdout = %q, want domains valid output", stdout.String())
	}
}

func TestRunDomainsValidateRejectsBadArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --config") {
		t.Fatalf("stderr = %q, want missing --config", stderr.String())
	}
}

func TestRunDomainsList(t *testing.T) {
	configPath := writeDomainsConfigFile(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"name",
		"type",
		"root",
		"default",
		"isolated",
		"default_agent",
		"general",
		"canonical_memory",
		"/vault/mysecondbrain",
		"escalasoft",
		"isolated_domain",
		"/domains/escalasoft_brain",
		"escalasoft-agent",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunRuntimeValidate(t *testing.T) {
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "runtime config valid: mode=docker") {
		t.Fatalf("stdout = %q, want runtime valid", stdout.String())
	}
}

func TestRunExampleSafeRuntimeValidate(t *testing.T) {
	configPath := examplePath(t, "configs", "examples", "runtime.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "runtime config valid: mode=docker") {
		t.Fatalf("stdout = %q, want runtime valid", stdout.String())
	}
	if strings.Contains(stdout.String(), "ZAI_API_KEY") || strings.Contains(stdout.String(), "OPENAI_API_KEY") {
		t.Fatalf("stdout = %q, safe runtime should not require provider passthrough", stdout.String())
	}
	if strings.Contains(stdout.String(), "warning:") {
		t.Fatalf("stdout = %q, safe runtime should not warn", stdout.String())
	}
}

func TestRunExampleOpenCodeZAISmokeRuntimeValidateWarnsForNetworkDefault(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	configPath := examplePath(t, "configs", "examples", "runtime-opencode-zai-smoke.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "runtime config valid: mode=docker") {
		t.Fatalf("stdout = %q, want runtime valid", output)
	}
	if !strings.Contains(output, "warning: runtime.docker.network default allows container network access") {
		t.Fatalf("stdout = %q, want network default warning", output)
	}
	if strings.Contains(output, "zai-real-secret") {
		t.Fatalf("stdout leaked secret: %q", output)
	}
}

func TestRunRuntimeValidateRejectsDangerousMount(t *testing.T) {
	configPath := writeCLIRuntimeConfig(t, `runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    mounts:
      - source: /
        target: /host
        mode: ro
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `runtime.docker.mounts[0].source "/" is not allowed`) {
		t.Fatalf("stderr = %q, want dangerous mount rejection", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunMCPValidateExample(t *testing.T) {
	configPath := examplePath(t, "configs", "examples", "mcp.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "mcp config valid: servers=2") {
		t.Fatalf("stdout = %q, want valid mcp config", stdout.String())
	}
}

func TestRunMCPValidateRejectsMissingCommand(t *testing.T) {
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    bad-server:
      command: ""
      enabled: false
      trust: local
      capabilities:
        - read
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "command is required") {
		t.Fatalf("stderr = %q, want missing command", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunMCPValidateRejectsInlineEnvWithoutLeakingValue(t *testing.T) {
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    github-readonly:
      command: placeholder
      enabled: false
      trust: external
      capabilities:
        - read
      env:
        passthrough:
          - GITHUB_TOKEN=super-secret-value
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "inline env values are forbidden") {
		t.Fatalf("stderr = %q, want inline env rejection", stderr.String())
	}
	if strings.Contains(stderr.String(), "super-secret-value") {
		t.Fatalf("stderr leaked secret: %q", stderr.String())
	}
}

func TestRunMCPValidateRejectsUnknownCapability(t *testing.T) {
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    github-readonly:
      command: placeholder
      enabled: false
      trust: external
      capabilities:
        - network
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `"network" is not supported`) {
		t.Fatalf("stderr = %q, want unknown capability", stderr.String())
	}
}

func TestRunMCPValidateWriteExecDisabledWarns(t *testing.T) {
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    risky:
      command: placeholder
      enabled: false
      trust: local
      capabilities:
        - write
        - exec
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `capability "write" is dangerous`) || !strings.Contains(stdout.String(), `capability "exec" is dangerous`) {
		t.Fatalf("stdout = %q, want write/exec warnings", stdout.String())
	}
}

func TestRunMCPListShowsServers(t *testing.T) {
	configPath := examplePath(t, "configs", "examples", "mcp.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"filesystem-readonly", "github-readonly", "external", "read"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunMCPPlanJSONNameOnly(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "super-secret-value")
	configPath := examplePath(t, "configs", "examples", "mcp.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "plan", "--config", configPath, "--server", "github-readonly", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
	var decoded struct {
		Server   string   `json:"server"`
		Command  string   `json:"command"`
		EnvNames []string `json:"env_names"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.Server != "github-readonly" || decoded.Command != "placeholder" {
		t.Fatalf("decoded = %#v, want github-readonly plan", decoded)
	}
	if len(decoded.EnvNames) != 1 || decoded.EnvNames[0] != "GITHUB_TOKEN" {
		t.Fatalf("env_names = %#v, want GITHUB_TOKEN", decoded.EnvNames)
	}
	if strings.Contains(stdout.String(), "super-secret-value") {
		t.Fatalf("stdout leaked env value: %q", stdout.String())
	}
}

func TestRunMCPDoctorTextReportsEnvAvailabilityAndRisk(t *testing.T) {
	tempDir := t.TempDir()
	writeCLIPathFakeExecutable(t, tempDir, "fake-mcp", 0)
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MCP_TOKEN", "super-secret-value")
	unsetEnvForTest(t, "MISSING_TOKEN")
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    filesystem-readonly:
      command: fake-mcp
      enabled: false
      trust: local
      capabilities:
        - read
      env:
        passthrough:
          - MCP_TOKEN
    github-readonly:
      command: missing-mcp-command
      enabled: false
      trust: external
      capabilities:
        - read
      env:
        passthrough:
          - MISSING_TOKEN
    risky-disabled:
      command: missing-risky-command
      enabled: false
      trust: local
      capabilities:
        - write
        - exec
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "doctor", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"mcp doctor:",
		"filesystem-readonly",
		"command_available=true",
		"risk=low",
		"MCP_TOKEN=set_masked",
		"github-readonly",
		"command_available=false",
		"risk=medium",
		"MISSING_TOKEN=missing",
		"risky-disabled",
		"risk=high",
		"warning:",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if strings.Contains(output+stderr.String(), "super-secret-value") {
		t.Fatalf("output leaked env value: stdout=%q stderr=%q", output, stderr.String())
	}
}

func TestRunMCPDoctorJSONValid(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    filesystem-readonly:
      command: placeholder
      enabled: false
      trust: local
      capabilities:
        - read
      env:
        passthrough:
          - MCP_TOKEN
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "doctor", "--config", configPath, "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
	var decoded struct {
		Servers []struct {
			Name             string `json:"name"`
			CommandAvailable bool   `json:"command_available"`
			RiskLevel        string `json:"risk_level"`
			EnvRequirements  []struct {
				Name  string `json:"name"`
				State string `json:"state"`
			} `json:"env_requirements"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if len(decoded.Servers) != 1 || decoded.Servers[0].Name != "filesystem-readonly" || decoded.Servers[0].RiskLevel != "low" {
		t.Fatalf("decoded = %#v, want filesystem-readonly low risk", decoded)
	}
	if len(decoded.Servers[0].EnvRequirements) != 1 || decoded.Servers[0].EnvRequirements[0].State != "set_masked" {
		t.Fatalf("env = %#v, want set_masked", decoded.Servers[0].EnvRequirements)
	}
	if strings.Contains(stdout.String(), "super-secret-value") {
		t.Fatalf("stdout leaked env value: %q", stdout.String())
	}
}

func TestRunMCPRiskJSONAggregatesCounts(t *testing.T) {
	unsetEnvForTest(t, "MISSING_TOKEN")
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    local-read:
      command: placeholder
      enabled: false
      trust: local
      capabilities:
        - read
    external-read:
      command: placeholder
      enabled: true
      trust: external
      capabilities:
        - read
      env:
        passthrough:
          - MISSING_TOKEN
    high-write-exec:
      command: placeholder
      enabled: false
      trust: local
      capabilities:
        - write
        - exec
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "risk", "--config", configPath, "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
	var decoded struct {
		TotalServers           int `json:"total_servers"`
		EnabledServers         int `json:"enabled_servers"`
		DisabledServers        int `json:"disabled_servers"`
		ExternalServers        int `json:"external_servers"`
		WriteCapabilityServers int `json:"write_capability_servers"`
		ExecCapabilityServers  int `json:"exec_capability_servers"`
		MissingEnvCount        int `json:"missing_env_count"`
		HighRiskCount          int `json:"high_risk_count"`
		MediumRiskCount        int `json:"medium_risk_count"`
		LowRiskCount           int `json:"low_risk_count"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if decoded.TotalServers != 3 || decoded.EnabledServers != 1 || decoded.DisabledServers != 2 ||
		decoded.ExternalServers != 1 || decoded.WriteCapabilityServers != 1 || decoded.ExecCapabilityServers != 1 ||
		decoded.MissingEnvCount != 1 || decoded.HighRiskCount != 1 || decoded.MediumRiskCount != 1 || decoded.LowRiskCount != 1 {
		t.Fatalf("decoded = %#v, want aggregate counts", decoded)
	}
}

func TestRunMCPDockerPlanTextDoesNotExecuteServerAndHidesEnvValue(t *testing.T) {
	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "server-executed")
	writeCLIFakeCommand(t, tempDir, "fake-mcp-server", cliFakeCommandBehavior{markerPath: markerPath})
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MCP_TOKEN", "super-secret-value")
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    filesystem-readonly:
      command: fake-mcp-server
      args:
        - --root
        - .
      enabled: false
      trust: local
      capabilities:
        - read
      env:
        passthrough:
          - MCP_TOKEN
`)
	runtimePath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "docker-plan", "--config", configPath, "--server", "filesystem-readonly", "--runtime-config", runtimePath, "--workspace", "."}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"mcp docker-plan: plan only, not executed", "docker run", "deonclaw-runner:latest", "fake-mcp-server", "--root", "-e MCP_TOKEN"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if strings.Contains(output+stderr.String(), "super-secret-value") {
		t.Fatalf("output leaked env value: stdout=%q stderr=%q", output, stderr.String())
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("fake MCP server was executed or stat failed: %v", err)
	}
}

func TestRunMCPDockerPlanJSONValid(t *testing.T) {
	configPath := writeCLIMCPConfig(t, `mcp:
  servers:
    filesystem-readonly:
      command: mcp-server
      enabled: false
      trust: local
      capabilities:
        - read
`)
	runtimePath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "docker-plan", "--config", configPath, "--server", "filesystem-readonly", "--runtime-config", runtimePath, "--workspace", ".", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
	var decoded struct {
		Server   string   `json:"server"`
		PlanOnly bool     `json:"plan_only"`
		Command  []string `json:"command"`
		Display  string   `json:"display"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if decoded.Server != "filesystem-readonly" || !decoded.PlanOnly || len(decoded.Command) == 0 || decoded.Command[0] != "docker" || !strings.Contains(decoded.Display, "mcp-server") {
		t.Fatalf("decoded = %#v, want docker MCP plan", decoded)
	}
}

func TestRunMCPFakeServerRespondsInitialize(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"exit"}`,
		"",
	}, "\n")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runMCPFakeServer(strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runMCPFakeServer() exit code = %d, stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"id":1`) || !strings.Contains(stdout.String(), `"serverInfo"`) {
		t.Fatalf("stdout = %q, want initialize response", stdout.String())
	}
}

func TestRunMCPSmokeLocalGeneratesArtifacts(t *testing.T) {
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER", "1")
	t.Setenv("MCP_TOKEN", "super-secret-value")
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", true, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "smoke", "--config", configPath, "--server", "fake-stdio", "--artifacts-dir", artifactsDir, "--timeout-seconds", "3"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fake/test smoke only") || !strings.Contains(stdout.String(), "status: succeeded") {
		t.Fatalf("stdout = %q, want fake smoke summary", stdout.String())
	}
	for _, name := range []string{"mcp-smoke-summary.md", "mcp-transcript.jsonl", "mcp-stdout.log", "mcp-stderr.log", "mcp-smoke-result.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-transcript.jsonl"))
	if !strings.Contains(transcript, `"tools/list"`) {
		t.Fatalf("transcript = %q, want tools/list", transcript)
	}
	for _, name := range []string{"mcp-smoke-summary.md", "mcp-transcript.jsonl", "mcp-stdout.log", "mcp-stderr.log", "mcp-smoke-result.json"} {
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
}

func TestRunMCPSmokeRejectsServerWithoutTestOnly(t *testing.T) {
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "smoke", "--config", configPath, "--server", "fake-stdio", "--artifacts-dir", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "test_only=true") {
		t.Fatalf("stderr = %q, want test_only rejection", stderr.String())
	}
}

func TestRunMCPSmokeRejectsWriteExecCapability(t *testing.T) {
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", true, []string{"write"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "smoke", "--config", configPath, "--server", "fake-stdio", "--artifacts-dir", t.TempDir()}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "write or exec") {
		t.Fatalf("stderr = %q, want write/exec rejection", stderr.String())
	}
}

func TestRunMCPSmokeTimeoutFailsControlled(t *testing.T) {
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER", "1")
	configPath := writeCLIMCPFakeConfig(t, nil, "hang", true, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "smoke", "--config", configPath, "--server", "fake-stdio", "--artifacts-dir", t.TempDir(), "--timeout-seconds", "1"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("stderr = %q, want timeout", stderr.String())
	}
}

func TestRunMCPSmokeDockerWithFakeDockerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installCLIMCPFakeDocker(t, "fake", "docker fake stderr\n")
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", true, []string{"read"})
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--artifacts-dir", artifactsDir,
		"--timeout-seconds", "3",
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fake/test smoke only") || !strings.Contains(stdout.String(), "runtime: docker") || !strings.Contains(stdout.String(), "status: succeeded") {
		t.Fatalf("stdout = %q, want docker fake smoke summary", stdout.String())
	}
	for _, name := range []string{"mcp-smoke-summary.md", "mcp-transcript.jsonl", "mcp-stdout.log", "mcp-stderr.log", "mcp-smoke-result.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-transcript.jsonl"))
	if !strings.Contains(transcript, `"tools/list"`) {
		t.Fatalf("transcript = %q, want tools/list", transcript)
	}
	assertCLIFileContains(t, filepath.Join(artifactsDir, "mcp-stdout.log"), `"serverInfo"`)
	assertCLIFileContains(t, filepath.Join(artifactsDir, "mcp-stderr.log"), "docker fake stderr")

	args := readCLIDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOfString(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestCLIMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want fake server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}
	for _, content := range []string{stdout.String(), stderr.String(), joinedArgs} {
		if strings.Contains(content, "super-secret-value") {
			t.Fatalf("secret leaked in output/args: %q", content)
		}
	}
	for _, name := range []string{"mcp-smoke-summary.md", "mcp-transcript.jsonl", "mcp-stdout.log", "mcp-stderr.log", "mcp-smoke-result.json"} {
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
}

func TestRunMCPSmokeDockerMissingEnvFailsBeforeDocker(t *testing.T) {
	unsetEnvForTest(t, "MCP_TOKEN")
	argsPath := installCLIMCPFakeDocker(t, "fake", "")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", true, []string{"read"})
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--artifacts-dir", t.TempDir(),
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "MCP_TOKEN") {
		t.Fatalf("stderr = %q, want missing env name", stderr.String())
	}
	if strings.Contains(stderr.String(), "super-secret-value") {
		t.Fatalf("stderr leaked secret: %q", stderr.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunMCPSmokeDockerRejectsServerWithoutTestOnly(t *testing.T) {
	argsPath := installCLIMCPFakeDocker(t, "fake", "")
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--artifacts-dir", t.TempDir(),
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "test_only=true") {
		t.Fatalf("stderr = %q, want test_only rejection", stderr.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunMCPSmokeDockerRejectsWriteExecCapability(t *testing.T) {
	argsPath := installCLIMCPFakeDocker(t, "fake", "")
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", true, []string{"exec"})
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--artifacts-dir", t.TempDir(),
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "write or exec") {
		t.Fatalf("stderr = %q, want write/exec rejection", stderr.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunMCPSmokeDockerTimeoutFailsControlled(t *testing.T) {
	_ = installCLIMCPFakeDocker(t, "hang", "")
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", true, []string{"read"})
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--artifacts-dir", t.TempDir(),
		"--timeout-seconds", "1",
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "timed out") {
		t.Fatalf("stderr = %q, want timeout", stderr.String())
	}
}

func TestRunMCPToolSmokeLocalGeneratesArtifacts(t *testing.T) {
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER", "1")
	t.Setenv("MCP_TOKEN", "super-secret-value")
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", true, []string{"read"})
	policyPath := writeCLIMCPToolPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "tool-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello"}`,
		"--artifacts-dir", artifactsDir,
		"--timeout-seconds", "3",
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fake/test tool smoke only") || !strings.Contains(stdout.String(), "status: succeeded") {
		t.Fatalf("stdout = %q, want tool smoke summary", stdout.String())
	}
	for _, name := range []string{"mcp-tool-smoke-summary.md", "mcp-tool-transcript.jsonl", "mcp-tool-stdout.log", "mcp-tool-stderr.log", "mcp-tool-result.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-tool-transcript.jsonl"))
	assertCLITranscriptRequestMethodCount(t, transcript, "tools/call", 1)
	if !strings.Contains(transcript, `"hello"`) {
		t.Fatalf("transcript = %q, want echo payload", transcript)
	}
	for _, name := range []string{"mcp-tool-smoke-summary.md", "mcp-tool-transcript.jsonl", "mcp-tool-stdout.log", "mcp-tool-stderr.log", "mcp-tool-result.json"} {
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
}

func TestRunMCPToolSmokeDockerWithFakeDockerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installCLIMCPFakeDocker(t, "fake", "docker fake stderr\n")
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", true, []string{"read"})
	policyPath := writeCLIMCPToolPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"})
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "tool-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello docker"}`,
		"--artifacts-dir", artifactsDir,
		"--timeout-seconds", "3",
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "fake/test tool smoke only") || !strings.Contains(stdout.String(), "runtime: docker") {
		t.Fatalf("stdout = %q, want docker tool smoke summary", stdout.String())
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-tool-transcript.jsonl"))
	assertCLITranscriptRequestMethodCount(t, transcript, "tools/call", 1)
	assertCLIFileContains(t, filepath.Join(artifactsDir, "mcp-tool-stderr.log"), "docker fake stderr")

	args := readCLIDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOfString(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestCLIMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want fake server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("docker args unsafe or leaked secret: %#v", args)
	}
	for _, name := range []string{"mcp-tool-smoke-summary.md", "mcp-tool-transcript.jsonl", "mcp-tool-stdout.log", "mcp-tool-stderr.log", "mcp-tool-result.json"} {
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
}

func TestRunMCPToolSmokeRejectsToolNotAllowlisted(t *testing.T) {
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER", "1")
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", true, []string{"read"})
	policyPath := writeCLIMCPToolPolicy(t, []string{"fake-stdio"}, []string{"other.tool"}, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "tool-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello"}`,
		"--artifacts-dir", t.TempDir(),
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not allowlisted") {
		t.Fatalf("stderr = %q, want allowlist rejection", stderr.String())
	}
}

func TestRunMCPToolSmokeRejectsServerNotAllowlisted(t *testing.T) {
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER", "1")
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", true, []string{"read"})
	policyPath := writeCLIMCPToolPolicy(t, []string{"other-server"}, []string{"deonclaw.fake.echo"}, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "tool-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello"}`,
		"--artifacts-dir", t.TempDir(),
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not allowlisted") {
		t.Fatalf("stderr = %q, want server allowlist rejection", stderr.String())
	}
}

func TestRunMCPToolSmokeRejectsInvalidArguments(t *testing.T) {
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER", "1")
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", true, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "tool-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":`,
		"--artifacts-dir", t.TempDir(),
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "arguments") {
		t.Fatalf("stderr = %q, want invalid arguments rejection", stderr.String())
	}
}

func TestRunMCPDiscoverRequiresPolicy(t *testing.T) {
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"mcp", "discover", "--config", configPath, "--server", "fake-stdio", "--artifacts-dir", t.TempDir()}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --policy") {
		t.Fatalf("stderr = %q, want missing policy rejection", stderr.String())
	}
}

func TestRunMCPDiscoverDockerWithFakeDockerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installCLIMCPFakeDocker(t, "fake", "docker fake stderr\n")
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", false, []string{"read"})
	policyPath := writeCLIMCPDiscoveryPolicy(t, []string{"fake-stdio"}, []string{"read"}, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "discover",
		"--config", configPath,
		"--server", "fake-stdio",
		"--artifacts-dir", artifactsDir,
		"--timeout-seconds", "3",
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "read-only tools/list only") || !strings.Contains(stdout.String(), "runtime: docker") || !strings.Contains(stdout.String(), "tool_count: 1") {
		t.Fatalf("stdout = %q, want discovery summary", stdout.String())
	}
	for _, name := range []string{"mcp-discovery-summary.md", "mcp-discovery-transcript.jsonl", "mcp-discovery-stdout.log", "mcp-discovery-stderr.log", "mcp-discovery-result.json", "mcp-tools-list.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-discovery-transcript.jsonl"))
	assertCLITranscriptRequestMethodCount(t, transcript, "initialize", 1)
	assertCLITranscriptRequestMethodCount(t, transcript, "tools/list", 1)
	assertCLITranscriptRequestMethodCount(t, transcript, "shutdown", 1)
	assertCLITranscriptRequestMethodCount(t, transcript, "exit", 1)
	assertCLITranscriptRequestMethodCount(t, transcript, "tools/call", 0)
	var toolsList struct {
		ToolCount int      `json:"tool_count"`
		ToolNames []string `json:"tool_names"`
	}
	if err := json.Unmarshal(mustReadCLIFile(t, filepath.Join(artifactsDir, "mcp-tools-list.json")), &toolsList); err != nil {
		t.Fatalf("mcp-tools-list.json invalid: %v", err)
	}
	if toolsList.ToolCount != 1 || !stringSliceContainsSequence(toolsList.ToolNames, []string{"deonclaw.fake.echo"}) {
		t.Fatalf("tools list = %#v, want fake echo metadata", toolsList)
	}
	args := readCLIDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOfString(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestCLIMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want fake server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("docker args unsafe or leaked secret: %#v", args)
	}
	for _, name := range []string{"mcp-discovery-summary.md", "mcp-discovery-transcript.jsonl", "mcp-discovery-stdout.log", "mcp-discovery-stderr.log", "mcp-discovery-result.json", "mcp-tools-list.json"} {
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
}

func TestRunMCPCallSmokeRequiresPolicy(t *testing.T) {
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "call-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello"}`,
		"--artifacts-dir", t.TempDir(),
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --policy") {
		t.Fatalf("stderr = %q, want missing policy rejection", stderr.String())
	}
}

func TestRunMCPCallSmokeDockerWithFakeDockerGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installCLIMCPFakeDocker(t, "fake", "docker fake stderr super-secret-value\n")
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", false, []string{"read"})
	policyPath := writeCLIMCPCallPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"}, 65536, 1048576, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "call-smoke",
		"--config", configPath,
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello cli"}`,
		"--artifacts-dir", artifactsDir,
		"--timeout-seconds", "3",
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "read-only policy-gated tool call") || !strings.Contains(stdout.String(), "tool_calls: 1") || !strings.Contains(stdout.String(), "response_truncated: false") {
		t.Fatalf("stdout = %q, want call smoke summary", stdout.String())
	}
	for _, name := range []string{"mcp-call-smoke-summary.md", "mcp-call-transcript.jsonl", "mcp-call-result.json", "mcp-call-stdout.log", "mcp-call-stderr.log", "mcp-call-response.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-call-transcript.jsonl"))
	assertCLITranscriptRequestMethodCount(t, transcript, "tools/call", 1)
	var response struct {
		Tool              string          `json:"tool"`
		Response          json.RawMessage `json:"response"`
		ResponseTruncated bool            `json:"response_truncated"`
	}
	if err := json.Unmarshal(mustReadCLIFile(t, filepath.Join(artifactsDir, "mcp-call-response.json")), &response); err != nil {
		t.Fatalf("mcp-call-response.json invalid: %v", err)
	}
	if response.Tool != "deonclaw.fake.echo" || response.ResponseTruncated || !strings.Contains(string(response.Response), "hello cli") {
		t.Fatalf("response = %#v, want fake echo response", response)
	}
	args := readCLIDockerArgs(t, argsPath)
	if !stringSliceContainsSequence(args, []string{"-e", "MCP_TOKEN"}) {
		t.Fatalf("docker args = %#v, want MCP_TOKEN passthrough by name", args)
	}
	imageIndex := indexOfString(args, "deonclaw-runner:latest")
	if imageIndex < 0 {
		t.Fatalf("docker args = %#v, want image", args)
	}
	if !stringSliceContainsSequence(args[imageIndex+1:], []string{os.Args[0], "-test.run=TestCLIMCPFakeServerHelperProcess", "--", "fake"}) {
		t.Fatalf("docker args tail = %#v, want fake server command after image", args[imageIndex+1:])
	}
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("docker args unsafe or leaked secret: %#v", args)
	}
	for _, name := range []string{"mcp-call-smoke-summary.md", "mcp-call-transcript.jsonl", "mcp-call-result.json", "mcp-call-stdout.log", "mcp-call-stderr.log", "mcp-call-response.json"} {
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
}

func TestRunMCPProposalWorkflowNewLintPreflightApprove(t *testing.T) {
	tempDir := t.TempDir()
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	policyPath := writeCLIMCPCallPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"}, 65536, 1048576, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	proposalPath := filepath.Join(tempDir, "proposal.json")
	preflightPath := filepath.Join(tempDir, "preflight.json")
	approvalPath := filepath.Join(tempDir, "approval.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "proposal", "new",
		"--server", "fake-stdio",
		"--tool", "deonclaw.fake.echo",
		"--arguments", `{"text":"hello"}`,
		"--reason", "Manual read-only call.",
		"--policy", policyPath,
		"--config", configPath,
		"--runtime", "docker",
		"--runtime-config", runtimeConfigPath,
		"--workspace", ".",
		"--output", proposalPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("proposal new exit code = %d, stderr = %q", code, stderr.String())
	}
	proposal := readMCPProposalFile(t, proposalPath)
	if proposal.Status != mcpapproval.ProposalStatusProposed ||
		proposal.ArgumentsSHA256 == "" ||
		proposal.ConfigSHA256 == "" ||
		proposal.RuntimeConfigSHA256 == "" {
		t.Fatalf("proposal = %#v, want proposed with file hashes", proposal)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "proposal", "inspect",
		"--proposal", proposalPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("proposal inspect exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "arguments_sha256:") || strings.Contains(stdout.String(), `"text"`) || strings.Contains(stdout.String(), "hello") {
		t.Fatalf("proposal inspect stdout = %q, want hashes without raw arguments", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "proposal", "lint",
		"--proposal", proposalPath,
		"--config", configPath,
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("proposal lint exit code = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: passed") {
		t.Fatalf("lint stdout = %q, want passed", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "proposal", "preflight",
		"--proposal", proposalPath,
		"--config", configPath,
		"--policy", policyPath,
		"--output", preflightPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("proposal preflight exit code = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	preflight := readMCPPreflightFile(t, preflightPath)
	if preflight.Status != mcpapproval.PreflightStatusPassed ||
		!preflight.Checks.PolicyLoaded ||
		!preflight.Checks.ServerAllowlisted ||
		!preflight.Checks.ToolAllowlisted ||
		!preflight.Checks.ReadOnlyCapability ||
		!preflight.Checks.DockerRequired ||
		!preflight.Checks.EnvAvailable ||
		!preflight.Checks.ArgumentsWithinLimit ||
		!preflight.Checks.NoWriteExec {
		t.Fatalf("preflight = %#v, want all checks passed", preflight)
	}
	if preflight.ConfigSHA256 == "" || preflight.RuntimeConfigSHA256 == "" {
		t.Fatalf("preflight = %#v, want config/runtime hashes", preflight)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", policyPath,
		"--decision", "approved",
		"--reason", "Approved after preflight.",
		"--output", approvalPath,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("approve without confirm exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --confirm-read-only") {
		t.Fatalf("stderr = %q, want confirm-read-only", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", policyPath,
		"--decision", "approved",
		"--reason", "Approved after preflight.",
		"--output", approvalPath,
		"--confirm-read-only",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("proposal approve exit code = %d, stderr = %q", code, stderr.String())
	}
	approval := readMCPApprovalFile(t, approvalPath)
	if approval.Decision != mcpapproval.ApprovalDecisionApproved ||
		approval.ArgumentsSHA256 != proposal.ArgumentsSHA256 ||
		approval.PolicySHA256 == "" ||
		approval.ConfigSHA256 == "" ||
		approval.RuntimeConfigSHA256 == "" ||
		!approval.ConfirmReadOnly {
		t.Fatalf("approval = %#v, want approved with hashes", approval)
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "approval", "inspect",
		"--approval", approvalPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("approval inspect exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "policy_sha256:") ||
		!strings.Contains(stdout.String(), "config_sha256:") ||
		!strings.Contains(stdout.String(), "runtime_config_sha256:") {
		t.Fatalf("approval inspect stdout = %q, want hashes", stdout.String())
	}
}

func TestRunMCPProposalLintRejectsInvalidArguments(t *testing.T) {
	tempDir := t.TempDir()
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	policyPath := writeCLIMCPCallPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"}, 65536, 1048576, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	proposal := newCLIMCPProposal(t, "fake-stdio", []byte(`{"text":"hello"}`), policyPath, runtimeConfigPath)
	proposal.Arguments = json.RawMessage(`"not-object"`)
	proposal.ArgumentsSHA256 = mcpapproval.ArgumentsSHA256(proposal.Arguments)
	proposalPath := writeRawJSONFile(t, tempDir, "proposal-invalid-arguments.json", proposal)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "proposal", "lint",
		"--proposal", proposalPath,
		"--config", configPath,
		"--policy", policyPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("proposal lint exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "arguments must be a valid JSON object") {
		t.Fatalf("stdout = %q, want arguments failure", stdout.String())
	}
}

func TestRunMCPProposalPreflightRejectsWriteExecAndMissingEnv(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := writeCLIMCPCallPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"}, 65536, 1048576, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	proposalPath := writeMCPProposalFile(t, tempDir, newCLIMCPProposal(t, "fake-stdio", []byte(`{"text":"hello"}`), policyPath, runtimeConfigPath))

	writeExecConfig := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"write"})
	writeExecPreflightPath := filepath.Join(tempDir, "preflight-write.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"mcp", "proposal", "preflight",
		"--proposal", proposalPath,
		"--config", writeExecConfig,
		"--policy", policyPath,
		"--output", writeExecPreflightPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("write preflight exit code = %d, want 1", code)
	}
	writePreflight := readMCPPreflightFile(t, writeExecPreflightPath)
	if writePreflight.Checks.NoWriteExec || writePreflight.Checks.ReadOnlyCapability {
		t.Fatalf("write preflight = %#v, want no_write_exec/read_only failure", writePreflight)
	}

	unsetEnvForTest(t, "MCP_TOKEN")
	missingEnvConfig := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", false, []string{"read"})
	missingEnvPreflightPath := filepath.Join(tempDir, "preflight-env.json")
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"mcp", "proposal", "preflight",
		"--proposal", proposalPath,
		"--config", missingEnvConfig,
		"--policy", policyPath,
		"--output", missingEnvPreflightPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("missing env preflight exit code = %d, want 1", code)
	}
	envPreflight := readMCPPreflightFile(t, missingEnvPreflightPath)
	if envPreflight.Checks.EnvAvailable || !strings.Contains(strings.Join(envPreflight.Failures, "\n"), "MCP_TOKEN") {
		t.Fatalf("env preflight = %#v, want missing MCP_TOKEN", envPreflight)
	}
}

func TestRunMCPProposalExecuteRejectsMissingRejectedAndHashChanges(t *testing.T) {
	tempDir := t.TempDir()
	configPath := writeCLIMCPFakeConfig(t, nil, "fake", false, []string{"read"})
	policyPath := writeCLIMCPCallPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"}, 65536, 1048576, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	proposal := newCLIMCPProposalWithConfig(t, "fake-stdio", []byte(`{"text":"hello"}`), policyPath, configPath, runtimeConfigPath)
	proposalPath := writeMCPProposalFile(t, tempDir, proposal)
	approval := newCLIMCPApproval(t, proposal, policyPath, mcpapproval.ApprovalDecisionApproved)
	approvalPath := writeMCPApprovalFile(t, tempDir, approval, "approval.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"mcp", "proposal", "execute",
		"--proposal", proposalPath,
		"--config", configPath,
		"--policy", policyPath,
		"--artifacts-dir", filepath.Join(tempDir, "artifacts-missing-approval"),
		"--confirm-execute",
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("execute missing approval exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --approval") {
		t.Fatalf("stderr = %q, want missing approval", stderr.String())
	}

	rejectedApproval := approval
	rejectedApproval.Decision = mcpapproval.ApprovalDecisionRejected
	rejectedPath := writeMCPApprovalFile(t, tempDir, rejectedApproval, "approval-rejected.json")
	stdout.Reset()
	stderr.Reset()
	code = runMCPProposalExecuteCLI(t, proposalPath, rejectedPath, configPath, policyPath, filepath.Join(tempDir, "artifacts-rejected"), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "approved") {
		t.Fatalf("rejected execute code=%d stderr=%q, want approval rejection", code, stderr.String())
	}

	changedProposal := proposal
	changedProposal.Arguments = json.RawMessage(`{"text":"changed"}`)
	changedProposal.ArgumentsSHA256 = mcpapproval.ArgumentsSHA256(changedProposal.Arguments)
	changedPath := writeMCPProposalFile(t, tempDir, changedProposal)
	stdout.Reset()
	stderr.Reset()
	code = runMCPProposalExecuteCLI(t, changedPath, approvalPath, configPath, policyPath, filepath.Join(tempDir, "artifacts-arg-hash"), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "arguments_sha256") {
		t.Fatalf("changed arguments execute code=%d stderr=%q, want hash rejection", code, stderr.String())
	}

	originalConfig := mustReadCLIFile(t, configPath)
	if err := os.WriteFile(configPath, append(originalConfig, []byte("\n# changed config\n")...), 0o600); err != nil {
		t.Fatalf("WriteFile(config changed) error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runMCPProposalExecuteCLI(t, proposalPath, approvalPath, configPath, policyPath, filepath.Join(tempDir, "artifacts-config-hash"), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "config_sha256") {
		t.Fatalf("changed config execute code=%d stderr=%q, want config hash rejection", code, stderr.String())
	}
	if err := os.WriteFile(configPath, originalConfig, 0o600); err != nil {
		t.Fatalf("WriteFile(config restore) error = %v", err)
	}

	originalRuntime := mustReadCLIFile(t, runtimeConfigPath)
	if err := os.WriteFile(runtimeConfigPath, append(originalRuntime, []byte("\n# changed runtime\n")...), 0o600); err != nil {
		t.Fatalf("WriteFile(runtime changed) error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runMCPProposalExecuteCLI(t, proposalPath, approvalPath, configPath, policyPath, filepath.Join(tempDir, "artifacts-runtime-hash"), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "runtime_config_sha256") {
		t.Fatalf("changed runtime execute code=%d stderr=%q, want runtime hash rejection", code, stderr.String())
	}
	if err := os.WriteFile(runtimeConfigPath, originalRuntime, 0o600); err != nil {
		t.Fatalf("WriteFile(runtime restore) error = %v", err)
	}

	if err := os.WriteFile(policyPath, append(mustReadCLIFile(t, policyPath), []byte("\n# changed\n")...), 0o600); err != nil {
		t.Fatalf("WriteFile(policy changed) error = %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	code = runMCPProposalExecuteCLI(t, proposalPath, approvalPath, configPath, policyPath, filepath.Join(tempDir, "artifacts-policy-hash"), &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "policy_sha256") {
		t.Fatalf("changed policy execute code=%d stderr=%q, want policy hash rejection", code, stderr.String())
	}
}

func TestRunMCPProposalExecuteCallsCallSmokeGeneratesArtifacts(t *testing.T) {
	t.Setenv("MCP_TOKEN", "super-secret-value")
	argsPath := installCLIMCPFakeDocker(t, "fake", "docker fake stderr super-secret-value\n")
	tempDir := t.TempDir()
	configPath := writeCLIMCPFakeConfig(t, []string{"MCP_TOKEN"}, "fake", false, []string{"read"})
	policyPath := writeCLIMCPCallPolicy(t, []string{"fake-stdio"}, []string{"deonclaw.fake.echo"}, []string{"read"}, 65536, 80, true)
	runtimeConfigPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	proposal := newCLIMCPProposalWithConfig(t, "fake-stdio", []byte(`{"text":"super-secret-value `+strings.Repeat("x", 200)+`"}`), policyPath, configPath, runtimeConfigPath)
	proposalPath := writeMCPProposalFile(t, tempDir, proposal)
	approvalPath := writeMCPApprovalFile(t, tempDir, newCLIMCPApproval(t, proposal, policyPath, mcpapproval.ApprovalDecisionApproved), "approval.json")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := runMCPProposalExecuteCLI(t, proposalPath, approvalPath, configPath, policyPath, artifactsDir, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("execute exit code = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "approved read-only call smoke") ||
		!strings.Contains(stdout.String(), "tool_calls: 1") ||
		!strings.Contains(stdout.String(), "response_truncated: true") {
		t.Fatalf("stdout = %q, want execute summary", stdout.String())
	}
	for _, name := range []string{"mcp-call-smoke-summary.md", "mcp-call-transcript.jsonl", "mcp-call-result.json", "mcp-call-stdout.log", "mcp-call-stderr.log", "mcp-call-response.json", "mcp-call-execution-bundle.json"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, name)); err != nil {
			t.Fatalf("artifact %s stat error = %v", name, err)
		}
		assertCLIFileNotContains(t, filepath.Join(artifactsDir, name), "super-secret-value")
	}
	transcript := assertCLITranscriptJSONLValid(t, filepath.Join(artifactsDir, "mcp-call-transcript.jsonl"))
	assertCLITranscriptRequestMethodCount(t, transcript, "tools/call", 1)
	var response struct {
		ResponseTruncated bool `json:"response_truncated"`
	}
	if err := json.Unmarshal(mustReadCLIFile(t, filepath.Join(artifactsDir, "mcp-call-response.json")), &response); err != nil {
		t.Fatalf("mcp-call-response.json invalid: %v", err)
	}
	if !response.ResponseTruncated {
		t.Fatalf("response = %#v, want truncation", response)
	}
	var bundle struct {
		ProposalID          string            `json:"proposal_id"`
		ApprovalSHA256      string            `json:"approval_sha256"`
		ProposalSHA256      string            `json:"proposal_sha256"`
		PolicySHA256        string            `json:"policy_sha256"`
		ConfigSHA256        string            `json:"config_sha256"`
		RuntimeConfigSHA256 string            `json:"runtime_config_sha256"`
		Artifacts           map[string]string `json:"artifacts"`
		ResponseTruncated   bool              `json:"response_truncated"`
		ToolCalls           int               `json:"tool_calls"`
	}
	if err := json.Unmarshal(mustReadCLIFile(t, filepath.Join(artifactsDir, "mcp-call-execution-bundle.json")), &bundle); err != nil {
		t.Fatalf("mcp-call-execution-bundle.json invalid: %v", err)
	}
	if bundle.ProposalID != proposal.ID ||
		bundle.ApprovalSHA256 == "" ||
		bundle.ProposalSHA256 == "" ||
		bundle.PolicySHA256 == "" ||
		bundle.ConfigSHA256 == "" ||
		bundle.RuntimeConfigSHA256 == "" ||
		!bundle.ResponseTruncated ||
		bundle.ToolCalls != 1 {
		t.Fatalf("bundle = %#v, want execution hashes/truncation/tool call count", bundle)
	}
	for _, name := range []string{"mcp-call-smoke-summary.md", "mcp-call-transcript.jsonl", "mcp-call-result.json", "mcp-call-stdout.log", "mcp-call-stderr.log", "mcp-call-response.json"} {
		if bundle.Artifacts[name] == "" {
			t.Fatalf("bundle artifacts = %#v, missing %s", bundle.Artifacts, name)
		}
	}
	args := readCLIDockerArgs(t, argsPath)
	joinedArgs := strings.Join(args, " ")
	if strings.Contains(joinedArgs, "sh -c") || strings.Contains(joinedArgs, "super-secret-value") {
		t.Fatalf("docker args unsafe or leaked secret: %#v", args)
	}
}

func TestCLIMCPFakeServerHelperProcess(t *testing.T) {
	if os.Getenv("DEONCLAW_CLI_MCP_FAKE_SERVER_HELPER") != "1" {
		return
	}
	args := cliHelperArgs()
	if len(args) > 0 && args[0] == "hang" {
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	os.Exit(runMCPFakeServer(os.Stdin, os.Stdout, os.Stderr))
}

func TestCLIMCPFakeDockerHelperProcess(t *testing.T) {
	if os.Getenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_HELPER") != "1" {
		return
	}
	os.Exit(runCLIMCPFakeDockerHelper())
}

func TestRunRuntimeDockerPlanText(t *testing.T) {
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-plan", "--config", configPath, "--workspace", "."}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"runtime: docker",
		"docker run --rm --network none",
		"--memory 2g",
		"--cpus 2",
		"--read-only",
		"-v .:/workspace:rw",
		"-v mysecondbrain:/memory/mysecondbrain:ro",
		"--label deonclaw.workspace=.",
		"deonclaw-runner:latest",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunRuntimeDockerPlanJSON(t *testing.T) {
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-plan", "--config", configPath, "--workspace", ".", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
	var decoded struct {
		Command []string `json:"command"`
		Display string   `json:"display"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v, stdout=%s", err, stdout.String())
	}
	if len(decoded.Command) == 0 || decoded.Command[0] != "docker" {
		t.Fatalf("command = %#v, want docker command", decoded.Command)
	}
	if strings.Contains(decoded.Display, "<prompt>") {
		t.Fatalf("display unexpectedly contains worker prompt marker: %q", decoded.Display)
	}
	if !strings.Contains(decoded.Display, "docker run --rm --network none") {
		t.Fatalf("display = %q, want docker plan", decoded.Display)
	}
}

func TestRunRuntimeDockerPlanIncludesEnvPassthroughNameOnly(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	configPath := writeCLIRuntimeConfig(t, runtimeConfigWithEnvPassthroughYAML("ZAI_API_KEY"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-plan", "--config", configPath, "--workspace", "."}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "-e ZAI_API_KEY") {
		t.Fatalf("stdout = %q, want env passthrough name", output)
	}
	if strings.Contains(output, "zai-real-secret") {
		t.Fatalf("stdout leaked secret: %q", output)
	}
}

func TestRunExampleOpenCodeZAISmokeDockerPlanIncludesEnvNameOnly(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	configPath := examplePath(t, "configs", "examples", "runtime-opencode-zai-smoke.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-plan", "--config", configPath, "--workspace", "."}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"docker run --rm --network default",
		"-e ZAI_API_KEY",
		"-v .:/workspace:rw",
		"-v mysecondbrain:/memory/mysecondbrain:ro",
		"-v escalasoft_brain:/memory/escalasoft_brain:ro",
		"deonclaw-runner:latest",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if strings.Contains(output, "zai-real-secret") {
		t.Fatalf("stdout leaked secret: %q", output)
	}
	if strings.Contains(output, "OPENAI_API_KEY") {
		t.Fatalf("stdout = %q, smoke runtime should not pass OPENAI_API_KEY", output)
	}
}

func TestRunRuntimeDockerExecWithFakeDockerCapturesArgs(t *testing.T) {
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
printf 'docker stdout\n'
printf 'docker stderr\n' >&2
exit 0
`)
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--", "echo", "hello"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "docker stdout") {
		t.Fatalf("stdout = %q, want fake docker stdout", stdout.String())
	}
	if !strings.Contains(stderr.String(), "docker stderr") {
		t.Fatalf("stderr = %q, want fake docker stderr", stderr.String())
	}
	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	for _, want := range []string{"run", "--rm", "--network", "none", "--read-only", "deonclaw-runner:latest", "echo", "hello"} {
		if !stringSliceContains(args, want) {
			t.Fatalf("docker args = %#v, want %q", args, want)
		}
	}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}
	if args[len(args)-2] != "echo" || args[len(args)-1] != "hello" {
		t.Fatalf("docker args tail = %#v, want command appended after image", args)
	}
}

func TestRunRuntimeDockerExecRejectsMissingEnvPassthroughBeforeDocker(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	configPath := writeCLIRuntimeConfig(t, runtimeConfigWithEnvPassthroughYAML("ZAI_API_KEY"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--", "echo", "hello"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want missing env name", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunRuntimeDockerExecWithEnvPassthroughCallsFakeDockerNameOnly(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
printf 'docker stdout\n'
exit 0
`)
	configPath := writeCLIRuntimeConfig(t, runtimeConfigWithEnvPassthroughYAML("ZAI_API_KEY"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--", "echo", "hello"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-e ZAI_API_KEY") {
		t.Fatalf("docker args = %#v, want env passthrough flag", args)
	}
	if strings.Contains(joined, "zai-real-secret") || strings.Contains(stdout.String(), "zai-real-secret") || strings.Contains(stderr.String(), "zai-real-secret") {
		t.Fatalf("secret leaked; args=%#v stdout=%q stderr=%q", args, stdout.String(), stderr.String())
	}
	if args[len(args)-2] != "echo" || args[len(args)-1] != "hello" {
		t.Fatalf("docker args tail = %#v, want command appended after image", args)
	}
}

func TestRunRuntimeDockerExecRejectsEmptyCommand(t *testing.T) {
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing command after --") {
		t.Fatalf("stderr = %q, want empty command rejection", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestRunRuntimeDockerExecInvalidConfigFailsBeforeDocker(t *testing.T) {
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	configPath := writeCLIRuntimeConfig(t, `runtime:
  mode: podman
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--", "echo", "hello"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `runtime.mode "podman" is not supported`) {
		t.Fatalf("stderr = %q, want invalid config", stderr.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunRuntimeDockerExecDangerousMountFailsBeforeDocker(t *testing.T) {
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	configPath := writeCLIRuntimeConfig(t, `runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    mounts:
      - source: /
        target: /workspace
        mode: ro
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--", "echo", "hello"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `runtime.docker.mounts[0].source "/" is not allowed`) {
		t.Fatalf("stderr = %q, want dangerous mount rejection", stderr.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunRuntimeDockerExecPropagatesNonZeroExitCode(t *testing.T) {
	installFakeDocker(t, `#!/bin/sh
printf 'docker stdout\n'
printf 'docker stderr\n' >&2
exit 17
`)
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"runtime", "docker-exec", "--config", configPath, "--workspace", ".", "--", "false"}, &stdout, &stderr)
	if code != 17 {
		t.Fatalf("run() exit code = %d, want 17", code)
	}
	if !strings.Contains(stdout.String(), "docker stdout") {
		t.Fatalf("stdout = %q, want fake docker stdout", stdout.String())
	}
	if !strings.Contains(stderr.String(), "docker stderr") {
		t.Fatalf("stderr = %q, want fake docker stderr", stderr.String())
	}
}

func TestRunContextBuild(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")
	domainsPath := writeDomainsConfigFile(t)
	outputPath := filepath.Join(t.TempDir(), "context.md")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"context", "build", "--task", taskPath, "--domains", domainsPath, "--output", outputPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "context pack written:") {
		t.Fatalf("stdout = %q, want context pack written output", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	output := string(data)
	for _, want := range []string{"id: worker-mismatch-001", "domain: general", "context sources"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunWorkerOpenCodeRun(t *testing.T) {
	restore := overrideOpenCodeRunDeps(t, &workers.RunResult{
		Worker:    "opencode",
		Command:   []string{"opencode", "run", "--dir", "workspace", "--format", "json", "<prompt>"},
		Events:    []workers.WorkerEvent{{Type: "message", Worker: "opencode", Payload: []byte("{\"type\":\"message\",\"text\":\"ok\"}")}},
		Artifacts: []artifacts.Artifact{{Path: "stdout.log", Kind: artifacts.KindLog, Content: []byte("opencode stdout\n")}},
		Stderr:    "opencode stderr\n",
	}, nil)
	defer restore()

	tempDir := t.TempDir()
	taskPath := writeTaskFile(t, "opencode")
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", storePath, "--artifacts-dir", artifactsDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "run_id: run-cli-opencode-001") {
		t.Fatalf("stdout = %q, want opencode run id", stdout.String())
	}
	runDir := filepath.Join(artifactsDir, "run-cli-opencode-001")
	assertCLIFileContent(t, filepath.Join(runDir, "stdout.log"), "opencode stdout\n")
	assertCLIFileContent(t, filepath.Join(runDir, "stderr.log"), "opencode stderr\n")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Worker: opencode")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	gotRun, err := db.Run(context.Background(), "run-cli-opencode-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Worker != "opencode" || gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run = %#v, want opencode succeeded", gotRun)
	}
}

func TestRunMemoryProposalNew(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "memory-proposal.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"memory",
		"proposal",
		"new",
		"--run",
		"run-001",
		"--task",
		"task-001",
		"--domain",
		"general",
		"--target",
		"memory/context/example.md",
		"--operation",
		"append",
		"--reason",
		"Capture stable workflow.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "memory proposal written: "+outputPath) {
		t.Fatalf("stdout = %q, want proposal written output", stdout.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	var proposal memory.MemoryProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		t.Fatalf("Unmarshal(output) error = %v", err)
	}
	if proposal.Status != memory.StatusProposed {
		t.Fatalf("status = %q, want %q", proposal.Status, memory.StatusProposed)
	}
	if proposal.Operation != memory.OperationAppend {
		t.Fatalf("operation = %q, want append", proposal.Operation)
	}
	if proposal.Domain != "general" || proposal.TargetPath != "memory/context/example.md" {
		t.Fatalf("proposal = %#v, want domain and target", proposal)
	}
	if len(proposal.Evidence) != 1 || proposal.Evidence[0].RunID != "run-001" {
		t.Fatalf("evidence = %#v, want run evidence", proposal.Evidence)
	}
	if len(proposal.Patches) != 1 || proposal.Patches[0].TargetPath != "memory/context/example.md" {
		t.Fatalf("patches = %#v, want target patch", proposal.Patches)
	}
}

func TestRunMemoryProposalNewRejectsInvalidOperation(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "memory-proposal.json")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"memory",
		"proposal",
		"new",
		"--run",
		"run-001",
		"--task",
		"task-001",
		"--domain",
		"general",
		"--target",
		"memory/context/example.md",
		"--operation",
		"delete",
		"--reason",
		"Invalid operation should fail.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `operation "delete" is not supported`) {
		t.Fatalf("stderr = %q, want invalid operation", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output file exists after invalid operation: %v", err)
	}
}

func TestRunMemoryProposalLint(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := filepath.Join(tempDir, "memory-proposal.json")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-lint-001",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/domains/escalasoft_brain/cases/case-001.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Lint CLI proposal.",
		CreatedAt:  time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC),
	})
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	if err := os.WriteFile(proposalPath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(proposal) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"lint",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"proposal_id: mem-cli-lint-001",
		"status: ok",
		"violations: 0",
		"warnings: 0",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunMemoryProposalApplyDryRunAppend(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory-target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-append",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preview append.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "append content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"proposal_id: mem-cli-apply-append",
		"domain: general",
		"target_path: " + targetPath,
		"operation: append",
		"status: dry_run_ok",
		"patch_count: 1",
		"would append content",
		"append content",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run: %v", err)
	}
}

func TestRunMemoryProposalApplyDryRunCreate(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "new-memory.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-create",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationCreate,
		Reason:     "Preview create.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationCreate, Content: "new memory\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "would create file") {
		t.Fatalf("stdout = %q, want create preview", stdout.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run: %v", err)
	}
}

func TestRunMemoryProposalApplyRejectsLintFailure(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-lint-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/vault/mysecondbrain/MEMORY.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Should fail lint.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "status: failed") {
		t.Fatalf("stdout = %q, want failed preview", stdout.String())
	}
	if !strings.Contains(stdout.String(), "lint violations:") {
		t.Fatalf("stdout = %q, want lint violations", stdout.String())
	}
}

func TestRunMemoryProposalApplyRequiresDryRun(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-require-dry-run",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: filepath.Join(tempDir, "target.md"),
		Operation:  memory.OperationAppend,
		Reason:     "Requires dry-run.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: filepath.Join(tempDir, "target.md"), Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --dry-run") {
		t.Fatalf("stderr = %q, want missing --dry-run", stderr.String())
	}
}

func TestRunMemoryProposalApplyOutputWritesPreviewJSON(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	outputPath := filepath.Join(tempDir, "apply-preview.json")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-output",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preview output.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile(output) error = %v", err)
	}
	var preview memory.ApplyPreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatalf("Unmarshal(output) error = %v", err)
	}
	if preview.Status != memory.ApplyStatusDryRunOK {
		t.Fatalf("preview status = %q, want %q", preview.Status, memory.ApplyStatusDryRunOK)
	}
}

func TestRunMemoryProposalApplyRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Do not write preview to target.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
		"--output",
		targetPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write apply preview to target_path") {
		t.Fatalf("stderr = %q, want target_path refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after dry-run output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyInvalidOperationFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-invalid-operation",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: filepath.Join(tempDir, "target.md"),
		Operation:  memory.OperationAppend,
		Reason:     "Invalid operation.",
		CreatedAt:  time.Date(2026, 5, 20, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: filepath.Join(tempDir, "target.md"), Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	proposal.Operation = memory.MemoryOperation("delete")
	proposal.Patches[0].Operation = memory.MemoryOperation("delete")
	proposalPath := writeRawJSONFile(t, tempDir, "memory-proposal-invalid.json", proposal)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), `operation "delete" is not supported`) {
		t.Fatalf("stdout = %q, want invalid operation", stdout.String())
	}
}

func TestRunMemoryProposalApproveApprovedWithLintOKGeneratesApproval(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory-target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-ok",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Approve lint-ok proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Reviewed and approved.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "memory approval written: "+outputPath) {
		t.Fatalf("stdout = %q, want approval written", stdout.String())
	}

	approval := readMemoryApprovalFile(t, outputPath)
	if approval.ProposalID != "mem-cli-approve-ok" {
		t.Fatalf("proposal_id = %q, want mem-cli-approve-ok", approval.ProposalID)
	}
	if approval.Decision != memory.DecisionApproved {
		t.Fatalf("decision = %q, want approved", approval.Decision)
	}
	if approval.Reviewer != "Davi" || approval.Reason != "Reviewed and approved." {
		t.Fatalf("approval = %#v, want reviewer and reason", approval)
	}
	if approval.LintStatus != memory.LintStatusOK {
		t.Fatalf("lint_status = %q, want ok", approval.LintStatus)
	}
	if len(approval.LintViolations) != 0 {
		t.Fatalf("lint_violations = %#v, want none", approval.LintViolations)
	}
	if approval.ApplyStatus != memory.ApplyStatusDryRunOK {
		t.Fatalf("apply_status = %q, want %q", approval.ApplyStatus, memory.ApplyStatusDryRunOK)
	}
	if approval.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", approval.PatchCount)
	}
	if approval.PatchWarnings == nil {
		t.Fatalf("patch_warnings is nil, want JSON array")
	}
	if approval.PatchViolations == nil || len(approval.PatchViolations) != 0 {
		t.Fatalf("patch_violations = %#v, want empty JSON array", approval.PatchViolations)
	}
	if approval.ProposalSHA256 == "" || approval.ApplyPreviewSHA256 == "" {
		t.Fatalf("approval hashes are empty: %#v", approval)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval: %v", err)
	}
}

func TestRunMemoryProposalApproveApprovedWithLintFailedFails(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-lint-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/vault/mysecondbrain/MEMORY.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Should not approve failed lint.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 5, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	}))
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Approve anyway.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "approved decision requires proposal lint status ok") {
		t.Fatalf("stderr = %q, want approval lint failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("approval output exists after failed approve: %v", err)
	}
}

func TestRunMemoryProposalApproveRejectedWithLintFailedGeneratesApproval(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-reject-lint-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "escalasoft",
		TargetPath: "/vault/mysecondbrain/MEMORY.md",
		Operation:  memory.OperationUpdate,
		Reason:     "Reject failed lint.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 10, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/MEMORY.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	}))
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"rejected",
		"--reason",
		"Violates protected memory boundary.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	approval := readMemoryApprovalFile(t, outputPath)
	if approval.Decision != memory.DecisionRejected {
		t.Fatalf("decision = %q, want rejected", approval.Decision)
	}
	if approval.LintStatus != memory.LintStatusFailed {
		t.Fatalf("lint_status = %q, want failed", approval.LintStatus)
	}
	if len(approval.LintViolations) == 0 {
		t.Fatalf("lint_violations = %#v, want violations", approval.LintViolations)
	}
	if approval.ApplyStatus != memory.ApplyStatusFailed {
		t.Fatalf("apply_status = %q, want failed", approval.ApplyStatus)
	}
}

func TestRunMemoryProposalApproveApprovedWithPatchViolationFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-patch-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Approve patch violation.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 11, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	})
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Approve despite patch violation.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "approved decision requires apply_status dry_run_ok") {
		t.Fatalf("stderr = %q, want apply status failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("approval output exists after failed approve: %v", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval failure: %v", err)
	}
}

func TestRunMemoryProposalApproveRejectedWithPatchViolationGeneratesApproval(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-reject-patch-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Reject patch violation.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 12, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Content: "bad\n"},
		},
	})
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"rejected",
		"--reason",
		"Patch violates protected path.",
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	approval := readMemoryApprovalFile(t, outputPath)
	if approval.LintStatus != memory.LintStatusOK {
		t.Fatalf("lint_status = %q, want ok", approval.LintStatus)
	}
	if approval.ApplyStatus != memory.ApplyStatusFailed {
		t.Fatalf("apply_status = %q, want failed", approval.ApplyStatus)
	}
	if approval.PatchCount != 1 {
		t.Fatalf("patch_count = %d, want 1", approval.PatchCount)
	}
	if len(approval.PatchViolations) != 1 || !strings.Contains(approval.PatchViolations[0].Violation, "protected path") {
		t.Fatalf("patch_violations = %#v, want protected path violation", approval.PatchViolations)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval: %v", err)
	}
}

func TestRunMemoryProposalApproveRequiresReviewer(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-missing-reviewer")
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--decision", "approved",
		"--reason", "Reviewed.",
		"--output", outputPath,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --reviewer") {
		t.Fatalf("stderr = %q, want missing reviewer", stderr.String())
	}
}

func TestRunMemoryProposalApproveRequiresReason(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-missing-reason")
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer", "Davi",
		"--decision", "approved",
		"--output", outputPath,
	}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --reason") {
		t.Fatalf("stderr = %q, want missing reason", stderr.String())
	}
}

func TestRunMemoryProposalApproveRejectsInvalidDecision(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-bad-decision")
	outputPath := filepath.Join(tempDir, memory.ApprovalJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer", "Davi",
		"--decision", "maybe",
		"--reason", "Invalid decision.",
		"--output", outputPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "decision \"maybe\" is not supported") {
		t.Fatalf("stderr = %q, want invalid decision", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("approval output exists after invalid decision: %v", err)
	}
}

func TestRunMemoryProposalApproveRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	proposalPath := writeApprovalTestProposal(t, tempDir, "mem-cli-approve-output-memory-domain")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"approve",
		"--proposal",
		proposalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer",
		"Davi",
		"--decision",
		"approved",
		"--reason",
		"Reviewed.",
		"--output",
		"/vault/mysecondbrain/memory-approval.json",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write approval artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalApproveRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposalPath := writeMemoryProposalFile(t, tempDir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-approve-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Do not write approval to target.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 20, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	}))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory", "proposal", "approve",
		"--proposal", proposalPath,
		"--policy", filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--reviewer", "Davi",
		"--decision", "approved",
		"--reason", "Reviewed.",
		"--output", targetPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write approval artifact to target_path") {
		t.Fatalf("stderr = %q, want target_path refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after approval output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightOKWritesOutput(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-preflight-ok",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preflight ok.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.ApplyPreflightJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"memory",
		"proposal",
		"apply-preflight",
		"--proposal",
		proposalPath,
		"--approval",
		approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--output",
		outputPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "status: preflight_ok") {
		t.Fatalf("stdout = %q, want preflight_ok", stdout.String())
	}

	preflight := readApplyPreflightFile(t, outputPath)
	if preflight.Status != memory.ApplyPreflightStatusOK {
		t.Fatalf("status = %q, want preflight_ok", preflight.Status)
	}
	if preflight.LintStatus != memory.LintStatusOK || preflight.ApplyStatus != memory.ApplyStatusDryRunOK {
		t.Fatalf("preflight = %#v, want lint/apply ok", preflight)
	}
	if preflight.ProposalSHA256 == "" || preflight.ApprovalProposalSHA256 == "" || preflight.ApplyPreviewSHA256 == "" || preflight.ApprovalApplyPreviewSHA256 == "" {
		t.Fatalf("preflight hashes are empty: %#v", preflight)
	}
	if preflight.ProposalSHA256 != preflight.ApprovalProposalSHA256 {
		t.Fatalf("proposal hashes = %q/%q, want equal", preflight.ProposalSHA256, preflight.ApprovalProposalSHA256)
	}
	if preflight.ApplyPreviewSHA256 != preflight.ApprovalApplyPreviewSHA256 {
		t.Fatalf("apply preview hashes = %q/%q, want equal", preflight.ApplyPreviewSHA256, preflight.ApprovalApplyPreviewSHA256)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after preflight: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightApprovalForDifferentProposalFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-mismatch")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	approval.ProposalID = "mem-other"
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "approval proposal_id") {
		t.Fatalf("stdout = %q, want proposal_id failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightRejectedDecisionFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-rejected")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionRejected)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "approval decision") {
		t.Fatalf("stdout = %q, want decision failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightCurrentLintFailureFails(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-lint-failed")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposal.TargetPath = "/vault/mysecondbrain/SOUL.md"
	approval.TargetPath = proposal.TargetPath
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "current lint_status") {
		t.Fatalf("stdout = %q, want current lint failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightCurrentApplyFailureFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	approvedProposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-preflight-apply-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preflight apply failed.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 10, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	proposal := approvedProposal
	proposal.Patches = []memory.MemoryPatch{
		{TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Content: "bad\n"},
	}
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, approvedProposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "current apply_status") {
		t.Fatalf("stdout = %q, want current apply failure", stdout.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after preflight: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightApprovalPatchViolationsFail(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-approval-patch-violations")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	approval.PatchViolations = []memory.ApplyPatchViolation{
		{PatchIndex: 0, TargetPath: "/vault/mysecondbrain/SOUL.md", Operation: memory.OperationUpdate, Violation: "protected path"},
	}
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "approval patch_violations") {
		t.Fatalf("stdout = %q, want approval patch violation failure", stdout.String())
	}
}

func TestRunMemoryProposalApplyPreflightRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-preflight-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Preflight output target.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 15, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, targetPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write preflight artifact to target_path") {
		t.Fatalf("stderr = %q, want target_path refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target file exists after preflight output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyPreflightRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-preflight-output-memory-domain")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyPreflight(t, proposalPath, approvalPath, "/vault/mysecondbrain/apply-preflight.json", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write preflight artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalBackupPlanExistingTargetWritesPlan(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-existing",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup existing target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "new content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "items: 1") {
		t.Fatalf("stdout = %q, want items", stdout.String())
	}

	plan := readBackupPlanFile(t, outputPath)
	if len(plan.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(plan.Items))
	}
	item := plan.Items[0]
	if !item.Exists {
		t.Fatalf("exists = false, want true")
	}
	if item.SHA256 == nil || *item.SHA256 == "" {
		t.Fatalf("sha256 is empty: %#v", item)
	}
	if item.SizeBytes == nil || *item.SizeBytes != int64(len(content)) {
		t.Fatalf("size_bytes = %v, want %d", item.SizeBytes, len(content))
	}
	if len(plan.RestorePlan.Items) != 1 {
		t.Fatalf("restore items = %d, want 1", len(plan.RestorePlan.Items))
	}
	if _, err := os.Stat(item.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("backup path was written unexpectedly: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupPlanMissingTargetWritesNotExists(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "missing.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-missing",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationCreate,
		Reason:     "Backup missing target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 5, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationCreate, Content: "new content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	plan := readBackupPlanFile(t, outputPath)
	item := plan.Items[0]
	if item.Exists {
		t.Fatalf("exists = true, want false")
	}
	if item.SHA256 != nil || item.SizeBytes != nil {
		t.Fatalf("item = %#v, want no hash/size for missing target", item)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupPlanMultiplePatches(t *testing.T) {
	tempDir := t.TempDir()
	first := filepath.Join(tempDir, "first.md")
	second := filepath.Join(tempDir, "second.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-multiple",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: first,
		Operation:  memory.OperationAppend,
		Reason:     "Backup multiple targets.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 10, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: first, Operation: memory.OperationAppend, Content: "first\n"},
			{TargetPath: second, Operation: memory.OperationCreate, Content: "second\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	plan := readBackupPlanFile(t, outputPath)
	if len(plan.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(plan.Items))
	}
}

func TestRunMemoryProposalBackupPlanDeduplicatesRepeatedTargets(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "same.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-dedupe",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup repeated target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 12, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "first\n"},
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "second\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	plan := readBackupPlanFile(t, outputPath)
	if len(plan.Items) != 1 {
		t.Fatalf("items = %d, want 1 deduplicated target", len(plan.Items))
	}
	if len(plan.RestorePlan.Items) != 1 {
		t.Fatalf("restore items = %d, want 1 deduplicated target", len(plan.RestorePlan.Items))
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupPlanPreflightFailureBlocksOutput(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-preflight-failed",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup preflight failed.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 15, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	approval.Decision = memory.DecisionRejected
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)
	outputPath := filepath.Join(tempDir, memory.BackupPlanJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, outputPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "apply preflight failed") {
		t.Fatalf("stderr = %q, want preflight failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("backup plan output exists after preflight failure: %v", err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupPlanRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-output-target",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "Backup output target.",
		CreatedAt:  time.Date(2026, 5, 23, 15, 20, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, targetPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup plan artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupPlanRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	proposal := testCLIPreflightProposal(t, tempDir, "mem-cli-backup-output-memory-domain")
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	proposalPath := writeMemoryProposalFile(t, tempDir, proposal)
	approvalPath := writeMemoryApprovalFile(t, tempDir, approval)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupPlan(t, proposalPath, approvalPath, "/vault/mysecondbrain/backup-plan.json", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup plan artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalBackupMaterializeExistingTargetWritesResult(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)
	outputPath := filepath.Join(tempDir, memory.BackupResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "items: 1") {
		t.Fatalf("stdout = %q, want items", stdout.String())
	}
	result := readBackupResultFile(t, outputPath)
	if len(result.Items) != 1 {
		t.Fatalf("result items = %d, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Status != memory.BackupResultStatusCopied {
		t.Fatalf("status = %q, want %q", item.Status, memory.BackupResultStatusCopied)
	}
	if item.BackupSHA256 == nil || *item.BackupSHA256 == "" {
		t.Fatalf("backup_sha256 is empty: %#v", item)
	}
	if got := string(mustReadCLIFile(t, plan.Items[0].BackupPath)); got != string(content) {
		t.Fatalf("backup content = %q, want %q", got, content)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupMaterializeMissingTargetSkipped(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "missing.md")
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationCreate)
	planPath := writeBackupPlanFile(t, tempDir, plan)
	outputPath := filepath.Join(tempDir, memory.BackupResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, outputPath, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	result := readBackupResultFile(t, outputPath)
	if got := result.Items[0].Status; got != memory.BackupResultStatusSkippedMissing {
		t.Fatalf("status = %q, want %q", got, memory.BackupResultStatusSkippedMissing)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written unexpectedly: %v", err)
	}
}

func TestRunMemoryProposalBackupMaterializeRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, targetPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup result artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupMaterializeRefusesOutputAtBackupPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)
	backupPath := plan.Items[0].BackupPath
	sentinel := []byte("already backed up\n")
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(backup dir) error = %v", err)
	}
	if err := os.WriteFile(backupPath, sentinel, 0o600); err != nil {
		t.Fatalf("WriteFile(backup sentinel) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, backupPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup result artifact to backup_path") {
		t.Fatalf("stderr = %q, want backup_path refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, backupPath)); got != string(sentinel) {
		t.Fatalf("backup_path content = %q, want sentinel unchanged", got)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupMaterializeRefusesOutputInsideBackupRoot(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	content := []byte("existing content\n")
	if err := os.WriteFile(targetPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)
	outputPath := filepath.Join(plan.BackupRoot, "backup-result.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, outputPath, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup result artifact inside backup_root") {
		t.Fatalf("stderr = %q, want backup_root refusal", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output inside backup_root was written unexpectedly: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != string(content) {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalBackupMaterializeRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	if err := os.WriteFile(targetPath, []byte("existing content\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(target) error = %v", err)
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, memory.OperationAppend)
	planPath := writeBackupPlanFile(t, tempDir, plan)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runBackupMaterialize(t, planPath, "/vault/mysecondbrain/backup-result.json", &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write backup result artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
}

func TestRunMemoryProposalRestoreDryRunExistingTargetWritesPreview(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestorePreviewJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "restore preview written: "+outputPath) {
		t.Fatalf("stdout = %q, want restore preview output", stdout.String())
	}
	preview := readRestorePreviewFile(t, outputPath)
	if len(preview.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(preview.Items))
	}
	if preview.Items[0].Action != memory.RestorePreviewActionRestoreFromBackup {
		t.Fatalf("action = %q, want %q", preview.Items[0].Action, memory.RestorePreviewActionRestoreFromBackup)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged dry-run target", got)
	}
}

func TestRunMemoryProposalRestoreDryRunMissingOriginalTargetShowsRemoveIfExists(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationCreate, "")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("created by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(created target) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestorePreviewJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	preview := readRestorePreviewFile(t, outputPath)
	if got := preview.Items[0].Action; got != memory.RestorePreviewActionRemoveIfExists {
		t.Fatalf("action = %q, want %q", got, memory.RestorePreviewActionRemoveIfExists)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "created by apply\n" {
		t.Fatalf("target content = %q, want unchanged dry-run target", got)
	}
}

func TestRunMemoryProposalRestoreRequiresDryRun(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	outputPath := filepath.Join(tempDir, memory.RestorePreviewJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, outputPath, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --dry-run") {
		t.Fatalf("stderr = %q, want missing dry-run", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("restore preview was written without dry-run: %v", err)
	}
}

func TestRunMemoryProposalRestoreCorruptedBackupFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(artifacts.backupPlan.Items[0].BackupPath, []byte("corrupted backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupted backup) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestorePreviewJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "backup_path") || !strings.Contains(stderr.String(), "sha256") {
		t.Fatalf("stderr = %q, want backup hash failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("restore preview was written after backup hash failure: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalRestoreBackupResultMismatchFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	result := readBackupResultFile(t, artifacts.backupResultPath)
	result.ProposalID = "other-proposal"
	artifacts.backupResultPath = writeRawJSONFile(t, tempDir, "backup-result-mismatch.json", result)
	outputPath := filepath.Join(tempDir, memory.RestorePreviewJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "backup result proposal_id") {
		t.Fatalf("stderr = %q, want backup result mismatch", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalRestoreRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, targetPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write restore preview artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalRestoreRefusesOutputAtBackupPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	backupPath := artifacts.backupPlan.Items[0].BackupPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, backupPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write restore preview artifact to backup_path") {
		t.Fatalf("stderr = %q, want backup_path refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, backupPath)); got != "original memory\n" {
		t.Fatalf("backup_path content = %q, want unchanged", got)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalRestoreRefusesOutputInsideBackupRoot(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	outputPath := filepath.Join(artifacts.backupPlan.BackupRoot, "restore-preview.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write restore preview artifact inside backup_root") {
		t.Fatalf("stderr = %q, want backup_root refusal", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output inside backup_root was written unexpectedly: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalRestoreRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreDryRun(t, artifacts, "/vault/mysecondbrain/restore-preview.json", true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write restore preview artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalRestoreExecuteExistingTargetRestoresTarget(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestoreResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "restore result written: "+outputPath) {
		t.Fatalf("stdout = %q, want restore result output", stdout.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "original memory\n" {
		t.Fatalf("target content = %q, want restored original", got)
	}
	result := readRestoreResultFile(t, outputPath)
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	if got := result.Items[0].Status; got != memory.RestoreResultStatusRestored {
		t.Fatalf("status = %q, want %q", got, memory.RestoreResultStatusRestored)
	}
}

func TestRunMemoryProposalRestoreExecuteMissingOriginalRemovesTarget(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationCreate, "")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(target dir) error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("created by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(created target) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestoreResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target exists after restore removal: %v", err)
	}
	result := readRestoreResultFile(t, outputPath)
	if got := result.Items[0].Status; got != memory.RestoreResultStatusRemoved {
		t.Fatalf("status = %q, want %q", got, memory.RestoreResultStatusRemoved)
	}
}

func TestRunMemoryProposalRestoreExecuteRequiresConfirmRestore(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestoreResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, outputPath, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --confirm-restore") {
		t.Fatalf("stderr = %q, want missing confirm-restore", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("restore result was written without confirm: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged without confirm", got)
	}
}

func TestRunMemoryProposalRestoreExecuteCorruptedBackupFailsWithoutChangingTarget(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	if err := os.WriteFile(artifacts.backupPlan.Items[0].BackupPath, []byte("corrupted backup\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(corrupted backup) error = %v", err)
	}
	outputPath := filepath.Join(tempDir, memory.RestoreResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "backup_path") || !strings.Contains(stderr.String(), "sha256") {
		t.Fatalf("stderr = %q, want backup hash failure", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("restore result was written after backup hash failure: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged after restore failure", got)
	}
}

func TestRunMemoryProposalRestoreExecutePreviewDivergenceFails(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	preview := readRestorePreviewFile(t, artifacts.restorePreviewPath)
	preview.Items[0].Action = memory.RestorePreviewActionRemoveIfExists
	artifacts.restorePreviewPath = writeRawJSONFile(t, tempDir, "restore-preview-diverged.json", preview)
	outputPath := filepath.Join(tempDir, memory.RestoreResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "restore-preview") || !strings.Contains(stderr.String(), "diverged") {
		t.Fatalf("stderr = %q, want restore-preview divergence", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged after preview divergence", got)
	}
}

func TestRunMemoryProposalRestoreExecuteRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, targetPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write restore result artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged after output refusal", got)
	}
}

func TestRunMemoryProposalRestoreExecuteRefusesOutputInsideBackupRoot(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIRestoreArtifacts(t, tempDir, targetPath, memory.OperationAppend, "original memory\n")
	if err := os.WriteFile(targetPath, []byte("changed by apply\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(changed target) error = %v", err)
	}
	outputPath := filepath.Join(artifacts.backupPlan.BackupRoot, "restore-result.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runRestoreExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write restore result artifact inside backup_root") {
		t.Fatalf("stderr = %q, want backup_root refusal", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output inside backup_root was written unexpectedly: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "changed by apply\n" {
		t.Fatalf("target content = %q, want unchanged after output refusal", got)
	}
}

func TestRunMemoryProposalApplyExecuteCreateWritesResult(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "created.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationCreate, "", "created memory\n")
	outputPath := filepath.Join(tempDir, memory.ApplyResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "apply result written: "+outputPath) {
		t.Fatalf("stdout = %q, want apply result output", stdout.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "created memory\n" {
		t.Fatalf("target content = %q, want created content", got)
	}
	result := readApplyResultFile(t, outputPath)
	if result.Status != memory.ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, memory.ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	if result.Items[0].Status != memory.ApplyResultStatusCreated {
		t.Fatalf("status = %q, want %q", result.Items[0].Status, memory.ApplyResultStatusCreated)
	}
}

func TestRunMemoryProposalApplyExecuteAppendWritesResult(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "target.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationAppend, "existing memory\n", "appended memory\n")
	outputPath := filepath.Join(tempDir, memory.ApplyResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "existing memory\nappended memory\n" {
		t.Fatalf("target content = %q, want appended content", got)
	}
	result := readApplyResultFile(t, outputPath)
	if result.Status != memory.ApplyResultStatusSucceeded {
		t.Fatalf("result status = %q, want %q", result.Status, memory.ApplyResultStatusSucceeded)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(result.Items))
	}
	if result.Items[0].Status != memory.ApplyResultStatusAppended {
		t.Fatalf("status = %q, want %q", result.Items[0].Status, memory.ApplyResultStatusAppended)
	}
	if result.Items[0].BytesWritten != int64(len("appended memory\n")) {
		t.Fatalf("bytes_written = %d, want %d", result.Items[0].BytesWritten, len("appended memory\n"))
	}
}

func TestRunMemoryProposalApplyExecuteWritesPartialResultOnPartialFailure(t *testing.T) {
	tempDir := t.TempDir()
	firstTarget := filepath.Join(tempDir, "memory", "created-one.md")
	secondTarget := filepath.Join(tempDir, "memory", "created-two.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, firstTarget, memory.OperationCreate, "", "created memory\n")
	outputPath := filepath.Join(tempDir, memory.ApplyResultJSONArtifactName)
	injectedErr := errors.New("injected second rename failure")
	partialResult := memory.ApplyResult{
		ProposalID: "proposal-partial",
		ApprovalID: "approval-partial",
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		Status:     memory.ApplyResultStatusPartialFailed,
		Items: []memory.ApplyResultItem{
			{
				TargetPath:   firstTarget,
				Operation:    memory.OperationCreate,
				Status:       memory.ApplyResultStatusCreated,
				BytesWritten: int64(len("created one\n")),
				SHA256:       strings.Repeat("a", 64),
			},
		},
		FailedItem: &memory.ApplyResultFailedItem{
			TargetPath: secondTarget,
			Operation:  memory.OperationCreate,
			Error:      injectedErr.Error(),
		},
		Error:     injectedErr.Error(),
		CreatedAt: time.Date(2026, 5, 24, 12, 30, 0, 0, time.UTC),
	}
	originalExecuteApply := memoryExecuteApply
	memoryExecuteApply = func(memory.MemoryProposal, memory.MemoryApproval, *memory.MemoryPolicy, memory.BackupPlan, memory.BackupResult, memory.NewApplyExecuteOptions) (memory.ApplyResult, error) {
		return partialResult, &memory.ApplyExecutionError{Result: partialResult, Err: injectedErr}
	}
	t.Cleanup(func() {
		memoryExecuteApply = originalExecuteApply
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "injected second rename failure") {
		t.Fatalf("stderr = %q, want injected error", stderr.String())
	}
	result := readApplyResultFile(t, outputPath)
	if result.Status != memory.ApplyResultStatusPartialFailed {
		t.Fatalf("result status = %q, want %q", result.Status, memory.ApplyResultStatusPartialFailed)
	}
	if len(result.Items) != 1 || result.Items[0].TargetPath != firstTarget {
		t.Fatalf("items = %#v, want first target applied only", result.Items)
	}
	if result.FailedItem == nil || result.FailedItem.TargetPath != secondTarget {
		t.Fatalf("failed_item = %#v, want second target", result.FailedItem)
	}
	if result.Error != injectedErr.Error() {
		t.Fatalf("result error = %q, want %q", result.Error, injectedErr.Error())
	}
}

func TestRunMemoryProposalApplyExecuteRequiresConfirmApply(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "created.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationCreate, "", "created memory\n")
	outputPath := filepath.Join(tempDir, memory.ApplyResultJSONArtifactName)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, outputPath, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --confirm-apply") {
		t.Fatalf("stderr = %q, want missing confirm", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written without confirm: %v", err)
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("apply result was written without confirm: %v", err)
	}
}

func TestRunMemoryProposalApplyExecuteRefusesOutputAtTargetPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "memory", "created.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationCreate, "", "created memory\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, targetPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write apply result artifact to target_path") {
		t.Fatalf("stderr = %q, want target refusal", stderr.String())
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("target was written despite output refusal: %v", err)
	}
}

func TestRunMemoryProposalApplyExecuteRefusesOutputInsideBackupRoot(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationAppend, "existing memory\n", "appended memory\n")
	outputPath := filepath.Join(artifacts.backupPlan.BackupRoot, "apply-result.json")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, outputPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write apply result artifact inside backup_root") {
		t.Fatalf("stderr = %q, want backup_root refusal", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("output inside backup_root was written unexpectedly: %v", err)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalApplyExecuteRefusesOutputAtBackupPath(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationAppend, "existing memory\n", "appended memory\n")
	backupPath := artifacts.backupPlan.Items[0].BackupPath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, backupPath, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write apply result artifact to backup_path") {
		t.Fatalf("stderr = %q, want backup_path refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, backupPath)); got != "existing memory\n" {
		t.Fatalf("backup_path content = %q, want backup copy unchanged", got)
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunMemoryProposalApplyExecuteRefusesOutputInsideMemoryDomain(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "target.md")
	artifacts := buildCLIApplyExecuteArtifacts(t, tempDir, targetPath, memory.OperationAppend, "existing memory\n", "appended memory\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runApplyExecute(t, artifacts, "/vault/mysecondbrain/apply-result.json", true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "refusing to write apply result artifact inside memory domain") {
		t.Fatalf("stderr = %q, want memory domain refusal", stderr.String())
	}
	if got := string(mustReadCLIFile(t, targetPath)); got != "existing memory\n" {
		t.Fatalf("target content = %q, want unchanged", got)
	}
}

func TestRunWorkerCodexDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker",
		"codex",
		"dry-run",
		filepath.Join("..", "..", "examples", "tasks", "codex-smoke.yaml"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "workspace: .") {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "command: codex exec --json --sandbox read-only --cd . -") {
		t.Fatalf("stdout = %q, want planned command", output)
	}
}

func TestRunWorkerCodexDryRunRejectsMismatchedTaskWorker(t *testing.T) {
	taskPath := writeTaskFile(t, "opencode")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"worker", "codex", "dry-run", taskPath}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	want := `task worker "opencode" does not match requested worker "codex"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunWorkerOpenCodeDryRun(t *testing.T) {
	taskPath := writeTaskFile(t, "opencode")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker",
		"opencode",
		"dry-run",
		taskPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "workspace: .") {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "policy: read-only") {
		t.Fatalf("stdout = %q, want policy", output)
	}
	if !strings.Contains(output, "command: opencode run --dir . --format json <prompt>") {
		t.Fatalf("stdout = %q, want planned command", output)
	}
}

func TestRunWorkerOpenCodeDryRunUsesWorkersConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(configPath, []byte(`workers:
  opencode:
    command: /usr/local/bin/opencode
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
`), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	taskPath := writeTaskFile(t, "opencode")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker",
		"opencode",
		"dry-run",
		taskPath,
		"--workers-config",
		configPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "command: /usr/local/bin/opencode run --dir . --format json <prompt>") {
		t.Fatalf("stdout = %q, want configured opencode command", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunWarnsWhenRequiredEnvMissing(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFile(t, "opencode")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "command: /usr/local/bin/opencode run --dir . --format json <prompt>") {
		t.Fatalf("stdout = %q, want planned command", stdout.String())
	}
	if strings.Contains(stdout.String(), "Do not execute") {
		t.Fatalf("stdout = %q, want masked prompt", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: worker opencode required env ZAI_API_KEY is missing") {
		t.Fatalf("stderr = %q, want missing env warning", stderr.String())
	}
}

func TestRunWorkerOpenCodeDryRunWarnsWhenProfileRequiredEnvMissing(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFileWithModelProfile(t, "opencode", "opencode-zai-glm-5-1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "command: /usr/local/bin/opencode run --dir . --format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want planned command with model", stdout.String())
	}
	if strings.Contains(stdout.String(), "Do not execute") {
		t.Fatalf("stdout = %q, want masked prompt", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: worker opencode required env ZAI_API_KEY is missing") {
		t.Fatalf("stderr = %q, want profile missing env warning", stderr.String())
	}
}

func TestRunWorkerOpenCodeDryRunRejectsMissingModelProfile(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
`)
	taskPath := writeTaskFileWithModelProfile(t, "opencode", "opencode-zai-glm-5-1")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `model profile "opencode-zai-glm-5-1" not found`) {
		t.Fatalf("stderr = %q, want missing model profile error", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") {
		t.Fatalf("stdout = %q, want no command", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunRejectsModelProfileWorkerMismatch(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  codex-profile:
    worker: codex
    provider: openai
    model: gpt-5
`)
	taskPath := writeTaskFileWithModelProfile(t, "opencode", "codex-profile")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `model profile "codex-profile" worker "codex" does not match task worker "opencode"`) {
		t.Fatalf("stderr = %q, want worker mismatch error", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") {
		t.Fatalf("stdout = %q, want no command", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunWithModelStrategyPlansFirstPreferredProfile(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    tags:
      - coding
  opencode-fast:
    worker: opencode
    provider: z-ai
    model: glm-5.1-mini
    model_arg: z-ai/glm-5.1-mini
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  fallback:
    - opencode-fast
  require_tags:
    - coding
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "model_strategy: planned") {
		t.Fatalf("stdout = %q, want planned model strategy", stdout.String())
	}
	if !strings.Contains(stdout.String(), "planned_model_profile: opencode-zai-glm-5-1") {
		t.Fatalf("stdout = %q, want first preferred profile", stdout.String())
	}
	if !strings.Contains(stdout.String(), "provider: z-ai") {
		t.Fatalf("stdout = %q, want planned provider", stdout.String())
	}
	if !strings.Contains(stdout.String(), "model: glm-5.1") {
		t.Fatalf("stdout = %q, want planned model", stdout.String())
	}
	if !strings.Contains(stdout.String(), "model_arg: z-ai/glm-5.1") {
		t.Fatalf("stdout = %q, want planned model_arg", stdout.String())
	}
	if !strings.Contains(stdout.String(), "command: /usr/local/bin/opencode run --dir . --format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want planned opencode command with first preferred --model", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunWarnsWhenPlannedStrategyProfileRequiredEnvMissing(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  require_tags:
    - coding
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "planned_model_profile: opencode-zai-glm-5-1") {
		t.Fatalf("stdout = %q, want planned profile", stdout.String())
	}
	if !strings.Contains(stdout.String(), "--model z-ai/glm-5.1") {
		t.Fatalf("stdout = %q, want planned --model", stdout.String())
	}
	if !strings.Contains(stderr.String(), "warning: worker opencode required env ZAI_API_KEY is missing") {
		t.Fatalf("stderr = %q, want planned profile missing env warning", stderr.String())
	}
}

func TestRunWorkerOpenCodeDryRunRejectsMissingModelStrategyProfile(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `model_strategy.preferred[0] profile "opencode-zai-glm-5-1" not found`) {
		t.Fatalf("stderr = %q, want missing model strategy profile error", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") {
		t.Fatalf("stdout = %q, want no command", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunRejectsModelStrategyMissingRequiredTag(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    tags:
      - general
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  require_tags:
    - coding
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `model_strategy.preferred[0] profile "opencode-zai-glm-5-1" missing required tag "coding"`) {
		t.Fatalf("stderr = %q, want missing required tag error", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") {
		t.Fatalf("stdout = %q, want no command", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunRejectsModelStrategyWorkerMismatch(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  codex-default:
    worker: codex
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - codex-default
  require_tags:
    - coding
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `model_strategy.preferred[0] profile "codex-default" worker "codex" does not match task worker "opencode"; automatic worker switching is not implemented`) {
		t.Fatalf("stderr = %q, want worker mismatch error", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") {
		t.Fatalf("stdout = %q, want no command", stdout.String())
	}
}

func TestRunWorkerCodexDryRunUsesWorkersConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(configPath, []byte("workers:\n  codex:\n    command: /usr/local/bin/codex\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}
	taskPath := writeTaskFile(t, "codex")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker",
		"codex",
		"dry-run",
		taskPath,
		"--workers-config",
		configPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "command: /usr/local/bin/codex exec --json --sandbox read-only --cd . -") {
		t.Fatalf("stdout = %q, want configured codex command", stdout.String())
	}
}

func TestRunWorkerOpenCodeRunUsesWorkersConfig(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	t.Setenv("ZAI_API_KEY", "zai-real-secret")

	tempDir := t.TempDir()
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{JSONLine: `{"type":"done"}`})
	configPath := filepath.Join(tempDir, "workers.yaml")
	if err := os.WriteFile(configPath, []byte(`workers:
  opencode:
    command: `+fakeOpenCode+`
    provider: z_ai_glm
    model: glm-5.1
    env:
      ZAI_API_KEY: required
`), 0o600); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	taskPath := writeTaskFile(t, "opencode")
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", storePath, "--artifacts-dir", artifactsDir, "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "run_id: run-cli-opencode-config-001") {
		t.Fatalf("stdout = %q, want configured opencode run id", stdout.String())
	}
	assertCLIFileContent(t, filepath.Join(artifactsDir, "run-cli-opencode-config-001", "stdout.jsonl"), "{\"type\":\"done\"}\n")
}

func TestRunWorkerOpenCodeRunFailsBeforeWorkerWhenRequiredEnvMissing(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	unsetEnvForTest(t, "ZAI_API_KEY")

	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "worker-executed")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{MarkerPath: markerPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
    env:
      ZAI_API_KEY: required
`)

	taskPath := writeTaskFile(t, "opencode")
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", storePath, "--artifacts-dir", artifactsDir, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "worker opencode missing required env: ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want missing env error", stderr.String())
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("worker marker exists or stat failed: %v", err)
	}
	if strings.Contains(stdout.String(), "run_id:") {
		t.Fatalf("stdout = %q, want no run output", stdout.String())
	}
}

func TestRunWorkerOpenCodeRunFailsBeforeWorkerWhenProfileRequiredEnvMissing(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	unsetEnvForTest(t, "ZAI_API_KEY")

	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "worker-executed")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{MarkerPath: markerPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
`)

	taskPath := writeTaskFileWithModelProfile(t, "opencode", "opencode-zai-glm-5-1")
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", storePath, "--artifacts-dir", artifactsDir, "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "worker opencode missing required env: ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want missing profile env error", stderr.String())
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("worker marker exists or stat failed: %v", err)
	}
	if strings.Contains(stdout.String(), "run_id:") {
		t.Fatalf("stdout = %q, want no run output", stdout.String())
	}
}

func TestRunWorkerOpenCodeRunWithProfileEnvPresentCallsOpenCodeRun(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	t.Setenv("ZAI_API_KEY", "dummy")

	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "opencode-args")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{ArgsPath: argsPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFileWithModelProfile(t, "opencode", "opencode-zai-glm-5-1")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", filepath.Join(tempDir, "deonclaw.db"), "--artifacts-dir", artifactsDir, "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "run_id: run-cli-opencode-config-001") {
		t.Fatalf("stdout = %q, want configured opencode run id", stdout.String())
	}
	if !strings.Contains(stdout.String(), "command: "+fakeOpenCode+" run --dir ") || !strings.Contains(stdout.String(), "--format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want masked command with --model", stdout.String())
	}
	if strings.Contains(stdout.String(), "Do not execute") {
		t.Fatalf("stdout = %q, want masked prompt", stdout.String())
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	if !strings.Contains(string(args), "--model z-ai/glm-5.1") {
		t.Fatalf("opencode args = %q, want --model", args)
	}
	runDir := filepath.Join(artifactsDir, "run-cli-opencode-config-001")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "# opencode run run-cli-opencode-config-001")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model profile: opencode-zai-glm-5-1")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Provider: z-ai")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model: glm-5.1")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model arg: z-ai/glm-5.1")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Execution trace: execution-trace.json")
	assertCLIFileContains(t, filepath.Join(runDir, "artifact-manifest.json"), "execution-trace.json")
	tracePath := filepath.Join(runDir, "execution-trace.json")
	traceJSON := string(mustReadCLIFile(t, tracePath))
	assertCLIFileContains(t, tracePath, `"model_profile": "opencode-zai-glm-5-1"`)
	assertCLIFileContains(t, tracePath, `"provider": "z-ai"`)
	assertCLIFileContains(t, tracePath, `"model": "glm-5.1"`)
	assertCLIFileContains(t, tracePath, `"model_arg": "z-ai/glm-5.1"`)
	assertCLIFileContains(t, tracePath, `--model z-ai/glm-5.1 <prompt>`)
	assertCLIFileContains(t, tracePath, `"prompt_sha256": "`)
	assertCLIFileContains(t, tracePath, `"name": "ZAI_API_KEY"`)
	assertCLIFileContains(t, tracePath, `"requirement": "required"`)
	assertCLIFileContains(t, tracePath, `"state": "set_masked"`)
	assertCLIFileContains(t, tracePath, `"stdout_format": "jsonl"`)
	assertCLIFileContains(t, tracePath, `"parsed_events": 1`)
	assertCLIFileContains(t, tracePath, `"parse_warnings": 0`)
	assertCLIFileContains(t, tracePath, `"validation_status": "skipped"`)
	assertCLIFileContains(t, tracePath, `"policy_status": "ok"`)
	assertCLIFileContains(t, tracePath, `"cleanup_action": "removed"`)
	if strings.Contains(traceJSON, "Do not execute unless validation passes") {
		t.Fatalf("execution trace leaked raw prompt: %s", traceJSON)
	}
	if strings.Contains(traceJSON, "dummy") {
		t.Fatalf("execution trace leaked env value: %s", traceJSON)
	}
	if strings.Contains(stdout.String()+stderr.String(), "dummy") {
		t.Fatalf("output leaked env value: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunWorkerOpenCodeRunWithModelStrategySelectsFirstPreferredProfile(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	t.Setenv("ZAI_API_KEY", "strategy-secret")
	unsetEnvForTest(t, "FALLBACK_API_KEY")

	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "opencode-args")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{ArgsPath: argsPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
  opencode-fast:
    worker: opencode
    provider: z-ai
    model: glm-5.1-mini
    model_arg: z-ai/glm-5.1-mini
    env:
      FALLBACK_API_KEY: required
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  fallback:
    - opencode-fast
  require_tags:
    - coding
  fallback_policy:
    enabled: false
    max_attempts: 1
    retry_on:
      - worker_failed
      - validation_failed
    never_retry_on:
      - policy_failed
      - memory_policy_failed
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", filepath.Join(tempDir, "deonclaw.db"), "--artifacts-dir", filepath.Join(tempDir, "artifacts"), "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	if !strings.Contains(string(args), "--model z-ai/glm-5.1") {
		t.Fatalf("opencode args = %q, want selected preferred --model", args)
	}
	if strings.Contains(string(args), "glm-5.1-mini") || strings.Contains(stdout.String(), "glm-5.1-mini") {
		t.Fatalf("fallback profile was used unexpectedly: stdout=%q args=%q", stdout.String(), string(args))
	}
	if !strings.Contains(stdout.String(), "--format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want masked selected strategy command", stdout.String())
	}
	runDir := filepath.Join(tempDir, "artifacts", "run-cli-opencode-config-001")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model profile: opencode-zai-glm-5-1")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model strategy: selected")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Selected model profile: opencode-zai-glm-5-1")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Provider: z-ai")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model: glm-5.1")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model arg: z-ai/glm-5.1")
	tracePath := filepath.Join(runDir, "execution-trace.json")
	traceJSON := string(mustReadCLIFile(t, tracePath))
	assertCLIFileContains(t, tracePath, `"model_strategy": "selected"`)
	assertCLIFileContains(t, tracePath, `"selected_model_profile": "opencode-zai-glm-5-1"`)
	assertCLIFileContains(t, tracePath, `"model_profile": "opencode-zai-glm-5-1"`)
	assertCLIFileContains(t, tracePath, `"model_arg": "z-ai/glm-5.1"`)
	assertCLIFileContains(t, tracePath, `--model z-ai/glm-5.1 <prompt>`)
	if strings.Contains(traceJSON, "strategy-secret") || strings.Contains(stdout.String()+stderr.String(), "strategy-secret") {
		t.Fatalf("secret leaked: stdout=%q stderr=%q trace=%s", stdout.String(), stderr.String(), traceJSON)
	}
}

func TestRunWorkerOpenCodeRunWithModelStrategyWithoutModelArgKeepsCommandAndRecordsProfile(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()

	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "opencode-args")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{ArgsPath: argsPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
model_profiles:
  opencode-default:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-default
  require_tags:
    - coding
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", filepath.Join(tempDir, "deonclaw.db"), "--artifacts-dir", filepath.Join(tempDir, "artifacts"), "--workers-config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	if strings.Contains(string(args), "--model") || strings.Contains(stdout.String(), "--model") {
		t.Fatalf("command used --model unexpectedly: stdout=%q args=%q", stdout.String(), string(args))
	}
	runDir := filepath.Join(tempDir, "artifacts", "run-cli-opencode-config-001")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model strategy: selected")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Selected model profile: opencode-default")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Provider: z-ai")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model: glm-5.1")
	tracePath := filepath.Join(runDir, "execution-trace.json")
	assertCLIFileContains(t, tracePath, `"selected_model_profile": "opencode-default"`)
	assertCLIFileContains(t, tracePath, `"provider": "z-ai"`)
	assertCLIFileContains(t, tracePath, `"model": "glm-5.1"`)
}

func TestRunWorkerOpenCodeRunWithModelStrategySelectedProfileMissingEnvFailsBeforeWorker(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	unsetEnvForTest(t, "ZAI_API_KEY")

	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "worker-executed")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{MarkerPath: markerPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  require_tags:
    - coding
`)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "run", taskPath, "--store", filepath.Join(tempDir, "deonclaw.db"), "--artifacts-dir", filepath.Join(tempDir, "artifacts"), "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "worker opencode missing required env: ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want selected profile missing env", stderr.String())
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("worker marker exists or stat failed: %v", err)
	}
}

func TestRunWorkerCodexRunFailsBeforeWorkerWhenRequiredEnvMissing(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	tempDir := t.TempDir()
	configPath := writeCLIWorkersConfig(t, `workers:
  codex:
    command: codex
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFile(t, "codex")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "codex", "run", taskPath, "--store", filepath.Join(tempDir, "deonclaw.db"), "--artifacts-dir", filepath.Join(tempDir, "artifacts"), "--workers-config", configPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "worker codex missing required env: ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want missing env error", stderr.String())
	}
	if strings.Contains(stdout.String(), "run_id:") {
		t.Fatalf("stdout = %q, want no run output", stdout.String())
	}
}

func TestRunWorkersSmokeDryRunWithMissingEnvWarnsAndDoesNotFail(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFile(t, "opencode")
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--workers-config", configPath,
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: worker opencode has missing required env") {
		t.Fatalf("stderr = %q, want sanitized missing env warning", stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "smoke_summary:") || !strings.Contains(output, "worker: opencode") {
		t.Fatalf("stdout = %q, want smoke summary", output)
	}
	if !strings.Contains(output, "env_required_ok: false") {
		t.Fatalf("stdout = %q, want env_required_ok false", output)
	}
	if !strings.Contains(output, "command: /usr/local/bin/opencode run --dir . --format json <prompt>") {
		t.Fatalf("stdout = %q, want planned command", output)
	}
	if strings.Contains(output, "model_strategy: planned") || strings.Contains(output, "planned_model_profile:") || strings.Contains(output, "--model") {
		t.Fatalf("stdout = %q, want current smoke dry-run behavior without strategy planning", output)
	}
	if !strings.Contains(output, "status: dry_run") {
		t.Fatalf("stdout = %q, want dry_run status", output)
	}
	if strings.Contains(output, "run_id:") {
		t.Fatalf("stdout = %q, want no run_id for dry-run", output)
	}
	assertNoSecretReference(t, stdout.String()+stderr.String())
}

func TestRunWorkersSmokeDryRunWithProfileMissingEnvWarnsAndDoesNotFail(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFileWithModelProfile(t, "opencode", "opencode-zai-glm-5-1")
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--workers-config", configPath,
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: worker opencode has missing required env") {
		t.Fatalf("stderr = %q, want sanitized missing env warning", stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "env_required_ok: false") || !strings.Contains(output, "status: dry_run") {
		t.Fatalf("stdout = %q, want dry-run env_missing summary", output)
	}
	if !strings.Contains(output, "command: /usr/local/bin/opencode run --dir . --format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want smoke command with --model", output)
	}
	assertNoSecretReference(t, stdout.String()+stderr.String())
}

func TestRunWorkersSmokeDryRunWithModelStrategyPlansFirstPreferredProfile(t *testing.T) {
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    tags:
      - coding
  opencode-fast:
    worker: opencode
    provider: z-ai
    model: glm-5.1-mini
    model_arg: z-ai/glm-5.1-mini
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  fallback:
    - opencode-fast
  require_tags:
    - coding
`)
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--workers-config", configPath,
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "model_strategy: planned") {
		t.Fatalf("stdout = %q, want planned model strategy", output)
	}
	if !strings.Contains(output, "planned_model_profile: opencode-zai-glm-5-1") {
		t.Fatalf("stdout = %q, want planned model profile", output)
	}
	if !strings.Contains(output, "provider: z-ai") {
		t.Fatalf("stdout = %q, want planned provider", output)
	}
	if !strings.Contains(output, "model: glm-5.1") {
		t.Fatalf("stdout = %q, want planned model", output)
	}
	if !strings.Contains(output, "model_arg: z-ai/glm-5.1") {
		t.Fatalf("stdout = %q, want planned model_arg", output)
	}
	if !strings.Contains(output, "command: /usr/local/bin/opencode run --dir . --format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want smoke dry-run command with planned --model", output)
	}
	if !strings.Contains(output, "env_required_ok: true") || !strings.Contains(output, "status: dry_run") {
		t.Fatalf("stdout = %q, want successful smoke dry-run summary", output)
	}
}

func TestRunWorkersSmokeDryRunWithModelStrategyMissingPlannedProfileEnvWarns(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: /usr/local/bin/opencode
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  require_tags:
    - coding
`)
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--workers-config", configPath,
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "warning: worker opencode has missing required env") {
		t.Fatalf("stderr = %q, want sanitized planned profile missing env warning", stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "planned_model_profile: opencode-zai-glm-5-1") {
		t.Fatalf("stdout = %q, want planned model profile", output)
	}
	if !strings.Contains(output, "command: /usr/local/bin/opencode run --dir . --format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want smoke dry-run command with planned --model", output)
	}
	if !strings.Contains(output, "env_required_ok: false") || !strings.Contains(output, "status: dry_run") {
		t.Fatalf("stdout = %q, want dry-run env warning summary", output)
	}
	assertNoSecretReference(t, stdout.String()+stderr.String())
}

func TestRunWorkersSmokeRunWithMissingEnvFailsBeforeWorker(t *testing.T) {
	unsetEnvForTest(t, "ZAI_API_KEY")
	tempDir := t.TempDir()
	markerPath := filepath.Join(tempDir, "worker-executed")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{MarkerPath: markerPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFile(t, "opencode")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--workers-config", configPath,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "error: worker opencode has missing required env") {
		t.Fatalf("stderr = %q, want sanitized missing env error", stderr.String())
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("worker marker exists or stat failed: %v", err)
	}
	output := stdout.String()
	if !strings.Contains(output, "env_required_ok: false") || !strings.Contains(output, "status: env_missing") {
		t.Fatalf("stdout = %q, want env_missing summary", output)
	}
	if strings.Contains(output, "run_id:") {
		t.Fatalf("stdout = %q, want no run output", output)
	}
	assertNoSecretReference(t, stdout.String()+stderr.String())
}

func TestRunWorkersSmokeRunWithEnvPresentCallsOpenCodeRun(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	t.Setenv("ZAI_API_KEY", "zai-real-secret")

	tempDir := t.TempDir()
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeTaskFile(t, "opencode")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", artifactsDir,
		"--workers-config", configPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "run_id: run-cli-opencode-config-001") {
		t.Fatalf("stdout = %q, want configured opencode run id", output)
	}
	if !strings.Contains(output, "env_required_ok: true") {
		t.Fatalf("stdout = %q, want env_required_ok true", output)
	}
	if !strings.Contains(output, "status: succeeded") {
		t.Fatalf("stdout = %q, want succeeded status", output)
	}
	if !strings.Contains(output, "artifacts_dir: "+filepath.Join(artifactsDir, "run-cli-opencode-config-001")) {
		t.Fatalf("stdout = %q, want artifacts dir", output)
	}
	assertCLIFileContent(t, filepath.Join(artifactsDir, "run-cli-opencode-config-001", "stdout.jsonl"), "{\"type\":\"done\"}\n")
	assertNoSecretReference(t, stdout.String()+stderr.String())
	if strings.Contains(stdout.String()+stderr.String(), "zai-real-secret") {
		t.Fatalf("output leaked env value: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunWorkersSmokeRunWithModelStrategySelectsFirstPreferredProfile(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	t.Setenv("ZAI_API_KEY", "smoke-strategy-secret")

	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "opencode-args")
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{ArgsPath: argsPath, JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
model_profiles:
  opencode-zai-glm-5-1:
    worker: opencode
    provider: z-ai
    model: glm-5.1
    model_arg: z-ai/glm-5.1
    env:
      ZAI_API_KEY: required
    tags:
      - coding
  opencode-fast:
    worker: opencode
    provider: z-ai
    model: glm-5.1-mini
    model_arg: z-ai/glm-5.1-mini
    tags:
      - coding
`)
	taskPath := writeTaskFileWithModelStrategy(t, "opencode", `  preferred:
    - opencode-zai-glm-5-1
  fallback:
    - opencode-fast
  require_tags:
    - coding
`)
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", artifactsDir,
		"--workers-config", configPath,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	if !strings.Contains(string(args), "--model z-ai/glm-5.1") {
		t.Fatalf("opencode args = %q, want selected preferred --model", args)
	}
	output := stdout.String()
	if !strings.Contains(output, "command: "+fakeOpenCode+" run --dir ") || !strings.Contains(output, "--format json --model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("stdout = %q, want smoke run selected strategy command", output)
	}
	if strings.Contains(output+string(args), "glm-5.1-mini") {
		t.Fatalf("fallback profile was used unexpectedly: stdout=%q args=%q", output, string(args))
	}
	runDir := filepath.Join(artifactsDir, "run-cli-opencode-config-001")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Model strategy: selected")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Selected model profile: opencode-zai-glm-5-1")
	assertCLIFileContains(t, filepath.Join(runDir, "execution-trace.json"), `"model_strategy": "selected"`)
	assertCLIFileContains(t, filepath.Join(runDir, "execution-trace.json"), `"selected_model_profile": "opencode-zai-glm-5-1"`)
	if strings.Contains(stdout.String()+stderr.String(), "smoke-strategy-secret") {
		t.Fatalf("output leaked env value: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestRunWorkersSmokeInvalidTaskFailsBeforeWorker(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai-real-secret")
	tempDir := t.TempDir()
	fakeOpenCode := writeCLIFakeOpenCode(t, tempDir, cliFakeOpenCodeOptions{JSONLine: `{"type":"done"}`})
	configPath := writeCLIWorkersConfig(t, `workers:
  opencode:
    command: `+fakeOpenCode+`
    env:
      ZAI_API_KEY: required
`)
	taskPath := writeInvalidTaskFile(t, "opencode")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"workers", "smoke",
		"--worker", "opencode",
		"--task", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--workers-config", configPath,
		"--dry-run",
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "validation failed") {
		t.Fatalf("stderr = %q, want validation failure", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") || strings.Contains(stdout.String(), "smoke_summary:") {
		t.Fatalf("stdout = %q, want no smoke command or summary for invalid task", stdout.String())
	}
	assertNoSecretReference(t, stdout.String()+stderr.String())
}

func TestRunWorkerOpenCodeDryRunRejectsInvalidTask(t *testing.T) {
	taskPath := writeInvalidTaskFile(t, "opencode")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"worker", "opencode", "dry-run", taskPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "validation failed") {
		t.Fatalf("stderr = %q, want validation failure", stderr.String())
	}
	if strings.Contains(stdout.String(), "command:") {
		t.Fatalf("stdout = %q, want no planned command for invalid task", stdout.String())
	}
}

func TestRunWorkerOpenCodeDryRunRejectsMismatchedTaskWorker(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"worker", "opencode", "dry-run", taskPath}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	want := `task worker "codex" does not match requested worker "opencode"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunWorkerCodexRunRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing store",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--artifacts-dir", t.TempDir()},
			want: "missing --store",
		},
		{
			name: "missing artifacts dir",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--store", filepath.Join(t.TempDir(), "deonclaw.db")},
			want: "missing --artifacts-dir",
		},
		{
			name: "unknown argument",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--store", "db.sqlite", "--artifacts-dir", "artifacts", "--unknown"},
			want: `unknown argument "--unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run() exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestParseCodexRunOptions(t *testing.T) {
	opts, err := parseCodexRunOptions([]string{"task.yaml", "--store", "deonclaw.db", "--artifacts-dir", "artifacts", "--domains", "domains.yaml", "--memory-policy", "memory-policy.yaml", "--workers-config", "workers.yaml", "--runtime-config", "runtime.yaml", "--validation-runtime", "docker", "--worker-runtime", "local"})
	if err != nil {
		t.Fatalf("parseCodexRunOptions() error = %v", err)
	}
	if opts.taskPath != "task.yaml" {
		t.Fatalf("taskPath = %q, want task.yaml", opts.taskPath)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
	if opts.domainsPath != "domains.yaml" {
		t.Fatalf("domainsPath = %q, want domains.yaml", opts.domainsPath)
	}
	if opts.memoryPolicyPath != "memory-policy.yaml" {
		t.Fatalf("memoryPolicyPath = %q, want memory-policy.yaml", opts.memoryPolicyPath)
	}
	if opts.workersConfigPath != "workers.yaml" {
		t.Fatalf("workersConfigPath = %q, want workers.yaml", opts.workersConfigPath)
	}
	if opts.runtimeConfigPath != "runtime.yaml" {
		t.Fatalf("runtimeConfigPath = %q, want runtime.yaml", opts.runtimeConfigPath)
	}
	if opts.validationRuntime != "docker" {
		t.Fatalf("validationRuntime = %q, want docker", opts.validationRuntime)
	}
	if opts.workerRuntime != "local" {
		t.Fatalf("workerRuntime = %q, want local", opts.workerRuntime)
	}
}

func TestParseOpenCodeRunOptions(t *testing.T) {
	opts, err := parseOpenCodeRunOptions([]string{"task.yaml", "--store", "deonclaw.db", "--artifacts-dir", "artifacts", "--domains", "domains.yaml", "--memory-policy", "memory-policy.yaml", "--workers-config", "workers.yaml", "--runtime-config", "runtime.yaml", "--validation-runtime", "docker", "--worker-runtime", "local"})
	if err != nil {
		t.Fatalf("parseOpenCodeRunOptions() error = %v", err)
	}
	if opts.taskPath != "task.yaml" {
		t.Fatalf("taskPath = %q, want task.yaml", opts.taskPath)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
	if opts.domainsPath != "domains.yaml" {
		t.Fatalf("domainsPath = %q, want domains.yaml", opts.domainsPath)
	}
	if opts.memoryPolicyPath != "memory-policy.yaml" {
		t.Fatalf("memoryPolicyPath = %q, want memory-policy.yaml", opts.memoryPolicyPath)
	}
	if opts.workersConfigPath != "workers.yaml" {
		t.Fatalf("workersConfigPath = %q, want workers.yaml", opts.workersConfigPath)
	}
	if opts.runtimeConfigPath != "runtime.yaml" {
		t.Fatalf("runtimeConfigPath = %q, want runtime.yaml", opts.runtimeConfigPath)
	}
	if opts.validationRuntime != "docker" {
		t.Fatalf("validationRuntime = %q, want docker", opts.validationRuntime)
	}
	if opts.workerRuntime != "local" {
		t.Fatalf("workerRuntime = %q, want local", opts.workerRuntime)
	}
}

func TestRunOpenCodeWorkerRuntimeDockerSmokeWithFakeDocker(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()

	rawPrompt := "RAW_PROMPT_SECRET_205"
	envSecret := "ENV_SECRET_205"
	t.Setenv("ZAI_API_KEY", envSecret)
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
printf '%s\n' '{"type":"message","text":"docker opencode ok"}'
printf '%s\n' 'docker opencode stderr' >&2
exit 0
`)
	taskPath := writeTaskFileWithGoal(t, "opencode", rawPrompt)
	configPath := writeCLIRuntimeConfig(t, runtimeConfigWithEnvPassthroughYAML("ZAI_API_KEY"))
	tempDir := t.TempDir()
	artifactsDir := filepath.Join(tempDir, "artifacts")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker", "opencode", "run", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", artifactsDir,
		"--runtime-config", configPath,
		"--worker-runtime", "docker",
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "run_id: run-cli-opencode-config-001") {
		t.Fatalf("stdout = %q, want configured opencode run id", stdout.String())
	}
	if !strings.Contains(stdout.String(), "command: docker run ") || !strings.Contains(stdout.String(), "opencode run --dir ") || !strings.Contains(stdout.String(), "--format json <prompt>") {
		t.Fatalf("stdout = %q, want masked docker opencode command", stdout.String())
	}
	if strings.Contains(stdout.String(), rawPrompt) || strings.Contains(stderr.String(), rawPrompt) {
		t.Fatalf("prompt leaked to CLI output; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), envSecret) || strings.Contains(stderr.String(), envSecret) {
		t.Fatalf("env value leaked to CLI output; stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	if !stringSliceContainsSequence(args, []string{"opencode", "run"}) || !stringSliceContainsSequence(args, []string{"--format", "json", rawPrompt}) {
		t.Fatalf("docker args = %#v, want opencode run with raw prompt arg", args)
	}
	if !stringSliceContainsSequence(args, []string{"--dir", "/workspace"}) {
		t.Fatalf("docker args = %#v, want container workspace dir", args)
	}
	workspaceMount := filepath.Join(artifactsDir, "run-cli-opencode-config-001", "workspace") + ":/workspace:rw"
	if !stringSliceContainsSequence(args, []string{"-v", workspaceMount}) {
		t.Fatalf("docker args = %#v, want prepared workspace mount %q", args, workspaceMount)
	}
	if !stringSliceContainsSequence(args, []string{"-v", "mysecondbrain:/memory/mysecondbrain:ro"}) || !stringSliceContainsSequence(args, []string{"-v", "escalasoft_brain:/memory/escalasoft_brain:ro"}) {
		t.Fatalf("docker args = %#v, want memory mounts preserved", args)
	}
	if !stringSliceContainsSequence(args, []string{"-e", "ZAI_API_KEY"}) {
		t.Fatalf("docker args = %#v, want env passthrough name", args)
	}
	if strings.Contains(strings.Join(args, " "), envSecret) {
		t.Fatalf("docker args leaked env value: %#v", args)
	}
	if strings.Contains(strings.Join(args, " "), "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}

	runDir := filepath.Join(artifactsDir, "run-cli-opencode-config-001")
	assertCLIFileContent(t, filepath.Join(runDir, "stdout.jsonl"), `{"type":"message","text":"docker opencode ok"}`+"\n")
	assertCLIFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"docker opencode ok"}`+"\n")
	assertCLIFileContent(t, filepath.Join(runDir, "stderr.log"), "docker opencode stderr\n")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Worker runtime: docker")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "Command: docker run")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "--dir /workspace")
	assertCLIFileContains(t, filepath.Join(runDir, "summary.md"), "--format json <prompt>")
	assertCLIFileContains(t, filepath.Join(runDir, "execution-trace.json"), `"worker_runtime": "docker"`)
	assertCLIFileContains(t, filepath.Join(runDir, "execution-trace.json"), `"command_display": "docker run`)
	assertCLIFileContains(t, filepath.Join(runDir, "execution-trace.json"), `--dir /workspace`)
	assertCLIFileContains(t, filepath.Join(runDir, "execution-trace.json"), `<prompt>`)
	for _, artifact := range []string{"summary.md", "execution-trace.json", "stdout.jsonl", "events.jsonl", "stderr.log"} {
		assertCLIFileNotContains(t, filepath.Join(runDir, artifact), rawPrompt)
		assertCLIFileNotContains(t, filepath.Join(runDir, artifact), envSecret)
	}
}

func TestRunOpenCodeWorkerRuntimeDockerMissingEnvFailsBeforeDocker(t *testing.T) {
	restore := overrideOpenCodeRunConfigDeps(t)
	defer restore()
	unsetEnvForTest(t, "ZAI_API_KEY")
	argsPath := installFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	taskPath := writeTaskFile(t, "opencode")
	configPath := writeCLIRuntimeConfig(t, runtimeConfigWithEnvPassthroughYAML("ZAI_API_KEY"))
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker", "opencode", "run", taskPath,
		"--store", filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir", filepath.Join(tempDir, "artifacts"),
		"--runtime-config", configPath,
		"--worker-runtime", "docker",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want missing env", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertFileEmptyOrMissing(t, argsPath)
}

func TestRunCodexWorkerRuntimeDockerRemainsBlocked(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")
	configPath := writeCLIRuntimeConfig(t, validRuntimeConfigYAML())
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker", "codex", "run", taskPath,
		"--store", filepath.Join(t.TempDir(), "deonclaw.db"),
		"--artifacts-dir", filepath.Join(t.TempDir(), "artifacts"),
		"--runtime-config", configPath,
		"--worker-runtime", "docker",
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Codex Docker worker runtime is not implemented; OpenCode Docker is experimental") {
		t.Fatalf("stderr = %q, want codex docker block", stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

func TestDocsMarkOpenCodeDockerSupportedExperimental(t *testing.T) {
	parity, err := os.ReadFile(examplePath(t, "docs", "WORKER_PARITY.md"))
	if err != nil {
		t.Fatalf("ReadFile(WORKER_PARITY) error = %v", err)
	}
	parityText := string(parity)
	for _, want := range []string{
		"supported_experimental",
		"| worker_runtime docker | blocked | supported_experimental |",
		"Codex Docker remains blocked",
	} {
		if !strings.Contains(parityText, want) {
			t.Fatalf("WORKER_PARITY missing %q", want)
		}
	}

	dockerRuntime, err := os.ReadFile(examplePath(t, "docs", "DOCKER_RUNTIME.md"))
	if err != nil {
		t.Fatalf("ReadFile(DOCKER_RUNTIME) error = %v", err)
	}
	dockerRuntimeText := string(dockerRuntime)
	for _, want := range []string{
		"OpenCode Docker worker runtime is now supported experimental",
		"Codex Docker worker runtime remains blocked",
		"prompt-as-argument contract",
		"configs/examples/runtime-opencode-zai-smoke.yaml",
	} {
		if !strings.Contains(dockerRuntimeText, want) {
			t.Fatalf("DOCKER_RUNTIME missing %q", want)
		}
	}

	eventContract, err := os.ReadFile(examplePath(t, "docs", "WORKER_EVENT_CONTRACT.md"))
	if err != nil {
		t.Fatalf("ReadFile(WORKER_EVENT_CONTRACT) error = %v", err)
	}
	eventContractText := string(eventContract)
	for _, want := range []string{
		"OpenCode Docker worker runtime marked `supported_experimental`",
		"Codex Docker remains blocked",
		"supported experimental, not production hardening",
	} {
		if !strings.Contains(eventContractText, want) {
			t.Fatalf("WORKER_EVENT_CONTRACT missing %q", want)
		}
	}
}

func TestRunArtifactsPruneDryRun(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	artifactPath := filepath.Join(artifactsDir, "run-old", "summary.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("summary\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Task",
		Domain: "general",
		Worker: "codex",
		Goal:   "Test prune",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"dry-run lists artifacts"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	runRecord := &runs.Run{
		ID:            "run-old",
		TaskID:        task.ID,
		Status:        runs.StatusSucceeded,
		Worker:        "codex",
		WorkspacePath: "workspace",
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	if err := db.SaveArtifact(ctx, &artifacts.Artifact{
		ID:        "artifact-old",
		RunID:     runRecord.ID,
		Path:      artifactPath,
		Kind:      artifacts.KindSummary,
		SizeBytes: 8,
		CreatedAt: oldTime,
	}); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"artifacts",
		"prune",
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
		"--older-than",
		"30d",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{"dry_run: true", "older_than: 30d", "candidates: 1", "deleted: 0", "skipped: 0"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("artifact file was removed in dry-run: %v", err)
	}

	db, err = storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	gotArtifacts, err := db.ArtifactsByRun(ctx, runRecord.ID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	if len(gotArtifacts) != 1 {
		t.Fatalf("len(artifacts) = %d, want 1 after dry-run", len(gotArtifacts))
	}
}

func TestRunArtifactsList(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Task",
		Domain: "general",
		Worker: "codex",
		Goal:   "List artifacts",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"artifacts are listed"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	createdAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	runRecords := []runs.Run{
		{ID: "run-succeeded", TaskID: task.ID, Status: runs.StatusSucceeded, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "run-failed", TaskID: task.ID, Status: runs.StatusFailed, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	for i := range runRecords {
		if err := db.SaveRun(ctx, &runRecords[i]); err != nil {
			t.Fatalf("SaveRun() error = %v", err)
		}
	}
	artifactRecords := []artifacts.Artifact{
		{
			ID:        "artifact-summary",
			RunID:     "run-succeeded",
			Path:      filepath.Join("artifacts", "run-succeeded", "summary.md"),
			Kind:      artifacts.KindSummary,
			SizeBytes: 12,
			SHA256:    strings.Repeat("a", 64),
			CreatedAt: createdAt,
		},
		{
			ID:        "artifact-stderr",
			RunID:     "run-failed",
			Path:      filepath.Join("artifacts", "run-failed", "stderr.log"),
			Kind:      artifacts.KindLog,
			SizeBytes: 9,
			SHA256:    strings.Repeat("b", 64),
			Keep:      true,
			CreatedAt: createdAt.Add(time.Second),
		},
	}
	for i := range artifactRecords {
		if err := db.SaveArtifact(ctx, &artifactRecords[i]); err != nil {
			t.Fatalf("SaveArtifact() error = %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"artifacts", "list", "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"artifact_id", "run_id", "kind", "path", "size_bytes", "sha256", "keep", "created_at", "artifact-summary", "artifact-stderr"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"artifacts", "list", "--store", storePath, "--run", "run-succeeded"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--run) exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "artifact-summary") || strings.Contains(stdout.String(), "artifact-stderr") {
		t.Fatalf("stdout with --run = %q, want only artifact-summary", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"artifacts", "list", "--store", storePath, "--status", string(runs.StatusFailed)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--status) exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "artifact-stderr") || strings.Contains(stdout.String(), "artifact-summary") {
		t.Fatalf("stdout with --status = %q, want only artifact-stderr", stdout.String())
	}
}

func TestRunRunsReportLegacyText(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	saveCLIRunsReportTask(t, ctx, db)
	createdAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	saveCLIRunsReportRun(t, ctx, db, "run-legacy", runs.StatusSucceeded, "codex", createdAt)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"runs", "report", "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"runs_report:", "total_runs: 1", "succeeded: 1", "legacy_runs: 1", "by_worker:", "codex"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestRunRunsReportJSONByModelProfile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	saveCLIRunsReportTask(t, ctx, db)
	createdAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	saveCLIRunsReportRun(t, ctx, db, "run-profile", runs.StatusFailed, "opencode", createdAt)
	saveCLIRunsReportTrace(t, ctx, db, tempDir, "run-profile", "{\n  \"worker\": \"opencode\",\n  \"model_profile\": \"opencode-zai-glm-5-1\",\n  \"duration_ms\": 120,\n  \"parsed_events\": 3,\n  \"parse_warnings\": 2,\n  \"validation_status\": \"failed\",\n  \"changed_paths_count\": 5\n}")
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"runs", "report", "--store", storePath, "--by", "model_profile", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	var decoded struct {
		TotalRuns        int `json:"total_runs"`
		Failed           int `json:"failed"`
		ValidationFailed int `json:"validation_failed"`
		ByModelProfile   []struct {
			Group      string `json:"group"`
			TotalRuns  int    `json:"total_runs"`
			Failed     int    `json:"failed"`
			LegacyRuns int    `json:"legacy_runs"`
		} `json:"by_model_profile"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%s", err, stdout.String())
	}
	if decoded.TotalRuns != 1 || decoded.Failed != 1 || decoded.ValidationFailed != 1 {
		t.Fatalf("decoded = %#v, want failed validation profile report", decoded)
	}
	if len(decoded.ByModelProfile) != 1 || decoded.ByModelProfile[0].Group != "opencode-zai-glm-5-1" || decoded.ByModelProfile[0].TotalRuns != 1 || decoded.ByModelProfile[0].Failed != 1 || decoded.ByModelProfile[0].LegacyRuns != 0 {
		t.Fatalf("by_model_profile = %#v, want one opencode profile group", decoded.ByModelProfile)
	}
}

func TestRunRunsReportFilters(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	saveCLIRunsReportTask(t, ctx, db)
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	saveCLIRunsReportRun(t, ctx, db, "run-old", runs.StatusSucceeded, "opencode", base)
	saveCLIRunsReportRun(t, ctx, db, "run-codex", runs.StatusSucceeded, "codex", base.Add(24*time.Hour))
	saveCLIRunsReportRun(t, ctx, db, "run-failed", runs.StatusFailed, "opencode", base.Add(48*time.Hour))
	saveCLIRunsReportRun(t, ctx, db, "run-want", runs.StatusSucceeded, "opencode", base.Add(72*time.Hour))
	for _, runID := range []string{"run-old", "run-codex", "run-failed", "run-want"} {
		saveCLIRunsReportTrace(t, ctx, db, tempDir, runID, "{\n  \"worker\": \"opencode\",\n  \"model_profile\": \"opencode-zai-glm-5-1\",\n  \"duration_ms\": 100,\n  \"parsed_events\": 1,\n  \"parse_warnings\": 0,\n  \"validation_status\": \"passed\",\n  \"changed_paths_count\": 1\n}")
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"runs", "report", "--store", storePath, "--by", "model_profile", "--worker", "opencode", "--status", "succeeded", "--since", "2026-06-03", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	var decoded struct {
		TotalRuns int `json:"total_runs"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; stdout=%s", err, stdout.String())
	}
	if decoded.TotalRuns != 1 {
		t.Fatalf("decoded.TotalRuns = %d, want 1", decoded.TotalRuns)
	}
}

func TestRunMCPProposalsListEmpty(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	saveCLIRunsReportTask(t, ctx, db)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mcp", "proposals", "list", "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "(empty)") {
		t.Fatalf("stdout = %q, want empty proposals list", stdout.String())
	}
}

func TestRunMCPProposalsListShowExportJSON(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	saveCLIMCPProposalsTask(t, ctx, db, "task-mcp-cli-001")
	saveCLIMCPProposalsRun(t, ctx, db, "run-mcp-cli-001", "task-mcp-cli-001", "codex")
	proposalPath := writeCLIMCPProposalArtifact(t, tempDir, "run-mcp-cli-001")
	saveCLIProposalArtifactRecord(t, ctx, db, "run-mcp-cli-001", tempDir, "mcp-tool-call-proposal.json", string(readCLIFile(t, proposalPath)))
	saveCLIProposalArtifactRecord(t, ctx, db, "run-mcp-cli-001", tempDir, "mcp-tool-call-proposal-lint.json", `{
  "proposal_id": "mcp-call-cli-001",
  "status": "passed",
  "violations": [],
  "warnings": []
}`)
	saveCLIProposalArtifactRecord(t, ctx, db, "run-mcp-cli-001", tempDir, "mcp-tool-call-preflight.json", `{
  "proposal_id": "mcp-call-cli-001",
  "status": "passed",
  "warnings": [],
  "failures": []
}`)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var listStdout bytes.Buffer
	var listStderr bytes.Buffer
	listCode := run([]string{"mcp", "proposals", "list", "--store", storePath, "--output-format", "json"}, &listStdout, &listStderr)
	if listCode != 0 {
		t.Fatalf("list exit code = %d, stderr = %q", listCode, listStderr.String())
	}
	if !json.Valid(listStdout.Bytes()) {
		t.Fatalf("list stdout is not valid JSON: %s", listStdout.String())
	}
	if strings.Contains(listStdout.String(), "super-secret-cli-argument") {
		t.Fatalf("list leaked raw arguments: %s", listStdout.String())
	}

	var showStdout bytes.Buffer
	var showStderr bytes.Buffer
	showCode := run([]string{"mcp", "proposals", "show", "--store", storePath, "--run", "run-mcp-cli-001", "--output-format", "json"}, &showStdout, &showStderr)
	if showCode != 0 {
		t.Fatalf("show exit code = %d, stderr = %q", showCode, showStderr.String())
	}
	if !json.Valid(showStdout.Bytes()) {
		t.Fatalf("show stdout is not valid JSON: %s", showStdout.String())
	}
	if strings.Contains(showStdout.String(), "super-secret-cli-argument") || strings.Contains(showStdout.String(), `"arguments"`) {
		t.Fatalf("show leaked raw arguments: %s", showStdout.String())
	}
	if !strings.Contains(showStdout.String(), "arguments_sha256") || !strings.Contains(showStdout.String(), "mcp-call-cli-001") {
		t.Fatalf("show stdout = %s, want sanitized proposal metadata", showStdout.String())
	}
	if !strings.Contains(showStdout.String(), "mcp proposal approve") || !strings.Contains(showStdout.String(), "mcp proposal execute") {
		t.Fatalf("show stdout = %s, want suggested commands", showStdout.String())
	}
	if !strings.Contains(showStdout.String(), `"ready_for_approval": true`) {
		t.Fatalf("show stdout = %s, want ready_for_approval true", showStdout.String())
	}

	exportPath := filepath.Join(tempDir, "exported.json")
	var exportStdout bytes.Buffer
	var exportStderr bytes.Buffer
	exportCode := run([]string{"mcp", "proposals", "export", "--store", storePath, "--run", "run-mcp-cli-001", "--output", exportPath}, &exportStdout, &exportStderr)
	if exportCode != 0 {
		t.Fatalf("export exit code = %d, stderr = %q", exportCode, exportStderr.String())
	}
	if string(readCLIFile(t, exportPath)) != string(readCLIFile(t, proposalPath)) {
		t.Fatalf("exported proposal differs from source artifact")
	}
	if !strings.Contains(exportStdout.String(), "exported_proposal_sha256:") {
		t.Fatalf("export stdout = %s, want exported_proposal_sha256", exportStdout.String())
	}
	if !strings.Contains(exportStdout.String(), "next_step_hint:") {
		t.Fatalf("export stdout = %s, want next_step_hint", exportStdout.String())
	}
}

func TestRunMCPProposalsListRefusedApproval(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	saveCLIMCPProposalsTask(t, ctx, db, "task-mcp-refused-001")
	saveCLIMCPProposalsRun(t, ctx, db, "run-mcp-refused-001", "task-mcp-refused-001", "codex")
	saveCLIProposalArtifactRecord(t, ctx, db, "run-mcp-refused-001", tempDir, "mcp-tool-call-approval.json", `{"decision":"approved"}`)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"mcp", "proposals", "list", "--store", storePath, "--status", "refused", "--output-format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"proposal_status": "refused"`) {
		t.Fatalf("stdout = %s, want refused status", stdout.String())
	}
}

func saveCLIMCPProposalsTask(t *testing.T, ctx context.Context, db storepkg.Store, taskID string) {
	t.Helper()
	task := &tasks.Task{
		ID:     taskID,
		Title:  "MCP proposals CLI task",
		Domain: "general",
		Worker: "codex",
		Goal:   "queue cli test",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"done"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
}

func saveCLIMCPProposalsRun(t *testing.T, ctx context.Context, db storepkg.Store, runID string, taskID string, worker string) {
	t.Helper()
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	runRecord := &runs.Run{
		ID:            runID,
		TaskID:        taskID,
		Status:        runs.StatusFailed,
		Worker:        worker,
		WorkspacePath: "workspace",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
}

func writeCLIMCPProposalArtifact(t *testing.T, root string, runID string) string {
	t.Helper()
	proposal, err := mcpapproval.NewProposal(mcpapproval.NewProposalOptions{
		ID:          "mcp-call-cli-001",
		Server:      "fake-stdio",
		Tool:        "deonclaw.fake.echo",
		Arguments:   []byte(`{"text":"super-secret-cli-argument"}`),
		Reason:      "worker suggestion",
		RequestedBy: "worker",
		PolicyPath:  "configs/examples/mcp-call-policy-fake.yaml",
		ConfigPath:  "configs/examples/mcp-fake.yaml",
		Runtime:     "local",
	})
	if err != nil {
		t.Fatalf("NewProposal() error = %v", err)
	}
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	path := filepath.Join(root, "artifacts", runID, "mcp-tool-call-proposal.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func saveCLIProposalArtifactRecord(t *testing.T, ctx context.Context, db storepkg.Store, runID string, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, "artifacts", runID, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	artifact := &artifacts.Artifact{
		ID:        runID + "-" + name,
		RunID:     runID,
		Path:      path,
		Kind:      artifacts.KindOther,
		SizeBytes: int64(len(content)),
		SHA256:    strings.Repeat("b", 64),
		CreatedAt: time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
	}
	if err := db.SaveArtifact(ctx, artifact); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
}

func readCLIFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func TestParseArtifactPruneOptions(t *testing.T) {
	opts, err := parseArtifactPruneOptions([]string{"--store", "deonclaw.db", "--artifacts-dir", "artifacts", "--older-than", "30d", "--dry-run"})
	if err != nil {
		t.Fatalf("parseArtifactPruneOptions() error = %v", err)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
	if opts.olderThan != 30*24*time.Hour {
		t.Fatalf("olderThan = %s, want 720h", opts.olderThan)
	}
	if !opts.dryRun {
		t.Fatal("dryRun = false, want true")
	}
}

func writeTaskFile(t *testing.T, worker string) string {
	t.Helper()

	return writeTaskFileWithGoal(t, worker, "Do not execute")
}

func writeTaskFileWithGoal(t *testing.T, worker string, goal string) string {
	t.Helper()

	content := `id: worker-mismatch-001
title: "Worker mismatch"
domain: general
worker: ` + worker + `
goal: "` + goal + `"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeTaskFileWithModelProfile(t *testing.T, worker string, modelProfile string) string {
	t.Helper()

	content := `id: worker-profile-001
title: "Worker profile"
domain: general
worker: ` + worker + `
model_profile: ` + modelProfile + `
goal: "Do not execute unless validation passes"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command validates profile
`
	path := filepath.Join(t.TempDir(), "task-with-profile.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeTaskFileWithModelStrategy(t *testing.T, worker string, modelStrategy string) string {
	t.Helper()

	content := `id: worker-strategy-001
title: "Worker strategy"
domain: general
worker: ` + worker + `
model_strategy:
` + modelStrategy + `goal: "Do not execute unless validation passes"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command validates strategy
`
	path := filepath.Join(t.TempDir(), "task-with-strategy.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeCLIWorkersConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "workers.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write workers config file: %v", err)
	}
	return path
}

func saveCLIRunsReportTask(t *testing.T, ctx context.Context, db storepkg.Store) {
	t.Helper()
	task := &tasks.Task{
		ID:     "task-runs-report-001",
		Title:  "Runs report task",
		Domain: "general",
		Worker: "opencode",
		Goal:   "Do not execute",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"runs report works"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
}

func saveCLIRunsReportRun(t *testing.T, ctx context.Context, db storepkg.Store, id string, status runs.RunStatus, worker string, createdAt time.Time) {
	t.Helper()
	runRecord := &runs.Run{
		ID:            id,
		TaskID:        "task-runs-report-001",
		Status:        status,
		Worker:        worker,
		WorkspacePath: "workspace",
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		t.Fatalf("SaveRun(%q) error = %v", id, err)
	}
}

func saveCLIRunsReportTrace(t *testing.T, ctx context.Context, db storepkg.Store, root string, runID string, traceJSON string) {
	t.Helper()
	tracePath := filepath.Join(root, "artifacts", runID, "execution-trace.json")
	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(tracePath, []byte(traceJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	artifact := &artifacts.Artifact{
		ID:        runID + "-trace",
		RunID:     runID,
		Path:      tracePath,
		Kind:      artifacts.KindOther,
		SizeBytes: int64(len(traceJSON)),
		SHA256:    strings.Repeat("a", 64),
		CreatedAt: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
	}
	if err := db.SaveArtifact(ctx, artifact); err != nil {
		t.Fatalf("SaveArtifact(%q) error = %v", artifact.ID, err)
	}
}

func unsetEnvForTest(t *testing.T, name string) {
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

func assertNoSecretReference(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "ZAI_API_KEY") {
		t.Fatalf("output leaked env name: %q", output)
	}
}

func writeInvalidTaskFile(t *testing.T, worker string) string {
	t.Helper()

	content := `id: invalid-task-001
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "invalid-task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write invalid task file: %v", err)
	}
	return path
}

func writeDomainsConfigFile(t *testing.T) string {
	t.Helper()

	content := `domains:
  general:
    type: canonical_memory
    root: /vault/mysecondbrain
    default: true
    read_only_for_workers: true

  escalasoft:
    type: isolated_domain
    root: /domains/escalasoft_brain
    default: false
    read_only_for_general_agents: true
    bridge_files:
      - /vault/mysecondbrain/memory/context/escalasoft-operacao.md
    structured_data:
      historical_sqlite: /data/escalasoft-sqlite/escalasoft_docs.db
    staging:
      - /tmp/drive-escalasoft-docs
    default_agent: escalasoft-agent
`
	path := filepath.Join(t.TempDir(), "domains.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write domains config file: %v", err)
	}
	return path
}

func writeMemoryProposalFile(t *testing.T, dir string, proposal memory.MemoryProposal) string {
	t.Helper()

	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	path := filepath.Join(dir, "memory-proposal.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write memory proposal file: %v", err)
	}
	return path
}

func writeRawJSONFile(t *testing.T, dir string, name string, value any) string {
	t.Helper()

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("MarshalIndent() error = %v", err)
	}
	data = append(data, '\n')

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write raw json file: %v", err)
	}
	return path
}

func writeMemoryApprovalFile(t *testing.T, dir string, approval memory.MemoryApproval) string {
	t.Helper()

	data, err := approval.JSON()
	if err != nil {
		t.Fatalf("approval.JSON() error = %v", err)
	}
	path := filepath.Join(dir, memory.ApprovalJSONArtifactName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write memory approval file: %v", err)
	}
	return path
}

func readBackupPlanFile(t *testing.T, path string) memory.BackupPlan {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(backup plan) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("backup plan output is invalid JSON: %s", data)
	}
	var plan memory.BackupPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatalf("Unmarshal(backup plan) error = %v", err)
	}
	return plan
}

func writeBackupPlanFile(t *testing.T, dir string, plan memory.BackupPlan) string {
	t.Helper()

	data, err := plan.JSON()
	if err != nil {
		t.Fatalf("plan.JSON() error = %v", err)
	}
	path := filepath.Join(dir, memory.BackupPlanJSONArtifactName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write backup plan file: %v", err)
	}
	return path
}

func readBackupResultFile(t *testing.T, path string) memory.BackupResult {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(backup result) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("backup result output is invalid JSON: %s", data)
	}
	var result memory.BackupResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal(backup result) error = %v", err)
	}
	return result
}

func writeBackupResultFile(t *testing.T, dir string, result memory.BackupResult) string {
	t.Helper()

	data, err := result.JSON()
	if err != nil {
		t.Fatalf("result.JSON() error = %v", err)
	}
	path := filepath.Join(dir, memory.BackupResultJSONArtifactName)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write backup result file: %v", err)
	}
	return path
}

func readApplyResultFile(t *testing.T, path string) memory.ApplyResult {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(apply result) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("apply result output is invalid JSON: %s", data)
	}
	var result memory.ApplyResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal(apply result) error = %v", err)
	}
	return result
}

func overrideOpenCodeRunDeps(t *testing.T, result *workers.RunResult, runErr error) func() {
	t.Helper()
	oldWorkerFactory := opencodeWorkerFactory
	oldRunIDFactory := runIDFactory
	oldGitDiffRunner := gitDiffRunner
	oldGitSnapshotRunner := gitSnapshotRunner
	oldWorkspaceManagerFactory := workspaceManagerFactory

	opencodeWorkerFactory = func() workers.Worker {
		return cliFakeWorker{runResult: result, runErr: runErr}
	}
	runIDFactory = func() string {
		return "run-cli-opencode-001"
	}
	gitDiffRunner = func(context.Context, string) ([]byte, error) {
		return nil, nil
	}
	callCount := 0
	gitSnapshotRunner = func(context.Context, string) (*git.Snapshot, error) {
		callCount++
		return &git.Snapshot{}, nil
	}
	workspaceManagerFactory = func() workspacePreparer {
		return cliFakeWorkspacePreparer{}
	}

	return func() {
		opencodeWorkerFactory = oldWorkerFactory
		runIDFactory = oldRunIDFactory
		gitDiffRunner = oldGitDiffRunner
		gitSnapshotRunner = oldGitSnapshotRunner
		workspaceManagerFactory = oldWorkspaceManagerFactory
	}
}

func overrideOpenCodeRunConfigDeps(t *testing.T) func() {
	t.Helper()
	oldRunIDFactory := runIDFactory
	oldGitDiffRunner := gitDiffRunner
	oldGitSnapshotRunner := gitSnapshotRunner
	oldWorkspaceManagerFactory := workspaceManagerFactory

	runIDFactory = func() string {
		return "run-cli-opencode-config-001"
	}
	gitDiffRunner = func(context.Context, string) ([]byte, error) {
		return nil, nil
	}
	gitSnapshotRunner = func(context.Context, string) (*git.Snapshot, error) {
		return &git.Snapshot{}, nil
	}
	workspaceManagerFactory = func() workspacePreparer {
		return cliCreatingWorkspacePreparer{}
	}

	return func() {
		runIDFactory = oldRunIDFactory
		gitDiffRunner = oldGitDiffRunner
		gitSnapshotRunner = oldGitSnapshotRunner
		workspaceManagerFactory = oldWorkspaceManagerFactory
	}
}

type cliFakeWorker struct {
	runResult *workers.RunResult
	runErr    error
}

func (f cliFakeWorker) DryRun(context.Context, workers.RunSpec) (*workers.WorkerEvent, error) {
	return nil, nil
}

func (f cliFakeWorker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	result := f.runResult
	if result != nil {
		copy := *result
		copy.Workspace = spec.Workspace
		if len(copy.Command) == 5 {
			copy.Command[3] = spec.Workspace
		}
		return &copy, f.runErr
	}
	return nil, f.runErr
}

type cliFakeWorkspacePreparer struct{}

func (cliFakeWorkspacePreparer) Prepare(ctx context.Context, spec runtime.WorkspaceSpec) (*runtime.Workspace, error) {
	workspacePath := filepath.Join(spec.RootDir, spec.RunID, "workspace")
	return &runtime.Workspace{
		Path:       workspacePath,
		SourcePath: spec.SourcePath,
		Method:     runtime.MethodGitWorktree,
	}, nil
}

func (cliFakeWorkspacePreparer) Cleanup(context.Context, *runtime.Workspace) error {
	return nil
}

type cliCreatingWorkspacePreparer struct{}

func (cliCreatingWorkspacePreparer) Prepare(ctx context.Context, spec runtime.WorkspaceSpec) (*runtime.Workspace, error) {
	workspacePath := filepath.Join(spec.RootDir, spec.RunID, "workspace")
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return nil, err
	}
	return &runtime.Workspace{
		Path:       workspacePath,
		SourcePath: spec.SourcePath,
		Method:     runtime.MethodGitWorktree,
	}, nil
}

func (cliCreatingWorkspacePreparer) Cleanup(context.Context, *runtime.Workspace) error {
	return nil
}

func readRestorePreviewFile(t *testing.T, path string) memory.RestorePreview {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(restore preview) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("restore preview output is invalid JSON: %s", data)
	}
	var preview memory.RestorePreview
	if err := json.Unmarshal(data, &preview); err != nil {
		t.Fatalf("Unmarshal(restore preview) error = %v", err)
	}
	return preview
}

func readRestoreResultFile(t *testing.T, path string) memory.RestoreResult {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(restore result) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("restore result output is invalid JSON: %s", data)
	}
	var result memory.RestoreResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal(restore result) error = %v", err)
	}
	return result
}

func runBackupPlan(t *testing.T, proposalPath string, approvalPath string, outputPath string, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	return run([]string{
		"memory",
		"proposal",
		"backup-plan",
		"--proposal",
		proposalPath,
		"--approval",
		approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--output",
		outputPath,
	}, stdout, stderr)
}

func runBackupMaterialize(t *testing.T, backupPlanPath string, outputPath string, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	return run([]string{
		"memory",
		"proposal",
		"backup-materialize",
		"--backup-plan",
		backupPlanPath,
		"--output",
		outputPath,
	}, stdout, stderr)
}

type cliRestoreArtifacts struct {
	backupPlanPath     string
	backupResultPath   string
	restorePreviewPath string
	backupPlan         memory.BackupPlan
}

func buildCLIRestoreArtifacts(t *testing.T, tempDir string, targetPath string, operation memory.MemoryOperation, initialContent string) cliRestoreArtifacts {
	t.Helper()
	if initialContent != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(target dir) error = %v", err)
		}
		if err := os.WriteFile(targetPath, []byte(initialContent), 0o600); err != nil {
			t.Fatalf("WriteFile(target) error = %v", err)
		}
	}
	plan := buildCLIBackupPlan(t, tempDir, targetPath, operation)
	backupResult, err := memory.MaterializeBackup(plan, memory.NewBackupMaterializeOptions{})
	if err != nil {
		t.Fatalf("MaterializeBackup() error = %v", err)
	}
	restorePreview, err := memory.BuildRestorePreview(plan, backupResult, memory.NewRestorePreviewOptions{
		CreatedAt: time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildRestorePreview() error = %v", err)
	}
	return cliRestoreArtifacts{
		backupPlanPath:     writeBackupPlanFile(t, tempDir, plan),
		backupResultPath:   writeBackupResultFile(t, tempDir, backupResult),
		restorePreviewPath: writeRestorePreviewFile(t, tempDir, restorePreview),
		backupPlan:         plan,
	}
}

func writeRestorePreviewFile(t *testing.T, tempDir string, preview memory.RestorePreview) string {
	t.Helper()
	path := filepath.Join(tempDir, "restore-preview-input.json")
	data, err := preview.JSON()
	if err != nil {
		t.Fatalf("restore preview JSON: %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write restore preview file: %v", err)
	}
	return path
}

func runRestoreDryRun(t *testing.T, artifacts cliRestoreArtifacts, outputPath string, dryRun bool, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	args := []string{
		"memory",
		"proposal",
		"restore",
		"--backup-plan",
		artifacts.backupPlanPath,
		"--backup-result",
		artifacts.backupResultPath,
		"--output",
		outputPath,
	}
	if dryRun {
		args = append(args, "--dry-run")
	}
	return run(args, stdout, stderr)
}

func runRestoreExecute(t *testing.T, artifacts cliRestoreArtifacts, outputPath string, confirm bool, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	args := []string{
		"memory",
		"proposal",
		"restore-execute",
		"--backup-plan",
		artifacts.backupPlanPath,
		"--backup-result",
		artifacts.backupResultPath,
		"--restore-preview",
		artifacts.restorePreviewPath,
		"--output",
		outputPath,
	}
	if confirm {
		args = append(args, "--confirm-restore")
	}
	return run(args, stdout, stderr)
}

type cliApplyExecuteArtifacts struct {
	proposalPath     string
	approvalPath     string
	backupPlanPath   string
	backupResultPath string
	backupPlan       memory.BackupPlan
}

func buildCLIApplyExecuteArtifacts(t *testing.T, tempDir string, targetPath string, operation memory.MemoryOperation, initialContent string, patchContent string) cliApplyExecuteArtifacts {
	t.Helper()
	if initialContent != "" {
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatalf("MkdirAll(target dir) error = %v", err)
		}
		if err := os.WriteFile(targetPath, []byte(initialContent), 0o600); err != nil {
			t.Fatalf("WriteFile(target) error = %v", err)
		}
	}
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-apply-execute-" + string(operation),
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Apply execute.",
		CreatedAt:  time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: operation, Content: patchContent},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	plan, err := memory.BuildBackupPlan(proposal, approval, policy, memory.NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	backupResult, err := memory.MaterializeBackup(plan, memory.NewBackupMaterializeOptions{})
	if err != nil {
		t.Fatalf("MaterializeBackup() error = %v", err)
	}
	return cliApplyExecuteArtifacts{
		proposalPath:     writeMemoryProposalFile(t, tempDir, proposal),
		approvalPath:     writeMemoryApprovalFile(t, tempDir, approval),
		backupPlanPath:   writeBackupPlanFile(t, tempDir, plan),
		backupResultPath: writeBackupResultFile(t, tempDir, backupResult),
		backupPlan:       plan,
	}
}

func runApplyExecute(t *testing.T, artifacts cliApplyExecuteArtifacts, outputPath string, confirm bool, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	args := []string{
		"memory",
		"proposal",
		"apply-execute",
		"--proposal",
		artifacts.proposalPath,
		"--approval",
		artifacts.approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
		"--backup-plan",
		artifacts.backupPlanPath,
		"--backup-result",
		artifacts.backupResultPath,
		"--output",
		outputPath,
	}
	if confirm {
		args = append(args, "--confirm-apply")
	}
	return run(args, stdout, stderr)
}

func buildCLIBackupPlan(t *testing.T, tempDir string, targetPath string, operation memory.MemoryOperation) memory.BackupPlan {
	t.Helper()
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: "mem-cli-backup-materialize-" + string(operation),
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Backup materialize.",
		CreatedAt:  time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: operation, Content: "new content\n"},
		},
	})
	policy := loadCLIExamplePolicy(t)
	approval := buildCLIMemoryApproval(t, proposal, policy, memory.DecisionApproved)
	plan, err := memory.BuildBackupPlan(proposal, approval, policy, memory.NewBackupPlanOptions{BackupRoot: filepath.Join(tempDir, "backups")})
	if err != nil {
		t.Fatalf("BuildBackupPlan() error = %v", err)
	}
	return plan
}

func mustReadCLIFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func assertCLIFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got := mustReadCLIFile(t, path)
	if string(got) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertCLIFileContains(t *testing.T, path string, want string) {
	t.Helper()
	got := mustReadCLIFile(t, path)
	if !strings.Contains(string(got), want) {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertCLIFileNotContains(t *testing.T, path string, want string) {
	t.Helper()
	got := mustReadCLIFile(t, path)
	if strings.Contains(string(got), want) {
		t.Fatalf("ReadFile(%q) = %q, did not want %q", path, got, want)
	}
}

func readApplyPreflightFile(t *testing.T, path string) memory.ApplyPreflight {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(preflight) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("preflight output is invalid JSON: %s", data)
	}
	var preflight memory.ApplyPreflight
	if err := json.Unmarshal(data, &preflight); err != nil {
		t.Fatalf("Unmarshal(preflight) error = %v", err)
	}
	return preflight
}

func readMemoryApprovalFile(t *testing.T, path string) memory.MemoryApproval {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(approval) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("approval output is invalid JSON: %s", data)
	}
	var approval memory.MemoryApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		t.Fatalf("Unmarshal(approval) error = %v", err)
	}
	return approval
}

func readMCPProposalFile(t *testing.T, path string) mcpapproval.MCPToolCallProposal {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(proposal) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("proposal output is invalid JSON: %s", data)
	}
	var proposal mcpapproval.MCPToolCallProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		t.Fatalf("Unmarshal(proposal) error = %v", err)
	}
	return proposal
}

func readMCPPreflightFile(t *testing.T, path string) mcpapproval.MCPToolCallPreflight {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(preflight) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("preflight output is invalid JSON: %s", data)
	}
	var preflight mcpapproval.MCPToolCallPreflight
	if err := json.Unmarshal(data, &preflight); err != nil {
		t.Fatalf("Unmarshal(preflight) error = %v", err)
	}
	return preflight
}

func readMCPApprovalFile(t *testing.T, path string) mcpapproval.MCPToolCallApproval {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(approval) error = %v", err)
	}
	if !json.Valid(data) {
		t.Fatalf("approval output is invalid JSON: %s", data)
	}
	var approval mcpapproval.MCPToolCallApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		t.Fatalf("Unmarshal(approval) error = %v", err)
	}
	return approval
}

func newCLIMCPProposal(t *testing.T, server string, arguments []byte, policyPath string, runtimeConfigPath string) mcpapproval.MCPToolCallProposal {
	t.Helper()
	return newCLIMCPProposalWithConfig(t, server, arguments, policyPath, "", runtimeConfigPath)
}

func newCLIMCPProposalWithConfig(t *testing.T, server string, arguments []byte, policyPath string, configPath string, runtimeConfigPath string) mcpapproval.MCPToolCallProposal {
	t.Helper()
	proposal, err := mcpapproval.NewProposal(mcpapproval.NewProposalOptions{
		ID:                "mcp-call-cli-test",
		CreatedAt:         time.Date(2026, 6, 16, 21, 30, 0, 0, time.UTC),
		Server:            server,
		Tool:              "deonclaw.fake.echo",
		Arguments:         arguments,
		Reason:            "CLI read-only approval test.",
		RequestedBy:       "davi",
		PolicyPath:        policyPath,
		ConfigPath:        configPath,
		Runtime:           "docker",
		RuntimeConfigPath: runtimeConfigPath,
		Workspace:         ".",
	})
	if err != nil {
		t.Fatalf("NewProposal() error = %v", err)
	}
	return proposal
}

func newCLIMCPApproval(t *testing.T, proposal mcpapproval.MCPToolCallProposal, policyPath string, decision string) mcpapproval.MCPToolCallApproval {
	t.Helper()
	approval, err := mcpapproval.BuildApproval(proposal, mcpapproval.NewApprovalOptions{
		Decision:        decision,
		ApprovedAt:      time.Date(2026, 6, 16, 21, 35, 0, 0, time.UTC),
		ApprovedBy:      "davi",
		Reason:          "CLI approval test.",
		PolicyPath:      policyPath,
		ConfirmReadOnly: true,
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	return approval
}

func writeMCPProposalFile(t *testing.T, dir string, proposal mcpapproval.MCPToolCallProposal) string {
	t.Helper()
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	file, err := os.CreateTemp(dir, "mcp-proposal-*.json")
	if err != nil {
		t.Fatalf("CreateTemp(MCP proposal) error = %v", err)
	}
	path := file.Name()
	if _, err := file.Write(data); err != nil {
		t.Fatalf("write MCP proposal file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close MCP proposal file: %v", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod MCP proposal file: %v", err)
	}
	return path
}

func writeMCPApprovalFile(t *testing.T, dir string, approval mcpapproval.MCPToolCallApproval, name string) string {
	t.Helper()
	data, err := approval.JSON()
	if err != nil {
		t.Fatalf("approval.JSON() error = %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write MCP approval file: %v", err)
	}
	return path
}

func runMCPProposalExecuteCLI(t *testing.T, proposalPath string, approvalPath string, configPath string, policyPath string, artifactsDir string, stdout io.Writer, stderr io.Writer) int {
	t.Helper()
	return run([]string{
		"mcp", "proposal", "execute",
		"--proposal", proposalPath,
		"--approval", approvalPath,
		"--config", configPath,
		"--policy", policyPath,
		"--artifacts-dir", artifactsDir,
		"--timeout-seconds", "3",
		"--confirm-execute",
	}, stdout, stderr)
}

func writeApprovalTestProposal(t *testing.T, dir string, id string) string {
	t.Helper()
	return writeMemoryProposalFile(t, dir, memory.NewProposal(memory.NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: filepath.Join(dir, "target.md"),
		Operation:  memory.OperationAppend,
		Reason:     "Approval test proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 12, 15, 0, 0, time.UTC),
		Patches:    []memory.MemoryPatch{{TargetPath: filepath.Join(dir, "target.md"), Operation: memory.OperationAppend, Content: "content\n"}},
	}))
}

func testCLIPreflightProposal(t *testing.T, dir string, id string) memory.MemoryProposal {
	t.Helper()
	targetPath := filepath.Join(dir, "target.md")
	return memory.NewProposal(memory.NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     "general",
		TargetPath: targetPath,
		Operation:  memory.OperationAppend,
		Reason:     "CLI preflight proposal.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 0, 0, 0, time.UTC),
		Patches: []memory.MemoryPatch{
			{TargetPath: targetPath, Operation: memory.OperationAppend, Content: "content\n"},
		},
	})
}

func loadCLIExamplePolicy(t *testing.T) *memory.MemoryPolicy {
	t.Helper()
	policy, err := memory.LoadPolicyFromFile(filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"))
	if err != nil {
		t.Fatalf("LoadPolicyFromFile() error = %v", err)
	}
	return policy
}

func installFakeDocker(t *testing.T, script string) string {
	t.Helper()
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "docker.args")
	behavior := cliFakeDockerBehaviorFromScript(script)
	behavior.argsPath = argsPath
	writeCLIFakeCommand(t, tempDir, "docker", behavior)
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func installCLIMCPFakeDocker(t *testing.T, mode string, stderr string) string {
	t.Helper()
	tempDir := t.TempDir()
	argsPath := filepath.Join(tempDir, "docker.args")
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_ARGS_PATH", argsPath)
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_MODE", mode)
	t.Setenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_STDERR", stderr)

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
		content = fmt.Sprintf("@echo off\r\nset DEONCLAW_CLI_MCP_FAKE_DOCKER_HELPER=1\r\n\"%s\" -test.run=TestCLIMCPFakeDockerHelperProcess -- %%*\r\nexit /b %%ERRORLEVEL%%\r\n", testBinary)
	} else {
		content = "#!/bin/sh\nDEONCLAW_CLI_MCP_FAKE_DOCKER_HELPER=1 exec " + cliShellQuote(testBinary) + " -test.run=TestCLIMCPFakeDockerHelperProcess -- \"$@\"\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(fake docker) error = %v", err)
	}
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func runCLIMCPFakeDockerHelper() int {
	if argsPath := os.Getenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_ARGS_PATH"); argsPath != "" {
		content := strings.Join(cliHelperArgs(), "\n")
		if content != "" {
			content += "\n"
		}
		_ = os.WriteFile(argsPath, []byte(content), 0o600)
	}
	if stderr := os.Getenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_STDERR"); stderr != "" {
		_, _ = fmt.Fprint(os.Stderr, stderr)
	}
	switch os.Getenv("DEONCLAW_CLI_MCP_FAKE_DOCKER_MODE") {
	case "hang":
		time.Sleep(10 * time.Second)
		return 0
	default:
		return runMCPFakeServer(os.Stdin, os.Stdout, os.Stderr)
	}
}

type cliFakeOpenCodeOptions struct {
	ArgsPath   string
	MarkerPath string
	JSONLine   string
}

type cliFakeCommandBehavior struct {
	argsPath   string
	argsFormat string
	stdinPath  string
	markerPath string
	stdout     string
	stderr     string
	exitCode   int
	sleepMS    int
}

func writeCLIPathFakeExecutable(t *testing.T, dir string, name string, exitCode int) string {
	t.Helper()
	return writeCLIFakeCommand(t, dir, name, cliFakeCommandBehavior{exitCode: exitCode})
}

func writeCLIFakeOpenCode(t *testing.T, dir string, opts cliFakeOpenCodeOptions) string {
	t.Helper()
	stdout := ""
	if opts.JSONLine != "" {
		stdout = opts.JSONLine + "\n"
	}
	return writeCLIFakeCommand(t, dir, "fake-opencode", cliFakeCommandBehavior{
		argsPath:   opts.ArgsPath,
		argsFormat: "space",
		markerPath: opts.MarkerPath,
		stdout:     stdout,
	})
}

func writeCLIFakeCommand(t *testing.T, dir string, name string, behavior cliFakeCommandBehavior) string {
	t.Helper()
	t.Setenv("DEONCLAW_FAKE_ARGS_PATH", behavior.argsPath)
	t.Setenv("DEONCLAW_FAKE_ARGS_FORMAT", behavior.argsFormat)
	t.Setenv("DEONCLAW_FAKE_STDIN_PATH", behavior.stdinPath)
	t.Setenv("DEONCLAW_FAKE_MARKER_PATH", behavior.markerPath)
	t.Setenv("DEONCLAW_FAKE_STDOUT", behavior.stdout)
	t.Setenv("DEONCLAW_FAKE_STDERR", behavior.stderr)
	t.Setenv("DEONCLAW_FAKE_EXIT_CODE", strconv.Itoa(behavior.exitCode))
	t.Setenv("DEONCLAW_FAKE_SLEEP_MS", strconv.Itoa(behavior.sleepMS))

	path := filepath.Join(dir, name)
	if stdruntime.GOOS == "windows" {
		path += ".bat"
	}
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("Abs(test binary) error = %v", err)
	}

	var content string
	if stdruntime.GOOS == "windows" {
		content = fmt.Sprintf("@echo off\r\nset DEONCLAW_CLI_FAKE_HELPER=1\r\n\"%s\" -test.run=TestCLIFakeCommandHelperProcess -- %%*\r\nexit /b %%ERRORLEVEL%%\r\n", testBinary)
	} else {
		content = "#!/bin/sh\nDEONCLAW_CLI_FAKE_HELPER=1 exec " + cliShellQuote(testBinary) + " -test.run=TestCLIFakeCommandHelperProcess -- \"$@\"\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(fake command) error = %v", err)
	}
	return path
}

func TestCLIFakeCommandHelperProcess(t *testing.T) {
	if os.Getenv("DEONCLAW_CLI_FAKE_HELPER") != "1" {
		return
	}
	os.Exit(runCLIFakeCommandHelper())
}

func runCLIFakeCommandHelper() int {
	if sleepMS, err := strconv.Atoi(os.Getenv("DEONCLAW_FAKE_SLEEP_MS")); err == nil && sleepMS > 0 {
		time.Sleep(time.Duration(sleepMS) * time.Millisecond)
	}
	args := cliHelperArgs()
	if argsPath := os.Getenv("DEONCLAW_FAKE_ARGS_PATH"); argsPath != "" {
		content := strings.Join(args, "\n")
		if os.Getenv("DEONCLAW_FAKE_ARGS_FORMAT") == "space" {
			content = strings.Join(args, " ")
		}
		if content != "" {
			content += "\n"
		}
		_ = os.WriteFile(argsPath, []byte(content), 0o600)
	}
	if stdinPath := os.Getenv("DEONCLAW_FAKE_STDIN_PATH"); stdinPath != "" {
		data, _ := io.ReadAll(os.Stdin)
		_ = os.WriteFile(stdinPath, data, 0o600)
	}
	if markerPath := os.Getenv("DEONCLAW_FAKE_MARKER_PATH"); markerPath != "" {
		_ = os.WriteFile(markerPath, []byte{}, 0o600)
	}
	_, _ = fmt.Fprint(os.Stdout, os.Getenv("DEONCLAW_FAKE_STDOUT"))
	_, _ = fmt.Fprint(os.Stderr, os.Getenv("DEONCLAW_FAKE_STDERR"))
	exitCode, err := strconv.Atoi(os.Getenv("DEONCLAW_FAKE_EXIT_CODE"))
	if err != nil {
		return 0
	}
	return exitCode
}

func cliHelperArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1:]
		}
	}
	return nil
}

func cliFakeDockerBehaviorFromScript(script string) cliFakeCommandBehavior {
	behavior := cliFakeCommandBehavior{argsFormat: "lines", exitCode: 0}
	if strings.Contains(script, "DEONCLAW_FAKE_DOCKER_STDIN") {
		behavior.stdinPath = os.Getenv("DEONCLAW_FAKE_DOCKER_STDIN")
	}
	if strings.Contains(script, "sleep 2") {
		behavior.sleepMS = 2000
	}
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "exit ") {
			fields := strings.Fields(trimmed)
			if len(fields) == 2 {
				if exitCode, err := strconv.Atoi(fields[1]); err == nil {
					behavior.exitCode = exitCode
				}
			}
		}
		if strings.Contains(trimmed, ">&2") {
			behavior.stderr += cliFakeDockerOutputForLine(trimmed)
			continue
		}
		if strings.Contains(trimmed, "DEONCLAW_FAKE_DOCKER_ARGS") || strings.Contains(trimmed, "DEONCLAW_FAKE_DOCKER_STDIN") {
			continue
		}
		behavior.stdout += cliFakeDockerOutputForLine(trimmed)
	}
	return behavior
}

func cliFakeDockerOutputForLine(line string) string {
	switch {
	case strings.Contains(line, `{"type":"message","text":"docker opencode ok"}`):
		return `{"type":"message","text":"docker opencode ok"}` + "\n"
	case strings.Contains(line, `{"type":"message","text":"docker worker ok"}`):
		return `{"type":"message","text":"docker worker ok"}` + "\n"
	case strings.Contains(line, "docker opencode stderr"):
		return "docker opencode stderr\n"
	case strings.Contains(line, "docker worker stderr"):
		return "docker worker stderr\n"
	case strings.Contains(line, "docker validation stdout"):
		return "docker validation stdout\n"
	case strings.Contains(line, "docker validation stderr"):
		return "docker validation stderr\n"
	case strings.Contains(line, "docker stdout"):
		return "docker stdout\n"
	case strings.Contains(line, "docker stderr"):
		return "docker stderr\n"
	case strings.Contains(line, "bad stdout"):
		return "bad stdout\n"
	case strings.Contains(line, "bad stderr"):
		return "bad stderr\n"
	default:
		return ""
	}
}

func cliShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func assertFileEmptyOrMissing(t *testing.T, path string) {
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

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func writeCLIRuntimeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}
	return path
}

func writeCLIMCPConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write mcp config: %v", err)
	}
	return path
}

func writeCLIMCPFakeConfig(t *testing.T, env []string, mode string, testOnly bool, capabilities []string) string {
	t.Helper()
	if len(capabilities) == 0 {
		capabilities = []string{"read"}
	}
	var builder strings.Builder
	builder.WriteString("mcp:\n  servers:\n    fake-stdio:\n")
	builder.WriteString("      command: " + strconv.Quote(os.Args[0]) + "\n")
	builder.WriteString("      args:\n")
	builder.WriteString("        - \"-test.run=TestCLIMCPFakeServerHelperProcess\"\n")
	builder.WriteString("        - \"--\"\n")
	builder.WriteString("        - " + strconv.Quote(mode) + "\n")
	builder.WriteString("      enabled: false\n")
	builder.WriteString(fmt.Sprintf("      test_only: %t\n", testOnly))
	builder.WriteString("      protocol: stdio\n")
	builder.WriteString("      trust: local\n")
	builder.WriteString("      capabilities:\n")
	for _, capability := range capabilities {
		builder.WriteString("        - " + strconv.Quote(capability) + "\n")
	}
	builder.WriteString("      env:\n")
	builder.WriteString("        passthrough:\n")
	for _, name := range env {
		builder.WriteString("          - " + strconv.Quote(name) + "\n")
	}
	return writeCLIMCPConfig(t, builder.String())
}

func writeCLIMCPToolPolicy(t *testing.T, servers []string, tools []string, capabilities []string) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("mcp_tool_policy:\n")
	builder.WriteString("  allow_test_only: true\n")
	builder.WriteString("  max_tool_calls: 1\n")
	builder.WriteString("  allowed_servers:\n")
	for _, server := range servers {
		builder.WriteString("    - " + strconv.Quote(server) + "\n")
	}
	builder.WriteString("  allowed_tools:\n")
	for _, tool := range tools {
		builder.WriteString("    - " + strconv.Quote(tool) + "\n")
	}
	builder.WriteString("  allowed_capabilities:\n")
	for _, capability := range capabilities {
		builder.WriteString("    - " + strconv.Quote(capability) + "\n")
	}
	path := filepath.Join(t.TempDir(), "mcp-tool-policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("WriteFile(tool policy) error = %v", err)
	}
	return path
}

func writeCLIMCPDiscoveryPolicy(t *testing.T, servers []string, capabilities []string, requireDocker bool) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("mcp_discovery_policy:\n")
	builder.WriteString("  allow_real_readonly: true\n")
	builder.WriteString("  max_tool_calls: 0\n")
	builder.WriteString("  allowed_servers:\n")
	for _, server := range servers {
		builder.WriteString("    - " + strconv.Quote(server) + "\n")
	}
	builder.WriteString("  allowed_capabilities:\n")
	for _, capability := range capabilities {
		builder.WriteString("    - " + strconv.Quote(capability) + "\n")
	}
	builder.WriteString(fmt.Sprintf("  require_docker_for_real: %t\n", requireDocker))
	path := filepath.Join(t.TempDir(), "mcp-discovery-policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("WriteFile(discovery policy) error = %v", err)
	}
	return path
}

func writeCLIMCPCallPolicy(t *testing.T, servers []string, tools []string, capabilities []string, maxArgumentsBytes int, maxResponseBytes int, requireDocker bool) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("mcp_call_policy:\n")
	builder.WriteString("  allow_real_readonly: true\n")
	builder.WriteString(fmt.Sprintf("  require_docker_for_real: %t\n", requireDocker))
	builder.WriteString("  max_tool_calls: 1\n")
	builder.WriteString("  allowed_servers:\n")
	for _, server := range servers {
		builder.WriteString("    - " + strconv.Quote(server) + "\n")
	}
	builder.WriteString("  allowed_tools:\n")
	for _, tool := range tools {
		builder.WriteString("    - " + strconv.Quote(tool) + "\n")
	}
	builder.WriteString("  allowed_capabilities:\n")
	for _, capability := range capabilities {
		builder.WriteString("    - " + strconv.Quote(capability) + "\n")
	}
	builder.WriteString(fmt.Sprintf("  max_arguments_bytes: %d\n", maxArgumentsBytes))
	builder.WriteString(fmt.Sprintf("  max_response_bytes: %d\n", maxResponseBytes))
	path := filepath.Join(t.TempDir(), "mcp-call-policy.yaml")
	if err := os.WriteFile(path, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("WriteFile(call policy) error = %v", err)
	}
	return path
}

func assertCLITranscriptJSONLValid(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		t.Fatalf("transcript %s is empty", path)
	}
	for _, line := range lines {
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
	return string(data)
}

func assertCLITranscriptRequestMethodCount(t *testing.T, content string, method string, want int) {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(content), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
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
	if count != want {
		t.Fatalf("request method %s count = %d, want %d in %q", method, count, want, content)
	}
}

func readCLIDockerArgs(t *testing.T, path string) []string {
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

func indexOfString(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

func examplePath(t *testing.T, parts ...string) string {
	t.Helper()
	items := append([]string{"..", ".."}, parts...)
	path := filepath.Join(items...)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("example path %q: %v", path, err)
	}
	return path
}

func validRuntimeConfigYAML() string {
	return `runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    memory_limit: 2g
    cpus: "2"
    mounts:
      - source: .
        target: /workspace
        mode: rw
      - source: mysecondbrain
        target: /memory/mysecondbrain
        mode: ro
      - source: escalasoft_brain
        target: /memory/escalasoft_brain
        mode: ro
`
}

func runtimeConfigWithEnvPassthroughYAML(name string) string {
	return `runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    memory_limit: 2g
    cpus: "2"
    env:
      passthrough:
        - ` + name + `
    mounts:
      - source: .
        target: /workspace
        mode: rw
      - source: mysecondbrain
        target: /memory/mysecondbrain
        mode: ro
      - source: escalasoft_brain
        target: /memory/escalasoft_brain
        mode: ro
`
}

func buildCLIMemoryApproval(t *testing.T, proposal memory.MemoryProposal, policy *memory.MemoryPolicy, decision memory.ApprovalDecision) memory.MemoryApproval {
	t.Helper()
	approval, err := memory.BuildApproval(proposal, policy, memory.NewApprovalOptions{
		ApprovalID: "approval-" + proposal.ProposalID,
		Reviewer:   "Davi",
		Decision:   decision,
		Reason:     "Approval for preflight.",
		CreatedAt:  time.Date(2026, 5, 22, 14, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("BuildApproval() error = %v", err)
	}
	return approval
}

func runApplyPreflight(t *testing.T, proposalPath string, approvalPath string, outputPath string, stdout *bytes.Buffer, stderr *bytes.Buffer) int {
	t.Helper()
	args := []string{
		"memory",
		"proposal",
		"apply-preflight",
		"--proposal",
		proposalPath,
		"--approval",
		approvalPath,
		"--policy",
		filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}
	if outputPath != "" {
		args = append(args, "--output", outputPath)
	}
	return run(args, stdout, stderr)
}

func TestRunRetrievalContextGovernanceFixtureE2E(t *testing.T) {
	root := retrievalContextFixtureRoot(t)
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	fixtureSrc := filepath.Join(root, "configs", "examples", "retrieval-context-fixture")
	for _, name := range []string{"retrieval-context.json", "memory-index-chunks.jsonl", "retrieval-injection-policy.yaml"} {
		data, err := os.ReadFile(filepath.Join(fixtureSrc, name))
		if err != nil {
			t.Fatalf("ReadFile(%q) error = %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}

	const (
		retrievalContextPath         = "retrieval-context.json"
		chunksPath                   = "memory-index-chunks.jsonl"
		materializedPath             = "retrieval-context-materialized.json"
		materializedSummary          = "retrieval-context-materialized.md"
		bundlePath                   = "retrieval-context-bundle.json"
		bundleSummary                = "retrieval-context-bundle.md"
		requestPath                  = "retrieval-context-approval-request.json"
		approvalPath                 = "retrieval-context-approval.json"
		injectionPlanPath            = "retrieval-context-injection-plan.json"
		injectionPlanSummary         = "retrieval-context-injection-plan.md"
		injectionPolicyPath          = "retrieval-injection-policy.yaml"
		governanceReportPath         = "retrieval-context-governance-report.json"
		injectionApprovalRequestPath = "retrieval-context-injection-approval-request.json"
		injectionApprovalPath        = "retrieval-context-injection-approval.json"
	)

	steps := [][]string{
		{"retrieval", "context", "inspect", "--artifact", retrievalContextPath},
		{
			"retrieval", "context", "materialize",
			"--retrieval-context", retrievalContextPath,
			"--chunks", chunksPath,
			"--output", materializedPath,
			"--summary", materializedSummary,
			"--max-chars-per-chunk", "1200",
			"--max-total-chars", "6000",
			"--confirm-include-chunk-text",
		},
		{"retrieval", "context", "materialized-report", "--artifact", materializedPath},
		{
			"retrieval", "context", "bundle",
			"--retrieval-context", retrievalContextPath,
			"--materialized", materializedPath,
			"--output", bundlePath,
			"--summary", bundleSummary,
		},
		{"retrieval", "context", "approval", "new", "--bundle", bundlePath, "--output", requestPath},
		{
			"retrieval", "context", "approval", "approve",
			"--request", requestPath,
			"--output", approvalPath,
			"--confirm-approve-materialized-context",
		},
		{"retrieval", "context", "approval", "inspect", "--approval", approvalPath},
		{
			"retrieval", "context", "injection-plan",
			"--approval", approvalPath,
			"--request", requestPath,
			"--bundle", bundlePath,
			"--materialized", materializedPath,
			"--output", injectionPlanPath,
			"--summary", injectionPlanSummary,
		},
		{
			"retrieval", "context", "governance-report",
			"--retrieval-context", retrievalContextPath,
			"--materialized", materializedPath,
			"--bundle", bundlePath,
			"--request", requestPath,
			"--approval", approvalPath,
			"--injection-plan", injectionPlanPath,
		},
		{"retrieval", "context", "injection-policy", "validate", "--policy", injectionPolicyPath},
		{"retrieval", "context", "injection-policy", "plan", "--policy", injectionPolicyPath, "--output-format", "json"},
	}

	var injectionPolicyPlanJSON string
	for i, step := range steps {
		var stdout, stderr bytes.Buffer
		if code := run(step, &stdout, &stderr); code != 0 {
			t.Fatalf("step %d %v exit=%d stderr=%q stdout=%q", i+1, step, code, stderr.String(), stdout.String())
		}
		if len(step) >= 5 && step[2] == "injection-policy" && step[3] == "plan" {
			injectionPolicyPlanJSON = stdout.String()
		}
	}

	injectionPlanData := readFixtureFileString(t, filepath.Join(dir, injectionPlanPath))
	if strings.Contains(injectionPlanData, `"can_inject_now": true`) {
		t.Fatal("injection plan must keep can_inject_now false")
	}
	if !strings.Contains(injectionPlanData, retrievalcontext.RequiredFutureInjectFlag) {
		t.Fatalf("injection plan missing %q", retrievalcontext.RequiredFutureInjectFlag)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"retrieval", "context", "governance-report",
		"--retrieval-context", retrievalContextPath,
		"--materialized", materializedPath,
		"--bundle", bundlePath,
		"--request", requestPath,
		"--approval", approvalPath,
		"--injection-plan", injectionPlanPath,
		"--output-format", "json",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("governance-report json exit=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"status": "ok"`) {
		t.Fatalf("governance stdout = %q, want status ok", stdout.String())
	}
	if strings.Contains(stdout.String(), "text_excerpt") {
		t.Fatal("governance json must not contain text_excerpt")
	}
	if strings.Contains(stdout.String(), `"can_inject_now": true`) {
		t.Fatal("governance report must keep can_inject_now false")
	}

	for _, path := range []string{
		retrievalContextPath,
		bundlePath,
		bundleSummary,
		requestPath,
		approvalPath,
		injectionPlanPath,
		injectionPlanSummary,
	} {
		if strings.Contains(readFixtureFileString(t, filepath.Join(dir, path)), "text_excerpt") {
			t.Fatalf("%s must not contain text_excerpt", path)
		}
	}
	materializedData := readFixtureFileString(t, filepath.Join(dir, materializedPath))
	if !strings.Contains(materializedData, "text_excerpt") {
		t.Fatal("materialized artifact should contain text_excerpt")
	}

	if !strings.Contains(injectionPolicyPlanJSON, `"would_inject": false`) {
		t.Fatalf("injection policy plan json = %q, want would_inject false", injectionPolicyPlanJSON)
	}
	if !strings.Contains(injectionPolicyPlanJSON, "runner_injection_allowed") {
		t.Fatalf("injection policy plan json = %q, want runner_injection_allowed warning", injectionPolicyPlanJSON)
	}
	if strings.Contains(injectionPolicyPlanJSON, "text_excerpt") {
		t.Fatal("injection policy plan json must not contain text_excerpt")
	}

	if err := os.WriteFile(filepath.Join(dir, governanceReportPath), []byte(stdout.String()), 0o644); err != nil {
		t.Fatalf("WriteFile(governance report) error = %v", err)
	}

	injectionApprovalSteps := [][]string{
		{
			"retrieval", "context", "injection-approval", "new",
			"--governance-report", governanceReportPath,
			"--policy", injectionPolicyPath,
			"--output", injectionApprovalRequestPath,
		},
		{
			"retrieval", "context", "injection-approval", "approve",
			"--request", injectionApprovalRequestPath,
			"--output", injectionApprovalPath,
			"--confirm-allow-runner-injection",
		},
		{
			"retrieval", "context", "injection-approval", "inspect",
			"--approval", injectionApprovalPath,
			"--request", injectionApprovalRequestPath,
			"--output-format", "json",
		},
	}
	var injectionApprovalInspectJSON string
	for i, step := range injectionApprovalSteps {
		var stepStdout, stepStderr bytes.Buffer
		if code := run(step, &stepStdout, &stepStderr); code != 0 {
			t.Fatalf("injection approval step %d %v exit=%d stderr=%q stdout=%q", i+1, step, code, stepStderr.String(), stepStdout.String())
		}
		if len(step) >= 5 && step[3] == "inspect" {
			injectionApprovalInspectJSON = stepStdout.String()
		}
	}
	if !strings.Contains(injectionApprovalInspectJSON, `"runner_injection_allowed": true`) {
		t.Fatalf("injection approval inspect json = %q, want runner_injection_allowed true", injectionApprovalInspectJSON)
	}
	if !strings.Contains(injectionApprovalInspectJSON, retrievalcontext.AllowedUseRunnerInjectionPolicyOnly) {
		t.Fatalf("injection approval inspect json = %q, want allowed_use runner_injection_policy_only", injectionApprovalInspectJSON)
	}
	if strings.Contains(injectionApprovalInspectJSON, "text_excerpt") {
		t.Fatal("injection approval inspect json must not contain text_excerpt")
	}

	oldApprovalData := readFixtureFileString(t, filepath.Join(dir, approvalPath))
	if strings.Contains(oldApprovalData, `"runner_injection_allowed": true`) {
		t.Fatal("materialized context approval must keep runner_injection_allowed false")
	}
}

func retrievalContextFixtureRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func readFixtureFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return string(data)
}
