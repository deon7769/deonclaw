package mcpconfig

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"gopkg.in/yaml.v3"
)

const (
	TrustLocal    = "local"
	TrustExternal = "external"

	CapabilityRead  = "read"
	CapabilityWrite = "write"
	CapabilityExec  = "exec"

	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"

	EnvStateSetMasked = "set_masked"
	EnvStateMissing   = "missing"
	commandUnknown    = "unknown"
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

type DoctorReport struct {
	Servers []DoctorServer `json:"servers"`
}

type DoctorServer struct {
	Name             string           `json:"name"`
	Enabled          bool             `json:"enabled"`
	Command          string           `json:"command"`
	CommandAvailable any              `json:"command_available"`
	Trust            string           `json:"trust"`
	Capabilities     []string         `json:"capabilities,omitempty"`
	RiskLevel        string           `json:"risk_level"`
	EnvRequirements  []EnvRequirement `json:"env_requirements,omitempty"`
	Warnings         []string         `json:"warnings,omitempty"`
}

type EnvRequirement struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type RiskReport struct {
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

type DockerLaunchPlan struct {
	Server       string   `json:"server"`
	PlanOnly     bool     `json:"plan_only"`
	Command      []string `json:"command"`
	Display      string   `json:"display"`
	EnvNames     []string `json:"env_names,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Trust        string   `json:"trust"`
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

func Doctor(cfg Config) (DoctorReport, error) {
	result, err := Validate(cfg)
	if err != nil {
		return DoctorReport{}, err
	}
	warningsByServer := serverWarnings(result.Warnings, cfg)
	report := DoctorReport{Servers: make([]DoctorServer, 0, len(cfg.MCP.Servers))}
	for _, name := range sortedServerNames(cfg.MCP.Servers) {
		server := cfg.MCP.Servers[name]
		envRequirements := make([]EnvRequirement, 0, len(server.Env.Passthrough))
		for _, envName := range server.Env.Passthrough {
			envRequirements = append(envRequirements, EnvRequirement{
				Name:  envName,
				State: envRequirementState(envName),
			})
		}
		warnings := append([]string(nil), warningsByServer[name]...)
		if riskLevel(server) == RiskHigh {
			warnings = append(warnings, "server has write or exec capability; keep disabled until MCP execution policy exists")
		}
		for _, envRequirement := range envRequirements {
			if envRequirement.State == EnvStateMissing {
				warnings = append(warnings, fmt.Sprintf("env %s is missing", envRequirement.Name))
			}
		}
		report.Servers = append(report.Servers, DoctorServer{
			Name:             name,
			Enabled:          server.Enabled,
			Command:          server.Command,
			CommandAvailable: commandAvailability(server.Command),
			Trust:            server.Trust,
			Capabilities:     append([]string(nil), server.Capabilities...),
			RiskLevel:        riskLevel(server),
			EnvRequirements:  envRequirements,
			Warnings:         uniqueStrings(warnings),
		})
	}
	return report, nil
}

func Risk(cfg Config) (RiskReport, error) {
	report, err := Doctor(cfg)
	if err != nil {
		return RiskReport{}, err
	}
	risk := RiskReport{
		TotalServers: len(report.Servers),
	}
	for _, server := range report.Servers {
		if server.Enabled {
			risk.EnabledServers++
		} else {
			risk.DisabledServers++
		}
		if server.Trust == TrustExternal {
			risk.ExternalServers++
		}
		if containsString(server.Capabilities, CapabilityWrite) {
			risk.WriteCapabilityServers++
		}
		if containsString(server.Capabilities, CapabilityExec) {
			risk.ExecCapabilityServers++
		}
		for _, envRequirement := range server.EnvRequirements {
			if envRequirement.State == EnvStateMissing {
				risk.MissingEnvCount++
			}
		}
		switch server.RiskLevel {
		case RiskHigh:
			risk.HighRiskCount++
		case RiskMedium:
			risk.MediumRiskCount++
		case RiskLow:
			risk.LowRiskCount++
		}
	}
	return risk, nil
}

func PlanDockerLaunch(cfg Config, serverName string, runtimeCfg runtimeconfig.Config, workspace string) (DockerLaunchPlan, error) {
	serverPlan, err := PlanServer(cfg, serverName)
	if err != nil {
		return DockerLaunchPlan{}, err
	}
	serverCommand := append([]string{serverPlan.Command}, serverPlan.Args...)
	dockerPlan, err := runtimeconfig.PlanDockerExec(runtimeCfg, workspace, serverCommand)
	if err != nil {
		return DockerLaunchPlan{}, err
	}
	command, err := addDockerEnvPassthrough(dockerPlan.Command, runtimeCfg.Runtime.Docker.Image, serverPlan.EnvNames)
	if err != nil {
		return DockerLaunchPlan{}, err
	}
	warnings := append([]string(nil), serverPlan.Warnings...)
	warnings = append(warnings, dockerPlan.Warnings...)
	for _, name := range serverPlan.EnvNames {
		if !envIsSet(name) {
			warnings = append(warnings, fmt.Sprintf("mcp server %s env passthrough %s is not set; docker-plan is plan only and future execution may fail", serverPlan.Server, name))
		}
	}
	return DockerLaunchPlan{
		Server:       serverPlan.Server,
		PlanOnly:     true,
		Command:      command,
		Display:      strings.Join(command, " "),
		EnvNames:     append([]string(nil), serverPlan.EnvNames...),
		Warnings:     uniqueStrings(warnings),
		Capabilities: append([]string(nil), serverPlan.Capabilities...),
		Trust:        serverPlan.Trust,
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

func serverWarnings(warnings []string, cfg Config) map[string][]string {
	byServer := make(map[string][]string)
	for _, warning := range warnings {
		for name := range cfg.MCP.Servers {
			if strings.Contains(warning, "mcp.servers."+name) {
				byServer[name] = append(byServer[name], warning)
			}
		}
	}
	return byServer
}

func commandAvailability(command string) any {
	first := firstCommand(command)
	if first == "" {
		return commandUnknown
	}
	if _, err := exec.LookPath(first); err != nil {
		return false
	}
	return true
}

func firstCommand(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func envRequirementState(name string) string {
	if envIsSet(name) {
		return EnvStateSetMasked
	}
	return EnvStateMissing
}

func riskLevel(server ServerConfig) string {
	if containsString(server.Capabilities, CapabilityWrite) || containsString(server.Capabilities, CapabilityExec) {
		return RiskHigh
	}
	if server.Trust == TrustExternal {
		return RiskMedium
	}
	return RiskLow
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func envIsSet(name string) bool {
	value, ok := os.LookupEnv(name)
	return ok && value != ""
}

func addDockerEnvPassthrough(command []string, image string, envNames []string) ([]string, error) {
	out := append([]string(nil), command...)
	if len(envNames) == 0 {
		return out, nil
	}
	imageIndex := -1
	for i, part := range out {
		if part == image {
			imageIndex = i
			break
		}
	}
	if imageIndex < 0 {
		return nil, fmt.Errorf("docker plan image %q not found", image)
	}
	existing := map[string]bool{}
	for i := 0; i+1 < len(out); i++ {
		if out[i] == "-e" {
			existing[out[i+1]] = true
		}
	}
	var insertion []string
	for _, name := range envNames {
		if existing[name] {
			continue
		}
		insertion = append(insertion, "-e", name)
	}
	if len(insertion) == 0 {
		return out, nil
	}
	updated := make([]string, 0, len(out)+len(insertion))
	updated = append(updated, out[:imageIndex]...)
	updated = append(updated, insertion...)
	updated = append(updated, out[imageIndex:]...)
	return updated, nil
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	return unique
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
