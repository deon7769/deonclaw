package dispatch

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/usage"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

func TestDispatchOnceFakeSuccess(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "dispatch.db")
	db, err := openTestStoreAdapter(dbPath)
	if err != nil {
		t.Fatalf("OpenTestStore() error = %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
		ModelProfile: "opencode-zai-glm-5-1",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err := db.SaveAgent(ctx, agent); err != nil {
		t.Fatalf("SaveAgent() error = %v", err)
	}

	policy := budget.Policy{
		ID: "backend-monthly", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
		WarningThresholdBasisPoints: []int{5000, 8000},
	}
	if err := db.SyncBudgetPolicies(ctx, []budget.Policy{policy}); err != nil {
		t.Fatalf("SyncBudgetPolicies() error = %v", err)
	}
	if err := db.SyncModelPrices(ctx, []usage.ModelPrice{{
		ID: "opencode-zai-glm-5-1", Provider: "z-ai", Model: "glm-5.1", Currency: "USD",
		InputMicroUSDPerMillion: 200_000, OutputMicroUSDPerMillion: 800_000, EffectiveAt: "2026-01-01",
	}}); err != nil {
		t.Fatalf("SyncModelPrices() error = %v", err)
	}

	task := tasks.Task{
		ID: "task-fake-success", Title: "Fake success", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace:        tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."},
		Memory:           tasks.MemorySpec{Scope: "none"},
		AllowedPaths:     []string{"/"},
		ForbiddenPaths:   []string{"/secrets"},
		ExpectedOutputs:  []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item, err := agents.WorkItemFromTask(agents.WorkItemFromTaskOptions{
		Task: task, AssignedAgentID: agent.ID, CreatedBy: "test", Now: now,
	})
	if err != nil {
		t.Fatalf("WorkItemFromTask() error = %v", err)
	}
	item.Status = agents.WorkItemStatusQueued
	item.BudgetPolicy = policy.ID
	item.TaskID = task.ID
	if err := db.SaveWorkItem(ctx, item); err != nil {
		t.Fatalf("SaveWorkItem() error = %v", err)
	}
	inbox := agents.InboxItem{
		ID: "inb_" + item.ID, AgentID: agent.ID, WorkItemID: item.ID,
		Status: agents.InboxStatusAccepted, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
	if err := db.SaveInboxItem(ctx, inbox); err != nil {
		t.Fatalf("SaveInboxItem() error = %v", err)
	}
	snapshot, err := agents.BuildWorkItemTaskSnapshot(item.ID, task, item.CreatedAt)
	if err != nil {
		t.Fatalf("BuildWorkItemTaskSnapshot() error = %v", err)
	}
	if err := db.SaveWorkItemTaskSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("SaveWorkItemTaskSnapshot() error = %v", err)
	}
	if err := db.SaveTask(ctx, &task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	claim, err := db.ClaimWorkItem(ctx, agent.ID, item.ID, 15*time.Minute, now)
	if err != nil || !claim.Claimed {
		t.Fatalf("ClaimWorkItem() = %+v err=%v", claim, err)
	}

	svc := Service{Repo: db}
	result, err := svc.DispatchOnce(ctx, OnceOptions{
		WorkItemID: item.ID, LeaseID: claim.Lease.ID, Mode: ModeFake,
		CodexRunner: NewFakeWorkerRunner(), OpenCodeRunner: NewFakeWorkerRunner(),
		PricingLoader: DefaultPricingLoader, Now: now,
	})
	if err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("DispatchOnce() status = %q, want ok (%+v)", result.Status, result)
	}
	if result.InsightReview == nil || !result.InsightReview.Queued {
		t.Fatalf("expected insight review work item, got %+v", result.InsightReview)
	}
}

func TestDispatchOnceBudgetBlocked(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "blocked.db")
	db, err := openTestStoreAdapter(dbPath)
	if err != nil {
		t.Fatalf("OpenTestStore() error = %v", err)
	}
	defer db.Close()

	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "blocked-agent", DisplayName: "Blocked", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "tiny", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly, Timezone: "UTC",
		HardLimitMicroUSD: 100_000, DefaultEstimateMicroUSD: 250_000, MaxSingleRunMicroUSD: 250_000,
		ReserveBeforeRun: false, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	_, _ = db.GetOrCreateWindow(ctx, policy, now)
	_, _ = db.ReserveBudget(ctx, budget.ReserveBudgetOptions{
		PolicyID: policy.ID, WorkItemID: "work_other", EstimatedMicroUSD: 90_000, Now: now,
	})

	task := tasks.Task{
		ID: "task-fake-success", Title: "Blocked", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace:        tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."},
		Memory:           tasks.MemorySpec{Scope: "none"},
		AllowedPaths:     []string{"/"},
		ForbiddenPaths:   []string{"/secrets"},
		ExpectedOutputs:  []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item, _ := agents.WorkItemFromTask(agents.WorkItemFromTaskOptions{Task: task, AssignedAgentID: agent.ID, CreatedBy: "test", Now: now})
	item.Status = agents.WorkItemStatusQueued
	item.BudgetPolicy = policy.ID
	_ = db.SaveWorkItem(ctx, item)
	snapshot, _ := agents.BuildWorkItemTaskSnapshot(item.ID, task, item.CreatedAt)
	_ = db.SaveWorkItemTaskSnapshot(ctx, snapshot)

	svc := Service{Repo: db}
	result, err := svc.DispatchOnce(ctx, OnceOptions{
		WorkItemID: item.ID, Mode: ModeFake, CodexRunner: NewFakeWorkerRunner(), Now: now,
	})
	if err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if result.Status != "budget_blocked" || result.BudgetBlocked == nil {
		t.Fatalf("DispatchOnce() = %+v, want budget_blocked", result)
	}
	_ = workqueue.EventBudgetBlocked
}
