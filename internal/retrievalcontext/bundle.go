package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type BundleOptions struct {
	RetrievalContextPath string
	MaterializedPath     string
	OutputPath           string
	SummaryPath          string
}

type BundleItem struct {
	Rank          int     `json:"rank"`
	ChunkID       string  `json:"chunk_id"`
	Distance      float64 `json:"distance,omitempty"`
	Domain        string  `json:"domain,omitempty"`
	SourcePath    string  `json:"source_path,omitempty"`
	SourceSHA256  string  `json:"source_sha256,omitempty"`
	TextSHA256    string  `json:"text_sha256,omitempty"`
	Truncated     bool    `json:"truncated"`
	IncludedChars int     `json:"included_chars"`
}

type BundleResult struct {
	Status                     string       `json:"status"`
	ContainsText               bool         `json:"contains_text"`
	MaterializedTextArtifact   string       `json:"materialized_text_artifact"`
	RetrievalContextSHA256     string       `json:"retrieval_context_sha256"`
	MaterializedSHA256         string       `json:"materialized_sha256"`
	RetrievalContextPath       string       `json:"retrieval_context_path"`
	RetrievalHitCount          int          `json:"retrieval_hit_count"`
	MaterializedChunkCount     int          `json:"materialized_chunk_count"`
	IncludedChunkCount         int          `json:"included_chunk_count"`
	OmittedChunkCount          int          `json:"omitted_chunk_count"`
	TotalCharsIncluded         int          `json:"total_chars_included"`
	SourceChunksSHA256         string       `json:"source_chunks_sha256,omitempty"`
	RetrievalUniqueChunkIDs    []string     `json:"retrieval_unique_chunk_ids"`
	MaterializedUniqueChunkIDs []string     `json:"materialized_unique_chunk_ids"`
	Warnings                   []string     `json:"warnings,omitempty"`
	Items                      []BundleItem `json:"items,omitempty"`
}

func Bundle(opts BundleOptions) (BundleResult, error) {
	if err := validateRelativeSafePath("retrieval context path", opts.RetrievalContextPath); err != nil {
		return BundleResult{}, err
	}
	if err := validateRelativeSafePath("materialized path", opts.MaterializedPath); err != nil {
		return BundleResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return BundleResult{}, err
	}
	if err := validateRelativeSafePath("summary path", opts.SummaryPath); err != nil {
		return BundleResult{}, err
	}

	retrievalData, err := os.ReadFile(opts.RetrievalContextPath)
	if err != nil {
		return BundleResult{}, fmt.Errorf("read retrieval context %q: %w", opts.RetrievalContextPath, err)
	}
	inspect, err := InspectArtifactBytes(retrievalData)
	if err != nil {
		return BundleResult{}, err
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		return BundleResult{}, fmt.Errorf("retrieval context inspect status %q", inspect.Status)
	}

	materializedData, err := os.ReadFile(opts.MaterializedPath)
	if err != nil {
		return BundleResult{}, fmt.Errorf("read materialized artifact %q: %w", opts.MaterializedPath, err)
	}
	materializedReport, err := MaterializedReportBytes(materializedData)
	if err != nil {
		return BundleResult{}, err
	}
	if materializedReport.Status == lancedbpolicy.StatusFailed {
		return BundleResult{}, fmt.Errorf("materialized report status %q", materializedReport.Status)
	}

	var materializedFile materializedArtifactFile
	if err := json.Unmarshal(materializedData, &materializedFile); err != nil {
		return BundleResult{}, fmt.Errorf("parse materialized artifact: %w", err)
	}

	retrievalSHA := sha256Hex(retrievalData)
	if strings.TrimSpace(materializedFile.SourceRetrievalContextSHA256) != retrievalSHA {
		return BundleResult{}, fmt.Errorf("source_retrieval_context_sha256 mismatch")
	}
	if materializedReport.IncludedChunkCount > inspect.HitCount {
		return BundleResult{}, fmt.Errorf("included_chunk_count %d exceeds retrieval hit_count %d", materializedReport.IncludedChunkCount, inspect.HitCount)
	}

	retrievalIDSet := make(map[string]struct{}, len(inspect.UniqueChunkIDs))
	for _, chunkID := range inspect.UniqueChunkIDs {
		retrievalIDSet[chunkID] = struct{}{}
	}
	for _, chunkID := range materializedReport.UniqueChunkIDs {
		if _, ok := retrievalIDSet[chunkID]; !ok {
			return BundleResult{}, fmt.Errorf("materialized chunk_id %q not found in retrieval context", chunkID)
		}
	}

	items, err := bundleItemsFromMaterialized(materializedFile.Items)
	if err != nil {
		return BundleResult{}, err
	}

	warnings := append([]string(nil), materializedReport.Warnings...)
	status := lancedbpolicy.StatusOK
	if materializedReport.Status == lancedbpolicy.StatusWarning || materializedReport.OmittedChunkCount > 0 {
		status = lancedbpolicy.StatusWarning
	}

	result := BundleResult{
		Status:                     status,
		ContainsText:               false,
		MaterializedTextArtifact:   opts.MaterializedPath,
		RetrievalContextSHA256:     retrievalSHA,
		MaterializedSHA256:         sha256Hex(materializedData),
		RetrievalContextPath:       opts.RetrievalContextPath,
		RetrievalHitCount:          inspect.HitCount,
		MaterializedChunkCount:     materializedReport.ChunkCount,
		IncludedChunkCount:         materializedReport.IncludedChunkCount,
		OmittedChunkCount:          materializedReport.OmittedChunkCount,
		TotalCharsIncluded:         materializedReport.TotalCharsIncluded,
		SourceChunksSHA256:         materializedFile.SourceChunksSHA256,
		RetrievalUniqueChunkIDs:    append([]string(nil), inspect.UniqueChunkIDs...),
		MaterializedUniqueChunkIDs: append([]string(nil), materializedReport.UniqueChunkIDs...),
		Warnings:                   warnings,
		Items:                      items,
	}

	if err := writeBundleJSON(opts.OutputPath, result); err != nil {
		return BundleResult{}, err
	}
	if err := writeBundleSummary(opts.SummaryPath, result); err != nil {
		return BundleResult{}, err
	}
	return result, nil
}

func bundleItemsFromMaterialized(rawItems []json.RawMessage) ([]BundleItem, error) {
	items := make([]BundleItem, 0, len(rawItems))
	for i, rawItem := range rawItems {
		var item MaterializeItem
		if err := json.Unmarshal(rawItem, &item); err != nil {
			return nil, fmt.Errorf("parse materialized items[%d]: %w", i, err)
		}
		items = append(items, BundleItem{
			Rank:          item.Rank,
			ChunkID:       item.ChunkID,
			Distance:      item.Distance,
			Domain:        item.Domain,
			SourcePath:    item.SourcePath,
			SourceSHA256:  item.SourceSHA256,
			TextSHA256:    item.TextSHA256,
			Truncated:     item.Truncated,
			IncludedChars: item.IncludedChars,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Rank == items[j].Rank {
			return items[i].ChunkID < items[j].ChunkID
		}
		return items[i].Rank < items[j].Rank
	})
	return items, nil
}

func writeBundleJSON(path string, result BundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create bundle output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal bundle json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write bundle json %q: %w", path, err)
	}
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("bundle json must not contain text_excerpt")
	}
	return nil
}

func writeBundleSummary(path string, result BundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create bundle summary dir: %w", err)
	}
	var b strings.Builder
	b.WriteString("# Retrieval context audit bundle\n\n")
	b.WriteString("Consolidated retrieval metadata and materialized artifact references only.\n")
	b.WriteString("Does not contain materialized chunk text. Markdown + Git remain canonical.\n\n")
	b.WriteString("- status: ")
	b.WriteString(result.Status)
	b.WriteByte('\n')
	b.WriteString("- contains_text: ")
	b.WriteString(fmt.Sprintf("%t", result.ContainsText))
	b.WriteByte('\n')
	b.WriteString("- materialized_text_artifact: ")
	b.WriteString(result.MaterializedTextArtifact)
	b.WriteByte('\n')
	b.WriteString("- retrieval_context_sha256: ")
	b.WriteString(result.RetrievalContextSHA256)
	b.WriteByte('\n')
	b.WriteString("- materialized_sha256: ")
	b.WriteString(result.MaterializedSHA256)
	b.WriteByte('\n')
	b.WriteString("- retrieval_hit_count: ")
	b.WriteString(fmt.Sprintf("%d", result.RetrievalHitCount))
	b.WriteByte('\n')
	b.WriteString("- included_chunk_count: ")
	b.WriteString(fmt.Sprintf("%d", result.IncludedChunkCount))
	b.WriteByte('\n')
	b.WriteString("- omitted_chunk_count: ")
	b.WriteString(fmt.Sprintf("%d", result.OmittedChunkCount))
	b.WriteByte('\n')
	b.WriteString("- total_chars_included: ")
	b.WriteString(fmt.Sprintf("%d", result.TotalCharsIncluded))
	b.WriteByte('\n')
	if len(result.RetrievalUniqueChunkIDs) > 0 {
		b.WriteString("- retrieval_unique_chunk_ids: ")
		b.WriteString(strings.Join(result.RetrievalUniqueChunkIDs, ", "))
		b.WriteByte('\n')
	}
	if len(result.MaterializedUniqueChunkIDs) > 0 {
		b.WriteString("- materialized_unique_chunk_ids: ")
		b.WriteString(strings.Join(result.MaterializedUniqueChunkIDs, ", "))
		b.WriteByte('\n')
	}
	if len(result.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, warning := range result.Warnings {
			b.WriteString("- ")
			b.WriteString(warning)
			b.WriteByte('\n')
		}
	}
	if len(result.Items) > 0 {
		b.WriteString("\n## Items\n\n")
		for _, item := range result.Items {
			b.WriteString("### ")
			b.WriteString(item.ChunkID)
			b.WriteByte('\n')
			b.WriteString("- rank: ")
			b.WriteString(fmt.Sprintf("%d", item.Rank))
			b.WriteByte('\n')
			b.WriteString("- text_sha256: ")
			b.WriteString(item.TextSHA256)
			b.WriteByte('\n')
			b.WriteString("- truncated: ")
			b.WriteString(fmt.Sprintf("%t", item.Truncated))
			b.WriteByte('\n')
			b.WriteString("- included_chars: ")
			b.WriteString(fmt.Sprintf("%d", item.IncludedChars))
			b.WriteByte('\n')
		}
	}
	if strings.Contains(b.String(), "text_excerpt") {
		return fmt.Errorf("bundle summary must not contain text_excerpt")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write bundle summary %q: %w", path, err)
	}
	return nil
}

func WriteBundleText(result BundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_bundle:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "contains_text: %t\n", result.ContainsText); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "materialized_text_artifact: %s\n", result.MaterializedTextArtifact); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "retrieval_context_sha256: %s\n", result.RetrievalContextSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "materialized_sha256: %s\n", result.MaterializedSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "retrieval_hit_count: %d\n", result.RetrievalHitCount); err != nil {
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
	if len(result.RetrievalUniqueChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "retrieval_unique_chunk_ids: %s\n", strings.Join(result.RetrievalUniqueChunkIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.MaterializedUniqueChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "materialized_unique_chunk_ids: %s\n", strings.Join(result.MaterializedUniqueChunkIDs, ", ")); err != nil {
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

func WriteBundleJSON(result BundleResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
