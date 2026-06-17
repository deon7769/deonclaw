package lancedbpolicy

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ModePlanOnly  = "plan_only"
	ModeFakeWrite = "fake_write"

	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusFailed  = "failed"
)

var allowedModes = map[string]struct{}{
	ModePlanOnly:  {},
	ModeFakeWrite: {},
}

type Config struct {
	LanceDBPolicy Policy `yaml:"lancedb_policy" json:"lancedb_policy"`
}

type Policy struct {
	Input    InputConfig    `yaml:"input" json:"input"`
	Database DatabaseConfig `yaml:"database" json:"database"`
	Schema   SchemaConfig   `yaml:"schema" json:"schema"`
	Limits   LimitsConfig   `yaml:"limits" json:"limits"`
	Mode     string         `yaml:"mode" json:"mode"`
}

type InputConfig struct {
	EmbeddingManifest string `yaml:"embedding_manifest" json:"embedding_manifest"`
	VectorsPath       string `yaml:"vectors_path" json:"vectors_path"`
	ChunksPath        string `yaml:"chunks_path" json:"chunks_path"`
}

type DatabaseConfig struct {
	Path  string `yaml:"path" json:"path"`
	Table string `yaml:"table" json:"table"`
}

type SchemaConfig struct {
	VectorColumn    string   `yaml:"vector_column" json:"vector_column"`
	TextRefColumn   string   `yaml:"text_ref_column" json:"text_ref_column"`
	MetadataColumns []string `yaml:"metadata_columns" json:"metadata_columns"`
}

type LimitsConfig struct {
	MaxVectors         int `yaml:"max_vectors" json:"max_vectors"`
	ExpectedDimensions int `yaml:"expected_dimensions" json:"expected_dimensions"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read lancedb policy %q: %w", path, err)
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse lancedb policy yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Validate(cfg Config) error {
	return validate(cfg)
}

func normalize(cfg *Config) {
	p := &cfg.LanceDBPolicy
	p.Mode = strings.TrimSpace(p.Mode)
	p.Input.EmbeddingManifest = strings.TrimSpace(p.Input.EmbeddingManifest)
	p.Input.VectorsPath = strings.TrimSpace(p.Input.VectorsPath)
	p.Input.ChunksPath = strings.TrimSpace(p.Input.ChunksPath)
	p.Database.Path = strings.TrimSpace(p.Database.Path)
	p.Database.Table = strings.TrimSpace(p.Database.Table)
	p.Schema.VectorColumn = strings.TrimSpace(p.Schema.VectorColumn)
	p.Schema.TextRefColumn = strings.TrimSpace(p.Schema.TextRefColumn)
	p.Schema.MetadataColumns = trimNonEmpty(p.Schema.MetadataColumns)
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
	p := cfg.LanceDBPolicy

	if strings.TrimSpace(p.Mode) == "" {
		errs = append(errs, errors.New("lancedb_policy.mode is required"))
	} else if _, ok := allowedModes[p.Mode]; !ok {
		errs = append(errs, fmt.Errorf("lancedb_policy.mode %q is not allowed", p.Mode))
	}

	if err := validateRelativePath("input.embedding_manifest", p.Input.EmbeddingManifest); err != nil {
		errs = append(errs, err)
	}
	if err := validateRelativePath("input.vectors_path", p.Input.VectorsPath); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(p.Input.ChunksPath) != "" {
		if err := validateRelativePath("input.chunks_path", p.Input.ChunksPath); err != nil {
			errs = append(errs, err)
		}
	}
	if err := validateRelativePath("database.path", p.Database.Path); err != nil {
		errs = append(errs, err)
	}
	if err := validateSafeIdentifier("database.table", p.Database.Table); err != nil {
		errs = append(errs, err)
	}
	if err := validateSafeIdentifier("schema.vector_column", p.Schema.VectorColumn); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(p.Schema.TextRefColumn) == "" {
		errs = append(errs, errors.New("lancedb_policy.schema.text_ref_column is required"))
	} else if err := validateSafeIdentifier("schema.text_ref_column", p.Schema.TextRefColumn); err != nil {
		errs = append(errs, err)
	}
	if len(p.Schema.MetadataColumns) == 0 {
		errs = append(errs, errors.New("lancedb_policy.schema.metadata_columns must not be empty"))
	}
	for i, column := range p.Schema.MetadataColumns {
		if err := validateSafeIdentifier(fmt.Sprintf("schema.metadata_columns[%d]", i), column); err != nil {
			errs = append(errs, err)
		}
	}
	if p.Limits.MaxVectors <= 0 {
		errs = append(errs, errors.New("lancedb_policy.limits.max_vectors must be > 0"))
	}
	if p.Limits.ExpectedDimensions <= 0 {
		errs = append(errs, errors.New("lancedb_policy.limits.expected_dimensions must be > 0"))
	}

	return errors.Join(errs...)
}
