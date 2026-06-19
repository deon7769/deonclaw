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

type ProviderCallExecutorDryRunOptions struct {
	ProviderCallExecutorOptions
	OutputPath string
}

type ProviderCallExecutorDryRunResult struct {
	Status                          string   `json:"status"`
	ExecutorDryRunReady             bool     `json:"executor_dry_run_ready"`
	ExecutorPolicyValidated         bool     `json:"executor_policy_validated"`
	ExecutorConfigValidated         bool     `json:"executor_config_validated"`
	ExecutionSupportedNow           bool     `json:"execution_supported_now"`
	ProviderCallAuthorizedForFuture bool     `json:"provider_call_authorized_for_future"`
	ChainContinuityReady            bool     `json:"chain_continuity_ready"`
	TransportCalled                 bool     `json:"transport_called"`
	ContainsText                    bool     `json:"contains_text"`
	PreviewOnly                     bool     `json:"preview_only"`
	ProviderCallAllowedNow          bool     `json:"provider_call_allowed_now"`
	ProviderCall                    bool     `json:"provider_call"`
	NetworkCall                     bool     `json:"network_call"`
	WorkerExecution                 bool     `json:"worker_execution"`
	SentToProvider                  bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner       bool     `json:"prompt_injection_real_runner"`
	BlockedReason                   string   `json:"blocked_reason"`
	ExecutorConfigSHA256            string   `json:"executor_config_sha256"`
	ExecutionBundleSHA256           string   `json:"execution_bundle_sha256"`
	ApprovalSHA256                  string   `json:"approval_sha256"`
	ProviderPayloadSHA256           string   `json:"provider_payload_sha256"`
	MaterializedSHA256              string   `json:"materialized_sha256,omitempty"`
	DryRunSteps                     []string `json:"dry_run_steps,omitempty"`
	Warnings                        []string `json:"warnings,omitempty"`
	Failures                        []string `json:"failures,omitempty"`
}

type ProviderCallExecutorDryRunReportOptions struct {
	DryRunPath         string
	ExecutorConfigPath string
}

type ProviderCallExecutorDryRunReportResult struct {
	Status                          string   `json:"status"`
	ExecutorDryRunValidated         bool     `json:"executor_dry_run_validated"`
	ExecutorPolicyValidated         bool     `json:"executor_policy_validated"`
	ProviderCallAuthorizedForFuture bool     `json:"provider_call_authorized_for_future"`
	ChainContinuityReady            bool     `json:"chain_continuity_ready"`
	TransportCalled                 bool     `json:"transport_called"`
	ContainsText                    bool     `json:"contains_text"`
	ProviderCall                    bool     `json:"provider_call"`
	NetworkCall                     bool     `json:"network_call"`
	WorkerExecution                 bool     `json:"worker_execution"`
	SentToProvider                  bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner       bool     `json:"prompt_injection_real_runner"`
	BlockedReason                   string   `json:"blocked_reason"`
	ExecutorConfigSHA256            string   `json:"executor_config_sha256"`
	ExecutionBundleSHA256           string   `json:"execution_bundle_sha256"`
	ApprovalSHA256                  string   `json:"approval_sha256"`
	ProviderPayloadSHA256           string   `json:"provider_payload_sha256"`
	MaterializedSHA256              string   `json:"materialized_sha256,omitempty"`
	Warnings                        []string `json:"warnings,omitempty"`
	Failures                        []string `json:"failures,omitempty"`
}

func ProviderCallExecutorDryRun(opts ProviderCallExecutorDryRunOptions) (ProviderCallExecutorDryRunResult, error) {
	if err := validateRelativeSafePath("dry-run output path", opts.OutputPath); err != nil {
		return ProviderCallExecutorDryRunResult{}, err
	}
	core, err := providerCallExecutorCore(opts.ProviderCallExecutorOptions, providerCallExecutorCoreMode{skipPayloadOutput: true})
	if err != nil {
		return ProviderCallExecutorDryRunResult{}, err
	}
	result := providerCallExecutorDryRunFromCore(core)
	if core.Status != lancedbpolicy.StatusFailed && core.ExecutorConfigValidated {
		result.ExecutorDryRunReady = true
		result.DryRunSteps = []string{
			"validate_executor_policy",
			"load_execution_bundle",
			"run_chain_continuity_audit_metadata_only",
			"inspect_provider_call_approval",
			"reconcile_hashes_and_policy_flags",
			"transport_not_called",
			"blocked: implementation_not_enabled",
		}
		if err := writeProviderCallExecutorDryRunJSON(opts.OutputPath, result); err != nil {
			return ProviderCallExecutorDryRunResult{}, err
		}
	}
	return result, nil
}

func ProviderCallExecutorDryRunReport(opts ProviderCallExecutorDryRunReportOptions) (ProviderCallExecutorDryRunReportResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"dry-run path", opts.DryRunPath},
		{"executor config path", opts.ExecutorConfigPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallExecutorDryRunReportResult{}, err
		}
	}

	dryRun, dryRunData, err := LoadProviderCallExecutorDryRun(opts.DryRunPath)
	if err != nil {
		return ProviderCallExecutorDryRunReportResult{}, err
	}

	result := ProviderCallExecutorDryRunReportResult{
		Status:                          lancedbpolicy.StatusOK,
		ExecutorPolicyValidated:         dryRun.ExecutorPolicyValidated,
		ProviderCallAuthorizedForFuture: dryRun.ProviderCallAuthorizedForFuture,
		ChainContinuityReady:            dryRun.ChainContinuityReady,
		TransportCalled:                 dryRun.TransportCalled,
		ContainsText:                    dryRun.ContainsText,
		ProviderCall:                    dryRun.ProviderCall,
		NetworkCall:                     dryRun.NetworkCall,
		WorkerExecution:                 dryRun.WorkerExecution,
		SentToProvider:                  dryRun.SentToProvider,
		PromptInjectionRealRunner:       dryRun.PromptInjectionRealRunner,
		BlockedReason:                   dryRun.BlockedReason,
		ExecutorConfigSHA256:            dryRun.ExecutorConfigSHA256,
		ExecutionBundleSHA256:           dryRun.ExecutionBundleSHA256,
		ApprovalSHA256:                  dryRun.ApprovalSHA256,
		ProviderPayloadSHA256:           dryRun.ProviderPayloadSHA256,
		MaterializedSHA256:              dryRun.MaterializedSHA256,
	}

	var failures []string
	var warnings []string
	warnings = mergeWarnings(warnings, dryRun.Warnings)

	if dryRun.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, "dry-run status must be ok or warning")
		failures = append(failures, dryRun.Failures...)
	} else if dryRun.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"dry-run status is warning"})
	} else if dryRun.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("dry-run status %q must be ok or warning", dryRun.Status))
	}
	if !dryRun.ExecutorDryRunReady {
		failures = append(failures, "dry-run executor_dry_run_ready must be true")
	}
	if !dryRun.ExecutorPolicyValidated {
		failures = append(failures, "dry-run executor_policy_validated must be true")
	}
	if !dryRun.ExecutorConfigValidated {
		failures = append(failures, "dry-run executor_config_validated must be true")
	}
	if !dryRun.ProviderCallAuthorizedForFuture {
		failures = append(failures, "dry-run provider_call_authorized_for_future must be true")
	}
	if !dryRun.ChainContinuityReady {
		failures = append(failures, "dry-run chain_continuity_ready must be true")
	}
	if dryRun.TransportCalled {
		failures = append(failures, "dry-run transport_called must be false")
	}
	if dryRun.ContainsText {
		failures = append(failures, "dry-run contains_text must be false")
	}
	if !dryRun.PreviewOnly {
		failures = append(failures, "dry-run preview_only must be true")
	}
	if dryRun.ExecutionSupportedNow {
		failures = append(failures, "dry-run execution_supported_now must be false")
	}
	if dryRun.ProviderCall || dryRun.NetworkCall || dryRun.WorkerExecution || dryRun.SentToProvider || dryRun.PromptInjectionRealRunner {
		failures = append(failures, "dry-run must keep provider_call, network_call, worker_execution, sent_to_provider, and prompt_injection_real_runner false")
	}
	if dryRun.BlockedReason != ProviderCallExecutorBlockedReason {
		failures = append(failures, fmt.Sprintf("dry-run blocked_reason %q must be %q", dryRun.BlockedReason, ProviderCallExecutorBlockedReason))
	}
	if dryRun.ExecutorConfigSHA256 == "" || dryRun.ExecutionBundleSHA256 == "" || dryRun.ApprovalSHA256 == "" || dryRun.ProviderPayloadSHA256 == "" {
		failures = append(failures, "dry-run hash fields must be populated")
	}
	if strings.Contains(string(dryRunData), "text_excerpt") || strings.Contains(string(dryRunData), "alpha text") {
		failures = append(failures, "dry-run artifact must not contain materialized preview text")
	}

	executorCfgData, err := readArtifactBytesNoTextExcerpt("executor config", opts.ExecutorConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if sha256Hex(executorCfgData) != dryRun.ExecutorConfigSHA256 {
		failures = append(failures, "executor config sha256 mismatch with dry-run artifact")
	} else {
		executorCfg, err := ParseProviderCallExecutorConfig(executorCfgData)
		if err != nil {
			failures = append(failures, err.Error())
		} else if err := ValidateProviderCallExecutorConfig(executorCfg); err != nil {
			failures = append(failures, err.Error())
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ExecutorDryRunValidated = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func LoadProviderCallExecutorDryRun(path string) (ProviderCallExecutorDryRunResult, []byte, error) {
	if err := validateRelativeSafePath("dry-run path", path); err != nil {
		return ProviderCallExecutorDryRunResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("provider call executor dry-run", path)
	if err != nil {
		return ProviderCallExecutorDryRunResult{}, nil, err
	}
	var result ProviderCallExecutorDryRunResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderCallExecutorDryRunResult{}, nil, fmt.Errorf("parse provider call executor dry-run: %w", err)
	}
	return result, data, nil
}

func providerCallExecutorDryRunFromCore(core ProviderCallExecutorResult) ProviderCallExecutorDryRunResult {
	return ProviderCallExecutorDryRunResult{
		Status:                          core.Status,
		ExecutorPolicyValidated:         core.ExecutorPolicyValidated,
		ExecutorConfigValidated:         core.ExecutorConfigValidated,
		ExecutionSupportedNow:           false,
		ProviderCallAuthorizedForFuture: core.ProviderCallAuthorizedForFuture,
		ChainContinuityReady:            core.ChainContinuityReady,
		TransportCalled:                 false,
		ContainsText:                    false,
		PreviewOnly:                     true,
		ProviderCallAllowedNow:          false,
		ProviderCall:                    false,
		NetworkCall:                     false,
		WorkerExecution:                 false,
		SentToProvider:                  false,
		PromptInjectionRealRunner:       false,
		BlockedReason:                   ProviderCallExecutorBlockedReason,
		ExecutorConfigSHA256:            core.ExecutorConfigSHA256,
		ExecutionBundleSHA256:           core.ExecutionBundleSHA256,
		ApprovalSHA256:                  core.ApprovalSHA256,
		ProviderPayloadSHA256:           core.ProviderPayloadSHA256,
		MaterializedSHA256:              core.MaterializedSHA256,
		Warnings:                        core.Warnings,
		Failures:                        core.Failures,
	}
}

func writeProviderCallExecutorDryRunJSON(path string, result ProviderCallExecutorDryRunResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider call executor dry-run output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor dry-run json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider call executor dry-run must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider call executor dry-run %q: %w", path, err)
	}
	return nil
}

func WriteProviderCallExecutorDryRunText(result ProviderCallExecutorDryRunResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_executor_dry_run:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"executor_dry_run_ready", fmt.Sprintf("%t", result.ExecutorDryRunReady)},
		{"executor_policy_validated", fmt.Sprintf("%t", result.ExecutorPolicyValidated)},
		{"executor_config_validated", fmt.Sprintf("%t", result.ExecutorConfigValidated)},
		{"execution_supported_now", fmt.Sprintf("%t", result.ExecutionSupportedNow)},
		{"provider_call_authorized_for_future", fmt.Sprintf("%t", result.ProviderCallAuthorizedForFuture)},
		{"chain_continuity_ready", fmt.Sprintf("%t", result.ChainContinuityReady)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"preview_only", fmt.Sprintf("%t", result.PreviewOnly)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"prompt_injection_real_runner", fmt.Sprintf("%t", result.PromptInjectionRealRunner)},
		{"blocked_reason", result.BlockedReason},
		{"executor_config_sha256", result.ExecutorConfigSHA256},
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
	if len(result.DryRunSteps) > 0 {
		if _, err := fmt.Fprintln(out, "\ndry_run_steps:"); err != nil {
			return err
		}
		for _, step := range result.DryRunSteps {
			if _, err := fmt.Fprintf(out, "- %s\n", step); err != nil {
				return err
			}
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
		return fmt.Errorf("provider call executor dry-run text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallExecutorDryRunJSON(result ProviderCallExecutorDryRunResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor dry-run json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call executor dry-run json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}

func WriteProviderCallExecutorDryRunReportText(result ProviderCallExecutorDryRunReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_executor_dry_run_report:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"executor_dry_run_validated", fmt.Sprintf("%t", result.ExecutorDryRunValidated)},
		{"executor_policy_validated", fmt.Sprintf("%t", result.ExecutorPolicyValidated)},
		{"provider_call_authorized_for_future", fmt.Sprintf("%t", result.ProviderCallAuthorizedForFuture)},
		{"chain_continuity_ready", fmt.Sprintf("%t", result.ChainContinuityReady)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"contains_text", fmt.Sprintf("%t", result.ContainsText)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"prompt_injection_real_runner", fmt.Sprintf("%t", result.PromptInjectionRealRunner)},
		{"blocked_reason", result.BlockedReason},
		{"executor_config_sha256", result.ExecutorConfigSHA256},
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
		return fmt.Errorf("provider call executor dry-run report text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallExecutorDryRunReportJSON(result ProviderCallExecutorDryRunReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor dry-run report json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call executor dry-run report json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
