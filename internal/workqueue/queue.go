package workqueue

import (
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/agents"
)

var claimableStatuses = map[string]struct{}{
	agents.WorkItemStatusQueued:   {},
	agents.WorkItemStatusAssigned: {},
}

func CanClaim(item agents.WorkItem) error {
	if _, ok := claimableStatuses[item.Status]; !ok {
		return fmt.Errorf("work item %q status %q is not claimable", item.ID, item.Status)
	}
	if strings.TrimSpace(item.AssignedAgentID) == "" {
		return fmt.Errorf("work item %q has no assigned agent", item.ID)
	}
	return nil
}

func MarkLeased(item agents.WorkItem, now string) agents.WorkItem {
	item.Status = agents.WorkItemStatusLeased
	item.Attempt++
	item.UpdatedAt = now
	return item
}

func MarkReleased(item agents.WorkItem, now string, requeue bool) agents.WorkItem {
	if requeue && item.MaxAttempts > 0 && item.Attempt >= item.MaxAttempts {
		item.Status = agents.WorkItemStatusDeadLetter
	} else if requeue {
		item.Status = agents.WorkItemStatusQueued
	} else {
		item.Status = agents.WorkItemStatusCancelled
	}
	item.UpdatedAt = now
	return item
}

func SortClaimCandidates(items []agents.WorkItem) []agents.WorkItem {
	out := make([]agents.WorkItem, 0, len(items))
	for _, item := range items {
		if err := CanClaim(item); err != nil {
			continue
		}
		out = append(out, item)
	}
	// Higher priority first, then oldest created_at.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Priority > out[i].Priority {
				out[i], out[j] = out[j], out[i]
				continue
			}
			if out[j].Priority == out[i].Priority && out[j].CreatedAt < out[i].CreatedAt {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func SelectNextCandidate(items []agents.WorkItem, agentID string) (agents.WorkItem, bool) {
	agentID = strings.TrimSpace(agentID)
	candidates := SortClaimCandidates(items)
	for _, item := range candidates {
		if agentID != "" && item.AssignedAgentID != agentID {
			continue
		}
		return item, true
	}
	return agents.WorkItem{}, false
}
