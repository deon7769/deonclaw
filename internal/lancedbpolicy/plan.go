package lancedbpolicy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
)

type PlanResult struct {
	Status                string   `json:"status"`
	DatabasePath          string   `json:"database_path"`
	Table                 string   `json:"table"`
	VectorCount           int      `json:"vector_count"`
	ExpectedDimensions    int      `json:"expected_dimensions"`
	ManifestDimensions    int      `json:"manifest_dimensions"`
	Provider              string   `json:"provider"`
	Model                 string   `json:"model"`
	FakeVectors           bool     `json:"fake_vectors"`
	LanceDBWritten        bool     `json:"lancedb_written"`
	WouldWriteLanceDB     bool     `json:"would_write_lancedb"`
	PlanOnly              bool     `json:"plan_only"`
	EstimatedRows         int      `json:"estimated_rows"`
	EmbeddingReportStatus string   `json:"embedding_report_status,omitempty"`
	Warnings              []string `json:"warnings"`
}

func Plan(cfg Config) (PlanResult, error) {
	if err := Validate(cfg); err != nil {
		return PlanResult{}, err
	}
	p := cfg.LanceDBPolicy

	result := PlanResult{
		Status:             StatusOK,
		DatabasePath:       p.Database.Path,
		Table:              p.Database.Table,
		ExpectedDimensions: p.Limits.ExpectedDimensions,
		WouldWriteLanceDB:  false,
		PlanOnly:           p.Mode == ModePlanOnly,
	}

	if !fileExists(p.Input.EmbeddingManifest) {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("embedding manifest missing: %s", p.Input.EmbeddingManifest))
		return result, nil
	}
	if !fileExists(p.Input.VectorsPath) {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("vectors file missing: %s", p.Input.VectorsPath))
		return result, nil
	}
	chunksPath := strings.TrimSpace(p.Input.ChunksPath)
	if chunksPath != "" && !fileExists(chunksPath) {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("chunks file missing: %s", chunksPath))
		return result, nil
	}

	manifest, err := embeddingpolicy.LoadEmbeddingManifest(p.Input.EmbeddingManifest)
	if err != nil {
		return PlanResult{}, err
	}
	result.ManifestDimensions = manifest.Dimensions
	result.Provider = manifest.Provider
	result.Model = manifest.Model
	result.FakeVectors = manifest.FakeVectors
	result.LanceDBWritten = manifest.LanceDBWritten
	result.VectorCount = manifest.VectorCount
	result.EstimatedRows = manifest.VectorCount

	vectorReport, err := embeddingpolicy.Report(p.Input.EmbeddingManifest, p.Input.VectorsPath, chunksPath)
	if err != nil {
		return PlanResult{}, err
	}
	result.EmbeddingReportStatus = vectorReport.Status
	if vectorReport.Status == embeddingpolicy.StatusFailed {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, "embedding vector report failed consistency checks")
		return result, nil
	}

	if manifest.VectorCount > p.Limits.MaxVectors {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("vector_count %d exceeds max_vectors %d", manifest.VectorCount, p.Limits.MaxVectors))
	}
	if manifest.Dimensions != p.Limits.ExpectedDimensions {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("manifest dimensions %d != expected_dimensions %d", manifest.Dimensions, p.Limits.ExpectedDimensions))
	}
	if manifest.LanceDBWritten {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, "embedding manifest lancedb_written must be false before plan")
	}

	if result.Status == StatusOK && len(result.Warnings) > 0 {
		result.Status = StatusWarning
	}
	return result, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func WritePlanText(result PlanResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_lancedb_plan:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "database_path: %s\n", result.DatabasePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "table: %s\n", result.Table); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "vector_count: %d\n", result.VectorCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "expected_dimensions: %d\n", result.ExpectedDimensions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "manifest_dimensions: %d\n", result.ManifestDimensions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "provider: %s\n", result.Provider); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "model: %s\n", result.Model); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "fake_vectors: %t\n", result.FakeVectors); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "lancedb_written: %t\n", result.LanceDBWritten); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "would_write_lancedb: %t\n", result.WouldWriteLanceDB); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "plan_only: %t\n", result.PlanOnly); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "estimated_rows: %d\n", result.EstimatedRows); err != nil {
		return err
	}
	if result.EmbeddingReportStatus != "" {
		if _, err := fmt.Fprintf(out, "embedding_report_status: %s\n", result.EmbeddingReportStatus); err != nil {
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

func WritePlanJSON(result PlanResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
