package lancedbpolicy_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/memoryindex"
)

func TestValidateAcceptsFakeWritePolicy(t *testing.T) {
	cfg := testLancedbFakeWritePolicy()
	if err := lancedbpolicy.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFakeWriteRequiresConfirmFlag(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)

	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", false)
	if err == nil || !strings.Contains(err.Error(), "--confirm-fake-write") {
		t.Fatalf("FakeWrite() error = %v, want confirm flag requirement", err)
	}
}

func TestFakeWriteRejectsPlanOnlyMode(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	cfg.LanceDBPolicy.Mode = lancedbpolicy.ModePlanOnly

	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true)
	if err == nil || !strings.Contains(err.Error(), "fake_write") {
		t.Fatalf("FakeWrite() error = %v, want fake_write mode requirement", err)
	}
}

func TestFakeWriteRejectsDimensionsMismatch(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	cfg.LanceDBPolicy.Limits.ExpectedDimensions = 32

	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true)
	if err == nil || !strings.Contains(err.Error(), "plan status") {
		t.Fatalf("FakeWrite() error = %v, want plan failure", err)
	}
}

func TestFakeWriteGeneratesManifestAndRows(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 2)

	result, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true)
	if err != nil {
		t.Fatalf("FakeWrite() error = %v", err)
	}
	if result.Manifest.RowCount != 2 {
		t.Fatalf("row_count = %d, want 2", result.Manifest.RowCount)
	}
	manifestPath := filepath.Join("artifacts", "lancedb-fake-write-manifest.json")
	rowsPath := filepath.Join("artifacts", "lancedb-fake-rows.jsonl")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	if _, err := os.Stat(rowsPath); err != nil {
		t.Fatalf("rows missing: %v", err)
	}
	if result.Manifest.FakeWrite != true || result.Manifest.LanceDBWritten {
		t.Fatalf("manifest flags = fake_write:%t lancedb_written:%t", result.Manifest.FakeWrite, result.Manifest.LanceDBWritten)
	}
}

func TestFakeWriteRowCountMatchesVectorCount(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 3)

	result, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true)
	if err != nil {
		t.Fatalf("FakeWrite() error = %v", err)
	}
	rows := readFakeRowsJSONL(t, filepath.Join("artifacts", "lancedb-fake-rows.jsonl"))
	if len(rows) != 3 || result.Manifest.RowCount != 3 {
		t.Fatalf("rows = %d manifest row_count = %d, want 3", len(rows), result.Manifest.RowCount)
	}
}

func TestFakeWriteRowsDoNotContainChunkText(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	cfg, policyBytes := setupLancedbFakeWriteWithText(t, marker, 1)

	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true)
	if err != nil {
		t.Fatalf("FakeWrite() error = %v", err)
	}
	rowsData, err := os.ReadFile(filepath.Join("artifacts", "lancedb-fake-rows.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(rowsData), marker) {
		t.Fatal("fake rows leaked chunk text")
	}
}

func TestFakeWriteRowsDoNotContainFullVector(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)

	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true)
	if err != nil {
		t.Fatalf("FakeWrite() error = %v", err)
	}
	rowsData, err := os.ReadFile(filepath.Join("artifacts", "lancedb-fake-rows.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(rowsData)
	if strings.Contains(content, `"vector":`) {
		t.Fatal("fake rows contain full vector field")
	}
	vectorsData, err := os.ReadFile(cfg.LanceDBPolicy.Input.VectorsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var vectorRecord struct {
		Vector []float64 `json:"vector"`
	}
	if err := json.Unmarshal([]byte(strings.Split(strings.TrimSpace(string(vectorsData)), "\n")[0]), &vectorRecord); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	vectorJSON, err := json.Marshal(vectorRecord.Vector)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(content, string(vectorJSON)) {
		t.Fatal("fake rows leaked full vector array")
	}
}

func TestFakeWriteDoesNotCreateDatabaseDirectory(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	dbPath := cfg.LanceDBPolicy.Database.Path
	if _, err := os.Stat(dbPath); err == nil {
		t.Fatalf("database path %q exists before fake write", dbPath)
	}
	if _, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts", true); err != nil {
		t.Fatalf("FakeWrite() error = %v", err)
	}
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("database path %q was created by fake write", dbPath)
	}
}

func TestFakeWriteRejectsAbsoluteArtifactsDir(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "/tmp/artifacts", true)
	if err == nil {
		t.Fatal("FakeWrite() error = nil, want absolute artifacts dir failure")
	}
}

func TestFakeWriteRejectsParentTraversalArtifactsDir(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "../artifacts", true)
	if err == nil {
		t.Fatal("FakeWrite() error = nil, want parent traversal failure")
	}
}

func TestFakeWriteRejectsSecretsArtifactsDir(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts/secrets", true)
	if err == nil {
		t.Fatal("FakeWrite() error = nil, want secrets path failure")
	}
}

func TestFakeWriteRejectsDotEnvArtifactsDir(t *testing.T) {
	cfg, policyBytes := setupLancedbFakeWrite(t, 1)
	_, err := lancedbpolicy.FakeWrite(cfg, policyBytes, "artifacts/.env", true)
	if err == nil {
		t.Fatal("FakeWrite() error = nil, want .env path failure")
	}
}

func testLancedbFakeWritePolicy() lancedbpolicy.Config {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Mode = lancedbpolicy.ModeFakeWrite
	cfg.LanceDBPolicy.Database.Path = "artifacts/lancedb-fake"
	return cfg
}

func setupLancedbFakeWrite(t *testing.T, chunkCount int) (lancedbpolicy.Config, []byte) {
	t.Helper()
	return setupLancedbFakeWriteWithText(t, "# note\n", chunkCount)
}

func setupLancedbFakeWriteWithText(t *testing.T, text string, chunkCount int) (lancedbpolicy.Config, []byte) {
	t.Helper()
	root := t.TempDir()
	chdirTo(t, root)
	if chunkCount == 1 {
		buildFakeArtifactsWithText(t, root, text)
	} else {
		buildFakeArtifacts(t, root, chunkCount)
	}
	cfg := testLancedbFakeWritePolicy()
	policyBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return cfg, policyBytes
}

func buildFakeArtifactsWithText(t *testing.T, root string, text string) {
	t.Helper()
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeIndexArtifactsWithText(t, artifactsDir, text)
	runEmbeddingBuildFake(t, artifactsDir)
}

func writeIndexArtifactsWithText(t *testing.T, artifactsDir string, text string) {
	t.Helper()
	chunksPath := filepath.Join(artifactsDir, "memory-index-chunks.jsonl")
	manifestPath := filepath.Join(artifactsDir, "memory-index-manifest.json")
	writeFile(t, chunksPath, chunkLine("chunk-a", text)+"\n")
	manifest := memoryindex.Manifest{
		GeneratedAt:  "2026-01-01T00:00:00Z",
		ConfigSHA256: "abc",
		Domains:      []string{"mysecondbrain"},
		SourceCount:  1,
		ChunkCount:   1,
		ManifestPath: manifestPath,
		ChunksPath:   chunksPath,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	writeFile(t, manifestPath, string(data)+"\n")
}

func runEmbeddingBuildFake(t *testing.T, _ string) {
	t.Helper()
	cfg := embeddingFakePolicyConfig()
	policyBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := embeddingpolicy.BuildFake(cfg, policyBytes, "artifacts", true); err != nil {
		t.Fatalf("BuildFake() error = %v", err)
	}
}

func embeddingFakePolicyConfig() embeddingpolicy.Config {
	return embeddingpolicy.Config{
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
}

func readFakeRowsJSONL(t *testing.T, path string) []lancedbpolicy.FakeWriteRow {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	var rows []lancedbpolicy.FakeWriteRow
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row lancedbpolicy.FakeWriteRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error = %v", err)
	}
	return rows
}
