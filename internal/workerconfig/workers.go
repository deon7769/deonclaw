package workerconfig

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Workers map[string]Worker `yaml:"workers" json:"workers"`
}

type Worker struct {
	Command  string            `yaml:"command" json:"command"`
	Provider string            `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model    string            `yaml:"model,omitempty" json:"model,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read workers config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse workers config yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Default() Config {
	return Config{
		Workers: map[string]Worker{
			"codex":    {Command: "codex"},
			"opencode": {Command: "opencode"},
			"kimi":     {Command: "kimi"},
		},
	}
}

func KnownWorkers() []string {
	return []string{"codex", "opencode", "kimi"}
}

func (c Config) Command(worker string) string {
	worker = strings.TrimSpace(worker)
	if worker == "" {
		return ""
	}
	if configured, ok := c.Workers[worker]; ok && strings.TrimSpace(configured.Command) != "" {
		return configured.Command
	}
	return Default().Workers[worker].Command
}

func (c Config) Worker(worker string) Worker {
	worker = strings.TrimSpace(worker)
	if worker == "" {
		return Worker{}
	}
	configured, ok := c.Workers[worker]
	if !ok {
		configured = Default().Workers[worker]
	}
	if strings.TrimSpace(configured.Command) == "" {
		configured.Command = Default().Workers[worker].Command
	}
	return configured
}

func normalize(cfg *Config) {
	if cfg.Workers == nil {
		cfg.Workers = map[string]Worker{}
	}
	for name, worker := range cfg.Workers {
		normalizedName := strings.TrimSpace(name)
		worker.Command = strings.TrimSpace(worker.Command)
		worker.Provider = strings.TrimSpace(worker.Provider)
		worker.Model = strings.TrimSpace(worker.Model)
		worker.Env = normalizeEnv(worker.Env)
		if normalizedName != name {
			delete(cfg.Workers, name)
		}
		cfg.Workers[normalizedName] = worker
	}
}

func normalizeEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(env))
	for name, value := range env {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		normalized[name] = strings.TrimSpace(value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
