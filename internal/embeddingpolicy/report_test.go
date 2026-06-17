package embeddingpolicy_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
)

func TestVectorReportOKForValidFakeBuild(t *testing.T) {
	manifestPath, vectorsPath, chunksPath := buildFakeArtifactPaths(t, 2)

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusOK {
		t.Fatalf("status = %q, want ok; invalid=%#v missing=%#v", report.Status, report.InvalidVectors, report.MissingChunkRefs)
	}
	if report.VectorCount != 2 || report.ManifestVectorCount != 2 {
		t.Fatalf("counts = %#v, want 2 vectors", report)
	}
}

func TestVectorReportFailsWhenVectorCountDiffers(t *testing.T) {
	manifestPath, vectorsPath, _ := buildFakeArtifactPaths(t, 1)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	manifestData = bytes.Replace(manifestData, []byte(`"vector_count": 1`), []byte(`"vector_count": 99`), 1)
	if err := os.WriteFile(manifestPath, manifestData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, "")
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestVectorReportFailsWithInvalidVectorSHA256(t *testing.T) {
	manifestPath, vectorsPath, _ := buildFakeArtifactPaths(t, 1)
	vectors := readVectorsJSONL(t, vectorsPath)
	vectors[0].VectorSHA256 = "deadbeef"
	if err := writeVectorsJSONLFile(t, vectorsPath, vectors); err != nil {
		t.Fatalf("writeVectorsJSONLFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, "")
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestVectorReportFailsWhenDimensionsDiffer(t *testing.T) {
	manifestPath, vectorsPath, _ := buildFakeArtifactPaths(t, 1)
	vectors := readVectorsJSONL(t, vectorsPath)
	vectors[0].Dimensions = 99
	if err := writeVectorsJSONLFile(t, vectorsPath, vectors); err != nil {
		t.Fatalf("writeVectorsJSONLFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, "")
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
}

func TestVectorReportDetectsDuplicateVectorIDs(t *testing.T) {
	manifestPath, vectorsPath, _ := buildFakeArtifactPaths(t, 1)
	data, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	data = append(data, data...)
	if err := os.WriteFile(vectorsPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, "")
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.DuplicateVectorIDs) == 0 {
		t.Fatal("duplicate_vector_ids is empty, want duplicates")
	}
}

func TestVectorReportWithChunksDetectsMissingChunkID(t *testing.T) {
	manifestPath, vectorsPath, chunksPath := buildFakeArtifactPaths(t, 1)
	vectors := readVectorsJSONL(t, vectorsPath)
	vectors[0].ChunkID = "missing-chunk-id"
	if err := writeVectorsJSONLFile(t, vectorsPath, vectors); err != nil {
		t.Fatalf("writeVectorsJSONLFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.MissingChunkRefs) == 0 {
		t.Fatal("missing_chunk_refs is empty, want missing chunk")
	}
}

func TestVectorReportWithChunksDetectsSHA256Mismatch(t *testing.T) {
	manifestPath, vectorsPath, chunksPath := buildFakeArtifactPaths(t, 1)
	vectors := readVectorsJSONL(t, vectorsPath)
	vectors[0].TextSHA256 = "deadbeef"
	if err := writeVectorsJSONLFile(t, vectorsPath, vectors); err != nil {
		t.Fatalf("writeVectorsJSONLFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	found := false
	for _, item := range report.MissingChunkRefs {
		if strings.Contains(item.Reason, "text_sha256 mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing_chunk_refs = %#v, want text_sha256 mismatch", report.MissingChunkRefs)
	}
}

func TestVectorReportWithChunksDetectsSourceSHA256Mismatch(t *testing.T) {
	manifestPath, vectorsPath, chunksPath := buildFakeArtifactPaths(t, 1)
	vectors := readVectorsJSONL(t, vectorsPath)
	vectors[0].SourceSHA256 = "deadbeef"
	if err := writeVectorsJSONLFile(t, vectorsPath, vectors); err != nil {
		t.Fatalf("writeVectorsJSONLFile() error = %v", err)
	}

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if report.Status != embeddingpolicy.StatusFailed {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	found := false
	for _, item := range report.MissingChunkRefs {
		if strings.Contains(item.Reason, "source_sha256 mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing_chunk_refs = %#v, want source_sha256 mismatch", report.MissingChunkRefs)
	}
}

func TestVectorReportTextDoesNotPrintVectorOrChunkText(t *testing.T) {
	marker := strings.Repeat("SECRET-CHUNK-TEXT-", 20)
	manifestPath, vectorsPath, chunksPath := buildFakeArtifactPathsWithText(t, marker)

	report, err := embeddingpolicy.Report(manifestPath, vectorsPath, chunksPath)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	var buf bytes.Buffer
	if err := embeddingpolicy.WriteVectorReportText(report, &buf); err != nil {
		t.Fatalf("WriteVectorReportText() error = %v", err)
	}
	output := buf.String()
	if strings.Contains(output, marker) {
		t.Fatal("report text leaked chunk text")
	}
	vectors := readVectorsJSONL(t, vectorsPath)
	if len(vectors) == 0 || len(vectors[0].Vector) == 0 {
		t.Fatal("expected vector data for leak check")
	}
	vectorJSON, err := json.Marshal(vectors[0].Vector)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(output, string(vectorJSON)) {
		t.Fatal("report text leaked full vector array")
	}
}

func buildFakeArtifactPaths(t *testing.T, chunkCount int) (string, string, string) {
	t.Helper()
	return buildFakeArtifactPathsWithText(t, "# note\n", chunkCount)
}

func buildFakeArtifactPathsWithText(t *testing.T, text string, chunkCount ...int) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	count := 1
	if len(chunkCount) > 0 {
		count = chunkCount[0]
	}
	cfg, artifactsDir, policyBytes := setupFakeBuildWithText(t, root, text, count)
	result, err := embeddingpolicy.BuildFake(cfg, policyBytes, artifactsDir, true)
	if err != nil {
		t.Fatalf("BuildFake() error = %v", err)
	}
	return result.Manifest.OutputManifestPath, result.Manifest.OutputVectorsPath, cfg.EmbeddingPolicy.Input.ChunksPath
}

func writeVectorsJSONLFile(t *testing.T, path string, vectors []embeddingpolicy.VectorRecord) error {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, vector := range vectors {
		line, err := json.Marshal(vector)
		if err != nil {
			return err
		}
		if _, err := file.Write(line); err != nil {
			return err
		}
		if _, err := file.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
