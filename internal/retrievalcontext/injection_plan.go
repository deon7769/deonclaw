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
	InjectionPlanReasonRunnerInjectionNotAllowed = "runner_injection_allowed_false"
	RequiredFutureInjectFlag                     = "--confirm-inject-materialized-context"
)

type InjectionPlanOptions struct {
	ApprovalPath     string
	RequestPath      string
	BundlePath       string
	MaterializedPath string
	OutputPath       string
	SummaryPath      string
}

type InjectionPlanResult struct {
	Status                   string   `json:"status"`
	CanInjectNow             bool     `json:"can_inject_now"`
	Reason                   string   `json:"reason"`
	ApprovedFor              string   `json:"approved_for"`
	MaterializedTextArtifact string   `json:"materialized_text_artifact"`
	IncludedChunkCount       int      `json:"included_chunk_count"`
	OmittedChunkCount        int      `json:"omitted_chunk_count"`
	TotalCharsIncluded       int      `json:"total_chars_included"`
	EstimatedPromptChars     int      `json:"estimated_prompt_chars"`
	RequiredFutureFlag       string   `json:"required_future_flag"`
	Warnings                 []string `json:"warnings,omitempty"`
}

func InjectionPlan(opts InjectionPlanOptions) (InjectionPlanResult, error) {
	if err := validateRelativeSafePath("approval path", opts.ApprovalPath); err != nil {
		return InjectionPlanResult{}, err
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return InjectionPlanResult{}, err
	}
	if err := validateRelativeSafePath("bundle path", opts.BundlePath); err != nil {
		return InjectionPlanResult{}, err
	}
	if err := validateRelativeSafePath("materialized path", opts.MaterializedPath); err != nil {
		return InjectionPlanResult{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return InjectionPlanResult{}, err
	}
	if err := validateRelativeSafePath("summary path", opts.SummaryPath); err != nil {
		return InjectionPlanResult{}, err
	}

	inspect, err := InspectApproval(opts.ApprovalPath, InspectApprovalOptions{RequestPath: opts.RequestPath})
	if err != nil {
		return InjectionPlanResult{}, err
	}
	if inspect.Status != lancedbpolicy.StatusOK {
		if len(inspect.Failures) > 0 {
			return InjectionPlanResult{}, fmt.Errorf("approval inspect status %q: %s", inspect.Status, strings.Join(inspect.Failures, "; "))
		}
		return InjectionPlanResult{}, fmt.Errorf("approval inspect status %q", inspect.Status)
	}

	approval, err := LoadMaterializedContextApproval(opts.ApprovalPath)
	if err != nil {
		return InjectionPlanResult{}, err
	}
	if err := validateInjectionPlanApproval(approval); err != nil {
		return InjectionPlanResult{}, err
	}

	bundle, bundleData, err := LoadBundle(opts.BundlePath)
	if err != nil {
		return InjectionPlanResult{}, err
	}
	if bundle.Status == lancedbpolicy.StatusFailed {
		return InjectionPlanResult{}, fmt.Errorf("bundle status %q", bundle.Status)
	}

	materializedData, err := os.ReadFile(opts.MaterializedPath)
	if err != nil {
		return InjectionPlanResult{}, fmt.Errorf("read materialized artifact %q: %w", opts.MaterializedPath, err)
	}

	bundleSHA := sha256Hex(bundleData)
	materializedSHA := sha256Hex(materializedData)
	if approval.BundleSHA256 != bundleSHA {
		return InjectionPlanResult{}, fmt.Errorf("approval bundle_sha256 mismatch")
	}
	if approval.MaterializedSHA256 != materializedSHA {
		return InjectionPlanResult{}, fmt.Errorf("approval materialized_sha256 mismatch")
	}
	if bundle.MaterializedSHA256 != materializedSHA {
		return InjectionPlanResult{}, fmt.Errorf("bundle materialized_sha256 mismatch")
	}

	materializedReport, err := MaterializedReportBytes(materializedData)
	if err != nil {
		return InjectionPlanResult{}, err
	}
	if materializedReport.Status == lancedbpolicy.StatusFailed {
		return InjectionPlanResult{}, fmt.Errorf("materialized report status %q", materializedReport.Status)
	}

	warnings := mergeWarnings(approval.Warnings, bundle.Warnings, materializedReport.Warnings)
	status := lancedbpolicy.StatusOK
	if materializedReport.Status == lancedbpolicy.StatusWarning || bundle.Status == lancedbpolicy.StatusWarning || len(warnings) > 0 {
		status = lancedbpolicy.StatusWarning
	}

	result := InjectionPlanResult{
		Status:                   status,
		CanInjectNow:             false,
		Reason:                   InjectionPlanReasonRunnerInjectionNotAllowed,
		ApprovedFor:              AllowedUseManualReviewOnly,
		MaterializedTextArtifact: bundle.MaterializedTextArtifact,
		IncludedChunkCount:       materializedReport.IncludedChunkCount,
		OmittedChunkCount:        materializedReport.OmittedChunkCount,
		TotalCharsIncluded:       materializedReport.TotalCharsIncluded,
		EstimatedPromptChars:     materializedReport.TotalCharsIncluded,
		RequiredFutureFlag:       RequiredFutureInjectFlag,
		Warnings:                 warnings,
	}

	if err := writeInjectionPlanJSON(opts.OutputPath, result); err != nil {
		return InjectionPlanResult{}, err
	}
	if err := writeInjectionPlanSummary(opts.SummaryPath, result); err != nil {
		return InjectionPlanResult{}, err
	}
	return result, nil
}

func validateInjectionPlanApproval(approval MaterializedContextApproval) error {
	if !approval.Approved {
		return fmt.Errorf("approval approved must be true")
	}
	if approval.AllowedUse != AllowedUseManualReviewOnly {
		return fmt.Errorf("approval allowed_use %q must be %q", approval.AllowedUse, AllowedUseManualReviewOnly)
	}
	if approval.RunnerInjectionAllowed {
		return fmt.Errorf("runner injection is not supported yet; approval runner_injection_allowed must be false")
	}
	return nil
}

func mergeWarnings(groups ...[]string) []string {
	seen := map[string]struct{}{}
	var merged []string
	for _, group := range groups {
		for _, warning := range group {
			warning = strings.TrimSpace(warning)
			if warning == "" {
				continue
			}
			if _, ok := seen[warning]; ok {
				continue
			}
			seen[warning] = struct{}{}
			merged = append(merged, warning)
		}
	}
	return merged
}

func writeInjectionPlanJSON(path string, result InjectionPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal injection plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("injection plan must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write injection plan json %q: %w", path, err)
	}
	return nil
}

func writeInjectionPlanSummary(path string, result InjectionPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection plan summary dir: %w", err)
	}
	var b strings.Builder
	b.WriteString("# Materialized retrieval context injection plan\n\n")
	b.WriteString("Plan-only artifact. Does not inject materialized text into runner prompts.\n")
	b.WriteString("Current approvals allow manual review only; runner injection remains disabled.\n\n")
	b.WriteString("- status: ")
	b.WriteString(result.Status)
	b.WriteByte('\n')
	b.WriteString("- can_inject_now: ")
	b.WriteString(fmt.Sprintf("%t", result.CanInjectNow))
	b.WriteByte('\n')
	b.WriteString("- reason: ")
	b.WriteString(result.Reason)
	b.WriteByte('\n')
	b.WriteString("- approved_for: ")
	b.WriteString(result.ApprovedFor)
	b.WriteByte('\n')
	b.WriteString("- materialized_text_artifact: ")
	b.WriteString(result.MaterializedTextArtifact)
	b.WriteByte('\n')
	b.WriteString("- included_chunk_count: ")
	b.WriteString(fmt.Sprintf("%d", result.IncludedChunkCount))
	b.WriteByte('\n')
	b.WriteString("- omitted_chunk_count: ")
	b.WriteString(fmt.Sprintf("%d", result.OmittedChunkCount))
	b.WriteByte('\n')
	b.WriteString("- total_chars_included: ")
	b.WriteString(fmt.Sprintf("%d", result.TotalCharsIncluded))
	b.WriteByte('\n')
	b.WriteString("- estimated_prompt_chars: ")
	b.WriteString(fmt.Sprintf("%d", result.EstimatedPromptChars))
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
	if strings.Contains(b.String(), "text_excerpt") {
		return fmt.Errorf("injection plan summary must not contain text_excerpt")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write injection plan summary %q: %w", path, err)
	}
	return nil
}
