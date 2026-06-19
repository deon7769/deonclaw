package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type MaterializedProviderPayloadReportOptions struct {
	PayloadDryRunPath string
	PayloadOutputPath string
}

type MaterializedProviderPayloadReportResult struct {
	Status                    string   `json:"status"`
	ProviderPayloadValidated  bool     `json:"provider_payload_validated"`
	Provider                  string   `json:"provider"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToProvider            bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner bool     `json:"prompt_injection_real_runner"`
	ContainsText              bool     `json:"contains_text"`
	PreviewOnly               bool     `json:"preview_only"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256     string   `json:"assembled_output_sha256,omitempty"`
	ProviderPayloadSHA256     string   `json:"provider_payload_sha256,omitempty"`
	PayloadBytes              int      `json:"payload_bytes"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func MaterializedProviderPayloadReport(opts MaterializedProviderPayloadReportOptions) (MaterializedProviderPayloadReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"payload dry-run path", opts.PayloadDryRunPath},
		{"payload output path", opts.PayloadOutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return MaterializedProviderPayloadReportResult{}, err
		}
	}

	payload, payloadData, err := LoadMaterializedProviderPayload(opts.PayloadDryRunPath)
	if err != nil {
		return MaterializedProviderPayloadReportResult{}, err
	}
	if strings.Contains(string(payloadData), "text_excerpt") || strings.Contains(string(payloadData), "alpha text") {
		return MaterializedProviderPayloadReportResult{}, fmt.Errorf("materialized provider payload must not contain materialized preview text")
	}

	result := MaterializedProviderPayloadReportResult{
		Status:                    lancedbpolicy.StatusOK,
		Provider:                  MaterializedProviderDispatchProvider,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		SentToProvider:            false,
		PromptInjectionRealRunner: false,
		ContainsText:              true,
		PreviewOnly:               true,
		MaterializedSHA256:        payload.MaterializedSHA256,
		AssembledOutputSHA256:     payload.AssembledOutputSHA256,
		PayloadBytes:              payload.PayloadBytes,
	}

	var failures []string
	var warnings []string
	warnings = mergeWarnings(warnings, payload.Warnings)

	if payload.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "payload status must be ok or warning")
		failures = append(failures, payload.Failures...)
	} else if payload.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"payload status is warning"})
	} else if payload.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("payload status %q must be ok or warning", payload.Status))
	}
	if !payload.ProviderPayloadRendered {
		failures = append(failures, "payload provider_payload_rendered must be true")
	}
	if payload.Provider != MaterializedProviderDispatchProvider {
		failures = append(failures, fmt.Sprintf("payload provider %q must be %q", payload.Provider, MaterializedProviderDispatchProvider))
	}
	if payload.ProviderCall {
		failures = append(failures, "payload provider_call must be false")
	}
	if payload.NetworkCall {
		failures = append(failures, "payload network_call must be false")
	}
	if payload.WorkerExecution {
		failures = append(failures, "payload worker_execution must be false")
	}
	if payload.SentToProvider {
		failures = append(failures, "payload sent_to_provider must be false")
	}
	if payload.PromptInjectionRealRunner {
		failures = append(failures, "payload prompt_injection_real_runner must be false")
	}
	if !payload.ContainsText {
		failures = append(failures, "payload contains_text must be true")
	}
	if !payload.PreviewOnly {
		failures = append(failures, "payload preview_only must be true")
	}
	if !payload.ConfirmFlagUsed {
		failures = append(failures, "payload confirm_flag_used must be true")
	}
	if payload.BlockedReason != MaterializedProviderDispatchBlockedReason {
		failures = append(failures, fmt.Sprintf("payload blocked_reason %q must be %q", payload.BlockedReason, MaterializedProviderDispatchBlockedReason))
	}
	if payload.ProviderPayloadSHA256 == "" {
		failures = append(failures, "payload provider_payload_sha256 is required")
	}

	payloadOutputData, err := os.ReadFile(opts.PayloadOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read payload output %q: %v", opts.PayloadOutputPath, err))
	} else {
		payloadSHA := sha256Hex(payloadOutputData)
		if payload.ProviderPayloadSHA256 != "" && payloadSHA != payload.ProviderPayloadSHA256 {
			failures = append(failures, "provider_payload_sha256 mismatch with payload output")
		}
		result.ProviderPayloadSHA256 = payloadSHA
		payloadText := string(payloadOutputData)
		if !strings.Contains(payloadText, ProviderPayloadDryRunNotice) {
			failures = append(failures, "payload output missing dry-run notice")
		}
		if !strings.Contains(payloadText, "text_excerpt") {
			failures = append(failures, "payload output missing text_excerpt")
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ProviderPayloadValidated = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func WriteMaterializedProviderPayloadReportText(result MaterializedProviderPayloadReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_provider_payload_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"provider_payload_validated", fmt.Sprintf("%t", result.ProviderPayloadValidated)},
		{"provider", result.Provider},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"prompt_injection_real_runner", fmt.Sprintf("%t", result.PromptInjectionRealRunner)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"materialized_sha256", result.MaterializedSHA256},
		{"assembled_output_sha256", result.AssembledOutputSHA256},
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
		return fmt.Errorf("materialized provider payload report text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedProviderPayloadReportJSON(result MaterializedProviderPayloadReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider payload report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized provider payload report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
