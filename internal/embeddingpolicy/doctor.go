package embeddingpolicy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/deon7769/deonclaw/internal/memoryindex"
)

type DoctorResult struct {
	Status              string           `json:"status"`
	Provider            string           `json:"provider"`
	Model               string           `json:"model"`
	Dimensions          int              `json:"dimensions"`
	Mode                string           `json:"mode"`
	EnvRequirements     []EnvRequirement `json:"env_requirements"`
	InputChunksPath     string           `json:"input_chunks_path"`
	InputManifestPath   string           `json:"input_manifest_path"`
	InputChunksExists   bool             `json:"input_chunks_exists"`
	InputManifestExists bool             `json:"input_manifest_exists"`
	ChunkCount          int              `json:"chunk_count"`
	ManifestChunkCount  int              `json:"manifest_chunk_count"`
	EstimatedBatches    int              `json:"estimated_batches"`
	Warnings            []string         `json:"warnings"`
}

func Doctor(cfg Config) (DoctorResult, error) {
	if err := Validate(cfg); err != nil {
		return DoctorResult{}, err
	}
	p := cfg.EmbeddingPolicy

	chunksExists := fileExists(p.Input.ChunksPath)
	manifestExists := fileExists(p.Input.ManifestPath)

	chunkCount := 0
	if chunksExists {
		count, err := countChunksJSONL(p.Input.ChunksPath)
		if err != nil {
			return DoctorResult{}, err
		}
		chunkCount = count
	}
	manifestChunkCount := 0
	if manifestExists {
		manifest, err := memoryindex.LoadManifest(p.Input.ManifestPath)
		if err != nil {
			return DoctorResult{}, err
		}
		manifestChunkCount = manifest.ChunkCount
	}

	envRequirements := EnvRequirements(cfg)
	warnings := doctorWarnings(p, chunksExists, manifestExists, chunkCount, manifestChunkCount, envRequirements)

	return DoctorResult{
		Status:              doctorStatus(warnings),
		Provider:            p.Provider,
		Model:               p.Model,
		Dimensions:          p.Dimensions,
		Mode:                p.Mode,
		EnvRequirements:     envRequirements,
		InputChunksPath:     p.Input.ChunksPath,
		InputManifestPath:   p.Input.ManifestPath,
		InputChunksExists:   chunksExists,
		InputManifestExists: manifestExists,
		ChunkCount:          chunkCount,
		ManifestChunkCount:  manifestChunkCount,
		EstimatedBatches:    EstimatedBatches(chunkCount, p.Limits.BatchSize),
		Warnings:            warnings,
	}, nil
}

func doctorWarnings(p Policy, chunksExists bool, manifestExists bool, chunkCount int, manifestChunkCount int, envRequirements []EnvRequirement) []string {
	var warnings []string
	for _, req := range envRequirements {
		if req.State == EnvStateMissing {
			warnings = append(warnings, fmt.Sprintf("required env %s is missing", req.Name))
		}
	}
	if !chunksExists {
		warnings = append(warnings, fmt.Sprintf("input chunks file missing: %s", p.Input.ChunksPath))
	}
	if !manifestExists {
		warnings = append(warnings, fmt.Sprintf("input manifest file missing: %s", p.Input.ManifestPath))
	}
	if chunksExists && manifestExists && chunkCount != manifestChunkCount {
		warnings = append(warnings, fmt.Sprintf("chunk_count %d does not match manifest chunk_count %d", chunkCount, manifestChunkCount))
	}
	if chunkCount > p.Limits.MaxChunks {
		warnings = append(warnings, fmt.Sprintf("chunk_count %d exceeds max_chunks %d", chunkCount, p.Limits.MaxChunks))
	}
	return warnings
}

func doctorStatus(warnings []string) string {
	if len(warnings) > 0 {
		return StatusWarning
	}
	return StatusOK
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func countChunksJSONL(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open chunks jsonl %q: %w", path, err)
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var probe struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal([]byte(line), &probe); err != nil {
			return 0, fmt.Errorf("parse chunks jsonl %q line %d: %w", path, count+1, err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read chunks jsonl %q: %w", path, err)
	}
	return count, nil
}

func WriteDoctorText(result DoctorResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_embedding_doctor:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "provider: %s\n", result.Provider); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "model: %s\n", result.Model); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "dimensions: %d\n", result.Dimensions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "mode: %s\n", result.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "input_chunks_path: %s\n", result.InputChunksPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "input_manifest_path: %s\n", result.InputManifestPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "input_chunks_exists: %t\n", result.InputChunksExists); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "input_manifest_exists: %t\n", result.InputManifestExists); err != nil {
		return err
	}
	if result.InputChunksExists {
		if _, err := fmt.Fprintf(out, "chunk_count: %d\n", result.ChunkCount); err != nil {
			return err
		}
	}
	if result.InputManifestExists {
		if _, err := fmt.Fprintf(out, "manifest_chunk_count: %d\n", result.ManifestChunkCount); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "estimated_batches: %d\n", result.EstimatedBatches); err != nil {
		return err
	}
	if len(result.EnvRequirements) > 0 {
		if _, err := fmt.Fprintf(out, "\nenv_requirements:\n"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "name\tstate"); err != nil {
			return err
		}
		for _, req := range result.EnvRequirements {
			if _, err := fmt.Fprintf(table, "%s\t%s\n", req.Name, req.State); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	if len(result.Warnings) > 0 {
		if _, err := fmt.Fprintf(out, "\nwarnings:\n"); err != nil {
			return err
		}
		for _, warning := range result.Warnings {
			if _, err := fmt.Fprintf(out, "- %s\n", warning); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteDoctorJSON(result DoctorResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
