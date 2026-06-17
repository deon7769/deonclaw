package lancedbpolicy_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

func TestSearchReportOKForValidResult(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	path := writeSearchSmokeResultFixture(t, validSearchSmokeEnvelope())

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok; failures = %v", result.Status, result.Failures)
	}
	if result.QueryMode != "vector" || result.TopK != 5 || result.ResultCount != 1 {
		t.Fatalf("result = %#v, want vector mode with top_k=5 result_count=1", result)
	}
	if result.MinDistance == nil || result.MaxDistance == nil || *result.MinDistance != 0.12 {
		t.Fatalf("distance range = %#v, want min/max around 0.12", result)
	}
	if len(result.UniqueChunkIDs) != 1 || result.UniqueChunkIDs[0] != "chunk-a" {
		t.Fatalf("unique_chunk_ids = %#v", result.UniqueChunkIDs)
	}
}

func TestSearchReportFailsWhenRetrievalPerformedFalse(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	envelope := validSearchSmokeEnvelope()
	envelope.Summary.RetrievalPerformed = false
	path := writeSearchSmokeResultFixture(t, envelope)

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestSearchReportFailsWhenRunnerIntegrationTrue(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	envelope := validSearchSmokeEnvelope()
	envelope.Summary.RunnerIntegration = true
	path := writeSearchSmokeResultFixture(t, envelope)

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestSearchReportFailsWhenResultCountExceedsTopK(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	envelope := validSearchSmokeEnvelope()
	envelope.Summary.TopK = 1
	envelope.Summary.ResultCount = 2
	envelope.Search.TopK = 1
	envelope.Search.Results = append(envelope.Search.Results, mustRawSearchHit(t, lancedbpolicy.SearchHit{
		Rank:     2,
		ChunkID:  "chunk-b",
		VectorID: "vec-b",
		Distance: 0.2,
	}))
	path := writeSearchSmokeResultFixture(t, envelope)

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestSearchReportFailsWithDuplicateRank(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	envelope := validSearchSmokeEnvelope()
	envelope.Summary.ResultCount = 2
	envelope.Search.Results = append(envelope.Search.Results, mustRawSearchHit(t, lancedbpolicy.SearchHit{
		Rank:     1,
		ChunkID:  "chunk-b",
		VectorID: "vec-b",
		Distance: 0.2,
	}))
	path := writeSearchSmokeResultFixture(t, envelope)

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestSearchReportFailsWithVectorPresent(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	envelope := validSearchSmokeEnvelope()
	raw := mustRawSearchHit(t, lancedbpolicy.SearchHit{
		Rank:     1,
		ChunkID:  "chunk-a",
		VectorID: "vec-a",
		Distance: 0.12,
	})
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	object["vector"] = []float64{0.1, 0.2, 0.3}
	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	envelope.Search.Results = []json.RawMessage{raw}
	path := writeSearchSmokeResultFixture(t, envelope)

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestSearchReportFailsWithForbiddenTextFields(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	for _, field := range []string{"text", "chunk_text", "content"} {
		t.Run(field, func(t *testing.T) {
			envelope := validSearchSmokeEnvelope()
			raw := mustRawSearchHit(t, lancedbpolicy.SearchHit{
				Rank:     1,
				ChunkID:  "chunk-a",
				VectorID: "vec-a",
				Distance: 0.12,
			})
			var object map[string]any
			if err := json.Unmarshal(raw, &object); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			object[field] = "SECRET-CHUNK-TEXT"
			raw, err := json.Marshal(object)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			envelope.Search.Results = []json.RawMessage{raw}
			path := writeSearchSmokeResultFixture(t, envelope)

			result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
			if err != nil {
				t.Fatalf("SearchReport() error = %v", err)
			}
			if result.Status != lancedbpolicy.StatusFailed {
				t.Fatalf("status = %q, want failed", result.Status)
			}
		})
	}
}

func TestSearchReportFailsWhenDatabasePathOrTableDivergeFromPolicy(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()

	envelope := validSearchSmokeEnvelope()
	envelope.Summary.DatabasePath = "artifacts/other-db"
	path := writeSearchSmokeResultFixture(t, envelope)
	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("database_path status = %q, want failed", result.Status)
	}

	envelope = validSearchSmokeEnvelope()
	envelope.Summary.Table = "other_table"
	path = writeSearchSmokeResultFixture(t, envelope)
	result, err = lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("table status = %q, want failed", result.Status)
	}
}

func TestSearchReportTextDoesNotPrintVectorOrChunkText(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	cfg := testLancedbWriteSmokePolicy()
	envelope := validSearchSmokeEnvelope()
	envelope.Search.Results = []json.RawMessage{mustRawSearchHit(t, lancedbpolicy.SearchHit{
		Rank:     1,
		ChunkID:  "chunk-a",
		VectorID: "vec-a",
		Distance: 0.12,
		Domain:   marker,
	})}
	path := writeSearchSmokeResultFixture(t, envelope)

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := lancedbpolicy.WriteSearchReportText(result, &buf); err != nil {
		t.Fatalf("WriteSearchReportText() error = %v", err)
	}
	output := buf.String()
	if strings.Contains(output, marker) {
		t.Fatal("report text leaked chunk text marker from result payload")
	}
	if strings.Contains(output, `"vector":`) {
		t.Fatal("report text leaked vector field")
	}
}

func TestSearchReportJSONOutputValid(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	path := writeSearchSmokeResultFixture(t, validSearchSmokeEnvelope())

	result, err := lancedbpolicy.SearchReport(path, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := lancedbpolicy.WriteSearchReportJSON(result, &buf); err != nil {
		t.Fatalf("WriteSearchReportJSON() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded["status"] != "ok" {
		t.Fatalf("decoded status = %v, want ok", decoded["status"])
	}
}

type searchSmokeEnvelopeFixture struct {
	GeneratedAt string `json:"generated_at"`
	Search      struct {
		Status    string            `json:"status"`
		QueryMode string            `json:"query_mode"`
		TopK      int               `json:"top_k"`
		Results   []json.RawMessage `json:"results"`
	} `json:"search"`
	Summary lancedbpolicy.SearchSmokeSummary `json:"summary"`
}

func validSearchSmokeEnvelope() searchSmokeEnvelopeFixture {
	envelope := searchSmokeEnvelopeFixture{
		GeneratedAt: "2026-01-01T00:00:00Z",
	}
	envelope.Search.Status = lancedbpolicy.StatusOK
	envelope.Search.QueryMode = "vector"
	envelope.Search.TopK = 5
	envelope.Search.Results = []json.RawMessage{
		marshalSearchHit(lancedbpolicy.SearchHit{
			Rank:           1,
			ChunkID:        "chunk-a",
			VectorID:       "vec-a",
			Distance:       0.12,
			Domain:         "general",
			SourcePath:     "notes/a.md",
			SourceSHA256:   "sha-source",
			TextSHA256:     "sha-text",
			EmbeddingModel: "deterministic-hash-v1",
			Provider:       "local",
		}),
	}
	envelope.Summary = lancedbpolicy.SearchSmokeSummary{
		QueryMode:          "vector",
		TopK:               5,
		ResultCount:        1,
		DatabasePath:       "artifacts/lancedb-smoke",
		Table:              "memory_vectors",
		RetrievalPerformed: true,
		RunnerIntegration:  false,
		Provider:           "local",
		Model:              "deterministic-hash-v1",
	}
	return envelope
}

func marshalSearchHit(hit lancedbpolicy.SearchHit) json.RawMessage {
	raw, err := json.Marshal(hit)
	if err != nil {
		panic(err)
	}
	return raw
}

func mustRawSearchHit(t *testing.T, hit lancedbpolicy.SearchHit) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(hit)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return raw
}

func writeSearchSmokeResultFixture(t *testing.T, envelope searchSmokeEnvelopeFixture) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "lancedb-search-smoke-result.json")
	data, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
