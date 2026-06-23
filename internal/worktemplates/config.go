package worktemplates

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	WorkTemplates map[string]Template `yaml:"work_templates"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read work templates config %q: %w", path, err)
	}
	return ParseConfig(data)
}

func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse work templates yaml: %w", err)
	}
	if cfg.WorkTemplates == nil {
		cfg.WorkTemplates = map[string]Template{}
	}
	for id, tmpl := range cfg.WorkTemplates {
		tmpl.ID = strings.TrimSpace(id)
		cfg.WorkTemplates[id] = tmpl
	}
	return cfg, nil
}

func ValidateConfig(cfg Config) error {
	var errs []error
	for id, tmpl := range cfg.WorkTemplates {
		tmpl.ID = id
		if err := ValidateTemplate(tmpl); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
