package embeddingpolicy

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ProviderOpenAI = "openai"
	ProviderZAI    = "zai"
	ProviderLocal  = "local"

	ModeDryRun = "dry_run"

	EnvStateSetMasked = "set_masked"
	EnvStateMissing   = "missing"

	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusFailed  = "failed"
)

var (
	allowedProviders = map[string]struct{}{
		ProviderOpenAI: {},
		ProviderZAI:    {},
		ProviderLocal:  {},
	}
	allowedModes = map[string]struct{}{
		ModeDryRun: {},
	}
	envNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

type Config struct {
	EmbeddingPolicy Policy `yaml:"embedding_policy" json:"embedding_policy"`
}

type Policy struct {
	Provider   string       `yaml:"provider" json:"provider"`
	Model      string       `yaml:"model" json:"model"`
	Dimensions int          `yaml:"dimensions" json:"dimensions"`
	Input      InputConfig  `yaml:"input" json:"input"`
	Output     OutputConfig `yaml:"output" json:"output"`
	Env        EnvConfig    `yaml:"env" json:"env"`
	Limits     LimitsConfig `yaml:"limits" json:"limits"`
	Mode       string       `yaml:"mode" json:"mode"`
}

type InputConfig struct {
	ChunksPath   string `yaml:"chunks_path" json:"chunks_path"`
	ManifestPath string `yaml:"manifest_path" json:"manifest_path"`
}

type OutputConfig struct {
	VectorsPath  string `yaml:"vectors_path" json:"vectors_path"`
	ManifestPath string `yaml:"manifest_path" json:"manifest_path"`
}

type EnvConfig struct {
	Required []string `yaml:"required" json:"required"`
}

type LimitsConfig struct {
	MaxChunks     int `yaml:"max_chunks" json:"max_chunks"`
	MaxChunkChars int `yaml:"max_chunk_chars" json:"max_chunk_chars"`
	BatchSize     int `yaml:"batch_size" json:"batch_size"`
}

type EnvRequirement struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read embedding policy %q: %w", path, err)
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse embedding policy yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Validate(cfg Config) error {
	return validate(cfg)
}

func normalize(cfg *Config) {
	p := &cfg.EmbeddingPolicy
	p.Provider = strings.TrimSpace(p.Provider)
	p.Model = strings.TrimSpace(p.Model)
	p.Mode = strings.TrimSpace(p.Mode)
	p.Input.ChunksPath = strings.TrimSpace(p.Input.ChunksPath)
	p.Input.ManifestPath = strings.TrimSpace(p.Input.ManifestPath)
	p.Output.VectorsPath = strings.TrimSpace(p.Output.VectorsPath)
	p.Output.ManifestPath = strings.TrimSpace(p.Output.ManifestPath)
	p.Env.Required = trimNonEmpty(p.Env.Required)
}

func trimNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func validate(cfg Config) error {
	var errs []error
	p := cfg.EmbeddingPolicy

	if strings.TrimSpace(p.Provider) == "" {
		errs = append(errs, errors.New("embedding_policy.provider is required"))
	} else if _, ok := allowedProviders[p.Provider]; !ok {
		errs = append(errs, fmt.Errorf("embedding_policy.provider %q is not allowed", p.Provider))
	}
	if strings.TrimSpace(p.Model) == "" {
		errs = append(errs, errors.New("embedding_policy.model is required"))
	}
	if p.Dimensions <= 0 {
		errs = append(errs, errors.New("embedding_policy.dimensions must be > 0"))
	}
	if strings.TrimSpace(p.Mode) == "" {
		errs = append(errs, errors.New("embedding_policy.mode is required"))
	} else if _, ok := allowedModes[p.Mode]; !ok {
		errs = append(errs, fmt.Errorf("embedding_policy.mode %q is not allowed", p.Mode))
	}
	if p.Limits.MaxChunks <= 0 {
		errs = append(errs, errors.New("embedding_policy.limits.max_chunks must be > 0"))
	}
	if p.Limits.MaxChunkChars <= 0 {
		errs = append(errs, errors.New("embedding_policy.limits.max_chunk_chars must be > 0"))
	}
	if p.Limits.BatchSize <= 0 {
		errs = append(errs, errors.New("embedding_policy.limits.batch_size must be > 0"))
	}

	if err := validateConfiguredPath("input.chunks_path", p.Input.ChunksPath); err != nil {
		errs = append(errs, err)
	}
	if err := validateConfiguredPath("input.manifest_path", p.Input.ManifestPath); err != nil {
		errs = append(errs, err)
	}

	artifactsDir := artifactsDirFromInput(p.Input.ChunksPath)
	if err := validateOutputPath("output.vectors_path", p.Output.VectorsPath, artifactsDir); err != nil {
		errs = append(errs, err)
	}
	if err := validateOutputPath("output.manifest_path", p.Output.ManifestPath, artifactsDir); err != nil {
		errs = append(errs, err)
	}

	for i, name := range p.Env.Required {
		if err := validateEnvName(fmt.Sprintf("env.required[%d]", i), name); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func validateEnvName(field string, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("%s is required", field)
	}
	if strings.Contains(name, "=") {
		return fmt.Errorf("%s must be an environment variable name, not NAME=value", field)
	}
	if !envNamePattern.MatchString(name) {
		return fmt.Errorf("%s %q is not a valid environment variable name", field, name)
	}
	return nil
}

func EnvRequirements(cfg Config) []EnvRequirement {
	names := append([]string(nil), cfg.EmbeddingPolicy.Env.Required...)
	requirements := make([]EnvRequirement, 0, len(names))
	for _, name := range names {
		requirements = append(requirements, EnvRequirement{
			Name:  name,
			State: envRequirementState(name),
		})
	}
	return requirements
}

func envRequirementState(name string) string {
	value, ok := os.LookupEnv(name)
	if ok && value != "" {
		return EnvStateSetMasked
	}
	return EnvStateMissing
}

func EstimatedBatches(chunkCount int, batchSize int) int {
	if batchSize <= 0 || chunkCount <= 0 {
		return 0
	}
	return (chunkCount + batchSize - 1) / batchSize
}
