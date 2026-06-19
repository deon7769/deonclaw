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
	MaterializedInjectionExecutionEnableBlockedReason = "implementation_not_enabled"
)

type MaterializedInjectionExecutionEnableConfig struct {
	MaterializedInjectionExecutionEnable MaterializedInjectionExecutionEnable `yaml:"materialized_injection_execution_enable" json:"materialized_injection_execution_enable"`
}

type MaterializedInjectionExecutionEnable struct {
	Enabled                bool   `yaml:"enabled" json:"enabled"`
	RequireConfirmFlag     bool   `yaml:"require_confirm_flag" json:"require_confirm_flag"`
	RequireReadinessReport bool   `yaml:"require_readiness_report" json:"require_readiness_report"`
	RequireExecutionGate   bool   `yaml:"require_execution_gate" json:"require_execution_gate"`
	RequireAssemblyReport  bool   `yaml:"require_assembly_report" json:"require_assembly_report"`
	AllowProviderCall      bool   `yaml:"allow_provider_call" json:"allow_provider_call"`
	AllowWorkerExecution   bool   `yaml:"allow_worker_execution" json:"allow_worker_execution"`
	AllowPromptInjection   bool   `yaml:"allow_prompt_injection" json:"allow_prompt_injection"`
	MaxTotalChars          int    `yaml:"max_total_chars" json:"max_total_chars"`
	BlockedReason          string `yaml:"blocked_reason" json:"blocked_reason"`
}

type MaterializedInjectionExecutionEnableValidateResult struct {
	Status               string   `json:"status"`
	Enabled              bool     `json:"enabled"`
	AllowProviderCall    bool     `json:"allow_provider_call"`
	AllowWorkerExecution bool     `json:"allow_worker_execution"`
	AllowPromptInjection bool     `json:"allow_prompt_injection"`
	BlockedReason        string   `json:"blocked_reason"`
	MaxTotalChars        int      `json:"max_total_chars"`
	Warnings             []string `json:"warnings,omitempty"`
	Failures             []string `json:"failures,omitempty"`
}

func LoadMaterializedInjectionExecutionEnable(path string) (MaterializedInjectionExecutionEnableConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionExecutionEnableConfig{}, fmt.Errorf("read materialized injection execution enable config %q: %w", path, err)
	}
	return ParseMaterializedInjectionExecutionEnable(data)
}

func ParseMaterializedInjectionExecutionEnable(data []byte) (MaterializedInjectionExecutionEnableConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionExecutionEnableConfig{}, fmt.Errorf("materialized injection execution enable config must not contain materialized preview text")
	}
	var cfg MaterializedInjectionExecutionEnableConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return MaterializedInjectionExecutionEnableConfig{}, fmt.Errorf("parse materialized injection execution enable yaml: %w", err)
	}
	cfg.MaterializedInjectionExecutionEnable.BlockedReason = strings.TrimSpace(cfg.MaterializedInjectionExecutionEnable.BlockedReason)
	return cfg, nil
}

func ValidateMaterializedInjectionExecutionEnable(cfg MaterializedInjectionExecutionEnableConfig) error {
	r := cfg.MaterializedInjectionExecutionEnable
	if r.Enabled {
		return fmt.Errorf("materialized_injection_execution_enable.enabled must be false")
	}
	if !r.RequireConfirmFlag {
		return fmt.Errorf("materialized_injection_execution_enable.require_confirm_flag must be true")
	}
	if !r.RequireReadinessReport {
		return fmt.Errorf("materialized_injection_execution_enable.require_readiness_report must be true")
	}
	if !r.RequireExecutionGate {
		return fmt.Errorf("materialized_injection_execution_enable.require_execution_gate must be true")
	}
	if !r.RequireAssemblyReport {
		return fmt.Errorf("materialized_injection_execution_enable.require_assembly_report must be true")
	}
	if r.AllowProviderCall {
		return fmt.Errorf("materialized_injection_execution_enable.allow_provider_call must be false")
	}
	if r.AllowWorkerExecution {
		return fmt.Errorf("materialized_injection_execution_enable.allow_worker_execution must be false")
	}
	if r.AllowPromptInjection {
		return fmt.Errorf("materialized_injection_execution_enable.allow_prompt_injection must be false")
	}
	if r.MaxTotalChars <= 0 {
		return fmt.Errorf("materialized_injection_execution_enable.max_total_chars must be > 0")
	}
	if r.BlockedReason != MaterializedInjectionExecutionEnableBlockedReason {
		return fmt.Errorf("materialized_injection_execution_enable.blocked_reason %q must be %q", r.BlockedReason, MaterializedInjectionExecutionEnableBlockedReason)
	}
	return nil
}

func MaterializedInjectionExecutionEnableValidate(cfg MaterializedInjectionExecutionEnableConfig) (MaterializedInjectionExecutionEnableValidateResult, error) {
	r := cfg.MaterializedInjectionExecutionEnable
	result := MaterializedInjectionExecutionEnableValidateResult{
		Status:               lancedbpolicy.StatusOK,
		Enabled:              r.Enabled,
		AllowProviderCall:    r.AllowProviderCall,
		AllowWorkerExecution: r.AllowWorkerExecution,
		AllowPromptInjection: r.AllowPromptInjection,
		MaxTotalChars:        r.MaxTotalChars,
		BlockedReason:        r.BlockedReason,
	}
	if err := ValidateMaterializedInjectionExecutionEnable(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
	}
	return result, nil
}

func WriteMaterializedInjectionExecutionEnableValidateText(result MaterializedInjectionExecutionEnableValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_execution_enable_validate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"enabled", fmt.Sprintf("%t", result.Enabled)},
		{"allow_provider_call", fmt.Sprintf("%t", result.AllowProviderCall)},
		{"allow_worker_execution", fmt.Sprintf("%t", result.AllowWorkerExecution)},
		{"allow_prompt_injection", fmt.Sprintf("%t", result.AllowPromptInjection)},
		{"blocked_reason", result.BlockedReason},
		{"max_total_chars", fmt.Sprintf("%d", result.MaxTotalChars)},
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
		return fmt.Errorf("materialized injection execution enable validate text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionExecutionEnableValidateJSON(result MaterializedInjectionExecutionEnableValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection execution enable validate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection execution enable validate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
