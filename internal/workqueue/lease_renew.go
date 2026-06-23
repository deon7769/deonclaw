package workqueue

import (
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

func RenewLease(lease Lease, ttl time.Duration, now time.Time) (Lease, error) {
	if lease.Status != LeaseStatusActive {
		return Lease{}, fmt.Errorf("lease %q status %q is not active", lease.ID, lease.Status)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if ttl <= 0 {
		if lease.TTLSeconds > 0 {
			ttl = time.Duration(lease.TTLSeconds) * time.Second
		} else {
			ttl = DefaultLeaseTTLSeconds * time.Second
		}
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, lease.ExpiresAt)
	if err != nil {
		return Lease{}, fmt.Errorf("parse lease expires_at: %w", err)
	}
	if now.After(expiresAt) {
		return Lease{}, fmt.Errorf("lease %q already expired", lease.ID)
	}
	lease.HeartbeatAt = now.Format(time.RFC3339Nano)
	lease.ExpiresAt = now.Add(ttl).Format(time.RFC3339Nano)
	lease.TTLSeconds = int(ttl.Seconds())
	lease.UpdatedAt = lease.HeartbeatAt
	return lease, nil
}

func MarkDeadLetter(item agents.WorkItem, now string) agents.WorkItem {
	item.Status = agents.WorkItemStatusDeadLetter
	item.UpdatedAt = now
	return item
}

func ShouldDeadLetterOnRecovery(item agents.WorkItem) bool {
	return item.MaxAttempts > 0 && item.Attempt >= item.MaxAttempts
}
