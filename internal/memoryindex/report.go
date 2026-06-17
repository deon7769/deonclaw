package memoryindex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

type InvalidChunkReport struct {
	Line   int    `json:"line"`
	ID     string `json:"id,omitempty"`
	Reason string `json:"reason"`
}

type ReportResult struct {
	Status             string               `json:"status"`
	ChunkCount         int                  `json:"chunk_count"`
	ManifestChunkCount int                  `json:"manifest_chunk_count"`
	SourceCount        int                  `json:"source_count"`
	Domains            []string             `json:"domains"`
	ChunksByDomain     map[string]int       `json:"chunks_by_domain"`
	AvgChunkChars      float64              `json:"avg_chunk_chars"`
	MaxChunkChars      int                  `json:"max_chunk_chars"`
	DuplicateChunkIDs  []string             `json:"duplicate_chunk_ids"`
	InvalidChunks      []InvalidChunkReport `json:"invalid_chunks"`
	Warnings           []string             `json:"warnings"`
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest %q: %w", path, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest %q: %w", path, err)
	}
	return manifest, nil
}

func Report(manifestPath string, chunksPath string) (ReportResult, error) {
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return ReportResult{}, err
	}
	chunks, parseIssues, err := loadChunksJSONL(chunksPath)
	if err != nil {
		return ReportResult{}, err
	}

	domainSet := map[string]struct{}{}
	for _, domain := range manifest.Domains {
		domainSet[domain] = struct{}{}
	}

	invalid := append([]InvalidChunkReport(nil), parseIssues...)
	idCounts := map[string]int{}
	chunksByDomain := map[string]int{}
	var totalChars int
	maxChars := 0

	for i, chunk := range chunks {
		line := i + 1
		if strings.TrimSpace(chunk.ID) == "" {
			invalid = append(invalid, InvalidChunkReport{Line: line, Reason: "missing chunk id"})
		} else {
			idCounts[chunk.ID]++
		}
		if strings.TrimSpace(chunk.SourceSHA256) == "" {
			invalid = append(invalid, InvalidChunkReport{Line: line, ID: chunk.ID, Reason: "missing source_sha256"})
		}
		if chunk.TextSHA256 != sha256Hex([]byte(chunk.Text)) {
			invalid = append(invalid, InvalidChunkReport{Line: line, ID: chunk.ID, Reason: "text_sha256 mismatch"})
		}
		if strings.TrimSpace(chunk.Domain) == "" {
			invalid = append(invalid, InvalidChunkReport{Line: line, ID: chunk.ID, Reason: "missing domain"})
		} else if _, ok := domainSet[chunk.Domain]; !ok {
			invalid = append(invalid, InvalidChunkReport{Line: line, ID: chunk.ID, Reason: fmt.Sprintf("domain %q not listed in manifest", chunk.Domain)})
		}
		chunksByDomain[chunk.Domain]++
		textLen := len([]rune(chunk.Text))
		totalChars += textLen
		if textLen > maxChars {
			maxChars = textLen
		}
	}

	duplicateIDs := make([]string, 0)
	for id, count := range idCounts {
		if count > 1 {
			duplicateIDs = append(duplicateIDs, id)
		}
	}
	sort.Strings(duplicateIDs)

	var warnings []string
	if manifest.ChunkCount != len(chunks) {
		invalid = append(invalid, InvalidChunkReport{
			Line:   0,
			Reason: fmt.Sprintf("manifest chunk_count %d != chunks jsonl lines %d", manifest.ChunkCount, len(chunks)),
		})
	}
	if len(chunks) > 0 && manifest.SourceCount <= 0 {
		invalid = append(invalid, InvalidChunkReport{
			Line:   0,
			Reason: "manifest source_count must be > 0 when chunks exist",
		})
	}
	if len(chunks) == 0 && manifest.ChunkCount > 0 {
		warnings = append(warnings, "manifest reports chunks but jsonl file is empty")
	}

	avgChars := 0.0
	if len(chunks) > 0 {
		avgChars = float64(totalChars) / float64(len(chunks))
	}

	status := reportStatus(len(invalid), len(duplicateIDs), warnings)
	domains := append([]string(nil), manifest.Domains...)
	sort.Strings(domains)

	return ReportResult{
		Status:             status,
		ChunkCount:         len(chunks),
		ManifestChunkCount: manifest.ChunkCount,
		SourceCount:        manifest.SourceCount,
		Domains:            domains,
		ChunksByDomain:     chunksByDomain,
		AvgChunkChars:      avgChars,
		MaxChunkChars:      maxChars,
		DuplicateChunkIDs:  duplicateIDs,
		InvalidChunks:      invalid,
		Warnings:           warnings,
	}, nil
}

func loadChunksJSONL(path string) ([]Chunk, []InvalidChunkReport, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open chunks jsonl %q: %w", path, err)
	}
	defer file.Close()

	var chunks []Chunk
	var invalid []InvalidChunkReport
	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var chunk Chunk
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			invalid = append(invalid, InvalidChunkReport{
				Line:   lineNo,
				Reason: fmt.Sprintf("invalid json: %v", err),
			})
			continue
		}
		chunks = append(chunks, chunk)
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("read chunks jsonl %q: %w", path, err)
	}
	return chunks, invalid, nil
}

func LoadChunksJSONL(path string) ([]Chunk, error) {
	chunks, invalid, err := loadChunksJSONL(path)
	if err != nil {
		return nil, err
	}
	if len(invalid) > 0 {
		return nil, fmt.Errorf("chunks jsonl has %d invalid lines", len(invalid))
	}
	return chunks, nil
}

func reportStatus(invalidCount int, duplicateCount int, warnings []string) string {
	if invalidCount > 0 || duplicateCount > 0 {
		return StatusFailed
	}
	if len(warnings) > 0 {
		return StatusWarning
	}
	return StatusOK
}

func WriteReportText(result ReportResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_index_report:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "chunk_count: %d\n", result.ChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "manifest_chunk_count: %d\n", result.ManifestChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "source_count: %d\n", result.SourceCount); err != nil {
		return err
	}
	if len(result.Domains) > 0 {
		if _, err := fmt.Fprintf(out, "domains: %s\n", strings.Join(result.Domains, ", ")); err != nil {
			return err
		}
	}
	for _, item := range sortedIntMap(result.ChunksByDomain) {
		if _, err := fmt.Fprintf(out, "chunks_by_domain[%s]: %d\n", item.key, item.value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "avg_chunk_chars: %.2f\n", result.AvgChunkChars); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_chunk_chars: %d\n", result.MaxChunkChars); err != nil {
		return err
	}
	if len(result.DuplicateChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "duplicate_chunk_ids: %s\n", strings.Join(result.DuplicateChunkIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.InvalidChunks) > 0 {
		if _, err := fmt.Fprintf(out, "\ninvalid_chunks:\n"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "line\tid\treason"); err != nil {
			return err
		}
		for _, item := range result.InvalidChunks {
			if _, err := fmt.Fprintf(table, "%d\t%s\t%s\n", item.Line, item.ID, item.Reason); err != nil {
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

func WriteReportJSON(result ReportResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
