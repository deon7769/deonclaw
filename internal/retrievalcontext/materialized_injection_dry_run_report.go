package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type MaterializedInjectionDryRunReportOptions struct {
	DryRunPath       string
	PromptOutputPath string
}

type MaterializedInjectionDryRunReportResult struct {
	Status                    string   `json:"status"`
	WorkerExecution           bool     `json:"worker_execution"`
	PromptChangedInRealRunner bool     `json:"prompt_changed_in_real_runner"`
	PromptSectionRendered     bool     `json:"prompt_section_rendered"`
	ContainsText              bool     `json:"contains_text"`
	PreviewOnly               bool     `json:"preview_only"`
	PromptOutputSHA256        string   `json:"prompt_output_sha256,omitempty"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedInjectionDryRunReport(opts MaterializedInjectionDryRunReportOptions) (MaterializedInjectionDryRunReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"dry-run path", opts.DryRunPath},
		{"prompt output path", opts.PromptOutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedInjectionDryRunReportResult{}, err
		}
	}

	dryRun, dryRunData, err := LoadMaterializedInjectionDryRun(opts.DryRunPath)
	if err != nil {
		return MaterializedInjectionDryRunReportResult{}, err
	}
	if strings.Contains(string(dryRunData), "text_excerpt") || strings.Contains(string(dryRunData), "alpha text") {
		return MaterializedInjectionDryRunReportResult{}, fmt.Errorf("materialized injection dry-run must not contain materialized preview text")
	}

	promptOutputData, err := os.ReadFile(opts.PromptOutputPath)
	if err != nil {
		return MaterializedInjectionDryRunReportResult{}, fmt.Errorf("read prompt output %q: %w", opts.PromptOutputPath, err)
	}
	promptOutputText := string(promptOutputData)
	promptOutputSHA := sha256Hex(promptOutputData)

	result := MaterializedInjectionDryRunReportResult{
		Status:                    lancedbpolicy.StatusOK,
		WorkerExecution:           dryRun.WorkerExecution,
		PromptChangedInRealRunner: dryRun.PromptChangedInRealRunner,
		PromptSectionRendered:     dryRun.PromptSectionRendered,
		ContainsText:              dryRun.ContainsText,
		PreviewOnly:               dryRun.PreviewOnly,
		PromptOutputSHA256:        dryRun.PromptOutputSHA256,
		MaterializedSHA256:        dryRun.MaterializedSHA256,
	}

	var failures []string
	var warnings []string
	warnings = mergeWarnings(warnings, dryRun.Warnings)

	if dryRun.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "dry-run status must be ok or warning")
		failures = append(failures, dryRun.Failures...)
	} else if dryRun.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"dry-run status is warning"})
	} else if dryRun.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("dry-run status %q must be ok or warning", dryRun.Status))
	}
	if dryRun.WorkerExecution {
		failures = append(failures, "dry-run worker_execution must be false")
	}
	if dryRun.PromptChangedInRealRunner {
		failures = append(failures, "dry-run prompt_changed_in_real_runner must be false")
	}
	if !dryRun.PromptSectionRendered {
		failures = append(failures, "dry-run prompt_section_rendered must be true")
	}
	if !dryRun.ContainsText {
		failures = append(failures, "dry-run contains_text must be true")
	}
	if !dryRun.PreviewOnly {
		failures = append(failures, "dry-run preview_only must be true")
	}
	if !dryRun.ConfirmFlagUsed {
		failures = append(failures, "dry-run confirm_flag_used must be true")
	}
	if dryRun.PromptOutputSHA256 == "" {
		failures = append(failures, "dry-run prompt_output_sha256 is required")
	} else if dryRun.PromptOutputSHA256 != promptOutputSHA {
		failures = append(failures, "dry-run prompt_output_sha256 mismatch with prompt output file")
	}
	if !strings.Contains(promptOutputText, MaterializedInjectionDryRunNotice) {
		failures = append(failures, "prompt output missing dry-run notice")
	}
	if !strings.Contains(promptOutputText, "text_excerpt") {
		failures = append(failures, "prompt output missing text_excerpt")
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}
	return result, nil
}

func LoadMaterializedInjectionDryRunReport(path string) (MaterializedInjectionDryRunReportResult, []byte, error) {
	if err := validateRelativeSafePath("dry-run report path", path); err != nil {
		return MaterializedInjectionDryRunReportResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionDryRunReportResult{}, nil, fmt.Errorf("read materialized injection dry-run report %q: %w", path, err)
	}
	return ParseMaterializedInjectionDryRunReportJSON(data)
}

func ParseMaterializedInjectionDryRunReportJSON(data []byte) (MaterializedInjectionDryRunReportResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionDryRunReportResult{}, nil, fmt.Errorf("materialized injection dry-run report must not contain materialized preview text")
	}
	var result MaterializedInjectionDryRunReportResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedInjectionDryRunReportResult{}, nil, fmt.Errorf("parse materialized injection dry-run report json: %w", err)
	}
	return result, data, nil
}

func WriteMaterializedInjectionDryRunReportText(result MaterializedInjectionDryRunReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_dry_run_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"prompt_changed_in_real_runner", fmt.Sprintf("%t", result.PromptChangedInRealRunner)},
		{"prompt_section_rendered", fmt.Sprintf("%t", result.PromptSectionRendered)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"prompt_output_sha256", result.PromptOutputSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
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
	text := fmt.Sprintf("%+v", result)
	if strings.Contains(text, "text_excerpt") || strings.Contains(text, "alpha text") {
		return fmt.Errorf("materialized injection dry-run report text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionDryRunReportJSON(result MaterializedInjectionDryRunReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection dry-run report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection dry-run report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
