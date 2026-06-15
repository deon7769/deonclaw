package mcpconfig

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/runtimeconfig"
)

func TestLoadAndValidateValidMCPConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "configs", "examples", "mcp.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	result, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", result.Warnings)
	}
}

func TestLoadAndValidateFakeMCPConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "configs", "examples", "mcp-fake.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	result, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", result.Warnings)
	}
	server := cfg.MCP.Servers["fake-stdio"]
	if !server.TestOnly || server.Protocol != ProtocolStdio {
		t.Fatalf("server = %#v, want test_only stdio", server)
	}
}

func TestValidateRejectsUnknownProtocol(t *testing.T) {
	cfg := validConfig()
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Protocol = "http"
	cfg.MCP.Servers["filesystem-readonly"] = server

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want unknown protocol")
	}
	if !strings.Contains(err.Error(), `protocol "http" is not supported`) {
		t.Fatalf("error = %v, want protocol rejection", err)
	}
}

func TestValidateRejectsMissingCommand(t *testing.T) {
	cfg := validConfig()
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Command = ""
	cfg.MCP.Servers["filesystem-readonly"] = server

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want missing command")
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("error = %v, want command required", err)
	}
}

func TestValidateRejectsInlineEnvWithoutLeakingValue(t *testing.T) {
	cfg := validConfig()
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Env.Passthrough = []string{"GITHUB_TOKEN=super-secret-value"}
	cfg.MCP.Servers["filesystem-readonly"] = server

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want invalid env")
	}
	if !strings.Contains(err.Error(), "inline env values are forbidden") {
		t.Fatalf("error = %v, want inline env rejection", err)
	}
	if strings.Contains(err.Error(), "super-secret-value") {
		t.Fatalf("error leaked env value: %v", err)
	}
}

func TestValidateRejectsUnknownCapability(t *testing.T) {
	cfg := validConfig()
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Capabilities = []string{"read", "network"}
	cfg.MCP.Servers["filesystem-readonly"] = server

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want unknown capability")
	}
	if !strings.Contains(err.Error(), `capabilities[1] "network" is not supported`) {
		t.Fatalf("error = %v, want unknown capability", err)
	}
}

func TestValidateWriteExecDisabledWarn(t *testing.T) {
	cfg := validConfig()
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Capabilities = []string{"read", "write", "exec"}
	server.Enabled = false
	cfg.MCP.Servers["filesystem-readonly"] = server

	result, err := Validate(cfg)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(result.Warnings) != 2 || !strings.Contains(strings.Join(result.Warnings, "\n"), "dangerous") {
		t.Fatalf("warnings = %#v, want dangerous warnings", result.Warnings)
	}
}

func TestValidateWriteExecEnabledFails(t *testing.T) {
	cfg := validConfig()
	server := cfg.MCP.Servers["filesystem-readonly"]
	server.Capabilities = []string{"exec"}
	server.Enabled = true
	cfg.MCP.Servers["filesystem-readonly"] = server

	_, err := Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want enabled exec rejection")
	}
	if !strings.Contains(err.Error(), "requires enabled=false") {
		t.Fatalf("error = %v, want enabled=false rejection", err)
	}
}

func TestPlanServerIncludesEnvNamesOnly(t *testing.T) {
	cfg := validConfig()
	plan, err := PlanServer(cfg, "github-readonly")
	if err != nil {
		t.Fatalf("PlanServer() error = %v", err)
	}
	if len(plan.EnvNames) != 1 || plan.EnvNames[0] != "GITHUB_TOKEN" {
		t.Fatalf("env names = %#v, want GITHUB_TOKEN", plan.EnvNames)
	}
	if strings.Contains(strings.Join(plan.EnvNames, " "), "secret") {
		t.Fatalf("plan leaked secret: %#v", plan)
	}
}

func TestDoctorReportsRiskCommandAvailabilityAndEnvState(t *testing.T) {
	tempDir := t.TempDir()
	writeMCPConfigTestCommand(t, tempDir, "fake-mcp")
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MCP_TOKEN", "super-secret-value")
	unsetMCPConfigEnvForTest(t, "MISSING_TOKEN")

	cfg := Config{
		MCP: MCPConfig{
			Servers: map[string]ServerConfig{
				"filesystem-readonly": {
					Command:      "fake-mcp",
					Enabled:      false,
					Trust:        TrustLocal,
					Capabilities: []string{CapabilityRead},
					Env:          ServerEnv{Passthrough: []string{"MCP_TOKEN"}},
				},
				"github-readonly": {
					Command:      "missing-mcp-command",
					Enabled:      false,
					Trust:        TrustExternal,
					Capabilities: []string{CapabilityRead},
					Env:          ServerEnv{Passthrough: []string{"MISSING_TOKEN"}},
				},
				"risky-disabled": {
					Command:      "missing-risky-command",
					Enabled:      false,
					Trust:        TrustLocal,
					Capabilities: []string{CapabilityWrite, CapabilityExec},
				},
			},
		},
	}

	report, err := Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	readonly := doctorServerByName(t, report, "filesystem-readonly")
	if readonly.CommandAvailable != true || readonly.RiskLevel != RiskLow {
		t.Fatalf("readonly doctor = %#v, want available low risk", readonly)
	}
	if len(readonly.EnvRequirements) != 1 || readonly.EnvRequirements[0].Name != "MCP_TOKEN" || readonly.EnvRequirements[0].State != EnvStateSetMasked {
		t.Fatalf("readonly env = %#v, want set_masked MCP_TOKEN", readonly.EnvRequirements)
	}
	external := doctorServerByName(t, report, "github-readonly")
	if external.CommandAvailable != false || external.RiskLevel != RiskMedium {
		t.Fatalf("external doctor = %#v, want missing command medium risk", external)
	}
	if len(external.EnvRequirements) != 1 || external.EnvRequirements[0].State != EnvStateMissing {
		t.Fatalf("external env = %#v, want missing", external.EnvRequirements)
	}
	risky := doctorServerByName(t, report, "risky-disabled")
	if risky.RiskLevel != RiskHigh || len(risky.Warnings) == 0 {
		t.Fatalf("risky doctor = %#v, want high risk warnings", risky)
	}
	if strings.Contains(strings.Join(risky.Warnings, "\n")+strings.Join(readonly.Warnings, "\n"), "super-secret-value") {
		t.Fatalf("doctor warnings leaked secret: %#v", report.Servers)
	}
}

func TestRiskReportAggregatesCounts(t *testing.T) {
	unsetMCPConfigEnvForTest(t, "MISSING_TOKEN")
	cfg := Config{
		MCP: MCPConfig{
			Servers: map[string]ServerConfig{
				"local-read": {
					Command:      "placeholder",
					Enabled:      false,
					Trust:        TrustLocal,
					Capabilities: []string{CapabilityRead},
				},
				"external-read": {
					Command:      "placeholder",
					Enabled:      true,
					Trust:        TrustExternal,
					Capabilities: []string{CapabilityRead},
					Env:          ServerEnv{Passthrough: []string{"MISSING_TOKEN"}},
				},
				"high-write-exec": {
					Command:      "placeholder",
					Enabled:      false,
					Trust:        TrustLocal,
					Capabilities: []string{CapabilityWrite, CapabilityExec},
				},
			},
		},
	}

	report, err := Risk(cfg)
	if err != nil {
		t.Fatalf("Risk() error = %v", err)
	}
	if report.TotalServers != 3 || report.EnabledServers != 1 || report.DisabledServers != 2 {
		t.Fatalf("risk counts = %#v, want total/enabled/disabled 3/1/2", report)
	}
	if report.ExternalServers != 1 || report.WriteCapabilityServers != 1 || report.ExecCapabilityServers != 1 {
		t.Fatalf("risk capability counts = %#v, want external/write/exec 1", report)
	}
	if report.MissingEnvCount != 1 || report.HighRiskCount != 1 || report.MediumRiskCount != 1 || report.LowRiskCount != 1 {
		t.Fatalf("risk levels = %#v, want missing/high/medium/low 1", report)
	}
}

func TestDockerLaunchPlanAppendsServerCommandAndWarnsForMissingMCPEnv(t *testing.T) {
	unsetMCPConfigEnvForTest(t, "MISSING_TOKEN")
	cfg := Config{
		MCP: MCPConfig{
			Servers: map[string]ServerConfig{
				"filesystem-readonly": {
					Command:      "mcp-server",
					Args:         []string{"--root", "."},
					Enabled:      false,
					Trust:        TrustLocal,
					Capabilities: []string{CapabilityRead},
					Env:          ServerEnv{Passthrough: []string{"MISSING_TOKEN"}},
				},
			},
		},
	}
	runtimeCfg := runtimeconfig.Config{
		Runtime: runtimeconfig.Runtime{
			Mode: runtimeconfig.ModeDocker,
			Docker: runtimeconfig.DockerConfig{
				Image:        "deonclaw-runner:latest",
				Workdir:      "/workspace",
				Network:      runtimeconfig.NetworkNone,
				ReadOnlyRoot: true,
				Mounts: []runtimeconfig.MountSpec{
					{Source: ".", Target: "/workspace", Mode: "rw"},
				},
			},
		},
	}

	plan, err := PlanDockerLaunch(cfg, "filesystem-readonly", runtimeCfg, "/tmp/workspace")
	if err != nil {
		t.Fatalf("PlanDockerLaunch() error = %v", err)
	}
	for _, want := range []string{"docker run", "deonclaw-runner:latest", "mcp-server", "--root", ".", "-e MISSING_TOKEN"} {
		if !strings.Contains(plan.Display, want) {
			t.Fatalf("display = %q, want %q", plan.Display, want)
		}
	}
	if !plan.PlanOnly || !strings.Contains(strings.Join(plan.Warnings, "\n"), "MISSING_TOKEN") {
		t.Fatalf("plan = %#v, want plan_only with missing MCP env warning", plan)
	}
}

func validConfig() Config {
	return Config{
		MCP: MCPConfig{
			Servers: map[string]ServerConfig{
				"filesystem-readonly": {
					Command:      "npx",
					Args:         []string{"@modelcontextprotocol/server-filesystem", "."},
					Enabled:      false,
					Trust:        TrustLocal,
					Capabilities: []string{CapabilityRead},
				},
				"github-readonly": {
					Command:      "placeholder",
					Enabled:      false,
					Trust:        TrustExternal,
					Capabilities: []string{CapabilityRead},
					Env:          ServerEnv{Passthrough: []string{"GITHUB_TOKEN"}},
				},
			},
		},
	}
}

func doctorServerByName(t *testing.T, report DoctorReport, name string) DoctorServer {
	t.Helper()
	for _, server := range report.Servers {
		if server.Name == name {
			return server
		}
	}
	t.Fatalf("server %q not found in %#v", name, report.Servers)
	return DoctorServer{}
}

func writeMCPConfigTestCommand(t *testing.T, dir string, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\nexit 0\n"
	if runtime.GOOS == "windows" {
		path += ".bat"
		content = "@echo off\r\nexit /b 0\r\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(fake command) error = %v", err)
	}
	return path
}

func unsetMCPConfigEnvForTest(t *testing.T, name string) {
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
