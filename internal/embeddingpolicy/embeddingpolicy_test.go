package embeddingpolicy_test

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
	"github.com/deon7769/deonclaw/internal/memoryindex"
)

func TestValidateAcceptsValidPolicy(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	if err := embeddingpolicy.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateRejectsInvalidProvider(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	cfg.EmbeddingPolicy.Provider = "anthropic"
	if err := embeddingpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want invalid provider failure")
	}
}

func TestValidateRejectsEnvNameValueWithoutLeakingValue(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	secret := "super-secret-token-12345"
	cfg.EmbeddingPolicy.Env.Required = []string{"OPENAI_API_KEY=" + secret}
	err := embeddingpolicy.Validate(cfg)
	if err == nil {
		t.Fatal("Validate() error = nil, want NAME=value failure")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked env value: %v", err)
	}
	if !strings.Contains(err.Error(), "NAME=value") {
		t.Fatalf("error = %v, want NAME=value guidance", err)
	}
}

func TestEnvRequirementsReportMissingAndSetMasked(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")

	missing := embeddingpolicy.EnvRequirements(cfg)
	if len(missing) != 1 || missing[0].State != embeddingpolicy.EnvStateMissing {
		t.Fatalf("requirements = %#v, want missing", missing)
	}

	t.Setenv("OPENAI_API_KEY", "masked-value-not-printed")
	masked := embeddingpolicy.EnvRequirements(cfg)
	if len(masked) != 1 || masked[0].State != embeddingpolicy.EnvStateSetMasked {
		t.Fatalf("requirements = %#v, want set_masked", masked)
	}
}

func TestDoctorDoesNotPrintEnvValue(t *testing.T) {
	root := t.TempDir()
	cfg, _ := writePolicyWithInputs(t, root)
	const secret = "doctor-secret-value-xyz"
	t.Setenv("OPENAI_API_KEY", secret)

	result, err := embeddingpolicy.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	var buf bytes.Buffer
	if err := embeddingpolicy.WriteDoctorText(result, &buf); err != nil {
		t.Fatalf("WriteDoctorText() error = %v", err)
	}
	if embeddingpolicy.DoctorOutputContainsEnvValue(buf.String(), "OPENAI_API_KEY", secret) {
		t.Fatalf("doctor text leaked env value: %q", buf.String())
	}
}

func TestDoctorJSONValid(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir := writePolicyWithInputs(t, root)
	writeIndexArtifacts(t, artifactsDir, 1)

	result, err := embeddingpolicy.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	var buf bytes.Buffer
	if err := embeddingpolicy.WriteDoctorJSON(result, &buf); err != nil {
		t.Fatalf("WriteDoctorJSON() error = %v", err)
	}
	var decoded embeddingpolicy.DoctorResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !decoded.InputChunksExists || !decoded.InputManifestExists {
		t.Fatalf("doctor = %#v, want existing inputs", decoded)
	}
}

func TestDoctorReportsChunkCounts(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir := writePolicyWithInputs(t, root)
	writeIndexArtifacts(t, artifactsDir, 2)

	result, err := embeddingpolicy.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if result.ChunkCount != 2 {
		t.Fatalf("chunk_count = %d, want 2", result.ChunkCount)
	}
	if result.ManifestChunkCount != 2 {
		t.Fatalf("manifest_chunk_count = %d, want 2", result.ManifestChunkCount)
	}
}

func TestPlanCalculatesEstimatedBatches(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir := writePolicyWithInputs(t, root)
	cfg.EmbeddingPolicy.Limits.BatchSize = 3
	cfg.EmbeddingPolicy.Limits.MaxChunks = 100
	writeIndexArtifacts(t, artifactsDir, 7)

	plan, err := embeddingpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.EstimatedBatches != 3 {
		t.Fatalf("estimated_batches = %d, want 3", plan.EstimatedBatches)
	}
	if plan.WouldCallProvider {
		t.Fatal("would_call_provider = true, want false")
	}
	if !plan.DryRunOnly {
		t.Fatal("dry_run_only = false, want true")
	}
}

func TestPlanFailsWhenChunksExceedMaxChunks(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir := writePolicyWithInputs(t, root)
	cfg.EmbeddingPolicy.Limits.MaxChunks = 2
	writeIndexArtifacts(t, artifactsDir, 3)

	plan, err := embeddingpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", plan.Status)
	}
}

func TestPlanFailsWhenInputMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	cfg := testPolicyConfig(root)
	plan, err := embeddingpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", plan.Status)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("warnings is empty, want missing input warning")
	}
}

func TestValidateRejectsAbsoluteOutputPath(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	cfg.EmbeddingPolicy.Output.VectorsPath = "/tmp/vectors.jsonl"
	if err := embeddingpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want absolute output failure")
	}
}

func TestValidateRejectsParentTraversalOutputPath(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	cfg.EmbeddingPolicy.Output.ManifestPath = "../escape-manifest.json"
	if err := embeddingpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want parent traversal failure")
	}
}

func TestValidateRejectsSecretsOutputPath(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	cfg.EmbeddingPolicy.Output.VectorsPath = "artifacts/secrets/vectors.jsonl"
	if err := embeddingpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want secrets output failure")
	}
}

func TestValidateRejectsDotEnvOutputPath(t *testing.T) {
	cfg := testPolicyConfig(t.TempDir())
	cfg.EmbeddingPolicy.Output.VectorsPath = "artifacts/.env/vectors.jsonl"
	if err := embeddingpolicy.Validate(cfg); err == nil {
		t.Fatal("Validate() error = nil, want .env output failure")
	}
}

func TestPlanOKForValidBuild(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir := writePolicyWithInputs(t, root)
	writeIndexArtifacts(t, artifactsDir, 2)

	plan, err := embeddingpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Status != embeddingpolicy.StatusOK {
		t.Fatalf("status = %q, want ok", plan.Status)
	}
	if plan.IndexReportStatus != memoryindex.StatusOK {
		t.Fatalf("index_report_status = %q, want ok", plan.IndexReportStatus)
	}
}

func TestPlanTextDoesNotPrintRawChunkContent(t *testing.T) {
	root := t.TempDir()
	cfg, artifactsDir := writePolicyWithInputs(t, root)
	marker := strings.Repeat("RAW-CHUNK-MARKER-", 30)
	writeIndexArtifactsWithText(t, artifactsDir, marker)

	plan, err := embeddingpolicy.Plan(cfg)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	var buf bytes.Buffer
	if err := embeddingpolicy.WritePlanText(plan, &buf); err != nil {
		t.Fatalf("WritePlanText() error = %v", err)
	}
	if strings.Contains(buf.String(), marker) {
		t.Fatal("plan text leaked raw chunk content")
	}
}

func testPolicyConfig(root string) embeddingpolicy.Config {
	_ = root
	return embeddingpolicy.Config{
		EmbeddingPolicy: embeddingpolicy.Policy{
			Provider:   embeddingpolicy.ProviderOpenAI,
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
			Input: embeddingpolicy.InputConfig{
				ChunksPath:   "artifacts/memory-index-chunks.jsonl",
				ManifestPath: "artifacts/memory-index-manifest.json",
			},
			Output: embeddingpolicy.OutputConfig{
				VectorsPath:  "artifacts/memory-index-vectors.jsonl",
				ManifestPath: "artifacts/memory-embedding-manifest.json",
			},
			Env: embeddingpolicy.EnvConfig{
				Required: []string{"OPENAI_API_KEY"},
			},
			Limits: embeddingpolicy.LimitsConfig{
				MaxChunks:     10000,
				MaxChunkChars: 8000,
				BatchSize:     64,
			},
			Mode: embeddingpolicy.ModeDryRun,
		},
	}
}

func writePolicyWithInputs(t *testing.T, root string) (embeddingpolicy.Config, string) {
	t.Helper()
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	cfg := testPolicyConfig(root)
	artifactsDir := filepath.Join(root, "artifacts")
	if err := os.MkdirAll(artifactsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	return cfg, artifactsDir
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
