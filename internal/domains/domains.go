package domains

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const TypeIsolatedDomain = "isolated_domain"

type DomainsConfig struct {
	Domains map[string]DomainConfig `yaml:"domains" json:"domains"`
}

type DomainConfig struct {
	Name                     string            `yaml:"-" json:"name"`
	Type                     string            `yaml:"type" json:"type"`
	Root                     string            `yaml:"root" json:"root"`
	Default                  bool              `yaml:"default" json:"default"`
	BridgeFiles              []string          `yaml:"bridge_files,omitempty" json:"bridge_files,omitempty"`
	StructuredData           map[string]string `yaml:"structured_data,omitempty" json:"structured_data,omitempty"`
	Staging                  []string          `yaml:"staging,omitempty" json:"staging,omitempty"`
	DefaultAgent             string            `yaml:"default_agent,omitempty" json:"default_agent,omitempty"`
	ReadOnlyForWorkers       bool              `yaml:"read_only_for_workers,omitempty" json:"read_only_for_workers,omitempty"`
	ReadOnlyForGeneralAgents bool              `yaml:"read_only_for_general_agents,omitempty" json:"read_only_for_general_agents,omitempty"`
	ExplicitIsolated         bool              `yaml:"isolated,omitempty" json:"isolated,omitempty"`
}

func Parse(data []byte) (*DomainsConfig, error) {
	var config DomainsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse domains yaml: %w", err)
	}
	normalize(&config)
	return &config, nil
}

func LoadFromFile(path string) (*DomainsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read domains config %q: %w", path, err)
	}
	return Parse(data)
}

func Validate(config *DomainsConfig) error {
	if config == nil {
		return errors.New("domains config is nil")
	}

	var errs []error
	if len(config.Domains) == 0 {
		errs = append(errs, errors.New("at least one domain is required"))
	}

	defaultCount := 0
	for name, domain := range config.Domains {
		if strings.TrimSpace(domain.Type) == "" {
			errs = append(errs, fmt.Errorf("domains[%s].type is required", name))
		}
		if strings.TrimSpace(domain.Root) == "" {
			errs = append(errs, fmt.Errorf("domains[%s].root is required", name))
		}
		if domain.Default {
			defaultCount++
		}
		if domain.IsIsolated() && domain.Default {
			errs = append(errs, fmt.Errorf("domains[%s] isolated domain must not be default", name))
		}
	}
	if defaultCount != 1 {
		errs = append(errs, fmt.Errorf("exactly one default domain is required, got %d", defaultCount))
	}

	return errors.Join(errs...)
}

func (c *DomainsConfig) List() []DomainConfig {
	if c == nil || len(c.Domains) == 0 {
		return nil
	}

	names := make([]string, 0, len(c.Domains))
	for name := range c.Domains {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]DomainConfig, 0, len(names))
	for _, name := range names {
		domain := c.Domains[name]
		if domain.Name == "" {
			domain.Name = name
		}
		result = append(result, domain)
	}
	return result
}

func (d DomainConfig) IsIsolated() bool {
	return d.ExplicitIsolated || d.Type == TypeIsolatedDomain
}

func normalize(config *DomainsConfig) {
	if config == nil {
		return
	}
	for name, domain := range config.Domains {
		normalizedName := strings.TrimSpace(name)
		domain.Name = normalizedName
		domain.Type = strings.TrimSpace(domain.Type)
		domain.Root = strings.TrimSpace(domain.Root)
		domain.DefaultAgent = strings.TrimSpace(domain.DefaultAgent)
		trimStrings(domain.BridgeFiles)
		trimStrings(domain.Staging)
		for key, value := range domain.StructuredData {
			trimmedKey := strings.TrimSpace(key)
			trimmedValue := strings.TrimSpace(value)
			if trimmedKey != key {
				delete(domain.StructuredData, key)
			}
			domain.StructuredData[trimmedKey] = trimmedValue
		}
		if normalizedName != name {
			delete(config.Domains, name)
		}
		config.Domains[normalizedName] = domain
	}
}

func trimStrings(values []string) {
	for i := range values {
		values[i] = strings.TrimSpace(values[i])
	}
}
