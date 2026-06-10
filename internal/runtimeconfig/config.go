package runtimeconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModeLocal  = "local"
	ModeDocker = "docker"

	NetworkNone    = "none"
	NetworkDefault = "default"

	MountModeReadOnly  = "ro"
	MountModeReadWrite = "rw"
)

type Config struct {
	Runtime Runtime `yaml:"runtime" json:"runtime"`
}

type Runtime struct {
	Mode   string       `yaml:"mode" json:"mode"`
	Docker DockerConfig `yaml:"docker,omitempty" json:"docker,omitempty"`
}

type DockerConfig struct {
	Image        string      `yaml:"image" json:"image"`
	Workdir      string      `yaml:"workdir" json:"workdir"`
	Network      string      `yaml:"network" json:"network"`
	ReadOnlyRoot bool        `yaml:"read_only_root" json:"read_only_root"`
	MemoryLimit  string      `yaml:"memory_limit,omitempty" json:"memory_limit,omitempty"`
	CPUs         string      `yaml:"cpus,omitempty" json:"cpus,omitempty"`
	Env          DockerEnv   `yaml:"env,omitempty" json:"env,omitempty"`
	Mounts       []MountSpec `yaml:"mounts,omitempty" json:"mounts,omitempty"`
}

type DockerEnv struct {
	Passthrough []string `yaml:"passthrough,omitempty" json:"passthrough,omitempty"`
}

type MountSpec struct {
	Source string `yaml:"source" json:"source"`
	Target string `yaml:"target" json:"target"`
	Mode   string `yaml:"mode" json:"mode"`
}

type ValidationResult struct {
	Warnings []string `json:"warnings,omitempty"`
}

type DockerPlan struct {
	Command  []string `json:"command"`
	Display  string   `json:"display"`
	Warnings []string `json:"warnings,omitempty"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read runtime config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse runtime config yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Validate(cfg Config) (ValidationResult, error) {
	var errs []error
	var warnings []string

	switch cfg.Runtime.Mode {
	case ModeLocal:
		return ValidationResult{}, nil
	case ModeDocker:
	default:
		errs = append(errs, fmt.Errorf("runtime.mode %q is not supported", cfg.Runtime.Mode))
	}

	docker := cfg.Runtime.Docker
	if cfg.Runtime.Mode == ModeDocker {
		requireNonEmpty := func(field, value string) {
			if strings.TrimSpace(value) == "" {
				errs = append(errs, fmt.Errorf("%s is required for docker runtime", field))
			}
		}
		requireNonEmpty("runtime.docker.image", docker.Image)
		requireNonEmpty("runtime.docker.workdir", docker.Workdir)
		if len(docker.Mounts) == 0 {
			errs = append(errs, errors.New("runtime.docker.mounts must not be empty for docker runtime"))
		}
	}

	switch docker.Network {
	case "", NetworkNone:
	case NetworkDefault:
		warnings = append(warnings, "runtime.docker.network default allows container network access")
	default:
		errs = append(errs, fmt.Errorf("runtime.docker.network %q is not supported", docker.Network))
	}

	for i, mount := range docker.Mounts {
		prefix := fmt.Sprintf("runtime.docker.mounts[%d]", i)
		if mount.Source == "" {
			errs = append(errs, fmt.Errorf("%s.source is required", prefix))
		} else if reason := dangerousMountSourceReason(mount.Source); reason != "" {
			errs = append(errs, fmt.Errorf("%s.source %q is not allowed: %s", prefix, mount.Source, reason))
		}
		if mount.Target == "" {
			errs = append(errs, fmt.Errorf("%s.target is required", prefix))
		} else if reason := invalidMountTargetReason(mount.Target); reason != "" {
			errs = append(errs, fmt.Errorf("%s.target %q is not allowed: %s", prefix, mount.Target, reason))
		}
		switch mount.Mode {
		case MountModeReadOnly, MountModeReadWrite:
		default:
			errs = append(errs, fmt.Errorf("%s.mode %q is not supported", prefix, mount.Mode))
		}
	}

	for i, name := range docker.Env.Passthrough {
		prefix := fmt.Sprintf("runtime.docker.env.passthrough[%d]", i)
		if reason := invalidEnvPassthroughNameReason(name); reason != "" {
			errs = append(errs, fmt.Errorf("%s is not allowed: %s", prefix, reason))
			continue
		}
		if !envIsSet(name) {
			warnings = append(warnings, fmt.Sprintf("runtime.docker.env.passthrough %s is not set in process env", name))
		}
	}

	return ValidationResult{Warnings: warnings}, errors.Join(errs...)
}

func PlanDocker(cfg Config, workspace string) (DockerPlan, error) {
	result, err := Validate(cfg)
	if err != nil {
		return DockerPlan{}, err
	}
	if cfg.Runtime.Mode != ModeDocker {
		return DockerPlan{}, fmt.Errorf("runtime.mode %q does not support docker-plan", cfg.Runtime.Mode)
	}

	docker := cfg.Runtime.Docker
	network := docker.Network
	if network == "" {
		network = NetworkNone
	}

	command := []string{"docker", "run", "--rm", "--network", network}
	if docker.MemoryLimit != "" {
		command = append(command, "--memory", docker.MemoryLimit)
	}
	if docker.CPUs != "" {
		command = append(command, "--cpus", docker.CPUs)
	}
	if docker.ReadOnlyRoot {
		command = append(command, "--read-only")
	}
	if docker.Workdir != "" {
		command = append(command, "-w", docker.Workdir)
	}
	for _, name := range docker.Env.Passthrough {
		command = append(command, "-e", name)
	}
	for _, mount := range docker.Mounts {
		command = append(command, "-v", mount.Source+":"+mount.Target+":"+mount.Mode)
	}
	if workspace != "" {
		command = append(command, "--label", "deonclaw.workspace="+workspace)
	}
	command = append(command, docker.Image)

	return DockerPlan{
		Command:  command,
		Display:  strings.Join(command, " "),
		Warnings: result.Warnings,
	}, nil
}

func PlanDockerExec(cfg Config, workspace string, execCommand []string) (DockerPlan, error) {
	if len(execCommand) == 0 || strings.TrimSpace(execCommand[0]) == "" {
		return DockerPlan{}, errors.New("docker exec command must not be empty")
	}

	plan, err := PlanDocker(cfg, workspace)
	if err != nil {
		return DockerPlan{}, err
	}
	if err := validateExecEnvPassthrough(cfg); err != nil {
		return DockerPlan{}, err
	}
	plan.Command = append(plan.Command, execCommand...)
	plan.Display = strings.Join(plan.Command, " ")
	return plan, nil
}

func normalize(cfg *Config) {
	cfg.Runtime.Mode = strings.TrimSpace(cfg.Runtime.Mode)
	cfg.Runtime.Docker.Image = strings.TrimSpace(cfg.Runtime.Docker.Image)
	cfg.Runtime.Docker.Workdir = strings.TrimSpace(cfg.Runtime.Docker.Workdir)
	cfg.Runtime.Docker.Network = strings.TrimSpace(cfg.Runtime.Docker.Network)
	cfg.Runtime.Docker.MemoryLimit = strings.TrimSpace(cfg.Runtime.Docker.MemoryLimit)
	cfg.Runtime.Docker.CPUs = strings.TrimSpace(cfg.Runtime.Docker.CPUs)
	for i := range cfg.Runtime.Docker.Env.Passthrough {
		cfg.Runtime.Docker.Env.Passthrough[i] = strings.TrimSpace(cfg.Runtime.Docker.Env.Passthrough[i])
	}
	for i := range cfg.Runtime.Docker.Mounts {
		cfg.Runtime.Docker.Mounts[i].Source = strings.TrimSpace(cfg.Runtime.Docker.Mounts[i].Source)
		cfg.Runtime.Docker.Mounts[i].Target = strings.TrimSpace(cfg.Runtime.Docker.Mounts[i].Target)
		cfg.Runtime.Docker.Mounts[i].Mode = strings.TrimSpace(cfg.Runtime.Docker.Mounts[i].Mode)
	}
}

func dangerousMountSourceReason(source string) string {
	lower := normalizedPath(source)

	switch lower {
	case "/", "~", "/home", "/root", "/var/run/docker.sock", ".env":
		return "dangerous host path"
	}
	if strings.HasPrefix(lower, "~/") {
		return "home directory mounts are not allowed"
	}
	if isPathOrDescendant(lower, "/home") {
		return "home directory mounts are not allowed"
	}
	if isPathOrDescendant(lower, "/root") {
		return "root home mounts are not allowed"
	}
	if lower == "~/.ssh" || strings.HasSuffix(lower, "/.ssh") || strings.Contains(lower, "/.ssh/") {
		return "ssh material must not be mounted"
	}
	for _, part := range strings.Split(lower, "/") {
		if part == ".env" {
			return "env files must not be mounted"
		}
		if part == "secrets" {
			return "secret directories must not be mounted"
		}
	}
	return ""
}

func invalidMountTargetReason(target string) string {
	lower := normalizedPath(target)
	if !strings.HasPrefix(lower, "/") {
		return "container target must be absolute"
	}
	switch lower {
	case "/", "/root", "/etc", "/var/run/docker.sock":
		return "dangerous container target"
	}
	if isPathOrDescendant(lower, "/root") || isPathOrDescendant(lower, "/etc") {
		return "dangerous container target"
	}
	return ""
}

func normalizedPath(path string) string {
	path = strings.TrimSpace(path)
	cleaned := filepath.Clean(path)
	return strings.ToLower(filepath.ToSlash(cleaned))
}

func isPathOrDescendant(path string, parent string) bool {
	return path == parent || strings.HasPrefix(path, parent+"/")
}

func invalidEnvPassthroughNameReason(name string) string {
	if name == "" {
		return "env name must not be empty"
	}
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' {
			continue
		}
		return "env name must contain only A-Z, 0-9, and _"
	}
	return ""
}

func validateExecEnvPassthrough(cfg Config) error {
	var errs []error
	for _, name := range cfg.Runtime.Docker.Env.Passthrough {
		if !envIsSet(name) {
			errs = append(errs, fmt.Errorf("runtime.docker.env.passthrough %s is not set in process env", name))
		}
	}
	return errors.Join(errs...)
}

func envIsSet(name string) bool {
	value, ok := os.LookupEnv(name)
	return ok && value != ""
}
