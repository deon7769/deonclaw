package retrievalcontext_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestBundleOKWithValidArtifacts(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	result, err := retrievalcontext.Bundle(bundleOpts())
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || result.ContainsText {
		t.Fatalf("result = %#v, want ok bundle without text", result)
	}
	if result.MaterializedTextArtifact != "materialized.json" {
		t.Fatalf("materialized_text_artifact = %q", result.MaterializedTextArtifact)
	}

	raw, err := os.ReadFile("bundle.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(raw), `"text_excerpt"`) {
		t.Fatal("bundle json contains text_excerpt")
	}
}

func TestBundleFailsWhenInspectFails(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	artifactPath := writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	payload := readJSONFile(t, artifactPath)
	payload["hit_count"] = 99
	writeJSONFile(t, artifactPath, payload)

	_, err := retrievalcontext.Bundle(bundleOpts())
	if err == nil || !strings.Contains(err.Error(), "inspect status") {
		t.Fatalf("error = %v, want inspect failure", err)
	}
}

func TestBundleFailsWhenMaterializedReportFails(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	payload := readJSONFile(t, "materialized.json")
	payload["chunk_count"] = 99
	writeJSONFile(t, "materialized.json", payload)

	_, err := retrievalcontext.Bundle(bundleOpts())
	if err == nil || !strings.Contains(err.Error(), "materialized report status") {
		t.Fatalf("error = %v, want materialized report failure", err)
	}
}

func TestBundleWarningWhenMaterializedStatusWarning(t *testing.T) {
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

	result, err := retrievalcontext.Bundle(bundleOpts())
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", result.Status)
	}
}

func TestBundleFailsWhenSourceRetrievalContextSHA256Diverges(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	payload := readJSONFile(t, "materialized.json")
	payload["source_retrieval_context_sha256"] = "wrong-sha"
	writeJSONFile(t, "materialized.json", payload)

	_, err := retrievalcontext.Bundle(bundleOpts())
	if err == nil || !strings.Contains(err.Error(), "source_retrieval_context_sha256 mismatch") {
		t.Fatalf("error = %v, want sha mismatch failure", err)
	}
}

func TestBundleFailsWhenMaterializedChunkIDNotInRetrievalContext(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	payload := readJSONFile(t, "materialized.json")
	items := payload["items"].([]any)
	item := items[0].(map[string]any)
	item["chunk_id"] = "chunk-unknown"
	writeJSONFile(t, "materialized.json", payload)

	_, err := retrievalcontext.Bundle(bundleOpts())
	if err == nil || !strings.Contains(err.Error(), `chunk_id "chunk-unknown" not found`) {
		t.Fatalf("error = %v, want unknown chunk failure", err)
	}
}

func TestBundleTextAndSummaryDoNotContainTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	result, err := retrievalcontext.Bundle(bundleOpts())
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}

	var stdout bytes.Buffer
	if err := retrievalcontext.WriteBundleText(result, &stdout); err != nil {
		t.Fatalf("WriteBundleText() error = %v", err)
	}
	if strings.Contains(stdout.String(), "alpha text") {
		t.Fatal("bundle stdout leaked text_excerpt content")
	}

	summary, err := os.ReadFile("bundle.md")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(summary), "alpha text") || strings.Contains(string(summary), "text_excerpt") {
		t.Fatal("bundle summary leaked text_excerpt")
	}
}

func TestBundleRejectsBlockedPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	cases := []struct {
		name string
		mut  func(*retrievalcontext.BundleOptions)
	}{
		{
			name: "absolute retrieval context",
			mut: func(opts *retrievalcontext.BundleOptions) {
				opts.RetrievalContextPath = "/tmp/retrieval-context.json"
			},
		},
		{
			name: "parent traversal materialized",
			mut: func(opts *retrievalcontext.BundleOptions) {
				opts.MaterializedPath = "../materialized.json"
			},
		},
		{
			name: "secrets output",
			mut: func(opts *retrievalcontext.BundleOptions) {
				opts.OutputPath = "secrets/bundle.json"
			},
		},
		{
			name: "dotenv summary",
			mut: func(opts *retrievalcontext.BundleOptions) {
				opts.SummaryPath = ".env/bundle.md"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := bundleOpts()
			tc.mut(&opts)
			_, err := retrievalcontext.Bundle(opts)
			if err == nil {
				t.Fatal("expected blocked path error")
			}
		})
	}
}

func bundleOpts() retrievalcontext.BundleOptions {
	return retrievalcontext.BundleOptions{
		RetrievalContextPath: "retrieval-context.json",
		MaterializedPath:     "materialized.json",
		OutputPath:           "bundle.json",
		SummaryPath:          "bundle.md",
	}
}

func TestBundleJSONStdoutHasNoTextExcerpt(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})
	if _, err := retrievalcontext.Materialize(materializeOpts()); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	result, err := retrievalcontext.Bundle(bundleOpts())
	if err != nil {
		t.Fatalf("Bundle() error = %v", err)
	}
	var buf bytes.Buffer
	if err := retrievalcontext.WriteBundleJSON(result, &buf); err != nil {
		t.Fatalf("WriteBundleJSON() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	encoded, _ := json.Marshal(payload)
	if strings.Contains(string(encoded), "text_excerpt") {
		t.Fatal("bundle stdout json contains text_excerpt")
	}
}
