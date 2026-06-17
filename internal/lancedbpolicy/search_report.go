package lancedbpolicy

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

var forbiddenSearchResultFields = []string{
	"vector",
	"text",
	"chunk_text",
	"content",
	"embedding",
}

type SearchReportOptions struct{}

type SearchReportResult struct {
	Status             string   `json:"status"`
	QueryMode          string   `json:"query_mode"`
	TopK               int      `json:"top_k"`
	ResultCount        int      `json:"result_count"`
	DatabasePath       string   `json:"database_path"`
	Table              string   `json:"table"`
	RetrievalPerformed bool     `json:"retrieval_performed"`
	RunnerIntegration  bool     `json:"runner_integration"`
	MinDistance        *float64 `json:"min_distance,omitempty"`
	MaxDistance        *float64 `json:"max_distance,omitempty"`
	UniqueChunkIDs     []string `json:"unique_chunk_ids"`
	InvalidResults     []string `json:"invalid_results"`
	Warnings           []string `json:"warnings"`
	Failures           []string `json:"failures,omitempty"`
}

type searchSmokeResultEnvelope struct {
	Search  searchSmokeSearchBlock `json:"search"`
	Summary SearchSmokeSummary     `json:"summary"`
}

type searchSmokeSearchBlock struct {
	Status    string            `json:"status"`
	QueryMode string            `json:"query_mode"`
	TopK      int               `json:"top_k"`
	Results   []json.RawMessage `json:"results"`
}

func LoadSearchSmokeResult(path string) (SearchSmokeResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SearchSmokeResult{}, fmt.Errorf("read search-smoke result %q: %w", path, err)
	}
	var payload SearchSmokeResult
	if err := json.Unmarshal(data, &payload); err != nil {
		return SearchSmokeResult{}, fmt.Errorf("parse search-smoke result %q: %w", path, err)
	}
	return payload, nil
}

func LoadSearchReportArtifact(path string) (SearchReportResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SearchReportResult{}, fmt.Errorf("read search-report artifact %q: %w", path, err)
	}
	var report SearchReportResult
	if err := json.Unmarshal(data, &report); err != nil {
		return SearchReportResult{}, fmt.Errorf("parse search-report artifact %q: %w", path, err)
	}
	return report, nil
}

func loadSearchSmokeResultEnvelope(path string) (searchSmokeResultEnvelope, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return searchSmokeResultEnvelope{}, fmt.Errorf("read search-smoke result %q: %w", path, err)
	}
	var envelope searchSmokeResultEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return searchSmokeResultEnvelope{}, fmt.Errorf("parse search-smoke result %q: %w", path, err)
	}
	return envelope, nil
}

func SearchReport(resultPath string, cfg Config, opts SearchReportOptions) (SearchReportResult, error) {
	_ = opts
	if err := Validate(cfg); err != nil {
		return SearchReportResult{}, err
	}

	envelope, err := loadSearchSmokeResultEnvelope(resultPath)
	if err != nil {
		return SearchReportResult{}, err
	}

	result := SearchReportResult{
		Status:             StatusOK,
		QueryMode:          envelope.Summary.QueryMode,
		TopK:               envelope.Summary.TopK,
		ResultCount:        envelope.Summary.ResultCount,
		DatabasePath:       envelope.Summary.DatabasePath,
		Table:              envelope.Summary.Table,
		RetrievalPerformed: envelope.Summary.RetrievalPerformed,
		RunnerIntegration:  envelope.Summary.RunnerIntegration,
	}

	failures := validateSearchSmokeSummary(envelope.Summary, envelope.Search)
	failures = append(failures, validateSearchSmokePolicyCrossCheck(envelope.Summary, cfg.LanceDBPolicy)...)
	invalidResults, hitFailures, distances, chunkIDs := validateSearchSmokeHits(envelope.Search.Results)
	result.InvalidResults = invalidResults
	failures = append(failures, hitFailures...)

	if len(chunkIDs) > 0 {
		result.UniqueChunkIDs = append([]string(nil), chunkIDs...)
	}
	if len(distances) > 0 {
		minDistance := distances[0]
		maxDistance := distances[0]
		for _, distance := range distances[1:] {
			if distance < minDistance {
				minDistance = distance
			}
			if distance > maxDistance {
				maxDistance = distance
			}
		}
		result.MinDistance = &minDistance
		result.MaxDistance = &maxDistance
	}

	result.Failures = failures
	if len(failures) > 0 {
		result.Status = StatusFailed
	} else if len(result.Warnings) > 0 {
		result.Status = StatusWarning
	}
	return result, nil
}

func validateSearchSmokeSummary(summary SearchSmokeSummary, search searchSmokeSearchBlock) []string {
	var failures []string
	if !summary.RetrievalPerformed {
		failures = append(failures, "summary.retrieval_performed must be true")
	}
	if summary.RunnerIntegration {
		failures = append(failures, "summary.runner_integration must be false")
	}
	if summary.TopK <= 0 || summary.TopK > MaxSearchTopK {
		failures = append(failures, fmt.Sprintf("summary.top_k must be > 0 and <= %d", MaxSearchTopK))
	}
	if summary.ResultCount > summary.TopK {
		failures = append(failures, fmt.Sprintf("summary.result_count %d exceeds summary.top_k %d", summary.ResultCount, summary.TopK))
	}
	if len(search.Results) > summary.TopK {
		failures = append(failures, fmt.Sprintf("search.results length %d exceeds top_k %d", len(search.Results), summary.TopK))
	}
	if summary.ResultCount != len(search.Results) {
		failures = append(failures, fmt.Sprintf("summary.result_count %d != search.results length %d", summary.ResultCount, len(search.Results)))
	}
	if summary.QueryMode != "vector" && summary.QueryMode != "chunk_id" {
		failures = append(failures, fmt.Sprintf("summary.query_mode %q must be vector or chunk_id", summary.QueryMode))
	}
	if search.QueryMode != "" && search.QueryMode != summary.QueryMode {
		failures = append(failures, fmt.Sprintf("search.query_mode %q != summary.query_mode %q", search.QueryMode, summary.QueryMode))
	}
	if search.TopK > 0 && search.TopK != summary.TopK {
		failures = append(failures, fmt.Sprintf("search.top_k %d != summary.top_k %d", search.TopK, summary.TopK))
	}
	return failures
}

func validateSearchSmokePolicyCrossCheck(summary SearchSmokeSummary, policy Policy) []string {
	var failures []string
	if summary.DatabasePath != policy.Database.Path {
		failures = append(failures, fmt.Sprintf("summary.database_path %q != policy database.path %q", summary.DatabasePath, policy.Database.Path))
	}
	if summary.Table != policy.Database.Table {
		failures = append(failures, fmt.Sprintf("summary.table %q != policy database.table %q", summary.Table, policy.Database.Table))
	}
	if summary.TopK > MaxSearchTopK {
		failures = append(failures, fmt.Sprintf("summary.top_k %d exceeds project limit %d", summary.TopK, MaxSearchTopK))
	}
	return failures
}

func validateSearchSmokeHits(rawResults []json.RawMessage) (invalidResults []string, failures []string, distances []float64, chunkIDs []string) {
	seenRanks := make(map[int]struct{})
	seenChunks := make(map[string]struct{})
	prevRank := 0

	for index, raw := range rawResults {
		label := fmt.Sprintf("results[%d]", index)
		fieldFailures, forbidden := validateSearchResultObject(raw, label)
		if len(fieldFailures) > 0 {
			invalidResults = append(invalidResults, label)
			failures = append(failures, fieldFailures...)
		}
		if len(forbidden) > 0 {
			invalidResults = appendUniqueString(invalidResults, label)
			for _, field := range forbidden {
				failures = append(failures, fmt.Sprintf("%s contains forbidden field %q", label, field))
			}
		}

		var hit SearchHit
		if err := json.Unmarshal(raw, &hit); err != nil {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s is not a valid search hit object: %v", label, err))
			continue
		}
		if hit.Rank <= 0 {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s rank must be > 0", label))
		}
		if _, exists := seenRanks[hit.Rank]; exists {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s duplicate rank %d", label, hit.Rank))
		} else if hit.Rank > 0 {
			seenRanks[hit.Rank] = struct{}{}
		}
		if hit.Rank > 0 && prevRank > 0 && hit.Rank <= prevRank {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s ranks must be strictly increasing", label))
		}
		if hit.Rank > prevRank {
			prevRank = hit.Rank
		}
		if strings.TrimSpace(hit.ChunkID) == "" {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s chunk_id must not be empty", label))
		} else if _, exists := seenChunks[hit.ChunkID]; !exists {
			seenChunks[hit.ChunkID] = struct{}{}
			chunkIDs = append(chunkIDs, hit.ChunkID)
		}
		if strings.TrimSpace(hit.VectorID) == "" {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s vector_id must not be empty", label))
		}
		if math.IsNaN(hit.Distance) || math.IsInf(hit.Distance, 0) {
			invalidResults = appendUniqueString(invalidResults, label)
			failures = append(failures, fmt.Sprintf("%s distance must be numeric", label))
		} else {
			distances = append(distances, hit.Distance)
		}
	}

	sort.Strings(chunkIDs)
	return invalidResults, failures, distances, chunkIDs
}

func validateSearchResultObject(raw json.RawMessage, label string) (failures []string, forbidden []string) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		return []string{fmt.Sprintf("%s is not a JSON object", label)}, nil
	}
	for _, field := range forbiddenSearchResultFields {
		if _, exists := object[field]; exists {
			forbidden = append(forbidden, field)
		}
	}
	if rawDomain, exists := object["domain"]; exists {
		var domain string
		if err := json.Unmarshal(rawDomain, &domain); err != nil {
			failures = append(failures, fmt.Sprintf("%s domain must be a string when present", label))
		} else if strings.TrimSpace(domain) == "" {
			failures = append(failures, fmt.Sprintf("%s domain must not be empty when present", label))
		}
	}
	if _, exists := object["distance"]; !exists {
		failures = append(failures, fmt.Sprintf("%s distance is required", label))
	} else {
		var distance float64
		if err := json.Unmarshal(object["distance"], &distance); err != nil {
			failures = append(failures, fmt.Sprintf("%s distance must be numeric", label))
		}
	}
	return failures, forbidden
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func WriteSearchReportText(result SearchReportResult, out io.Writer) error {
	if _, err := fmt.Fprintf(out, "memory_lancedb_search_report:\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "query_mode: %s\n", result.QueryMode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "top_k: %d\n", result.TopK); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "result_count: %d\n", result.ResultCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "database_path: %s\n", result.DatabasePath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "table: %s\n", result.Table); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "retrieval_performed: %t\n", result.RetrievalPerformed); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "runner_integration: %t\n", result.RunnerIntegration); err != nil {
		return err
	}
	if result.MinDistance != nil {
		if _, err := fmt.Fprintf(out, "min_distance: %g\n", *result.MinDistance); err != nil {
			return err
		}
	}
	if result.MaxDistance != nil {
		if _, err := fmt.Fprintf(out, "max_distance: %g\n", *result.MaxDistance); err != nil {
			return err
		}
	}
	if len(result.UniqueChunkIDs) > 0 {
		if _, err := fmt.Fprintf(out, "unique_chunk_ids: %s\n", strings.Join(result.UniqueChunkIDs, ", ")); err != nil {
			return err
		}
	}
	if len(result.InvalidResults) > 0 {
		if _, err := fmt.Fprintf(out, "invalid_results: %s\n", strings.Join(result.InvalidResults, ", ")); err != nil {
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

func WriteSearchReportJSON(result SearchReportResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
