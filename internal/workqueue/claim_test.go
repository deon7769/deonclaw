package workqueue

import (
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

func TestValidateClaimRejectsAssigned(t *testing.T) {
	agent := agents.Agent{ID: "a", Status: agents.StatusActive, MaxConcurrentRuns: 2}
	err := ValidateClaim(ClaimContext{
		Agent:           agent,
		WorkItem:        agents.WorkItem{ID: "w1", Status: agents.WorkItemStatusAssigned, AssignedAgentID: "a"},
		InboxStatus:     agents.InboxStatusPending,
		HasTaskSnapshot: true,
	})
	if err == nil {
		t.Fatal("expected error for assigned work item")
	}
}

func TestValidateClaimRequiresAcceptedInbox(t *testing.T) {
	agent := agents.Agent{ID: "a", Status: agents.StatusActive, MaxConcurrentRuns: 2}
	err := ValidateClaim(ClaimContext{
		Agent:           agent,
		WorkItem:        agents.WorkItem{ID: "w1", Status: agents.WorkItemStatusQueued, AssignedAgentID: "a"},
		InboxStatus:     agents.InboxStatusPending,
		HasTaskSnapshot: true,
	})
	if err == nil {
		t.Fatal("expected error for pending inbox")
	}
}

func TestValidateClaimRejectsDeferredInbox(t *testing.T) {
	agent := agents.Agent{ID: "a", Status: agents.StatusActive, MaxConcurrentRuns: 2}
	err := ValidateClaim(ClaimContext{
		Agent:           agent,
		WorkItem:        agents.WorkItem{ID: "w1", Status: agents.WorkItemStatusQueued, AssignedAgentID: "a"},
		InboxStatus:     agents.InboxStatusDeferred,
		HasTaskSnapshot: true,
	})
	if err == nil {
		t.Fatal("expected error for deferred inbox")
	}
}

func TestValidateClaimRejectsPausedAgent(t *testing.T) {
	agent := agents.Agent{ID: "a", Status: agents.StatusPaused, MaxConcurrentRuns: 2}
	err := ValidateClaim(ClaimContext{
		Agent:           agent,
		WorkItem:        agents.WorkItem{ID: "w1", Status: agents.WorkItemStatusQueued, AssignedAgentID: "a"},
		InboxStatus:     agents.InboxStatusAccepted,
		HasTaskSnapshot: true,
	})
	if err == nil {
		t.Fatal("expected error for paused agent")
	}
}

func TestValidateClaimRespectsMaxConcurrentRuns(t *testing.T) {
	agent := agents.Agent{ID: "a", Status: agents.StatusActive, MaxConcurrentRuns: 1}
	err := ValidateClaim(ClaimContext{
		Agent:            agent,
		WorkItem:         agents.WorkItem{ID: "w1", Status: agents.WorkItemStatusQueued, AssignedAgentID: "a"},
		InboxStatus:      agents.InboxStatusAccepted,
		ActiveLeaseCount: 1,
		HasTaskSnapshot:  true,
	})
	if err == nil {
		t.Fatal("expected max concurrent runs error")
	}
}

func TestRenewLeaseUpdatesExpiry(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	lease := Lease{
		ID: "lease_1", Status: LeaseStatusActive,
		ExpiresAt:  now.Add(5 * time.Minute).Format(time.RFC3339Nano),
		TTLSeconds: 300,
	}
	renewed, err := RenewLease(lease, 15*time.Minute, now)
	if err != nil {
		t.Fatalf("RenewLease() error = %v", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, renewed.ExpiresAt)
	if err != nil {
		t.Fatalf("parse expires_at: %v", err)
	}
	if !expires.After(now.Add(14 * time.Minute)) {
		t.Fatalf("expires_at = %v, want ~15m from now", expires)
	}
	if renewed.TTLSeconds != 900 {
		t.Fatalf("TTLSeconds = %d, want 900", renewed.TTLSeconds)
	}
}
