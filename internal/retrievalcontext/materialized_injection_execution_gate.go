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
	MaterializedInjectionExecutionGateBlockedReason = "implementation_not_enabled"
)

type MaterializedInjectionExecutionGateOptions struct {
	TaskPath                         string
	ReadinessReportPath              string
	PromptOutputPath                 string
	ConfirmInjectMaterializedContext bool
	OutputPath                       string
}

type MaterializedInjectionExecutionGateResult struct {
	Status                           string   `json:"status"`
	ExecutionGateReady               bool     `json:"execution_gate_ready"`
	ImplementationAllowsExecutionNow bool     `json:"implementation_allows_execution_now"`
	WorkerExecutionAllowed           bool     `json:"worker_execution_allowed"`
	PromptInjectionAllowedNow        bool     `json:"prompt_injection_allowed_now"`
	ConfirmFlagUsed                  bool     `json:"confirm_flag_used"`
	BlockedReason                    string   `json:"blocked_reason"`
	MaterializedSHA256               string   `json:"materialized_sha256,omitempty"`
	PromptOutputSHA256               string   `json:"prompt_output_sha256,omitempty"`
	Warnings                         []string `json:"warnings,omitempty"`
	Failures                         []string `json:"failures,omitempty"`
}

func MaterializedInjectionExecutionGate(opts MaterializedInjectionExecutionGateOptions) (MaterializedInjectionExecutionGateResult, error) {
	if !opts.ConfirmInjectMaterializedContext {
		return MaterializedInjectionExecutionGateResult{}, fmt.Errorf("--%s is required", strings.TrimPrefix(RequiredFutureInjectFlag, "--"))
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"task path", opts.TaskPath},
		{"readiness report path", opts.ReadinessReportPath},
		{"prompt output path", opts.PromptOutputPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedInjectionExecutionGateResult{}, err
		}
	}

	result := MaterializedInjectionExecutionGateResult{
		Status:                           lancedbpolicy.StatusOK,
		ExecutionGateReady:               false,
		ImplementationAllowsExecutionNow: false,
		WorkerExecutionAllowed:           false,
		PromptInjectionAllowedNow:        false,
		ConfirmFlagUsed:                  true,
		BlockedReason:                    MaterializedInjectionExecutionGateBlockedReason,
	}

	var failures []string
	var warnings []string

	task, err := tasks.LoadFromFile(opts.TaskPath)
	if err != nil {
		return MaterializedInjectionExecutionGateResult{}, fmt.Errorf("load task %q: %w", opts.TaskPath, err)
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
		if spec.PromptPreview == "" {
			failures = append(failures, "materialized_injection.prompt_preview is required")
		}
	}

	readiness, readinessData, err := LoadMaterializedInjectionReadinessReport(opts.ReadinessReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if strings.Contains(string(readinessData), "text_excerpt") || strings.Contains(string(readinessData), "alpha text") {
			failures = append(failures, "readiness report must not contain materialized preview text")
		}
		if readiness.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "readiness status must be ok or warning")
			failures = append(failures, readiness.Failures...)
		} else if readiness.Status == lancedbpolicy.StatusWarning {
			warnings = mergeWarnings(warnings, []string{"readiness status is warning"})
			warnings = mergeWarnings(warnings, readiness.Warnings)
		} else if readiness.Status != lancedbpolicy.StatusOK {
			failures = append(failures, fmt.Sprintf("readiness status %q must be ok or warning", readiness.Status))
		}
		if !readiness.GovernanceReadyForFutureExecution {
			failures = append(failures, "readiness governance_ready_for_future_execution must be true")
		}
		if readiness.ExecutionAllowedNow {
			failures = append(failures, "readiness execution_allowed_now must be false")
		}
		if readiness.PromptInjectionAllowedNow {
			failures = append(failures, "readiness prompt_injection_allowed_now must be false")
		}
		if readiness.WorkerExecution {
			failures = append(failures, "readiness worker_execution must be false")
		}
		if readiness.RequiredFutureFlag != RequiredFutureInjectFlag {
			failures = append(failures, fmt.Sprintf("readiness required_future_flag %q must be %q", readiness.RequiredFutureFlag, RequiredFutureInjectFlag))
		}
		if readiness.MaterializedSHA256 == "" {
			failures = append(failures, "readiness materialized_sha256 is required")
		} else {
			result.MaterializedSHA256 = readiness.MaterializedSHA256
		}
		if readiness.PromptOutputSHA256 == "" {
			failures = append(failures, "readiness prompt_output_sha256 is required")
		}
	}

	promptOutputData, err := os.ReadFile(opts.PromptOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read prompt output %q: %v", opts.PromptOutputPath, err))
	} else {
		promptOutputSHA := sha256Hex(promptOutputData)
		if readiness.PromptOutputSHA256 != "" && promptOutputSHA != readiness.PromptOutputSHA256 {
			failures = append(failures, "prompt_output_sha256 mismatch with readiness report")
		}
		result.PromptOutputSHA256 = promptOutputSHA
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ExecutionGateReady = true
		result.Status = lancedbpolicy.StatusOK
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}

	if err := writeMaterializedInjectionExecutionGateJSON(opts.OutputPath, result); err != nil {
		return MaterializedInjectionExecutionGateResult{}, err
	}
	return result, nil
}

func LoadMaterializedInjectionExecutionGate(path string) (MaterializedInjectionExecutionGateResult, []byte, error) {
	if err := validateRelativeSafePath("execution gate path", path); err != nil {
		return MaterializedInjectionExecutionGateResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionExecutionGateResult{}, nil, fmt.Errorf("read materialized injection execution gate %q: %w", path, err)
	}
	return ParseMaterializedInjectionExecutionGateJSON(data)
}

func ParseMaterializedInjectionExecutionGateJSON(data []byte) (MaterializedInjectionExecutionGateResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionExecutionGateResult{}, nil, fmt.Errorf("materialized injection execution gate must not contain materialized preview text")
	}
	var result MaterializedInjectionExecutionGateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedInjectionExecutionGateResult{}, nil, fmt.Errorf("parse materialized injection execution gate json: %w", err)
	}
	return result, data, nil
}

func writeMaterializedInjectionExecutionGateJSON(path string, result MaterializedInjectionExecutionGateResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized injection execution gate output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection execution gate json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized injection execution gate must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized injection execution gate %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedInjectionExecutionGateText(result MaterializedInjectionExecutionGateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_execution_gate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"execution_gate_ready", fmt.Sprintf("%t", result.ExecutionGateReady)},
		{"implementation_allows_execution_now", fmt.Sprintf("%t", result.ImplementationAllowsExecutionNow)},
		{"worker_execution_allowed", fmt.Sprintf("%t", result.WorkerExecutionAllowed)},
		{"prompt_injection_allowed_now", fmt.Sprintf("%t", result.PromptInjectionAllowedNow)},
		{"confirm_flag_used", fmt.Sprintf("%t", result.ConfirmFlagUsed)},
		{"blocked_reason", result.BlockedReason},
		{"materialized_sha256", result.MaterializedSHA256},
		{"prompt_output_sha256", result.PromptOutputSHA256},
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
		return fmt.Errorf("materialized injection execution gate text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionExecutionGateJSON(result MaterializedInjectionExecutionGateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection execution gate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection execution gate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
