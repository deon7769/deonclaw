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

type ProviderTransportPlanResult struct {
	Status                 string   `json:"status"`
	TransportPlanReady     bool     `json:"transport_plan_ready"`
	TransportEnabled       bool     `json:"transport_enabled"`
	TransportCalled        bool     `json:"transport_called"`
	ProviderCall           bool     `json:"provider_call"`
	NetworkCall            bool     `json:"network_call"`
	SentToProvider         bool     `json:"sent_to_provider"`
	BlockedReason          string   `json:"blocked_reason"`
	Provider               string   `json:"provider"`
	TransportMode          string   `json:"transport_mode"`
	ExecutorConfigSHA256   string   `json:"executor_config_sha256"`
	PreflightSHA256        string   `json:"preflight_sha256"`
	DispatchApprovalSHA256 string   `json:"dispatch_approval_sha256"`
	Warnings               []string `json:"warnings,omitempty"`
	Failures               []string `json:"failures,omitempty"`
}

type ProviderTransportPlanOptions struct {
	ExecutorConfigPath    string
	ExecutorPreflightPath string
	DispatchApprovalPath  string
	OutputPath            string
}

func ProviderTransportPlan(opts ProviderTransportPlanOptions) (ProviderTransportPlanResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"executor config path", opts.ExecutorConfigPath},
		{"executor preflight path", opts.ExecutorPreflightPath},
		{"dispatch approval path", opts.DispatchApprovalPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderTransportPlanResult{}, err
		}
	}

	cfgData, err := readArtifactBytesNoTextExcerpt("executor config", opts.ExecutorConfigPath)
	if err != nil {
		return ProviderTransportPlanResult{}, err
	}
	cfg, err := ParseProviderCallExecutorConfig(cfgData)
	if err != nil {
		return ProviderTransportPlanResult{}, err
	}
	if err := ValidateProviderCallExecutorConfig(cfg); err != nil {
		return ProviderTransportPlanResult{}, err
	}

	preflight, preflightData, err := LoadProviderCallExecutorPreflight(opts.ExecutorPreflightPath)
	if err != nil {
		return ProviderTransportPlanResult{}, err
	}
	if !preflight.ExecutorPreflightReady {
		return ProviderTransportPlanResult{}, fmt.Errorf("executor preflight executor_preflight_ready must be true")
	}

	dispatchApproval, dispatchData, err := loadProviderCallExecutorDispatchApproval(opts.DispatchApprovalPath)
	if err != nil {
		return ProviderTransportPlanResult{}, err
	}
	if err := dispatchApproval.Validate(); err != nil {
		return ProviderTransportPlanResult{}, err
	}

	executorSHA := sha256Hex(cfgData)
	preflightSHA := sha256Hex(preflightData)
	dispatchSHA := sha256Hex(dispatchData)

	var failures []string
	if preflight.ExecutorConfigSHA256 != executorSHA {
		failures = append(failures, "executor_config_sha256 mismatch with preflight")
	}
	if dispatchApproval.ExecutorConfigSHA256 != executorSHA {
		failures = append(failures, "executor_config_sha256 mismatch with dispatch approval")
	}
	if dispatchApproval.PreflightSHA256 != preflightSHA {
		failures = append(failures, "preflight_sha256 mismatch with dispatch approval")
	}
	if !dispatchApproval.DispatchAuthorizedForFuture {
		failures = append(failures, "dispatch_authorized_for_future must be true")
	}

	result := ProviderTransportPlanResult{
		Status:                 lancedbpolicy.StatusOK,
		TransportPlanReady:     len(failures) == 0,
		TransportEnabled:       false,
		TransportCalled:        false,
		ProviderCall:           false,
		NetworkCall:            false,
		SentToProvider:         false,
		BlockedReason:          ProviderCallExecutorBlockedReason,
		Provider:               cfg.ProviderCallExecutor.Provider,
		TransportMode:          "BlockedProviderTransport",
		ExecutorConfigSHA256:   executorSHA,
		PreflightSHA256:        preflightSHA,
		DispatchApprovalSHA256: dispatchSHA,
		Warnings:               append([]string(nil), preflight.Warnings...),
		Failures:               failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
		result.TransportPlanReady = false
	}
	if err := writeProviderTransportPlanJSON(opts.OutputPath, result); err != nil {
		return ProviderTransportPlanResult{}, err
	}
	return result, nil
}

func LoadProviderTransportPlan(path string) (ProviderTransportPlanResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderTransportPlanResult{}, fmt.Errorf("read transport plan %q: %w", path, err)
	}
	return ParseProviderTransportPlanJSON(data)
}

func ParseProviderTransportPlanJSON(data []byte) (ProviderTransportPlanResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderTransportPlanResult{}, fmt.Errorf("transport plan must not contain materialized preview text")
	}
	var result ProviderTransportPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderTransportPlanResult{}, fmt.Errorf("parse transport plan json: %w", err)
	}
	return result, nil
}

func loadProviderCallExecutorDispatchApproval(path string) (ProviderCallExecutorDispatchApproval, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("dispatch approval", path)
	if err != nil {
		return ProviderCallExecutorDispatchApproval{}, nil, err
	}
	approval, err := ParseProviderCallExecutorDispatchApprovalJSON(data)
	if err != nil {
		return ProviderCallExecutorDispatchApproval{}, nil, err
	}
	return approval, data, nil
}

func writeProviderTransportPlanJSON(path string, result ProviderTransportPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create transport plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal transport plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("transport plan must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderTransportPlanText(result ProviderTransportPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_transport_plan:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"transport_plan_ready", fmt.Sprintf("%t", result.TransportPlanReady)},
		{"transport_enabled", fmt.Sprintf("%t", result.TransportEnabled)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"blocked_reason", result.BlockedReason},
		{"provider", result.Provider},
		{"transport_mode", result.TransportMode},
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
