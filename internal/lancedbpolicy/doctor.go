package lancedbpolicy

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

type DoctorOptions struct {
	Reader LanceDBReader
}

type DoctorResult struct {
	Status             string   `json:"status"`
	DatabasePath       string   `json:"database_path"`
	Table              string   `json:"table"`
	TableExists        bool     `json:"table_exists"`
	RowCount           int      `json:"row_count"`
	ExpectedDimensions int      `json:"expected_dimensions"`
	InferredDimensions int      `json:"inferred_dimensions"`
	Columns            []string `json:"columns"`
	Warnings           []string `json:"warnings"`
}

func Doctor(cfg Config, opts DoctorOptions) (DoctorResult, error) {
	if err := Validate(cfg); err != nil {
		return DoctorResult{}, err
	}
	p := cfg.LanceDBPolicy

	reader := opts.Reader
	if reader == nil {
		preflight := PreflightReadback()
		if preflight.Status != StatusOK {
			return DoctorResult{}, fmt.Errorf("lancedb readback preflight failed: %s", preflight.Message)
		}
		reader = DefaultReader
	}

	result := DoctorResult{
		Status:             StatusOK,
		DatabasePath:       p.Database.Path,
		Table:              p.Database.Table,
		ExpectedDimensions: p.Limits.ExpectedDimensions,
	}

	if _, err := os.Stat(p.Database.Path); os.IsNotExist(err) {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("database path does not exist: %s", p.Database.Path))
		return result, nil
	}

	readback, err := reader.Readback(ReadbackRequest{
		DatabasePath:       p.Database.Path,
		Table:              p.Database.Table,
		VectorColumn:       p.Schema.VectorColumn,
		TextRefColumn:      p.Schema.TextRefColumn,
		MetadataColumns:    append([]string(nil), p.Schema.MetadataColumns...),
		ExpectedDimensions: p.Limits.ExpectedDimensions,
	})
	if err != nil {
		return DoctorResult{}, err
	}

	result.TableExists = readback.TableExists
	result.RowCount = readback.RowCount
	result.InferredDimensions = readback.InferredDimensions
	result.Columns = append([]string(nil), readback.Columns...)

	evaluateDoctorReadback(&result, p, readback)
	return result, nil
}

func evaluateDoctorReadback(result *DoctorResult, p Policy, readback ReadbackResponse) {
	if !readback.TableExists {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("table %q does not exist in database %q", p.Database.Table, p.Database.Path))
	}
	if readback.RowCount == 0 {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, "row_count is 0")
	}
	if !readback.VectorColumnExists {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("vector column %q is missing", p.Schema.VectorColumn))
	}
	if !readback.TextRefColumnExists {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("text reference column %q is missing", p.Schema.TextRefColumn))
	}
	for _, column := range p.Schema.MetadataColumns {
		found := false
		for _, present := range readback.MetadataColumnsPresent {
			if present == column {
				found = true
				break
			}
		}
		if !found {
			result.Status = StatusFailed
			result.Warnings = append(result.Warnings, fmt.Sprintf("metadata column %q is missing", column))
		}
	}
	if readback.InferredDimensions > 0 && readback.InferredDimensions != p.Limits.ExpectedDimensions {
		result.Status = StatusFailed
		result.Warnings = append(result.Warnings, fmt.Sprintf("inferred dimensions %d != expected_dimensions %d", readback.InferredDimensions, p.Limits.ExpectedDimensions))
	}
	if result.Status == StatusOK && len(result.Warnings) > 0 {
		result.Status = StatusWarning
	}
}

func WriteDoctorText(result DoctorResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_lancedb_doctor:\n"); err != nil {
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
	if _, err := fmt.Fprintf(out, "table_exists: %t\n", result.TableExists); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "row_count: %d\n", result.RowCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "expected_dimensions: %d\n", result.ExpectedDimensions); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "inferred_dimensions: %d\n", result.InferredDimensions); err != nil {
		return err
	}
	if len(result.Columns) > 0 {
		if _, err := fmt.Fprintf(out, "columns: %s\n", strings.Join(result.Columns, ", ")); err != nil {
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

func LoadWriteSmokeManifest(path string) (WriteSmokeManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WriteSmokeManifest{}, fmt.Errorf("read write-smoke manifest %q: %w", path, err)
	}
	var manifest WriteSmokeManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return WriteSmokeManifest{}, fmt.Errorf("parse write-smoke manifest %q: %w", path, err)
	}
	return manifest, nil
}

type ReadbackReportOptions struct {
	Reader LanceDBReader
}

type ReadbackReportResult struct {
	Status             string   `json:"status"`
	RowCount           int      `json:"row_count"`
	ManifestRowCount   int      `json:"manifest_row_count"`
	Table              string   `json:"table"`
	DatabasePath       string   `json:"database_path"`
	RetrievalPerformed bool     `json:"retrieval_performed"`
	RunnerIntegration  bool     `json:"runner_integration"`
	SampleChunkIDs     []string `json:"sample_chunk_ids"`
	Warnings           []string `json:"warnings"`
	Failures           []string `json:"failures"`
}

func ReadbackReport(manifestPath string, cfg Config, opts ReadbackReportOptions) (ReadbackReportResult, error) {
	if err := Validate(cfg); err != nil {
		return ReadbackReportResult{}, err
	}
	manifest, err := LoadWriteSmokeManifest(manifestPath)
	if err != nil {
		return ReadbackReportResult{}, err
	}
	p := cfg.LanceDBPolicy

	reader := opts.Reader
	if reader == nil {
		preflight := PreflightReadback()
		if preflight.Status != StatusOK {
			return ReadbackReportResult{}, fmt.Errorf("lancedb readback preflight failed: %s", preflight.Message)
		}
		reader = DefaultReader
	}

	readback, err := reader.Readback(ReadbackRequest{
		DatabasePath:       p.Database.Path,
		Table:              p.Database.Table,
		VectorColumn:       p.Schema.VectorColumn,
		TextRefColumn:      p.Schema.TextRefColumn,
		MetadataColumns:    append([]string(nil), p.Schema.MetadataColumns...),
		ExpectedDimensions: p.Limits.ExpectedDimensions,
	})
	if err != nil {
		return ReadbackReportResult{}, err
	}

	result := ReadbackReportResult{
		Status:             StatusOK,
		RowCount:           readback.RowCount,
		ManifestRowCount:   manifest.RowCount,
		Table:              manifest.Table,
		DatabasePath:       manifest.DatabasePath,
		RetrievalPerformed: manifest.RetrievalPerformed,
		RunnerIntegration:  manifest.RunnerIntegration,
		SampleChunkIDs:     append([]string(nil), readback.SampleChunkIDs...),
	}

	failures := validateReadbackManifest(manifest, p, readback)
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = StatusFailed
	}
	if manifest.RowCount != readback.RowCount {
		result.Failures = append(result.Failures, fmt.Sprintf("manifest row_count %d != readback row_count %d", manifest.RowCount, readback.RowCount))
		result.Status = StatusFailed
	}

	doctor := DoctorResult{
		Status:             StatusOK,
		ExpectedDimensions: p.Limits.ExpectedDimensions,
	}
	evaluateDoctorReadback(&doctor, p, readback)
	for _, warning := range doctor.Warnings {
		result.Failures = append(result.Failures, warning)
	}
	if doctor.Status == StatusFailed {
		result.Status = StatusFailed
	}

	return result, nil
}

func validateReadbackManifest(manifest WriteSmokeManifest, p Policy, readback ReadbackResponse) []string {
	var failures []string
	if !manifest.LanceDBWritten {
		failures = append(failures, "manifest lancedb_written must be true")
	}
	if manifest.RetrievalPerformed {
		failures = append(failures, "manifest retrieval_performed must be false")
	}
	if manifest.RunnerIntegration {
		failures = append(failures, "manifest runner_integration must be false")
	}
	if manifest.Table != p.Database.Table {
		failures = append(failures, fmt.Sprintf("manifest table %q != policy table %q", manifest.Table, p.Database.Table))
	}
	if manifest.DatabasePath != p.Database.Path {
		failures = append(failures, fmt.Sprintf("manifest database_path %q != policy database_path %q", manifest.DatabasePath, p.Database.Path))
	}
	if manifest.Dimensions != p.Limits.ExpectedDimensions {
		failures = append(failures, fmt.Sprintf("manifest dimensions %d != expected_dimensions %d", manifest.Dimensions, p.Limits.ExpectedDimensions))
	}
	if readback.InferredDimensions > 0 && manifest.Dimensions != readback.InferredDimensions {
		failures = append(failures, fmt.Sprintf("manifest dimensions %d != readback inferred_dimensions %d", manifest.Dimensions, readback.InferredDimensions))
	}
	return failures
}

func WriteReadbackReportText(result ReadbackReportResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_lancedb_report:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "row_count: %d\n", result.RowCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "manifest_row_count: %d\n", result.ManifestRowCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "table: %s\n", result.Table); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "database_path: %s\n", result.DatabasePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "retrieval_performed: %t\n", result.RetrievalPerformed); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "runner_integration: %t\n", result.RunnerIntegration); err != nil {
		return err
	}
	if len(result.SampleChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "sample_chunk_ids: %s\n", strings.Join(result.SampleChunkIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintf(out, "\nfailures:\n"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(table, "- %s\n", failure); err != nil {
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

func WriteReadbackReportJSON(result ReadbackReportResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
