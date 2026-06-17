package embeddingpolicy_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
)

func TestValidateAcceptsFakePolicy(t *testing.T) {
	cfg := testFakePolicyConfig()
	if err := embeddingpolicy.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestBuildFakeRequiresConfirmFlag(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 1)

	_, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, false)
	if err == nil || !strings.Contains(err.Error(), "--confirm-fake-vectors") {
		t.Fatalf("BuildFake() error = %v, want confirm flag requirement", err)
	}
}

func TestBuildFakeRejectsOpenAIProvider(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 1)
	cfg.EmbeddingPolicy.Provider = embeddingpolicy.ProviderOpenAI

	_, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err == nil || !strings.Contains(err.Error(), "must be local") {
		t.Fatalf("BuildFake() error = %v, want local provider requirement", err)
	}
}

func TestBuildFakeRejectsDryRunMode(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 1)
	cfg.EmbeddingPolicy.Mode = embeddingpolicy.ModeDryRun

	_, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err == nil || !strings.Contains(err.Error(), "fake_vectors") {
		t.Fatalf("BuildFake() error = %v, want fake_vectors mode requirement", err)
	}
}

func TestBuildFakeGeneratesVectorsAndManifest(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 2)

	result, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err != nil {
		t.Fatalf("BuildFake() error = %v", err)
	}
	if result.Manifest.VectorCount != 2 || result.Manifest.ChunkCount != 2 {
		t.Fatalf("manifest counts = %#v, want 2 chunks and vectors", result.Manifest)
	}
	if !result.Manifest.FakeVectors || result.Manifest.LanceDBWritten || result.Manifest.DryRunOnly {
		t.Fatalf("manifest flags = %#v, want fake only", result.Manifest)
	}
	if _, err := os.Stat(result.Manifest.OutputVectorsPath); err != nil {
		t.Fatalf("vectors path missing: %v", err)
	}
	if _, err := os.Stat(result.Manifest.OutputManifestPath); err != nil {
		t.Fatalf("manifest path missing: %v", err)
	}
}

func TestBuildFakeVectorDimensionsMatchPolicy(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 1)
	cfg.EmbeddingPolicy.Dimensions = 16

	result, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err != nil {
		t.Fatalf("BuildFake() error = %v", err)
	}
	vectors := readVectorsJSONL(t, result.Manifest.OutputVectorsPath)
	if len(vectors) != 1 {
		t.Fatalf("vector count = %d, want 1", len(vectors))
	}
	if len(vectors[0].Vector) != 16 || vectors[0].Dimensions != 16 {
		t.Fatalf("vector = %#v, want 16 dimensions", vectors[0])
	}
}

func TestBuildFakeVectorGenerationIsDeterministic(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 1)

	first, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err != nil {
		t.Fatalf("first BuildFake() error = %v", err)
	}
	vectorsA := readVectorsJSONL(t, first.Manifest.OutputVectorsPath)

	second, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err != nil {
		t.Fatalf("second BuildFake() error = %v", err)
	}
	vectorsB := readVectorsJSONL(t, second.Manifest.OutputVectorsPath)

	if vectorsA[0].VectorSHA256 != vectorsB[0].VectorSHA256 {
		t.Fatalf("vector hashes differ: %q vs %q", vectorsA[0].VectorSHA256, vectorsB[0].VectorSHA256)
	}
}

func TestBuildFakeManifestDoesNotContainChunkText(t *testing.T) {
	root := t.TempDir()
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	cfg, artifactsDir, policyBytes := setupFakeBuildWithText(t, root, marker)

	result, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err != nil {
		t.Fatalf("BuildFake() error = %v", err)
	}
	manifestData, err := os.ReadFile(result.Manifest.OutputManifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(manifestData), marker) {
		t.Fatal("embedding manifest leaked chunk text")
	}
	vectorsData, err := os.ReadFile(result.Manifest.OutputVectorsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(vectorsData), marker) {
		t.Fatal("vectors jsonl leaked chunk text")
	}
}

func TestBuildFakeBlockedByFailedIndexReport(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 1)
	manifestPath := filepath.Join(artifactsDir, "memory-index-manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = []byte(strings.Replace(string(manifestData), `"chunk_count": 1`, `"chunk_count": 99`, 1))
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err = embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err == nil || !strings.Contains(err.Error(), "memory index report failed") {
		t.Fatalf("BuildFake() error = %v, want report failure", err)
	}
}

func TestBuildFakeBlockedWhenMaxChunksExceeded(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir, policyBytes := setupFakeBuild(t, root, 3)
	cfg.EmbeddingPolicy.Limits.MaxChunks = 2

	_, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err == nil || !strings.Contains(err.Error(), "exceeds max_chunks") {
		t.Fatalf("BuildFake() error = %v, want max_chunks failure", err)
	}
}

func testFakePolicyConfig() embeddingpolicy.Config {
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

func setupFakeBuild(t *testing.T, root string, chunkCount int) (embeddingpolicy.Config, string, []byte) {
	t.Helper()
	return setupFakeBuildWithText(t, root, "# note\n", chunkCount)
}

func setupFakeBuildWithText(t *testing.T, root string, text string, chunkCount ...int) (embeddingpolicy.Config, string, []byte) {
	t.Helper()
	chdirTo(t, root)
	cfg := testFakePolicyConfig()
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	count := 1
	if len(chunkCount) > 0 {
		count = chunkCount[0]
	}
	if count == 1 {
		writeIndexArtifactsWithText(t, artifactsDir, text)
	} else {
		writeIndexArtifacts(t, artifactsDir, count)
	}
	policyBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return cfg, "artifacts", policyBytes
}

func readVectorsJSONL(t *testing.T, path string) []embeddingpolicy.VectorRecord {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer file.Close()

	var vectors []embeddingpolicy.VectorRecord
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var vector embeddingpolicy.VectorRecord
		if err := json.Unmarshal([]byte(line), &vector); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		vectors = append(vectors, vector)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error = %v", err)
	}
	return vectors
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
