package lancedbpolicy_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
)

func TestValidateAcceptsValidPolicy(t *testing.T) {
	if err := lancedbpolicy.Validate(testLancedbPolicy()); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidTableName(t *testing.T) {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Database.Table = "bad-table"
	if err := lancedbpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want invalid table failure")
	}
}

func TestValidateRejectsAbsoluteDatabasePath(t *testing.T) {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Database.Path = "/tmp/lancedb"
	if err := lancedbpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want absolute database path failure")
	}
}

func TestValidateRejectsParentTraversalDatabasePath(t *testing.T) {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Database.Path = "../lancedb"
	if err := lancedbpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want parent traversal failure")
	}
}

func TestValidateRejectsSecretsDatabasePath(t *testing.T) {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Database.Path = "artifacts/secrets/lancedb"
	if err := lancedbpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want secrets path failure")
	}
}

func TestValidateRejectsDotEnvDatabasePath(t *testing.T) {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Database.Path = "artifacts/.env/lancedb"
	if err := lancedbpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want .env path failure")
	}
}

func TestPlanOKForFakeVectorArtifacts(t *testing.T) {
	cfg := setupLancedbPlan(t, 2)

	plan, err := lancedbpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != lancedbpolicy.StatusOK {
		t.Fatalf("status = %q, want ok; warnings=%#v", plan.Status, plan.Warnings)
	}
	if plan.VectorCount != 2 || plan.EstimatedRows != 2 {
		t.Fatalf("plan = %#v, want 2 rows", plan)
	}
	if plan.WouldWriteLanceDB || !plan.PlanOnly {
		t.Fatalf("flags = would_write=%t plan_only=%t", plan.WouldWriteLanceDB, plan.PlanOnly)
	}
}

func TestPlanFailsWhenEmbeddingReportFails(t *testing.T) {
	cfg := setupLancedbPlan(t, 1)
	manifestPath := cfg.LanceDBPolicy.Input.EmbeddingManifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = bytes.Replace(manifestData, []byte(`"vector_count": 1`), []byte(`"vector_count": 99`), 1)
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	plan, err := lancedbpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", plan.Status)
	}
}

func TestPlanFailsWhenDimensionsDiffer(t *testing.T) {
	cfg := setupLancedbPlan(t, 1)
	cfg.LanceDBPolicy.Limits.ExpectedDimensions = 32

	plan, err := lancedbpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", plan.Status)
	}
}

func TestPlanFailsWhenVectorCountExceedsMaxVectors(t *testing.T) {
	cfg := setupLancedbPlan(t, 3)
	cfg.LanceDBPolicy.Limits.MaxVectors = 2

	plan, err := lancedbpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", plan.Status)
	}
}

func TestPlanTextShowsWouldWriteLanceDBFalse(t *testing.T) {
	cfg := setupLancedbPlan(t, 1)
	plan, err := lancedbpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	var buf bytes.Buffer
	if err := lancedbpolicy.WritePlanText(plan, &buf); err != nil {
		t.Fatalf("WritePlanText() error = %v", err)
	}
	if !strings.Contains(buf.String(), "would_write_lancedb: false") {
		t.Fatalf("output = %q, want would_write_lancedb false", buf.String())
	}
}

func TestPlanDoesNotCreateDatabaseDirectory(t *testing.T) {
	cfg := setupLancedbPlan(t, 1)
	dbPath := cfg.LanceDBPolicy.Database.Path
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatalf("database path %q exists before plan", dbPath)
	}
	if _, err := lancedbpolicy.Plan(cfg); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("database path %q was created by plan", dbPath)
	}
}

func testLancedbPolicy() lancedbpolicy.Config {
	return lancedbpolicy.Config{
		LanceDBPolicy: lancedbpolicy.Policy{
			Input: lancedbpolicy.InputConfig{
				EmbeddingManifest: "artifacts/memory-embedding-manifest.json",
				VectorsPath:       "artifacts/memory-index-vectors.jsonl",
				ChunksPath:        "artifacts/memory-index-chunks.jsonl",
			},
			Database: lancedbpolicy.DatabaseConfig{
				Path:  "artifacts/lancedb",
				Table: "memory_vectors",
			},
			Schema: lancedbpolicy.SchemaConfig{
				VectorColumn:  "vector",
				TextRefColumn: "chunk_id",
				MetadataColumns: []string{
					"domain",
					"source_path",
					"source_sha256",
					"text_sha256",
					"embedding_model",
					"provider",
				},
			},
			Limits: lancedbpolicy.LimitsConfig{
				MaxVectors:         10000,
				ExpectedDimensions: 16,
			},
			Mode: lancedbpolicy.ModePlanOnly,
		},
	}
}

func setupLancedbPlan(t *testing.T, chunkCount int) lancedbpolicy.Config {
	t.Helper()
	root := t.TempDir()
	chdirTo(t, root)
	buildFakeArtifacts(t, root, chunkCount)
	return testLancedbPolicy()
}

func buildFakeArtifacts(t *testing.T, root string, chunkCount int) {
	t.Helper()
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeIndexArtifacts(t, artifactsDir, chunkCount)

	cfg := embeddingpolicy.Config{
		EmbeddingPolicy: embeddingpolicy.Policy{
			Provider:   embeddingpolicy.ProviderLocal,
			Model:      "deterministic-hash-v1",
			Dimensions: 16,
			Input: embeddingpolicy.InputConfig{
				ChunksPath:   "artifacts/memory-index-chunks.jsonl",
				ManifestPath: "artifacts/memory-index-manifest.json",
			},
			Output: embeddingpolicy.OutputConfig{
				VectorsPath:  "artifacts/memory-index-vectors.jsonl",
				ManifestPath: "artifacts/memory-embedding-manifest.json",
			},
			Limits: embeddingpolicy.LimitsConfig{
				MaxChunks:     10000,
				MaxChunkChars: 8000,
				BatchSize:     64,
			},
			Mode: embeddingpolicy.ModeFakeVectors,
		},
	}
	policyBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := embeddingpolicy.BuildFake(cfg, policyBytes, "artifacts", true); err != nil {
		t.Fatalf("BuildFake() error = %v", err)
	}
}

func writeIndexArtifacts(t *testing.T, artifactsDir string, chunkCount int) {
	t.Helper()
	chunksPath := filepath.Join(artifactsDir, "memory-index-chunks.jsonl")
	manifestPath := filepath.Join(artifactsDir, "memory-index-manifest.json")
	var chunks strings.Builder
	for i := 0; i < chunkCount; i++ {
		id := "chunk-" + string(rune('a'+i))
		text := "# note " + string(rune('a'+i)) + "\n"
		chunks.WriteString(chunkLine(id, text))
		chunks.WriteByte('\n')
	}
	writeFile(t, chunksPath, chunks.String())

	manifest := memoryindex.Manifest{
		GeneratedAt:  "2026-01-01T00:00:00Z",
		ConfigSHA256: "abc",
		Domains:      []string{"mysecondbrain"},
		SourceCount:  1,
		ChunkCount:   chunkCount,
		ManifestPath: manifestPath,
		ChunksPath:   chunksPath,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	writeFile(t, manifestPath, string(data)+"\n")
}

func chunkLine(id string, text string) string {
	return `{"id":"` + id + `","domain":"mysecondbrain","source_path":"note.md","source_sha256":"abc","chunk_index":0,"text":` + jsonString(text) + `,"text_sha256":"` + sha256Hex([]byte(text)) + `","char_start":0,"char_end":` + strconv.Itoa(len([]rune(text))) + `}`
}

func jsonString(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

func chdirTo(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
}
