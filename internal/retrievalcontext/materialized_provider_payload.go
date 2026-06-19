package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const (
	ProviderPayloadDryRunNotice           = "Provider payload dry-run only — not sent to provider."
	ProviderPayloadAssembledSectionHeader = "## Assembled prompt"
)

type MaterializedProviderPayloadOptions struct {
	DispatchConfigPath               string
	ProviderRunPlanPath              string
	AssembledOutputPath              string
	OutputPath                       string
	PayloadOutputPath                string
	ConfirmInjectMaterializedContext bool
}

type MaterializedProviderPayloadResult struct {
	Status                    string   `json:"status"`
	ProviderPayloadRendered   bool     `json:"provider_payload_rendered"`
	Provider                  string   `json:"provider"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToProvider            bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner bool     `json:"prompt_injection_real_runner"`
	ContainsText              bool     `json:"contains_text"`
	PreviewOnly               bool     `json:"preview_only"`
	ConfirmFlagUsed           bool     `json:"confirm_flag_used"`
	BlockedReason             string   `json:"blocked_reason"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256     string   `json:"assembled_output_sha256,omitempty"`
	ProviderPayloadSHA256     string   `json:"provider_payload_sha256,omitempty"`
	PayloadBytes              int      `json:"payload_bytes"`
	MaxPayloadBytes           int      `json:"max_payload_bytes"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedProviderPayload(opts MaterializedProviderPayloadOptions) (MaterializedProviderPayloadResult, error) {
	if !opts.ConfirmInjectMaterializedContext {
		return MaterializedProviderPayloadResult{}, fmt.Errorf("--%s is required", strings.TrimPrefix(RequiredFutureInjectFlag, "--"))
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"dispatch config path", opts.DispatchConfigPath},
		{"provider run plan path", opts.ProviderRunPlanPath},
		{"assembled output path", opts.AssembledOutputPath},
		{"output path", opts.OutputPath},
		{"payload output path", opts.PayloadOutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedProviderPayloadResult{}, err
		}
	}

	result := MaterializedProviderPayloadResult{
		Status:                    lancedbpolicy.StatusOK,
		Provider:                  MaterializedProviderDispatchProvider,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		SentToProvider:            false,
		PromptInjectionRealRunner: false,
		ContainsText:              true,
		PreviewOnly:               true,
		ConfirmFlagUsed:           true,
		BlockedReason:             MaterializedProviderDispatchBlockedReason,
	}

	var failures []string
	var warnings []string
	var maxTotalChars int
	var maxPayloadBytes int

	dispatchCfg, err := LoadMaterializedProviderDispatch(opts.DispatchConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		dispatchResult, err := MaterializedProviderDispatchValidate(dispatchCfg)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			if dispatchResult.Status == lancedbpolicy.StatusFailed {
				failures = append(failures, "dispatch config validation failed")
				failures = append(failures, dispatchResult.Failures...)
			}
			if dispatchResult.Enabled {
				failures = append(failures, "dispatch config enabled must be false")
			}
			if dispatchResult.Provider != MaterializedProviderDispatchProvider {
				failures = append(failures, fmt.Sprintf("dispatch config provider %q must be %q", dispatchResult.Provider, MaterializedProviderDispatchProvider))
			}
			if dispatchResult.AllowProviderCall {
				failures = append(failures, "dispatch config allow_provider_call must be false")
			}
			if dispatchResult.AllowNetwork {
				failures = append(failures, "dispatch config allow_network must be false")
			}
			maxTotalChars = dispatchResult.MaxTotalChars
			maxPayloadBytes = dispatchResult.MaxPayloadBytes
			result.MaxPayloadBytes = maxPayloadBytes
		}
	}

	runPlan, runPlanData, err := LoadMaterializedInjectionProviderRunPlan(opts.ProviderRunPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if strings.Contains(string(runPlanData), "text_excerpt") || strings.Contains(string(runPlanData), "alpha text") {
			failures = append(failures, "provider run plan must not contain materialized preview text")
		}
		if runPlan.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "provider run plan status must be ok or warning")
			failures = append(failures, runPlan.Failures...)
		} else if runPlan.Status == lancedbpolicy.StatusWarning {
			warnings = mergeWarnings(warnings, []string{"provider run plan status is warning"})
			warnings = mergeWarnings(warnings, runPlan.Warnings)
		} else if runPlan.Status != lancedbpolicy.StatusOK {
			failures = append(failures, fmt.Sprintf("provider run plan status %q must be ok or warning", runPlan.Status))
		}
		if !runPlan.ProviderRunPlanReady {
			failures = append(failures, "provider run plan provider_run_plan_ready must be true")
		}
		if runPlan.ProviderCallAllowedNow {
			failures = append(failures, "provider run plan provider_call_allowed_now must be false")
		}
		if runPlan.WorkerExecutionAllowedNow {
			failures = append(failures, "provider run plan worker_execution_allowed_now must be false")
		}
		if runPlan.PromptInjectionAllowedNow {
			failures = append(failures, "provider run plan prompt_injection_allowed_now must be false")
		}
		if !runPlan.WouldUseAssembledPrompt {
			failures = append(failures, "provider run plan would_use_assembled_prompt must be true")
		}
		if !runPlan.AssembledPromptValidated {
			failures = append(failures, "provider run plan assembled_prompt_validated must be true")
		}
		if runPlan.SentToProvider {
			failures = append(failures, "provider run plan sent_to_provider must be false")
		}
		if !runPlan.ConfirmFlagUsed {
			failures = append(failures, "provider run plan confirm_flag_used must be true")
		}
		if runPlan.BlockedReason != MaterializedProviderDispatchBlockedReason {
			failures = append(failures, fmt.Sprintf("provider run plan blocked_reason %q must be %q", runPlan.BlockedReason, MaterializedProviderDispatchBlockedReason))
		}
		if runPlan.MaterializedSHA256 == "" {
			failures = append(failures, "provider run plan materialized_sha256 is required")
		} else {
			result.MaterializedSHA256 = runPlan.MaterializedSHA256
		}
		if runPlan.AssembledOutputSHA256 == "" {
			failures = append(failures, "provider run plan assembled_output_sha256 is required")
		}
	}

	assembledData, err := os.ReadFile(opts.AssembledOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read assembled output %q: %v", opts.AssembledOutputPath, err))
	} else {
		assembledSHA := sha256Hex(assembledData)
		if runPlan.AssembledOutputSHA256 != "" && assembledSHA != runPlan.AssembledOutputSHA256 {
			failures = append(failures, "assembled_output_sha256 mismatch with provider run plan")
		}
		result.AssembledOutputSHA256 = assembledSHA
		if maxTotalChars > 0 && utf8.RuneCount(assembledData) > maxTotalChars {
			failures = append(failures, fmt.Sprintf("assembled output exceeds max_total_chars %d", maxTotalChars))
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
		payload, err := renderMaterializedProviderPayload(MaterializedProviderDispatchProvider, string(assembledData))
		if err != nil {
			return MaterializedProviderPayloadResult{}, err
		}
		payloadBytes := len([]byte(payload))
		if maxPayloadBytes > 0 && payloadBytes > maxPayloadBytes {
			result.Status = lancedbpolicy.StatusFailed
			result.Failures = append(result.Failures, fmt.Sprintf("payload bytes %d exceed max_payload_bytes %d", payloadBytes, maxPayloadBytes))
		} else {
			if err := os.MkdirAll(filepath.Dir(opts.PayloadOutputPath), 0o755); err != nil {
				return MaterializedProviderPayloadResult{}, fmt.Errorf("create payload output dir: %w", err)
			}
			if err := os.WriteFile(opts.PayloadOutputPath, []byte(payload), 0o644); err != nil {
				return MaterializedProviderPayloadResult{}, fmt.Errorf("write payload output %q: %w", opts.PayloadOutputPath, err)
			}
			result.ProviderPayloadRendered = true
			result.ProviderPayloadSHA256 = sha256Hex([]byte(payload))
			result.PayloadBytes = payloadBytes
		}
	}

	if err := writeMaterializedProviderPayloadJSON(opts.OutputPath, result); err != nil {
		return MaterializedProviderPayloadResult{}, err
	}
	return result, nil
}

func renderMaterializedProviderPayload(provider, assembledOutput string) (string, error) {
	assembled := strings.TrimSpace(assembledOutput)
	if assembled == "" {
		return "", fmt.Errorf("assembled output is empty")
	}
	var b strings.Builder
	b.WriteString("# Provider payload dry-run\n\n")
	b.WriteString(ProviderPayloadDryRunNotice)
	b.WriteString("\n\n")
	b.WriteString("provider: ")
	b.WriteString(provider)
	b.WriteString("\n\n")
	b.WriteString(ProviderPayloadAssembledSectionHeader)
	b.WriteString("\n\n")
	b.WriteString(assembled)
	b.WriteByte('\n')
	return b.String(), nil
}

func LoadMaterializedProviderPayload(path string) (MaterializedProviderPayloadResult, []byte, error) {
	if err := validateRelativeSafePath("payload dry-run path", path); err != nil {
		return MaterializedProviderPayloadResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedProviderPayloadResult{}, nil, fmt.Errorf("read materialized provider payload %q: %w", path, err)
	}
	return ParseMaterializedProviderPayloadJSON(data)
}

func ParseMaterializedProviderPayloadJSON(data []byte) (MaterializedProviderPayloadResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedProviderPayloadResult{}, nil, fmt.Errorf("materialized provider payload must not contain materialized preview text")
	}
	var result MaterializedProviderPayloadResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedProviderPayloadResult{}, nil, fmt.Errorf("parse materialized provider payload json: %w", err)
	}
	return result, data, nil
}

func writeMaterializedProviderPayloadJSON(path string, result MaterializedProviderPayloadResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized provider payload output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider payload json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized provider payload must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized provider payload %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedProviderPayloadText(result MaterializedProviderPayloadResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_provider_payload_dry_run:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"provider_payload_rendered", fmt.Sprintf("%t", result.ProviderPayloadRendered)},
		{"provider", result.Provider},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"prompt_injection_real_runner", fmt.Sprintf("%t", result.PromptInjectionRealRunner)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"confirm_flag_used", fmt.Sprintf("%t", result.ConfirmFlagUsed)},
		{"blocked_reason", result.BlockedReason},
		{"materialized_sha256", result.MaterializedSHA256},
		{"assembled_output_sha256", result.AssembledOutputSHA256},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
		{"payload_bytes", fmt.Sprintf("%d", result.PayloadBytes)},
		{"max_payload_bytes", fmt.Sprintf("%d", result.MaxPayloadBytes)},
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
		return fmt.Errorf("materialized provider payload text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedProviderPayloadJSON(result MaterializedProviderPayloadResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider payload json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized provider payload json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
