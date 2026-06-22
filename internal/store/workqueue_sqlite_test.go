package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

func TestClaimWorkItemIsAtomic(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "workqueue.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err := db.SaveAgent(ctx, agent); err != nil {
		t.Fatalf("SaveAgent() error = %v", err)
	}
	item := agents.WorkItem{
		ID: "work_test", Title: "Test", Status: agents.WorkItemStatusQueued,
		AssignedAgentID: agent.ID, Priority: 50, MaxAttempts: 2,
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}
	if err := db.SaveWorkItem(ctx, item); err != nil {
		t.Fatalf("SaveWorkItem() error = %v", err)
	}

	var wg sync.WaitGroup
	claims := make(chan bool, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := db.ClaimWorkItem(ctx, agent.ID, "", 15*time.Minute, now)
			if err != nil {
				t.Errorf("ClaimWorkItem() error = %v", err)
				return
			}
			claims <- result.Claimed
		}()
	}
	wg.Wait()
	close(claims)
	success := 0
	for ok := range claims {
		if ok {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful claims = %d, want 1", success)
	}
}

func TestReleaseLeaseRequeuesWork(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "release.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	item := agents.WorkItem{
		ID: "work_release", Title: "Release", Status: agents.WorkItemStatusQueued,
		AssignedAgentID: agent.ID, Priority: 50, MaxAttempts: 3,
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
	}
	_ = db.SaveWorkItem(ctx, item)

	claim, err := db.ClaimWorkItem(ctx, agent.ID, item.ID, 15*time.Minute, now)
	if err != nil || !claim.Claimed {
		t.Fatalf("ClaimWorkItem() = %+v err=%v", claim, err)
	}
	if err := db.ReleaseLease(ctx, claim.Lease.ID, "test release", true, now); err != nil {
		t.Fatalf("ReleaseLease() error = %v", err)
	}
	updated, err := db.WorkItem(ctx, item.ID)
	if err != nil {
		t.Fatalf("WorkItem() error = %v", err)
	}
	if updated.Status != agents.WorkItemStatusQueued {
		t.Fatalf("status = %q, want queued", updated.Status)
	}
}

func TestRecoverWorkQueueMarksLostLease(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "recover.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	claimTime := now.Add(-2 * time.Hour)
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, claimTime.Format(time.RFC3339Nano), claimTime.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	item := agents.WorkItem{
		ID: "work_recover", Title: "Recover", Status: agents.WorkItemStatusLeased,
		AssignedAgentID: agent.ID, Attempt: 1, MaxAttempts: 3,
		CreatedAt: claimTime.Format(time.RFC3339Nano), UpdatedAt: claimTime.Format(time.RFC3339Nano),
	}
	_ = db.SaveWorkItem(ctx, item)
	lease, _ := workqueue.BuildLease(workqueue.LeaseOptions{
		WorkItemID: item.ID, AgentID: agent.ID, TTL: 15 * time.Minute, Now: claimTime,
	})
	_ = db.SaveLease(ctx, lease)

	report, err := db.RecoverWorkQueue(ctx, now)
	if err != nil {
		t.Fatalf("RecoverWorkQueue() error = %v", err)
	}
	if report.Status != "warning" {
		t.Fatalf("report status = %q", report.Status)
	}
	stored, _ := db.Lease(ctx, lease.ID)
	if stored.Status != workqueue.LeaseStatusLost {
		t.Fatalf("lease status = %q", stored.Status)
	}
}
