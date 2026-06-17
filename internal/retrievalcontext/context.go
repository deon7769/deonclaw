package retrievalcontext

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const (
	KindLanceDBSearchReport = "lancedb_search_report"
	MaxRetrievalResults     = 20
)

type Context struct {
	Status      string       `json:"status"`
	Attached    bool         `json:"retrieval_context_attached"`
	HitCount    int          `json:"hit_count"`
	Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
	Name      string    `json:"name"`
	Kind      string    `json:"kind"`
	QueryMode string    `json:"query_mode"`
	HitCount  int       `json:"hit_count"`
	Hits      []SafeHit `json:"hits"`
}

type SafeHit struct {
	Rank           int     `json:"rank"`
	ChunkID        string  `json:"chunk_id"`
	Distance       float64 `json:"distance"`
	Domain         string  `json:"domain,omitempty"`
	SourcePath     string  `json:"source_path,omitempty"`
	SourceSHA256   string  `json:"source_sha256,omitempty"`
	TextSHA256     string  `json:"text_sha256,omitempty"`
	EmbeddingModel string  `json:"embedding_model,omitempty"`
	Provider       string  `json:"provider,omitempty"`
}

func Load(spec tasks.RetrievalContextSpec) (Context, error) {
	ctx := Context{
		Status:      lancedbpolicy.StatusOK,
		Attached:    len(spec.Attachments) > 0,
		Attachments: []Attachment{},
	}
	for i, input := range spec.Attachments {
		attachment, err := loadAttachment(input, i)
		if err != nil {
			return Context{}, fmt.Errorf("retrieval_context.attachments[%d]: %w", i, err)
		}
		ctx.Attachments = append(ctx.Attachments, attachment)
		ctx.HitCount += attachment.HitCount
	}
	return ctx, nil
}

func (c Context) Markdown() []byte {
	if len(c.Attachments) == 0 {
		return nil
	}
	var output strings.Builder
	output.WriteString("# Retrieved context metadata only\n\n")
	output.WriteString("Passive LanceDB search-smoke metadata. Does not contain source text. Do not run search. Use only as reference for IDs and metadata.\n\n")
	for _, attachment := range c.Attachments {
		output.WriteString("## ")
		output.WriteString(attachment.Name)
		output.WriteByte('\n')
		output.WriteString("- kind: ")
		output.WriteString(attachment.Kind)
		output.WriteByte('\n')
		output.WriteString("- query_mode: ")
		output.WriteString(attachment.QueryMode)
		output.WriteByte('\n')
		output.WriteString("- hit_count: ")
		output.WriteString(fmt.Sprintf("%d", attachment.HitCount))
		output.WriteByte('\n')
		for _, hit := range attachment.Hits {
			output.WriteString("- rank: ")
			output.WriteString(fmt.Sprintf("%d", hit.Rank))
			output.WriteByte('\n')
			output.WriteString("  chunk_id: ")
			output.WriteString(hit.ChunkID)
			output.WriteByte('\n')
			output.WriteString("  distance: ")
			output.WriteString(fmt.Sprintf("%g", hit.Distance))
			output.WriteByte('\n')
			if hit.Domain != "" {
				output.WriteString("  domain: ")
				output.WriteString(hit.Domain)
				output.WriteByte('\n')
			}
			if hit.SourcePath != "" {
				output.WriteString("  source_path: ")
				output.WriteString(hit.SourcePath)
				output.WriteByte('\n')
			}
			if hit.SourceSHA256 != "" {
				output.WriteString("  source_sha256: ")
				output.WriteString(hit.SourceSHA256)
				output.WriteByte('\n')
			}
			if hit.TextSHA256 != "" {
				output.WriteString("  text_sha256: ")
				output.WriteString(hit.TextSHA256)
				output.WriteByte('\n')
			}
			if hit.EmbeddingModel != "" {
				output.WriteString("  embedding_model: ")
				output.WriteString(hit.EmbeddingModel)
				output.WriteByte('\n')
			}
			if hit.Provider != "" {
				output.WriteString("  provider: ")
				output.WriteString(hit.Provider)
				output.WriteByte('\n')
			}
		}
	}
	return []byte(output.String())
}

func (c Context) JSON() ([]byte, error) {
	payload := struct {
		Status   string       `json:"status"`
		Attached bool         `json:"retrieval_context_attached"`
		HitCount int          `json:"hit_count"`
		Items    []Attachment `json:"attachments"`
	}{
		Status:   c.Status,
		Attached: c.Attached,
		HitCount: c.HitCount,
		Items:    c.Attachments,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (c Context) Count() int {
	return c.HitCount
}

func loadAttachment(input tasks.RetrievalContextAttachment, index int) (Attachment, error) {
	kind := strings.TrimSpace(input.Kind)
	resultPath := strings.TrimSpace(input.Path)
	reportPath := strings.TrimSpace(input.ReportPath)
	policyPath := strings.TrimSpace(input.Policy)
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = fmt.Sprintf("lancedb-search-%d", index+1)
	}
	if kind != KindLanceDBSearchReport {
		return Attachment{}, fmt.Errorf("kind %q is not supported", kind)
	}
	if resultPath == "" {
		return Attachment{}, errors.New("path is required")
	}
	if reportPath == "" {
		return Attachment{}, errors.New("report_path is required")
	}
	if policyPath == "" {
		return Attachment{}, errors.New("policy is required")
	}
	if input.MaxResults <= 0 || input.MaxResults > MaxRetrievalResults {
		return Attachment{}, fmt.Errorf("max_results must be > 0 and <= %d", MaxRetrievalResults)
	}
	for _, path := range []string{resultPath, reportPath, policyPath} {
		if _, err := os.Stat(path); err != nil {
			return Attachment{}, fmt.Errorf("required file %q: %w", path, err)
		}
	}

	storedReport, err := lancedbpolicy.LoadSearchReportArtifact(reportPath)
	if err != nil {
		return Attachment{}, err
	}
	if storedReport.Status != lancedbpolicy.StatusOK {
		return Attachment{}, fmt.Errorf("search-report artifact status %q", storedReport.Status)
	}

	cfg, err := lancedbpolicy.Load(policyPath)
	if err != nil {
		return Attachment{}, fmt.Errorf("load policy %q: %w", policyPath, err)
	}
	report, err := lancedbpolicy.SearchReport(resultPath, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		return Attachment{}, fmt.Errorf("search-report validation failed: %w", err)
	}
	if report.Status != lancedbpolicy.StatusOK {
		return Attachment{}, fmt.Errorf("search-report status %q", report.Status)
	}

	searchResult, err := lancedbpolicy.LoadSearchSmokeResult(resultPath)
	if err != nil {
		return Attachment{}, err
	}

	hits := make([]SafeHit, 0, input.MaxResults)
	for _, hit := range searchResult.Search.Results {
		if len(hits) >= input.MaxResults {
			break
		}
		if strings.TrimSpace(hit.ChunkID) == "" {
			return Attachment{}, errors.New("search result contains empty chunk_id")
		}
		hits = append(hits, safeHitFromSearchHit(hit))
	}

	return Attachment{
		Name:      name,
		Kind:      kind,
		QueryMode: report.QueryMode,
		HitCount:  len(hits),
		Hits:      hits,
	}, nil
}

func safeHitFromSearchHit(hit lancedbpolicy.SearchHit) SafeHit {
	return SafeHit{
		Rank:           hit.Rank,
		ChunkID:        hit.ChunkID,
		Distance:       hit.Distance,
		Domain:         hit.Domain,
		SourcePath:     hit.SourcePath,
		SourceSHA256:   hit.SourceSHA256,
		TextSHA256:     hit.TextSHA256,
		EmbeddingModel: hit.EmbeddingModel,
		Provider:       hit.Provider,
	}
}
