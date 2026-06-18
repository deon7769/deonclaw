package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type GovernanceReportOptions struct {
	RetrievalContextPath string
	MaterializedPath     string
	BundlePath           string
	RequestPath          string
	ApprovalPath         string
	InjectionPlanPath    string
}

type GovernanceStageStatus struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type GovernanceHashes struct {
	RetrievalContextSHA256     string `json:"retrieval_context_sha256"`
	MaterializedSHA256         string `json:"materialized_sha256"`
	BundleSHA256               string `json:"bundle_sha256"`
	RequestSHA256              string `json:"request_sha256"`
	ApprovalRequestSHA256      string `json:"approval_request_sha256"`
	ApprovalBundleSHA256       string `json:"approval_bundle_sha256"`
	ApprovalMaterializedSHA256 string `json:"approval_materialized_sha256"`
}

type GovernanceCounts struct {
	RetrievalHitCount    int `json:"retrieval_hit_count"`
	IncludedChunkCount   int `json:"included_chunk_count"`
	OmittedChunkCount    int `json:"omitted_chunk_count"`
	TotalCharsIncluded   int `json:"total_chars_included"`
	EstimatedPromptChars int `json:"estimated_prompt_chars"`
}

type GovernanceReportResult struct {
	Status                   string                  `json:"status"`
	Stages                   []GovernanceStageStatus `json:"stages"`
	Hashes                   GovernanceHashes        `json:"hashes"`
	Counts                   GovernanceCounts        `json:"counts"`
	Warnings                 []string                `json:"warnings"`
	Failures                 []string                `json:"failures,omitempty"`
	CanInjectNow             bool                    `json:"can_inject_now"`
	RequiredFutureFlag       string                  `json:"required_future_flag,omitempty"`
	MaterializedTextArtifact string                  `json:"materialized_text_artifact,omitempty"`
}

func GovernanceReport(opts GovernanceReportOptions) (GovernanceReportResult, error) {
	if err := validateRelativeSafePath("retrieval context path", opts.RetrievalContextPath); err != nil {
		return GovernanceReportResult{}, err
	}
	if err := validateRelativeSafePath("materialized path", opts.MaterializedPath); err != nil {
		return GovernanceReportResult{}, err
	}
	if err := validateRelativeSafePath("bundle path", opts.BundlePath); err != nil {
		return GovernanceReportResult{}, err
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return GovernanceReportResult{}, err
	}
	if err := validateRelativeSafePath("approval path", opts.ApprovalPath); err != nil {
		return GovernanceReportResult{}, err
	}
	if err := validateRelativeSafePath("injection plan path", opts.InjectionPlanPath); err != nil {
		return GovernanceReportResult{}, err
	}

	result := GovernanceReportResult{
		Status: lancedbpolicy.StatusOK,
	}

	retrievalData, err := os.ReadFile(opts.RetrievalContextPath)
	if err != nil {
		return GovernanceReportResult{}, fmt.Errorf("read retrieval context %q: %w", opts.RetrievalContextPath, err)
	}
	retrievalInspect, err := InspectArtifactBytes(retrievalData)
	if err != nil {
		return GovernanceReportResult{}, err
	}
	result.Stages = append(result.Stages, GovernanceStageStatus{Name: "retrieval_context", Status: retrievalInspect.Status})
	result.Warnings = mergeWarnings(result.Warnings, retrievalInspect.Warnings)
	result.Failures = append(result.Failures, retrievalInspect.Failures...)
	retrievalSHA := sha256Hex(retrievalData)
	result.Hashes.RetrievalContextSHA256 = retrievalSHA
	result.Counts.RetrievalHitCount = retrievalInspect.HitCount

	materializedData, err := os.ReadFile(opts.MaterializedPath)
	if err != nil {
		return GovernanceReportResult{}, fmt.Errorf("read materialized artifact %q: %w", opts.MaterializedPath, err)
	}
	materializedReport, err := MaterializedReportBytes(materializedData)
	if err != nil {
		return GovernanceReportResult{}, err
	}
	result.Stages = append(result.Stages, GovernanceStageStatus{Name: "materialized", Status: materializedReport.Status})
	result.Warnings = mergeWarnings(result.Warnings, materializedReport.Warnings)
	result.Failures = append(result.Failures, materializedReport.Failures...)
	materializedSHA := sha256Hex(materializedData)
	result.Hashes.MaterializedSHA256 = materializedSHA
	result.Counts.IncludedChunkCount = materializedReport.IncludedChunkCount
	result.Counts.OmittedChunkCount = materializedReport.OmittedChunkCount
	result.Counts.TotalCharsIncluded = materializedReport.TotalCharsIncluded

	var materializedFile materializedArtifactFile
	if err := json.Unmarshal(materializedData, &materializedFile); err != nil {
		return GovernanceReportResult{}, fmt.Errorf("parse materialized artifact: %w", err)
	}

	bundle, bundleData, err := LoadBundle(opts.BundlePath)
	if err != nil {
		return GovernanceReportResult{}, err
	}
	bundleStage := lancedbpolicy.StatusOK
	if bundle.Status == lancedbpolicy.StatusFailed {
		bundleStage = lancedbpolicy.StatusFailed
	} else if bundle.Status == lancedbpolicy.StatusWarning {
		bundleStage = lancedbpolicy.StatusWarning
	}
	result.Stages = append(result.Stages, GovernanceStageStatus{Name: "bundle", Status: bundleStage})
	result.Warnings = mergeWarnings(result.Warnings, bundle.Warnings)
	result.Failures = append(result.Failures, validateBundleBasic(bundle, bundleData)...)
	bundleSHA := sha256Hex(bundleData)
	result.Hashes.BundleSHA256 = bundleSHA
	result.MaterializedTextArtifact = bundle.MaterializedTextArtifact
	if result.Counts.IncludedChunkCount == 0 {
		result.Counts.IncludedChunkCount = bundle.IncludedChunkCount
	}
	if result.Counts.OmittedChunkCount == 0 && bundle.OmittedChunkCount > 0 {
		result.Counts.OmittedChunkCount = bundle.OmittedChunkCount
	}
	if result.Counts.TotalCharsIncluded == 0 {
		result.Counts.TotalCharsIncluded = bundle.TotalCharsIncluded
	}

	requestData, err := os.ReadFile(opts.RequestPath)
	if err != nil {
		return GovernanceReportResult{}, fmt.Errorf("read approval request %q: %w", opts.RequestPath, err)
	}
	request, err := ParseApprovalRequestJSON(requestData)
	if err != nil {
		return GovernanceReportResult{}, err
	}
	requestStage := lancedbpolicy.StatusOK
	if err := request.Validate(); err != nil {
		requestStage = lancedbpolicy.StatusFailed
		result.Failures = append(result.Failures, fmt.Sprintf("approval request: %v", err))
	}
	result.Stages = append(result.Stages, GovernanceStageStatus{Name: "approval_request", Status: requestStage})
	result.Warnings = mergeWarnings(result.Warnings, request.Warnings)
	requestSHA := sha256Hex(requestData)
	result.Hashes.RequestSHA256 = requestSHA

	approvalInspect, err := InspectApproval(opts.ApprovalPath, InspectApprovalOptions{RequestPath: opts.RequestPath})
	if err != nil {
		return GovernanceReportResult{}, err
	}
	result.Stages = append(result.Stages, GovernanceStageStatus{Name: "approval", Status: approvalInspect.Status})
	result.Warnings = mergeWarnings(result.Warnings, approvalInspect.Warnings)
	result.Failures = append(result.Failures, approvalInspect.Failures...)
	approval, err := LoadMaterializedContextApproval(opts.ApprovalPath)
	if err != nil {
		return GovernanceReportResult{}, err
	}
	result.Hashes.ApprovalRequestSHA256 = approval.RequestSHA256
	result.Hashes.ApprovalBundleSHA256 = approval.BundleSHA256
	result.Hashes.ApprovalMaterializedSHA256 = approval.MaterializedSHA256

	injectionPlan, err := LoadInjectionPlan(opts.InjectionPlanPath)
	if err != nil {
		return GovernanceReportResult{}, err
	}
	injectionStage := injectionPlan.Status
	if injectionStage == "" {
		injectionStage = lancedbpolicy.StatusOK
	}
	result.Stages = append(result.Stages, GovernanceStageStatus{Name: "injection_plan", Status: injectionStage})
	result.Warnings = mergeWarnings(result.Warnings, injectionPlan.Warnings)
	result.CanInjectNow = injectionPlan.CanInjectNow
	result.RequiredFutureFlag = injectionPlan.RequiredFutureFlag
	result.Counts.EstimatedPromptChars = injectionPlan.EstimatedPromptChars
	if result.MaterializedTextArtifact == "" {
		result.MaterializedTextArtifact = injectionPlan.MaterializedTextArtifact
	}

	result.Failures = append(result.Failures, validateGovernanceCoherence(
		retrievalSHA,
		materializedSHA,
		bundleSHA,
		requestSHA,
		materializedFile.SourceRetrievalContextSHA256,
		bundle,
		request,
		approval,
		injectionPlan,
	)...)

	result.Status = aggregateGovernanceStatus(result.Stages, result.Failures, result.Warnings)
	return result, nil
}

func validateBundleBasic(bundle BundleResult, bundleData []byte) []string {
	var failures []string
	if bundle.Status == lancedbpolicy.StatusFailed {
		failures = append(failures, fmt.Sprintf("bundle status %q", bundle.Status))
	}
	if bundle.ContainsText {
		failures = append(failures, "bundle contains_text must be false")
	}
	if strings.Contains(string(bundleData), `"text_excerpt"`) {
		failures = append(failures, "bundle contains forbidden field text_excerpt")
	}
	return failures
}

func validateGovernanceCoherence(
	retrievalSHA string,
	materializedSHA string,
	bundleSHA string,
	requestSHA string,
	materializedSourceRetrievalSHA string,
	bundle BundleResult,
	request ApprovalRequest,
	approval MaterializedContextApproval,
	injectionPlan InjectionPlanResult,
) []string {
	var failures []string
	if strings.TrimSpace(materializedSourceRetrievalSHA) != retrievalSHA {
		failures = append(failures, "materialized source_retrieval_context_sha256 mismatch")
	}
	if bundle.RetrievalContextSHA256 != retrievalSHA {
		failures = append(failures, "bundle retrieval_context_sha256 mismatch")
	}
	if bundle.MaterializedSHA256 != materializedSHA {
		failures = append(failures, "bundle materialized_sha256 mismatch")
	}
	if request.BundleSHA256 != bundleSHA {
		failures = append(failures, "request bundle_sha256 mismatch")
	}
	if approval.RequestSHA256 != requestSHA {
		failures = append(failures, "approval request_sha256 mismatch")
	}
	if approval.BundleSHA256 != request.BundleSHA256 {
		failures = append(failures, "approval bundle_sha256 mismatch with request")
	}
	if approval.MaterializedSHA256 != request.MaterializedSHA256 {
		failures = append(failures, "approval materialized_sha256 mismatch with request")
	}
	if injectionPlan.MaterializedTextArtifact != bundle.MaterializedTextArtifact {
		failures = append(failures, "injection_plan materialized_text_artifact mismatch with bundle")
	}
	if injectionPlan.CanInjectNow {
		failures = append(failures, "injection_plan can_inject_now must be false")
	}
	if injectionPlan.Reason != InjectionPlanReasonRunnerInjectionNotAllowed {
		failures = append(failures, fmt.Sprintf("injection_plan reason %q must be %q", injectionPlan.Reason, InjectionPlanReasonRunnerInjectionNotAllowed))
	}
	return failures
}

func aggregateGovernanceStatus(stages []GovernanceStageStatus, failures, warnings []string) string {
	if len(failures) > 0 {
		return lancedbpolicy.StatusFailed
	}
	for _, stage := range stages {
		if stage.Status == lancedbpolicy.StatusFailed {
			return lancedbpolicy.StatusFailed
		}
	}
	for _, stage := range stages {
		if stage.Status == lancedbpolicy.StatusWarning {
			return lancedbpolicy.StatusWarning
		}
	}
	if len(warnings) > 0 {
		return lancedbpolicy.StatusWarning
	}
	return lancedbpolicy.StatusOK
}

func WriteGovernanceReportText(result GovernanceReportResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_governance_report:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "can_inject_now: %t\n", result.CanInjectNow); err != nil {
		return err
	}
	if result.RequiredFutureFlag != "" {
		if _, err := fmt.Fprintf(out, "required_future_flag: %s\n", result.RequiredFutureFlag); err != nil {
			return err
		}
	}
	if result.MaterializedTextArtifact != "" {
		if _, err := fmt.Fprintf(out, "materialized_text_artifact: %s\n", result.MaterializedTextArtifact); err != nil {
			return err
		}
	}
	if len(result.Stages) > 0 {
		if _, err := fmt.Fprintln(out, "\nstages:"); err != nil {
			return err
		}
		table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(table, "name\tstatus"); err != nil {
			return err
		}
		for _, stage := range result.Stages {
			if _, err := fmt.Fprintf(table, "%s\t%s\n", stage.Name, stage.Status); err != nil {
				return err
			}
		}
		if err := table.Flush(); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(out, "\ncounts:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "retrieval_hit_count: %d\n", result.Counts.RetrievalHitCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "included_chunk_count: %d\n", result.Counts.IncludedChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "omitted_chunk_count: %d\n", result.Counts.OmittedChunkCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "total_chars_included: %d\n", result.Counts.TotalCharsIncluded); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "estimated_prompt_chars: %d\n", result.Counts.EstimatedPromptChars); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "\nhashes:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "retrieval_context_sha256: %s\n", result.Hashes.RetrievalContextSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "materialized_sha256: %s\n", result.Hashes.MaterializedSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "bundle_sha256: %s\n", result.Hashes.BundleSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "request_sha256: %s\n", result.Hashes.RequestSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "approval_request_sha256: %s\n", result.Hashes.ApprovalRequestSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "approval_bundle_sha256: %s\n", result.Hashes.ApprovalBundleSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "approval_materialized_sha256: %s\n", result.Hashes.ApprovalMaterializedSHA256); err != nil {
		return err
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
	return nil
}

func WriteGovernanceReportJSON(result GovernanceReportResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal governance report json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("governance report json must not contain text_excerpt")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
