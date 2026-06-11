package mcpconfig

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	TrustLocal    = "local"
	TrustExternal = "external"

	CapabilityRead  = "read"
	CapabilityWrite = "write"
	CapabilityExec  = "exec"
)

var safeServerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Config struct {
	MCP MCPConfig `yaml:"mcp" json:"mcp"`
}

type MCPConfig struct {
	Servers map[string]ServerConfig `yaml:"servers" json:"servers"`
}

type ServerConfig struct {
	Command      string    `yaml:"command" json:"command"`
	Args         []string  `yaml:"args,omitempty" json:"args,omitempty"`
	Enabled      bool      `yaml:"enabled" json:"enabled"`
	Trust        string    `yaml:"trust" json:"trust"`
	Capabilities []string  `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	Env          ServerEnv `yaml:"env,omitempty" json:"env,omitempty"`
}

type ServerEnv struct {
	Passthrough []string `yaml:"passthrough,omitempty" json:"passthrough,omitempty"`
}

type ValidationResult struct {
	Warnings []string `json:"warnings,omitempty"`
}

type ServerListItem struct {
	Name         string   `json:"name"`
	Command      string   `json:"command"`
	Enabled      bool     `json:"enabled"`
	Trust        string   `json:"trust"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type Plan struct {
	Server       string   `json:"server"`
	Command      string   `json:"command"`
	Args         []string `json:"args,omitempty"`
	EnvNames     []string `json:"env_names,omitempty"`
	Enabled      bool     `json:"enabled"`
	Trust        string   `json:"trust"`
	Capabilities []string `json:"capabilities,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read mcp config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse mcp config yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Validate(cfg Config) (ValidationResult, error) {
	var errs []error
	var warnings []string

	if len(cfg.MCP.Servers) == 0 {
		errs = append(errs, errors.New("mcp.servers must not be empty"))
	}

	for _, name := range sortedServerNames(cfg.MCP.Servers) {
		server := cfg.MCP.Servers[name]
		prefix := fmt.Sprintf("mcp.servers.%s", name)
		if reason := invalidServerNameReason(name); reason != "" {
			errs = append(errs, fmt.Errorf("mcp server name is not allowed: %s", reason))
		}
		if strings.TrimSpace(server.Command) == "" {
			errs = append(errs, fmt.Errorf("%s.command is required", prefix))
		}
		switch server.Trust {
		case TrustLocal, TrustExternal:
		default:
			errs = append(errs, fmt.Errorf("%s.trust %q is not supported", prefix, server.Trust))
		}
		for i, capability := range server.Capabilities {
			switch capability {
			case CapabilityRead:
			case CapabilityWrite, CapabilityExec:
				if server.Enabled {
					errs = append(errs, fmt.Errorf("%s.capabilities[%d] %q requires enabled=false in registry foundation", prefix, i, capability))
				} else {
					warnings = append(warnings, fmt.Sprintf("%s capability %q is dangerous and remains disabled", prefix, capability))
				}
			default:
				errs = append(errs, fmt.Errorf("%s.capabilities[%d] %q is not supported", prefix, i, capability))
			}
		}
		for i, name := range server.Env.Passthrough {
			if reason := invalidEnvPassthroughNameReason(name); reason != "" {
				errs = append(errs, fmt.Errorf("%s.env.passthrough[%d] is not allowed: %s", prefix, i, reason))
			}
		}
	}

	return ValidationResult{Warnings: warnings}, errors.Join(errs...)
}

func ListServers(cfg Config) ([]ServerListItem, error) {
	if _, err := Validate(cfg); err != nil {
		return nil, err
	}
	items := make([]ServerListItem, 0, len(cfg.MCP.Servers))
	for _, name := range sortedServerNames(cfg.MCP.Servers) {
		server := cfg.MCP.Servers[name]
		items = append(items, ServerListItem{
			Name:         name,
			Command:      server.Command,
			Enabled:      server.Enabled,
			Trust:        server.Trust,
			Capabilities: append([]string(nil), server.Capabilities...),
		})
	}
	return items, nil
}

func PlanServer(cfg Config, serverName string) (Plan, error) {
	result, err := Validate(cfg)
	if err != nil {
		return Plan{}, err
	}
	name := strings.TrimSpace(serverName)
	if name == "" {
		return Plan{}, errors.New("mcp plan requires --server")
	}
	server, ok := cfg.MCP.Servers[name]
	if !ok {
		return Plan{}, fmt.Errorf("mcp server %q not found", name)
	}
	return Plan{
		Server:       name,
		Command:      server.Command,
		Args:         append([]string(nil), server.Args...),
		EnvNames:     append([]string(nil), server.Env.Passthrough...),
		Enabled:      server.Enabled,
		Trust:        server.Trust,
		Capabilities: append([]string(nil), server.Capabilities...),
		Warnings:     result.Warnings,
	}, nil
}

func normalize(cfg *Config) {
	for name, server := range cfg.MCP.Servers {
		server.Command = strings.TrimSpace(server.Command)
		server.Trust = strings.TrimSpace(server.Trust)
		for i := range server.Args {
			server.Args[i] = strings.TrimSpace(server.Args[i])
		}
		for i := range server.Capabilities {
			server.Capabilities[i] = strings.TrimSpace(server.Capabilities[i])
		}
		for i := range server.Env.Passthrough {
			server.Env.Passthrough[i] = strings.TrimSpace(server.Env.Passthrough[i])
		}
		cfg.MCP.Servers[name] = server
	}
}

func sortedServerNames(servers map[string]ServerConfig) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func invalidServerNameReason(name string) string {
	if strings.TrimSpace(name) == "" {
		return "name is required"
	}
	if !safeServerNamePattern.MatchString(name) {
		return "use only letters, numbers, '.', '_' and '-', starting with a letter or number"
	}
	return ""
}

func invalidEnvPassthroughNameReason(name string) string {
	if name == "" {
		return "name is required"
	}
	if strings.Contains(name, "=") {
		return "inline env values are forbidden"
	}
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return "use only A-Z, 0-9 and _"
	}
	return ""
}
