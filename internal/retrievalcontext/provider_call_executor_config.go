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
	ProviderCallExecutorBlockedReason = "implementation_not_enabled"
	ProviderCallExecutorProvider      = "codex"
)

type ProviderCallExecutorConfig struct {
	ProviderCallExecutor ProviderCallExecutorPolicy `yaml:"provider_call_executor" json:"provider_call_executor"`
}

type ProviderCallExecutorPolicy struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	Provider               string `yaml:"provider" json:"provider"`
	RequireConfirmFlag     bool   `yaml:"require_confirm_flag" json:"require_confirm_flag"`
	RequireExecutionBundle bool   `yaml:"require_execution_bundle" json:"require_execution_bundle"`
	RequireChainAudit      bool   `yaml:"require_chain_audit" json:"require_chain_audit"`
	RequireApproval        bool   `yaml:"require_approval" json:"require_approval"`
	RequireDispatchConfig  bool   `yaml:"require_dispatch_config" json:"require_dispatch_config"`
	AllowProviderCall      bool   `yaml:"allow_provider_call" json:"allow_provider_call"`
	AllowNetwork           bool   `yaml:"allow_network" json:"allow_network"`
	AllowWorkerExecution   bool   `yaml:"allow_worker_execution" json:"allow_worker_execution"`
	AllowPromptInjection   bool   `yaml:"allow_prompt_injection" json:"allow_prompt_injection"`
	MaxTotalChars          int    `yaml:"max_total_chars" json:"max_total_chars"`
	MaxPayloadBytes        int    `yaml:"max_payload_bytes" json:"max_payload_bytes"`
	BlockedReason          string `yaml:"blocked_reason" json:"blocked_reason"`
}

type ProviderCallExecutorConfigValidateResult struct {
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

func LoadProviderCallExecutorConfig(path string) (ProviderCallExecutorConfig, error) {
	if err := validateRelativeSafePath("provider call executor config path", path); err != nil {
		return ProviderCallExecutorConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCallExecutorConfig{}, fmt.Errorf("read provider call executor config %q: %w", path, err)
	}
	return ParseProviderCallExecutorConfig(data)
}

func ParseProviderCallExecutorConfig(data []byte) (ProviderCallExecutorConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallExecutorConfig{}, fmt.Errorf("provider call executor config must not contain materialized preview text")
	}
	var cfg ProviderCallExecutorConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProviderCallExecutorConfig{}, fmt.Errorf("parse provider call executor yaml: %w", err)
	}
	cfg.ProviderCallExecutor.Provider = strings.TrimSpace(cfg.ProviderCallExecutor.Provider)
	cfg.ProviderCallExecutor.BlockedReason = strings.TrimSpace(cfg.ProviderCallExecutor.BlockedReason)
	return cfg, nil
}

func ValidateProviderCallExecutorConfig(cfg ProviderCallExecutorConfig) error {
	r := cfg.ProviderCallExecutor
	if r.Enabled {
		return fmt.Errorf("provider_call_executor.enabled must be false")
	}
	if r.Provider != ProviderCallExecutorProvider {
		return fmt.Errorf("provider_call_executor.provider %q must be %q", r.Provider, ProviderCallExecutorProvider)
	}
	if !r.RequireConfirmFlag {
		return fmt.Errorf("provider_call_executor.require_confirm_flag must be true")
	}
	if !r.RequireExecutionBundle {
		return fmt.Errorf("provider_call_executor.require_execution_bundle must be true")
	}
	if !r.RequireChainAudit {
		return fmt.Errorf("provider_call_executor.require_chain_audit must be true")
	}
	if !r.RequireApproval {
		return fmt.Errorf("provider_call_executor.require_approval must be true")
	}
	if !r.RequireDispatchConfig {
		return fmt.Errorf("provider_call_executor.require_dispatch_config must be true")
	}
	if r.AllowProviderCall {
		return fmt.Errorf("provider_call_executor.allow_provider_call must be false")
	}
	if r.AllowNetwork {
		return fmt.Errorf("provider_call_executor.allow_network must be false")
	}
	if r.AllowWorkerExecution {
		return fmt.Errorf("provider_call_executor.allow_worker_execution must be false")
	}
	if r.AllowPromptInjection {
		return fmt.Errorf("provider_call_executor.allow_prompt_injection must be false")
	}
	if r.MaxTotalChars <= 0 {
		return fmt.Errorf("provider_call_executor.max_total_chars must be > 0")
	}
	if r.MaxPayloadBytes <= 0 {
		return fmt.Errorf("provider_call_executor.max_payload_bytes must be > 0")
	}
	if r.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("provider_call_executor.blocked_reason %q must be %q", r.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	return nil
}

func ProviderCallExecutorConfigValidate(cfg ProviderCallExecutorConfig) (ProviderCallExecutorConfigValidateResult, error) {
	r := cfg.ProviderCallExecutor
	result := ProviderCallExecutorConfigValidateResult{
		Status:               lancedbpolicy.StatusOK,
		Enabled:              r.Enabled,
		Provider:             r.Provider,
		AllowProviderCall:    r.AllowProviderCall,
		AllowNetwork:         r.AllowNetwork,
		AllowWorkerExecution: r.AllowWorkerExecution,
		AllowPromptInjection: r.AllowPromptInjection,
		MaxTotalChars:        r.MaxTotalChars,
		MaxPayloadBytes:      r.MaxPayloadBytes,
		BlockedReason:        r.BlockedReason,
	}
	if err := ValidateProviderCallExecutorConfig(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
	}
	return result, nil
}

func WriteProviderCallExecutorConfigValidateText(result ProviderCallExecutorConfigValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_executor_config_validate:"); err != nil {
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
	text := fmt.Sprintf("%+v", result)
	if strings.Contains(text, "text_excerpt") || strings.Contains(text, "alpha text") {
		return fmt.Errorf("provider call executor config validate text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallExecutorConfigValidateJSON(result ProviderCallExecutorConfigValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor config validate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call executor config validate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
