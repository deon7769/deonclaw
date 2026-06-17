package embeddingpolicy

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/memoryindex"
)

type VectorRecord struct {
	ID             string    `json:"id"`
	ChunkID        string    `json:"chunk_id"`
	Domain         string    `json:"domain"`
	SourcePath     string    `json:"source_path"`
	SourceSHA256   string    `json:"source_sha256"`
	TextSHA256     string    `json:"text_sha256"`
	EmbeddingModel string    `json:"embedding_model"`
	Provider       string    `json:"provider"`
	Dimensions     int       `json:"dimensions"`
	Vector         []float64 `json:"vector"`
	VectorSHA256   string    `json:"vector_sha256"`
}

type EmbeddingManifest struct {
	GeneratedAt         string `json:"generated_at"`
	PolicySHA256        string `json:"policy_sha256"`
	InputManifestSHA256 string `json:"input_manifest_sha256"`
	InputChunksSHA256   string `json:"input_chunks_sha256"`
	ChunkCount          int    `json:"chunk_count"`
	VectorCount         int    `json:"vector_count"`
	Dimensions          int    `json:"dimensions"`
	Provider            string `json:"provider"`
	Model               string `json:"model"`
	Mode                string `json:"mode"`
	FakeVectors         bool   `json:"fake_vectors"`
	DryRunOnly          bool   `json:"dry_run_only"`
	LanceDBWritten      bool   `json:"lancedb_written"`
	EstimatedBatches    int    `json:"estimated_batches"`
	OutputVectorsPath   string `json:"output_vectors_path"`
	OutputManifestPath  string `json:"output_manifest_path"`
}

type BuildFakeResult struct {
	Manifest EmbeddingManifest `json:"manifest"`
}

func BuildFake(cfg Config, policyBytes []byte, artifactsDir string, confirmFake bool) (BuildFakeResult, error) {
	if !confirmFake {
		return BuildFakeResult{}, fmt.Errorf("--confirm-fake-vectors is required")
	}
	if err := Validate(cfg); err != nil {
		return BuildFakeResult{}, err
	}
	p := cfg.EmbeddingPolicy
	if p.Provider != ProviderLocal {
		return BuildFakeResult{}, fmt.Errorf("embedding_policy.provider must be local for fake vector generation")
	}
	if p.Mode != ModeFakeVectors {
		return BuildFakeResult{}, fmt.Errorf("embedding_policy.mode must be fake_vectors for fake vector generation")
	}
	if strings.TrimSpace(artifactsDir) == "" {
		return BuildFakeResult{}, fmt.Errorf("artifacts dir is required")
	}
	if err := validateArtifactsDir(cfg, artifactsDir); err != nil {
		return BuildFakeResult{}, err
	}

	indexReport, err := memoryindex.Report(p.Input.ManifestPath, p.Input.ChunksPath)
	if err != nil {
		return BuildFakeResult{}, err
	}
	if indexReport.Status == memoryindex.StatusFailed {
		return BuildFakeResult{}, fmt.Errorf("memory index report failed; refusing to generate fake vectors")
	}
	if indexReport.ChunkCount > p.Limits.MaxChunks {
		return BuildFakeResult{}, fmt.Errorf("chunk_count %d exceeds max_chunks %d", indexReport.ChunkCount, p.Limits.MaxChunks)
	}
	if indexReport.MaxChunkChars > p.Limits.MaxChunkChars {
		return BuildFakeResult{}, fmt.Errorf("max chunk chars %d exceeds max_chunk_chars %d", indexReport.MaxChunkChars, p.Limits.MaxChunkChars)
	}

	chunks, err := memoryindex.LoadChunksJSONL(p.Input.ChunksPath)
	if err != nil {
		return BuildFakeResult{}, err
	}
	for _, chunk := range chunks {
		if len([]rune(chunk.Text)) > p.Limits.MaxChunkChars {
			return BuildFakeResult{}, fmt.Errorf("chunk %q exceeds max_chunk_chars %d", chunk.ID, p.Limits.MaxChunkChars)
		}
	}

	vectorsPath := p.Output.VectorsPath
	manifestPath := p.Output.ManifestPath
	if err := os.MkdirAll(filepath.Dir(vectorsPath), 0o755); err != nil {
		return BuildFakeResult{}, fmt.Errorf("create vectors dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return BuildFakeResult{}, fmt.Errorf("create manifest dir: %w", err)
	}

	vectors := make([]VectorRecord, 0, len(chunks))
	for _, chunk := range chunks {
		vector, vectorSHA, err := deterministicVector(chunk.TextSHA256, chunk.ID, p.Dimensions)
		if err != nil {
			return BuildFakeResult{}, err
		}
		vectors = append(vectors, VectorRecord{
			ID:             vectorID(chunk.ID),
			ChunkID:        chunk.ID,
			Domain:         chunk.Domain,
			SourcePath:     chunk.SourcePath,
			SourceSHA256:   chunk.SourceSHA256,
			TextSHA256:     chunk.TextSHA256,
			EmbeddingModel: p.Model,
			Provider:       p.Provider,
			Dimensions:     p.Dimensions,
			Vector:         vector,
			VectorSHA256:   vectorSHA,
		})
	}

	if err := writeVectorsJSONL(vectorsPath, vectors); err != nil {
		return BuildFakeResult{}, err
	}

	inputManifestSHA, err := fileSHA256(p.Input.ManifestPath)
	if err != nil {
		return BuildFakeResult{}, err
	}
	inputChunksSHA, err := fileSHA256(p.Input.ChunksPath)
	if err != nil {
		return BuildFakeResult{}, err
	}

	manifest := EmbeddingManifest{
		GeneratedAt:         time.Now().UTC().Format(time.RFC3339Nano),
		PolicySHA256:        PolicySHA256(policyBytes),
		InputManifestSHA256: inputManifestSHA,
		InputChunksSHA256:   inputChunksSHA,
		ChunkCount:          len(chunks),
		VectorCount:         len(vectors),
		Dimensions:          p.Dimensions,
		Provider:            p.Provider,
		Model:               p.Model,
		Mode:                p.Mode,
		FakeVectors:         true,
		DryRunOnly:          false,
		LanceDBWritten:      false,
		EstimatedBatches:    EstimatedBatches(len(chunks), p.Limits.BatchSize),
		OutputVectorsPath:   vectorsPath,
		OutputManifestPath:  manifestPath,
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return BuildFakeResult{}, err
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := os.WriteFile(manifestPath, manifestJSON, 0o644); err != nil {
		return BuildFakeResult{}, fmt.Errorf("write embedding manifest %q: %w", manifestPath, err)
	}

	return BuildFakeResult{Manifest: manifest}, nil
}

func validateArtifactsDir(cfg Config, artifactsDir string) error {
	expected := artifactsDirFromInput(cfg.EmbeddingPolicy.Input.ChunksPath)
	if filepath.Clean(strings.TrimSpace(artifactsDir)) != filepath.Clean(expected) {
		return fmt.Errorf("artifacts dir %q does not match policy input base %q", artifactsDir, expected)
	}
	return nil
}

func vectorID(chunkID string) string {
	return "vec:" + chunkID
}

func deterministicVector(textSHA256 string, chunkID string, dimensions int) ([]float64, string, error) {
	if dimensions <= 0 {
		return nil, "", fmt.Errorf("dimensions must be > 0")
	}
	base := sha256.Sum256([]byte(textSHA256 + ":" + chunkID))
	vector := make([]float64, dimensions)
	for i := 0; i < dimensions; i++ {
		seed := append(append([]byte(nil), base[:]...), byte(i), byte(i>>8), byte(i>>16))
		block := sha256.Sum256(seed)
		value := binary.BigEndian.Uint64(block[:8])
		vector[i] = (float64(value)/float64(math.MaxUint64))*2 - 1
	}
	vectorJSON, err := json.Marshal(vector)
	if err != nil {
		return nil, "", err
	}
	return vector, sha256Hex(vectorJSON), nil
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return sha256Hex(data), nil
}

func writeVectorsJSONL(path string, vectors []VectorRecord) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open vectors jsonl %q: %w", path, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, vector := range vectors {
		line, err := json.Marshal(vector)
		if err != nil {
			return err
		}
		if _, err := writer.Write(line); err != nil {
			return err
		}
		if err := writer.WriteByte('\n'); err != nil {
			return err
		}
	}
	return writer.Flush()
}
