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

type MaterializedProviderCallGateOptions struct {
	DispatchConfigPath               string
	PayloadReportPath                string
	PayloadOutputPath                string
	ConfirmInjectMaterializedContext bool
	OutputPath                       string
}

type MaterializedProviderCallGateResult struct {
	Status                    string   `json:"status"`
	ProviderCallGateReady     bool     `json:"provider_call_gate_ready"`
	ProviderCallAllowedNow    bool     `json:"provider_call_allowed_now"`
	NetworkCallAllowedNow     bool     `json:"network_call_allowed_now"`
	WorkerExecutionAllowedNow bool     `json:"worker_execution_allowed_now"`
	PromptInjectionAllowedNow bool     `json:"prompt_injection_allowed_now"`
	Provider                  string   `json:"provider"`
	ProviderPayloadValidated  bool     `json:"provider_payload_validated"`
	PayloadReadyForFutureCall bool     `json:"payload_ready_for_future_call"`
	SentToProvider            bool     `json:"sent_to_provider"`
	ConfirmFlagUsed           bool     `json:"confirm_flag_used"`
	BlockedReason             string   `json:"blocked_reason"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	ProviderPayloadSHA256     string   `json:"provider_payload_sha256,omitempty"`
	PayloadBytes              int      `json:"payload_bytes"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedProviderCallGate(opts MaterializedProviderCallGateOptions) (MaterializedProviderCallGateResult, error) {
	if !opts.ConfirmInjectMaterializedContext {
		return MaterializedProviderCallGateResult{}, fmt.Errorf("--%s is required", strings.TrimPrefix(RequiredFutureInjectFlag, "--"))
	}
	for _, check := range []struct {
		field string
		path  string
	}{
		{"dispatch config path", opts.DispatchConfigPath},
		{"payload report path", opts.PayloadReportPath},
		{"payload output path", opts.PayloadOutputPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedProviderCallGateResult{}, err
		}
	}

	result := MaterializedProviderCallGateResult{
		Status:                    lancedbpolicy.StatusOK,
		ProviderCallAllowedNow:    false,
		NetworkCallAllowedNow:     false,
		WorkerExecutionAllowedNow: false,
		PromptInjectionAllowedNow: false,
		Provider:                  MaterializedProviderDispatchProvider,
		SentToProvider:            false,
		ConfirmFlagUsed:           true,
		BlockedReason:             MaterializedProviderDispatchBlockedReason,
	}

	var failures []string
	var warnings []string

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
			if dispatchResult.AllowProviderCall {
				failures = append(failures, "dispatch config allow_provider_call must be false")
			}
			if dispatchResult.AllowNetwork {
				failures = append(failures, "dispatch config allow_network must be false")
			}
			if dispatchResult.AllowWorkerExecution {
				failures = append(failures, "dispatch config allow_worker_execution must be false")
			}
			if dispatchResult.AllowPromptInjection {
				failures = append(failures, "dispatch config allow_prompt_injection must be false")
			}
		}
	}

	payloadReport, payloadReportData, err := LoadMaterializedProviderPayloadReport(opts.PayloadReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if strings.Contains(string(payloadReportData), "text_excerpt") || strings.Contains(string(payloadReportData), "alpha text") {
			failures = append(failures, "payload report must not contain materialized preview text")
		}
		if payloadReport.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "payload report status must be ok or warning")
			failures = append(failures, payloadReport.Failures...)
		} else if payloadReport.Status == lancedbpolicy.StatusWarning {
			warnings = mergeWarnings(warnings, []string{"payload report status is warning"})
			warnings = mergeWarnings(warnings, payloadReport.Warnings)
		} else if payloadReport.Status != lancedbpolicy.StatusOK {
			failures = append(failures, fmt.Sprintf("payload report status %q must be ok or warning", payloadReport.Status))
		}
		if !payloadReport.ProviderPayloadValidated {
			failures = append(failures, "payload report provider_payload_validated must be true")
		}
		if payloadReport.Provider != MaterializedProviderDispatchProvider {
			failures = append(failures, fmt.Sprintf("payload report provider %q must be %q", payloadReport.Provider, MaterializedProviderDispatchProvider))
		}
		if payloadReport.ProviderCall {
			failures = append(failures, "payload report provider_call must be false")
		}
		if payloadReport.NetworkCall {
			failures = append(failures, "payload report network_call must be false")
		}
		if payloadReport.WorkerExecution {
			failures = append(failures, "payload report worker_execution must be false")
		}
		if payloadReport.SentToProvider {
			failures = append(failures, "payload report sent_to_provider must be false")
		}
		if payloadReport.PromptInjectionRealRunner {
			failures = append(failures, "payload report prompt_injection_real_runner must be false")
		}
		if !payloadReport.ContainsText {
			failures = append(failures, "payload report contains_text must be true")
		}
		if !payloadReport.PreviewOnly {
			failures = append(failures, "payload report preview_only must be true")
		}
		if payloadReport.ProviderPayloadSHA256 == "" {
			failures = append(failures, "payload report provider_payload_sha256 is required")
		}
		if payloadReport.MaterializedSHA256 != "" {
			result.MaterializedSHA256 = payloadReport.MaterializedSHA256
		}
		result.PayloadBytes = payloadReport.PayloadBytes
	}

	payloadOutputData, err := os.ReadFile(opts.PayloadOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read payload output %q: %v", opts.PayloadOutputPath, err))
	} else {
		payloadSHA := sha256Hex(payloadOutputData)
		if payloadReport.ProviderPayloadSHA256 != "" && payloadSHA != payloadReport.ProviderPayloadSHA256 {
			failures = append(failures, "provider_payload_sha256 mismatch with payload output")
		}
		result.ProviderPayloadSHA256 = payloadSHA
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ProviderCallGateReady = true
		result.ProviderPayloadValidated = true
		result.PayloadReadyForFutureCall = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}

	if err := writeMaterializedProviderCallGateJSON(opts.OutputPath, result); err != nil {
		return MaterializedProviderCallGateResult{}, err
	}
	return result, nil
}

func LoadMaterializedProviderCallGate(path string) (MaterializedProviderCallGateResult, []byte, error) {
	if err := validateRelativeSafePath("provider call gate path", path); err != nil {
		return MaterializedProviderCallGateResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedProviderCallGateResult{}, nil, fmt.Errorf("read materialized provider call gate %q: %w", path, err)
	}
	return ParseMaterializedProviderCallGateJSON(data)
}

func ParseMaterializedProviderCallGateJSON(data []byte) (MaterializedProviderCallGateResult, []byte, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedProviderCallGateResult{}, nil, fmt.Errorf("materialized provider call gate must not contain materialized preview text")
	}
	var result MaterializedProviderCallGateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return MaterializedProviderCallGateResult{}, nil, fmt.Errorf("parse materialized provider call gate json: %w", err)
	}
	return result, data, nil
}

func writeMaterializedProviderCallGateJSON(path string, result MaterializedProviderCallGateResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create materialized provider call gate output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider call gate json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("materialized provider call gate must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized provider call gate %q: %w", path, err)
	}
	return nil
}

func WriteMaterializedProviderCallGateText(result MaterializedProviderCallGateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_provider_call_gate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"provider_call_gate_ready", fmt.Sprintf("%t", result.ProviderCallGateReady)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"network_call_allowed_now", fmt.Sprintf("%t", result.NetworkCallAllowedNow)},
		{"worker_execution_allowed_now", fmt.Sprintf("%t", result.WorkerExecutionAllowedNow)},
		{"prompt_injection_allowed_now", fmt.Sprintf("%t", result.PromptInjectionAllowedNow)},
		{"provider", result.Provider},
		{"provider_payload_validated", fmt.Sprintf("%t", result.ProviderPayloadValidated)},
		{"payload_ready_for_future_call", fmt.Sprintf("%t", result.PayloadReadyForFutureCall)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"confirm_flag_used", fmt.Sprintf("%t", result.ConfirmFlagUsed)},
		{"blocked_reason", result.BlockedReason},
		{"materialized_sha256", result.MaterializedSHA256},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
		{"payload_bytes", fmt.Sprintf("%d", result.PayloadBytes)},
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
		return fmt.Errorf("materialized provider call gate text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedProviderCallGateJSON(result MaterializedProviderCallGateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider call gate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized provider call gate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
