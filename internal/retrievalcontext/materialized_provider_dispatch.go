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

const (
	MaterializedProviderDispatchBlockedReason = "implementation_not_enabled"
	MaterializedProviderDispatchProvider      = "codex"
)

type MaterializedProviderDispatchConfig struct {
	MaterializedProviderDispatch MaterializedProviderDispatch `yaml:"materialized_provider_dispatch" json:"materialized_provider_dispatch"`
}

type MaterializedProviderDispatch struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	Provider               string `yaml:"provider" json:"provider"`
	RequireConfirmFlag     bool   `yaml:"require_confirm_flag" json:"require_confirm_flag"`
	RequireProviderRunPlan bool   `yaml:"require_provider_run_plan" json:"require_provider_run_plan"`
	RequireAssembledPrompt bool   `yaml:"require_assembled_prompt" json:"require_assembled_prompt"`
	AllowProviderCall      bool   `yaml:"allow_provider_call" json:"allow_provider_call"`
	AllowNetwork           bool   `yaml:"allow_network" json:"allow_network"`
	AllowWorkerExecution   bool   `yaml:"allow_worker_execution" json:"allow_worker_execution"`
	AllowPromptInjection   bool   `yaml:"allow_prompt_injection" json:"allow_prompt_injection"`
	MaxTotalChars          int    `yaml:"max_total_chars" json:"max_total_chars"`
	MaxPayloadBytes        int    `yaml:"max_payload_bytes" json:"max_payload_bytes"`
	BlockedReason          string `yaml:"blocked_reason" json:"blocked_reason"`
}

type MaterializedProviderDispatchValidateResult struct {
	Status               string   `json:"status"`
	Enabled              bool     `json:"enabled"`
	Provider             string   `json:"provider"`
	AllowProviderCall    bool     `json:"allow_provider_call"`
	AllowNetwork         bool     `json:"allow_network"`
	AllowWorkerExecution bool     `json:"allow_worker_execution"`
	AllowPromptInjection bool     `json:"allow_prompt_injection"`
	BlockedReason        string   `json:"blocked_reason"`
	MaxTotalChars        int      `json:"max_total_chars"`
	MaxPayloadBytes      int      `json:"max_payload_bytes"`
	Warnings             []string `json:"warnings,omitempty"`
	Failures             []string `json:"failures,omitempty"`
}

func LoadMaterializedProviderDispatch(path string) (MaterializedProviderDispatchConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedProviderDispatchConfig{}, fmt.Errorf("read materialized provider dispatch config %q: %w", path, err)
	}
	return ParseMaterializedProviderDispatch(data)
}

func ParseMaterializedProviderDispatch(data []byte) (MaterializedProviderDispatchConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedProviderDispatchConfig{}, fmt.Errorf("materialized provider dispatch config must not contain materialized preview text")
	}
	var cfg MaterializedProviderDispatchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return MaterializedProviderDispatchConfig{}, fmt.Errorf("parse materialized provider dispatch yaml: %w", err)
	}
	cfg.MaterializedProviderDispatch.Provider = strings.TrimSpace(cfg.MaterializedProviderDispatch.Provider)
	cfg.MaterializedProviderDispatch.BlockedReason = strings.TrimSpace(cfg.MaterializedProviderDispatch.BlockedReason)
	return cfg, nil
}

func ValidateMaterializedProviderDispatch(cfg MaterializedProviderDispatchConfig) error {
	r := cfg.MaterializedProviderDispatch
	if r.Enabled {
		return fmt.Errorf("materialized_provider_dispatch.enabled must be false")
	}
	if r.Provider != MaterializedProviderDispatchProvider {
		return fmt.Errorf("materialized_provider_dispatch.provider %q must be %q", r.Provider, MaterializedProviderDispatchProvider)
	}
	if !r.RequireConfirmFlag {
		return fmt.Errorf("materialized_provider_dispatch.require_confirm_flag must be true")
	}
	if !r.RequireProviderRunPlan {
		return fmt.Errorf("materialized_provider_dispatch.require_provider_run_plan must be true")
	}
	if !r.RequireAssembledPrompt {
		return fmt.Errorf("materialized_provider_dispatch.require_assembled_prompt must be true")
	}
	if r.AllowProviderCall {
		return fmt.Errorf("materialized_provider_dispatch.allow_provider_call must be false")
	}
	if r.AllowNetwork {
		return fmt.Errorf("materialized_provider_dispatch.allow_network must be false")
	}
	if r.AllowWorkerExecution {
		return fmt.Errorf("materialized_provider_dispatch.allow_worker_execution must be false")
	}
	if r.AllowPromptInjection {
		return fmt.Errorf("materialized_provider_dispatch.allow_prompt_injection must be false")
	}
	if r.MaxTotalChars <= 0 {
		return fmt.Errorf("materialized_provider_dispatch.max_total_chars must be > 0")
	}
	if r.MaxPayloadBytes <= 0 {
		return fmt.Errorf("materialized_provider_dispatch.max_payload_bytes must be > 0")
	}
	if r.BlockedReason != MaterializedProviderDispatchBlockedReason {
		return fmt.Errorf("materialized_provider_dispatch.blocked_reason %q must be %q", r.BlockedReason, MaterializedProviderDispatchBlockedReason)
	}
	return nil
}

func MaterializedProviderDispatchValidate(cfg MaterializedProviderDispatchConfig) (MaterializedProviderDispatchValidateResult, error) {
	r := cfg.MaterializedProviderDispatch
	result := MaterializedProviderDispatchValidateResult{
		Status:               lancedbpolicy.StatusOK,
		Enabled:              r.Enabled,
		Provider:             r.Provider,
		AllowProviderCall:    r.AllowProviderCall,
		AllowNetwork:         r.AllowNetwork,
		AllowWorkerExecution: r.AllowWorkerExecution,
		AllowPromptInjection: r.AllowPromptInjection,
		BlockedReason:        r.BlockedReason,
		MaxTotalChars:        r.MaxTotalChars,
		MaxPayloadBytes:      r.MaxPayloadBytes,
	}
	if err := ValidateMaterializedProviderDispatch(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
	}
	return result, nil
}

func WriteMaterializedProviderDispatchValidateText(result MaterializedProviderDispatchValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_provider_dispatch_validate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"enabled", fmt.Sprintf("%t", result.Enabled)},
		{"provider", result.Provider},
		{"allow_provider_call", fmt.Sprintf("%t", result.AllowProviderCall)},
		{"allow_network", fmt.Sprintf("%t", result.AllowNetwork)},
		{"allow_worker_execution", fmt.Sprintf("%t", result.AllowWorkerExecution)},
		{"allow_prompt_injection", fmt.Sprintf("%t", result.AllowPromptInjection)},
		{"blocked_reason", result.BlockedReason},
		{"max_total_chars", fmt.Sprintf("%d", result.MaxTotalChars)},
		{"max_payload_bytes", fmt.Sprintf("%d", result.MaxPayloadBytes)},
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
		return fmt.Errorf("materialized provider dispatch validate text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedProviderDispatchValidateJSON(result MaterializedProviderDispatchValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized provider dispatch validate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized provider dispatch validate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
