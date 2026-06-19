package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"gopkg.in/yaml.v3"
)

type MaterializedInjectionDispatchConfig struct {
	MaterializedInjectionDispatch MaterializedInjectionDispatch `yaml:"materialized_injection_dispatch" json:"materialized_injection_dispatch"`
}

type MaterializedInjectionDispatch struct {
	Enabled              bool   `yaml:"enabled" json:"enabled"`
	AllowProviderCall    bool   `yaml:"allow_provider_call" json:"allow_provider_call"`
	AllowNetwork         bool   `yaml:"allow_network" json:"allow_network"`
	AllowWorkerExecution bool   `yaml:"allow_worker_execution" json:"allow_worker_execution"`
	BlockedReason        string `yaml:"blocked_reason" json:"blocked_reason"`
}

type MaterializedInjectionDispatchValidateResult struct {
	Status               string   `json:"status"`
	Enabled              bool     `json:"enabled"`
	AllowProviderCall    bool     `json:"allow_provider_call"`
	AllowNetwork         bool     `json:"allow_network"`
	AllowWorkerExecution bool     `json:"allow_worker_execution"`
	BlockedReason        string   `json:"blocked_reason"`
	Warnings             []string `json:"warnings,omitempty"`
	Failures             []string `json:"failures,omitempty"`
}

func LoadMaterializedInjectionDispatchConfig(path string) (MaterializedInjectionDispatchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionDispatchConfig{}, fmt.Errorf("read materialized injection dispatch config %q: %w", path, err)
	}
	return ParseMaterializedInjectionDispatchConfig(data)
}

func ParseMaterializedInjectionDispatchConfig(data []byte) (MaterializedInjectionDispatchConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionDispatchConfig{}, fmt.Errorf("materialized injection dispatch config must not contain materialized preview text")
	}
	var cfg MaterializedInjectionDispatchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return MaterializedInjectionDispatchConfig{}, fmt.Errorf("parse materialized injection dispatch yaml: %w", err)
	}
	cfg.MaterializedInjectionDispatch.BlockedReason = strings.TrimSpace(cfg.MaterializedInjectionDispatch.BlockedReason)
	return cfg, nil
}

func ValidateMaterializedInjectionDispatchConfig(cfg MaterializedInjectionDispatchConfig) error {
	d := cfg.MaterializedInjectionDispatch
	if d.Enabled {
		return fmt.Errorf("materialized_injection_dispatch.enabled must be false")
	}
	if d.AllowProviderCall {
		return fmt.Errorf("materialized_injection_dispatch.allow_provider_call must be false")
	}
	if d.AllowNetwork {
		return fmt.Errorf("materialized_injection_dispatch.allow_network must be false")
	}
	if d.AllowWorkerExecution {
		return fmt.Errorf("materialized_injection_dispatch.allow_worker_execution must be false")
	}
	if d.BlockedReason != MaterializedInjectionExecutionEnableBlockedReason {
		return fmt.Errorf("materialized_injection_dispatch.blocked_reason %q must be %q", d.BlockedReason, MaterializedInjectionExecutionEnableBlockedReason)
	}
	return nil
}

func MaterializedInjectionDispatchValidate(cfg MaterializedInjectionDispatchConfig) (MaterializedInjectionDispatchValidateResult, error) {
	d := cfg.MaterializedInjectionDispatch
	result := MaterializedInjectionDispatchValidateResult{
		Status:               lancedbpolicy.StatusOK,
		Enabled:              d.Enabled,
		AllowProviderCall:    d.AllowProviderCall,
		AllowNetwork:         d.AllowNetwork,
		AllowWorkerExecution: d.AllowWorkerExecution,
		BlockedReason:        d.BlockedReason,
	}
	if err := ValidateMaterializedInjectionDispatchConfig(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
	}
	return result, nil
}

func WriteMaterializedInjectionDispatchValidateText(result MaterializedInjectionDispatchValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_dispatch_validate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"enabled", fmt.Sprintf("%t", result.Enabled)},
		{"allow_provider_call", fmt.Sprintf("%t", result.AllowProviderCall)},
		{"allow_network", fmt.Sprintf("%t", result.AllowNetwork)},
		{"allow_worker_execution", fmt.Sprintf("%t", result.AllowWorkerExecution)},
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
	text := fmt.Sprintf("%+v", result)
	if strings.Contains(text, "text_excerpt") || strings.Contains(text, "alpha text") {
		return fmt.Errorf("materialized injection dispatch validate text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionDispatchValidateJSON(result MaterializedInjectionDispatchValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection dispatch validate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection dispatch validate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
