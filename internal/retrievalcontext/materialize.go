package retrievalcontext

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
)

type MaterializeOptions struct {
	RetrievalContextPath    string
	ChunksPath              string
	OutputPath              string
	SummaryPath             string
	MaxCharsPerChunk        int
	MaxTotalChars           int
	ConfirmIncludeChunkText bool
}

type MaterializeItem struct {
	Rank          int     `json:"rank"`
	ChunkID       string  `json:"chunk_id"`
	Distance      float64 `json:"distance"`
	Domain        string  `json:"domain,omitempty"`
	SourcePath    string  `json:"source_path,omitempty"`
	SourceSHA256  string  `json:"source_sha256,omitempty"`
	TextSHA256    string  `json:"text_sha256,omitempty"`
	TextExcerpt   string  `json:"text_excerpt"`
	Truncated     bool    `json:"truncated"`
	IncludedChars int     `json:"included_chars"`
}

type MaterializeResult struct {
	Status                       string            `json:"status"`
	SourceRetrievalContextSHA256 string            `json:"source_retrieval_context_sha256"`
	SourceChunksSHA256           string            `json:"source_chunks_sha256"`
	ChunkCount                   int               `json:"chunk_count"`
	IncludedChunkCount           int               `json:"included_chunk_count"`
	OmittedChunkCount            int               `json:"omitted_chunk_count"`
	MaxCharsPerChunk             int               `json:"max_chars_per_chunk"`
	MaxTotalChars                int               `json:"max_total_chars"`
	TotalCharsIncluded           int               `json:"total_chars_included"`
	Warnings                     []string          `json:"warnings,omitempty"`
	Items                        []MaterializeItem `json:"items"`
}

func Materialize(opts MaterializeOptions) (MaterializeResult, error) {
	if !opts.ConfirmIncludeChunkText {
		return MaterializeResult{}, fmt.Errorf("--confirm-include-chunk-text is required")
	}
	if opts.MaxCharsPerChunk <= 0 {
		return MaterializeResult{}, fmt.Errorf("max_chars_per_chunk must be > 0")
	}
	if opts.MaxTotalChars <= 0 {
		return MaterializeResult{}, fmt.Errorf("max_total_chars must be > 0")
	}
	if err := validateRelativeSafePath("chunks path", opts.ChunksPath); err != nil {
		return MaterializeResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return MaterializeResult{}, err
	}
	if err := validateRelativeSafePath("summary path", opts.SummaryPath); err != nil {
		return MaterializeResult{}, err
	}
	if err := validateRelativeSafePath("retrieval context path", opts.RetrievalContextPath); err != nil {
		return MaterializeResult{}, err
	}

	retrievalData, err := os.ReadFile(opts.RetrievalContextPath)
	if err != nil {
		return MaterializeResult{}, fmt.Errorf("read retrieval context %q: %w", opts.RetrievalContextPath, err)
	}
	inspect, err := InspectArtifactBytes(retrievalData)
	if err != nil {
		return MaterializeResult{}, err
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		return MaterializeResult{}, fmt.Errorf("retrieval context inspect status %q", inspect.Status)
	}

	chunksData, err := os.ReadFile(opts.ChunksPath)
	if err != nil {
		return MaterializeResult{}, fmt.Errorf("read chunks jsonl %q: %w", opts.ChunksPath, err)
	}
	chunks, err := memoryindex.LoadChunksJSONL(opts.ChunksPath)
	if err != nil {
		return MaterializeResult{}, fmt.Errorf("load chunks jsonl: %w", err)
	}
	chunkByID := make(map[string]memoryindex.Chunk, len(chunks))
	for _, chunk := range chunks {
		if existing, ok := chunkByID[chunk.ID]; ok {
			return MaterializeResult{}, fmt.Errorf("duplicate chunk id %q in chunks jsonl", existing.ID)
		}
		chunkByID[chunk.ID] = chunk
	}

	hits, err := loadArtifactHits(retrievalData)
	if err != nil {
		return MaterializeResult{}, err
	}

	var warnings []string
	items := make([]MaterializeItem, 0, len(hits))
	totalChars := 0
	remainingBudget := opts.MaxTotalChars

	for _, hit := range hits {
		chunk, ok := chunkByID[hit.ChunkID]
		if !ok {
			return MaterializeResult{}, fmt.Errorf("chunk_id %q not found in chunks jsonl", hit.ChunkID)
		}
		if err := validateChunkHashes(hit, chunk); err != nil {
			return MaterializeResult{}, err
		}
		if remainingBudget <= 0 {
			warnings = append(warnings, fmt.Sprintf("chunk_id %q omitted: max_total_chars budget exhausted", hit.ChunkID))
			continue
		}

		excerpt, truncated, includedChars, chunkWarnings := excerptChunkText(chunk.Text, opts.MaxCharsPerChunk, remainingBudget)
		warnings = append(warnings, chunkWarnings...)
		if includedChars == 0 {
			warnings = append(warnings, fmt.Sprintf("chunk_id %q omitted: no remaining char budget", hit.ChunkID))
			continue
		}

		item := MaterializeItem{
			Rank:          hit.Rank,
			ChunkID:       hit.ChunkID,
			Distance:      hit.Distance,
			Domain:        firstNonEmpty(hit.Domain, chunk.Domain),
			SourcePath:    firstNonEmpty(hit.SourcePath, chunk.SourcePath),
			SourceSHA256:  firstNonEmpty(hit.SourceSHA256, chunk.SourceSHA256),
			TextSHA256:    firstNonEmpty(hit.TextSHA256, chunk.TextSHA256),
			TextExcerpt:   excerpt,
			Truncated:     truncated,
			IncludedChars: includedChars,
		}
		items = append(items, item)
		totalChars += includedChars
		remainingBudget -= includedChars
	}

	omitted := len(hits) - len(items)
	status := lancedbpolicy.StatusOK
	if len(warnings) > 0 {
		status = lancedbpolicy.StatusWarning
	}

	result := MaterializeResult{
		Status:                       status,
		SourceRetrievalContextSHA256: sha256Hex(retrievalData),
		SourceChunksSHA256:           sha256Hex(chunksData),
		ChunkCount:                   len(hits),
		IncludedChunkCount:           len(items),
		OmittedChunkCount:            omitted,
		MaxCharsPerChunk:             opts.MaxCharsPerChunk,
		MaxTotalChars:                opts.MaxTotalChars,
		TotalCharsIncluded:           totalChars,
		Warnings:                     warnings,
		Items:                        items,
	}

	if err := writeMaterializeJSON(opts.OutputPath, result); err != nil {
		return MaterializeResult{}, err
	}
	if err := writeMaterializeSummary(opts.SummaryPath, result); err != nil {
		return MaterializeResult{}, err
	}
	return result, nil
}

func loadArtifactHits(data []byte) ([]SafeHit, error) {
	var file artifactFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse retrieval context artifact: %w", err)
	}
	hits := make([]SafeHit, 0, file.HitCount)
	for _, attachment := range file.Attachments {
		for _, rawHit := range attachment.Hits {
			for _, field := range forbiddenHitFields {
				if hasJSONField(rawHit, field) {
					return nil, fmt.Errorf("retrieval context hit contains forbidden field %q", field)
				}
			}
			var hit SafeHit
			if err := json.Unmarshal(rawHit, &hit); err != nil {
				return nil, fmt.Errorf("parse retrieval context hit: %w", err)
			}
			hits = append(hits, hit)
		}
	}
	return hits, nil
}

func validateChunkHashes(hit SafeHit, chunk memoryindex.Chunk) error {
	computedTextSHA := sha256Hex([]byte(chunk.Text))
	if chunk.TextSHA256 != computedTextSHA {
		return fmt.Errorf("chunk_id %q chunks jsonl text_sha256 mismatch", chunk.ID)
	}
	if strings.TrimSpace(hit.TextSHA256) != "" && hit.TextSHA256 != computedTextSHA {
		return fmt.Errorf("chunk_id %q retrieval context text_sha256 mismatch", hit.ChunkID)
	}
	if strings.TrimSpace(hit.SourceSHA256) != "" && hit.SourceSHA256 != chunk.SourceSHA256 {
		return fmt.Errorf("chunk_id %q retrieval context source_sha256 mismatch", hit.ChunkID)
	}
	return nil
}

func excerptChunkText(text string, maxPerChunk int, remainingBudget int) (excerpt string, truncated bool, includedChars int, warnings []string) {
	limit := maxPerChunk
	if remainingBudget < limit {
		limit = remainingBudget
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text, false, len(runes), nil
	}
	excerpt = string(runes[:limit])
	if limit < maxPerChunk {
		warnings = append(warnings, fmt.Sprintf("text truncated to %d chars to respect max_total_chars", limit))
	} else {
		warnings = append(warnings, fmt.Sprintf("text truncated to %d chars (max_chars_per_chunk)", limit))
	}
	return excerpt, true, utf8.RuneCountInString(excerpt), warnings
}

func writeMaterializeJSON(path string, result MaterializeResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized json: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized json %q: %w", path, err)
	}
	return nil
}

func writeMaterializeSummary(path string, result MaterializeResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create summary dir: %w", err)
	}
	var b strings.Builder
	b.WriteString("# Materialized retrieval context\n\n")
	b.WriteString("Contains derived chunk text excerpts from memory-index chunks JSONL only.\n")
	b.WriteString("This is not canonical memory. Markdown + Git remain the source of truth.\n")
	b.WriteString("Do not treat this artifact as authoritative for memory writes.\n\n")
	b.WriteString("- status: ")
	b.WriteString(result.Status)
	b.WriteByte('\n')
	b.WriteString("- chunk_count: ")
	b.WriteString(fmt.Sprintf("%d", result.ChunkCount))
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
	b.WriteString("- source_retrieval_context_sha256: ")
	b.WriteString(result.SourceRetrievalContextSHA256)
	b.WriteByte('\n')
	b.WriteString("- source_chunks_sha256: ")
	b.WriteString(result.SourceChunksSHA256)
	b.WriteByte('\n')
	if len(result.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, warning := range result.Warnings {
			b.WriteString("- ")
			b.WriteString(warning)
			b.WriteByte('\n')
		}
	}
	if len(result.Items) > 0 {
		b.WriteString("\n## Chunks\n\n")
		for _, item := range result.Items {
			b.WriteString("### rank ")
			b.WriteString(fmt.Sprintf("%d", item.Rank))
			b.WriteString(" — ")
			b.WriteString(item.ChunkID)
			b.WriteByte('\n')
			b.WriteString("- distance: ")
			b.WriteString(fmt.Sprintf("%g", item.Distance))
			b.WriteByte('\n')
			if item.Domain != "" {
				b.WriteString("- domain: ")
				b.WriteString(item.Domain)
				b.WriteByte('\n')
			}
			if item.SourcePath != "" {
				b.WriteString("- source_path: ")
				b.WriteString(item.SourcePath)
				b.WriteByte('\n')
			}
			if item.SourceSHA256 != "" {
				b.WriteString("- source_sha256: ")
				b.WriteString(item.SourceSHA256)
				b.WriteByte('\n')
			}
			if item.TextSHA256 != "" {
				b.WriteString("- text_sha256: ")
				b.WriteString(item.TextSHA256)
				b.WriteByte('\n')
			}
			b.WriteString("- truncated: ")
			b.WriteString(fmt.Sprintf("%t", item.Truncated))
			b.WriteByte('\n')
			b.WriteString("- included_chars: ")
			b.WriteString(fmt.Sprintf("%d", item.IncludedChars))
			b.WriteByte('\n')
			b.WriteString("- text_excerpt: ")
			b.WriteString(item.TextExcerpt)
			b.WriteByte('\n')
		}
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write materialized summary %q: %w", path, err)
	}
	return nil
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
