package lancedbpolicy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type fakeLanceDBSearcher struct {
	response lancedbpolicy.SearchResponse
	err      error
	lastReq  lancedbpolicy.SearchRequest
}

func (s *fakeLanceDBSearcher) Search(req lancedbpolicy.SearchRequest) (lancedbpolicy.SearchResponse, error) {
	s.lastReq = req
	if s.err != nil {
		return lancedbpolicy.SearchResponse{}, s.err
	}
	return s.response, nil
}

func testQueryVector(dim int) []float64 {
	out := make([]float64, dim)
	for i := range out {
		out[i] = float64(i) * 0.01
	}
	return out
}

func successfulDoctorReader(cfg lancedbpolicy.Config) *fakeLanceDBReader {
	return &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:                 lancedbpolicy.StatusOK,
		TableExists:            true,
		RowCount:               1,
		VectorColumnExists:     true,
		TextRefColumnExists:    true,
		MetadataColumnsPresent: cfg.LanceDBPolicy.Schema.MetadataColumns,
		InferredDimensions:     cfg.LanceDBPolicy.Limits.ExpectedDimensions,
		SampleChunkIDs:         []string{"chunk-a"},
	}}
}

func successfulFakeSearcher() *fakeLanceDBSearcher {
	return &fakeLanceDBSearcher{response: lancedbpolicy.SearchResponse{
		Status:    lancedbpolicy.StatusOK,
		QueryMode: "vector",
		TopK:      2,
		Results: []lancedbpolicy.SearchHit{
			{
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
			},
		},
	}}
}

func TestSearchSmokeRequiresConfirm(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	_, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir: "artifacts",
		QueryVector:  testQueryVector(16),
		TopK:         5,
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-search-smoke") {
		t.Fatalf("SearchSmoke() error = %v, want confirm requirement", err)
	}
}

func TestSearchSmokeRequiresExactlyOneQueryMode(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	base := lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		TopK:               5,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           successfulFakeSearcher(),
	}

	_, err := lancedbpolicy.SearchSmoke(cfg, base)
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("SearchSmoke() error = %v, want exactly one query mode", err)
	}

	opts := base
	opts.QueryVector = testQueryVector(16)
	opts.QueryChunkID = "chunk-a"
	_, err = lancedbpolicy.SearchSmoke(cfg, opts)
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("SearchSmoke() error = %v, want exactly one query mode", err)
	}
}

func TestSearchSmokeRejectsInvalidTopK(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	reader := successfulDoctorReader(cfg)
	searcher := successfulFakeSearcher()

	for _, topK := range []int{0, -1, 51} {
		_, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
			ArtifactsDir:       "artifacts",
			ConfirmSearchSmoke: true,
			QueryVector:        testQueryVector(16),
			TopK:               topK,
			DoctorReader:       reader,
			Searcher:           searcher,
		})
		if err == nil || !strings.Contains(err.Error(), "top_k") {
			t.Fatalf("SearchSmoke(top_k=%d) error = %v, want top_k validation failure", topK, err)
		}
	}
}

func TestSearchSmokeRejectsQueryVectorDimensionMismatch(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	_, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryVector:        testQueryVector(15),
		TopK:               5,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           successfulFakeSearcher(),
	})
	if err == nil || !strings.Contains(err.Error(), "expected_dimensions") {
		t.Fatalf("SearchSmoke() error = %v, want dimension mismatch", err)
	}
}

func TestSearchSmokeBlockedWhenDoctorFailed(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	cfg := testLancedbWriteSmokePolicy()
	if err := os.MkdirAll(cfg.LanceDBPolicy.Database.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	reader := &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:      lancedbpolicy.StatusOK,
		TableExists: false,
		RowCount:    0,
	}}
	_, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryVector:        testQueryVector(16),
		TopK:               5,
		DoctorReader:       reader,
		Searcher:           successfulFakeSearcher(),
	})
	if err == nil || !strings.Contains(err.Error(), "doctor status") {
		t.Fatalf("SearchSmoke() error = %v, want doctor block", err)
	}
}

func TestSearchSmokeWithFakeSearcherWritesArtifacts(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll("artifacts/lancedb-smoke", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfg := testLancedbWriteSmokePolicy()

	result, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryVector:        testQueryVector(16),
		TopK:               5,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           successfulFakeSearcher(),
	})
	if err != nil {
		t.Fatalf("SearchSmoke() error = %v", err)
	}
	for _, path := range []string{
		result.ResultPath,
		result.SummaryPath,
		result.LogPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("Stat(%q) error = %v", path, err)
		}
	}
	if result.Summary.RetrievalPerformed != true || result.Summary.RunnerIntegration != false {
		t.Fatalf("summary flags = %#v, want retrieval true and runner false", result.Summary)
	}
}

func TestSearchSmokeSummaryDoesNotContainFullVector(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll("artifacts/lancedb-smoke", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfg := testLancedbWriteSmokePolicy()

	result, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryVector:        testQueryVector(16),
		TopK:               5,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           successfulFakeSearcher(),
	})
	if err != nil {
		t.Fatalf("SearchSmoke() error = %v", err)
	}

	summaryBytes, err := os.ReadFile(result.SummaryPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	summary := string(summaryBytes)
	if strings.Contains(summary, `"vector"`) || strings.Contains(summary, "0.01, 0.02") {
		t.Fatal("summary leaked vector payload")
	}

	resultBytes, err := os.ReadFile(result.ResultPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	resultText := string(resultBytes)
	if strings.Contains(resultText, `"vector":`) {
		t.Fatal("result json leaked vector field")
	}
}

func TestSearchSmokeResultJSONDoesNotContainChunkText(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	root := t.TempDir()
	chdirTo(t, root)
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll("artifacts/lancedb-smoke", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfg := testLancedbWriteSmokePolicy()
	searcher := &fakeLanceDBSearcher{response: lancedbpolicy.SearchResponse{
		Status:    lancedbpolicy.StatusOK,
		QueryMode: "chunk_id",
		TopK:      1,
		Results: []lancedbpolicy.SearchHit{{
			Rank:     1,
			ChunkID:  "chunk-a",
			VectorID: "vec-a",
			Distance: 0.1,
		}},
	}}

	result, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryChunkID:       "chunk-a",
		TopK:               1,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           searcher,
	})
	if err != nil {
		t.Fatalf("SearchSmoke() error = %v", err)
	}
	_ = marker
	resultBytes, err := os.ReadFile(result.ResultPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(resultBytes), marker) || strings.Contains(string(resultBytes), `"text"`) {
		t.Fatal("result json leaked chunk text")
	}
	if searcher.lastReq.QueryChunkID != "chunk-a" {
		t.Fatalf("lastReq.QueryChunkID = %q, want chunk-a", searcher.lastReq.QueryChunkID)
	}
}

func TestSearchSmokeFailsWhenQueryChunkIDMissing(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll("artifacts/lancedb-smoke", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfg := testLancedbWriteSmokePolicy()
	searcher := &fakeLanceDBSearcher{err: &chunkNotFoundError{id: "missing-chunk"}}

	_, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryChunkID:       "missing-chunk",
		TopK:               5,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           searcher,
	})
	if err == nil || !strings.Contains(err.Error(), "missing-chunk") {
		t.Fatalf("SearchSmoke() error = %v, want missing chunk failure", err)
	}
}

type chunkNotFoundError struct {
	id string
}

func (e *chunkNotFoundError) Error() string {
	return "query_chunk_id " + e.id + " not found"
}

func TestPreflightSearchSmokeFailsWhenLanceDBMissing(t *testing.T) {
	result := lancedbpolicy.PreflightSearchSmoke()
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.Message == "" {
		t.Fatal("message is empty, want actionable dependency error")
	}
}

func TestSearchSmokeLoadsProviderModelFromWriteManifest(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	if err := os.MkdirAll("artifacts", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.MkdirAll("artifacts/lancedb-smoke", 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	cfg := testLancedbWriteSmokePolicy()
	manifest := lancedbpolicy.WriteSmokeManifest{
		Provider: "local",
		Model:    "deterministic-hash-v1",
	}
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join("artifacts", "lancedb-write-smoke-manifest.json"), manifestBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := lancedbpolicy.SearchSmoke(cfg, lancedbpolicy.SearchSmokeOptions{
		ArtifactsDir:       "artifacts",
		ConfirmSearchSmoke: true,
		QueryVector:        testQueryVector(16),
		TopK:               3,
		DoctorReader:       successfulDoctorReader(cfg),
		Searcher:           successfulFakeSearcher(),
	})
	if err != nil {
		t.Fatalf("SearchSmoke() error = %v", err)
	}
	if result.Summary.Provider != "local" || result.Summary.Model != "deterministic-hash-v1" {
		t.Fatalf("summary provider/model = %q/%q", result.Summary.Provider, result.Summary.Model)
	}
}
