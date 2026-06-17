package lancedbpolicy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type recordingLanceDBWriter struct {
	request lancedbpolicy.WriteRequest
}

func (w *recordingLanceDBWriter) Write(req lancedbpolicy.WriteRequest) (lancedbpolicy.WriteResponse, error) {
	w.request = req
	if err := os.MkdirAll(req.DatabasePath, 0o755); err != nil {
		return lancedbpolicy.WriteResponse{}, err
	}
	marker := filepath.Join(req.DatabasePath, ".write-smoke-marker")
	if err := os.WriteFile(marker, []byte("ok"), 0o644); err != nil {
		return lancedbpolicy.WriteResponse{}, err
	}
	return lancedbpolicy.WriteResponse{RowCount: len(req.Rows)}, nil
}

func TestValidateAcceptsWriteSmokePolicy(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	if err := lancedbpolicy.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestWriteSmokeRequiresConfirmFlag(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	writer := &recordingLanceDBWriter{}

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir: "artifacts",
		Writer:       writer,
	})
	if err == nil || !strings.Contains(err.Error(), "--confirm-lancedb-write") {
		t.Fatalf("WriteSmoke() error = %v, want confirm flag requirement", err)
	}
}

func TestWriteSmokeRejectsPlanOnlyMode(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	cfg.LanceDBPolicy.Mode = lancedbpolicy.ModePlanOnly
	writer := &recordingLanceDBWriter{}

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err == nil || !strings.Contains(err.Error(), "write_smoke") {
		t.Fatalf("WriteSmoke() error = %v, want write_smoke mode requirement", err)
	}
}

func TestWriteSmokeRejectsFakeWriteMode(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	cfg.LanceDBPolicy.Mode = lancedbpolicy.ModeFakeWrite
	writer := &recordingLanceDBWriter{}

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err == nil || !strings.Contains(err.Error(), "write_smoke") {
		t.Fatalf("WriteSmoke() error = %v, want write_smoke mode requirement", err)
	}
}

func TestWriteSmokeFailsWhenPlanFailed(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	cfg.LanceDBPolicy.Limits.ExpectedDimensions = 32
	writer := &recordingLanceDBWriter{}

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err == nil || !strings.Contains(err.Error(), "plan status") {
		t.Fatalf("WriteSmoke() error = %v, want plan failure", err)
	}
}

func TestWriteSmokeFailsWhenEmbeddingReportFailed(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	manifestPath := cfg.LanceDBPolicy.Input.EmbeddingManifest
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = []byte(strings.Replace(string(manifestData), `"vector_count": 1`, `"vector_count": 99`, 1))
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	writer := &recordingLanceDBWriter{}

	_, err = lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err == nil || (!strings.Contains(err.Error(), "embedding vector report") && !strings.Contains(err.Error(), "plan status")) {
		t.Fatalf("WriteSmoke() error = %v, want embedding report or plan failure", err)
	}
}

func TestWriteSmokeFailsWhenDatabasePathOutsideArtifactsDir(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	cfg.LanceDBPolicy.Database.Path = "other/lancedb-smoke"
	writer := &recordingLanceDBWriter{}

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err == nil || !strings.Contains(err.Error(), "under artifacts dir") {
		t.Fatalf("WriteSmoke() error = %v, want database path under artifacts failure", err)
	}
}

func TestWriteSmokeFailsWhenDatabaseDirNotEmpty(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	dbPath := cfg.LanceDBPolicy.Database.Path
	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dbPath, "existing.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	writer := &recordingLanceDBWriter{}

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err == nil || !strings.Contains(err.Error(), "not empty") {
		t.Fatalf("WriteSmoke() error = %v, want non-empty database dir failure", err)
	}
}

func TestWriteSmokeFailsWhenDependencyMissing(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)

	_, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
	})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") {
		t.Fatalf("WriteSmoke() error = %v, want preflight failure", err)
	}
}

func TestWriteSmokeGeneratesManifestWithLanceDBWrittenTrue(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 2)
	writer := &recordingLanceDBWriter{}

	result, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err != nil {
		t.Fatalf("WriteSmoke() error = %v", err)
	}
	if !result.Manifest.LanceDBWritten || result.Manifest.RetrievalPerformed || result.Manifest.RunnerIntegration {
		t.Fatalf("manifest flags = %#v", result.Manifest)
	}
	if result.Manifest.RowCount != 2 {
		t.Fatalf("row_count = %d, want 2", result.Manifest.RowCount)
	}
	manifestPath := filepath.Join("artifacts", "lancedb-write-smoke-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), `"lancedb_written": true`) {
		t.Fatalf("manifest = %q, want lancedb_written true", string(data))
	}
}

func TestWriteSmokeRowsSummaryDoesNotContainFullVector(t *testing.T) {
	cfg, policyBytes := setupLancedbWriteSmoke(t, 1)
	writer := &recordingLanceDBWriter{}

	if _, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	}); err != nil {
		t.Fatalf("WriteSmoke() error = %v", err)
	}
	summaryData, err := os.ReadFile(filepath.Join("artifacts", "lancedb-write-smoke-rows-summary.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(summaryData)
	if strings.Contains(content, `"vector":`) {
		t.Fatal("rows summary contains full vector field")
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
		t.Fatal("rows summary leaked full vector array")
	}
}

func TestWriteSmokeRowsSummaryDoesNotContainChunkText(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	cfg, policyBytes := setupLancedbWriteSmokeWithText(t, marker, 1)
	writer := &recordingLanceDBWriter{}

	if _, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	}); err != nil {
		t.Fatalf("WriteSmoke() error = %v", err)
	}
	summaryData, err := os.ReadFile(filepath.Join("artifacts", "lancedb-write-smoke-rows-summary.json"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(summaryData), marker) {
		t.Fatal("rows summary leaked chunk text")
	}
	logData, err := os.ReadFile(filepath.Join("artifacts", "lancedb-write-smoke.log"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(logData), marker) || strings.Contains(string(logData), `"vector":`) {
		t.Fatal("write smoke log leaked sensitive content")
	}
}

func TestPreflightWriteSmokeFailsWhenLanceDBMissing(t *testing.T) {
	result := lancedbpolicy.PreflightWriteSmoke()
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.Message == "" {
		t.Fatal("message is empty, want actionable dependency error")
	}
}

func testLancedbWriteSmokePolicy() lancedbpolicy.Config {
	cfg := testLancedbPolicy()
	cfg.LanceDBPolicy.Mode = lancedbpolicy.ModeWriteSmoke
	cfg.LanceDBPolicy.Database.Path = "artifacts/lancedb-smoke"
	return cfg
}

func setupLancedbWriteSmoke(t *testing.T, chunkCount int) (lancedbpolicy.Config, []byte) {
	t.Helper()
	return setupLancedbWriteSmokeWithText(t, "# note\n", chunkCount)
}

func setupLancedbWriteSmokeWithText(t *testing.T, text string, chunkCount int) (lancedbpolicy.Config, []byte) {
	t.Helper()
	root := t.TempDir()
	chdirTo(t, root)
	if chunkCount == 1 {
		buildFakeArtifactsWithText(t, root, text)
	} else {
		buildFakeArtifacts(t, root, chunkCount)
	}
	cfg := testLancedbWriteSmokePolicy()
	policyBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return cfg, policyBytes
}
