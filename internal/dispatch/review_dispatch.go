package dispatch

import (
	"context"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

type InsightReviewResult struct {
	WorkItemID string `json:"work_item_id,omitempty"`
	Queued     bool   `json:"queued"`
	Skipped    bool   `json:"skipped"`
	Reason     string `json:"reason,omitempty"`
}

func EnqueueInsightReview(ctx context.Context, repo Repository, parent agents.WorkItem, parentRunID string, now time.Time) (InsightReviewResult, error) {
	if parent.Kind == agents.WorkItemKindInsightReview {
		return InsightReviewResult{Skipped: true, Reason: "recursive insight review blocked"}, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	reviewID := "work_insight_" + parent.ID
	item := agents.WorkItem{
		ID:               reviewID,
		Title:            "Insight review for " + parent.Title,
		Status:           agents.WorkItemStatusQueued,
		Kind:             agents.WorkItemKindInsightReview,
		AssignedAgentID:  parent.AssignedAgentID,
		ParentWorkItemID: parent.ID,
		TaskID:           parent.TaskID,
		Priority:         parent.Priority,
		MaxAttempts:      1,
		BudgetPolicy:     parent.BudgetPolicy,
		AssignmentMode:   "auto_accept",
		TriggerType:      "dispatch",
		CreatedBy:        "dispatch:" + parentRunID,
		CreatedAt:        stamp,
		UpdatedAt:        stamp,
	}
	inbox := agents.InboxItem{
		ID: "inb_" + reviewID, AgentID: item.AssignedAgentID, WorkItemID: reviewID,
		Status: agents.InboxStatusAccepted, CreatedAt: stamp, UpdatedAt: stamp,
	}
	if err := repo.SaveWorkItem(ctx, item); err != nil {
		return InsightReviewResult{}, err
	}
	if err := repo.SaveInboxItem(ctx, inbox); err != nil {
		return InsightReviewResult{}, err
	}
	if err := repo.AppendWorkQueueEvent(ctx, workqueue.QueueEvent{
		ID: "wqe_review_" + reviewID, WorkItemID: reviewID, EventType: workqueue.EventReviewQueued,
		Payload: fmt.Sprintf(`{"parent_work_item_id":%q,"parent_run_id":%q}`, parent.ID, parentRunID), CreatedAt: stamp,
	}); err != nil {
		return InsightReviewResult{}, err
	}
	return InsightReviewResult{WorkItemID: reviewID, Queued: true}, nil
}
