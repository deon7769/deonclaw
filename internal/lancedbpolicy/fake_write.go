package lancedbpolicy

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
)

const (
	fakeWriteManifestName = "lancedb-fake-write-manifest.json"
	fakeWriteRowsName     = "lancedb-fake-rows.jsonl"
)

type FakeWriteRow struct {
	ChunkID          string `json:"chunk_id"`
	VectorID         string `json:"vector_id"`
	VectorDimensions int    `json:"vector_dimensions"`
	Domain           string `json:"domain"`
	SourcePath       string `json:"source_path"`
	SourceSHA256     string `json:"source_sha256"`
	TextSHA256       string `json:"text_sha256"`
	EmbeddingModel   string `json:"embedding_model"`
	Provider         string `json:"provider"`
	VectorSHA256     string `json:"vector_sha256"`
}

type FakeWriteManifest struct {
	GeneratedAt                  string `json:"generated_at"`
	PolicySHA256                 string `json:"policy_sha256"`
	InputEmbeddingManifestSHA256 string `json:"input_embedding_manifest_sha256"`
	InputVectorsSHA256           string `json:"input_vectors_sha256"`
	Table                        string `json:"table"`
	DatabasePath                 string `json:"database_path"`
	RowCount                     int    `json:"row_count"`
	Dimensions                   int    `json:"dimensions"`
	Mode                         string `json:"mode"`
	FakeWrite                    bool   `json:"fake_write"`
	LanceDBWritten               bool   `json:"lancedb_written"`
	SourceVectorsPath            string `json:"source_vectors_path"`
	OutputRowsPath               string `json:"output_rows_path"`
}

type FakeWriteResult struct {
	Manifest FakeWriteManifest `json:"manifest"`
	Plan     PlanResult        `json:"plan"`
}

func FakeWrite(cfg Config, policyBytes []byte, artifactsDir string, confirmFakeWrite bool) (FakeWriteResult, error) {
	if !confirmFakeWrite {
		return FakeWriteResult{}, fmt.Errorf("--confirm-fake-write is required")
	}
	if err := Validate(cfg); err != nil {
		return FakeWriteResult{}, err
	}
	p := cfg.LanceDBPolicy
	if p.Mode != ModeFakeWrite {
		return FakeWriteResult{}, fmt.Errorf("lancedb_policy.mode must be fake_write for fake write generation")
	}
	if err := validateArtifactsDir(cfg, artifactsDir); err != nil {
		return FakeWriteResult{}, err
	}

	plan, err := Plan(cfg)
	if err != nil {
		return FakeWriteResult{}, err
	}
	if plan.Status != StatusOK {
		return FakeWriteResult{}, fmt.Errorf("lancedb plan status %q; refusing fake write", plan.Status)
	}

	vectors, _, err := embeddingpolicy.LoadVectorsJSONL(p.Input.VectorsPath)
	if err != nil {
		return FakeWriteResult{}, err
	}

	rows := make([]FakeWriteRow, 0, len(vectors))
	for _, vector := range vectors {
		rows = append(rows, FakeWriteRow{
			ChunkID:          vector.ChunkID,
			VectorID:         vector.ID,
			VectorDimensions: vector.Dimensions,
			Domain:           vector.Domain,
			SourcePath:       vector.SourcePath,
			SourceSHA256:     vector.SourceSHA256,
			TextSHA256:       vector.TextSHA256,
			EmbeddingModel:   vector.EmbeddingModel,
			Provider:         vector.Provider,
			VectorSHA256:     vector.VectorSHA256,
		})
	}

	rowsPath := filepath.Join(artifactsDir, fakeWriteRowsName)
	manifestPath := filepath.Join(artifactsDir, fakeWriteManifestName)
	if err := writeFakeRowsJSONL(rowsPath, rows); err != nil {
		return FakeWriteResult{}, err
	}

	embeddingManifestSHA, err := fileSHA256(p.Input.EmbeddingManifest)
	if err != nil {
		return FakeWriteResult{}, err
	}
	vectorsSHA, err := fileSHA256(p.Input.VectorsPath)
	if err != nil {
		return FakeWriteResult{}, err
	}

	manifest := FakeWriteManifest{
		GeneratedAt:                  time.Now().UTC().Format(time.RFC3339Nano),
		PolicySHA256:                 policySHA256(policyBytes),
		InputEmbeddingManifestSHA256: embeddingManifestSHA,
		InputVectorsSHA256:           vectorsSHA,
		Table:                        p.Database.Table,
		DatabasePath:                 p.Database.Path,
		RowCount:                     len(rows),
		Dimensions:                   plan.ManifestDimensions,
		Mode:                         p.Mode,
		FakeWrite:                    true,
		LanceDBWritten:               false,
		SourceVectorsPath:            p.Input.VectorsPath,
		OutputRowsPath:               rowsPath,
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return FakeWriteResult{}, err
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := os.WriteFile(manifestPath, manifestJSON, 0o644); err != nil {
		return FakeWriteResult{}, fmt.Errorf("write fake write manifest %q: %w", manifestPath, err)
	}

	return FakeWriteResult{
		Manifest: manifest,
		Plan:     plan,
	}, nil
}

func writeFakeRowsJSONL(path string, rows []FakeWriteRow) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create fake rows dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open fake rows jsonl %q: %w", path, err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, row := range rows {
		line, err := json.Marshal(row)
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

func policySHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}
	return policySHA256(data), nil
}
