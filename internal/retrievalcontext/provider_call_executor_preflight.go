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

const RequiredFutureConfirmProviderExecutorDispatch = "--confirm-provider-executor-dispatch"

type ProviderCallExecutorPreflightOptions struct {
	ExecutorConfigPath  string
	DryRunPath          string
	DryRunReportPath    string
	ExecutionBundlePath string
	ApprovalPath        string
	OutputPath          string
}

type ProviderCallExecutorPreflightResult struct {
	Status                    string   `json:"status"`
	ExecutorPreflightReady    bool     `json:"executor_preflight_ready"`
	ExecutionAllowedNow       bool     `json:"execution_allowed_now"`
	TransportCalled           bool     `json:"transport_called"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToProvider            bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner bool     `json:"prompt_injection_real_runner"`
	RequiredFutureConfirmFlag string   `json:"required_future_confirm_flag"`
	BlockedReason             string   `json:"blocked_reason"`
	ExecutorConfigSHA256      string   `json:"executor_config_sha256"`
	DryRunSHA256              string   `json:"dry_run_sha256"`
	DryRunReportSHA256        string   `json:"dry_run_report_sha256"`
	ExecutionBundleSHA256     string   `json:"execution_bundle_sha256"`
	ApprovalSHA256            string   `json:"approval_sha256"`
	ProviderPayloadSHA256     string   `json:"provider_payload_sha256"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func ProviderCallExecutorPreflight(opts ProviderCallExecutorPreflightOptions) (ProviderCallExecutorPreflightResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"executor config path", opts.ExecutorConfigPath},
		{"dry-run path", opts.DryRunPath},
		{"dry-run report path", opts.DryRunReportPath},
		{"execution bundle path", opts.ExecutionBundlePath},
		{"approval path", opts.ApprovalPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallExecutorPreflightResult{}, err
		}
	}

	result := ProviderCallExecutorPreflightResult{
		Status:                    lancedbpolicy.StatusOK,
		ExecutionAllowedNow:       false,
		TransportCalled:           false,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		SentToProvider:            false,
		PromptInjectionRealRunner: false,
		RequiredFutureConfirmFlag: RequiredFutureConfirmProviderExecutorDispatch,
		BlockedReason:             ProviderCallExecutorBlockedReason,
	}

	var failures []string
	var warnings []string

	executorCfgData, err := readArtifactBytesNoTextExcerpt("executor config", opts.ExecutorConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.ExecutorConfigSHA256 = sha256Hex(executorCfgData)
		cfg, err := ParseProviderCallExecutorConfig(executorCfgData)
		if err != nil {
			failures = append(failures, err.Error())
		} else if err := ValidateProviderCallExecutorConfig(cfg); err != nil {
			failures = append(failures, err.Error())
		}
	}

	dryRunReport, dryRunReportData, err := LoadProviderCallExecutorDryRunReportJSON(opts.DryRunReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.DryRunReportSHA256 = sha256Hex(dryRunReportData)
		reportResult, err := ProviderCallExecutorDryRunReport(ProviderCallExecutorDryRunReportOptions{
			DryRunPath: opts.DryRunPath, ExecutorConfigPath: opts.ExecutorConfigPath,
		})
		if err != nil {
			failures = append(failures, err.Error())
		} else if reportResult.Status == lancedbpolicy.StatusFailed || !reportResult.ExecutorDryRunValidated {
			failures = append(failures, "dry-run report validation failed")
			failures = append(failures, reportResult.Failures...)
		} else {
			warnings = mergeWarnings(warnings, reportResult.Warnings)
			result.ExecutionBundleSHA256 = reportResult.ExecutionBundleSHA256
			result.ApprovalSHA256 = reportResult.ApprovalSHA256
			result.ProviderPayloadSHA256 = reportResult.ProviderPayloadSHA256
			result.MaterializedSHA256 = reportResult.MaterializedSHA256
		}
		_ = dryRunReport
	}

	dryRun, dryRunData, err := LoadProviderCallExecutorDryRun(opts.DryRunPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.DryRunSHA256 = sha256Hex(dryRunData)
		if !dryRun.ExecutorDryRunReady {
			failures = append(failures, "dry-run executor_dry_run_ready must be true")
		}
		if dryRun.TransportCalled || dryRun.ContainsText {
			failures = append(failures, "dry-run must keep transport_called false and contains_text false")
		}
		if dryRun.ProviderCall || dryRun.NetworkCall || dryRun.WorkerExecution || dryRun.SentToProvider {
			failures = append(failures, "dry-run must keep execution flags false")
		}
		warnings = mergeWarnings(warnings, dryRun.Warnings)
		reconcileProviderExecutorHash("dry-run", dryRun.ExecutorConfigSHA256, result.ExecutorConfigSHA256, &failures)
		reconcileProviderExecutorHash("dry-run", dryRun.ExecutionBundleSHA256, result.ExecutionBundleSHA256, &failures)
		reconcileProviderExecutorHash("dry-run", dryRun.ApprovalSHA256, result.ApprovalSHA256, &failures)
		reconcileProviderExecutorHash("dry-run", dryRun.ProviderPayloadSHA256, result.ProviderPayloadSHA256, &failures)
		if result.MaterializedSHA256 == "" {
			result.MaterializedSHA256 = dryRun.MaterializedSHA256
		}
	}

	bundle, bundleData, err := LoadProviderCallExecutionBundle(opts.ExecutionBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.ExecutionBundleSHA256 = sha256Hex(bundleData)
		if bundle.Status == lancedbpolicy.StatusFailed || !bundle.ProviderCallAuthorizedForFuture {
			failures = append(failures, "execution bundle must be authorized for future provider call")
		}
		if bundle.ProviderCall || bundle.NetworkCall || bundle.WorkerExecution || bundle.SentToProvider {
			failures = append(failures, "execution bundle must keep execution flags false")
		}
		reconcileProviderExecutorHash("execution bundle", bundle.ApprovalSHA256, result.ApprovalSHA256, &failures)
		reconcileProviderExecutorHash("execution bundle", bundle.ProviderPayloadSHA256, result.ProviderPayloadSHA256, &failures)
		if result.MaterializedSHA256 == "" {
			result.MaterializedSHA256 = bundle.MaterializedSHA256
		}
		warnings = mergeWarnings(warnings, bundle.Warnings)
	}

	approvalData, err := readArtifactBytesNoTextExcerpt("provider call approval", opts.ApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		inspect, err := InspectProviderCallApprovalBytes(approvalData, InspectProviderCallApprovalOptions{})
		if err != nil {
			failures = append(failures, err.Error())
		} else if inspect.Status != lancedbpolicy.StatusOK {
			failures = append(failures, "provider call approval inspect failed")
			failures = append(failures, inspect.Failures...)
		} else {
			result.ApprovalSHA256 = sha256Hex(approvalData)
			reconcileProviderExecutorHash("approval", inspect.ProviderPayloadSHA256, result.ProviderPayloadSHA256, &failures)
			if result.MaterializedSHA256 == "" {
				result.MaterializedSHA256 = inspect.MaterializedSHA256
			}
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ExecutorPreflightReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
		if err := writeProviderCallExecutorPreflightJSON(opts.OutputPath, result); err != nil {
			return ProviderCallExecutorPreflightResult{}, err
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func LoadProviderCallExecutorPreflight(path string) (ProviderCallExecutorPreflightResult, []byte, error) {
	if err := validateRelativeSafePath("preflight path", path); err != nil {
		return ProviderCallExecutorPreflightResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("provider call executor preflight", path)
	if err != nil {
		return ProviderCallExecutorPreflightResult{}, nil, err
	}
	var result ProviderCallExecutorPreflightResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderCallExecutorPreflightResult{}, nil, fmt.Errorf("parse provider call executor preflight: %w", err)
	}
	return result, data, nil
}

func LoadProviderCallExecutorDryRunReportJSON(path string) (ProviderCallExecutorDryRunReportResult, []byte, error) {
	if err := validateRelativeSafePath("dry-run report path", path); err != nil {
		return ProviderCallExecutorDryRunReportResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("provider call executor dry-run report", path)
	if err != nil {
		return ProviderCallExecutorDryRunReportResult{}, nil, err
	}
	var result ProviderCallExecutorDryRunReportResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderCallExecutorDryRunReportResult{}, nil, fmt.Errorf("parse provider call executor dry-run report: %w", err)
	}
	return result, data, nil
}

func reconcileProviderExecutorHash(label, got, want string, failures *[]string) {
	if got == "" || want == "" {
		return
	}
	if got != want {
		*failures = append(*failures, fmt.Sprintf("%s hash mismatch", label))
	}
}

func writeProviderCallExecutorPreflightJSON(path string, result ProviderCallExecutorPreflightResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider call executor preflight output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor preflight json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider call executor preflight must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider call executor preflight %q: %w", path, err)
	}
	return nil
}

func WriteProviderCallExecutorPreflightText(result ProviderCallExecutorPreflightResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_executor_preflight:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"executor_preflight_ready", fmt.Sprintf("%t", result.ExecutorPreflightReady)},
		{"execution_allowed_now", fmt.Sprintf("%t", result.ExecutionAllowedNow)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"prompt_injection_real_runner", fmt.Sprintf("%t", result.PromptInjectionRealRunner)},
		{"required_future_confirm_flag", result.RequiredFutureConfirmFlag},
		{"blocked_reason", result.BlockedReason},
		{"executor_config_sha256", result.ExecutorConfigSHA256},
		{"dry_run_sha256", result.DryRunSHA256},
		{"dry_run_report_sha256", result.DryRunReportSHA256},
		{"execution_bundle_sha256", result.ExecutionBundleSHA256},
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
		return fmt.Errorf("provider call executor preflight text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallExecutorPreflightJSON(result ProviderCallExecutorPreflightResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor preflight json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call executor preflight json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
