package lancedbpolicy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	searchSmokeResultName  = "lancedb-search-smoke-result.json"
	searchSmokeSummaryName = "lancedb-search-smoke-summary.md"
	searchSmokeLogName     = "lancedb-search-smoke.log"
)

type SearchSmokeSummary struct {
	QueryMode          string `json:"query_mode"`
	TopK               int    `json:"top_k"`
	ResultCount        int    `json:"result_count"`
	DatabasePath       string `json:"database_path"`
	Table              string `json:"table"`
	RetrievalPerformed bool   `json:"retrieval_performed"`
	RunnerIntegration  bool   `json:"runner_integration"`
	Provider           string `json:"provider,omitempty"`
	Model              string `json:"model,omitempty"`
}

type SearchSmokeResult struct {
	GeneratedAt string             `json:"generated_at"`
	Search      SearchResponse     `json:"search"`
	Summary     SearchSmokeSummary `json:"summary"`
	ResultPath  string             `json:"result_path"`
	SummaryPath string             `json:"summary_path"`
	LogPath     string             `json:"log_path"`
}

type SearchSmokeOptions struct {
	ArtifactsDir       string
	ConfirmSearchSmoke bool
	QueryVector        []float64
	QueryChunkID       string
	TopK               int
	Searcher           LanceDBSearcher
	DoctorReader       LanceDBReader
}

func SearchSmoke(cfg Config, opts SearchSmokeOptions) (SearchSmokeResult, error) {
	if !opts.ConfirmSearchSmoke {
		return SearchSmokeResult{}, fmt.Errorf("--confirm-search-smoke is required")
	}
	if err := Validate(cfg); err != nil {
		return SearchSmokeResult{}, err
	}
	p := cfg.LanceDBPolicy

	if err := validateArtifactsDir(cfg, opts.ArtifactsDir); err != nil {
		return SearchSmokeResult{}, err
	}
	if err := validateDatabasePathUnderArtifacts(opts.ArtifactsDir, p.Database.Path); err != nil {
		return SearchSmokeResult{}, err
	}

	hasVector := len(opts.QueryVector) > 0
	hasChunk := strings.TrimSpace(opts.QueryChunkID) != ""
	if hasVector == hasChunk {
		return SearchSmokeResult{}, fmt.Errorf("exactly one of query_vector or query_chunk_id is required")
	}
	if opts.TopK <= 0 || opts.TopK > MaxSearchTopK {
		return SearchSmokeResult{}, fmt.Errorf("top_k must be > 0 and <= %d", MaxSearchTopK)
	}
	if hasVector && len(opts.QueryVector) != p.Limits.ExpectedDimensions {
		return SearchSmokeResult{}, fmt.Errorf("query_vector length %d != expected_dimensions %d", len(opts.QueryVector), p.Limits.ExpectedDimensions)
	}

	logPath := filepath.Join(opts.ArtifactsDir, searchSmokeLogName)
	logLines := []string{}

	doctor, err := Doctor(cfg, DoctorOptions{Reader: opts.DoctorReader})
	if err != nil {
		return SearchSmokeResult{}, err
	}
	logLines = append(logLines, fmt.Sprintf("%s doctor status=%s row_count=%d", time.Now().UTC().Format(time.RFC3339Nano), doctor.Status, doctor.RowCount))
	if doctor.Status != StatusOK {
		_ = writeSmokeLog(logPath, logLines)
		return SearchSmokeResult{}, fmt.Errorf("lancedb doctor status %q; refusing search smoke", doctor.Status)
	}

	searcher := opts.Searcher
	if searcher == nil {
		preflight := PreflightSearchSmoke()
		logLines = append(logLines, fmt.Sprintf("%s preflight status=%s python=%s", time.Now().UTC().Format(time.RFC3339Nano), preflight.Status, preflight.PythonPath))
		if preflight.Status != StatusOK {
			_ = writeSmokeLog(logPath, append(logLines, preflight.Message))
			return SearchSmokeResult{}, fmt.Errorf("lancedb search preflight failed: %s", preflight.Message)
		}
		searcher = DefaultSearcher
	}

	searchReq := SearchRequest{
		DatabasePath:       p.Database.Path,
		Table:              p.Database.Table,
		VectorColumn:       p.Schema.VectorColumn,
		TextRefColumn:      p.Schema.TextRefColumn,
		TopK:               opts.TopK,
		ExpectedDimensions: p.Limits.ExpectedDimensions,
	}
	if hasVector {
		searchReq.QueryVector = append([]float64(nil), opts.QueryVector...)
	} else {
		searchReq.QueryChunkID = strings.TrimSpace(opts.QueryChunkID)
	}

	logLines = append(logLines, fmt.Sprintf("%s search top_k=%d mode=%s", time.Now().UTC().Format(time.RFC3339Nano), opts.TopK, queryModeLabel(searchReq)))
	searchResp, err := searcher.Search(searchReq)
	if err != nil {
		_ = writeSmokeLog(logPath, append(logLines, err.Error()))
		return SearchSmokeResult{}, err
	}
	logLines = append(logLines, fmt.Sprintf("%s search complete result_count=%d", time.Now().UTC().Format(time.RFC3339Nano), len(searchResp.Results)))

	provider, model := loadWriteSmokeProviderModel(opts.ArtifactsDir)
	summary := SearchSmokeSummary{
		QueryMode:          searchResp.QueryMode,
		TopK:               searchResp.TopK,
		ResultCount:        len(searchResp.Results),
		DatabasePath:       p.Database.Path,
		Table:              p.Database.Table,
		RetrievalPerformed: true,
		RunnerIntegration:  false,
		Provider:           provider,
		Model:              model,
	}

	resultPath := filepath.Join(opts.ArtifactsDir, searchSmokeResultName)
	summaryPath := filepath.Join(opts.ArtifactsDir, searchSmokeSummaryName)

	payload := SearchSmokeResult{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Search:      searchResp,
		Summary:     summary,
		ResultPath:  resultPath,
		SummaryPath: summaryPath,
		LogPath:     logPath,
	}
	resultJSON, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return SearchSmokeResult{}, err
	}
	resultJSON = append(resultJSON, '\n')
	if err := os.WriteFile(resultPath, resultJSON, 0o644); err != nil {
		return SearchSmokeResult{}, fmt.Errorf("write search result %q: %w", resultPath, err)
	}

	if err := os.WriteFile(summaryPath, []byte(renderSearchSmokeSummary(summary)+"\n"), 0o644); err != nil {
		return SearchSmokeResult{}, fmt.Errorf("write search summary %q: %w", summaryPath, err)
	}
	if err := writeSmokeLog(logPath, logLines); err != nil {
		return SearchSmokeResult{}, err
	}

	payload.ResultPath = resultPath
	payload.SummaryPath = summaryPath
	payload.LogPath = logPath
	return payload, nil
}

func queryModeLabel(req SearchRequest) string {
	if len(req.QueryVector) > 0 {
		return "vector"
	}
	return "chunk_id"
}

func loadWriteSmokeProviderModel(artifactsDir string) (string, string) {
	path := filepath.Join(artifactsDir, writeSmokeManifestName)
	manifest, err := LoadWriteSmokeManifest(path)
	if err != nil {
		return "", ""
	}
	return manifest.Provider, manifest.Model
}

func renderSearchSmokeSummary(summary SearchSmokeSummary) string {
	var b strings.Builder
	b.WriteString("# LanceDB search smoke summary\n\n")
	fmt.Fprintf(&b, "- query_mode: %s\n", summary.QueryMode)
	fmt.Fprintf(&b, "- top_k: %d\n", summary.TopK)
	fmt.Fprintf(&b, "- result_count: %d\n", summary.ResultCount)
	fmt.Fprintf(&b, "- database_path: %s\n", summary.DatabasePath)
	fmt.Fprintf(&b, "- table: %s\n", summary.Table)
	fmt.Fprintf(&b, "- retrieval_performed: %t\n", summary.RetrievalPerformed)
	fmt.Fprintf(&b, "- runner_integration: %t\n", summary.RunnerIntegration)
	if summary.Provider != "" {
		fmt.Fprintf(&b, "- provider: %s\n", summary.Provider)
	}
	if summary.Model != "" {
		fmt.Fprintf(&b, "- model: %s\n", summary.Model)
	}
	return b.String()
}
