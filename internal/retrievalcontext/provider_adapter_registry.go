package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"gopkg.in/yaml.v3"
)

type ProviderAdaptersConfig struct {
	ProviderAdapters ProviderAdaptersPolicy `yaml:"provider_adapters" json:"provider_adapters"`
}

type ProviderAdaptersPolicy struct {
	Enabled       bool                             `yaml:"enabled" json:"enabled"`
	BlockedReason string                           `yaml:"blocked_reason" json:"blocked_reason"`
	Adapters      map[string]ProviderAdapterPolicy `yaml:"adapters" json:"adapters"`
}

type ProviderAdapterPolicy struct {
	Enabled                   bool   `yaml:"enabled" json:"enabled"`
	Provider                  string `yaml:"provider" json:"provider"`
	AdapterAvailableForFuture bool   `yaml:"adapter_available_for_future" json:"adapter_available_for_future"`
	AdapterEnabledNow         bool   `yaml:"adapter_enabled_now" json:"adapter_enabled_now"`
	TransportEnabled          bool   `yaml:"transport_enabled" json:"transport_enabled"`
	AllowProviderCall         bool   `yaml:"allow_provider_call" json:"allow_provider_call"`
	AllowNetwork              bool   `yaml:"allow_network" json:"allow_network"`
	BlockedReason             string `yaml:"blocked_reason" json:"blocked_reason"`
}

type ProviderAdapterRegistryValidateResult struct {
	Status                   string   `json:"status"`
	AdapterRegistryValidated bool     `json:"adapter_registry_validated"`
	Enabled                  bool     `json:"enabled"`
	BlockedReason            string   `json:"blocked_reason"`
	AdapterCount             int      `json:"adapter_count"`
	Warnings                 []string `json:"warnings,omitempty"`
	Failures                 []string `json:"failures,omitempty"`
}

type ProviderAdapterPlanOptions struct {
	ConfigPath          string
	RequestEnvelopePath string
	OutputPath          string
}

type ProviderAdapterPlanResult struct {
	Status                    string   `json:"status"`
	AdapterPlanReady          bool     `json:"adapter_plan_ready"`
	AdapterRegistryValidated  bool     `json:"adapter_registry_validated"`
	Provider                  string   `json:"provider"`
	AdapterAvailableForFuture bool     `json:"adapter_available_for_future"`
	AdapterEnabledNow         bool     `json:"adapter_enabled_now"`
	TransportEnabled          bool     `json:"transport_enabled"`
	NetworkCall               bool     `json:"network_call"`
	ProviderCall              bool     `json:"provider_call"`
	WorkerExecution           bool     `json:"worker_execution"`
	SentToProvider            bool     `json:"sent_to_provider"`
	TransportCalled           bool     `json:"transport_called"`
	BlockedReason             string   `json:"blocked_reason"`
	ConfigSHA256              string   `json:"config_sha256"`
	RequestEnvelopeSHA256     string   `json:"request_envelope_sha256"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func LoadProviderAdaptersConfig(path string) (ProviderAdaptersConfig, error) {
	if err := validateRelativeSafePath("provider adapters config path", path); err != nil {
		return ProviderAdaptersConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderAdaptersConfig{}, fmt.Errorf("read provider adapters config %q: %w", path, err)
	}
	return ParseProviderAdaptersConfig(data)
}

func ParseProviderAdaptersConfig(data []byte) (ProviderAdaptersConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderAdaptersConfig{}, fmt.Errorf("provider adapters config must not contain materialized preview text")
	}
	var cfg ProviderAdaptersConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProviderAdaptersConfig{}, fmt.Errorf("parse provider adapters yaml: %w", err)
	}
	cfg.ProviderAdapters.BlockedReason = strings.TrimSpace(cfg.ProviderAdapters.BlockedReason)
	for name, adapter := range cfg.ProviderAdapters.Adapters {
		adapter.Provider = strings.TrimSpace(adapter.Provider)
		adapter.BlockedReason = strings.TrimSpace(adapter.BlockedReason)
		cfg.ProviderAdapters.Adapters[name] = adapter
	}
	return cfg, nil
}

func ValidateProviderAdaptersConfig(cfg ProviderAdaptersConfig) error {
	p := cfg.ProviderAdapters
	if p.Enabled {
		return fmt.Errorf("provider_adapters.enabled must be false")
	}
	if p.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("provider_adapters.blocked_reason %q must be %q", p.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	codex, ok := p.Adapters["codex"]
	if !ok {
		return fmt.Errorf("provider_adapters.adapters.codex is required")
	}
	if err := validateProviderAdapterPolicy("codex", codex); err != nil {
		return err
	}
	return nil
}

func validateProviderAdapterPolicy(name string, adapter ProviderAdapterPolicy) error {
	if adapter.Enabled || adapter.AdapterEnabledNow || adapter.TransportEnabled {
		return fmt.Errorf("provider_adapters.adapters.%s must keep enabled flags false", name)
	}
	if !adapter.AdapterAvailableForFuture {
		return fmt.Errorf("provider_adapters.adapters.%s.adapter_available_for_future must be true", name)
	}
	if adapter.Provider != ProviderCallExecutorProvider {
		return fmt.Errorf("provider_adapters.adapters.%s.provider %q must be %q", name, adapter.Provider, ProviderCallExecutorProvider)
	}
	if adapter.AllowProviderCall || adapter.AllowNetwork {
		return fmt.Errorf("provider_adapters.adapters.%s must keep allow_provider_call and allow_network false", name)
	}
	if adapter.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("provider_adapters.adapters.%s.blocked_reason %q must be %q", name, adapter.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	return nil
}

func ProviderAdapterRegistryValidate(cfg ProviderAdaptersConfig) (ProviderAdapterRegistryValidateResult, error) {
	result := ProviderAdapterRegistryValidateResult{
		Status:        lancedbpolicy.StatusOK,
		Enabled:       cfg.ProviderAdapters.Enabled,
		BlockedReason: cfg.ProviderAdapters.BlockedReason,
		AdapterCount:  len(cfg.ProviderAdapters.Adapters),
	}
	if err := ValidateProviderAdaptersConfig(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
		return result, nil
	}
	result.AdapterRegistryValidated = true
	return result, nil
}

func ProviderAdapterPlan(opts ProviderAdapterPlanOptions) (ProviderAdapterPlanResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"config path", opts.ConfigPath},
		{"request envelope path", opts.RequestEnvelopePath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderAdapterPlanResult{}, err
		}
	}

	cfgData, err := readArtifactBytesNoTextExcerpt("provider adapters config", opts.ConfigPath)
	if err != nil {
		return ProviderAdapterPlanResult{}, err
	}
	cfg, err := ParseProviderAdaptersConfig(cfgData)
	if err != nil {
		return ProviderAdapterPlanResult{}, err
	}
	validateResult, err := ProviderAdapterRegistryValidate(cfg)
	if err != nil {
		return ProviderAdapterPlanResult{}, err
	}

	envelope, envelopeData, err := LoadProviderRequestEnvelope(opts.RequestEnvelopePath)
	if err != nil {
		return ProviderAdapterPlanResult{}, err
	}

	var failures []string
	if validateResult.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, validateResult.Failures...)
	}
	if !envelope.ProviderRequestReady {
		failures = append(failures, "request envelope provider_request_ready must be true")
	}
	if envelope.RequestContainsText || envelope.SentToProvider {
		failures = append(failures, "request envelope must keep request flags blocked")
	}

	codex := cfg.ProviderAdapters.Adapters["codex"]
	result := ProviderAdapterPlanResult{
		Status:                    lancedbpolicy.StatusOK,
		AdapterRegistryValidated:  validateResult.AdapterRegistryValidated,
		Provider:                  codex.Provider,
		AdapterAvailableForFuture: codex.AdapterAvailableForFuture,
		AdapterEnabledNow:         false,
		TransportEnabled:          false,
		NetworkCall:               false,
		ProviderCall:              false,
		WorkerExecution:           false,
		SentToProvider:            false,
		TransportCalled:           false,
		BlockedReason:             ProviderCallExecutorBlockedReason,
		ConfigSHA256:              sha256Hex(cfgData),
		RequestEnvelopeSHA256:     sha256Hex(envelopeData),
		Failures:                  failures,
	}
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.AdapterPlanReady = true
		if err := writeProviderAdapterPlanJSON(opts.OutputPath, result); err != nil {
			return ProviderAdapterPlanResult{}, err
		}
	}
	return result, nil
}

func LoadProviderAdapterPlan(path string) (ProviderAdapterPlanResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("adapter plan", path)
	if err != nil {
		return ProviderAdapterPlanResult{}, nil, err
	}
	var result ProviderAdapterPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderAdapterPlanResult{}, nil, fmt.Errorf("parse adapter plan json: %w", err)
	}
	return result, data, nil
}

func writeProviderAdapterPlanJSON(path string, result ProviderAdapterPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create adapter plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal adapter plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("adapter plan must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderAdapterRegistryValidateText(result ProviderAdapterRegistryValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_adapter_registry_validate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"adapter_registry_validated", fmt.Sprintf("%t", result.AdapterRegistryValidated)},
		{"enabled", fmt.Sprintf("%t", result.Enabled)},
		{"blocked_reason", result.BlockedReason},
		{"adapter_count", fmt.Sprintf("%d", result.AdapterCount)},
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

func WriteProviderAdapterRegistryValidateJSON(result ProviderAdapterRegistryValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal adapter registry validate json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderAdapterPlanText(result ProviderAdapterPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_adapter_plan:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"adapter_plan_ready", fmt.Sprintf("%t", result.AdapterPlanReady)},
		{"adapter_registry_validated", fmt.Sprintf("%t", result.AdapterRegistryValidated)},
		{"provider", result.Provider},
		{"adapter_available_for_future", fmt.Sprintf("%t", result.AdapterAvailableForFuture)},
		{"adapter_enabled_now", fmt.Sprintf("%t", result.AdapterEnabledNow)},
		{"transport_enabled", fmt.Sprintf("%t", result.TransportEnabled)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
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

func WriteProviderAdapterPlanJSON(result ProviderAdapterPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal adapter plan json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}
