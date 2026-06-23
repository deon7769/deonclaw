package dispatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

type InsightReviewResult struct {
	WorkItemID string `json:"work_item_id,omitempty"`
	Queued     bool   `json:"queued"`
	Skipped    bool   `json:"skipped"`
	Reason     string `json:"reason,omitempty"`
}

func EnqueueInsightReview(ctx context.Context, repo Repository, parent agents.WorkItem, parentRunID string, evidence insights.EvidenceBundle, evidencePath string, reviewer agents.Agent, now time.Time) (InsightReviewResult, error) {
	if parent.Kind == agents.WorkItemKindInsightReview {
		return InsightReviewResult{Skipped: true, Reason: "recursive insight review blocked"}, nil
	}
	if strings.TrimSpace(evidence.EvidenceBundleID) == "" || strings.TrimSpace(evidence.SHA256) == "" {
		return InsightReviewResult{Skipped: true, Reason: "evidence bundle missing"}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	reviewID := "work_insight_" + parent.ID
	reviewTask, err := BuildInsightReviewTask(parentRunID, evidence, reviewer)
	if err != nil {
		return InsightReviewResult{}, err
	}
	item := agents.WorkItem{
		ID:                   reviewID,
		Title:                "Insight review for " + parent.Title,
		Status:               agents.WorkItemStatusQueued,
		Kind:                 agents.WorkItemKindInsightReview,
		AssignedAgentID:      reviewer.ID,
		ParentWorkItemID:     parent.ID,
		ParentRunID:          parentRunID,
		EvidenceBundleID:     evidence.EvidenceBundleID,
		EvidenceBundleSHA256: evidence.SHA256,
		EvidencePath:         evidencePath,
		ReviewerAgentID:      reviewer.ID,
		TaskID:               reviewTask.ID,
		Priority:             parent.Priority,
		MaxAttempts:          1,
		BudgetPolicy:         parent.BudgetPolicy,
		AssignmentMode:       "auto_accept",
		TriggerType:          "dispatch",
		CreatedBy:            "dispatch:" + parentRunID,
		CreatedAt:            stamp,
		UpdatedAt:            stamp,
	}
	inbox := agents.InboxItem{
		ID: "inb_" + reviewID, AgentID: item.AssignedAgentID, WorkItemID: reviewID,
		Status: agents.InboxStatusAccepted, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := repo.SaveWorkItem(ctx, item); err != nil {
		return InsightReviewResult{}, err
	}
	snapshot, err := agents.BuildWorkItemTaskSnapshot(item.ID, reviewTask, stamp)
	if err != nil {
		return InsightReviewResult{}, err
	}
	if err := repo.SaveWorkItemTaskSnapshot(ctx, snapshot); err != nil {
		return InsightReviewResult{}, err
	}
	if err := repo.SaveInboxItem(ctx, inbox); err != nil {
		return InsightReviewResult{}, err
	}
	if err := repo.AppendWorkQueueEvent(ctx, workqueue.QueueEvent{
		ID: "wqe_review_" + reviewID, WorkItemID: reviewID, EventType: workqueue.EventReviewQueued,
		Payload: fmt.Sprintf(`{"parent_work_item_id":%q,"parent_run_id":%q,"evidence_bundle_id":%q}`,
			parent.ID, parentRunID, evidence.EvidenceBundleID),
		CreatedAt: stamp,
	}); err != nil {
		return InsightReviewResult{}, err
	}
	return InsightReviewResult{WorkItemID: reviewID, Queued: true}, nil
}
