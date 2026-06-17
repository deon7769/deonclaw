package lancedbpolicy_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type fakeLanceDBReader struct {
	response lancedbpolicy.ReadbackResponse
	err      error
}

func (r *fakeLanceDBReader) Readback(req lancedbpolicy.ReadbackRequest) (lancedbpolicy.ReadbackResponse, error) {
	if r.err != nil {
		return lancedbpolicy.ReadbackResponse{}, r.err
	}
	return r.response, nil
}

func TestPreflightReadbackFailsWhenLanceDBMissing(t *testing.T) {
	result := lancedbpolicy.PreflightReadback()
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
	if result.Message == "" {
		t.Fatal("message is empty, want actionable dependency error")
	}
}

func TestDoctorFailsWhenDependencyMissing(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	cfg := testLancedbWriteSmokePolicy()
	os.MkdirAll(cfg.LanceDBPolicy.Database.Path, 0o755)

	_, err := lancedbpolicy.Doctor(cfg, lancedbpolicy.DoctorOptions{})
	if err == nil || !strings.Contains(err.Error(), "preflight failed") {
		t.Fatalf("Doctor() error = %v, want preflight failure", err)
	}
}

func TestDoctorFailsWhenTableMissing(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	cfg := testLancedbWriteSmokePolicy()
	if err := os.MkdirAll(cfg.LanceDBPolicy.Database.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	reader := &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:      lancedbpolicy.StatusOK,
		TableExists: false,
		RowCount:    0,
	}}

	result, err := lancedbpolicy.Doctor(cfg, lancedbpolicy.DoctorOptions{Reader: reader})
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestDoctorReportsRowCountWithFakeReader(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	cfg := testLancedbWriteSmokePolicy()
	if err := os.MkdirAll(cfg.LanceDBPolicy.Database.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	reader := &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:                 lancedbpolicy.StatusOK,
		TableExists:            true,
		RowCount:               3,
		Columns:                []string{"vector", "chunk_id", "domain", "source_path", "source_sha256", "text_sha256", "embedding_model", "provider", "vector_id"},
		VectorColumnExists:     true,
		TextRefColumnExists:    true,
		MetadataColumnsPresent: cfg.LanceDBPolicy.Schema.MetadataColumns,
		InferredDimensions:     16,
		SampleChunkIDs:         []string{"chunk-a", "chunk-b"},
	}}

	result, err := lancedbpolicy.Doctor(cfg, lancedbpolicy.DoctorOptions{Reader: reader})
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusOK || result.RowCount != 3 {
		t.Fatalf("result = %#v, want ok with 3 rows", result)
	}
}

func TestDoctorFailsWhenDimensionMismatch(t *testing.T) {
	root := t.TempDir()
	chdirTo(t, root)
	cfg := testLancedbWriteSmokePolicy()
	if err := os.MkdirAll(cfg.LanceDBPolicy.Database.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	reader := &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:                 lancedbpolicy.StatusOK,
		TableExists:            true,
		RowCount:               1,
		Columns:                []string{"vector", "chunk_id"},
		VectorColumnExists:     true,
		TextRefColumnExists:    true,
		MetadataColumnsPresent: cfg.LanceDBPolicy.Schema.MetadataColumns,
		InferredDimensions:     32,
	}}

	result, err := lancedbpolicy.Doctor(cfg, lancedbpolicy.DoctorOptions{Reader: reader})
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestReadbackReportFailsWhenManifestLanceDBWrittenFalse(t *testing.T) {
	cfg, manifestPath := setupReadbackReportFixture(t, 1)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = bytes.Replace(manifestData, []byte(`"lancedb_written": true`), []byte(`"lancedb_written": false`), 1)
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	reader := successfulReadbackReader(cfg)

	result, err := lancedbpolicy.ReadbackReport(manifestPath, cfg, lancedbpolicy.ReadbackReportOptions{Reader: reader})
	if err != nil {
		t.Fatalf("ReadbackReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestReadbackReportFailsWhenRetrievalPerformedTrue(t *testing.T) {
	cfg, manifestPath := setupReadbackReportFixture(t, 1)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = bytes.Replace(manifestData, []byte(`"retrieval_performed": false`), []byte(`"retrieval_performed": true`), 1)
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	reader := successfulReadbackReader(cfg)

	result, err := lancedbpolicy.ReadbackReport(manifestPath, cfg, lancedbpolicy.ReadbackReportOptions{Reader: reader})
	if err != nil {
		t.Fatalf("ReadbackReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestReadbackReportFailsWhenRowCountDiffers(t *testing.T) {
	cfg, manifestPath := setupReadbackReportFixture(t, 2)
	reader := &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:                 lancedbpolicy.StatusOK,
		TableExists:            true,
		RowCount:               1,
		VectorColumnExists:     true,
		TextRefColumnExists:    true,
		MetadataColumnsPresent: cfg.LanceDBPolicy.Schema.MetadataColumns,
		InferredDimensions:     16,
	}}

	result, err := lancedbpolicy.ReadbackReport(manifestPath, cfg, lancedbpolicy.ReadbackReportOptions{Reader: reader})
	if err != nil {
		t.Fatalf("ReadbackReport() error = %v", err)
	}
	if result.Status != lancedbpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", result.Status)
	}
}

func TestReadbackReportTextDoesNotPrintFullVectorOrChunkText(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	cfg, manifestPath := setupReadbackReportFixtureWithText(t, marker, 1)
	reader := &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:                 lancedbpolicy.StatusOK,
		TableExists:            true,
		RowCount:               1,
		VectorColumnExists:     true,
		TextRefColumnExists:    true,
		MetadataColumnsPresent: cfg.LanceDBPolicy.Schema.MetadataColumns,
		InferredDimensions:     16,
		SampleChunkIDs:         []string{"chunk-a"},
	}}

	result, err := lancedbpolicy.ReadbackReport(manifestPath, cfg, lancedbpolicy.ReadbackReportOptions{Reader: reader})
	if err != nil {
		t.Fatalf("ReadbackReport() error = %v", err)
	}
	var buf bytes.Buffer
	if err := lancedbpolicy.WriteReadbackReportText(result, &buf); err != nil {
		t.Fatalf("WriteReadbackReportText() error = %v", err)
	}
	output := buf.String()
	if strings.Contains(output, marker) {
		t.Fatal("report text leaked chunk text")
	}
	if strings.Contains(output, `"vector":`) {
		t.Fatal("report text leaked vector field")
	}
}

func TestValidateArtifactsDirRejectsAbsolutePath(t *testing.T) {
	cfg := testLancedbWriteSmokePolicy()
	if err := lancedbpolicy.Validate(cfg); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	_, err := lancedbpolicy.WriteSmoke(cfg, []byte("{}"), lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "/tmp/artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              &recordingLanceDBWriter{},
	})
	if err == nil {
		t.Fatal("WriteSmoke() error = nil, want absolute artifacts dir failure")
	}
}

func setupReadbackReportFixture(t *testing.T, chunkCount int) (lancedbpolicy.Config, string) {
	t.Helper()
	return setupReadbackReportFixtureWithText(t, "# note\n", chunkCount)
}

func successfulReadbackReader(cfg lancedbpolicy.Config) *fakeLanceDBReader {
	return &fakeLanceDBReader{response: lancedbpolicy.ReadbackResponse{
		Status:                 lancedbpolicy.StatusOK,
		TableExists:            true,
		RowCount:               1,
		VectorColumnExists:     true,
		TextRefColumnExists:    true,
		MetadataColumnsPresent: cfg.LanceDBPolicy.Schema.MetadataColumns,
		InferredDimensions:     16,
		SampleChunkIDs:         []string{"chunk-a"},
	}}
}

func setupReadbackReportFixtureWithText(t *testing.T, text string, chunkCount int) (lancedbpolicy.Config, string) {
	t.Helper()
	if chunkCount != 1 {
		return setupReadbackReportFixtureMulti(t, chunkCount)
	}
	cfg, policyBytes := setupLancedbWriteSmokeWithText(t, text, 1)
	return finalizeReadbackFixture(t, cfg, policyBytes, 1)
}

func setupReadbackReportFixtureMulti(t *testing.T, chunkCount int) (lancedbpolicy.Config, string) {
	t.Helper()
	cfg, policyBytes := setupLancedbWriteSmoke(t, chunkCount)
	return finalizeReadbackFixture(t, cfg, policyBytes, chunkCount)
}

func finalizeReadbackFixture(t *testing.T, cfg lancedbpolicy.Config, policyBytes []byte, chunkCount int) (lancedbpolicy.Config, string) {
	t.Helper()
	writer := &recordingLanceDBWriter{}
	result, err := lancedbpolicy.WriteSmoke(cfg, policyBytes, lancedbpolicy.WriteSmokeOptions{
		ArtifactsDir:        "artifacts",
		ConfirmLanceDBWrite: true,
		Writer:              writer,
	})
	if err != nil {
		t.Fatalf("WriteSmoke() error = %v", err)
	}
	if result.Manifest.RowCount != chunkCount {
		t.Fatalf("row_count = %d, want %d", result.Manifest.RowCount, chunkCount)
	}
	return cfg, filepath.Join("artifacts", "lancedb-write-smoke-manifest.json")
}
