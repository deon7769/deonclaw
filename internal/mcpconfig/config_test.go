package mcpconfig

import (
	"path/filepath"
	"strings"
	"testing"
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
