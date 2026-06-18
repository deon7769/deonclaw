package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type InjectionGovernanceBundleOptions struct {
	GovernanceReportPath         string
	PolicyPath                   string
	InjectionApprovalRequestPath string
	InjectionApprovalPath        string
	ExecutionPlanPath            string
	PromptPreviewManifestPath    string
	PromptPreviewReportPath      string
	OutputPath                   string
	SummaryPath                  string
}

type InjectionGovernanceBundleCaps struct {
	MaxTotalChars    int `json:"max_total_chars"`
	MaxCharsPerChunk int `json:"max_chars_per_chunk"`
	MaxChunks        int `json:"max_chunks"`
}

type InjectionGovernanceBundleResult struct {
	Status                       string                        `json:"status"`
	ContainsText                 bool                          `json:"contains_text"`
	RunnerExecution              bool                          `json:"runner_execution"`
	InjectionAuthorizedForFuture bool                          `json:"injection_authorized_for_future"`
	ExecutionSupportedNow        bool                          `json:"execution_supported_now"`
	MaterializedSHA256           string                        `json:"materialized_sha256"`
	PolicySHA256                 string                        `json:"policy_sha256"`
	GovernanceReportSHA256       string                        `json:"governance_report_sha256"`
	ApprovalSHA256               string                        `json:"approval_sha256"`
	ExecutionPlanSHA256          string                        `json:"execution_plan_sha256"`
	PromptPreviewManifestSHA256  string                        `json:"prompt_preview_manifest_sha256"`
	PromptPreviewSHA256          string                        `json:"prompt_preview_sha256"`
	Caps                         InjectionGovernanceBundleCaps `json:"caps"`
	Warnings                     []string                      `json:"warnings,omitempty"`
	Failures                     []string                      `json:"failures,omitempty"`
}

func InjectionGovernanceBundle(opts InjectionGovernanceBundleOptions) (InjectionGovernanceBundleResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"governance report path", opts.GovernanceReportPath},
		{"policy path", opts.PolicyPath},
		{"injection approval request path", opts.InjectionApprovalRequestPath},
		{"injection approval path", opts.InjectionApprovalPath},
		{"execution plan path", opts.ExecutionPlanPath},
		{"prompt preview manifest path", opts.PromptPreviewManifestPath},
		{"prompt preview report path", opts.PromptPreviewReportPath},
		{"output path", opts.OutputPath},
		{"summary path", opts.SummaryPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return InjectionGovernanceBundleResult{}, err
		}
	}

	governanceReport, governanceData, err := LoadGovernanceReport(opts.GovernanceReportPath)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	if err := validateGovernanceReportForInjectionApproval(governanceReport); err != nil {
		return InjectionGovernanceBundleResult{}, err
	}

	policyData, err := readArtifactBytesNoTextExcerpt("injection policy", opts.PolicyPath)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	cfg, err := ParseInjectionPolicy(policyData)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	if err := ValidateInjectionPolicy(cfg); err != nil {
		return InjectionGovernanceBundleResult{}, fmt.Errorf("injection policy invalid: %w", err)
	}
	policySHA := sha256Hex(policyData)

	approvalInspect, err := InspectInjectionApproval(opts.InjectionApprovalPath, InspectInjectionApprovalOptions{
		RequestPath: opts.InjectionApprovalRequestPath,
	})
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	if approvalInspect.Status != lancedbpolicy.StatusOK {
		if len(approvalInspect.Failures) > 0 {
			return InjectionGovernanceBundleResult{}, fmt.Errorf("injection approval inspect status %q: %s", approvalInspect.Status, strings.Join(approvalInspect.Failures, "; "))
		}
		return InjectionGovernanceBundleResult{}, fmt.Errorf("injection approval inspect status %q", approvalInspect.Status)
	}

	approvalData, err := readArtifactBytesNoTextExcerpt("injection approval", opts.InjectionApprovalPath)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	approval, err := ParseRunnerInjectionApprovalJSON(approvalData)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	if err := validateInjectionExecutionPlanApproval(approval); err != nil {
		return InjectionGovernanceBundleResult{}, fmt.Errorf("injection approval invalid: %w", err)
	}
	approvalSHA := sha256Hex(approvalData)

	executionPlanData, err := readArtifactBytesNoTextExcerpt("execution plan", opts.ExecutionPlanPath)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	executionPlan, err := ParseInjectionExecutionPlanJSON(executionPlanData)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	if err := validateExecutionPlanForPromptPreview(executionPlan); err != nil {
		return InjectionGovernanceBundleResult{}, fmt.Errorf("execution plan invalid: %w", err)
	}
	executionPlanSHA := sha256Hex(executionPlanData)

	manifest, manifestData, err := LoadPromptPreviewManifest(opts.PromptPreviewManifestPath)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	manifestSHA := sha256Hex(manifestData)

	previewReport, _, err := LoadPromptPreviewReport(opts.PromptPreviewReportPath)
	if err != nil {
		return InjectionGovernanceBundleResult{}, err
	}

	governanceSHA := sha256Hex(governanceData)

	var failures []string
	if approval.PolicySHA256 != policySHA {
		failures = append(failures, "approval policy_sha256 mismatch")
	}
	if approval.GovernanceReportSHA256 != governanceSHA {
		failures = append(failures, "approval governance_report_sha256 mismatch")
	}
	if approval.MaterializedSHA256 != executionPlan.MaterializedSHA256 {
		failures = append(failures, "approval materialized_sha256 mismatch with execution plan")
	}
	if previewReport.Hashes.PromptPreviewSHA256 != manifest.PromptPreviewSHA256 {
		failures = append(failures, "prompt preview report prompt_preview_sha256 mismatch with manifest")
	}
	if previewReport.Hashes.MaterializedSHA256 != executionPlan.MaterializedSHA256 {
		failures = append(failures, "prompt preview report materialized_sha256 mismatch with execution plan")
	}
	if !approval.RunnerInjectionAllowed {
		failures = append(failures, "approval runner_injection_allowed must be true")
	}
	if executionPlan.WouldExecuteRunner {
		failures = append(failures, "execution plan would_execute_runner must be false")
	}
	if executionPlan.ExecutionSupportedNow {
		failures = append(failures, "execution plan execution_supported_now must be false")
	}
	if !manifest.PreviewOnly {
		failures = append(failures, "prompt preview manifest preview_only must be true")
	}
	if manifest.RunnerExecution {
		failures = append(failures, "prompt preview manifest runner_execution must be false")
	}
	if previewReport.Status != lancedbpolicy.StatusOK {
		failures = append(failures, fmt.Sprintf("prompt preview report status %q must be ok", previewReport.Status))
	}
	if len(failures) > 0 {
		return InjectionGovernanceBundleResult{}, fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	warnings := mergeWarnings(
		append([]string(nil), governanceReport.Warnings...),
		append([]string(nil), approvalInspect.Warnings...),
		append([]string(nil), approval.Warnings...),
		append([]string(nil), executionPlan.Warnings...),
		append([]string(nil), previewReport.Warnings...),
	)
	if governanceReport.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"governance report status is warning"})
	}
	if executionPlan.Status == lancedbpolicy.StatusWarning {
		warnings = mergeWarnings(warnings, []string{"execution plan status is warning"})
	}

	status := lancedbpolicy.StatusOK
	if len(warnings) > 0 {
		status = lancedbpolicy.StatusWarning
	}

	result := InjectionGovernanceBundleResult{
		Status:                       status,
		ContainsText:                 false,
		RunnerExecution:              false,
		InjectionAuthorizedForFuture: true,
		ExecutionSupportedNow:        false,
		MaterializedSHA256:           executionPlan.MaterializedSHA256,
		PolicySHA256:                 policySHA,
		GovernanceReportSHA256:       governanceSHA,
		ApprovalSHA256:               approvalSHA,
		ExecutionPlanSHA256:          executionPlanSHA,
		PromptPreviewManifestSHA256:  manifestSHA,
		PromptPreviewSHA256:          manifest.PromptPreviewSHA256,
		Caps: InjectionGovernanceBundleCaps{
			MaxTotalChars:    approval.MaxTotalChars,
			MaxCharsPerChunk: approval.MaxCharsPerChunk,
			MaxChunks:        approval.MaxChunks,
		},
		Warnings: warnings,
	}

	if err := writeInjectionGovernanceBundleJSON(opts.OutputPath, result); err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	if err := writeInjectionGovernanceBundleSummary(opts.SummaryPath, result); err != nil {
		return InjectionGovernanceBundleResult{}, err
	}
	return result, nil
}

func LoadInjectionGovernanceBundle(path string) (InjectionGovernanceBundleResult, []byte, error) {
	if err := validateRelativeSafePath("governance bundle path", path); err != nil {
		return InjectionGovernanceBundleResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("governance bundle", path)
	if err != nil {
		return InjectionGovernanceBundleResult{}, nil, err
	}
	return ParseInjectionGovernanceBundleJSON(data)
}

func ParseInjectionGovernanceBundleJSON(data []byte) (InjectionGovernanceBundleResult, []byte, error) {
	if strings.Contains(string(data), `"text_excerpt"`) {
		return InjectionGovernanceBundleResult{}, nil, fmt.Errorf("governance bundle must not contain text_excerpt")
	}
	var result InjectionGovernanceBundleResult
	if err := json.Unmarshal(data, &result); err != nil {
		return InjectionGovernanceBundleResult{}, nil, fmt.Errorf("parse governance bundle json: %w", err)
	}
	return result, data, nil
}

func ValidateInjectionGovernanceBundleForTaskDeclaration(bundle InjectionGovernanceBundleResult) error {
	var failures []string
	if bundle.ContainsText {
		failures = append(failures, "governance bundle contains_text must be false")
	}
	if bundle.RunnerExecution {
		failures = append(failures, "governance bundle runner_execution must be false")
	}
	if !bundle.InjectionAuthorizedForFuture {
		failures = append(failures, "governance bundle injection_authorized_for_future must be true")
	}
	if bundle.ExecutionSupportedNow {
		failures = append(failures, "governance bundle execution_supported_now must be false")
	}
	switch bundle.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		failures = append(failures, fmt.Sprintf("governance bundle status %q must be ok or warning", bundle.Status))
	}
	if len(failures) > 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func readArtifactBytesNoTextExcerpt(label, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s %q: %w", label, path, err)
	}
	if strings.Contains(string(data), `"text_excerpt"`) {
		return nil, fmt.Errorf("%s %q must not contain text_excerpt", label, path)
	}
	return data, nil
}

func writeInjectionGovernanceBundleJSON(path string, result InjectionGovernanceBundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection governance bundle output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal injection governance bundle json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") {
		return fmt.Errorf("injection governance bundle must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write injection governance bundle %q: %w", path, err)
	}
	return nil
}

func writeInjectionGovernanceBundleSummary(path string, result InjectionGovernanceBundleResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection governance bundle summary dir: %w", err)
	}
	var b strings.Builder
	b.WriteString("# Retrieval context injection governance bundle\n\n")
	b.WriteString("Metadata-only release bundle for the injection governance chain.\n")
	b.WriteString("Does not contain preview markdown or materialized chunk text.\n\n")
	b.WriteString("- status: ")
	b.WriteString(result.Status)
	b.WriteByte('\n')
	b.WriteString("- contains_text: ")
	b.WriteString(fmt.Sprintf("%t", result.ContainsText))
	b.WriteByte('\n')
	b.WriteString("- runner_execution: ")
	b.WriteString(fmt.Sprintf("%t", result.RunnerExecution))
	b.WriteByte('\n')
	b.WriteString("- injection_authorized_for_future: ")
	b.WriteString(fmt.Sprintf("%t", result.InjectionAuthorizedForFuture))
	b.WriteByte('\n')
	b.WriteString("- execution_supported_now: ")
	b.WriteString(fmt.Sprintf("%t", result.ExecutionSupportedNow))
	b.WriteByte('\n')
	b.WriteString("- materialized_sha256: ")
	b.WriteString(result.MaterializedSHA256)
	b.WriteByte('\n')
	b.WriteString("- policy_sha256: ")
	b.WriteString(result.PolicySHA256)
	b.WriteByte('\n')
	b.WriteString("- governance_report_sha256: ")
	b.WriteString(result.GovernanceReportSHA256)
	b.WriteByte('\n')
	b.WriteString("- approval_sha256: ")
	b.WriteString(result.ApprovalSHA256)
	b.WriteByte('\n')
	b.WriteString("- execution_plan_sha256: ")
	b.WriteString(result.ExecutionPlanSHA256)
	b.WriteByte('\n')
	b.WriteString("- prompt_preview_manifest_sha256: ")
	b.WriteString(result.PromptPreviewManifestSHA256)
	b.WriteByte('\n')
	b.WriteString("- prompt_preview_sha256: ")
	b.WriteString(result.PromptPreviewSHA256)
	b.WriteByte('\n')
	b.WriteString("- max_total_chars: ")
	b.WriteString(fmt.Sprintf("%d", result.Caps.MaxTotalChars))
	b.WriteByte('\n')
	b.WriteString("- max_chars_per_chunk: ")
	b.WriteString(fmt.Sprintf("%d", result.Caps.MaxCharsPerChunk))
	b.WriteByte('\n')
	b.WriteString("- max_chunks: ")
	b.WriteString(fmt.Sprintf("%d", result.Caps.MaxChunks))
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
		return fmt.Errorf("injection governance bundle summary must not contain materialized text")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write injection governance bundle summary %q: %w", path, err)
	}
	return nil
}
