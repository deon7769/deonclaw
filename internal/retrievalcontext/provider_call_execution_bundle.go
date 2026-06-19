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

type ProviderCallExecutionBundleOptions struct {
	DispatchConfigPath   string
	PayloadReportPath    string
	ProviderCallGatePath string
	ReadinessReportPath  string
	ApprovalPath         string
	OutputPath           string
}

type ProviderCallExecutionBundleResult struct {
	Status                          string   `json:"status"`
	ContainsText                    bool     `json:"contains_text"`
	ProviderCall                    bool     `json:"provider_call"`
	NetworkCall                     bool     `json:"network_call"`
	WorkerExecution                 bool     `json:"worker_execution"`
	SentToProvider                  bool     `json:"sent_to_provider"`
	ProviderCallAuthorizedForFuture bool     `json:"provider_call_authorized_for_future"`
	ProviderCallAllowedNow          bool     `json:"provider_call_allowed_now"`
	NetworkCallAllowedNow           bool     `json:"network_call_allowed_now"`
	WorkerExecutionAllowedNow       bool     `json:"worker_execution_allowed_now"`
	ExecutionSupportedNow           bool     `json:"execution_supported_now"`
	BlockedReason                   string   `json:"blocked_reason"`
	DispatchConfigSHA256            string   `json:"dispatch_config_sha256"`
	PayloadReportSHA256             string   `json:"payload_report_sha256"`
	ProviderCallGateSHA256          string   `json:"provider_call_gate_sha256"`
	ReadinessReportSHA256           string   `json:"readiness_report_sha256"`
	ApprovalSHA256                  string   `json:"approval_sha256"`
	ProviderPayloadSHA256           string   `json:"provider_payload_sha256"`
	MaterializedSHA256              string   `json:"materialized_sha256,omitempty"`
	Warnings                        []string `json:"warnings,omitempty"`
	Failures                        []string `json:"failures,omitempty"`
}

func ProviderCallExecutionBundle(opts ProviderCallExecutionBundleOptions) (ProviderCallExecutionBundleResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"dispatch config path", opts.DispatchConfigPath},
		{"payload report path", opts.PayloadReportPath},
		{"provider call gate path", opts.ProviderCallGatePath},
		{"readiness report path", opts.ReadinessReportPath},
		{"approval path", opts.ApprovalPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallExecutionBundleResult{}, err
		}
	}

	result := ProviderCallExecutionBundleResult{
		Status:                    lancedbpolicy.StatusOK,
		ContainsText:              false,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		SentToProvider:            false,
		ProviderCallAllowedNow:    false,
		NetworkCallAllowedNow:     false,
		WorkerExecutionAllowedNow: false,
		ExecutionSupportedNow:     false,
		BlockedReason:             MaterializedProviderDispatchBlockedReason,
	}

	var failures []string
	var warnings []string

	dispatchData, err := readArtifactBytesNoTextExcerpt("dispatch config", opts.DispatchConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		dispatchCfg, err := ParseMaterializedProviderDispatch(dispatchData)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			dispatchResult, err := MaterializedProviderDispatchValidate(dispatchCfg)
			if err != nil {
				failures = append(failures, err.Error())
			} else if dispatchResult.Status == lancedbpolicy.StatusFailed {
				failures = append(failures, "dispatch config validation failed")
				failures = append(failures, dispatchResult.Failures...)
			} else if dispatchResult.Enabled || dispatchResult.AllowProviderCall || dispatchResult.AllowNetwork || dispatchResult.AllowWorkerExecution {
				failures = append(failures, "dispatch config must keep provider call, network, and worker execution disabled")
			}
		}
		result.DispatchConfigSHA256 = sha256Hex(dispatchData)
	}

	payloadReport, payloadReportData, err := LoadMaterializedProviderPayloadReport(opts.PayloadReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if err := validateMaterializedProviderPayloadReportForApproval(payloadReport); err != nil {
			failures = append(failures, err.Error())
		}
		result.PayloadReportSHA256 = sha256Hex(payloadReportData)
		result.ProviderPayloadSHA256 = payloadReport.ProviderPayloadSHA256
		result.MaterializedSHA256 = payloadReport.MaterializedSHA256
		warnings = mergeWarnings(warnings, payloadReport.Warnings)
	}

	gate, gateData, err := LoadMaterializedProviderCallGate(opts.ProviderCallGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if err := validateMaterializedProviderCallGateForApproval(gate); err != nil {
			failures = append(failures, err.Error())
		}
		result.ProviderCallGateSHA256 = sha256Hex(gateData)
		if result.ProviderPayloadSHA256 != "" && gate.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
			failures = append(failures, "provider_payload_sha256 mismatch between payload report and gate")
		}
		if result.MaterializedSHA256 == "" {
			result.MaterializedSHA256 = gate.MaterializedSHA256
		}
		warnings = mergeWarnings(warnings, gate.Warnings)
	}

	readiness, readinessData, err := LoadMaterializedProviderCallReadinessReport(opts.ReadinessReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		if err := validateMaterializedProviderCallReadinessReportForApproval(readiness); err != nil {
			failures = append(failures, err.Error())
		}
		result.ReadinessReportSHA256 = sha256Hex(readinessData)
		if result.ProviderPayloadSHA256 != "" && readiness.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
			failures = append(failures, "provider_payload_sha256 mismatch between readiness report and payload report")
		}
		warnings = mergeWarnings(warnings, readiness.Warnings)
	}

	approvalData, err := readArtifactBytesNoTextExcerpt("provider call approval", opts.ApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		approvalInspect, err := InspectProviderCallApprovalBytes(approvalData, InspectProviderCallApprovalOptions{})
		if err != nil {
			failures = append(failures, err.Error())
		} else if approvalInspect.Status != lancedbpolicy.StatusOK {
			failures = append(failures, "provider call approval inspect failed")
			failures = append(failures, approvalInspect.Failures...)
		} else {
			approval, err := ParseProviderCallApprovalJSON(approvalData)
			if err != nil {
				failures = append(failures, err.Error())
			} else {
				result.ApprovalSHA256 = sha256Hex(approvalData)
				if result.ReadinessReportSHA256 != "" && approval.ReadinessReportSHA256 != result.ReadinessReportSHA256 {
					failures = append(failures, "approval readiness_report_sha256 mismatch")
				}
				if result.ProviderCallGateSHA256 != "" && approval.ProviderCallGateSHA256 != result.ProviderCallGateSHA256 {
					failures = append(failures, "approval provider_call_gate_sha256 mismatch")
				}
				if result.PayloadReportSHA256 != "" && approval.PayloadReportSHA256 != result.PayloadReportSHA256 {
					failures = append(failures, "approval payload_report_sha256 mismatch")
				}
				if result.ProviderPayloadSHA256 != "" && approval.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
					failures = append(failures, "approval provider_payload_sha256 mismatch")
				}
				if !approval.ProviderCallAuthorizedForFuture {
					failures = append(failures, "approval provider_call_authorized_for_future must be true")
				}
				if approval.ProviderCallAllowedNow || approval.NetworkCallAllowedNow || approval.WorkerExecutionAllowedNow || approval.SentToProvider {
					failures = append(failures, "approval must keep provider_call, network_call, worker_execution, and sent_to_provider blocked")
				}
				warnings = mergeWarnings(warnings, approval.Warnings)
			}
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ProviderCallAuthorizedForFuture = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}

	if err := writeProviderCallExecutionBundleJSON(opts.OutputPath, result); err != nil {
		return ProviderCallExecutionBundleResult{}, err
	}
	return result, nil
}

func writeProviderCallExecutionBundleJSON(path string, result ProviderCallExecutionBundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider call execution bundle output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call execution bundle json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider call execution bundle must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider call execution bundle %q: %w", path, err)
	}
	return nil
}

func WriteProviderCallExecutionBundleText(result ProviderCallExecutionBundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_execution_bundle:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"provider_call_authorized_for_future", fmt.Sprintf("%t", result.ProviderCallAuthorizedForFuture)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"network_call_allowed_now", fmt.Sprintf("%t", result.NetworkCallAllowedNow)},
		{"worker_execution_allowed_now", fmt.Sprintf("%t", result.WorkerExecutionAllowedNow)},
		{"execution_supported_now", fmt.Sprintf("%t", result.ExecutionSupportedNow)},
		{"blocked_reason", result.BlockedReason},
		{"dispatch_config_sha256", result.DispatchConfigSHA256},
		{"payload_report_sha256", result.PayloadReportSHA256},
		{"provider_call_gate_sha256", result.ProviderCallGateSHA256},
		{"readiness_report_sha256", result.ReadinessReportSHA256},
		{"approval_sha256", result.ApprovalSHA256},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
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
	text := fmt.Sprintf("%+v", result)
	if strings.Contains(text, "text_excerpt") || strings.Contains(text, "alpha text") {
		return fmt.Errorf("provider call execution bundle text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallExecutionBundleJSON(result ProviderCallExecutionBundleResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call execution bundle json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call execution bundle json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
