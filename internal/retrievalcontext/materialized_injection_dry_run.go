package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const (
	ConfirmInjectMaterializedContextFlag = "confirm_inject_materialized_context"
	MaterializedInjectionDryRunNotice    = "Dry-run only — this materialized prompt section was rendered for inspection and was not sent to any worker."
)

type MaterializedInjectionDryRunOptions struct {
	TaskPath                         string
	PreflightPath                    string
	PromptPreviewPath                string
	OutputPath                       string
	PromptOutputPath                 string
	ConfirmInjectMaterializedContext bool
}

type MaterializedInjectionDryRunResult struct {
	Status                    string   `json:"status"`
	WorkerExecution           bool     `json:"worker_execution"`
	PromptChangedInRealRunner bool     `json:"prompt_changed_in_real_runner"`
	PromptSectionRendered     bool     `json:"prompt_section_rendered"`
	PromptOutputSHA256        string   `json:"prompt_output_sha256,omitempty"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	ContainsText              bool     `json:"contains_text"`
	PreviewOnly               bool     `json:"preview_only"`
	ConfirmFlagUsed           bool     `json:"confirm_flag_used"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedInjectionDryRun(opts MaterializedInjectionDryRunOptions) (MaterializedInjectionDryRunResult, error) {
	if !opts.ConfirmInjectMaterializedContext {
		return MaterializedInjectionDryRunResult{}, fmt.Errorf("--%s is required", strings.TrimPrefix(RequiredFutureInjectFlag, "--"))
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"task path", opts.TaskPath},
		{"preflight path", opts.PreflightPath},
		{"prompt preview path", opts.PromptPreviewPath},
		{"output path", opts.OutputPath},
		{"prompt output path", opts.PromptOutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedInjectionDryRunResult{}, err
		}
	}

	result := MaterializedInjectionDryRunResult{
		Status:                    lancedbpolicy.StatusOK,
		WorkerExecution:           false,
		PromptChangedInRealRunner: false,
		PromptSectionRendered:     false,
		ContainsText:              true,
		PreviewOnly:               true,
		ConfirmFlagUsed:           true,
	}

	var failures []string
	var warnings []string

	task, err := tasks.LoadFromFile(opts.TaskPath)
	if err != nil {
		return MaterializedInjectionDryRunResult{}, fmt.Errorf("load task %q: %w", opts.TaskPath, err)
	}
	if err := tasks.Validate(task); err != nil {
		failures = append(failures, err.Error())
	}

	spec := task.RetrievalContext.MaterializedInjection
	if spec == nil {
		failures = append(failures, "retrieval_context.materialized_injection declaration is required")
	} else {
		if spec.Enabled {
			failures = append(failures, "materialized_injection.enabled must be false")
		}
		if spec.PromptPreview != opts.PromptPreviewPath {
			failures = append(failures, fmt.Sprintf("prompt preview path %q must match task declaration %q", opts.PromptPreviewPath, spec.PromptPreview))
		}
	}

	preflight, _, err := LoadMaterializedInjectionPreflight(opts.PreflightPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		warnings = mergeWarnings(warnings, preflight.Warnings)
		if preflight.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "materialized injection preflight status must be ok or warning")
			failures = append(failures, preflight.Failures...)
		} else if preflight.Status == lancedbpolicy.StatusWarning {
			warnings = mergeWarnings(warnings, []string{"materialized injection preflight status is warning"})
		}
		if preflight.WorkerExecutionAllowed {
			failures = append(failures, "preflight worker_execution_allowed must be false")
		}
		if preflight.PromptInjectionAllowedNow {
			failures = append(failures, "preflight prompt_injection_allowed_now must be false")
		}
		if preflight.RequiredFutureFlag != RequiredFutureInjectFlag {
			failures = append(failures, fmt.Sprintf("preflight required_future_flag %q must be %q", preflight.RequiredFutureFlag, RequiredFutureInjectFlag))
		}
		if !preflight.MaterializedInjectionDeclared {
			failures = append(failures, "preflight materialized_injection_declared must be true")
		}
		if preflight.MaterializedSHA256 == "" {
			failures = append(failures, "preflight materialized_sha256 is required")
		} else {
			result.MaterializedSHA256 = preflight.MaterializedSHA256
		}
	}

	var promptSection string
	if len(failures) == 0 {
		previewData, err := os.ReadFile(opts.PromptPreviewPath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("read prompt preview %q: %v", opts.PromptPreviewPath, err))
		} else {
			promptSection, err = renderMaterializedInjectionDryRunPromptSection(string(previewData))
			if err != nil {
				failures = append(failures, err.Error())
			}
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}

	if len(failures) == 0 {
		if err := os.MkdirAll(filepath.Dir(opts.PromptOutputPath), 0o755); err != nil {
			return MaterializedInjectionDryRunResult{}, fmt.Errorf("create prompt output dir: %w", err)
		}
		if err := os.WriteFile(opts.PromptOutputPath, []byte(promptSection), 0o644); err != nil {
			return MaterializedInjectionDryRunResult{}, fmt.Errorf("write prompt output %q: %w", opts.PromptOutputPath, err)
		}
		result.PromptSectionRendered = true
		result.PromptOutputSHA256 = sha256Hex([]byte(promptSection))
	}

	if err := writeMaterializedInjectionDryRunJSON(opts.OutputPath, result); err != nil {
		return MaterializedInjectionDryRunResult{}, err
	}
	return result, nil
}

func LoadMaterializedInjectionDryRun(path string) (MaterializedInjectionDryRunResult, []byte, error) {
	if err := validateRelativeSafePath("dry-run path", path); err != nil {
		return MaterializedInjectionDryRunResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionDryRunResult{}, nil, fmt.Errorf("read materialized injection dry-run %q: %w", path, err)
	}
	return ParseMaterializedInjectionDryRunJSON(data)
}

func ParseMaterializedInjectionDryRunJSON(data []byte) (MaterializedInjectionDryRunResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionDryRunResult{}, nil, fmt.Errorf("materialized injection dry-run must not contain materialized preview text")
	}
	var result MaterializedInjectionDryRunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedInjectionDryRunResult{}, nil, fmt.Errorf("parse materialized injection dry-run json: %w", err)
	}
	return result, data, nil
}

func renderMaterializedInjectionDryRunPromptSection(previewMarkdown string) (string, error) {
	section := strings.TrimSpace(previewMarkdown)
	if section == "" {
		return "", fmt.Errorf("prompt preview is empty")
	}
	if !strings.Contains(section, "text_excerpt:") {
		return "", fmt.Errorf("prompt preview must contain rendered text_excerpt lines")
	}
	var b strings.Builder
	b.WriteString("## Dry-run only — materialized injection prompt section\n\n")
	b.WriteString(MaterializedInjectionDryRunNotice)
	b.WriteString("\n\n")
	b.WriteString(section)
	b.WriteByte('\n')
	return b.String(), nil
}

func writeMaterializedInjectionDryRunJSON(path string, result MaterializedInjectionDryRunResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized injection dry-run output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection dry-run json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized injection dry-run must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized injection dry-run %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedInjectionDryRunText(result MaterializedInjectionDryRunResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_dry_run:"); err != nil {
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
		{"prompt_output_sha256", result.PromptOutputSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"confirm_flag_used", fmt.Sprintf("%t", result.ConfirmFlagUsed)},
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
		return fmt.Errorf("materialized injection dry-run text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionDryRunJSON(result MaterializedInjectionDryRunResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection dry-run json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection dry-run json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
