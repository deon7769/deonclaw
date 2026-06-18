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
	InjectionPolicyModePlanOnly        = "plan_only"
	InjectionPolicyReasonSchemaOnly    = "schema_only_no_execution"
	InjectionPolicyFutureConfirmFlag   = "--confirm-inject-materialized-context"
	InjectionPolicyFutureApprovalGate  = "approval.runner_injection_allowed must be true"
	InjectionPolicyFutureGovernanceOK  = "governance-report status must be ok"
	InjectionPolicyFutureInjectionPlan = "injection-plan must allow future execution gate"
)

type InjectionPolicyConfig struct {
	RetrievalInjectionPolicy InjectionPolicy `yaml:"retrieval_injection_policy" json:"retrieval_injection_policy"`
}

type InjectionPolicy struct {
	Mode         string                      `yaml:"mode" json:"mode"`
	Approval     InjectionPolicyApproval     `yaml:"approval" json:"approval"`
	Bundle       InjectionPolicyBundle       `yaml:"bundle" json:"bundle"`
	Materialized InjectionPolicyMaterialized `yaml:"materialized" json:"materialized"`
	Limits       InjectionPolicyLimits       `yaml:"limits" json:"limits"`
	Prompt       InjectionPolicyPrompt       `yaml:"prompt" json:"prompt"`
	Safety       InjectionPolicySafety       `yaml:"safety" json:"safety"`
}

type InjectionPolicyApproval struct {
	Required                      bool   `yaml:"required" json:"required"`
	RequireRunnerInjectionAllowed bool   `yaml:"require_runner_injection_allowed" json:"require_runner_injection_allowed"`
	ApprovalPath                  string `yaml:"approval_path" json:"approval_path"`
	RequestPath                   string `yaml:"request_path" json:"request_path"`
}

type InjectionPolicyBundle struct {
	Path string `yaml:"path" json:"path"`
}

type InjectionPolicyMaterialized struct {
	Path string `yaml:"path" json:"path"`
}

type InjectionPolicyLimits struct {
	MaxTotalChars    int `yaml:"max_total_chars" json:"max_total_chars"`
	MaxCharsPerChunk int `yaml:"max_chars_per_chunk" json:"max_chars_per_chunk"`
	MaxChunks        int `yaml:"max_chunks" json:"max_chunks"`
}

type InjectionPolicyPrompt struct {
	SectionTitle      string `yaml:"section_title" json:"section_title"`
	IncludeMetadata   bool   `yaml:"include_metadata" json:"include_metadata"`
	IncludeSourcePath bool   `yaml:"include_source_path" json:"include_source_path"`
	IncludeHashes     bool   `yaml:"include_hashes" json:"include_hashes"`
}

type InjectionPolicySafety struct {
	RequireGovernanceReportOK bool `yaml:"require_governance_report_ok" json:"require_governance_report_ok"`
	ForbidActiveSearch        bool `yaml:"forbid_active_search" json:"forbid_active_search"`
	ForbidProviderCalls       bool `yaml:"forbid_provider_calls" json:"forbid_provider_calls"`
	ForbidSourceFileReads     bool `yaml:"forbid_source_file_reads" json:"forbid_source_file_reads"`
	ForbidMemoryApply         bool `yaml:"forbid_memory_apply" json:"forbid_memory_apply"`
}

type InjectionPolicyPlanResult struct {
	Status                 string   `json:"status"`
	WouldInject            bool     `json:"would_inject"`
	Reason                 string   `json:"reason"`
	Mode                   string   `json:"mode"`
	MaxTotalChars          int      `json:"max_total_chars"`
	MaxCharsPerChunk       int      `json:"max_chars_per_chunk"`
	MaxChunks              int      `json:"max_chunks"`
	PromptSectionTitle     string   `json:"prompt_section_title"`
	RequiredFutureGates    []string `json:"required_future_gates"`
	ApprovalPresent        bool     `json:"approval_present"`
	BundlePresent          bool     `json:"bundle_present"`
	MaterializedPresent    bool     `json:"materialized_present"`
	MaterializedStatus     string   `json:"materialized_report_status,omitempty"`
	RunnerInjectionAllowed *bool    `json:"runner_injection_allowed,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
	Failures               []string `json:"failures,omitempty"`
}

func LoadInjectionPolicy(path string) (InjectionPolicyConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InjectionPolicyConfig{}, fmt.Errorf("read retrieval injection policy %q: %w", path, err)
	}
	return ParseInjectionPolicy(data)
}

func ParseInjectionPolicy(data []byte) (InjectionPolicyConfig, error) {
	if strings.Contains(string(data), "text_excerpt") {
		return InjectionPolicyConfig{}, fmt.Errorf("policy must not contain text_excerpt")
	}
	var cfg InjectionPolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return InjectionPolicyConfig{}, fmt.Errorf("parse retrieval injection policy yaml: %w", err)
	}
	normalizeInjectionPolicy(&cfg)
	return cfg, nil
}

func ValidateInjectionPolicy(cfg InjectionPolicyConfig) error {
	return validateInjectionPolicy(cfg)
}

func InjectionPolicyPlan(cfg InjectionPolicyConfig) (InjectionPolicyPlanResult, error) {
	if err := validateInjectionPolicy(cfg); err != nil {
		return InjectionPolicyPlanResult{}, err
	}
	p := cfg.RetrievalInjectionPolicy

	result := InjectionPolicyPlanResult{
		Status:              lancedbpolicy.StatusOK,
		WouldInject:         false,
		Reason:              InjectionPolicyReasonSchemaOnly,
		Mode:                p.Mode,
		MaxTotalChars:       p.Limits.MaxTotalChars,
		MaxCharsPerChunk:    p.Limits.MaxCharsPerChunk,
		MaxChunks:           p.Limits.MaxChunks,
		PromptSectionTitle:  p.Prompt.SectionTitle,
		RequiredFutureGates: defaultInjectionPolicyFutureGates(p),
	}

	var warnings []string
	var failures []string

	if p.Approval.ApprovalPath != "" {
		if _, err := os.Stat(p.Approval.ApprovalPath); err == nil {
			result.ApprovalPresent = true
			approval, err := LoadMaterializedContextApproval(p.Approval.ApprovalPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("approval: %v", err))
			} else {
				allowed := approval.RunnerInjectionAllowed
				result.RunnerInjectionAllowed = &allowed
				if p.Approval.RequireRunnerInjectionAllowed && !approval.RunnerInjectionAllowed {
					warnings = append(warnings, "approval runner_injection_allowed is false; injection not authorized yet")
				}
			}
			if p.Approval.RequestPath != "" {
				inspect, err := InspectApproval(p.Approval.ApprovalPath, InspectApprovalOptions{RequestPath: p.Approval.RequestPath})
				if err != nil {
					failures = append(failures, fmt.Sprintf("approval inspect: %v", err))
				} else if inspect.Status == lancedbpolicy.StatusFailed {
					failures = append(failures, "approval inspect status failed")
				} else if inspect.Status == lancedbpolicy.StatusWarning {
					warnings = append(warnings, "approval inspect status warning")
				}
			}
		}
	}

	if p.Bundle.Path != "" {
		if _, err := os.Stat(p.Bundle.Path); err == nil {
			result.BundlePresent = true
			bundle, bundleData, err := LoadBundle(p.Bundle.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("bundle: %v", err))
			} else if bundle.Status == lancedbpolicy.StatusFailed {
				failures = append(failures, fmt.Sprintf("bundle status %q", bundle.Status))
			} else if strings.Contains(string(bundleData), `"text_excerpt"`) {
				failures = append(failures, "bundle contains forbidden field text_excerpt")
			}
		}
	}

	if p.Materialized.Path != "" {
		if _, err := os.Stat(p.Materialized.Path); err == nil {
			result.MaterializedPresent = true
			report, err := MaterializedReport(p.Materialized.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("materialized report: %v", err))
			} else {
				result.MaterializedStatus = report.Status
				if report.Status == lancedbpolicy.StatusFailed {
					failures = append(failures, fmt.Sprintf("materialized report status %q", report.Status))
				}
				if report.IncludedChunkCount > p.Limits.MaxChunks {
					warnings = append(warnings, fmt.Sprintf("materialized included_chunk_count %d exceeds policy max_chunks %d", report.IncludedChunkCount, p.Limits.MaxChunks))
				}
				if report.TotalCharsIncluded > p.Limits.MaxTotalChars {
					warnings = append(warnings, fmt.Sprintf("materialized total_chars_included %d exceeds policy max_total_chars %d", report.TotalCharsIncluded, p.Limits.MaxTotalChars))
				}
			}
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	} else if len(warnings) > 0 {
		result.Status = lancedbpolicy.StatusWarning
	}
	return result, nil
}

func normalizeInjectionPolicy(cfg *InjectionPolicyConfig) {
	p := &cfg.RetrievalInjectionPolicy
	p.Mode = strings.TrimSpace(p.Mode)
	p.Approval.ApprovalPath = strings.TrimSpace(p.Approval.ApprovalPath)
	p.Approval.RequestPath = strings.TrimSpace(p.Approval.RequestPath)
	p.Bundle.Path = strings.TrimSpace(p.Bundle.Path)
	p.Materialized.Path = strings.TrimSpace(p.Materialized.Path)
	p.Prompt.SectionTitle = strings.TrimSpace(p.Prompt.SectionTitle)
}

func validateInjectionPolicy(cfg InjectionPolicyConfig) error {
	p := cfg.RetrievalInjectionPolicy
	if p.Mode != InjectionPolicyModePlanOnly {
		return fmt.Errorf("mode %q must be %q", p.Mode, InjectionPolicyModePlanOnly)
	}
	if !p.Approval.Required {
		return fmt.Errorf("approval.required must be true")
	}
	if !p.Approval.RequireRunnerInjectionAllowed {
		return fmt.Errorf("approval.require_runner_injection_allowed must be true")
	}
	for _, err := range []error{
		validateRelativeSafePath("approval.approval_path", p.Approval.ApprovalPath),
		validateRelativeSafePath("approval.request_path", p.Approval.RequestPath),
		validateRelativeSafePath("bundle.path", p.Bundle.Path),
		validateRelativeSafePath("materialized.path", p.Materialized.Path),
	} {
		if err != nil {
			return err
		}
	}
	if p.Limits.MaxTotalChars <= 0 {
		return fmt.Errorf("limits.max_total_chars must be > 0")
	}
	if p.Limits.MaxCharsPerChunk <= 0 {
		return fmt.Errorf("limits.max_chars_per_chunk must be > 0")
	}
	if p.Limits.MaxChunks <= 0 {
		return fmt.Errorf("limits.max_chunks must be > 0")
	}
	if p.Limits.MaxTotalChars < p.Limits.MaxCharsPerChunk {
		return fmt.Errorf("limits.max_total_chars must be >= max_chars_per_chunk")
	}
	if p.Prompt.SectionTitle == "" {
		return fmt.Errorf("prompt.section_title must not be empty")
	}
	if err := validateInjectionPolicySafety(p.Safety); err != nil {
		return err
	}
	return nil
}

func validateInjectionPolicySafety(s InjectionPolicySafety) error {
	checks := []struct {
		name  string
		value bool
	}{
		{"safety.require_governance_report_ok", s.RequireGovernanceReportOK},
		{"safety.forbid_active_search", s.ForbidActiveSearch},
		{"safety.forbid_provider_calls", s.ForbidProviderCalls},
		{"safety.forbid_source_file_reads", s.ForbidSourceFileReads},
		{"safety.forbid_memory_apply", s.ForbidMemoryApply},
	}
	for _, check := range checks {
		if !check.value {
			return fmt.Errorf("%s must be true", check.name)
		}
	}
	return nil
}

func defaultInjectionPolicyFutureGates(p InjectionPolicy) []string {
	gates := []string{
		InjectionPolicyFutureConfirmFlag,
		InjectionPolicyFutureApprovalGate,
		InjectionPolicyFutureGovernanceOK,
		InjectionPolicyFutureInjectionPlan,
		"runner execution task with explicit injection support",
	}
	if p.Safety.RequireGovernanceReportOK {
		_ = true
	}
	return gates
}

func WriteInjectionPolicyPlanText(result InjectionPolicyPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_injection_policy_plan:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "would_inject: %t\n", result.WouldInject); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "reason: %s\n", result.Reason); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "mode: %s\n", result.Mode); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_total_chars: %d\n", result.MaxTotalChars); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_chars_per_chunk: %d\n", result.MaxCharsPerChunk); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_chunks: %d\n", result.MaxChunks); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "prompt_section_title: %s\n", result.PromptSectionTitle); err != nil {
		return err
	}
	if len(result.RequiredFutureGates) > 0 {
		if _, err := fmt.Fprintln(out, "\nrequired_future_gates:"); err != nil {
			return err
		}
		for _, gate := range result.RequiredFutureGates {
			if _, err := fmt.Fprintf(out, "- %s\n", gate); err != nil {
				return err
			}
		}
	}
	if result.ApprovalPresent || result.BundlePresent || result.MaterializedPresent {
		if _, err := fmt.Fprintln(out, "\nartifact_probes:"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "approval_present: %t\n", result.ApprovalPresent); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "bundle_present: %t\n", result.BundlePresent); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "materialized_present: %t\n", result.MaterializedPresent); err != nil {
			return err
		}
		if result.MaterializedStatus != "" {
			if _, err := fmt.Fprintf(out, "materialized_report_status: %s\n", result.MaterializedStatus); err != nil {
				return err
			}
		}
		if result.RunnerInjectionAllowed != nil {
			if _, err := fmt.Fprintf(out, "runner_injection_allowed: %t\n", *result.RunnerInjectionAllowed); err != nil {
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
	if strings.Contains(fmt.Sprintf("%+v", result), "text_excerpt") {
		return fmt.Errorf("injection policy plan text must not contain text_excerpt")
	}
	return nil
}

func WriteInjectionPolicyPlanJSON(result InjectionPolicyPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal injection policy plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") {
		return fmt.Errorf("injection policy plan json must not contain text_excerpt")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
