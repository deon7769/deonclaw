package retrievalcontext_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializedReportOKForValidArtifact(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	result, err := retrievalcontext.Materialize(materializeOpts())
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	report, err := retrievalcontext.MaterializedReport("materialized.json")
	if err != nil {
		t.Fatalf("MaterializedReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if report.ChunkCount != 1 || report.IncludedChunkCount != 1 || report.TotalCharsIncluded != len([]rune("alpha text")) {
		t.Fatalf("report = %#v", report)
	}
	if len(report.UniqueChunkIDs) != 1 || report.UniqueChunkIDs[0] != "chunk-a" {
		t.Fatalf("unique_chunk_ids = %#v", report.UniqueChunkIDs)
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("materialize status = %q, want ok", result.Status)
	}
}

func TestMaterializedReportAcceptsWarningStatus(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	longText := strings.Repeat("x", 40)
	payload := validRetrievalArtifactMap(t)
	attachments := payload["attachments"].([]map[string]any)
	hits := attachments[0]["hits"].([]map[string]any)
	hits[0]["text_sha256"] = textSHA(longText)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, payload))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", longText, "sha-source")})

	opts := materializeOpts()
	opts.MaxCharsPerChunk = 10
	if _, err := retrievalcontext.Materialize(opts); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	report, err := retrievalcontext.MaterializedReport("materialized.json")
	if err != nil {
		t.Fatalf("MaterializedReport() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", report.Status)
	}
	if len(report.Warnings) == 0 {
		t.Fatal("expected artifact warnings in report")
	}
}

func TestMaterializedReportFailsWhenCountsDiverge(t *testing.T) {
	artifact := validMaterializedArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	payload["chunk_count"] = 99
	artifact, _ = json.Marshal(payload)

	report, err := retrievalcontext.MaterializedReportBytes(artifact)
	if err != nil {
		t.Fatalf("MaterializedReportBytes() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestMaterializedReportFailsWhenIncludedCharsDivergeFromText(t *testing.T) {
	artifact := validMaterializedArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	items := payload["items"].([]any)
	item := items[0].(map[string]any)
	item["included_chars"] = 99
	artifact, _ = json.Marshal(payload)

	report, err := retrievalcontext.MaterializedReportBytes(artifact)
	if err != nil {
		t.Fatalf("MaterializedReportBytes() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestMaterializedReportFailsWhenTotalCharsExceedsLimit(t *testing.T) {
	artifact := validMaterializedArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	payload["total_chars_included"] = 9999
	artifact, _ = json.Marshal(payload)

	report, err := retrievalcontext.MaterializedReportBytes(artifact)
	if err != nil {
		t.Fatalf("MaterializedReportBytes() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestMaterializedReportFailsWhenForbiddenFieldPresent(t *testing.T) {
	artifact := validMaterializedArtifactJSON(t)
	var payload map[string]any
	if err := json.Unmarshal(artifact, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	items := payload["items"].([]any)
	item := items[0].(map[string]any)
	item["embedding"] = []float64{0.1, 0.2}
	artifact, _ = json.Marshal(payload)

	report, err := retrievalcontext.MaterializedReportBytes(artifact)
	if err != nil {
		t.Fatalf("MaterializedReportBytes() error = %v", err)
	}
	if report.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestMaterializedReportTextDoesNotPrintTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	report, err := retrievalcontext.MaterializedReport("materialized.json")
	if err != nil {
		t.Fatalf("MaterializedReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteMaterializedReportText(report, &buf); err != nil {
		t.Fatalf("WriteMaterializedReportText() error = %v", err)
	}
	output := buf.String()
	if strings.Contains(output, "alpha text") {
		t.Fatal("report text leaked text_excerpt content")
	}
}

func TestMaterializeRejectsBlockedRetrievalContextPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})

	cases := []struct {
		name string
		path string
	}{
		{name: "absolute", path: "/tmp/retrieval-context.json"},
		{name: "parent traversal", path: "../retrieval-context.json"},
		{name: "secrets", path: "secrets/retrieval-context.json"},
		{name: "dotenv", path: ".env/retrieval-context.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := materializeOpts()
			opts.RetrievalContextPath = tc.path
			_, err := retrievalcontext.Materialize(opts)
			if err == nil {
				t.Fatal("expected blocked retrieval context path error")
			}
		})
	}
}

func validMaterializedArtifactJSON(t *testing.T) []byte {
	t.Helper()
	text := "alpha text"
	payload := map[string]any{
		"status":                          "ok",
		"source_retrieval_context_sha256": "sha-retrieval",
		"source_chunks_sha256":            "sha-chunks",
		"chunk_count":                     1,
		"included_chunk_count":            1,
		"omitted_chunk_count":             0,
		"max_chars_per_chunk":             1200,
		"max_total_chars":                 6000,
		"total_chars_included":            len([]rune(text)),
		"items": []map[string]any{{
			"rank":           1,
			"chunk_id":       "chunk-a",
			"distance":       0.12,
			"text_sha256":    textSHA(text),
			"text_excerpt":   text,
			"truncated":      false,
			"included_chars": len([]rune(text)),
		}},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return data
}
