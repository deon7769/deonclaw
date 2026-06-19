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

type MaterializedInjectionProviderRunPlanOptions struct {
	TaskPath                         string
	EnableConfigPath                 string
	ExecutionGatePath                string
	AssemblyReportPath               string
	AssembledOutputPath              string
	ConfirmInjectMaterializedContext bool
	OutputPath                       string
}

type MaterializedInjectionProviderRunPlanResult struct {
	Status                    string   `json:"status"`
	ProviderRunPlanReady      bool     `json:"provider_run_plan_ready"`
	ProviderCallAllowedNow    bool     `json:"provider_call_allowed_now"`
	WorkerExecutionAllowedNow bool     `json:"worker_execution_allowed_now"`
	PromptInjectionAllowedNow bool     `json:"prompt_injection_allowed_now"`
	WouldUseAssembledPrompt   bool     `json:"would_use_assembled_prompt"`
	AssembledPromptValidated  bool     `json:"assembled_prompt_validated"`
	SentToProvider            bool     `json:"sent_to_provider"`
	ConfirmFlagUsed           bool     `json:"confirm_flag_used"`
	BlockedReason             string   `json:"blocked_reason"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256     string   `json:"assembled_output_sha256,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedInjectionProviderRunPlan(opts MaterializedInjectionProviderRunPlanOptions) (MaterializedInjectionProviderRunPlanResult, error) {
	if !opts.ConfirmInjectMaterializedContext {
		return MaterializedInjectionProviderRunPlanResult{}, fmt.Errorf("--%s is required", strings.TrimPrefix(RequiredFutureInjectFlag, "--"))
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"task path", opts.TaskPath},
		{"enable config path", opts.EnableConfigPath},
		{"execution gate path", opts.ExecutionGatePath},
		{"assembly report path", opts.AssemblyReportPath},
		{"assembled output path", opts.AssembledOutputPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedInjectionProviderRunPlanResult{}, err
		}
	}

	result := MaterializedInjectionProviderRunPlanResult{
		Status:                    lancedbpolicy.StatusOK,
		ProviderRunPlanReady:      false,
		ProviderCallAllowedNow:    false,
		WorkerExecutionAllowedNow: false,
		PromptInjectionAllowedNow: false,
		WouldUseAssembledPrompt:   false,
		AssembledPromptValidated:  false,
		SentToProvider:            false,
		ConfirmFlagUsed:           true,
		BlockedReason:             MaterializedInjectionExecutionEnableBlockedReason,
	}

	var failures []string
	var warnings []string

	task, err := tasks.LoadFromFile(opts.TaskPath)
	if err != nil {
		return MaterializedInjectionProviderRunPlanResult{}, fmt.Errorf("load task %q: %w", opts.TaskPath, err)
	}
	if err := tasks.Validate(task); err != nil {
		failures = append(failures, err.Error())
	}
	spec := task.RetrievalContext.MaterializedInjection
	if spec == nil {
		failures = append(failures, "retrieval_context.materialized_injection declaration is required")
	} else if spec.Enabled {
		failures = append(failures, "materialized_injection.enabled must be false")
	}

	enableCfg, err := LoadMaterializedInjectionExecutionEnable(opts.EnableConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		enableResult, err := MaterializedInjectionExecutionEnableValidate(enableCfg)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			if enableResult.Status == lancedbpolicy.StatusFailed {
				failures = append(failures, "enable config validation failed")
				failures = append(failures, enableResult.Failures...)
			}
			if enableResult.Enabled {
				failures = append(failures, "enable config enabled must be false")
			}
			if enableResult.AllowProviderCall {
				failures = append(failures, "enable config allow_provider_call must be false")
			}
			if enableResult.AllowWorkerExecution {
				failures = append(failures, "enable config allow_worker_execution must be false")
			}
			if enableResult.AllowPromptInjection {
				failures = append(failures, "enable config allow_prompt_injection must be false")
			}
		}
	}

	gate, gateData, err := LoadMaterializedInjectionExecutionGate(opts.ExecutionGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if strings.Contains(string(gateData), "text_excerpt") || strings.Contains(string(gateData), "alpha text") {
			failures = append(failures, "execution gate must not contain materialized preview text")
		}
		if gate.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "execution gate status must be ok or warning")
			failures = append(failures, gate.Failures...)
		} else if gate.Status == lancedbpolicy.StatusWarning {
			warnings = mergeWarnings(warnings, []string{"execution gate status is warning"})
			warnings = mergeWarnings(warnings, gate.Warnings)
		} else if gate.Status != lancedbpolicy.StatusOK {
			failures = append(failures, fmt.Sprintf("execution gate status %q must be ok or warning", gate.Status))
		}
		if !gate.ExecutionGateReady {
			failures = append(failures, "execution gate execution_gate_ready must be true")
		}
		if gate.ImplementationAllowsExecutionNow {
			failures = append(failures, "execution gate implementation_allows_execution_now must be false")
		}
		if gate.WorkerExecutionAllowed {
			failures = append(failures, "execution gate worker_execution_allowed must be false")
		}
		if gate.PromptInjectionAllowedNow {
			failures = append(failures, "execution gate prompt_injection_allowed_now must be false")
		}
		if gate.BlockedReason != MaterializedInjectionExecutionEnableBlockedReason {
			failures = append(failures, fmt.Sprintf("execution gate blocked_reason %q must be %q", gate.BlockedReason, MaterializedInjectionExecutionEnableBlockedReason))
		}
		if gate.MaterializedSHA256 != "" {
			result.MaterializedSHA256 = gate.MaterializedSHA256
		}
	}

	assemblyReport, assemblyReportData, err := LoadMaterializedPromptAssemblyReport(opts.AssemblyReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if strings.Contains(string(assemblyReportData), "text_excerpt") || strings.Contains(string(assemblyReportData), "alpha text") {
			failures = append(failures, "assembly report must not contain materialized preview text")
		}
		if assemblyReport.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "assembly report status must be ok or warning")
			failures = append(failures, assemblyReport.Failures...)
		} else if assemblyReport.Status == lancedbpolicy.StatusWarning {
			warnings = mergeWarnings(warnings, []string{"assembly report status is warning"})
			warnings = mergeWarnings(warnings, assemblyReport.Warnings)
		} else if assemblyReport.Status != lancedbpolicy.StatusOK {
			failures = append(failures, fmt.Sprintf("assembly report status %q must be ok or warning", assemblyReport.Status))
		}
		if !assemblyReport.AssembledPromptValidated {
			failures = append(failures, "assembly report assembled_prompt_validated must be true")
		}
		if assemblyReport.WorkerExecution {
			failures = append(failures, "assembly report worker_execution must be false")
		}
		if assemblyReport.SentToWorker {
			failures = append(failures, "assembly report sent_to_worker must be false")
		}
		if assemblyReport.PromptChangedInRealRunner {
			failures = append(failures, "assembly report prompt_changed_in_real_runner must be false")
		}
		if assemblyReport.AssembledOutputSHA256 == "" {
			failures = append(failures, "assembly report assembled_output_sha256 is required")
		}
		if assemblyReport.MaterializedSHA256 != "" {
			if result.MaterializedSHA256 != "" && assemblyReport.MaterializedSHA256 != result.MaterializedSHA256 {
				failures = append(failures, "materialized_sha256 mismatch between execution gate and assembly report")
			}
			if result.MaterializedSHA256 == "" {
				result.MaterializedSHA256 = assemblyReport.MaterializedSHA256
			}
		}
	}

	assembledData, err := os.ReadFile(opts.AssembledOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read assembled output %q: %v", opts.AssembledOutputPath, err))
	} else {
		assembledSHA := sha256Hex(assembledData)
		if assemblyReport.AssembledOutputSHA256 != "" && assembledSHA != assemblyReport.AssembledOutputSHA256 {
			failures = append(failures, "assembled_output_sha256 mismatch with assembly report")
		}
		result.AssembledOutputSHA256 = assembledSHA
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ProviderRunPlanReady = true
		result.WouldUseAssembledPrompt = true
		result.AssembledPromptValidated = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}

	if err := writeMaterializedInjectionProviderRunPlanJSON(opts.OutputPath, result); err != nil {
		return MaterializedInjectionProviderRunPlanResult{}, err
	}
	return result, nil
}

func writeMaterializedInjectionProviderRunPlanJSON(path string, result MaterializedInjectionProviderRunPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized injection provider run plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection provider run plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized injection provider run plan must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized injection provider run plan %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedInjectionProviderRunPlanText(result MaterializedInjectionProviderRunPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_provider_run_plan:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"provider_run_plan_ready", fmt.Sprintf("%t", result.ProviderRunPlanReady)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"worker_execution_allowed_now", fmt.Sprintf("%t", result.WorkerExecutionAllowedNow)},
		{"prompt_injection_allowed_now", fmt.Sprintf("%t", result.PromptInjectionAllowedNow)},
		{"would_use_assembled_prompt", fmt.Sprintf("%t", result.WouldUseAssembledPrompt)},
		{"assembled_prompt_validated", fmt.Sprintf("%t", result.AssembledPromptValidated)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"confirm_flag_used", fmt.Sprintf("%t", result.ConfirmFlagUsed)},
		{"blocked_reason", result.BlockedReason},
		{"materialized_sha256", result.MaterializedSHA256},
		{"assembled_output_sha256", result.AssembledOutputSHA256},
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
		return fmt.Errorf("materialized injection provider run plan text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionProviderRunPlanJSON(result MaterializedInjectionProviderRunPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection provider run plan json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection provider run plan json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
