package memoryindex

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

const (
	StatusOK      = "ok"
	StatusWarning = "warning"
	StatusFailed  = "failed"
)

type LargestFile struct {
	Domain    string `json:"domain"`
	Path      string `json:"path"`
	RelPath   string `json:"rel_path"`
	SizeBytes int64  `json:"size_bytes"`
}

type DoctorSourceInfo struct {
	Domain          string `json:"domain"`
	Root            string `json:"root"`
	RootExists      bool   `json:"root_exists"`
	SourceCount     int    `json:"source_count"`
	EstimatedChunks int    `json:"estimated_chunks"`
}

type DoctorResult struct {
	Status                  string             `json:"status"`
	Domains                 []string           `json:"domains"`
	Sources                 []DoctorSourceInfo `json:"sources"`
	SourceCount             int                `json:"source_count"`
	SkippedCount            int                `json:"skipped_count"`
	Skipped                 SkippedStats       `json:"skipped"`
	EstimatedChunksByDomain map[string]int     `json:"estimated_chunks_by_domain"`
	LargestFiles            []LargestFile      `json:"largest_files"`
	Warnings                []string           `json:"warnings"`
}

func Doctor(cfg Config) (DoctorResult, error) {
	if err := Validate(cfg); err != nil {
		return DoctorResult{}, err
	}
	scan, err := scanConfig(cfg)
	if err != nil {
		return DoctorResult{}, err
	}

	estimatedByDomain := map[string]int{}
	sourceCountByDomain := map[string]int{}
	for _, candidate := range scan.Candidates {
		sourceCountByDomain[candidate.Domain]++
		count, err := estimateChunksForFile(candidate.Path, cfg.MemoryIndex.Chunking.MaxChars, cfg.MemoryIndex.Chunking.OverlapChars)
		if err != nil {
			return DoctorResult{}, fmt.Errorf("estimate chunks for %q: %w", candidate.Path, err)
		}
		estimatedByDomain[candidate.Domain] += count
	}

	sources := make([]DoctorSourceInfo, 0, len(cfg.MemoryIndex.Sources))
	for _, source := range cfg.MemoryIndex.Sources {
		_, statErr := os.Stat(source.Root)
		sources = append(sources, DoctorSourceInfo{
			Domain:          source.Domain,
			Root:            source.Root,
			RootExists:      statErr == nil,
			SourceCount:     sourceCountByDomain[source.Domain],
			EstimatedChunks: estimatedByDomain[source.Domain],
		})
	}

	largest, err := largestCandidateFiles(scan.Candidates, 10)
	if err != nil {
		return DoctorResult{}, err
	}

	warnings := doctorWarnings(cfg, scan, largest)
	result := DoctorResult{
		Status:                  doctorStatus(warnings),
		Domains:                 append([]string(nil), cfg.MemoryIndex.Domains...),
		Sources:                 sources,
		SourceCount:             len(scan.Candidates),
		SkippedCount:            scan.SkippedCount,
		Skipped:                 scan.Skipped,
		EstimatedChunksByDomain: estimatedByDomain,
		LargestFiles:            largest,
		Warnings:                warnings,
	}
	sort.Strings(result.Domains)
	return result, nil
}

func estimateChunksForFile(path string, maxChars int, overlap int) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return len(chunkText(string(content), maxChars, overlap)), nil
}

func largestCandidateFiles(candidates []CandidateFile, limit int) ([]LargestFile, error) {
	type sized struct {
		candidate CandidateFile
		size      int64
	}
	sizedFiles := make([]sized, 0, len(candidates))
	for _, candidate := range candidates {
		info, err := os.Stat(candidate.Path)
		if err != nil {
			return nil, fmt.Errorf("stat candidate %q: %w", candidate.Path, err)
		}
		sizedFiles = append(sizedFiles, sized{candidate: candidate, size: info.Size()})
	}
	sort.Slice(sizedFiles, func(i, j int) bool {
		if sizedFiles[i].size == sizedFiles[j].size {
			return sizedFiles[i].candidate.Path < sizedFiles[j].candidate.Path
		}
		return sizedFiles[i].size > sizedFiles[j].size
	})
	if limit > len(sizedFiles) {
		limit = len(sizedFiles)
	}
	out := make([]LargestFile, 0, limit)
	for i := 0; i < limit; i++ {
		item := sizedFiles[i]
		out = append(out, LargestFile{
			Domain:    item.candidate.Domain,
			Path:      item.candidate.Path,
			RelPath:   item.candidate.RelPath,
			SizeBytes: item.size,
		})
	}
	return out, nil
}

func doctorWarnings(cfg Config, scan ScanResult, largest []LargestFile) []string {
	var warnings []string
	if len(scan.Candidates) == 0 {
		warnings = append(warnings, "no candidate sources found")
	}
	if scan.Skipped.Symlinks > 0 {
		warnings = append(warnings, fmt.Sprintf("skipped %d symlink path(s); symlinks are never indexed", scan.Skipped.Symlinks))
	}
	if scan.Skipped.SecretPaths > 0 {
		warnings = append(warnings, fmt.Sprintf("skipped %d secret path(s)", scan.Skipped.SecretPaths))
	}
	if scan.Skipped.Excluded > 0 {
		warnings = append(warnings, fmt.Sprintf("skipped %d excluded path(s)", scan.Skipped.Excluded))
	}
	for _, source := range cfg.MemoryIndex.Sources {
		if _, err := os.Stat(source.Root); err != nil {
			warnings = append(warnings, fmt.Sprintf("source root missing for domain %q: %s", source.Domain, source.Root))
		}
	}
	const largeFileBytes = int64(1 << 20)
	for _, file := range largest {
		if file.SizeBytes >= largeFileBytes {
			warnings = append(warnings, fmt.Sprintf("large source file %q (%d bytes) may produce many chunks", file.RelPath, file.SizeBytes))
			break
		}
	}
	return warnings
}

func doctorStatus(warnings []string) string {
	if len(warnings) > 0 {
		return StatusWarning
	}
	return StatusOK
}

func WriteDoctorText(result DoctorResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_index_doctor:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "source_count: %d\n", result.SourceCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "skipped_count: %d\n", result.SkippedCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "skipped_symlinks: %d\n", result.Skipped.Symlinks); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "skipped_secret_paths: %d\n", result.Skipped.SecretPaths); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "skipped_excluded: %d\n", result.Skipped.Excluded); err != nil {
		return err
	}
	if len(result.Domains) > 0 {
		if _, err := fmt.Fprintf(out, "domains: %s\n", strings.Join(result.Domains, ", ")); err != nil {
			return err
		}
	}
	for _, item := range sortedIntMap(result.EstimatedChunksByDomain) {
		if _, err := fmt.Fprintf(out, "estimated_chunks[%s]: %d\n", item.key, item.value); err != nil {
			return err
		}
	}
	for _, source := range result.Sources {
		if _, err := fmt.Fprintf(out, "\nsource:\n"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "  domain: %s\n", source.Domain); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "  root: %s\n", source.Root); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "  root_exists: %t\n", source.RootExists); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "  source_count: %d\n", source.SourceCount); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "  estimated_chunks: %d\n", source.EstimatedChunks); err != nil {
			return err
		}
	}
	if len(result.LargestFiles) > 0 {
		if _, err := fmt.Fprintf(out, "\nlargest_files:\n"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "domain\trel_path\tsize_bytes"); err != nil {
			return err
		}
		for _, file := range result.LargestFiles {
			if _, err := fmt.Fprintf(table, "%s\t%s\t%d\n", file.Domain, file.RelPath, file.SizeBytes); err != nil {
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

func WriteDoctorJSON(result DoctorResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func sortedIntMap(values map[string]int) []struct {
	key   string
	value int
} {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]struct {
		key   string
		value int
	}, 0, len(keys))
	for _, key := range keys {
		out = append(out, struct {
			key   string
			value int
		}{key: key, value: values[key]})
	}
	return out
}
