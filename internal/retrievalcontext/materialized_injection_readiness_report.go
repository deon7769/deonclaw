package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type MaterializedInjectionReadinessReportOptions struct {
	PreflightPath    string
	DryRunPath       string
	DryRunReportPath string
}

type MaterializedInjectionReadinessReportResult struct {
	Status                            string   `json:"status"`
	GovernanceReadyForFutureExecution bool     `json:"governance_ready_for_future_execution"`
	ExecutionAllowedNow               bool     `json:"execution_allowed_now"`
	PromptInjectionAllowedNow         bool     `json:"prompt_injection_allowed_now"`
	WorkerExecution                   bool     `json:"worker_execution"`
	MaterializedSHA256                string   `json:"materialized_sha256,omitempty"`
	PromptOutputSHA256                string   `json:"prompt_output_sha256,omitempty"`
	RequiredFutureFlag                string   `json:"required_future_flag"`
	Warnings                          []string `json:"warnings,omitempty"`
	Failures                          []string `json:"failures,omitempty"`
}

func MaterializedInjectionReadinessReport(opts MaterializedInjectionReadinessReportOptions) (MaterializedInjectionReadinessReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"preflight path", opts.PreflightPath},
		{"dry-run path", opts.DryRunPath},
		{"dry-run report path", opts.DryRunReportPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedInjectionReadinessReportResult{}, err
		}
	}

	preflight, preflightData, err := LoadMaterializedInjectionPreflight(opts.PreflightPath)
	if err != nil {
		return MaterializedInjectionReadinessReportResult{}, err
	}
	if strings.Contains(string(preflightData), "text_excerpt") || strings.Contains(string(preflightData), "alpha text") {
		return MaterializedInjectionReadinessReportResult{}, fmt.Errorf("materialized injection preflight must not contain materialized preview text")
	}

	dryRun, dryRunData, err := LoadMaterializedInjectionDryRun(opts.DryRunPath)
	if err != nil {
		return MaterializedInjectionReadinessReportResult{}, err
	}
	if strings.Contains(string(dryRunData), "text_excerpt") || strings.Contains(string(dryRunData), "alpha text") {
		return MaterializedInjectionReadinessReportResult{}, fmt.Errorf("materialized injection dry-run must not contain materialized preview text")
	}

	dryRunReport, dryRunReportData, err := LoadMaterializedInjectionDryRunReport(opts.DryRunReportPath)
	if err != nil {
		return MaterializedInjectionReadinessReportResult{}, err
	}
	if strings.Contains(string(dryRunReportData), "text_excerpt") || strings.Contains(string(dryRunReportData), "alpha text") {
		return MaterializedInjectionReadinessReportResult{}, fmt.Errorf("materialized injection dry-run report must not contain materialized preview text")
	}

	result := MaterializedInjectionReadinessReportResult{
		Status:                            lancedbpolicy.StatusOK,
		GovernanceReadyForFutureExecution: false,
		ExecutionAllowedNow:               false,
		PromptInjectionAllowedNow:         false,
		WorkerExecution:                   false,
		RequiredFutureFlag:                RequiredFutureInjectFlag,
	}

	var failures []string
	var warnings []string
	warnings = mergeWarnings(warnings, preflight.Warnings, dryRun.Warnings, dryRunReport.Warnings)

	if preflight.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "preflight status must be ok or warning")
		failures = append(failures, preflight.Failures...)
	} else if preflight.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"preflight status is warning"})
	} else if preflight.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("preflight status %q must be ok or warning", preflight.Status))
	}
	if preflight.WorkerExecutionAllowed {
		failures = append(failures, "preflight worker_execution_allowed must be false")
	}
	if preflight.PromptInjectionAllowedNow {
		failures = append(failures, "preflight prompt_injection_allowed_now must be false")
	}

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
	if !dryRun.ConfirmFlagUsed {
		failures = append(failures, "dry-run confirm_flag_used must be true")
	}

	if dryRunReport.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "dry-run report status must be ok or warning")
		failures = append(failures, dryRunReport.Failures...)
	} else if dryRunReport.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"dry-run report status is warning"})
	} else if dryRunReport.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("dry-run report status %q must be ok or warning", dryRunReport.Status))
	}
	if dryRunReport.WorkerExecution {
		failures = append(failures, "dry-run report worker_execution must be false")
	}
	if dryRunReport.PromptChangedInRealRunner {
		failures = append(failures, "dry-run report prompt_changed_in_real_runner must be false")
	}
	if !dryRunReport.PromptSectionRendered {
		failures = append(failures, "dry-run report prompt_section_rendered must be true")
	}

	materializedSHA, materializedFailures := coherentMaterializedSHA256(
		"preflight", preflight.MaterializedSHA256,
		"dry-run", dryRun.MaterializedSHA256,
		"dry-run report", dryRunReport.MaterializedSHA256,
	)
	failures = append(failures, materializedFailures...)
	if materializedSHA != "" {
		result.MaterializedSHA256 = materializedSHA
	}

	promptOutputSHA, promptOutputFailures := coherentPromptOutputSHA256(
		"dry-run", dryRun.PromptOutputSHA256,
		"dry-run report", dryRunReport.PromptOutputSHA256,
	)
	failures = append(failures, promptOutputFailures...)
	if promptOutputSHA != "" {
		result.PromptOutputSHA256 = promptOutputSHA
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
		result.GovernanceReadyForFutureExecution = true
	} else {
		result.GovernanceReadyForFutureExecution = true
	}
	return result, nil
}

func LoadMaterializedInjectionReadinessReport(path string) (MaterializedInjectionReadinessReportResult, []byte, error) {
	if err := validateRelativeSafePath("readiness report path", path); err != nil {
		return MaterializedInjectionReadinessReportResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionReadinessReportResult{}, nil, fmt.Errorf("read materialized injection readiness report %q: %w", path, err)
	}
	return ParseMaterializedInjectionReadinessReportJSON(data)
}

func ParseMaterializedInjectionReadinessReportJSON(data []byte) (MaterializedInjectionReadinessReportResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionReadinessReportResult{}, nil, fmt.Errorf("materialized injection readiness report must not contain materialized preview text")
	}
	var result MaterializedInjectionReadinessReportResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedInjectionReadinessReportResult{}, nil, fmt.Errorf("parse materialized injection readiness report json: %w", err)
	}
	return result, data, nil
}

func coherentMaterializedSHA256(sources ...string) (string, []string) {
	if len(sources)%2 != 0 {
		return "", []string{"internal materialized sha256 source pairing error"}
	}
	var values []struct {
		name string
		hash string
	}
	for i := 0; i < len(sources); i += 2 {
		hash := strings.TrimSpace(sources[i+1])
		if hash == "" {
			continue
		}
		values = append(values, struct {
			name string
			hash string
		}{sources[i], hash})
	}
	if len(values) == 0 {
		return "", nil
	}
	first := values[0]
	var failures []string
	for _, value := range values[1:] {
		if value.hash != first.hash {
			failures = append(failures, fmt.Sprintf("materialized_sha256 mismatch between %s and %s", first.name, value.name))
		}
	}
	return first.hash, failures
}

func coherentPromptOutputSHA256(sources ...string) (string, []string) {
	if len(sources)%2 != 0 {
		return "", []string{"internal prompt output sha256 source pairing error"}
	}
	var values []struct {
		name string
		hash string
	}
	for i := 0; i < len(sources); i += 2 {
		hash := strings.TrimSpace(sources[i+1])
		if hash == "" {
			continue
		}
		values = append(values, struct {
			name string
			hash string
		}{sources[i], hash})
	}
	if len(values) == 0 {
		return "", nil
	}
	first := values[0]
	var failures []string
	for _, value := range values[1:] {
		if value.hash != first.hash {
			failures = append(failures, fmt.Sprintf("prompt_output_sha256 mismatch between %s and %s", first.name, value.name))
		}
	}
	return first.hash, failures
}

func WriteMaterializedInjectionReadinessReportText(result MaterializedInjectionReadinessReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_readiness_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"governance_ready_for_future_execution", fmt.Sprintf("%t", result.GovernanceReadyForFutureExecution)},
		{"execution_allowed_now", fmt.Sprintf("%t", result.ExecutionAllowedNow)},
		{"prompt_injection_allowed_now", fmt.Sprintf("%t", result.PromptInjectionAllowedNow)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"materialized_sha256", result.MaterializedSHA256},
		{"prompt_output_sha256", result.PromptOutputSHA256},
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
		return fmt.Errorf("materialized injection readiness report text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionReadinessReportJSON(result MaterializedInjectionReadinessReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection readiness report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection readiness report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
