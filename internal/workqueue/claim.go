package workqueue

import (
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

type ClaimContext struct {
	Agent            agents.Agent
	WorkItem         agents.WorkItem
	InboxStatus      string
	AutoAccept       bool
	ActiveLeaseCount int
	ItemsByID        map[string]agents.WorkItem
	HasTaskSnapshot  bool
	Now              time.Time
}

func ValidateClaim(ctx ClaimContext) error {
	if err := agents.CanStartRun(ctx.Agent); err != nil {
		return err
	}
	if ctx.WorkItem.Status != agents.WorkItemStatusQueued {
		return fmt.Errorf("work item %q status %q is not queued", ctx.WorkItem.ID, ctx.WorkItem.Status)
	}
	if ctx.WorkItem.MaxAttempts > 0 && ctx.WorkItem.Attempt >= ctx.WorkItem.MaxAttempts {
		return fmt.Errorf("work item %q attempt %d reached max_attempts %d", ctx.WorkItem.ID, ctx.WorkItem.Attempt, ctx.WorkItem.MaxAttempts)
	}
	maxRuns := ctx.Agent.MaxConcurrentRuns
	if maxRuns <= 0 {
		maxRuns = 1
	}
	if ctx.ActiveLeaseCount >= maxRuns {
		return fmt.Errorf("agent %q reached max_concurrent_runs %d", ctx.Agent.ID, maxRuns)
	}
	if err := agents.DependenciesSatisfied(ctx.WorkItem, ctx.ItemsByID); err != nil {
		return err
	}
	if !ctx.HasTaskSnapshot {
		return fmt.Errorf("work item %q has no task snapshot", ctx.WorkItem.ID)
	}
	inboxStatus := strings.TrimSpace(ctx.InboxStatus)
	if ctx.AutoAccept {
		if inboxStatus != "" && inboxStatus != agents.InboxStatusAccepted {
			return fmt.Errorf("work item %q inbox status %q blocks claim", ctx.WorkItem.ID, inboxStatus)
		}
		return nil
	}
	switch inboxStatus {
	case agents.InboxStatusAccepted:
		return nil
	case agents.InboxStatusDeferred:
		return fmt.Errorf("work item %q inbox is deferred", ctx.WorkItem.ID)
	case agents.InboxStatusPending:
		return fmt.Errorf("work item %q inbox is pending acceptance", ctx.WorkItem.ID)
	default:
		if inboxStatus == "" {
			return fmt.Errorf("work item %q has no accepted inbox item", ctx.WorkItem.ID)
		}
		return fmt.Errorf("work item %q inbox status %q blocks claim", ctx.WorkItem.ID, inboxStatus)
	}
}

func ItemsByID(items []agents.WorkItem) map[string]agents.WorkItem {
	out := make(map[string]agents.WorkItem, len(items))
	for _, item := range items {
		out[item.ID] = item
	}
	return out
}

func CountActiveLeasesForAgent(leases []Lease, agentID string) int {
	count := 0
	for _, lease := range leases {
		if lease.Status == LeaseStatusActive && lease.AgentID == agentID {
			count++
		}
	}
	return count
}

func InboxStatusForWork(inboxItems []agents.InboxItem, workItemID string) string {
	for _, item := range inboxItems {
		if item.WorkItemID == workItemID {
			return item.Status
		}
	}
	return ""
}
