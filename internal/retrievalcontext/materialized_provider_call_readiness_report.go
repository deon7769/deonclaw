package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type MaterializedProviderCallReadinessReportOptions struct {
	ProviderCallGatePath string
	PayloadReportPath    string
}

type MaterializedProviderCallReadinessReportResult struct {
	Status                     string   `json:"status"`
	ProviderCallReadinessReady bool     `json:"provider_call_readiness_ready"`
	ProviderCallAllowedNow     bool     `json:"provider_call_allowed_now"`
	NetworkCallAllowedNow      bool     `json:"network_call_allowed_now"`
	WorkerExecutionAllowedNow  bool     `json:"worker_execution_allowed_now"`
	PromptInjectionAllowedNow  bool     `json:"prompt_injection_allowed_now"`
	Provider                   string   `json:"provider"`
	PayloadReadyForFutureCall  bool     `json:"payload_ready_for_future_call"`
	SentToProvider             bool     `json:"sent_to_provider"`
	BlockedReason              string   `json:"blocked_reason"`
	MaterializedSHA256         string   `json:"materialized_sha256,omitempty"`
	ProviderPayloadSHA256      string   `json:"provider_payload_sha256,omitempty"`
	Warnings                   []string `json:"warnings,omitempty"`
	Failures                   []string `json:"failures,omitempty"`
}

func MaterializedProviderCallReadinessReport(opts MaterializedProviderCallReadinessReportOptions) (MaterializedProviderCallReadinessReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"provider call gate path", opts.ProviderCallGatePath},
		{"payload report path", opts.PayloadReportPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedProviderCallReadinessReportResult{}, err
		}
	}

	gate, gateData, err := LoadMaterializedProviderCallGate(opts.ProviderCallGatePath)
	if err != nil {
		return MaterializedProviderCallReadinessReportResult{}, err
	}
	if strings.Contains(string(gateData), "text_excerpt") || strings.Contains(string(gateData), "alpha text") {
		return MaterializedProviderCallReadinessReportResult{}, fmt.Errorf("materialized provider call gate must not contain materialized preview text")
	}

	payloadReport, payloadReportData, err := LoadMaterializedProviderPayloadReport(opts.PayloadReportPath)
	if err != nil {
		return MaterializedProviderCallReadinessReportResult{}, err
	}
	if strings.Contains(string(payloadReportData), "text_excerpt") || strings.Contains(string(payloadReportData), "alpha text") {
		return MaterializedProviderCallReadinessReportResult{}, fmt.Errorf("materialized provider payload report must not contain materialized preview text")
	}

	result := MaterializedProviderCallReadinessReportResult{
		Status:                    lancedbpolicy.StatusOK,
		ProviderCallAllowedNow:    false,
		NetworkCallAllowedNow:     false,
		WorkerExecutionAllowedNow: false,
		PromptInjectionAllowedNow: false,
		Provider:                  MaterializedProviderDispatchProvider,
		SentToProvider:            false,
		BlockedReason:             MaterializedProviderDispatchBlockedReason,
	}

	var failures []string
	var warnings []string
	warnings = mergeWarnings(warnings, gate.Warnings, payloadReport.Warnings)

	if gate.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "provider call gate status must be ok or warning")
		failures = append(failures, gate.Failures...)
	} else if gate.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"provider call gate status is warning"})
	} else if gate.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("provider call gate status %q must be ok or warning", gate.Status))
	}
	if !gate.ProviderCallGateReady {
		failures = append(failures, "provider call gate provider_call_gate_ready must be true")
	}
	if gate.ProviderCallAllowedNow {
		failures = append(failures, "provider call gate provider_call_allowed_now must be false")
	}
	if gate.NetworkCallAllowedNow {
		failures = append(failures, "provider call gate network_call_allowed_now must be false")
	}
	if gate.WorkerExecutionAllowedNow {
		failures = append(failures, "provider call gate worker_execution_allowed_now must be false")
	}
	if gate.PromptInjectionAllowedNow {
		failures = append(failures, "provider call gate prompt_injection_allowed_now must be false")
	}
	if gate.Provider != MaterializedProviderDispatchProvider {
		failures = append(failures, fmt.Sprintf("provider call gate provider %q must be %q", gate.Provider, MaterializedProviderDispatchProvider))
	}
	if !gate.ProviderPayloadValidated {
		failures = append(failures, "provider call gate provider_payload_validated must be true")
	}
	if !gate.PayloadReadyForFutureCall {
		failures = append(failures, "provider call gate payload_ready_for_future_call must be true")
	}
	if gate.SentToProvider {
		failures = append(failures, "provider call gate sent_to_provider must be false")
	}
	if !gate.ConfirmFlagUsed {
		failures = append(failures, "provider call gate confirm_flag_used must be true")
	}
	if gate.BlockedReason != MaterializedProviderDispatchBlockedReason {
		failures = append(failures, fmt.Sprintf("provider call gate blocked_reason %q must be %q", gate.BlockedReason, MaterializedProviderDispatchBlockedReason))
	}
	if gate.MaterializedSHA256 != "" {
		result.MaterializedSHA256 = gate.MaterializedSHA256
	}
	if gate.ProviderPayloadSHA256 != "" {
		result.ProviderPayloadSHA256 = gate.ProviderPayloadSHA256
	}

	if payloadReport.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "payload report status must be ok or warning")
		failures = append(failures, payloadReport.Failures...)
	} else if payloadReport.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"payload report status is warning"})
	} else if payloadReport.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("payload report status %q must be ok or warning", payloadReport.Status))
	}
	if !payloadReport.ProviderPayloadValidated {
		failures = append(failures, "payload report provider_payload_validated must be true")
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
	if payloadReport.MaterializedSHA256 != "" {
		if result.MaterializedSHA256 != "" && payloadReport.MaterializedSHA256 != result.MaterializedSHA256 {
			failures = append(failures, "materialized_sha256 mismatch between provider call gate and payload report")
		}
		if result.MaterializedSHA256 == "" {
			result.MaterializedSHA256 = payloadReport.MaterializedSHA256
		}
	}
	if payloadReport.ProviderPayloadSHA256 != "" {
		if result.ProviderPayloadSHA256 != "" && payloadReport.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
			failures = append(failures, "provider_payload_sha256 mismatch between provider call gate and payload report")
		}
		if result.ProviderPayloadSHA256 == "" {
			result.ProviderPayloadSHA256 = payloadReport.ProviderPayloadSHA256
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ProviderCallReadinessReady = true
		result.PayloadReadyForFutureCall = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func WriteMaterializedProviderCallReadinessReportText(result MaterializedProviderCallReadinessReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_provider_call_readiness_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"provider_call_readiness_ready", fmt.Sprintf("%t", result.ProviderCallReadinessReady)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"network_call_allowed_now", fmt.Sprintf("%t", result.NetworkCallAllowedNow)},
		{"worker_execution_allowed_now", fmt.Sprintf("%t", result.WorkerExecutionAllowedNow)},
		{"prompt_injection_allowed_now", fmt.Sprintf("%t", result.PromptInjectionAllowedNow)},
		{"provider", result.Provider},
		{"payload_ready_for_future_call", fmt.Sprintf("%t", result.PayloadReadyForFutureCall)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"blocked_reason", result.BlockedReason},
		{"materialized_sha256", result.MaterializedSHA256},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
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
		return fmt.Errorf("materialized provider call readiness report text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedProviderCallReadinessReportJSON(result MaterializedProviderCallReadinessReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider call readiness report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized provider call readiness report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
