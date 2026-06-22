package workqueue

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type LeaseOptions struct {
	WorkItemID string
	AgentID    string
	RunID      string
	TTL        time.Duration
	Now        time.Time
}

func NewLeaseID(workItemID string, agentID string, now time.Time) string {
	sum := sha256.Sum256([]byte(workItemID + "|" + agentID + "|" + now.Format(time.RFC3339Nano)))
	return "lease_" + hex.EncodeToString(sum[:8])
}

func BuildLease(opts LeaseOptions) (Lease, error) {
	agentID := strings.TrimSpace(opts.AgentID)
	workItemID := strings.TrimSpace(opts.WorkItemID)
	if agentID == "" {
		return Lease{}, fmt.Errorf("agent id is required")
	}
	if workItemID == "" {
		return Lease{}, fmt.Errorf("work item id is required")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ttl := opts.TTL
	if ttl <= 0 {
		ttl = DefaultLeaseTTLSeconds * time.Second
	}
	expires := now.Add(ttl)
	return Lease{
		ID:          NewLeaseID(workItemID, agentID, now),
		WorkItemID:  workItemID,
		AgentID:     agentID,
		RunID:       strings.TrimSpace(opts.RunID),
		Status:      LeaseStatusActive,
		TTLSeconds:  int(ttl.Seconds()),
		ExpiresAt:   expires.Format(time.RFC3339Nano),
		HeartbeatAt: now.Format(time.RFC3339Nano),
		CreatedAt:   now.Format(time.RFC3339Nano),
		UpdatedAt:   now.Format(time.RFC3339Nano),
	}, nil
}

func ReleaseLease(lease Lease, reason string, now time.Time) Lease {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	lease.Status = LeaseStatusReleased
	lease.ReleaseReason = strings.TrimSpace(reason)
	lease.UpdatedAt = now.Format(time.RFC3339Nano)
	return lease
}

func RecoverStaleLeases(leases []Lease, now time.Time) ([]Lease, RecoveryReport) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	findings := make([]RecoveryFinding, 0)
	updated := make([]Lease, 0, len(leases))
	for _, lease := range leases {
		if lease.Status != LeaseStatusActive {
			updated = append(updated, lease)
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339Nano, lease.ExpiresAt)
		if err != nil {
			updated = append(updated, lease)
			continue
		}
		if !now.After(expiresAt) {
			updated = append(updated, lease)
			continue
		}
		lease.Status = LeaseStatusLost
		lease.ReleaseReason = "lease ttl exceeded"
		lease.UpdatedAt = now.Format(time.RFC3339Nano)
		updated = append(updated, lease)
		findings = append(findings, RecoveryFinding{
			Severity: "warning",
			Code:     "stale_lease",
			Message:  "active lease exceeded ttl and was marked lost",
			LeaseID:  lease.ID,
		})
	}
	status := "ok"
	if len(findings) > 0 {
		status = "warning"
	}
	return updated, RecoveryReport{Status: status, Findings: findings}
}

func HasActiveLeaseForWork(leases []Lease, workItemID string) bool {
	for _, lease := range leases {
		if lease.WorkItemID != workItemID {
			continue
		}
		if lease.Status == LeaseStatusActive {
			return true
		}
	}
	return false
}
