package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/tasks"
)

type MaterializedInjectionTaskReportResult struct {
	Status                            string   `json:"status"`
	TaskID                            string   `json:"task_id"`
	MaterializedInjectionDeclared     bool     `json:"materialized_injection_declared"`
	MaterializedInjectionEnabled      bool     `json:"materialized_injection_enabled"`
	MaterializedInjectionSupportedNow bool     `json:"materialized_injection_supported_now"`
	GovernanceBundleSHA256            string   `json:"governance_bundle_sha256,omitempty"`
	PromptPreviewPathDeclared         bool     `json:"prompt_preview_path_declared"`
	PromptPreviewRead                 bool     `json:"prompt_preview_read"`
	RunnerPromptChanged               bool     `json:"runner_prompt_changed"`
	Warnings                          []string `json:"warnings,omitempty"`
	Failures                          []string `json:"failures,omitempty"`
}

func MaterializedInjectionTaskReport(task *tasks.Task) (MaterializedInjectionTaskReportResult, error) {
	result := MaterializedInjectionTaskReportResult{
		Status:                            lancedbpolicy.StatusOK,
		MaterializedInjectionSupportedNow: false,
		PromptPreviewRead:                 false,
		RunnerPromptChanged:               false,
	}
	if task == nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{"task is nil"}
		return result, nil
	}
	result.TaskID = task.ID

	if err := tasks.Validate(task); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = append(result.Failures, err.Error())
		return result, nil
	}

	spec := task.RetrievalContext.MaterializedInjection
	if spec == nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = append(result.Failures, "retrieval_context.materialized_injection declaration is required")
		return result, nil
	}

	result.MaterializedInjectionDeclared = true
	result.MaterializedInjectionEnabled = spec.Enabled
	result.PromptPreviewPathDeclared = strings.TrimSpace(spec.PromptPreview) != ""

	var failures []string
	var warnings []string
	var bundleSHA string

	if spec.Enabled {
		failures = append(failures, "materialized_injection_enabled must be false")
	}
	if result.MaterializedInjectionSupportedNow {
		failures = append(failures, "materialized_injection_supported_now must be false")
	}
	if err := tasks.ValidateMaterializedInjectionGovernanceBundlePath(spec.GovernanceBundle); err != nil {
		failures = append(failures, err.Error())
	} else {
		loadedBundle, bundleData, err := LoadInjectionGovernanceBundle(spec.GovernanceBundle)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			bundleSHA = sha256Hex(bundleData)
			if err := ValidateInjectionGovernanceBundleForTaskDeclaration(loadedBundle); err != nil {
				failures = append(failures, err.Error())
			}
			if loadedBundle.ContainsText {
				failures = append(failures, "governance bundle contains_text must be false")
			}
			if loadedBundle.RunnerExecution {
				failures = append(failures, "governance bundle runner_execution must be false")
			}
			if !loadedBundle.InjectionAuthorizedForFuture {
				failures = append(failures, "governance bundle injection_authorized_for_future must be true")
			}
			if loadedBundle.ExecutionSupportedNow {
				failures = append(failures, "governance bundle execution_supported_now must be false")
			}
			if loadedBundle.Status == lancedbpolicy.StatusWarning {
				warnings = append(warnings, "governance bundle status is warning")
			}
		}
	}
	if err := tasks.ValidateMaterializedInjectionPromptPreviewPath(spec.PromptPreview); err != nil {
		failures = append(failures, err.Error())
	}

	validation, err := ValidateTaskMaterializedInjection(spec)
	if err != nil {
		failures = append(failures, err.Error())
	}
	if validation.GovernanceBundleSHA256 != "" {
		if bundleSHA != "" && validation.GovernanceBundleSHA256 != bundleSHA {
			failures = append(failures, "governance_bundle_sha256 mismatch")
		}
		bundleSHA = validation.GovernanceBundleSHA256
	}
	if bundleSHA == "" {
		failures = append(failures, "governance_bundle_sha256 is required")
	} else {
		result.GovernanceBundleSHA256 = bundleSHA
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

func WriteMaterializedInjectionTaskReportText(result MaterializedInjectionTaskReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "task_materialized_injection_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"task_id", result.TaskID},
		{"materialized_injection_declared", fmt.Sprintf("%t", result.MaterializedInjectionDeclared)},
		{"materialized_injection_enabled", fmt.Sprintf("%t", result.MaterializedInjectionEnabled)},
		{"materialized_injection_supported_now", fmt.Sprintf("%t", result.MaterializedInjectionSupportedNow)},
		{"governance_bundle_sha256", result.GovernanceBundleSHA256},
		{"prompt_preview_path_declared", fmt.Sprintf("%t", result.PromptPreviewPathDeclared)},
		{"prompt_preview_read", fmt.Sprintf("%t", result.PromptPreviewRead)},
		{"runner_prompt_changed", fmt.Sprintf("%t", result.RunnerPromptChanged)},
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
		return fmt.Errorf("materialized injection task report text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionTaskReportJSON(result MaterializedInjectionTaskReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection task report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection task report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
