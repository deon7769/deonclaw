package memoryindex_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/memoryindex"
)

func TestDoctorTextOutput(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	var buf bytes.Buffer
	result, err := memoryindex.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if err := memoryindex.WriteDoctorText(result, &buf); err != nil {
		t.Fatalf("WriteDoctorText() error = %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "memory_index_doctor:") {
		t.Fatalf("output missing header: %q", output)
	}
	if !strings.Contains(output, "status: ok") && !strings.Contains(output, "status: warning") {
		t.Fatalf("output missing status: %q", output)
	}
	if !strings.Contains(output, "source_count: 1") {
		t.Fatalf("output missing source_count: %q", output)
	}
}

func TestDoctorJSONValid(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	result, err := memoryindex.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	var buf bytes.Buffer
	if err := memoryindex.WriteDoctorJSON(result, &buf); err != nil {
		t.Fatalf("WriteDoctorJSON() error = %v", err)
	}
	var decoded memoryindex.DoctorResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if decoded.SourceCount != 1 {
		t.Fatalf("source_count = %d, want 1", decoded.SourceCount)
	}
}

func TestDoctorCountsSkippedSymlinks(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	realPath := filepath.Join(root, "memory", "mysecondbrain", "real.md")
	writeFile(t, realPath, "# real\n")
	linkPath := filepath.Join(root, "memory", "mysecondbrain", "link.md")
	symlinkOrSkip(t, realPath, linkPath)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "inside.md"), "# inside\n")

	result, err := memoryindex.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if result.Skipped.Symlinks != 1 {
		t.Fatalf("skipped_symlinks = %d, want 1", result.Skipped.Symlinks)
	}
	if result.SourceCount != 2 {
		t.Fatalf("source_count = %d, want 2", result.SourceCount)
	}
}

func TestDoctorCountsSkippedSecretPaths(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "live.md"), "# live\n")
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "secrets", "token.md"), "# secret\n")

	result, err := memoryindex.Doctor(cfg)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if result.Skipped.SecretPaths == 0 {
		t.Fatalf("skipped_secret_paths = %d, want > 0", result.Skipped.SecretPaths)
	}
}

func TestReportOKForValidBuild(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	artifactsDir := filepath.Join(root, "artifacts")
	buildResult, err := memoryindex.Build(loaded, artifactsDir, configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	report, err := memoryindex.Report(buildResult.Manifest.ManifestPath, buildResult.Manifest.ChunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != memoryindex.StatusOK {
		t.Fatalf("status = %q, want ok", report.Status)
	}
	if report.ChunkCount != buildResult.Manifest.ChunkCount {
		t.Fatalf("chunk_count = %d, want %d", report.ChunkCount, buildResult.Manifest.ChunkCount)
	}
}

func TestReportFailsWhenChunkCountDiffers(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	buildResult, err := memoryindex.Build(loaded, filepath.Join(root, "artifacts"), configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	manifestPath := buildResult.Manifest.ManifestPath
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = bytes.Replace(manifestData, []byte(`"chunk_count": 1`), []byte(`"chunk_count": 99`), 1)
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := memoryindex.Report(manifestPath, buildResult.Manifest.ChunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != memoryindex.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestReportFailsWithInvalidTextSHA256(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	buildResult, err := memoryindex.Build(loaded, filepath.Join(root, "artifacts"), configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	chunksPath := buildResult.Manifest.ChunksPath
	chunks := readChunksJSONL(t, chunksPath)
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	chunks[0].TextSHA256 = "deadbeef"
	if err := writeChunksJSONL(t, chunksPath, chunks); err != nil {
		t.Fatalf("writeChunksJSONL() error = %v", err)
	}

	report, err := memoryindex.Report(buildResult.Manifest.ManifestPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != memoryindex.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	foundMismatch := false
	for _, item := range report.InvalidChunks {
		if strings.Contains(item.Reason, "text_sha256 mismatch") {
			foundMismatch = true
			break
		}
	}
	if !foundMismatch {
		t.Fatalf("invalid_chunks = %#v, want text_sha256 mismatch", report.InvalidChunks)
	}
}

func TestReportDetectsDuplicateChunkID(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), "# note\n")

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	buildResult, err := memoryindex.Build(loaded, filepath.Join(root, "artifacts"), configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	chunksPath := buildResult.Manifest.ChunksPath
	chunksData, err := os.ReadFile(chunksPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chunksData = append(chunksData, chunksData...)
	if err := os.WriteFile(chunksPath, chunksData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := memoryindex.Report(buildResult.Manifest.ManifestPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != memoryindex.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.DuplicateChunkIDs) == 0 {
		t.Fatal("duplicate_chunk_ids is empty, want duplicates")
	}
}

func TestReportTextDoesNotPrintRawChunkContent(t *testing.T) {
	root := t.TempDir()
	cfg := writeTestIndexLayout(t, root)
	cfgPath := writeTestIndexConfig(t, root, cfg)
	secretMarker := strings.Repeat("RAW-CHUNK-CONTENT-MARKER-", 40)
	writeFile(t, filepath.Join(root, "memory", "mysecondbrain", "note.md"), secretMarker)

	loaded, err := memoryindex.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	configBytes, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	buildResult, err := memoryindex.Build(loaded, filepath.Join(root, "artifacts"), configBytes)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	report, err := memoryindex.Report(buildResult.Manifest.ManifestPath, buildResult.Manifest.ChunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	var buf bytes.Buffer
	if err := memoryindex.WriteReportText(report, &buf); err != nil {
		t.Fatalf("WriteReportText() error = %v", err)
	}
	if strings.Contains(buf.String(), secretMarker) {
		t.Fatal("report text leaked raw chunk content")
	}
}
