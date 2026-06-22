package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type InboxItem struct {
	ID         string `json:"id"`
	AgentID    string `json:"agent_id"`
	WorkItemID string `json:"work_item_id"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type AssignTaskResult struct {
	WorkItem  WorkItem  `json:"work_item"`
	InboxItem InboxItem `json:"inbox_item"`
}

type AssignTaskOptions struct {
	Agent     Agent
	WorkItem  WorkItem
	CreatedBy string
	Now       time.Time
}

func AssignWorkItem(opts AssignTaskOptions) (AssignTaskResult, error) {
	if err := CanReceiveWork(opts.Agent); err != nil {
		return AssignTaskResult{}, err
	}
	if opts.WorkItem.AssignedAgentID != opts.Agent.ID {
		return AssignTaskResult{}, fmt.Errorf("work item assigned_agent_id %q does not match agent %q", opts.WorkItem.AssignedAgentID, opts.Agent.ID)
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	inboxID := newInboxID(opts.Agent.ID, opts.WorkItem.ID, now)
	item := InboxItem{
		ID:         inboxID,
		AgentID:    opts.Agent.ID,
		WorkItemID: opts.WorkItem.ID,
		Status:     InboxStatusPending,
		CreatedAt:  now.Format(time.RFC3339Nano),
		UpdatedAt:  now.Format(time.RFC3339Nano),
	}
	workItem := opts.WorkItem
	workItem.Status = WorkItemStatusAssigned
	workItem.UpdatedAt = item.UpdatedAt
	return AssignTaskResult{WorkItem: workItem, InboxItem: item}, nil
}

func AcceptInboxItem(item InboxItem, now time.Time) (InboxItem, error) {
	if item.Status != InboxStatusPending && item.Status != InboxStatusDeferred {
		return InboxItem{}, fmt.Errorf("inbox item status %q cannot be accepted", item.Status)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	item.Status = InboxStatusAccepted
	item.UpdatedAt = now.Format(time.RFC3339Nano)
	return item, nil
}

func DeferInboxItem(item InboxItem, now time.Time) (InboxItem, error) {
	if item.Status != InboxStatusPending {
		return InboxItem{}, fmt.Errorf("inbox item status %q cannot be deferred", item.Status)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	item.Status = InboxStatusDeferred
	item.UpdatedAt = now.Format(time.RFC3339Nano)
	return item, nil
}

func newInboxID(agentID string, workItemID string, now time.Time) string {
	sum := sha256.Sum256([]byte(agentID + "|" + workItemID + "|" + now.Format(time.RFC3339Nano)))
	return "inb_" + hex.EncodeToString(sum[:8])
}
