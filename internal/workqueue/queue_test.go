package workqueue

import (
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

func TestRecoverStaleLeasesMarksLost(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	leases := []Lease{{
		ID: "lease_1", WorkItemID: "work_1", AgentID: "agent", Status: LeaseStatusActive,
		ExpiresAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
	}}
	updated, report := RecoverStaleLeases(leases, now)
	if report.Status != "warning" {
		t.Fatalf("report status = %q", report.Status)
	}
	if updated[0].Status != LeaseStatusLost {
		t.Fatalf("lease status = %q", updated[0].Status)
	}
}

func TestSelectNextCandidateByPriority(t *testing.T) {
	items := []agents.WorkItem{
		{ID: "low", Status: agents.WorkItemStatusQueued, AssignedAgentID: "a", Priority: 10, CreatedAt: "2026-06-22T08:00:00Z"},
		{ID: "high", Status: agents.WorkItemStatusQueued, AssignedAgentID: "a", Priority: 90, CreatedAt: "2026-06-22T09:00:00Z"},
	}
	next, ok := SelectNextCandidate(items, "a")
	if !ok || next.ID != "high" {
		t.Fatalf("candidate = %+v ok=%t", next, ok)
	}
}

func TestCanClaimRejectsLeased(t *testing.T) {
	err := CanClaim(agents.WorkItem{ID: "w1", Status: agents.WorkItemStatusLeased, AssignedAgentID: "a"})
	if err == nil {
		t.Fatal("expected error for leased work item")
	}
}
