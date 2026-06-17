package embeddingpolicy

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/memoryindex"
)

type PlanResult struct {
	Status              string                           `json:"status"`
	ChunksPath          string                           `json:"chunks_path"`
	ManifestPath        string                           `json:"manifest_path"`
	ChunkCount          int                              `json:"chunk_count"`
	MaxChunks           int                              `json:"max_chunks"`
	BatchSize           int                              `json:"batch_size"`
	EstimatedBatches    int                              `json:"estimated_batches"`
	OutputVectorsPath   string                           `json:"output_vectors_path"`
	OutputManifestPath  string                           `json:"output_manifest_path"`
	WouldCallProvider   bool                             `json:"would_call_provider"`
	DryRunOnly          bool                             `json:"dry_run_only"`
	IndexReportStatus   string                           `json:"index_report_status,omitempty"`
	IndexReportWarnings []string                         `json:"index_report_warnings,omitempty"`
	IndexReportInvalid  []memoryindex.InvalidChunkReport `json:"index_report_invalid_chunks,omitempty"`
	Warnings            []string                         `json:"warnings"`
}

func Plan(cfg Config) (PlanResult, error) {
	if err := Validate(cfg); err != nil {
		return PlanResult{}, err
	}
	p := cfg.EmbeddingPolicy

	result := PlanResult{
		Status:             StatusOK,
		ChunksPath:         p.Input.ChunksPath,
		ManifestPath:       p.Input.ManifestPath,
		MaxChunks:          p.Limits.MaxChunks,
		BatchSize:          p.Limits.BatchSize,
		OutputVectorsPath:  p.Output.VectorsPath,
		OutputManifestPath: p.Output.ManifestPath,
		WouldCallProvider:  false,
		DryRunOnly:         p.Mode == ModeDryRun,
	}

	if !fileExists(p.Input.ChunksPath) {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("input chunks file missing: %s", p.Input.ChunksPath))
		return result, nil
	}
	if !fileExists(p.Input.ManifestPath) {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("input manifest file missing: %s", p.Input.ManifestPath))
		return result, nil
	}

	indexReport, err := memoryindex.Report(p.Input.ManifestPath, p.Input.ChunksPath)
	if err != nil {
		return PlanResult{}, err
	}
	result.IndexReportStatus = indexReport.Status
	result.IndexReportWarnings = indexReport.Warnings
	result.IndexReportInvalid = indexReport.InvalidChunks
	result.ChunkCount = indexReport.ChunkCount
	result.EstimatedBatches = EstimatedBatches(indexReport.ChunkCount, p.Limits.BatchSize)

	if indexReport.Status == memoryindex.StatusFailed {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, "memory index report failed consistency checks")
	}
	if indexReport.ChunkCount > p.Limits.MaxChunks {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("chunk_count %d exceeds max_chunks %d", indexReport.ChunkCount, p.Limits.MaxChunks))
	}
	if result.Status == StatusOK && len(result.Warnings) > 0 {
		result.Status = StatusWarning
	}
	return result, nil
}

func WritePlanText(result PlanResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_embedding_plan:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "chunks_path: %s\n", result.ChunksPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "manifest_path: %s\n", result.ManifestPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "chunk_count: %d\n", result.ChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_chunks: %d\n", result.MaxChunks); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "batch_size: %d\n", result.BatchSize); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "estimated_batches: %d\n", result.EstimatedBatches); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "output_vectors_path: %s\n", result.OutputVectorsPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "output_manifest_path: %s\n", result.OutputManifestPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "would_call_provider: %t\n", result.WouldCallProvider); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "dry_run_only: %t\n", result.DryRunOnly); err != nil {
		return err
	}
	if result.IndexReportStatus != "" {
		if _, err := fmt.Fprintf(out, "index_report_status: %s\n", result.IndexReportStatus); err != nil {
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

func DoctorOutputContainsEnvValue(output string, envName string, envValue string) bool {
	if envValue == "" {
		return false
	}
	return strings.Contains(output, envValue) || strings.Contains(output, envName+"="+envValue)
}
