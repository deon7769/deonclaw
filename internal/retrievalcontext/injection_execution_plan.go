package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const (
	InjectionExecutionPlanReasonExecutionPlanOnly = "execution_plan_only"
)

type InjectionExecutionPlanOptions struct {
	PolicyPath                   string
	GovernanceReportPath         string
	InjectionApprovalPath        string
	InjectionApprovalRequestPath string
	MaterializedPath             string
	OutputPath                   string
	SummaryPath                  string
}

type InjectionExecutionPlanResult struct {
	Status                         string   `json:"status"`
	WouldExecuteRunner             bool     `json:"would_execute_runner"`
	WouldInjectMaterializedContext bool     `json:"would_inject_materialized_context"`
	ExecutionSupportedNow          bool     `json:"execution_supported_now"`
	Reason                         string   `json:"reason"`
	PromptSectionTitle             string   `json:"prompt_section_title"`
	MaterializedSHA256             string   `json:"materialized_sha256"`
	TotalCharsIncluded             int      `json:"total_chars_included"`
	IncludedChunkCount             int      `json:"included_chunk_count"`
	RequiredFutureFlag             string   `json:"required_future_flag"`
	Warnings                       []string `json:"warnings,omitempty"`
	Failures                       []string `json:"failures,omitempty"`
}

func InjectionExecutionPlan(opts InjectionExecutionPlanOptions) (InjectionExecutionPlanResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"policy path", opts.PolicyPath},
		{"governance report path", opts.GovernanceReportPath},
		{"injection approval path", opts.InjectionApprovalPath},
		{"injection approval request path", opts.InjectionApprovalRequestPath},
		{"materialized path", opts.MaterializedPath},
		{"output path", opts.OutputPath},
		{"summary path", opts.SummaryPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return InjectionExecutionPlanResult{}, err
		}
	}

	policyData, err := os.ReadFile(opts.PolicyPath)
	if err != nil {
		return InjectionExecutionPlanResult{}, fmt.Errorf("read injection policy %q: %w", opts.PolicyPath, err)
	}
	cfg, err := ParseInjectionPolicy(policyData)
	if err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if err := ValidateInjectionPolicy(cfg); err != nil {
		return InjectionExecutionPlanResult{}, fmt.Errorf("injection policy invalid: %w", err)
	}

	policyPlan, err := InjectionPolicyPlan(cfg)
	if err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if policyPlan.Status == lancedbpolicy.StatusFailed {
		return InjectionExecutionPlanResult{}, fmt.Errorf("injection policy plan status %q", policyPlan.Status)
	}

	governanceReport, governanceData, err := LoadGovernanceReport(opts.GovernanceReportPath)
	if err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if err := validateGovernanceReportForInjectionApproval(governanceReport); err != nil {
		return InjectionExecutionPlanResult{}, err
	}

	inspect, err := InspectInjectionApproval(opts.InjectionApprovalPath, InspectInjectionApprovalOptions{
		RequestPath: opts.InjectionApprovalRequestPath,
	})
	if err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		if len(inspect.Failures) > 0 {
			return InjectionExecutionPlanResult{}, fmt.Errorf("injection approval inspect status %q: %s", inspect.Status, strings.Join(inspect.Failures, "; "))
		}
		return InjectionExecutionPlanResult{}, fmt.Errorf("injection approval inspect status %q", inspect.Status)
	}

	approval, err := LoadRunnerInjectionApproval(opts.InjectionApprovalPath)
	if err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if err := validateInjectionExecutionPlanApproval(approval); err != nil {
		return InjectionExecutionPlanResult{}, err
	}

	policySHA := sha256Hex(policyData)
	governanceSHA := sha256Hex(governanceData)
	if approval.PolicySHA256 != policySHA {
		return InjectionExecutionPlanResult{}, fmt.Errorf("approval policy_sha256 mismatch")
	}
	if approval.GovernanceReportSHA256 != governanceSHA {
		return InjectionExecutionPlanResult{}, fmt.Errorf("approval governance_report_sha256 mismatch")
	}

	materializedData, err := os.ReadFile(opts.MaterializedPath)
	if err != nil {
		return InjectionExecutionPlanResult{}, fmt.Errorf("read materialized artifact %q: %w", opts.MaterializedPath, err)
	}
	materializedSHA := sha256Hex(materializedData)
	if approval.MaterializedSHA256 != materializedSHA {
		return InjectionExecutionPlanResult{}, fmt.Errorf("approval materialized_sha256 mismatch")
	}

	materializedReport, err := MaterializedReportBytes(materializedData)
	if err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if materializedReport.Status == lancedbpolicy.StatusFailed {
		return InjectionExecutionPlanResult{}, fmt.Errorf("materialized report status %q", materializedReport.Status)
	}

	limits := cfg.RetrievalInjectionPolicy.Limits
	if materializedReport.TotalCharsIncluded > limits.MaxTotalChars {
		return InjectionExecutionPlanResult{}, fmt.Errorf("materialized total_chars_included %d exceeds policy max_total_chars %d", materializedReport.TotalCharsIncluded, limits.MaxTotalChars)
	}
	if materializedReport.IncludedChunkCount > limits.MaxChunks {
		return InjectionExecutionPlanResult{}, fmt.Errorf("materialized included_chunk_count %d exceeds policy max_chunks %d", materializedReport.IncludedChunkCount, limits.MaxChunks)
	}

	warnings := mergeWarnings(
		append([]string(nil), governanceReport.Warnings...),
		append([]string(nil), policyPlan.Warnings...),
		append([]string(nil), approval.Warnings...),
		append([]string(nil), materializedReport.Warnings...),
	)
	if governanceReport.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"governance report status is warning; execution plan recorded with documented acceptance"})
	}
	if policyPlan.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"injection policy plan status is warning"})
	}

	status := lancedbpolicy.StatusOK
	if materializedReport.Status == lancedbpolicy.StatusWarning || len(warnings) > 0 {
		status = lancedbpolicy.StatusWarning
	}

	result := InjectionExecutionPlanResult{
		Status:                         status,
		WouldExecuteRunner:             false,
		WouldInjectMaterializedContext: true,
		ExecutionSupportedNow:          false,
		Reason:                         InjectionExecutionPlanReasonExecutionPlanOnly,
		PromptSectionTitle:             cfg.RetrievalInjectionPolicy.Prompt.SectionTitle,
		MaterializedSHA256:             materializedSHA,
		TotalCharsIncluded:             materializedReport.TotalCharsIncluded,
		IncludedChunkCount:             materializedReport.IncludedChunkCount,
		RequiredFutureFlag:             RequiredFutureInjectFlag,
		Warnings:                       warnings,
	}

	if err := writeInjectionExecutionPlanJSON(opts.OutputPath, result); err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	if err := writeInjectionExecutionPlanSummary(opts.SummaryPath, result); err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	return result, nil
}

func LoadInjectionExecutionPlan(path string) (InjectionExecutionPlanResult, error) {
	if err := validateRelativeSafePath("injection execution plan path", path); err != nil {
		return InjectionExecutionPlanResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InjectionExecutionPlanResult{}, fmt.Errorf("read injection execution plan %q: %w", path, err)
	}
	return ParseInjectionExecutionPlanJSON(data)
}

func ParseInjectionExecutionPlanJSON(data []byte) (InjectionExecutionPlanResult, error) {
	if strings.Contains(string(data), `"text_excerpt"`) {
		return InjectionExecutionPlanResult{}, fmt.Errorf("injection execution plan contains forbidden field text_excerpt")
	}
	var result InjectionExecutionPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return InjectionExecutionPlanResult{}, fmt.Errorf("parse injection execution plan json: %w", err)
	}
	return result, nil
}

func validateInjectionExecutionPlanApproval(approval RunnerInjectionApproval) error {
	if !approval.Approved {
		return fmt.Errorf("injection approval approved must be true")
	}
	if !approval.RunnerInjectionAllowed {
		return fmt.Errorf("injection approval runner_injection_allowed must be true")
	}
	if approval.AllowedUse != AllowedUseRunnerInjectionPolicyOnly {
		return fmt.Errorf("injection approval allowed_use %q must be %q", approval.AllowedUse, AllowedUseRunnerInjectionPolicyOnly)
	}
	if !approval.ConfirmAllowRunnerInjection {
		return fmt.Errorf("injection approval %s must be true", confirmAllowRunnerInjection)
	}
	return nil
}

func writeInjectionExecutionPlanJSON(path string, result InjectionExecutionPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection execution plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal injection execution plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("injection execution plan must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write injection execution plan json %q: %w", path, err)
	}
	return nil
}

func writeInjectionExecutionPlanSummary(path string, result InjectionExecutionPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection execution plan summary dir: %w", err)
	}
	var b strings.Builder
	b.WriteString("# Materialized retrieval context injection execution plan\n\n")
	b.WriteString("Execution-plan artifact only. Does not execute worker runs or inject materialized text into runner prompts.\n")
	b.WriteString("Runner execution remains unsupported until a future task enables it behind explicit confirm flags.\n\n")
	b.WriteString("- status: ")
	b.WriteString(result.Status)
	b.WriteByte('\n')
	b.WriteString("- would_execute_runner: ")
	b.WriteString(fmt.Sprintf("%t", result.WouldExecuteRunner))
	b.WriteByte('\n')
	b.WriteString("- would_inject_materialized_context: ")
	b.WriteString(fmt.Sprintf("%t", result.WouldInjectMaterializedContext))
	b.WriteByte('\n')
	b.WriteString("- execution_supported_now: ")
	b.WriteString(fmt.Sprintf("%t", result.ExecutionSupportedNow))
	b.WriteByte('\n')
	b.WriteString("- reason: ")
	b.WriteString(result.Reason)
	b.WriteByte('\n')
	b.WriteString("- prompt_section_title: ")
	b.WriteString(result.PromptSectionTitle)
	b.WriteByte('\n')
	b.WriteString("- materialized_sha256: ")
	b.WriteString(result.MaterializedSHA256)
	b.WriteByte('\n')
	b.WriteString("- total_chars_included: ")
	b.WriteString(fmt.Sprintf("%d", result.TotalCharsIncluded))
	b.WriteByte('\n')
	b.WriteString("- included_chunk_count: ")
	b.WriteString(fmt.Sprintf("%d", result.IncludedChunkCount))
	b.WriteByte('\n')
	b.WriteString("- required_future_flag: ")
	b.WriteString(result.RequiredFutureFlag)
	b.WriteByte('\n')
	if len(result.Warnings) > 0 {
		b.WriteString("\n## Warnings\n\n")
		for _, warning := range result.Warnings {
			b.WriteString("- ")
			b.WriteString(warning)
			b.WriteByte('\n')
		}
	}
	if strings.Contains(b.String(), "text_excerpt") || strings.Contains(b.String(), "alpha text") {
		return fmt.Errorf("injection execution plan summary must not contain materialized text")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write injection execution plan summary %q: %w", path, err)
	}
	return nil
}
