package worktemplates

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/wakeup"
)

type MaterializeOptions struct {
	Template Template
	Task     tasks.Task
	Wakeup   wakeup.Wakeup
	Now      time.Time
}

type MaterializeResult struct {
	WorkItem agents.WorkItem
	Inbox    agents.InboxItem
	Snapshot agents.WorkItemTaskSnapshot
}

func MaterializeWorkItem(opts MaterializeOptions) (MaterializeResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workID := workItemIDFromWakeup(opts.Wakeup.ID)
	item, err := agents.WorkItemFromTask(agents.WorkItemFromTaskOptions{
		Task:            opts.Task,
		AssignedAgentID: opts.Template.AssignedAgentID,
		CreatedBy:       "schedule:" + opts.Wakeup.ScheduleID,
		Priority:        opts.Template.Priority,
		Now:             now,
	})
	if err != nil {
		return MaterializeResult{}, err
	}
	item.ID = workID
	item.MaxAttempts = opts.Template.MaxAttempts
	item.BudgetPolicy = opts.Template.BudgetPolicy
	item.AssignmentMode = opts.Template.AssignmentMode
	item.TriggerType = "schedule"
	item.ScheduleID = opts.Wakeup.ScheduleID
	item.WakeupID = opts.Wakeup.ID

	snapshot, err := agents.BuildWorkItemTaskSnapshot(item.ID, opts.Task, item.CreatedAt)
	if err != nil {
		return MaterializeResult{}, err
	}
	item.TaskSnapshotSHA256 = snapshot.TaskSHA256

	if opts.Template.AssignmentMode == "auto_accept" {
		queued, err := agents.QueueWorkItemFromAutoAccept(item, now)
		if err != nil {
			return MaterializeResult{}, err
		}
		item = queued
		inbox := agents.InboxItem{
			ID: "inb_" + item.ID, AgentID: item.AssignedAgentID, WorkItemID: item.ID,
			Status: agents.InboxStatusAccepted, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		}
		return MaterializeResult{WorkItem: item, Inbox: inbox, Snapshot: snapshot}, nil
	}

	assign := agents.AssignTaskOptions{
		WorkItem: item, CreatedBy: "schedule:" + opts.Wakeup.ScheduleID, Now: now,
	}
	// assigned path without agent validation here; caller validates agent
	item.Status = agents.WorkItemStatusAssigned
	inbox := agents.InboxItem{
		ID: "inb_" + item.ID, AgentID: item.AssignedAgentID, WorkItemID: item.ID,
		Status: agents.InboxStatusPending, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	_ = assign
	return MaterializeResult{WorkItem: item, Inbox: inbox, Snapshot: snapshot}, nil
}

func workItemIDFromWakeup(wakeupID string) string {
	sum := sha256.Sum256([]byte("work|" + strings.TrimSpace(wakeupID)))
	return "work_" + hex.EncodeToString(sum[:8])
}

func ValidateWakeupWorkLink(w wakeup.Wakeup) error {
	if strings.TrimSpace(w.WorkItemID) != "" {
		return nil
	}
	return fmt.Errorf("wakeup %q has no work_item_id", w.ID)
}
