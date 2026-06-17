package lancedbpolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/embeddingpolicy"
)

type WriteSmokeRowsSummary struct {
	RowCount        int      `json:"row_count"`
	VectorCount     int      `json:"vector_count"`
	Dimensions      int      `json:"dimensions"`
	MetadataColumns []string `json:"metadata_columns"`
	SampleChunkIDs  []string `json:"sample_chunk_ids"`
}

type WriteSmokeManifest struct {
	GeneratedAt                  string `json:"generated_at"`
	PolicySHA256                 string `json:"policy_sha256"`
	InputEmbeddingManifestSHA256 string `json:"input_embedding_manifest_sha256"`
	InputVectorsSHA256           string `json:"input_vectors_sha256"`
	DatabasePath                 string `json:"database_path"`
	Table                        string `json:"table"`
	RowCount                     int    `json:"row_count"`
	Dimensions                   int    `json:"dimensions"`
	Provider                     string `json:"provider"`
	Model                        string `json:"model"`
	Mode                         string `json:"mode"`
	FakeVectors                  bool   `json:"fake_vectors"`
	LanceDBWritten               bool   `json:"lancedb_written"`
	RetrievalPerformed           bool   `json:"retrieval_performed"`
	RunnerIntegration            bool   `json:"runner_integration"`
}

type WriteSmokeOptions struct {
	ArtifactsDir        string
	ConfirmLanceDBWrite bool
	AllowOverwriteSmoke bool
	Writer              LanceDBWriter
}

type WriteSmokeResult struct {
	Manifest    WriteSmokeManifest    `json:"manifest"`
	RowsSummary WriteSmokeRowsSummary `json:"rows_summary"`
	Plan        PlanResult            `json:"plan"`
	LogPath     string                `json:"log_path"`
}

func WriteSmoke(cfg Config, policyBytes []byte, opts WriteSmokeOptions) (WriteSmokeResult, error) {
	if !opts.ConfirmLanceDBWrite {
		return WriteSmokeResult{}, fmt.Errorf("--confirm-lancedb-write is required")
	}
	if err := Validate(cfg); err != nil {
		return WriteSmokeResult{}, err
	}
	p := cfg.LanceDBPolicy
	if p.Mode != ModeWriteSmoke {
		return WriteSmokeResult{}, fmt.Errorf("lancedb_policy.mode must be write_smoke for real write smoke")
	}
	if err := validateArtifactsDir(cfg, opts.ArtifactsDir); err != nil {
		return WriteSmokeResult{}, err
	}
	if err := validateDatabasePathUnderArtifacts(opts.ArtifactsDir, p.Database.Path); err != nil {
		return WriteSmokeResult{}, err
	}
	if err := validateDatabaseDirectory(p.Database.Path, opts.AllowOverwriteSmoke); err != nil {
		return WriteSmokeResult{}, err
	}

	writer := opts.Writer
	if writer == nil {
		writer = DefaultWriter
	}

	logPath := filepath.Join(opts.ArtifactsDir, writeSmokeLogName)
	logLines := []string{}

	plan, err := Plan(cfg)
	if err != nil {
		return WriteSmokeResult{}, err
	}
	logLines = append(logLines, fmt.Sprintf("%s plan status=%s vector_count=%d", time.Now().UTC().Format(time.RFC3339Nano), plan.Status, plan.VectorCount))
	if plan.Status != StatusOK {
		_ = writeSmokeLog(logPath, logLines)
		return WriteSmokeResult{}, fmt.Errorf("lancedb plan status %q; refusing write smoke", plan.Status)
	}

	chunksPath := strings.TrimSpace(p.Input.ChunksPath)
	vectorReport, err := embeddingpolicy.Report(p.Input.EmbeddingManifest, p.Input.VectorsPath, chunksPath)
	if err != nil {
		return WriteSmokeResult{}, err
	}
	logLines = append(logLines, fmt.Sprintf("%s embedding_report status=%s", time.Now().UTC().Format(time.RFC3339Nano), vectorReport.Status))
	if vectorReport.Status == embeddingpolicy.StatusFailed {
		_ = writeSmokeLog(logPath, logLines)
		return WriteSmokeResult{}, fmt.Errorf("embedding vector report status %q; refusing write smoke", vectorReport.Status)
	}

	preflight := PreflightResult{Status: StatusOK, PythonPath: "skipped"}
	if opts.Writer == nil {
		preflight = PreflightWriteSmoke()
	}
	logLines = append(logLines, fmt.Sprintf("%s preflight status=%s python=%s", time.Now().UTC().Format(time.RFC3339Nano), preflight.Status, preflight.PythonPath))
	if preflight.Status != StatusOK {
		_ = writeSmokeLog(logPath, append(logLines, preflight.Message))
		return WriteSmokeResult{}, fmt.Errorf("lancedb write preflight failed: %s", preflight.Message)
	}

	embeddingManifest, err := embeddingpolicy.LoadEmbeddingManifest(p.Input.EmbeddingManifest)
	if err != nil {
		return WriteSmokeResult{}, err
	}
	vectors, _, err := embeddingpolicy.LoadVectorsJSONL(p.Input.VectorsPath)
	if err != nil {
		return WriteSmokeResult{}, err
	}

	writeRows := make([]WriteRow, 0, len(vectors))
	sampleChunkIDs := make([]string, 0, 5)
	for _, vector := range vectors {
		writeRows = append(writeRows, WriteRow{
			ChunkID:        vector.ChunkID,
			VectorID:       vector.ID,
			Vector:         vector.Vector,
			Domain:         vector.Domain,
			SourcePath:     vector.SourcePath,
			SourceSHA256:   vector.SourceSHA256,
			TextSHA256:     vector.TextSHA256,
			EmbeddingModel: vector.EmbeddingModel,
			Provider:       vector.Provider,
		})
		if len(sampleChunkIDs) < 5 {
			sampleChunkIDs = append(sampleChunkIDs, vector.ChunkID)
		}
	}

	logLines = append(logLines, fmt.Sprintf("%s writing rows=%d database_path=%s table=%s", time.Now().UTC().Format(time.RFC3339Nano), len(writeRows), p.Database.Path, p.Database.Table))
	writeResp, err := writer.Write(WriteRequest{
		DatabasePath:    p.Database.Path,
		Table:           p.Database.Table,
		VectorColumn:    p.Schema.VectorColumn,
		TextRefColumn:   p.Schema.TextRefColumn,
		MetadataColumns: append([]string(nil), p.Schema.MetadataColumns...),
		Rows:            writeRows,
	})
	if err != nil {
		_ = writeSmokeLog(logPath, append(logLines, err.Error()))
		return WriteSmokeResult{}, err
	}
	logLines = append(logLines, fmt.Sprintf("%s write complete row_count=%d", time.Now().UTC().Format(time.RFC3339Nano), writeResp.RowCount))

	embeddingManifestSHA, err := fileSHA256(p.Input.EmbeddingManifest)
	if err != nil {
		return WriteSmokeResult{}, err
	}
	vectorsSHA, err := fileSHA256(p.Input.VectorsPath)
	if err != nil {
		return WriteSmokeResult{}, err
	}

	rowsSummary := WriteSmokeRowsSummary{
		RowCount:        writeResp.RowCount,
		VectorCount:     len(vectors),
		Dimensions:      embeddingManifest.Dimensions,
		MetadataColumns: append([]string(nil), p.Schema.MetadataColumns...),
		SampleChunkIDs:  sampleChunkIDs,
	}
	summaryPath := filepath.Join(opts.ArtifactsDir, writeSmokeRowsSummaryName)
	summaryJSON, err := json.MarshalIndent(rowsSummary, "", "  ")
	if err != nil {
		return WriteSmokeResult{}, err
	}
	summaryJSON = append(summaryJSON, '\n')
	if err := os.WriteFile(summaryPath, summaryJSON, 0o644); err != nil {
		return WriteSmokeResult{}, fmt.Errorf("write rows summary %q: %w", summaryPath, err)
	}

	manifest := WriteSmokeManifest{
		GeneratedAt:                  time.Now().UTC().Format(time.RFC3339Nano),
		PolicySHA256:                 policySHA256(policyBytes),
		InputEmbeddingManifestSHA256: embeddingManifestSHA,
		InputVectorsSHA256:           vectorsSHA,
		DatabasePath:                 p.Database.Path,
		Table:                        p.Database.Table,
		RowCount:                     writeResp.RowCount,
		Dimensions:                   embeddingManifest.Dimensions,
		Provider:                     embeddingManifest.Provider,
		Model:                        embeddingManifest.Model,
		Mode:                         p.Mode,
		FakeVectors:                  embeddingManifest.FakeVectors,
		LanceDBWritten:               true,
		RetrievalPerformed:           false,
		RunnerIntegration:            false,
	}
	manifestPath := filepath.Join(opts.ArtifactsDir, writeSmokeManifestName)
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return WriteSmokeResult{}, err
	}
	manifestJSON = append(manifestJSON, '\n')
	if err := os.WriteFile(manifestPath, manifestJSON, 0o644); err != nil {
		return WriteSmokeResult{}, fmt.Errorf("write smoke manifest %q: %w", manifestPath, err)
	}

	if err := writeSmokeLog(logPath, logLines); err != nil {
		return WriteSmokeResult{}, err
	}

	return WriteSmokeResult{
		Manifest:    manifest,
		RowsSummary: rowsSummary,
		Plan:        plan,
		LogPath:     logPath,
	}, nil
}

func validateDatabasePathUnderArtifacts(artifactsDir, databasePath string) error {
	cleanArtifacts := filepath.Clean(strings.TrimSpace(artifactsDir))
	cleanDB := filepath.Clean(strings.TrimSpace(databasePath))
	rel, err := filepath.Rel(cleanArtifacts, cleanDB)
	if err != nil {
		return fmt.Errorf("database.path %q must be under artifacts dir %q", databasePath, artifactsDir)
	}
	if rel == "." || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("database.path %q must be under artifacts dir %q", databasePath, artifactsDir)
	}
	return nil
}

func validateDatabaseDirectory(databasePath string, allowOverwrite bool) error {
	info, err := os.Stat(databasePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat database.path %q: %w", databasePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("database.path %q exists and is not a directory", databasePath)
	}
	entries, err := os.ReadDir(databasePath)
	if err != nil {
		return fmt.Errorf("read database.path %q: %w", databasePath, err)
	}
	if len(entries) > 0 && !allowOverwrite {
		return fmt.Errorf("database.path %q is not empty; use --allow-overwrite-smoke to replace", databasePath)
	}
	return nil
}

func writeSmokeLog(path string, lines []string) error {
	if len(lines) == 0 {
		return nil
	}
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0o644)
}
