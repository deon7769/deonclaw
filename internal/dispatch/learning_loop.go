package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/skills"
)

type LearningLoopResult struct {
	Materialized          bool     `json:"materialized"`
	Skipped               bool     `json:"skipped"`
	Reason                string   `json:"reason,omitempty"`
	InsightID             string   `json:"insight_id,omitempty"`
	InsightReportPath     string   `json:"insight_report_path,omitempty"`
	ProposalBundlePath    string   `json:"proposal_bundle_path,omitempty"`
	ProposalCount         int      `json:"proposal_count,omitempty"`
	ApprovalPaths         []string `json:"approval_paths,omitempty"`
	ApprovalCount         int      `json:"approval_count,omitempty"`
	ApplyResultPaths      []string `json:"apply_result_paths,omitempty"`
	ApplyPreviewPaths     []string `json:"apply_preview_paths,omitempty"`
	ApplyCount            int      `json:"apply_count,omitempty"`
	EffectivenessPaths    []string `json:"effectiveness_paths,omitempty"`
	EffectivenessCount    int      `json:"effectiveness_count,omitempty"`
	SessionRefreshPath    string   `json:"session_refresh_path,omitempty"`
	SessionSnapshotPath   string   `json:"session_snapshot_path,omitempty"`
	SessionSnapshotSHA256 string   `json:"session_snapshot_sha256,omitempty"`
	EvidenceBundleID      string   `json:"evidence_bundle_id,omitempty"`
	EvidenceBundleSHA256  string   `json:"evidence_bundle_sha256,omitempty"`
	Reviewer              string   `json:"reviewer,omitempty"`
}

type InsightReviewLearningOptions struct {
	WorkItem             agents.WorkItem
	Agent                agents.Agent
	WorkspacePath        string
	RegistryRoot         string
	SkillPolicy          skills.Policy
	Reviewer             string
	ReviewerResponsePath string
	Policy               insights.Policy
	ApprovalDecision     string
	ApprovalReason       string
	ApprovalReviewer     string
	ConfirmApply         bool
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

	approvalPaths, err := writeLearningApprovals(proposalBundle, LearningApprovalOptions{
		Decision: opts.ApprovalDecision,
		Reason:   opts.ApprovalReason,
		Reviewer: resolveLearningApprovalReviewer(opts.Policy, opts.ApprovalReviewer),
		OutDir:   outDir,
		Now:      now,
	})
	if err != nil {
		return LearningLoopResult{}, err
	}
	result.ApprovalPaths = approvalPaths
	result.ApprovalCount = len(approvalPaths)

	applyResultPaths, applyPreviewPaths, err := writeLearningApplyResults(proposalBundle, approvalPaths, LearningApplyOptions{
		ConfirmApply: opts.ConfirmApply,
		OutDir:       outDir,
		Now:          now,
	})
	if err != nil {
		return LearningLoopResult{}, err
	}
	result.ApplyResultPaths = applyResultPaths
	result.ApplyPreviewPaths = applyPreviewPaths
	result.ApplyCount = len(applyResultPaths)

	effectivenessPaths, err := writeLearningEffectiveness(proposalBundle, applyResultPaths, LearningEffectivenessOptions{
		RunID:  runID,
		OutDir: outDir,
		Now:    now,
	})
	if err != nil {
		return LearningLoopResult{}, err
	}
	result.EffectivenessPaths = effectivenessPaths
	result.EffectivenessCount = len(effectivenessPaths)

	refresh, err := writeLearningSessionRefresh(LearningSessionRefreshOptions{
		Agent:            opts.Agent,
		WorkspacePath:    opts.WorkspacePath,
		RegistryRoot:     opts.RegistryRoot,
		SkillPolicy:      opts.SkillPolicy,
		RunID:            runID,
		ProposalBundle:   proposalBundle,
		ApplyResultPaths: applyResultPaths,
		OutDir:           outDir,
		Now:              now,
	})
	if err != nil {
		return LearningLoopResult{}, err
	}
	result.SessionRefreshPath = refresh.RefreshPath
	result.SessionSnapshotPath = refresh.SnapshotPath
	result.SessionSnapshotSHA256 = refresh.SnapshotSHA256
	return result, nil
}

type LearningApprovalOptions struct {
	Decision string
	Reason   string
	Reviewer string
	OutDir   string
	Now      time.Time
}

func writeLearningApprovals(bundle insights.LearningProposalBundle, opts LearningApprovalOptions) ([]string, error) {
	decision := strings.TrimSpace(opts.Decision)
	reason := strings.TrimSpace(opts.Reason)
	if decision == "" && reason == "" {
		return nil, nil
	}
	if decision == "" || reason == "" {
		return nil, fmt.Errorf("learning approval decision and reason are both required")
	}
	reviewer := strings.TrimSpace(opts.Reviewer)
	if reviewer == "" {
		reviewer = insights.ReviewerCodex
	}
	paths := make([]string, 0, len(bundle.Proposals))
	for _, proposal := range bundle.Proposals {
		approval, err := insights.BuildProposalApproval(proposal, bundle, insights.BuildProposalApprovalOptions{
			Reviewer: reviewer,
			Decision: decision,
			Reason:   reason,
			Now:      opts.Now,
		})
		if err != nil {
			return nil, err
		}
		path := filepath.Join(opts.OutDir, "learning-approval-"+proposal.ProposalID+".json")
		if err := insights.WriteApprovalJSON(approval, path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

type LearningApplyOptions struct {
	ConfirmApply bool
	OutDir       string
	Now          time.Time
}

func writeLearningApplyResults(bundle insights.LearningProposalBundle, approvalPaths []string, opts LearningApplyOptions) ([]string, []string, error) {
	if !opts.ConfirmApply {
		return nil, nil, nil
	}
	if len(approvalPaths) == 0 {
		return nil, nil, fmt.Errorf("learning apply requires approval artifacts")
	}
	approvals := make(map[string]insights.LearningProposalApproval, len(approvalPaths))
	for _, approvalPath := range approvalPaths {
		approval, err := insights.ReadApprovalJSON(approvalPath)
		if err != nil {
			return nil, nil, err
		}
		approvals[approval.ProposalID] = approval
	}

	resultPaths := make([]string, 0, len(bundle.Proposals))
	previewPaths := make([]string, 0, len(bundle.Proposals))
	for _, proposal := range bundle.Proposals {
		approval, ok := approvals[proposal.ProposalID]
		if !ok {
			return nil, nil, fmt.Errorf("learning apply approval for proposal %q is missing", proposal.ProposalID)
		}
		previewPath := filepath.Join(opts.OutDir, "learning-apply-preview-"+proposal.ProposalID+".md")
		resultPath := filepath.Join(opts.OutDir, "learning-apply-result-"+proposal.ProposalID+".json")
		applyResult, err := insights.ApplyExecute(insights.ApplyExecuteOptions{
			ApplyOptions:      insights.ApplyOptions{Proposal: proposal, Approval: approval, Now: opts.Now},
			PreviewOutputPath: previewPath,
			ConfirmApply:      true,
		})
		if err != nil {
			return nil, nil, err
		}
		if err := insights.WriteApplyExecuteJSON(applyResult, resultPath); err != nil {
			return nil, nil, err
		}
		if !applyResult.Executed {
			return nil, nil, fmt.Errorf("learning apply blocked for proposal %q: %s", proposal.ProposalID, applyResult.BlockedReason)
		}
		resultPaths = append(resultPaths, resultPath)
		previewPaths = append(previewPaths, previewPath)
	}
	return resultPaths, previewPaths, nil
}

type LearningEffectivenessOptions struct {
	RunID  string
	OutDir string
	Now    time.Time
}

func writeLearningEffectiveness(bundle insights.LearningProposalBundle, applyResultPaths []string, opts LearningEffectivenessOptions) ([]string, error) {
	if len(applyResultPaths) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(bundle.Proposals))
	for _, proposal := range bundle.Proposals {
		record, err := insights.RecordEffectiveness(insights.RecordEffectivenessOptions{
			Proposal: proposal,
			RunID:    opts.RunID,
			Metric:   insights.EffectivenessMetricValidationPassRate,
			Value:    1.0,
			Now:      opts.Now,
		})
		if err != nil {
			return nil, err
		}
		bundle, err := insights.AppendEffectivenessRecord(insights.EffectivenessBundle{}, record)
		if err != nil {
			return nil, err
		}
		path := filepath.Join(opts.OutDir, "learning-effectiveness-"+proposal.ProposalID+".json")
		if err := insights.WriteEffectivenessBundleJSON(bundle, path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

type LearningSessionRefreshOptions struct {
	Agent            agents.Agent
	WorkspacePath    string
	RegistryRoot     string
	SkillPolicy      skills.Policy
	RunID            string
	ProposalBundle   insights.LearningProposalBundle
	ApplyResultPaths []string
	OutDir           string
	Now              time.Time
}

type LearningSessionRefreshArtifact struct {
	Status              string   `json:"status"`
	RunID               string   `json:"run_id"`
	AgentID             string   `json:"agent_id"`
	SessionID           string   `json:"session_id"`
	WorkspacePath       string   `json:"workspace_path,omitempty"`
	SkillSnapshotPath   string   `json:"skill_snapshot_path"`
	SkillSnapshotSHA256 string   `json:"skill_snapshot_sha256"`
	ProposalIDs         []string `json:"proposal_ids"`
	ApplyResultPaths    []string `json:"apply_result_paths"`
	CreatedAt           string   `json:"created_at"`
}

type learningSessionRefreshResult struct {
	RefreshPath    string
	SnapshotPath   string
	SnapshotSHA256 string
}

func writeLearningSessionRefresh(opts LearningSessionRefreshOptions) (learningSessionRefreshResult, error) {
	if len(opts.ApplyResultPaths) == 0 {
		return learningSessionRefreshResult{}, nil
	}
	if strings.TrimSpace(opts.Agent.ID) == "" || strings.TrimSpace(opts.RegistryRoot) == "" {
		return learningSessionRefreshResult{}, nil
	}
	sessionID := "ses_learning_" + strings.TrimPrefix(strings.TrimSpace(opts.RunID), "run_")
	snapshot, err := skills.BuildSnapshot(skills.SnapshotOptions{
		AgentID:      opts.Agent.ID,
		SessionID:    sessionID,
		RegistryRoot: opts.RegistryRoot,
		Policy:       opts.SkillPolicy,
		Now:          opts.Now,
	})
	if err != nil {
		return learningSessionRefreshResult{}, err
	}
	snapshotPath := filepath.Join(opts.OutDir, "learning-session-snapshot-"+sessionID+".json")
	if err := skills.WriteSnapshotJSON(snapshot, snapshotPath); err != nil {
		return learningSessionRefreshResult{}, err
	}
	proposalIDs := make([]string, 0, len(opts.ProposalBundle.Proposals))
	for _, proposal := range opts.ProposalBundle.Proposals {
		proposalIDs = append(proposalIDs, proposal.ProposalID)
	}
	artifact := LearningSessionRefreshArtifact{
		Status:              "planned",
		RunID:               opts.RunID,
		AgentID:             opts.Agent.ID,
		SessionID:           sessionID,
		WorkspacePath:       strings.TrimSpace(opts.WorkspacePath),
		SkillSnapshotPath:   snapshotPath,
		SkillSnapshotSHA256: snapshot.SHA256,
		ProposalIDs:         proposalIDs,
		ApplyResultPaths:    append([]string(nil), opts.ApplyResultPaths...),
		CreatedAt:           opts.Now.Format(time.RFC3339Nano),
	}
	refreshPath := filepath.Join(opts.OutDir, "learning-session-refresh.json")
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return learningSessionRefreshResult{}, err
	}
	data = append(data, '\n')
	if err := os.WriteFile(refreshPath, data, 0o644); err != nil {
		return learningSessionRefreshResult{}, err
	}
	return learningSessionRefreshResult{RefreshPath: refreshPath, SnapshotPath: snapshotPath, SnapshotSHA256: snapshot.SHA256}, nil
}

func resolveInsightReviewer(policy insights.Policy, fallback string) string {
	return resolveReviewer(policy.Reviewer.Preferred, fallback, policy.Reviewer.Fallback)
}

func resolveLearningApprovalReviewer(policy insights.Policy, explicit string) string {
	return resolveReviewer(explicit, policy.Reviewer.Preferred, policy.Reviewer.Fallback)
}

func resolveReviewer(candidates ...string) string {
	for _, candidate := range candidates {
		switch strings.TrimSpace(candidate) {
		case insights.ReviewerCodex:
			return insights.ReviewerCodex
		case insights.ReviewerOpenCode:
			return insights.ReviewerOpenCode
		}
	}
	return insights.ReviewerCodex
}
