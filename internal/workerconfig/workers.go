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
	Command string `yaml:"command" json:"command"`
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

func normalize(cfg *Config) {
	if cfg.Workers == nil {
		cfg.Workers = map[string]Worker{}
	}
	for name, worker := range cfg.Workers {
		normalizedName := strings.TrimSpace(name)
		worker.Command = strings.TrimSpace(worker.Command)
		if normalizedName != name {
			delete(cfg.Workers, name)
		}
		cfg.Workers[normalizedName] = worker
	}
}
