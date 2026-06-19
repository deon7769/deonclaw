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

type ProviderRequestEnvelopeDryRunOptions struct {
	ExecutorConfigPath    string
	ExecutionBundlePath   string
	ExecutorPreflightPath string
	DispatchApprovalPath  string
	TransportPlanPath     string
	ReleaseBundlePath     string
	ReleaseGatePath       string
	OutputPath            string
}

type ProviderRequestEnvelopeResult struct {
	Status                    string   `json:"status"`
	ProviderRequestReady      bool     `json:"provider_request_ready"`
	RequestContainsText       bool     `json:"request_contains_text"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToProvider            bool     `json:"sent_to_provider"`
	TransportCalled           bool     `json:"transport_called"`
	PromptInjectionRealRunner bool     `json:"prompt_injection_real_runner"`
	ExecutionSupportedNow     bool     `json:"execution_supported_now"`
	ActivationAllowedNow      bool     `json:"activation_allowed_now"`
	BlockedReason             string   `json:"blocked_reason"`
	Provider                  string   `json:"provider"`
	ExecutorConfigSHA256      string   `json:"executor_config_sha256"`
	ExecutionBundleSHA256     string   `json:"execution_bundle_sha256"`
	ProviderPayloadSHA256     string   `json:"provider_payload_sha256"`
	MaterializedSHA256        string   `json:"materialized_sha256,omitempty"`
	ReleaseBundleSHA256       string   `json:"release_bundle_sha256"`
	DispatchApprovalSHA256    string   `json:"dispatch_approval_sha256"`
	PreflightSHA256           string   `json:"preflight_sha256"`
	TransportPlanSHA256       string   `json:"transport_plan_sha256"`
	ReleaseGateSHA256         string   `json:"release_gate_sha256"`
	RequestEnvelopeSHA256     string   `json:"request_envelope_sha256,omitempty"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

type ProviderRequestEnvelopeReportOptions struct {
	RequestEnvelopePath   string
	ExecutorConfigPath    string
	ExecutionBundlePath   string
	ExecutorPreflightPath string
	DispatchApprovalPath  string
	TransportPlanPath     string
	ReleaseBundlePath     string
	ReleaseGatePath       string
}

func ProviderRequestEnvelopeDryRun(opts ProviderRequestEnvelopeDryRunOptions) (ProviderRequestEnvelopeResult, error) {
	chain, failures, warnings, err := loadProviderRequestEnvelopeChain(opts.ExecutorConfigPath, opts.ExecutionBundlePath, opts.ExecutorPreflightPath, opts.DispatchApprovalPath, opts.TransportPlanPath, opts.ReleaseBundlePath, opts.ReleaseGatePath)
	if err != nil {
		return ProviderRequestEnvelopeResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderRequestEnvelopeResult{}, err
	}

	result := providerRequestEnvelopeResultFromChain(chain, failures, warnings)
	if len(failures) == 0 {
		result.ProviderRequestReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
		if err := writeProviderRequestEnvelopeJSON(opts.OutputPath, result); err != nil {
			return ProviderRequestEnvelopeResult{}, err
		}
		data, err := readArtifactBytesNoTextExcerpt("request envelope", opts.OutputPath)
		if err != nil {
			return ProviderRequestEnvelopeResult{}, err
		}
		result.RequestEnvelopeSHA256 = sha256Hex(data)
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func ProviderRequestEnvelopeReport(opts ProviderRequestEnvelopeReportOptions) (ProviderRequestEnvelopeResult, error) {
	envelope, envelopeData, err := LoadProviderRequestEnvelope(opts.RequestEnvelopePath)
	if err != nil {
		return ProviderRequestEnvelopeResult{}, err
	}
	chain, failures, warnings, err := loadProviderRequestEnvelopeChain(opts.ExecutorConfigPath, opts.ExecutionBundlePath, opts.ExecutorPreflightPath, opts.DispatchApprovalPath, opts.TransportPlanPath, opts.ReleaseBundlePath, opts.ReleaseGatePath)
	if err != nil {
		return ProviderRequestEnvelopeResult{}, err
	}

	result := providerRequestEnvelopeResultFromChain(chain, failures, warnings)
	result.RequestEnvelopeSHA256 = sha256Hex(envelopeData)

	if !envelope.ProviderRequestReady {
		failures = append(failures, "envelope provider_request_ready must be true")
	}
	if envelope.RequestContainsText || envelope.SentToProvider || envelope.ProviderCall || envelope.NetworkCall || envelope.TransportCalled {
		failures = append(failures, "envelope must keep execution flags blocked")
	}
	reconcileProviderExecutorHash("envelope", envelope.ExecutorConfigSHA256, result.ExecutorConfigSHA256, &failures)
	reconcileProviderExecutorHash("envelope", envelope.ExecutionBundleSHA256, result.ExecutionBundleSHA256, &failures)
	reconcileProviderExecutorHash("envelope", envelope.ProviderPayloadSHA256, result.ProviderPayloadSHA256, &failures)
	reconcileProviderExecutorHash("envelope", envelope.ReleaseBundleSHA256, result.ReleaseBundleSHA256, &failures)
	reconcileProviderExecutorHash("envelope", envelope.DispatchApprovalSHA256, result.DispatchApprovalSHA256, &failures)

	result.Failures = failures
	result.Warnings = warnings
	if len(failures) == 0 {
		result.ProviderRequestReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

type providerRequestEnvelopeChain struct {
	provider               string
	executorConfigSHA256   string
	executionBundleSHA256  string
	providerPayloadSHA256  string
	materializedSHA256     string
	preflightSHA256        string
	dispatchApprovalSHA256 string
	transportPlanSHA256    string
	releaseBundleSHA256    string
	releaseGateSHA256      string
}

func loadProviderRequestEnvelopeChain(executorConfigPath, executionBundlePath, preflightPath, dispatchApprovalPath, transportPlanPath, releaseBundlePath, releaseGatePath string) (providerRequestEnvelopeChain, []string, []string, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"executor config path", executorConfigPath},
		{"execution bundle path", executionBundlePath},
		{"executor preflight path", preflightPath},
		{"dispatch approval path", dispatchApprovalPath},
		{"transport plan path", transportPlanPath},
		{"release bundle path", releaseBundlePath},
		{"release gate path", releaseGatePath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return providerRequestEnvelopeChain{}, nil, nil, err
		}
	}

	var chain providerRequestEnvelopeChain
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
		} else {
			chain.provider = cfg.ProviderCallExecutor.Provider
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
		chain.providerPayloadSHA256 = preflight.ProviderPayloadSHA256
		chain.materializedSHA256 = preflight.MaterializedSHA256
		chain.executionBundleSHA256 = preflight.ExecutionBundleSHA256
		warnings = mergeWarnings(warnings, preflight.Warnings)
	}

	bundleData, err := readArtifactBytesNoTextExcerpt("execution bundle", executionBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else if chain.executionBundleSHA256 != "" && sha256Hex(bundleData) != chain.executionBundleSHA256 {
		failures = append(failures, "execution bundle sha256 mismatch with preflight")
	}

	dispatchApproval, dispatchData, err := loadProviderCallExecutorDispatchApproval(dispatchApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.dispatchApprovalSHA256 = sha256Hex(dispatchData)
		if err := dispatchApproval.Validate(); err != nil {
			failures = append(failures, err.Error())
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
	}

	releaseBundle, releaseBundleData, err := loadProviderExecutorReleaseBundle(releaseBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.releaseBundleSHA256 = sha256Hex(releaseBundleData)
		if !releaseBundle.ReleaseBundleReady {
			failures = append(failures, "release bundle release_bundle_ready must be true")
		}
		if releaseBundle.ProviderPayloadSHA256 != "" && chain.providerPayloadSHA256 != "" && releaseBundle.ProviderPayloadSHA256 != chain.providerPayloadSHA256 {
			failures = append(failures, "release bundle provider_payload_sha256 mismatch")
		}
	}

	releaseGate, releaseGateData, err := LoadProviderExecutorReleaseGate(releaseGatePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.releaseGateSHA256 = sha256Hex(releaseGateData)
		if !releaseGate.ActivationGateReady {
			failures = append(failures, "release gate activation_gate_ready must be true")
		}
		if releaseGate.ActivationAllowedNow || releaseGate.ExecutionSupportedNow {
			failures = append(failures, "release gate must keep activation flags blocked")
		}
	}

	return chain, failures, warnings, nil
}

func providerRequestEnvelopeResultFromChain(chain providerRequestEnvelopeChain, failures, warnings []string) ProviderRequestEnvelopeResult {
	return ProviderRequestEnvelopeResult{
		Status:                    lancedbpolicy.StatusOK,
		RequestContainsText:       false,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		SentToProvider:            false,
		TransportCalled:           false,
		PromptInjectionRealRunner: false,
		ExecutionSupportedNow:     false,
		ActivationAllowedNow:      false,
		BlockedReason:             ProviderCallExecutorBlockedReason,
		Provider:                  chain.provider,
		ExecutorConfigSHA256:      chain.executorConfigSHA256,
		ExecutionBundleSHA256:     chain.executionBundleSHA256,
		ProviderPayloadSHA256:     chain.providerPayloadSHA256,
		MaterializedSHA256:        chain.materializedSHA256,
		ReleaseBundleSHA256:       chain.releaseBundleSHA256,
		DispatchApprovalSHA256:    chain.dispatchApprovalSHA256,
		PreflightSHA256:           chain.preflightSHA256,
		TransportPlanSHA256:       chain.transportPlanSHA256,
		ReleaseGateSHA256:         chain.releaseGateSHA256,
		Warnings:                  warnings,
		Failures:                  failures,
	}
}

func LoadProviderRequestEnvelope(path string) (ProviderRequestEnvelopeResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("request envelope", path)
	if err != nil {
		return ProviderRequestEnvelopeResult{}, nil, err
	}
	var result ProviderRequestEnvelopeResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderRequestEnvelopeResult{}, nil, fmt.Errorf("parse request envelope json: %w", err)
	}
	return result, data, nil
}

func loadProviderExecutorReleaseBundle(path string) (ProviderExecutorReleaseBundleResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("release bundle", path)
	if err != nil {
		return ProviderExecutorReleaseBundleResult{}, nil, err
	}
	result, err := ParseProviderExecutorReleaseBundleJSON(data)
	if err != nil {
		return ProviderExecutorReleaseBundleResult{}, nil, err
	}
	return result, data, nil
}

func LoadProviderExecutorReleaseGate(path string) (ProviderExecutorReleaseGateResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("release gate", path)
	if err != nil {
		return ProviderExecutorReleaseGateResult{}, nil, err
	}
	var result ProviderExecutorReleaseGateResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderExecutorReleaseGateResult{}, nil, fmt.Errorf("parse release gate json: %w", err)
	}
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderExecutorReleaseGateResult{}, nil, fmt.Errorf("release gate must not contain materialized preview text")
	}
	return result, data, nil
}

func writeProviderRequestEnvelopeJSON(path string, result ProviderRequestEnvelopeResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create request envelope output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal request envelope json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("request envelope must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderRequestEnvelopeText(result ProviderRequestEnvelopeResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_request_envelope:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"provider_request_ready", fmt.Sprintf("%t", result.ProviderRequestReady)},
		{"request_contains_text", fmt.Sprintf("%t", result.RequestContainsText)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
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

func WriteProviderRequestEnvelopeJSON(result ProviderRequestEnvelopeResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal request envelope json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("request envelope json must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}
