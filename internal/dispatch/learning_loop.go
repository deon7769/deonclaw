package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/insights"
)

type LearningLoopResult struct {
	Materialized         bool   `json:"materialized"`
	Skipped              bool   `json:"skipped"`
	Reason               string `json:"reason,omitempty"`
	InsightID            string `json:"insight_id,omitempty"`
	InsightReportPath    string `json:"insight_report_path,omitempty"`
	ProposalBundlePath   string `json:"proposal_bundle_path,omitempty"`
	ProposalCount        int    `json:"proposal_count,omitempty"`
	EvidenceBundleID     string `json:"evidence_bundle_id,omitempty"`
	EvidenceBundleSHA256 string `json:"evidence_bundle_sha256,omitempty"`
	Reviewer             string `json:"reviewer,omitempty"`
}

type InsightReviewLearningOptions struct {
	WorkItem             agents.WorkItem
	Reviewer             string
	ReviewerResponsePath string
	Policy               insights.Policy
	ArtifactsDir         string
	RunID                string
	Now                  time.Time
}

func MaterializeInsightReviewLearning(opts InsightReviewLearningOptions) (LearningLoopResult, error) {
	if opts.WorkItem.Kind != agents.WorkItemKindInsightReview {
		return LearningLoopResult{Skipped: true, Reason: "not insight_review work"}, nil
	}
	if !opts.Policy.Enabled {
		return LearningLoopResult{Skipped: true, Reason: "insight policy disabled"}, nil
	}
	responsePath := strings.TrimSpace(opts.ReviewerResponsePath)
	if responsePath == "" {
		return LearningLoopResult{Skipped: true, Reason: "reviewer response fixture not configured"}, nil
	}
	evidencePath := strings.TrimSpace(opts.WorkItem.EvidencePath)
	if evidencePath == "" {
		return LearningLoopResult{}, fmt.Errorf("insight review evidence path is required")
	}
	artifactsDir := strings.TrimSpace(opts.ArtifactsDir)
	if artifactsDir == "" {
		return LearningLoopResult{}, fmt.Errorf("artifacts dir is required for insight review materialization")
	}
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		return LearningLoopResult{}, fmt.Errorf("run id is required for insight review materialization")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	reviewer := resolveInsightReviewer(opts.Policy, opts.Reviewer)

	evidence, err := insights.ReadEvidenceJSON(evidencePath)
	if err != nil {
		return LearningLoopResult{}, err
	}
	response, err := insights.ReadReviewerResponse(responsePath)
	if err != nil {
		return LearningLoopResult{}, err
	}
	report, err := insights.MaterializeInsightReport(insights.EvaluateOptions{
		Evidence: evidence, Reviewer: reviewer, ResponsePath: responsePath, Now: now,
	})
	if err != nil {
		return LearningLoopResult{}, err
	}

	outDir := filepath.Join(artifactsDir, "insights", runID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return LearningLoopResult{}, fmt.Errorf("create insight review artifact dir: %w", err)
	}
	reportPath := filepath.Join(outDir, "insight-report.json")
	if err := insights.WriteReportJSON(report, reportPath); err != nil {
		return LearningLoopResult{}, err
	}

	result := LearningLoopResult{
		Materialized:         true,
		InsightID:            report.InsightID,
		InsightReportPath:    reportPath,
		EvidenceBundleID:     evidence.EvidenceBundleID,
		EvidenceBundleSHA256: evidence.SHA256,
		Reviewer:             reviewer,
	}
	if !opts.Policy.AutoPropose {
		return result, nil
	}

	proposalBundle, err := insights.MaterializeProposals(insights.MaterializeProposalsOptions{
		Report: report, Response: response, Policy: opts.Policy, Now: now,
	})
	if err != nil {
		return LearningLoopResult{}, err
	}
	proposalPath := filepath.Join(outDir, "learning-proposals.json")
	if err := insights.WriteProposalBundleJSON(proposalBundle, proposalPath); err != nil {
		return LearningLoopResult{}, err
	}
	result.ProposalBundlePath = proposalPath
	result.ProposalCount = len(proposalBundle.Proposals)
	return result, nil
}

func resolveInsightReviewer(policy insights.Policy, fallback string) string {
	for _, candidate := range []string{policy.Reviewer.Preferred, fallback, policy.Reviewer.Fallback} {
		switch strings.TrimSpace(candidate) {
		case insights.ReviewerCodex:
			return insights.ReviewerCodex
		case insights.ReviewerOpenCode:
			return insights.ReviewerOpenCode
		}
	}
	return insights.ReviewerCodex
}
