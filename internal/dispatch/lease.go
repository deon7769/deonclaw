package dispatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

func AcquireLease(ctx context.Context, repo Repository, agent agents.Agent, workItem agents.WorkItem, leaseID string, ttl time.Duration, now time.Time) (workqueue.Lease, bool, error) {
	if strings.TrimSpace(leaseID) != "" {
		lease, err := repo.Lease(ctx, leaseID)
		if err != nil {
			return workqueue.Lease{}, false, err
		}
		if lease.WorkItemID != workItem.ID {
			return workqueue.Lease{}, false, fmt.Errorf("lease %q belongs to work item %q, not %q", lease.ID, lease.WorkItemID, workItem.ID)
		}
		if lease.Status != workqueue.LeaseStatusActive {
			return workqueue.Lease{}, false, fmt.Errorf("lease %q status %q is not active", lease.ID, lease.Status)
		}
		if lease.AgentID != agent.ID {
			return workqueue.Lease{}, false, fmt.Errorf("lease %q agent %q does not match %q", lease.ID, lease.AgentID, agent.ID)
		}
		return lease, true, nil
	}
	if workItem.Status != agents.WorkItemStatusQueued {
		return workqueue.Lease{}, false, nil
	}
	if ttl <= 0 {
		ttl = workqueue.DefaultLeaseTTLSeconds * time.Second
	}
	result, err := repo.ClaimWorkItem(ctx, agent.ID, workItem.ID, ttl, now)
	if err != nil {
		return workqueue.Lease{}, false, err
	}
	if !result.Claimed {
		return workqueue.Lease{}, false, nil
	}
	return result.Lease, true, nil
}

func ReleaseActiveLease(ctx context.Context, repo Repository, lease workqueue.Lease, reason string, requeue bool, now time.Time) error {
	if strings.TrimSpace(lease.ID) == "" {
		return nil
	}
	if lease.Status != workqueue.LeaseStatusActive {
		return nil
	}
	return repo.ReleaseLease(ctx, lease.ID, reason, requeue, now)
}
