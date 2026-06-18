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

type MaterializedInjectionPreflightOptions struct {
	TaskPath                string
	PromptPreviewReportPath string
	OutputPath              string
}

type MaterializedInjectionPreflightResult struct {
	Status                        string   `json:"status"`
	WorkerExecutionAllowed        bool     `json:"worker_execution_allowed"`
	PromptInjectionAllowedNow     bool     `json:"prompt_injection_allowed_now"`
	MaterializedInjectionDeclared bool     `json:"materialized_injection_declared"`
	GovernanceBundleSHA256        string   `json:"governance_bundle_sha256,omitempty"`
	PromptPreviewReportSHA256     string   `json:"prompt_preview_report_sha256,omitempty"`
	MaterializedSHA256            string   `json:"materialized_sha256,omitempty"`
	RequiredFutureFlag            string   `json:"required_future_flag"`
	Warnings                      []string `json:"warnings,omitempty"`
	Failures                      []string `json:"failures,omitempty"`
}

func MaterializedInjectionPreflight(opts MaterializedInjectionPreflightOptions) (MaterializedInjectionPreflightResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"task path", opts.TaskPath},
		{"prompt preview report path", opts.PromptPreviewReportPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedInjectionPreflightResult{}, err
		}
	}

	task, err := tasks.LoadFromFile(opts.TaskPath)
	if err != nil {
		return MaterializedInjectionPreflightResult{}, fmt.Errorf("load task %q: %w", opts.TaskPath, err)
	}

	result := MaterializedInjectionPreflightResult{
		Status:                    lancedbpolicy.StatusOK,
		WorkerExecutionAllowed:    false,
		PromptInjectionAllowedNow: false,
		RequiredFutureFlag:        RequiredFutureInjectFlag,
	}

	var failures []string
	var warnings []string

	taskReport, err := MaterializedInjectionTaskReport(task)
	if err != nil {
		return MaterializedInjectionPreflightResult{}, err
	}
	warnings = mergeWarnings(warnings, taskReport.Warnings)
	if taskReport.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, taskReport.Failures...)
	} else if taskReport.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"task materialized injection report status is warning"})
	}
	if !taskReport.MaterializedInjectionDeclared {
		failures = append(failures, "materialized_injection_declared must be true")
	} else {
		result.MaterializedInjectionDeclared = true
	}
	if taskReport.MaterializedInjectionEnabled {
		failures = append(failures, "materialized_injection_enabled must be false")
	}
	if taskReport.MaterializedInjectionSupportedNow {
		failures = append(failures, "materialized_injection_supported_now must be false")
	}
	if taskReport.PromptPreviewRead {
		failures = append(failures, "prompt_preview_read must be false")
	}
	if taskReport.RunnerPromptChanged {
		failures = append(failures, "runner_prompt_changed must be false")
	}
	if taskReport.GovernanceBundleSHA256 == "" {
		failures = append(failures, "governance_bundle_sha256 is required")
	} else {
		result.GovernanceBundleSHA256 = taskReport.GovernanceBundleSHA256
	}

	spec := task.RetrievalContext.MaterializedInjection
	var bundle InjectionGovernanceBundleResult
	if spec == nil {
		failures = append(failures, "retrieval_context.materialized_injection declaration is required")
	} else if err := tasks.ValidateMaterializedInjectionGovernanceBundlePath(spec.GovernanceBundle); err != nil {
		failures = append(failures, err.Error())
	} else {
		loadedBundle, _, err := LoadInjectionGovernanceBundle(spec.GovernanceBundle)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			bundle = loadedBundle
			if err := ValidateInjectionGovernanceBundleForTaskDeclaration(bundle); err != nil {
				failures = append(failures, err.Error())
			}
			if bundle.ContainsText {
				failures = append(failures, "governance bundle contains_text must be false")
			}
			if bundle.RunnerExecution {
				failures = append(failures, "governance bundle runner_execution must be false")
			}
			if !bundle.InjectionAuthorizedForFuture {
				failures = append(failures, "governance bundle injection_authorized_for_future must be true")
			}
			if bundle.ExecutionSupportedNow {
				failures = append(failures, "governance bundle execution_supported_now must be false")
			}
			if bundle.Status == lancedbpolicy.StatusWarning {
				warnings = mergeWarnings(warnings, []string{"governance bundle status is warning"})
			}
			if bundle.MaterializedSHA256 != "" {
				result.MaterializedSHA256 = bundle.MaterializedSHA256
			}
		}
	}

	previewReport, previewReportData, err := LoadPromptPreviewReport(opts.PromptPreviewReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.PromptPreviewReportSHA256 = sha256Hex(previewReportData)
		if previewReport.Status != lancedbpolicy.StatusOK {
			failures = append(failures, fmt.Sprintf("prompt preview report status %q must be ok", previewReport.Status))
		}
		if previewReport.RunnerExecution {
			failures = append(failures, "prompt preview report runner_execution must be false")
		}
		if !previewReport.PreviewOnly {
			failures = append(failures, "prompt preview report preview_only must be true")
		}
		warnings = mergeWarnings(warnings, previewReport.Warnings)
		if len(previewReport.Failures) > 0 {
			failures = append(failures, previewReport.Failures...)
		}
		if result.MaterializedSHA256 != "" && previewReport.Hashes.MaterializedSHA256 != result.MaterializedSHA256 {
			failures = append(failures, "prompt preview report materialized_sha256 mismatch with governance bundle")
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}

	if err := writeMaterializedInjectionPreflightJSON(opts.OutputPath, result); err != nil {
		return MaterializedInjectionPreflightResult{}, err
	}
	return result, nil
}

func writeMaterializedInjectionPreflightJSON(path string, result MaterializedInjectionPreflightResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized injection preflight output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection preflight json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized injection preflight must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized injection preflight %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedInjectionPreflightText(result MaterializedInjectionPreflightResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_preflight:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"worker_execution_allowed", fmt.Sprintf("%t", result.WorkerExecutionAllowed)},
		{"prompt_injection_allowed_now", fmt.Sprintf("%t", result.PromptInjectionAllowedNow)},
		{"materialized_injection_declared", fmt.Sprintf("%t", result.MaterializedInjectionDeclared)},
		{"governance_bundle_sha256", result.GovernanceBundleSHA256},
		{"prompt_preview_report_sha256", result.PromptPreviewReportSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
		{"required_future_flag", result.RequiredFutureFlag},
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
		return fmt.Errorf("materialized injection preflight text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionPreflightJSON(result MaterializedInjectionPreflightResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection preflight json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection preflight json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
