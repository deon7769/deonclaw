package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

var forbiddenMaterializedFields = []string{
	"vector",
	"embedding",
	"raw_embedding",
	"env",
	"secret",
}

type MaterializedReportResult struct {
	Status             string   `json:"status"`
	ChunkCount         int      `json:"chunk_count"`
	IncludedChunkCount int      `json:"included_chunk_count"`
	OmittedChunkCount  int      `json:"omitted_chunk_count"`
	TotalCharsIncluded int      `json:"total_chars_included"`
	MaxTotalChars      int      `json:"max_total_chars"`
	MaxCharsPerChunk   int      `json:"max_chars_per_chunk"`
	UniqueChunkIDs     []string `json:"unique_chunk_ids"`
	Warnings           []string `json:"warnings"`
	Failures           []string `json:"failures,omitempty"`
}

type materializedArtifactFile struct {
	Status                       string            `json:"status"`
	SourceRetrievalContextSHA256 string            `json:"source_retrieval_context_sha256"`
	SourceChunksSHA256           string            `json:"source_chunks_sha256"`
	ChunkCount                   int               `json:"chunk_count"`
	IncludedChunkCount           int               `json:"included_chunk_count"`
	OmittedChunkCount            int               `json:"omitted_chunk_count"`
	MaxCharsPerChunk             int               `json:"max_chars_per_chunk"`
	MaxTotalChars                int               `json:"max_total_chars"`
	TotalCharsIncluded           int               `json:"total_chars_included"`
	Warnings                     []string          `json:"warnings"`
	Items                        []json.RawMessage `json:"items"`
}

func MaterializedReport(path string) (MaterializedReportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedReportResult{}, fmt.Errorf("read materialized artifact %q: %w", path, err)
	}
	return MaterializedReportBytes(data)
}

func MaterializedReportBytes(data []byte) (MaterializedReportResult, error) {
	var file materializedArtifactFile
	if err := json.Unmarshal(data, &file); err != nil {
		return MaterializedReportResult{}, fmt.Errorf("parse materialized artifact: %w", err)
	}

	result := MaterializedReportResult{
		Status:             lancedbpolicy.StatusOK,
		ChunkCount:         file.ChunkCount,
		IncludedChunkCount: file.IncludedChunkCount,
		OmittedChunkCount:  file.OmittedChunkCount,
		TotalCharsIncluded: file.TotalCharsIncluded,
		MaxTotalChars:      file.MaxTotalChars,
		MaxCharsPerChunk:   file.MaxCharsPerChunk,
		Warnings:           append([]string(nil), file.Warnings...),
	}

	var failures []string
	for _, field := range forbiddenMaterializedFields {
		if hasJSONField(data, field) {
			failures = append(failures, fmt.Sprintf("artifact contains forbidden field %q", field))
		}
	}

	artifactStatus := strings.TrimSpace(file.Status)
	switch artifactStatus {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		failures = append(failures, fmt.Sprintf("artifact status %q must be ok or warning", file.Status))
	}
	if strings.TrimSpace(file.SourceRetrievalContextSHA256) == "" {
		failures = append(failures, "source_retrieval_context_sha256 is required")
	}
	if strings.TrimSpace(file.SourceChunksSHA256) == "" {
		failures = append(failures, "source_chunks_sha256 is required")
	}
	if file.ChunkCount != file.IncludedChunkCount+file.OmittedChunkCount {
		failures = append(failures, fmt.Sprintf("chunk_count %d != included_chunk_count %d + omitted_chunk_count %d", file.ChunkCount, file.IncludedChunkCount, file.OmittedChunkCount))
	}
	if file.IncludedChunkCount != len(file.Items) {
		failures = append(failures, fmt.Sprintf("included_chunk_count %d != items length %d", file.IncludedChunkCount, len(file.Items)))
	}
	if file.MaxCharsPerChunk <= 0 {
		failures = append(failures, "max_chars_per_chunk must be > 0")
	}
	if file.MaxTotalChars <= 0 {
		failures = append(failures, "max_total_chars must be > 0")
	}

	chunkSet := map[string]struct{}{}
	var charSum int
	for i, rawItem := range file.Items {
		label := fmt.Sprintf("items[%d]", i)
		for _, field := range forbiddenMaterializedFields {
			if hasJSONField(rawItem, field) {
				failures = append(failures, fmt.Sprintf("%s contains forbidden field %q", label, field))
			}
		}
		var item MaterializeItem
		if err := json.Unmarshal(rawItem, &item); err != nil {
			failures = append(failures, fmt.Sprintf("%s is not a valid item object: %v", label, err))
			continue
		}
		if item.Rank <= 0 {
			failures = append(failures, fmt.Sprintf("%s rank must be > 0", label))
		}
		if strings.TrimSpace(item.ChunkID) == "" {
			failures = append(failures, fmt.Sprintf("%s chunk_id must not be empty", label))
		} else {
			chunkSet[item.ChunkID] = struct{}{}
		}
		if strings.TrimSpace(item.TextSHA256) == "" {
			failures = append(failures, fmt.Sprintf("%s text_sha256 must not be empty", label))
		}
		if item.IncludedChars > 0 && strings.TrimSpace(item.TextExcerpt) == "" {
			failures = append(failures, fmt.Sprintf("%s text_excerpt is required when included_chars > 0", label))
		}
		excerptRunes := utf8.RuneCountInString(item.TextExcerpt)
		if item.IncludedChars != excerptRunes {
			failures = append(failures, fmt.Sprintf("%s included_chars %d != text_excerpt rune count %d", label, item.IncludedChars, excerptRunes))
		}
		if item.IncludedChars > file.MaxCharsPerChunk {
			failures = append(failures, fmt.Sprintf("%s included_chars %d exceeds max_chars_per_chunk %d", label, item.IncludedChars, file.MaxCharsPerChunk))
		}
		charSum += item.IncludedChars
	}
	if charSum != file.TotalCharsIncluded {
		failures = append(failures, fmt.Sprintf("total_chars_included %d != sum of item included_chars %d", file.TotalCharsIncluded, charSum))
	}
	if file.TotalCharsIncluded > file.MaxTotalChars {
		failures = append(failures, fmt.Sprintf("total_chars_included %d exceeds max_total_chars %d", file.TotalCharsIncluded, file.MaxTotalChars))
	}

	for chunkID := range chunkSet {
		result.UniqueChunkIDs = append(result.UniqueChunkIDs, chunkID)
	}
	sort.Strings(result.UniqueChunkIDs)

	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if artifactStatus == lancedbpolicy.StatusWarning || len(result.Warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}
	return result, nil
}

func WriteMaterializedReportText(result MaterializedReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_materialized_report:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "chunk_count: %d\n", result.ChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "included_chunk_count: %d\n", result.IncludedChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "omitted_chunk_count: %d\n", result.OmittedChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total_chars_included: %d\n", result.TotalCharsIncluded); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_total_chars: %d\n", result.MaxTotalChars); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_chars_per_chunk: %d\n", result.MaxCharsPerChunk); err != nil {
		return err
	}
	if len(result.UniqueChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "unique_chunk_ids: %s\n", strings.Join(result.UniqueChunkIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
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
		if _, err := fmt.Fprintln(out, "\nwarnings:"); err != nil {
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

func WriteMaterializedReportJSON(result MaterializedReportResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
