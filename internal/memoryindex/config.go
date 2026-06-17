package memoryindex

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DomainMySecondBrain   = "mysecondbrain"
	DomainEscalasoftBrain = "escalasoft_brain"
)

var knownDomains = map[string]struct{}{
	DomainMySecondBrain:   {},
	DomainEscalasoftBrain: {},
}

type Config struct {
	MemoryIndex MemoryIndexConfig `yaml:"memory_index" json:"memory_index"`
}

type MemoryIndexConfig struct {
	Domains  []string       `yaml:"domains" json:"domains"`
	Sources  []Source       `yaml:"sources" json:"sources"`
	Chunking ChunkingConfig `yaml:"chunking" json:"chunking"`
	Output   OutputConfig   `yaml:"output" json:"output"`
}

type Source struct {
	Domain  string   `yaml:"domain" json:"domain"`
	Root    string   `yaml:"root" json:"root"`
	Include []string `yaml:"include" json:"include"`
	Exclude []string `yaml:"exclude" json:"exclude"`
}

type ChunkingConfig struct {
	MaxChars     int `yaml:"max_chars" json:"max_chars"`
	OverlapChars int `yaml:"overlap_chars" json:"overlap_chars"`
}

type OutputConfig struct {
	Manifest string `yaml:"manifest" json:"manifest"`
	Chunks   string `yaml:"chunks" json:"chunks"`
}

type Chunk struct {
	ID           string `json:"id"`
	Domain       string `json:"domain"`
	SourcePath   string `json:"source_path"`
	SourceSHA256 string `json:"source_sha256"`
	ChunkIndex   int    `json:"chunk_index"`
	Text         string `json:"text"`
	TextSHA256   string `json:"text_sha256"`
	CharStart    int    `json:"char_start"`
	CharEnd      int    `json:"char_end"`
}

type Manifest struct {
	GeneratedAt  string       `json:"generated_at"`
	ConfigSHA256 string       `json:"config_sha256"`
	Domains      []string     `json:"domains"`
	SourceCount  int          `json:"source_count"`
	ChunkCount   int          `json:"chunk_count"`
	SkippedCount int          `json:"skipped_count"`
	Skipped      SkippedStats `json:"skipped"`
	ManifestPath string       `json:"manifest_path"`
	ChunksPath   string       `json:"chunks_path"`
}

type BuildResult struct {
	Manifest Manifest `json:"manifest"`
	Chunks   []Chunk  `json:"chunks"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read memory index config %q: %w", path, err)
	}
	return Parse(data)
}

func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse memory index yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func ConfigSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func Validate(cfg Config) error {
	return validate(cfg, true)
}

func ValidateSchema(cfg Config) error {
	return validate(cfg, false)
}

func validate(cfg Config, checkFilesystem bool) error {
	var errs []error

	if len(cfg.MemoryIndex.Domains) == 0 {
		errs = append(errs, errors.New("memory_index.domains must not be empty"))
	}

	domainSet := map[string]struct{}{}
	for i, domain := range cfg.MemoryIndex.Domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			errs = append(errs, fmt.Errorf("memory_index.domains[%d] is required", i))
			continue
		}
		if _, ok := knownDomains[domain]; !ok {
			errs = append(errs, fmt.Errorf("memory_index.domains[%d] %q is not a known domain", i, domain))
		}
		domainSet[domain] = struct{}{}
	}

	if len(cfg.MemoryIndex.Sources) == 0 {
		errs = append(errs, errors.New("memory_index.sources must not be empty"))
	}

	if cfg.MemoryIndex.Chunking.MaxChars <= 0 {
		errs = append(errs, errors.New("memory_index.chunking.max_chars must be > 0"))
	}
	if cfg.MemoryIndex.Chunking.OverlapChars < 0 {
		errs = append(errs, errors.New("memory_index.chunking.overlap_chars must be >= 0"))
	}
	if cfg.MemoryIndex.Chunking.MaxChars > 0 && cfg.MemoryIndex.Chunking.OverlapChars >= cfg.MemoryIndex.Chunking.MaxChars {
		errs = append(errs, errors.New("memory_index.chunking.overlap_chars must be < max_chars"))
	}

	if strings.TrimSpace(cfg.MemoryIndex.Output.Manifest) == "" {
		errs = append(errs, errors.New("memory_index.output.manifest is required"))
	} else if err := validateConfiguredOutputPath("manifest", cfg.MemoryIndex.Output.Manifest); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(cfg.MemoryIndex.Output.Chunks) == "" {
		errs = append(errs, errors.New("memory_index.output.chunks is required"))
	} else if err := validateConfiguredOutputPath("chunks", cfg.MemoryIndex.Output.Chunks); err != nil {
		errs = append(errs, err)
	}

	for i, source := range cfg.MemoryIndex.Sources {
		prefix := fmt.Sprintf("memory_index.sources[%d]", i)
		if strings.TrimSpace(source.Domain) == "" {
			errs = append(errs, fmt.Errorf("%s.domain is required", prefix))
		} else if _, ok := domainSet[source.Domain]; !ok {
			errs = append(errs, fmt.Errorf("%s.domain %q is not listed in memory_index.domains", prefix, source.Domain))
		}
		if strings.TrimSpace(source.Root) == "" {
			errs = append(errs, fmt.Errorf("%s.root is required", prefix))
		} else if isSecretsPath(source.Root) {
			errs = append(errs, fmt.Errorf("%s.root %q is blocked because secrets paths are not indexable", prefix, source.Root))
		} else if checkFilesystem {
			if _, err := os.Stat(source.Root); err != nil {
				errs = append(errs, fmt.Errorf("%s.root %q does not exist: %w", prefix, source.Root, err))
			}
		}
		if len(source.Include) == 0 {
			errs = append(errs, fmt.Errorf("%s.include must not be empty", prefix))
		}
		if len(source.Exclude) == 0 {
			errs = append(errs, fmt.Errorf("%s.exclude must not be empty", prefix))
		}
		for j, pattern := range source.Include {
			if strings.TrimSpace(pattern) == "" {
				errs = append(errs, fmt.Errorf("%s.include[%d] is required", prefix, j))
			}
		}
		for j, pattern := range source.Exclude {
			if strings.TrimSpace(pattern) == "" {
				errs = append(errs, fmt.Errorf("%s.exclude[%d] is required", prefix, j))
			}
		}
	}

	return errors.Join(errs...)
}

func normalize(cfg *Config) {
	cfg.MemoryIndex.Domains = trimNonEmpty(cfg.MemoryIndex.Domains)
	for i := range cfg.MemoryIndex.Sources {
		cfg.MemoryIndex.Sources[i].Domain = strings.TrimSpace(cfg.MemoryIndex.Sources[i].Domain)
		cfg.MemoryIndex.Sources[i].Root = strings.TrimSpace(cfg.MemoryIndex.Sources[i].Root)
		cfg.MemoryIndex.Sources[i].Include = trimNonEmpty(cfg.MemoryIndex.Sources[i].Include)
		cfg.MemoryIndex.Sources[i].Exclude = trimNonEmpty(cfg.MemoryIndex.Sources[i].Exclude)
	}
	cfg.MemoryIndex.Output.Manifest = strings.TrimSpace(cfg.MemoryIndex.Output.Manifest)
	cfg.MemoryIndex.Output.Chunks = strings.TrimSpace(cfg.MemoryIndex.Output.Chunks)
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
