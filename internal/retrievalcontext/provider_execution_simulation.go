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

type ProviderExecutionSimulationBundleOptions struct {
	RequestEnvelopePath string
	AdapterPlanPath     string
	ResponseFixturePath string
	ReleaseGatePath     string
	ExecutorConfigPath  string
	OutputPath          string
}

type ProviderExecutionSimulationBundleResult struct {
	Status                   string   `json:"status"`
	SimulationBundleReady    bool     `json:"simulation_bundle_ready"`
	SimulatedResponseReady   bool     `json:"simulated_response_ready"`
	ExecutionResultAvailable bool     `json:"execution_result_available"`
	ProviderCall             bool     `json:"provider_call"`
	NetworkCall              bool     `json:"network_call"`
	TransportCalled          bool     `json:"transport_called"`
	SentToProvider           bool     `json:"sent_to_provider"`
	ReceivedFromProvider     bool     `json:"received_from_provider"`
	WorkerExecution          bool     `json:"worker_execution"`
	ExecutionSupportedNow    bool     `json:"execution_supported_now"`
	ActivationAllowedNow     bool     `json:"activation_allowed_now"`
	BlockedReason            string   `json:"blocked_reason"`
	Provider                 string   `json:"provider"`
	ExecutorConfigSHA256     string   `json:"executor_config_sha256"`
	RequestEnvelopeSHA256    string   `json:"request_envelope_sha256"`
	AdapterPlanSHA256        string   `json:"adapter_plan_sha256"`
	ResponseFixtureSHA256    string   `json:"response_fixture_sha256"`
	ReleaseGateSHA256        string   `json:"release_gate_sha256"`
	ProviderPayloadSHA256    string   `json:"provider_payload_sha256,omitempty"`
	ResponseFixtureID        string   `json:"response_fixture_id"`
	Warnings                 []string `json:"warnings,omitempty"`
	Failures                 []string `json:"failures,omitempty"`
}

type ProviderExecutionSimulationReportOptions struct {
	SimulationBundlePath string
	RequestEnvelopePath  string
	AdapterPlanPath      string
	ResponseFixturePath  string
	ReleaseGatePath      string
	ExecutorConfigPath   string
}

func ProviderExecutionSimulationBundle(opts ProviderExecutionSimulationBundleOptions) (ProviderExecutionSimulationBundleResult, error) {
	chain, failures, err := loadProviderExecutionSimulationChain(opts.RequestEnvelopePath, opts.AdapterPlanPath, opts.ResponseFixturePath, opts.ReleaseGatePath, opts.ExecutorConfigPath)
	if err != nil {
		return ProviderExecutionSimulationBundleResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderExecutionSimulationBundleResult{}, err
	}

	result := providerExecutionSimulationBundleFromChain(chain, failures)
	if len(failures) == 0 {
		result.SimulationBundleReady = true
		result.SimulatedResponseReady = true
		if err := writeProviderExecutionSimulationBundleJSON(opts.OutputPath, result); err != nil {
			return ProviderExecutionSimulationBundleResult{}, err
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func ProviderExecutionSimulationReport(opts ProviderExecutionSimulationReportOptions) (ProviderExecutionSimulationBundleResult, error) {
	if err := validateRelativeSafePath("simulation bundle path", opts.SimulationBundlePath); err != nil {
		return ProviderExecutionSimulationBundleResult{}, err
	}
	bundleData, err := readArtifactBytesNoTextExcerpt("simulation bundle", opts.SimulationBundlePath)
	if err != nil {
		return ProviderExecutionSimulationBundleResult{}, err
	}
	var stored ProviderExecutionSimulationBundleResult
	if err := json.Unmarshal(bundleData, &stored); err != nil {
		return ProviderExecutionSimulationBundleResult{}, fmt.Errorf("parse simulation bundle json: %w", err)
	}

	chain, failures, err := loadProviderExecutionSimulationChain(opts.RequestEnvelopePath, opts.AdapterPlanPath, opts.ResponseFixturePath, opts.ReleaseGatePath, opts.ExecutorConfigPath)
	if err != nil {
		return ProviderExecutionSimulationBundleResult{}, err
	}

	result := providerExecutionSimulationBundleFromChain(chain, failures)
	if !stored.SimulationBundleReady {
		failures = append(failures, "stored simulation bundle simulation_bundle_ready must be true")
	}
	reconcileProviderExecutorHash("simulation bundle", stored.RequestEnvelopeSHA256, result.RequestEnvelopeSHA256, &failures)
	reconcileProviderExecutorHash("simulation bundle", stored.AdapterPlanSHA256, result.AdapterPlanSHA256, &failures)
	reconcileProviderExecutorHash("simulation bundle", stored.ResponseFixtureSHA256, result.ResponseFixtureSHA256, &failures)
	reconcileProviderExecutorHash("simulation bundle", stored.ReleaseGateSHA256, result.ReleaseGateSHA256, &failures)
	reconcileProviderExecutorHash("simulation bundle", stored.ExecutorConfigSHA256, result.ExecutorConfigSHA256, &failures)
	if stored.ProviderCall || stored.NetworkCall || stored.TransportCalled || stored.SentToProvider || stored.ReceivedFromProvider || stored.WorkerExecution {
		failures = append(failures, "stored simulation bundle must keep execution flags blocked")
	}

	result.Failures = failures
	if len(failures) == 0 {
		result.SimulationBundleReady = true
		result.SimulatedResponseReady = true
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

type providerExecutionSimulationChain struct {
	provider              string
	executorConfigSHA256  string
	requestEnvelopeSHA256 string
	adapterPlanSHA256     string
	responseFixtureSHA256 string
	releaseGateSHA256     string
	providerPayloadSHA256 string
	responseFixtureID     string
}

func loadProviderExecutionSimulationChain(requestEnvelopePath, adapterPlanPath, responseFixturePath, releaseGatePath, executorConfigPath string) (providerExecutionSimulationChain, []string, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"request envelope path", requestEnvelopePath},
		{"adapter plan path", adapterPlanPath},
		{"response fixture path", responseFixturePath},
		{"release gate path", releaseGatePath},
		{"executor config path", executorConfigPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return providerExecutionSimulationChain{}, nil, err
		}
	}

	var chain providerExecutionSimulationChain
	var failures []string

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

	envelope, envelopeData, err := LoadProviderRequestEnvelope(requestEnvelopePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.requestEnvelopeSHA256 = sha256Hex(envelopeData)
		if !envelope.ProviderRequestReady {
			failures = append(failures, "request envelope provider_request_ready must be true")
		}
		chain.providerPayloadSHA256 = envelope.ProviderPayloadSHA256
	}

	adapterPlan, adapterData, err := LoadProviderAdapterPlan(adapterPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.adapterPlanSHA256 = sha256Hex(adapterData)
		if !adapterPlan.AdapterPlanReady {
			failures = append(failures, "adapter plan adapter_plan_ready must be true")
		}
		if adapterPlan.ProviderCall || adapterPlan.NetworkCall || adapterPlan.TransportCalled {
			failures = append(failures, "adapter plan must keep transport flags blocked")
		}
	}

	responseFixture, responseData, err := LoadProviderResponseFixture(responseFixturePath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		chain.responseFixtureSHA256 = sha256Hex(responseData)
		chain.responseFixtureID = responseFixture.ResponseFixtureID
		if !responseFixture.ResponseFixtureReady {
			failures = append(failures, "response fixture response_fixture_ready must be true")
		}
		if responseFixture.ReceivedFromProvider || responseFixture.ProviderCall || responseFixture.NetworkCall {
			failures = append(failures, "response fixture must keep provider flags blocked")
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
	}

	return chain, failures, nil
}

func providerExecutionSimulationBundleFromChain(chain providerExecutionSimulationChain, failures []string) ProviderExecutionSimulationBundleResult {
	return ProviderExecutionSimulationBundleResult{
		Status:                   lancedbpolicy.StatusOK,
		ExecutionResultAvailable: false,
		ProviderCall:             false,
		NetworkCall:              false,
		TransportCalled:          false,
		SentToProvider:           false,
		ReceivedFromProvider:     false,
		WorkerExecution:          false,
		ExecutionSupportedNow:    false,
		ActivationAllowedNow:     false,
		BlockedReason:            ProviderCallExecutorBlockedReason,
		Provider:                 chain.provider,
		ExecutorConfigSHA256:     chain.executorConfigSHA256,
		RequestEnvelopeSHA256:    chain.requestEnvelopeSHA256,
		AdapterPlanSHA256:        chain.adapterPlanSHA256,
		ResponseFixtureSHA256:    chain.responseFixtureSHA256,
		ReleaseGateSHA256:        chain.releaseGateSHA256,
		ProviderPayloadSHA256:    chain.providerPayloadSHA256,
		ResponseFixtureID:        chain.responseFixtureID,
		Failures:                 failures,
	}
}

func writeProviderExecutionSimulationBundleJSON(path string, result ProviderExecutionSimulationBundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create simulation bundle output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal simulation bundle json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("simulation bundle must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderExecutionSimulationBundleText(result ProviderExecutionSimulationBundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_execution_simulation_bundle:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"simulation_bundle_ready", fmt.Sprintf("%t", result.SimulationBundleReady)},
		{"simulated_response_ready", fmt.Sprintf("%t", result.SimulatedResponseReady)},
		{"execution_result_available", fmt.Sprintf("%t", result.ExecutionResultAvailable)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"received_from_provider", fmt.Sprintf("%t", result.ReceivedFromProvider)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
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

func WriteProviderExecutionSimulationBundleJSON(result ProviderExecutionSimulationBundleResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal simulation bundle json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderExecutionSimulationReportText(result ProviderExecutionSimulationBundleResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_execution_simulation_report:"); err != nil {
		return err
	}
	return WriteProviderExecutionSimulationBundleText(result, out)
}

func WriteProviderExecutionSimulationReportJSON(result ProviderExecutionSimulationBundleResult, out io.Writer) error {
	return WriteProviderExecutionSimulationBundleJSON(result, out)
}
