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

type ProviderExecutorReleaseBundleOptions struct {
	ExecutorConfigPath    string
	ExecutionBundlePath   string
	DryRunPath            string
	DryRunReportPath      string
	ExecutorPreflightPath string
	DispatchApprovalPath  string
	TransportPlanPath     string
	OutputPath            string
}

type ProviderExecutorReleaseBundleResult struct {
	Status                    string   `json:"status"`
	ReleaseBundleReady        bool     `json:"release_bundle_ready"`
	ActivationAllowedNow      bool     `json:"activation_allowed_now"`
	ExecutionSupportedNow     bool     `json:"execution_supported_now"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	TransportCalled           bool     `json:"transport_called"`
	SentToProvider            bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner bool     `json:"prompt_injection_real_runner"`
	BlockedReason             string   `json:"blocked_reason"`
	ExecutorConfigSHA256      string   `json:"executor_config_sha256"`
	ExecutionBundleSHA256     string   `json:"execution_bundle_sha256"`
	DryRunSHA256              string   `json:"dry_run_sha256"`
	DryRunReportSHA256        string   `json:"dry_run_report_sha256"`
	PreflightSHA256           string   `json:"preflight_sha256"`
	DispatchApprovalSHA256    string   `json:"dispatch_approval_sha256"`
	TransportPlanSHA256       string   `json:"transport_plan_sha256"`
	ProviderPayloadSHA256     string   `json:"provider_payload_sha256"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

type ProviderExecutorReleaseGateOptions struct {
	ExecutorConfigPath    string
	ExecutionBundlePath   string
	DryRunPath            string
	DryRunReportPath      string
	ExecutorPreflightPath string
	DispatchApprovalPath  string
	TransportPlanPath     string
	ReleaseBundlePath     string
}

type ProviderExecutorReleaseGateResult struct {
	Status                    string   `json:"status"`
	ActivationGateReady       bool     `json:"activation_gate_ready"`
	ReleaseBundleReady        bool     `json:"release_bundle_ready"`
	ActivationAllowedNow      bool     `json:"activation_allowed_now"`
	ExecutionSupportedNow     bool     `json:"execution_supported_now"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	TransportCalled           bool     `json:"transport_called"`
	SentToProvider            bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner bool     `json:"prompt_injection_real_runner"`
	BlockedReason             string   `json:"blocked_reason"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func ProviderExecutorReleaseBundle(opts ProviderExecutorReleaseBundleOptions) (ProviderExecutorReleaseBundleResult, error) {
	chain, failures, warnings, err := validateProviderExecutorReleaseChain(opts.ExecutorConfigPath, opts.ExecutionBundlePath, opts.DryRunPath, opts.DryRunReportPath, opts.ExecutorPreflightPath, opts.DispatchApprovalPath, opts.TransportPlanPath)
	if err != nil {
		return ProviderExecutorReleaseBundleResult{}, err
	}

	result := ProviderExecutorReleaseBundleResult{
		Status:                    lancedbpolicy.StatusOK,
		ActivationAllowedNow:      false,
		ExecutionSupportedNow:     false,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		TransportCalled:           false,
		SentToProvider:            false,
		PromptInjectionRealRunner: false,
		BlockedReason:             ProviderCallExecutorBlockedReason,
		ExecutorConfigSHA256:      chain.executorConfigSHA256,
		ExecutionBundleSHA256:     chain.executionBundleSHA256,
		DryRunSHA256:              chain.dryRunSHA256,
		DryRunReportSHA256:        chain.dryRunReportSHA256,
		PreflightSHA256:           chain.preflightSHA256,
		DispatchApprovalSHA256:    chain.dispatchApprovalSHA256,
		TransportPlanSHA256:       chain.transportPlanSHA256,
		ProviderPayloadSHA256:     chain.providerPayloadSHA256,
		MaterializedSHA256:        chain.materializedSHA256,
		Warnings:                  warnings,
		Failures:                  failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.ReleaseBundleReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
		if err := writeProviderExecutorReleaseBundleJSON(opts.OutputPath, result); err != nil {
			return ProviderExecutorReleaseBundleResult{}, err
		}
	}
	return result, nil
}

func ProviderExecutorReleaseGate(opts ProviderExecutorReleaseGateOptions) (ProviderExecutorReleaseGateResult, error) {
	if err := validateRelativeSafePath("release bundle path", opts.ReleaseBundlePath); err != nil {
		return ProviderExecutorReleaseGateResult{}, err
	}

	chain, failures, warnings, err := validateProviderExecutorReleaseChain(opts.ExecutorConfigPath, opts.ExecutionBundlePath, opts.DryRunPath, opts.DryRunReportPath, opts.ExecutorPreflightPath, opts.DispatchApprovalPath, opts.TransportPlanPath)
	if err != nil {
		return ProviderExecutorReleaseGateResult{}, err
	}

	bundleData, err := readArtifactBytesNoTextExcerpt("release bundle", opts.ReleaseBundlePath)
	if err != nil {
		return ProviderExecutorReleaseGateResult{}, err
	}
	bundle, err := ParseProviderExecutorReleaseBundleJSON(bundleData)
	if err != nil {
		return ProviderExecutorReleaseGateResult{}, err
	}
	if !bundle.ReleaseBundleReady {
		failures = append(failures, "release bundle release_bundle_ready must be true")
	}
	if bundle.ExecutorConfigSHA256 != chain.executorConfigSHA256 {
		failures = append(failures, "release bundle executor_config_sha256 mismatch")
	}
	if bundle.PreflightSHA256 != chain.preflightSHA256 {
		failures = append(failures, "release bundle preflight_sha256 mismatch")
	}
	if bundle.TransportPlanSHA256 != chain.transportPlanSHA256 {
		failures = append(failures, "release bundle transport_plan_sha256 mismatch")
	}
	if bundle.ActivationAllowedNow || bundle.ExecutionSupportedNow || bundle.ProviderCall || bundle.NetworkCall || bundle.WorkerExecution || bundle.TransportCalled || bundle.SentToProvider {
		failures = append(failures, "release bundle must keep activation and execution flags blocked")
	}

	result := ProviderExecutorReleaseGateResult{
		Status:                    lancedbpolicy.StatusOK,
		ReleaseBundleReady:        bundle.ReleaseBundleReady,
		ActivationAllowedNow:      false,
		ExecutionSupportedNow:     false,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		TransportCalled:           false,
		SentToProvider:            false,
		PromptInjectionRealRunner: false,
		BlockedReason:             ProviderCallExecutorBlockedReason,
		Warnings:                  warnings,
		Failures:                  failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.ActivationGateReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	}
	return result, nil
}

type providerExecutorReleaseChain struct {
	executorConfigSHA256   string
	executionBundleSHA256  string
	dryRunSHA256           string
	dryRunReportSHA256     string
	preflightSHA256        string
	dispatchApprovalSHA256 string
	transportPlanSHA256    string
	providerPayloadSHA256  string
	materializedSHA256     string
}

func validateProviderExecutorReleaseChain(executorConfigPath, executionBundlePath, dryRunPath, dryRunReportPath, preflightPath, dispatchApprovalPath, transportPlanPath string) (providerExecutorReleaseChain, []string, []string, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"executor config path", executorConfigPath},
		{"execution bundle path", executionBundlePath},
		{"dry-run path", dryRunPath},
		{"dry-run report path", dryRunReportPath},
		{"executor preflight path", preflightPath},
		{"dispatch approval path", dispatchApprovalPath},
		{"transport plan path", transportPlanPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return providerExecutorReleaseChain{}, nil, nil, err
		}
	}

	var chain providerExecutorReleaseChain
	var failures []string
	var warnings []string

	executorCfgData, err := readArtifactBytesNoTextExcerpt("executor config", executorConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.executorConfigSHA256 = sha256Hex(executorCfgData)
		cfg, err := ParseProviderCallExecutorConfig(executorCfgData)
		if err != nil {
			failures = append(failures, err.Error())
		} else if err := ValidateProviderCallExecutorConfig(cfg); err != nil {
			failures = append(failures, err.Error())
		}
	}

	preflight, preflightData, err := LoadProviderCallExecutorPreflight(preflightPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.preflightSHA256 = sha256Hex(preflightData)
		if !preflight.ExecutorPreflightReady {
			failures = append(failures, "preflight executor_preflight_ready must be true")
		}
		if preflight.ExecutorConfigSHA256 != chain.executorConfigSHA256 {
			failures = append(failures, "preflight executor_config_sha256 mismatch")
		}
		chain.dryRunSHA256 = preflight.DryRunSHA256
		chain.dryRunReportSHA256 = preflight.DryRunReportSHA256
		chain.executionBundleSHA256 = preflight.ExecutionBundleSHA256
		chain.providerPayloadSHA256 = preflight.ProviderPayloadSHA256
		chain.materializedSHA256 = preflight.MaterializedSHA256
		warnings = mergeWarnings(warnings, preflight.Warnings)
		if preflight.ExecutionAllowedNow || preflight.TransportCalled || preflight.ProviderCall || preflight.NetworkCall || preflight.WorkerExecution || preflight.SentToProvider {
			failures = append(failures, "preflight must keep execution flags blocked")
		}
	}

	dispatchApproval, dispatchData, err := loadProviderCallExecutorDispatchApproval(dispatchApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.dispatchApprovalSHA256 = sha256Hex(dispatchData)
		if err := dispatchApproval.Validate(); err != nil {
			failures = append(failures, err.Error())
		}
		if dispatchApproval.PreflightSHA256 != chain.preflightSHA256 {
			failures = append(failures, "dispatch approval preflight_sha256 mismatch")
		}
	}

	transportPlan, transportData, err := loadProviderTransportPlan(transportPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.transportPlanSHA256 = sha256Hex(transportData)
		if !transportPlan.TransportPlanReady {
			failures = append(failures, "transport plan transport_plan_ready must be true")
		}
		if transportPlan.TransportEnabled || transportPlan.TransportCalled || transportPlan.ProviderCall || transportPlan.NetworkCall || transportPlan.SentToProvider {
			failures = append(failures, "transport plan must keep transport flags blocked")
		}
		if transportPlan.ExecutorConfigSHA256 != chain.executorConfigSHA256 {
			failures = append(failures, "transport plan executor_config_sha256 mismatch")
		}
		if transportPlan.PreflightSHA256 != chain.preflightSHA256 {
			failures = append(failures, "transport plan preflight_sha256 mismatch")
		}
		if transportPlan.DispatchApprovalSHA256 != chain.dispatchApprovalSHA256 {
			failures = append(failures, "transport plan dispatch_approval_sha256 mismatch")
		}
	}

	dryRunData, err := readArtifactBytesNoTextExcerpt("dry-run", dryRunPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if chain.dryRunSHA256 != "" && sha256Hex(dryRunData) != chain.dryRunSHA256 {
		failures = append(failures, "dry-run sha256 mismatch with preflight")
	}

	dryRunReportData, err := readArtifactBytesNoTextExcerpt("dry-run report", dryRunReportPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if chain.dryRunReportSHA256 != "" && sha256Hex(dryRunReportData) != chain.dryRunReportSHA256 {
		failures = append(failures, "dry-run report sha256 mismatch with preflight")
	}

	bundleData, err := readArtifactBytesNoTextExcerpt("execution bundle", executionBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if chain.executionBundleSHA256 != "" && sha256Hex(bundleData) != chain.executionBundleSHA256 {
		failures = append(failures, "execution bundle sha256 mismatch with preflight")
	}

	return chain, failures, warnings, nil
}

func loadProviderTransportPlan(path string) (ProviderTransportPlanResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("transport plan", path)
	if err != nil {
		return ProviderTransportPlanResult{}, nil, err
	}
	result, err := ParseProviderTransportPlanJSON(data)
	if err != nil {
		return ProviderTransportPlanResult{}, nil, err
	}
	return result, data, nil
}

func ParseProviderExecutorReleaseBundleJSON(data []byte) (ProviderExecutorReleaseBundleResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderExecutorReleaseBundleResult{}, fmt.Errorf("release bundle must not contain materialized preview text")
	}
	var result ProviderExecutorReleaseBundleResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderExecutorReleaseBundleResult{}, fmt.Errorf("parse release bundle json: %w", err)
	}
	return result, nil
}

func writeProviderExecutorReleaseBundleJSON(path string, result ProviderExecutorReleaseBundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create release bundle output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal release bundle json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("release bundle must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderExecutorReleaseBundleText(result ProviderExecutorReleaseBundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_executor_release_bundle:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"release_bundle_ready", fmt.Sprintf("%t", result.ReleaseBundleReady)},
		{"activation_allowed_now", fmt.Sprintf("%t", result.ActivationAllowedNow)},
		{"execution_supported_now", fmt.Sprintf("%t", result.ExecutionSupportedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
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
	return nil
}

func WriteProviderExecutorReleaseBundleJSON(result ProviderExecutorReleaseBundleResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal release bundle json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("release bundle json must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}

func WriteProviderExecutorReleaseGateText(result ProviderExecutorReleaseGateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_executor_release_gate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"activation_gate_ready", fmt.Sprintf("%t", result.ActivationGateReady)},
		{"release_bundle_ready", fmt.Sprintf("%t", result.ReleaseBundleReady)},
		{"activation_allowed_now", fmt.Sprintf("%t", result.ActivationAllowedNow)},
		{"execution_supported_now", fmt.Sprintf("%t", result.ExecutionSupportedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
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
	return nil
}

func WriteProviderExecutorReleaseGateJSON(result ProviderExecutorReleaseGateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal release gate json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("release gate json must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}
