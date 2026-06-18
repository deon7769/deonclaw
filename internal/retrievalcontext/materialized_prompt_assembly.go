package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const (
	MaterializedPromptAssemblyDryRunNotice  = "Dry-run only — assembled prompt was not sent to any worker."
	MaterializedPromptAssemblyBaseHeader    = "## Base worker prompt fixture (dry-run)"
	MaterializedPromptAssemblySectionHeader = "## Materialized retrieval context prompt section (dry-run)"
)

type MaterializedPromptAssemblyOptions struct {
	ExecutionGatePath                string
	BasePromptFixturePath            string
	PromptOutputPath                 string
	OutputPath                       string
	AssembledOutputPath              string
	ConfirmInjectMaterializedContext bool
}

type MaterializedPromptAssemblyResult struct {
	Status                    string   `json:"status"`
	AssembledPromptRendered   bool     `json:"assembled_prompt_rendered"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToWorker              bool     `json:"sent_to_worker"`
	PromptChangedInRealRunner bool     `json:"prompt_changed_in_real_runner"`
	ContainsText              bool     `json:"contains_text"`
	PreviewOnly               bool     `json:"preview_only"`
	ConfirmFlagUsed           bool     `json:"confirm_flag_used"`
	BasePromptSHA256          string   `json:"base_prompt_sha256,omitempty"`
	PromptOutputSHA256        string   `json:"prompt_output_sha256,omitempty"`
	AssembledOutputSHA256     string   `json:"assembled_output_sha256,omitempty"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	BlockedReason             string   `json:"blocked_reason"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedPromptAssembly(opts MaterializedPromptAssemblyOptions) (MaterializedPromptAssemblyResult, error) {
	if !opts.ConfirmInjectMaterializedContext {
		return MaterializedPromptAssemblyResult{}, fmt.Errorf("--%s is required", strings.TrimPrefix(RequiredFutureInjectFlag, "--"))
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"execution gate path", opts.ExecutionGatePath},
		{"base prompt fixture path", opts.BasePromptFixturePath},
		{"prompt output path", opts.PromptOutputPath},
		{"output path", opts.OutputPath},
		{"assembled output path", opts.AssembledOutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedPromptAssemblyResult{}, err
		}
	}

	result := MaterializedPromptAssemblyResult{
		Status:                    lancedbpolicy.StatusOK,
		WorkerExecution:           false,
		SentToWorker:              false,
		PromptChangedInRealRunner: false,
		ContainsText:              true,
		PreviewOnly:               true,
		ConfirmFlagUsed:           true,
		BlockedReason:             MaterializedInjectionExecutionGateBlockedReason,
	}

	var failures []string
	var warnings []string

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
		if !gate.ConfirmFlagUsed {
			failures = append(failures, "execution gate confirm_flag_used must be true")
		}
		if gate.BlockedReason != MaterializedInjectionExecutionGateBlockedReason {
			failures = append(failures, fmt.Sprintf("execution gate blocked_reason %q must be %q", gate.BlockedReason, MaterializedInjectionExecutionGateBlockedReason))
		}
		if gate.PromptOutputSHA256 == "" {
			failures = append(failures, "execution gate prompt_output_sha256 is required")
		}
		if gate.MaterializedSHA256 != "" {
			result.MaterializedSHA256 = gate.MaterializedSHA256
		}
	}

	basePromptData, err := os.ReadFile(opts.BasePromptFixturePath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read base prompt fixture %q: %v", opts.BasePromptFixturePath, err))
	} else {
		result.BasePromptSHA256 = sha256Hex(basePromptData)
	}

	promptOutputData, err := os.ReadFile(opts.PromptOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read prompt output %q: %v", opts.PromptOutputPath, err))
	} else {
		promptOutputSHA := sha256Hex(promptOutputData)
		if gate.PromptOutputSHA256 != "" && gate.PromptOutputSHA256 != promptOutputSHA {
			failures = append(failures, "prompt_output_sha256 mismatch with execution gate")
		}
		result.PromptOutputSHA256 = promptOutputSHA
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}

	if len(failures) == 0 {
		assembled, err := renderMaterializedPromptAssembly(string(basePromptData), string(promptOutputData))
		if err != nil {
			return MaterializedPromptAssemblyResult{}, err
		}
		if err := os.MkdirAll(filepath.Dir(opts.AssembledOutputPath), 0o755); err != nil {
			return MaterializedPromptAssemblyResult{}, fmt.Errorf("create assembled output dir: %w", err)
		}
		if err := os.WriteFile(opts.AssembledOutputPath, []byte(assembled), 0o644); err != nil {
			return MaterializedPromptAssemblyResult{}, fmt.Errorf("write assembled output %q: %w", opts.AssembledOutputPath, err)
		}
		result.AssembledPromptRendered = true
		result.AssembledOutputSHA256 = sha256Hex([]byte(assembled))
	}

	if err := writeMaterializedPromptAssemblyJSON(opts.OutputPath, result); err != nil {
		return MaterializedPromptAssemblyResult{}, err
	}
	return result, nil
}

func LoadMaterializedPromptAssembly(path string) (MaterializedPromptAssemblyResult, []byte, error) {
	if err := validateRelativeSafePath("assembly dry-run path", path); err != nil {
		return MaterializedPromptAssemblyResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedPromptAssemblyResult{}, nil, fmt.Errorf("read materialized prompt assembly %q: %w", path, err)
	}
	return ParseMaterializedPromptAssemblyJSON(data)
}

func ParseMaterializedPromptAssemblyJSON(data []byte) (MaterializedPromptAssemblyResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedPromptAssemblyResult{}, nil, fmt.Errorf("materialized prompt assembly must not contain materialized preview text")
	}
	var result MaterializedPromptAssemblyResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedPromptAssemblyResult{}, nil, fmt.Errorf("parse materialized prompt assembly json: %w", err)
	}
	return result, data, nil
}

func renderMaterializedPromptAssembly(basePrompt, promptOutput string) (string, error) {
	base := strings.TrimSpace(basePrompt)
	section := strings.TrimSpace(promptOutput)
	if base == "" {
		return "", fmt.Errorf("base prompt fixture is empty")
	}
	if section == "" {
		return "", fmt.Errorf("prompt output is empty")
	}
	if !strings.Contains(section, "text_excerpt") {
		return "", fmt.Errorf("prompt output must contain rendered text_excerpt content")
	}
	var b strings.Builder
	b.WriteString("# Dry-run assembled worker prompt\n\n")
	b.WriteString(MaterializedPromptAssemblyDryRunNotice)
	b.WriteString("\n\n")
	b.WriteString(MaterializedPromptAssemblyBaseHeader)
	b.WriteString("\n\n")
	b.WriteString(base)
	b.WriteString("\n\n---\n\n")
	b.WriteString(MaterializedPromptAssemblySectionHeader)
	b.WriteString("\n\n")
	b.WriteString(section)
	b.WriteByte('\n')
	return b.String(), nil
}

func writeMaterializedPromptAssemblyJSON(path string, result MaterializedPromptAssemblyResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized prompt assembly output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized prompt assembly json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized prompt assembly must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized prompt assembly %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedPromptAssemblyText(result MaterializedPromptAssemblyResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_prompt_assembly_dry_run:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"assembled_prompt_rendered", fmt.Sprintf("%t", result.AssembledPromptRendered)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_worker", fmt.Sprintf("%t", result.SentToWorker)},
		{"prompt_changed_in_real_runner", fmt.Sprintf("%t", result.PromptChangedInRealRunner)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"confirm_flag_used", fmt.Sprintf("%t", result.ConfirmFlagUsed)},
		{"base_prompt_sha256", result.BasePromptSHA256},
		{"prompt_output_sha256", result.PromptOutputSHA256},
		{"assembled_output_sha256", result.AssembledOutputSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
		{"blocked_reason", result.BlockedReason},
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
		return fmt.Errorf("materialized prompt assembly text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedPromptAssemblyJSON(result MaterializedPromptAssemblyResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized prompt assembly json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized prompt assembly json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
