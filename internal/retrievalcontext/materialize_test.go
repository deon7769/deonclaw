package retrievalcontext_test

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestMaterializeRequiresConfirmFlag(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})

	_, err := retrievalcontext.Materialize(retrievalcontext.MaterializeOptions{
		RetrievalContextPath: "retrieval-context.json",
		ChunksPath:           "chunks.jsonl",
		OutputPath:           "out.json",
		SummaryPath:          "out.md",
		MaxCharsPerChunk:     1200,
		MaxTotalChars:        6000,
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-include-chunk-text") {
		t.Fatalf("error = %v, want confirm flag required", err)
	}
}

func TestMaterializeFailsWhenInspectFails(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	artifact := writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})

	payload := readJSONFile(t, artifact)
	payload["hit_count"] = 99
	writeJSONFile(t, "retrieval-context.json", payload)

	_, err := retrievalcontext.Materialize(materializeOpts())
	if err == nil || !strings.Contains(err.Error(), "inspect status") {
		t.Fatalf("error = %v, want inspect failure", err)
	}
}

func TestMaterializeFailsWhenChunkMissing(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("other-chunk", "alpha text", "sha-source")})

	_, err := retrievalcontext.Materialize(materializeOpts())
	if err == nil || !strings.Contains(err.Error(), `chunk_id "chunk-a" not found`) {
		t.Fatalf("error = %v, want missing chunk failure", err)
	}
}

func TestMaterializeFailsOnTextSHA256Mismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	chunk := sampleChunk("chunk-a", "alpha text", "sha-source")
	chunk.TextSHA256 = "wrong"
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{chunk})

	_, err := retrievalcontext.Materialize(materializeOpts())
	if err == nil || !strings.Contains(err.Error(), "text_sha256 mismatch") {
		t.Fatalf("error = %v, want text_sha256 mismatch", err)
	}
}

func TestMaterializeFailsOnSourceSHA256Mismatch(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	payload := validRetrievalArtifactMap(t)
	attachments := payload["attachments"].([]map[string]any)
	hits := attachments[0]["hits"].([]map[string]any)
	hits[0]["source_sha256"] = "wrong-source"
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, payload))

	chunk := sampleChunk("chunk-a", "alpha text", "sha-source")
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{chunk})

	_, err := retrievalcontext.Materialize(materializeOpts())
	if err == nil || !strings.Contains(err.Error(), "source_sha256 mismatch") {
		t.Fatalf("error = %v, want source_sha256 mismatch", err)
	}
}

func TestMaterializeTruncatesPerChunkWithWarning(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	longText := strings.Repeat("x", 40)
	payload := validRetrievalArtifactMap(t)
	attachments := payload["attachments"].([]map[string]any)
	hits := attachments[0]["hits"].([]map[string]any)
	hits[0]["text_sha256"] = textSHA(longText)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, payload))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", longText, "sha-source")})

	opts := materializeOpts()
	opts.MaxCharsPerChunk = 10
	opts.MaxTotalChars = 6000

	result, err := retrievalcontext.Materialize(opts)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if len(result.Items) != 1 || !result.Items[0].Truncated || result.Items[0].IncludedChars != 10 {
		t.Fatalf("items = %#v, want truncated 10-char excerpt", result.Items)
	}
	if result.Status != lancedbpolicy.StatusWarning {
		t.Fatalf("status = %q, want warning", result.Status)
	}
	if !strings.Contains(strings.Join(result.Warnings, " "), "max_chars_per_chunk") {
		t.Fatalf("warnings = %#v, want per-chunk truncation warning", result.Warnings)
	}
}

func TestMaterializeRespectsMaxTotalChars(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	payload := validRetrievalArtifactMap(t)
	attachments := payload["attachments"].([]map[string]any)
	attachments[0]["hit_count"] = 2
	attachments[0]["hits"] = []map[string]any{
		{
			"rank": 1, "chunk_id": "chunk-a", "distance": 0.1,
			"domain": "general", "source_path": "notes/a.md",
			"source_sha256": "sha-source-a", "text_sha256": textSHA("aaaaa"),
		},
		{
			"rank": 2, "chunk_id": "chunk-b", "distance": 0.2,
			"domain": "general", "source_path": "notes/b.md",
			"source_sha256": "sha-source-b", "text_sha256": textSHA("bbbb"),
		},
	}
	payload["hit_count"] = 2
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, payload))

	_ = writeChunksFile(t, dir, []memoryindex.Chunk{
		sampleChunkWithSource("chunk-a", "aaaaa", "sha-source-a"),
		sampleChunkWithSource("chunk-b", "bbbb", "sha-source-b"),
	})

	opts := materializeOpts()
	opts.MaxCharsPerChunk = 10
	opts.MaxTotalChars = 5

	result, err := retrievalcontext.Materialize(opts)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if result.IncludedChunkCount != 1 || result.OmittedChunkCount != 1 {
		t.Fatalf("included=%d omitted=%d, want 1/1", result.IncludedChunkCount, result.OmittedChunkCount)
	}
	if result.TotalCharsIncluded != 5 {
		t.Fatalf("total_chars_included = %d, want 5", result.TotalCharsIncluded)
	}
}

func TestMaterializeOutputJSONHasExcerptNotVector(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})

	opts := materializeOpts()
	result, err := retrievalcontext.Materialize(opts)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}

	raw, err := os.ReadFile(opts.OutputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	body := string(raw)
	for _, forbidden := range []string{`"vector"`, `"embedding"`, `"chunk_text"`} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("output json contains forbidden field %s", forbidden)
		}
	}
	if !strings.Contains(body, `"text_excerpt"`) {
		t.Fatal("output json missing text_excerpt")
	}
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", result.Status)
	}
}

func TestMaterializeSummaryHasNoVectorOrEmbedding(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})

	opts := materializeOpts()
	if _, err := retrievalcontext.Materialize(opts); err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	body, err := os.ReadFile(opts.SummaryPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	summary := string(body)
	if !strings.Contains(summary, "Materialized retrieval context") {
		t.Fatal("summary missing title")
	}
	if !strings.Contains(summary, "Markdown + Git remain the source of truth") {
		t.Fatal("summary missing canonical source disclaimer")
	}
	for _, forbidden := range []string{"vector:", "embedding:", `"vector"`, `"embedding"`} {
		if strings.Contains(summary, forbidden) {
			t.Fatalf("summary contains forbidden token %q", forbidden)
		}
	}
}

func TestMaterializeRejectsBlockedOutputPaths(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{sampleChunk("chunk-a", "alpha text", "sha-source")})

	cases := []struct {
		name string
		mut  func(*retrievalcontext.MaterializeOptions)
	}{
		{
			name: "absolute output",
			mut: func(opts *retrievalcontext.MaterializeOptions) {
				opts.OutputPath = "/tmp/out.json"
			},
		},
		{
			name: "parent traversal output",
			mut: func(opts *retrievalcontext.MaterializeOptions) {
				opts.OutputPath = "../out.json"
			},
		},
		{
			name: "secrets output",
			mut: func(opts *retrievalcontext.MaterializeOptions) {
				opts.OutputPath = "secrets/out.json"
			},
		},
		{
			name: "dotenv output",
			mut: func(opts *retrievalcontext.MaterializeOptions) {
				opts.OutputPath = ".env/out.json"
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := materializeOpts()
			tc.mut(&opts)
			_, err := retrievalcontext.Materialize(opts)
			if err == nil {
				t.Fatal("expected blocked output path error")
			}
		})
	}
}

func TestMaterializeDoesNotOpenSourceFiles(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	_ = writeRetrievalArtifactFile(t, dir, mustMarshal(t, validRetrievalArtifactMap(t)))
	_ = writeChunksFile(t, dir, []memoryindex.Chunk{
		sampleChunkWithSource("chunk-a", "alpha text", "sha-source"),
	})

	payload := readJSONFile(t, "retrieval-context.json")
	attachments := payload["attachments"].([]any)
	attachment := attachments[0].(map[string]any)
	hits := attachment["hits"].([]any)
	hit := hits[0].(map[string]any)
	hit["source_path"] = "missing-source.md"
	writeJSONFile(t, "retrieval-context.json", payload)

	opts := materializeOpts()
	result, err := retrievalcontext.Materialize(opts)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if _, err := os.Stat("missing-source.md"); !os.IsNotExist(err) {
		t.Fatalf("source file stat err = %v, want not exist", err)
	}
	if len(result.Items) != 1 || result.Items[0].TextExcerpt != "alpha text" {
		t.Fatalf("items = %#v, want excerpt from chunks jsonl only", result.Items)
	}
}

func materializeOpts() retrievalcontext.MaterializeOptions {
	return retrievalcontext.MaterializeOptions{
		RetrievalContextPath:    "retrieval-context.json",
		ChunksPath:              "chunks.jsonl",
		OutputPath:              "materialized.json",
		SummaryPath:             "materialized.md",
		MaxCharsPerChunk:        1200,
		MaxTotalChars:           6000,
		ConfirmIncludeChunkText: true,
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})
}

func writeRetrievalArtifactFile(t *testing.T, dir string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, "retrieval-context.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return "retrieval-context.json"
}

func writeChunksFile(t *testing.T, dir string, chunks []memoryindex.Chunk) string {
	t.Helper()
	path := filepath.Join(dir, "chunks.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	defer file.Close()
	writer := bufio.NewWriter(file)
	for _, chunk := range chunks {
		if chunk.TextSHA256 == "" {
			chunk.TextSHA256 = textSHA(chunk.Text)
		}
		line, err := json.Marshal(chunk)
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if _, err := writer.Write(line); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if err := writer.WriteByte('\n'); err != nil {
			t.Fatalf("WriteByte() error = %v", err)
		}
	}
	if err := writer.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	return path
}

func sampleChunk(id, text, sourceSHA string) memoryindex.Chunk {
	return sampleChunkWithSource(id, text, sourceSHA)
}

func sampleChunkWithSource(id, text, sourceSHA string) memoryindex.Chunk {
	return memoryindex.Chunk{
		ID:           id,
		Domain:       "mysecondbrain",
		SourcePath:   "notes/" + id + ".md",
		SourceSHA256: sourceSHA,
		ChunkIndex:   0,
		Text:         text,
		TextSHA256:   textSHA(text),
		CharStart:    0,
		CharEnd:      len([]rune(text)),
	}
}

func validRetrievalArtifactMap(t *testing.T) map[string]any {
	t.Helper()
	text := "alpha text"
	return map[string]any{
		"status":                     "ok",
		"retrieval_context_attached": true,
		"hit_count":                  1,
		"attachments": []map[string]any{{
			"name":       "lancedb-search-1",
			"kind":       retrievalcontext.KindLanceDBSearchReport,
			"query_mode": "vector",
			"hit_count":  1,
			"hits": []map[string]any{{
				"rank":            1,
				"chunk_id":        "chunk-a",
				"distance":        0.12,
				"domain":          "general",
				"source_path":     "notes/a.md",
				"source_sha256":   "sha-source",
				"text_sha256":     textSHA(text),
				"embedding_model": "deterministic-hash-v1",
				"provider":        "local",
			}},
		}},
	}
}

func mustMarshal(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return append(data, '\n')
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return payload
}

func writeJSONFile(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	if err := os.WriteFile(path, mustMarshal(t, payload), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func textSHA(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
