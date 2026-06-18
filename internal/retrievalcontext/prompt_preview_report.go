package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type PromptPreviewReportOptions struct {
	PreviewPath       string
	ManifestPath      string
	ExecutionPlanPath string
}

type PromptPreviewReportHashes struct {
	PromptPreviewSHA256 string `json:"prompt_preview_sha256"`
	MaterializedSHA256  string `json:"materialized_sha256"`
}

type PromptPreviewReportResult struct {
	Status             string                    `json:"status"`
	PreviewOnly        bool                      `json:"preview_only"`
	RunnerExecution    bool                      `json:"runner_execution"`
	TotalCharsRendered int                       `json:"total_chars_rendered"`
	ChunkCount         int                       `json:"chunk_count"`
	Hashes             PromptPreviewReportHashes `json:"hashes"`
	Warnings           []string                  `json:"warnings,omitempty"`
	Failures           []string                  `json:"failures,omitempty"`
}

func LoadPromptPreviewManifest(path string) (PromptPreviewManifest, []byte, error) {
	if err := validateRelativeSafePath("manifest path", path); err != nil {
		return PromptPreviewManifest{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PromptPreviewManifest{}, nil, fmt.Errorf("read prompt preview manifest %q: %w", path, err)
	}
	return ParsePromptPreviewManifestJSON(data)
}

func ParsePromptPreviewManifestJSON(data []byte) (PromptPreviewManifest, []byte, error) {
	if strings.Contains(string(data), `"text_excerpt"`) {
		return PromptPreviewManifest{}, nil, fmt.Errorf("prompt preview manifest must not contain text_excerpt")
	}
	var manifest PromptPreviewManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return PromptPreviewManifest{}, nil, fmt.Errorf("parse prompt preview manifest json: %w", err)
	}
	return manifest, data, nil
}

func PromptPreviewReport(opts PromptPreviewReportOptions) (PromptPreviewReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"preview path", opts.PreviewPath},
		{"manifest path", opts.ManifestPath},
		{"execution plan path", opts.ExecutionPlanPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return PromptPreviewReportResult{}, err
		}
	}

	manifest, _, err := LoadPromptPreviewManifest(opts.ManifestPath)
	if err != nil {
		return PromptPreviewReportResult{}, err
	}

	executionPlan, err := LoadInjectionExecutionPlan(opts.ExecutionPlanPath)
	if err != nil {
		return PromptPreviewReportResult{}, err
	}

	previewData, err := os.ReadFile(opts.PreviewPath)
	if err != nil {
		return PromptPreviewReportResult{}, fmt.Errorf("read prompt preview %q: %w", opts.PreviewPath, err)
	}
	previewText := string(previewData)
	previewSHA := sha256Hex(previewData)

	result := PromptPreviewReportResult{
		Status:             lancedbpolicy.StatusOK,
		PreviewOnly:        manifest.PreviewOnly,
		RunnerExecution:    manifest.RunnerExecution,
		TotalCharsRendered: manifest.TotalCharsRendered,
		ChunkCount:         manifest.ChunkCount,
		Hashes: PromptPreviewReportHashes{
			PromptPreviewSHA256: manifest.PromptPreviewSHA256,
			MaterializedSHA256:  manifest.MaterializedSHA256,
		},
	}

	var failures []string
	if !manifest.ContainsText {
		failures = append(failures, "manifest contains_text must be true")
	}
	if !manifest.PreviewOnly {
		failures = append(failures, "manifest preview_only must be true")
	}
	if manifest.RunnerExecution {
		failures = append(failures, "manifest runner_execution must be false")
	}
	if manifest.PromptPreviewSHA256 != previewSHA {
		failures = append(failures, "manifest prompt_preview_sha256 mismatch")
	}
	if manifest.MaterializedSHA256 != executionPlan.MaterializedSHA256 {
		failures = append(failures, "manifest materialized_sha256 mismatch with execution plan")
	}
	if manifest.TotalCharsRendered > manifest.MaxTotalChars {
		failures = append(failures, fmt.Sprintf("manifest total_chars_rendered %d exceeds max_total_chars %d", manifest.TotalCharsRendered, manifest.MaxTotalChars))
	}
	if manifest.ChunkCount > manifest.MaxChunks {
		failures = append(failures, fmt.Sprintf("manifest chunk_count %d exceeds max_chunks %d", manifest.ChunkCount, manifest.MaxChunks))
	}
	if executionPlan.WouldExecuteRunner {
		failures = append(failures, "execution plan would_execute_runner must be false")
	}
	if executionPlan.ExecutionSupportedNow {
		failures = append(failures, "execution plan execution_supported_now must be false")
	}
	if executionPlan.Reason != InjectionExecutionPlanReasonExecutionPlanOnly {
		failures = append(failures, fmt.Sprintf("execution plan reason %q must be %q", executionPlan.Reason, InjectionExecutionPlanReasonExecutionPlanOnly))
	}
	sectionTitle := strings.TrimSpace(executionPlan.PromptSectionTitle)
	if sectionTitle == "" {
		failures = append(failures, "execution plan prompt_section_title is required")
	} else if !strings.Contains(previewText, sectionTitle) {
		failures = append(failures, "preview missing prompt section title")
	}
	if !strings.Contains(previewText, PromptPreviewDerivedContextNotice) {
		failures = append(failures, "preview missing derived context notice")
	}
	if !strings.Contains(previewText, "text_excerpt") {
		failures = append(failures, "preview missing text_excerpt")
	}
	previewLower := strings.ToLower(previewText)
	if strings.Contains(previewLower, "embedding") || strings.Contains(previewLower, "vector") {
		failures = append(failures, "preview contains forbidden provider or search terms")
	}

	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func WritePromptPreviewReportText(result PromptPreviewReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_prompt_preview_report:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "preview_only: %t\n", result.PreviewOnly); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "runner_execution: %t\n", result.RunnerExecution); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total_chars_rendered: %d\n", result.TotalCharsRendered); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "chunk_count: %d\n", result.ChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nhashes:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "prompt_preview_sha256: %s\n", result.Hashes.PromptPreviewSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "materialized_sha256: %s\n", result.Hashes.MaterializedSHA256); err != nil {
		return err
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	if len(result.Warnings) > 0 {
		if _, err := fmt.Fprintln(out, "\nwarnings:"); err != nil {
			return err
		}
		for _, warning := range result.Warnings {
			if _, err := fmt.Fprintf(out, "- %s\n", warning); err != nil {
				return err
			}
		}
	}
	if strings.Contains(fmt.Sprintf("%+v", result), "text_excerpt") {
		return fmt.Errorf("prompt preview report text must not contain text_excerpt")
	}
	return nil
}

func WritePromptPreviewReportJSON(result PromptPreviewReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal prompt preview report json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") {
		return fmt.Errorf("prompt preview report json must not contain text_excerpt")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
