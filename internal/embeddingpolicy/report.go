package embeddingpolicy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type InvalidVectorReport struct {
	Line   int    `json:"line"`
	ID     string `json:"id,omitempty"`
	Reason string `json:"reason"`
}

type MissingChunkRef struct {
	Line     int    `json:"line"`
	VectorID string `json:"vector_id,omitempty"`
	ChunkID  string `json:"chunk_id,omitempty"`
	Reason   string `json:"reason"`
}

type VectorReportResult struct {
	Status              string                `json:"status"`
	VectorCount         int                   `json:"vector_count"`
	ManifestVectorCount int                   `json:"manifest_vector_count"`
	ManifestChunkCount  int                   `json:"manifest_chunk_count"`
	Dimensions          int                   `json:"dimensions"`
	Provider            string                `json:"provider"`
	Model               string                `json:"model"`
	Mode                string                `json:"mode"`
	FakeVectors         bool                  `json:"fake_vectors"`
	LanceDBWritten      bool                  `json:"lancedb_written"`
	DuplicateVectorIDs  []string              `json:"duplicate_vector_ids"`
	InvalidVectors      []InvalidVectorReport `json:"invalid_vectors"`
	MissingChunkRefs    []MissingChunkRef     `json:"missing_chunk_refs,omitempty"`
	AvgVectorNorm       float64               `json:"avg_vector_norm"`
	MinVectorNorm       float64               `json:"min_vector_norm"`
	MaxVectorNorm       float64               `json:"max_vector_norm"`
	Warnings            []string              `json:"warnings"`
}

func LoadEmbeddingManifest(path string) (EmbeddingManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EmbeddingManifest{}, fmt.Errorf("read embedding manifest %q: %w", path, err)
	}
	var manifest EmbeddingManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return EmbeddingManifest{}, fmt.Errorf("parse embedding manifest %q: %w", path, err)
	}
	return manifest, nil
}

func LoadVectorsJSONL(path string) ([]VectorRecord, []InvalidVectorReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open vectors jsonl %q: %w", path, err)
	}
	defer file.Close()

	var vectors []VectorRecord
	var invalid []InvalidVectorReport
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var vector VectorRecord
		if err := json.Unmarshal([]byte(line), &vector); err != nil {
			invalid = append(invalid, InvalidVectorReport{
				Line:   lineNo,
				Reason: fmt.Sprintf("invalid json: %v", err),
			})
			continue
		}
		vectors = append(vectors, vector)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read vectors jsonl %q: %w", path, err)
	}
	return vectors, invalid, nil
}

func Report(manifestPath string, vectorsPath string, chunksPath string) (VectorReportResult, error) {
	manifest, err := LoadEmbeddingManifest(manifestPath)
	if err != nil {
		return VectorReportResult{}, err
	}
	vectors, parseIssues, err := LoadVectorsJSONL(vectorsPath)
	if err != nil {
		return VectorReportResult{}, err
	}

	invalid := append([]InvalidVectorReport(nil), parseIssues...)
	missingChunkRefs := []MissingChunkRef{}
	warnings := []string{}

	if manifest.VectorCount != len(vectors) {
		invalid = append(invalid, InvalidVectorReport{
			Line:   0,
			Reason: fmt.Sprintf("manifest vector_count %d != vectors jsonl lines %d", manifest.VectorCount, len(vectors)),
		})
	}
	if manifest.ChunkCount != manifest.VectorCount {
		invalid = append(invalid, InvalidVectorReport{
			Line:   0,
			Reason: fmt.Sprintf("manifest chunk_count %d != manifest vector_count %d", manifest.ChunkCount, manifest.VectorCount),
		})
	}
	if strings.TrimSpace(manifest.Provider) == "" {
		invalid = append(invalid, InvalidVectorReport{Line: 0, Reason: "manifest provider is required"})
	}
	if strings.TrimSpace(manifest.Model) == "" {
		invalid = append(invalid, InvalidVectorReport{Line: 0, Reason: "manifest model is required"})
	}
	if strings.TrimSpace(manifest.Mode) == "" {
		invalid = append(invalid, InvalidVectorReport{Line: 0, Reason: "manifest mode is required"})
	}
	if manifest.Mode == ModeFakeVectors && !manifest.FakeVectors {
		invalid = append(invalid, InvalidVectorReport{Line: 0, Reason: "manifest fake_vectors must be true for fake_vectors mode"})
	}
	if manifest.LanceDBWritten {
		invalid = append(invalid, InvalidVectorReport{Line: 0, Reason: "manifest lancedb_written must be false"})
	}

	idCounts := map[string]int{}
	var normTotal float64
	minNorm := math.MaxFloat64
	maxNorm := 0.0
	normCount := 0

	for i, vector := range vectors {
		line := i + 1
		if strings.TrimSpace(vector.ID) == "" {
			invalid = append(invalid, InvalidVectorReport{Line: line, Reason: "missing vector id"})
		} else {
			idCounts[vector.ID]++
		}
		if strings.TrimSpace(vector.ChunkID) == "" {
			invalid = append(invalid, InvalidVectorReport{Line: line, ID: vector.ID, Reason: "missing chunk_id"})
		}
		if strings.TrimSpace(vector.Provider) == "" {
			invalid = append(invalid, InvalidVectorReport{Line: line, ID: vector.ID, Reason: "missing provider"})
		}
		if strings.TrimSpace(vector.EmbeddingModel) == "" {
			invalid = append(invalid, InvalidVectorReport{Line: line, ID: vector.ID, Reason: "missing embedding_model"})
		}
		if vector.Provider != manifest.Provider {
			invalid = append(invalid, InvalidVectorReport{
				Line:   line,
				ID:     vector.ID,
				Reason: fmt.Sprintf("provider %q != manifest provider %q", vector.Provider, manifest.Provider),
			})
		}
		if vector.EmbeddingModel != manifest.Model {
			invalid = append(invalid, InvalidVectorReport{
				Line:   line,
				ID:     vector.ID,
				Reason: fmt.Sprintf("embedding_model %q != manifest model %q", vector.EmbeddingModel, manifest.Model),
			})
		}
		if vector.Dimensions != manifest.Dimensions {
			invalid = append(invalid, InvalidVectorReport{
				Line:   line,
				ID:     vector.ID,
				Reason: fmt.Sprintf("record dimensions %d != manifest dimensions %d", vector.Dimensions, manifest.Dimensions),
			})
		}
		if len(vector.Vector) != manifest.Dimensions {
			invalid = append(invalid, InvalidVectorReport{
				Line:   line,
				ID:     vector.ID,
				Reason: fmt.Sprintf("vector length %d != manifest dimensions %d", len(vector.Vector), manifest.Dimensions),
			})
		}
		if vector.VectorSHA256 != sha256Hex(mustMarshalVector(vector.Vector)) {
			invalid = append(invalid, InvalidVectorReport{Line: line, ID: vector.ID, Reason: "vector_sha256 mismatch"})
		}

		if len(vector.Vector) > 0 {
			norm := vectorNorm(vector.Vector)
			normTotal += norm
			normCount++
			if norm < minNorm {
				minNorm = norm
			}
			if norm > maxNorm {
				maxNorm = norm
			}
		}
	}

	duplicateIDs := make([]string, 0)
	for id, count := range idCounts {
		if count > 1 {
			duplicateIDs = append(duplicateIDs, id)
		}
	}
	sort.Strings(duplicateIDs)

	chunksPath = strings.TrimSpace(chunksPath)
	if chunksPath != "" {
		chunksByID, chunkErr := loadChunksByID(chunksPath)
		if chunkErr != nil {
			return VectorReportResult{}, chunkErr
		}
		for i, vector := range vectors {
			line := i + 1
			chunk, ok := chunksByID[vector.ChunkID]
			if !ok {
				missingChunkRefs = append(missingChunkRefs, MissingChunkRef{
					Line:     line,
					VectorID: vector.ID,
					ChunkID:  vector.ChunkID,
					Reason:   "chunk_id not found in chunks jsonl",
				})
				continue
			}
			if vector.TextSHA256 != chunk.TextSHA256 {
				missingChunkRefs = append(missingChunkRefs, MissingChunkRef{
					Line:     line,
					VectorID: vector.ID,
					ChunkID:  vector.ChunkID,
					Reason:   "text_sha256 mismatch with chunk",
				})
			}
			if vector.SourceSHA256 != chunk.SourceSHA256 {
				missingChunkRefs = append(missingChunkRefs, MissingChunkRef{
					Line:     line,
					VectorID: vector.ID,
					ChunkID:  vector.ChunkID,
					Reason:   "source_sha256 mismatch with chunk",
				})
			}
		}
	}

	avgNorm := 0.0
	if normCount > 0 {
		avgNorm = normTotal / float64(normCount)
	}
	if normCount == 0 {
		minNorm = 0
	}

	status := vectorReportStatus(len(invalid), len(duplicateIDs), len(missingChunkRefs), warnings)

	return VectorReportResult{
		Status:              status,
		VectorCount:         len(vectors),
		ManifestVectorCount: manifest.VectorCount,
		ManifestChunkCount:  manifest.ChunkCount,
		Dimensions:          manifest.Dimensions,
		Provider:            manifest.Provider,
		Model:               manifest.Model,
		Mode:                manifest.Mode,
		FakeVectors:         manifest.FakeVectors,
		LanceDBWritten:      manifest.LanceDBWritten,
		DuplicateVectorIDs:  duplicateIDs,
		InvalidVectors:      invalid,
		MissingChunkRefs:    missingChunkRefs,
		AvgVectorNorm:       avgNorm,
		MinVectorNorm:       minNorm,
		MaxVectorNorm:       maxNorm,
		Warnings:            warnings,
	}, nil
}

func loadChunksByID(path string) (map[string]chunkRef, error) {
	chunks, err := loadChunkRefsJSONL(path)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]chunkRef, len(chunks))
	for _, chunk := range chunks {
		byID[chunk.ID] = chunk
	}
	return byID, nil
}

type chunkRef struct {
	ID           string
	TextSHA256   string
	SourceSHA256 string
}

func loadChunkRefsJSONL(path string) ([]chunkRef, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open chunks jsonl %q: %w", path, err)
	}
	defer file.Close()

	var chunks []chunkRef
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk struct {
			ID           string `json:"id"`
			TextSHA256   string `json:"text_sha256"`
			SourceSHA256 string `json:"source_sha256"`
		}
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			return nil, fmt.Errorf("parse chunks jsonl %q: %w", path, err)
		}
		chunks = append(chunks, chunkRef{
			ID:           chunk.ID,
			TextSHA256:   chunk.TextSHA256,
			SourceSHA256: chunk.SourceSHA256,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read chunks jsonl %q: %w", path, err)
	}
	return chunks, nil
}

func mustMarshalVector(vector []float64) []byte {
	data, err := json.Marshal(vector)
	if err != nil {
		return []byte("[]")
	}
	return data
}

func vectorNorm(vector []float64) float64 {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	return math.Sqrt(sum)
}

func vectorReportStatus(invalidCount int, duplicateCount int, missingChunkCount int, warnings []string) string {
	if invalidCount > 0 || duplicateCount > 0 || missingChunkCount > 0 {
		return StatusFailed
	}
	if len(warnings) > 0 {
		return StatusWarning
	}
	return StatusOK
}

func WriteVectorReportText(result VectorReportResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_embedding_report:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "vector_count: %d\n", result.VectorCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "manifest_vector_count: %d\n", result.ManifestVectorCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "manifest_chunk_count: %d\n", result.ManifestChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "dimensions: %d\n", result.Dimensions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "provider: %s\n", result.Provider); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "model: %s\n", result.Model); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "mode: %s\n", result.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "fake_vectors: %t\n", result.FakeVectors); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "lancedb_written: %t\n", result.LanceDBWritten); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "avg_vector_norm: %.6f\n", result.AvgVectorNorm); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "min_vector_norm: %.6f\n", result.MinVectorNorm); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_vector_norm: %.6f\n", result.MaxVectorNorm); err != nil {
		return err
	}
	if len(result.DuplicateVectorIDs) > 0 {
		if _, err := fmt.Fprintf(out, "duplicate_vector_ids: %s\n", strings.Join(result.DuplicateVectorIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.InvalidVectors) > 0 {
		if _, err := fmt.Fprintf(out, "\ninvalid_vectors:\n"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "line\tid\treason"); err != nil {
			return err
		}
		for _, item := range result.InvalidVectors {
			if _, err := fmt.Fprintf(table, "%d\t%s\t%s\n", item.Line, item.ID, item.Reason); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	if len(result.MissingChunkRefs) > 0 {
		if _, err := fmt.Fprintf(out, "\nmissing_chunk_refs:\n"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "line\tvector_id\tchunk_id\treason"); err != nil {
			return err
		}
		for _, item := range result.MissingChunkRefs {
			if _, err := fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", item.Line, item.VectorID, item.ChunkID, item.Reason); err != nil {
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

func WriteVectorReportJSON(result VectorReportResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
