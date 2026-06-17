package retrievalcontext_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/tasks"
	"gopkg.in/yaml.v3"
)

func TestLoadOKForValidatedSearchArtifacts(t *testing.T) {
	resultPath, reportPath, policyPath := writeValidatedSearchSmokeFixtures(t, validEnvelope())

	ctx, err := retrievalcontext.Load(tasks.RetrievalContextSpec{Attachments: []tasks.RetrievalContextAttachment{{
		Kind:       retrievalcontext.KindLanceDBSearchReport,
		Path:       resultPath,
		ReportPath: reportPath,
		Policy:     policyPath,
		MaxResults: 5,
	}}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ctx.Status != lancedbpolicy.StatusOK || ctx.Count() != 1 {
		t.Fatalf("context = %#v, want ok with 1 hit", ctx)
	}
	markdown := string(ctx.Markdown())
	if !strings.Contains(markdown, "Retrieved context metadata only") || !strings.Contains(markdown, "chunk-a") {
		t.Fatalf("markdown = %q, want passive metadata section", markdown)
	}
	if strings.Contains(markdown, `"vector"`) || strings.Contains(markdown, "SECRET-CHUNK-TEXT") {
		t.Fatal("markdown leaked forbidden payload")
	}
	jsonBytes, err := ctx.JSON()
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	if strings.Contains(string(jsonBytes), `"vector":`) {
		t.Fatal("json leaked vector field")
	}
}

func TestLoadFailsWhenStoredReportStatusFailed(t *testing.T) {
	resultPath, reportPath, policyPath := writeValidatedSearchSmokeFixtures(t, validEnvelope())
	reportData, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	reportData = bytesReplace(reportData, []byte(`"status": "ok"`), []byte(`"status": "failed"`))
	if err := os.WriteFile(reportPath, reportData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = retrievalcontext.Load(tasks.RetrievalContextSpec{Attachments: []tasks.RetrievalContextAttachment{{
		Kind:       retrievalcontext.KindLanceDBSearchReport,
		Path:       resultPath,
		ReportPath: reportPath,
		Policy:     policyPath,
		MaxResults: 5,
	}}})
	if err == nil || !strings.Contains(err.Error(), "search-report artifact status") {
		t.Fatalf("Load() error = %v, want stored report failure", err)
	}
}

func TestLoadFailsWhenForbiddenFieldPresent(t *testing.T) {
	envelope := validEnvelope()
	raw := marshalHit(lancedbpolicy.SearchHit{
		Rank: 1, ChunkID: "chunk-a", VectorID: "vec-a", Distance: 0.1,
	})
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	object["text"] = "SECRET-CHUNK-TEXT"
	raw, _ = json.Marshal(object)
	envelope.Search.Results = []json.RawMessage{raw}

	resultPath, reportPath, policyPath := writeValidatedSearchSmokeFixtures(t, envelope)
	_, err := retrievalcontext.Load(tasks.RetrievalContextSpec{Attachments: []tasks.RetrievalContextAttachment{{
		Kind:       retrievalcontext.KindLanceDBSearchReport,
		Path:       resultPath,
		ReportPath: reportPath,
		Policy:     policyPath,
		MaxResults: 5,
	}}})
	if err == nil || !strings.Contains(err.Error(), "search-report") {
		t.Fatalf("Load() error = %v, want search-report validation failure", err)
	}
}

func TestLoadRespectsMaxResults(t *testing.T) {
	envelope := validEnvelope()
	envelope.Summary.ResultCount = 2
	envelope.Summary.TopK = 5
	envelope.Search.Results = []json.RawMessage{
		marshalHit(lancedbpolicy.SearchHit{Rank: 1, ChunkID: "chunk-a", VectorID: "vec-a", Distance: 0.1, Domain: "general"}),
		marshalHit(lancedbpolicy.SearchHit{Rank: 2, ChunkID: "chunk-b", VectorID: "vec-b", Distance: 0.2, Domain: "general"}),
	}
	resultPath, reportPath, policyPath := writeValidatedSearchSmokeFixtures(t, envelope)

	ctx, err := retrievalcontext.Load(tasks.RetrievalContextSpec{Attachments: []tasks.RetrievalContextAttachment{{
		Kind:       retrievalcontext.KindLanceDBSearchReport,
		Path:       resultPath,
		ReportPath: reportPath,
		Policy:     policyPath,
		MaxResults: 1,
	}}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if ctx.Count() != 1 {
		t.Fatalf("count = %d, want 1", ctx.Count())
	}
}

type envelopeFixture struct {
	GeneratedAt string `json:"generated_at"`
	Search      struct {
		Status    string            `json:"status"`
		QueryMode string            `json:"query_mode"`
		TopK      int               `json:"top_k"`
		Results   []json.RawMessage `json:"results"`
	} `json:"search"`
	Summary lancedbpolicy.SearchSmokeSummary `json:"summary"`
}

func validEnvelope() envelopeFixture {
	envelope := envelopeFixture{GeneratedAt: "2026-01-01T00:00:00Z"}
	envelope.Search.Status = lancedbpolicy.StatusOK
	envelope.Search.QueryMode = "vector"
	envelope.Search.TopK = 5
	envelope.Search.Results = []json.RawMessage{marshalHit(lancedbpolicy.SearchHit{
		Rank: 1, ChunkID: "chunk-a", VectorID: "vec-a", Distance: 0.12, Domain: "general",
		SourcePath: "notes/a.md", SourceSHA256: "sha-source", TextSHA256: "sha-text",
		EmbeddingModel: "deterministic-hash-v1", Provider: "local",
	})}
	envelope.Summary = lancedbpolicy.SearchSmokeSummary{
		QueryMode: "vector", TopK: 5, ResultCount: 1,
		DatabasePath: "artifacts/lancedb-smoke", Table: "memory_vectors",
		RetrievalPerformed: true, RunnerIntegration: false,
	}
	return envelope
}

func marshalHit(hit lancedbpolicy.SearchHit) json.RawMessage {
	raw, err := json.Marshal(hit)
	if err != nil {
		panic(err)
	}
	return raw
}

func writeValidatedSearchSmokeFixtures(t *testing.T, envelope envelopeFixture) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "lancedb-policy.yaml")
	resultPath := filepath.Join(dir, "lancedb-search-smoke-result.json")
	reportPath := filepath.Join(dir, "lancedb-search-report.json")

	cfg := lancedbpolicy.Config{LanceDBPolicy: lancedbpolicy.Policy{
		Input: lancedbpolicy.InputConfig{
			EmbeddingManifest: "artifacts/memory-embedding-manifest.json",
			VectorsPath:       "artifacts/memory-index-vectors.jsonl",
		},
		Database: lancedbpolicy.DatabaseConfig{Path: "artifacts/lancedb-smoke", Table: "memory_vectors"},
		Schema: lancedbpolicy.SchemaConfig{
			VectorColumn:  "vector",
			TextRefColumn: "chunk_id",
			MetadataColumns: []string{
				"domain", "source_path", "source_sha256", "text_sha256", "embedding_model", "provider",
			},
		},
		Limits: lancedbpolicy.LimitsConfig{MaxVectors: 10000, ExpectedDimensions: 16},
		Mode:   lancedbpolicy.ModeWriteSmoke,
	}}
	policyBytes, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(policyPath, policyBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resultBytes, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	resultBytes = append(resultBytes, '\n')
	if err := os.WriteFile(resultPath, resultBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := lancedbpolicy.SearchReport(resultPath, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	reportBytes = append(reportBytes, '\n')
	if err := os.WriteFile(reportPath, reportBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return resultPath, reportPath, policyPath
}

func bytesReplace(data, old, new []byte) []byte {
	return []byte(strings.Replace(string(data), string(old), string(new), 1))
}
