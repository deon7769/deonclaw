package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const (
	ConfirmRenderMaterializedContextFlag = "confirm_render_materialized_context"
	PromptPreviewDerivedContextNotice    = "Derived governed retrieval context preview. Not canonical memory."
)

type PromptPreviewOptions struct {
	ExecutionPlanPath                string
	PolicyPath                       string
	MaterializedPath                 string
	OutputPath                       string
	ManifestPath                     string
	ConfirmRenderMaterializedContext bool
}

type PromptPreviewManifest struct {
	ContainsText        bool   `json:"contains_text"`
	PreviewOnly         bool   `json:"preview_only"`
	RunnerExecution     bool   `json:"runner_execution"`
	MaterializedSHA256  string `json:"materialized_sha256"`
	PromptPreviewSHA256 string `json:"prompt_preview_sha256"`
	TotalCharsRendered  int    `json:"total_chars_rendered"`
	ChunkCount          int    `json:"chunk_count"`
	MaxTotalChars       int    `json:"max_total_chars"`
	MaxCharsPerChunk    int    `json:"max_chars_per_chunk"`
	MaxChunks           int    `json:"max_chunks"`
}

type PromptPreviewResult struct {
	Manifest PromptPreviewManifest `json:"manifest"`
}

func PromptPreview(opts PromptPreviewOptions) (PromptPreviewResult, error) {
	if !opts.ConfirmRenderMaterializedContext {
		return PromptPreviewResult{}, fmt.Errorf("--confirm-render-materialized-context is required")
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"execution plan path", opts.ExecutionPlanPath},
		{"policy path", opts.PolicyPath},
		{"materialized path", opts.MaterializedPath},
		{"output path", opts.OutputPath},
		{"manifest path", opts.ManifestPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return PromptPreviewResult{}, err
		}
	}

	executionPlan, err := LoadInjectionExecutionPlan(opts.ExecutionPlanPath)
	if err != nil {
		return PromptPreviewResult{}, err
	}
	if err := validateExecutionPlanForPromptPreview(executionPlan); err != nil {
		return PromptPreviewResult{}, err
	}

	policyData, err := os.ReadFile(opts.PolicyPath)
	if err != nil {
		return PromptPreviewResult{}, fmt.Errorf("read injection policy %q: %w", opts.PolicyPath, err)
	}
	cfg, err := ParseInjectionPolicy(policyData)
	if err != nil {
		return PromptPreviewResult{}, err
	}
	if err := ValidateInjectionPolicy(cfg); err != nil {
		return PromptPreviewResult{}, fmt.Errorf("injection policy invalid: %w", err)
	}
	if err := validateInjectionPolicySafety(cfg.RetrievalInjectionPolicy.Safety); err != nil {
		return PromptPreviewResult{}, err
	}

	materializedData, err := os.ReadFile(opts.MaterializedPath)
	if err != nil {
		return PromptPreviewResult{}, fmt.Errorf("read materialized artifact %q: %w", opts.MaterializedPath, err)
	}
	materializedSHA := sha256Hex(materializedData)
	if executionPlan.MaterializedSHA256 != materializedSHA {
		return PromptPreviewResult{}, fmt.Errorf("execution plan materialized_sha256 mismatch")
	}

	materializedReport, err := MaterializedReportBytes(materializedData)
	if err != nil {
		return PromptPreviewResult{}, err
	}
	if materializedReport.Status == lancedbpolicy.StatusFailed {
		return PromptPreviewResult{}, fmt.Errorf("materialized report status %q", materializedReport.Status)
	}

	limits := cfg.RetrievalInjectionPolicy.Limits
	if materializedReport.TotalCharsIncluded > limits.MaxTotalChars {
		return PromptPreviewResult{}, fmt.Errorf("materialized total_chars_included %d exceeds policy max_total_chars %d", materializedReport.TotalCharsIncluded, limits.MaxTotalChars)
	}
	if materializedReport.IncludedChunkCount > limits.MaxChunks {
		return PromptPreviewResult{}, fmt.Errorf("materialized included_chunk_count %d exceeds policy max_chunks %d", materializedReport.IncludedChunkCount, limits.MaxChunks)
	}

	materialized, err := ParseMaterializeResultJSON(materializedData)
	if err != nil {
		return PromptPreviewResult{}, err
	}

	previewMarkdown, totalCharsRendered, chunkCount, err := renderPromptPreviewMarkdown(cfg.RetrievalInjectionPolicy, materialized)
	if err != nil {
		return PromptPreviewResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return PromptPreviewResult{}, fmt.Errorf("create prompt preview output dir: %w", err)
	}
	if err := os.WriteFile(opts.OutputPath, []byte(previewMarkdown), 0o644); err != nil {
		return PromptPreviewResult{}, fmt.Errorf("write prompt preview %q: %w", opts.OutputPath, err)
	}

	manifest := PromptPreviewManifest{
		ContainsText:        true,
		PreviewOnly:         true,
		RunnerExecution:     false,
		MaterializedSHA256:  materializedSHA,
		PromptPreviewSHA256: sha256Hex([]byte(previewMarkdown)),
		TotalCharsRendered:  totalCharsRendered,
		ChunkCount:          chunkCount,
		MaxTotalChars:       limits.MaxTotalChars,
		MaxCharsPerChunk:    limits.MaxCharsPerChunk,
		MaxChunks:           limits.MaxChunks,
	}
	if err := writePromptPreviewManifest(opts.ManifestPath, manifest); err != nil {
		return PromptPreviewResult{}, err
	}
	return PromptPreviewResult{Manifest: manifest}, nil
}

func ParseMaterializeResultJSON(data []byte) (MaterializeResult, error) {
	var result MaterializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializeResult{}, fmt.Errorf("parse materialized artifact json: %w", err)
	}
	return result, nil
}

func validateExecutionPlanForPromptPreview(plan InjectionExecutionPlanResult) error {
	if !plan.WouldInjectMaterializedContext {
		return fmt.Errorf("execution plan would_inject_materialized_context must be true")
	}
	if plan.WouldExecuteRunner {
		return fmt.Errorf("execution plan would_execute_runner must be false")
	}
	if plan.ExecutionSupportedNow {
		return fmt.Errorf("execution plan execution_supported_now must be false")
	}
	if plan.Reason != InjectionExecutionPlanReasonExecutionPlanOnly {
		return fmt.Errorf("execution plan reason %q must be %q", plan.Reason, InjectionExecutionPlanReasonExecutionPlanOnly)
	}
	return nil
}

func renderPromptPreviewMarkdown(policy InjectionPolicy, materialized MaterializeResult) (string, int, int, error) {
	var b strings.Builder
	title := strings.TrimSpace(policy.Prompt.SectionTitle)
	if title == "" {
		return "", 0, 0, fmt.Errorf("prompt.section_title must not be empty")
	}
	b.WriteString("## ")
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(PromptPreviewDerivedContextNotice)
	b.WriteString("\n\n")

	totalRendered := 0
	chunkCount := 0
	remainingBudget := policy.Limits.MaxTotalChars

	for i, item := range materialized.Items {
		if chunkCount >= policy.Limits.MaxChunks {
			break
		}
		if remainingBudget <= 0 {
			break
		}
		if strings.TrimSpace(item.ChunkID) == "" {
			continue
		}

		excerpt := item.TextExcerpt
		perChunkLimit := policy.Limits.MaxCharsPerChunk
		if perChunkLimit > remainingBudget {
			perChunkLimit = remainingBudget
		}
		excerpt, truncated := truncateRunes(excerpt, perChunkLimit)
		excerptRunes := utf8.RuneCountInString(excerpt)
		if excerptRunes == 0 {
			continue
		}

		b.WriteString("### chunk ")
		b.WriteString(item.ChunkID)
		b.WriteByte('\n')
		if policy.Prompt.IncludeSourcePath && strings.TrimSpace(item.SourcePath) != "" {
			b.WriteString("- source_path: ")
			b.WriteString(item.SourcePath)
			b.WriteByte('\n')
		}
		if policy.Prompt.IncludeHashes {
			if strings.TrimSpace(item.TextSHA256) != "" {
				b.WriteString("- text_sha256: ")
				b.WriteString(item.TextSHA256)
				b.WriteByte('\n')
			}
			if strings.TrimSpace(item.SourceSHA256) != "" {
				b.WriteString("- source_sha256: ")
				b.WriteString(item.SourceSHA256)
				b.WriteByte('\n')
			}
		}
		if item.Truncated || truncated {
			b.WriteString("- truncated: true\n")
		}
		b.WriteString("- text_excerpt: ")
		b.WriteString(excerpt)
		b.WriteByte('\n')
		b.WriteByte('\n')

		totalRendered += excerptRunes
		remainingBudget -= excerptRunes
		chunkCount++
		_ = i
	}

	if chunkCount == 0 {
		return "", 0, 0, fmt.Errorf("no chunk text available for prompt preview")
	}
	if strings.Contains(b.String(), "vector") || strings.Contains(b.String(), "embedding") {
		return "", 0, 0, fmt.Errorf("prompt preview must not include provider or search fields")
	}
	return b.String(), totalRendered, chunkCount, nil
}

func truncateRunes(text string, maxRunes int) (string, bool) {
	if maxRunes <= 0 {
		return "", true
	}
	if utf8.RuneCountInString(text) <= maxRunes {
		return text, false
	}
	runes := []rune(text)
	return string(runes[:maxRunes]), true
}

func writePromptPreviewManifest(path string, manifest PromptPreviewManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create prompt preview manifest dir: %w", err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal prompt preview manifest json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") {
		return fmt.Errorf("prompt preview manifest must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write prompt preview manifest %q: %w", path, err)
	}
	return nil
}
