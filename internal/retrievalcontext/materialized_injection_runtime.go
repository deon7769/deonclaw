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
	MaterializedInjectionRuntimeBlockedReason = "implementation_not_enabled"
)

type MaterializedInjectionRuntimeConfig struct {
	MaterializedInjectionRuntime MaterializedInjectionRuntime `yaml:"materialized_injection_runtime" json:"materialized_injection_runtime"`
}

type MaterializedInjectionRuntime struct {
	Enabled               bool   `yaml:"enabled" json:"enabled"`
	RequireConfirmFlag    bool   `yaml:"require_confirm_flag" json:"require_confirm_flag"`
	RequireExecutionGate  bool   `yaml:"require_execution_gate" json:"require_execution_gate"`
	RequireAssemblyReport bool   `yaml:"require_assembly_report" json:"require_assembly_report"`
	AllowWorkerExecution  bool   `yaml:"allow_worker_execution" json:"allow_worker_execution"`
	AllowPromptInjection  bool   `yaml:"allow_prompt_injection" json:"allow_prompt_injection"`
	MaxTotalChars         int    `yaml:"max_total_chars" json:"max_total_chars"`
	BlockedReason         string `yaml:"blocked_reason" json:"blocked_reason"`
}

type MaterializedInjectionRuntimeValidateResult struct {
	Status                string   `json:"status"`
	Enabled               bool     `json:"enabled"`
	RequireConfirmFlag    bool     `json:"require_confirm_flag"`
	RequireExecutionGate  bool     `json:"require_execution_gate"`
	RequireAssemblyReport bool     `json:"require_assembly_report"`
	AllowWorkerExecution  bool     `json:"allow_worker_execution"`
	AllowPromptInjection  bool     `json:"allow_prompt_injection"`
	MaxTotalChars         int      `json:"max_total_chars"`
	BlockedReason         string   `json:"blocked_reason"`
	Warnings              []string `json:"warnings,omitempty"`
	Failures              []string `json:"failures,omitempty"`
}

func LoadMaterializedInjectionRuntime(path string) (MaterializedInjectionRuntimeConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedInjectionRuntimeConfig{}, fmt.Errorf("read materialized injection runtime config %q: %w", path, err)
	}
	return ParseMaterializedInjectionRuntime(data)
}

func ParseMaterializedInjectionRuntime(data []byte) (MaterializedInjectionRuntimeConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return MaterializedInjectionRuntimeConfig{}, fmt.Errorf("materialized injection runtime config must not contain materialized preview text")
	}
	var cfg MaterializedInjectionRuntimeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return MaterializedInjectionRuntimeConfig{}, fmt.Errorf("parse materialized injection runtime yaml: %w", err)
	}
	cfg.MaterializedInjectionRuntime.BlockedReason = strings.TrimSpace(cfg.MaterializedInjectionRuntime.BlockedReason)
	return cfg, nil
}

func ValidateMaterializedInjectionRuntime(cfg MaterializedInjectionRuntimeConfig) error {
	r := cfg.MaterializedInjectionRuntime
	if r.Enabled {
		return fmt.Errorf("materialized_injection_runtime.enabled must be false")
	}
	if !r.RequireConfirmFlag {
		return fmt.Errorf("materialized_injection_runtime.require_confirm_flag must be true")
	}
	if !r.RequireExecutionGate {
		return fmt.Errorf("materialized_injection_runtime.require_execution_gate must be true")
	}
	if !r.RequireAssemblyReport {
		return fmt.Errorf("materialized_injection_runtime.require_assembly_report must be true")
	}
	if r.AllowWorkerExecution {
		return fmt.Errorf("materialized_injection_runtime.allow_worker_execution must be false")
	}
	if r.AllowPromptInjection {
		return fmt.Errorf("materialized_injection_runtime.allow_prompt_injection must be false")
	}
	if r.MaxTotalChars <= 0 {
		return fmt.Errorf("materialized_injection_runtime.max_total_chars must be > 0")
	}
	if r.BlockedReason != MaterializedInjectionRuntimeBlockedReason {
		return fmt.Errorf("materialized_injection_runtime.blocked_reason %q must be %q", r.BlockedReason, MaterializedInjectionRuntimeBlockedReason)
	}
	return nil
}

func MaterializedInjectionRuntimeValidate(cfg MaterializedInjectionRuntimeConfig) (MaterializedInjectionRuntimeValidateResult, error) {
	r := cfg.MaterializedInjectionRuntime
	result := MaterializedInjectionRuntimeValidateResult{
		Status:                lancedbpolicy.StatusOK,
		Enabled:               r.Enabled,
		RequireConfirmFlag:    r.RequireConfirmFlag,
		RequireExecutionGate:  r.RequireExecutionGate,
		RequireAssemblyReport: r.RequireAssemblyReport,
		AllowWorkerExecution:  r.AllowWorkerExecution,
		AllowPromptInjection:  r.AllowPromptInjection,
		MaxTotalChars:         r.MaxTotalChars,
		BlockedReason:         r.BlockedReason,
	}
	if err := ValidateMaterializedInjectionRuntime(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
	}
	return result, nil
}

func WriteMaterializedInjectionRuntimeValidateText(result MaterializedInjectionRuntimeValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_materialized_injection_runtime_validate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"enabled", fmt.Sprintf("%t", result.Enabled)},
		{"require_confirm_flag", fmt.Sprintf("%t", result.RequireConfirmFlag)},
		{"require_execution_gate", fmt.Sprintf("%t", result.RequireExecutionGate)},
		{"require_assembly_report", fmt.Sprintf("%t", result.RequireAssemblyReport)},
		{"allow_worker_execution", fmt.Sprintf("%t", result.AllowWorkerExecution)},
		{"allow_prompt_injection", fmt.Sprintf("%t", result.AllowPromptInjection)},
		{"max_total_chars", fmt.Sprintf("%d", result.MaxTotalChars)},
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
		return fmt.Errorf("materialized injection runtime validate text must not contain materialized preview text")
	}
	return nil
}

func WriteMaterializedInjectionRuntimeValidateJSON(result MaterializedInjectionRuntimeValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized injection runtime validate json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("materialized injection runtime validate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
