package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type MaterializedPromptAssemblyReportOptions struct {
	AssemblyDryRunPath  string
	AssembledOutputPath string
}

type MaterializedPromptAssemblyReportResult struct {
	Status                    string   `json:"status"`
	AssembledPromptValidated  bool     `json:"assembled_prompt_validated"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToWorker              bool     `json:"sent_to_worker"`
	PromptChangedInRealRunner bool     `json:"prompt_changed_in_real_runner"`
	ContainsText              bool     `json:"contains_text"`
	PreviewOnly               bool     `json:"preview_only"`
	AssembledOutputSHA256     string   `json:"assembled_output_sha256,omitempty"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedPromptAssemblyReport(opts MaterializedPromptAssemblyReportOptions) (MaterializedPromptAssemblyReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"assembly dry-run path", opts.AssemblyDryRunPath},
		{"assembled output path", opts.AssembledOutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedPromptAssemblyReportResult{}, err
		}
	}

	assembly, assemblyData, err := LoadMaterializedPromptAssembly(opts.AssemblyDryRunPath)
	if err != nil {
		return MaterializedPromptAssemblyReportResult{}, err
	}
	if strings.Contains(string(assemblyData), "text_excerpt") || strings.Contains(string(assemblyData), "alpha text") {
		return MaterializedPromptAssemblyReportResult{}, fmt.Errorf("materialized prompt assembly must not contain materialized preview text")
	}

	result := MaterializedPromptAssemblyReportResult{
		Status:                    lancedbpolicy.StatusOK,
		WorkerExecution:           false,
		SentToWorker:              false,
		PromptChangedInRealRunner: false,
		ContainsText:              true,
		PreviewOnly:               true,
		MaterializedSHA256:        assembly.MaterializedSHA256,
	}

	var failures []string
	var warnings []string
	warnings = mergeWarnings(warnings, assembly.Warnings)

	if assembly.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "assembly status must be ok or warning")
		failures = append(failures, assembly.Failures...)
	} else if assembly.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"assembly status is warning"})
	} else if assembly.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("assembly status %q must be ok or warning", assembly.Status))
	}
	if !assembly.AssembledPromptRendered {
		failures = append(failures, "assembly assembled_prompt_rendered must be true")
	}
	if assembly.WorkerExecution {
		failures = append(failures, "assembly worker_execution must be false")
	}
	if assembly.SentToWorker {
		failures = append(failures, "assembly sent_to_worker must be false")
	}
	if assembly.PromptChangedInRealRunner {
		failures = append(failures, "assembly prompt_changed_in_real_runner must be false")
	}
	if !assembly.ContainsText {
		failures = append(failures, "assembly contains_text must be true")
	}
	if !assembly.PreviewOnly {
		failures = append(failures, "assembly preview_only must be true")
	}
	if !assembly.ConfirmFlagUsed {
		failures = append(failures, "assembly confirm_flag_used must be true")
	}
	if assembly.BlockedReason != MaterializedInjectionExecutionGateBlockedReason {
		failures = append(failures, fmt.Sprintf("assembly blocked_reason %q must be %q", assembly.BlockedReason, MaterializedInjectionExecutionGateBlockedReason))
	}
	if assembly.AssembledOutputSHA256 == "" {
		failures = append(failures, "assembly assembled_output_sha256 is required")
	}

	assembledData, err := os.ReadFile(opts.AssembledOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read assembled output %q: %v", opts.AssembledOutputPath, err))
	} else {
		assembledSHA := sha256Hex(assembledData)
		if assembly.AssembledOutputSHA256 != "" && assembledSHA != assembly.AssembledOutputSHA256 {
			failures = append(failures, "assembled_output_sha256 mismatch with assembly dry-run")
		}
		result.AssembledOutputSHA256 = assembledSHA
		assembledText := string(assembledData)
		if !strings.Contains(assembledText, MaterializedPromptAssemblyDryRunNotice) {
			failures = append(failures, "assembled output missing dry-run notice")
		}
		if !strings.Contains(assembledText, "text_excerpt") {
			failures = append(failures, "assembled output missing text_excerpt")
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.AssembledPromptValidated = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func WriteMaterializedPromptAssemblyReportText(result MaterializedPromptAssemblyReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_prompt_assembly_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"assembled_prompt_validated", fmt.Sprintf("%t", result.AssembledPromptValidated)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_worker", fmt.Sprintf("%t", result.SentToWorker)},
		{"prompt_changed_in_real_runner", fmt.Sprintf("%t", result.PromptChangedInRealRunner)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"assembled_output_sha256", result.AssembledOutputSHA256},
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
		return fmt.Errorf("materialized prompt assembly report text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedPromptAssemblyReportJSON(result MaterializedPromptAssemblyReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized prompt assembly report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized prompt assembly report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
