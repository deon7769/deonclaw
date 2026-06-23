package dispatch

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/usage"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

func testDispatchOpts(db *testStoreAdapter, t *testing.T, item agents.WorkItem, leaseID string, runner WorkerRunner) OnceOptions {
	t.Helper()
	artifactsDir := t.TempDir()
	return OnceOptions{
		WorkItemID: item.ID, LeaseID: leaseID, Mode: ModeFake,
		ArtifactsDir: artifactsDir,
		EvidenceBuilder: func(ctx context.Context, repo Repository, runID string, now time.Time) (insights.EvidenceBundle, error) {
			return insights.EvidenceBundle{
				EvidenceBundleID: "evb_test_" + runID,
				SHA256:           "sha256-test",
				RunIDs:           []string{runID},
			}, nil
		},
		CodexRunner: runner, OpenCodeRunner: runner, PricingLoader: DefaultPricingLoader,
		Now: time.Now().UTC(),
	}
}

func seedQueuedWork(t *testing.T, db *testStoreAdapter, agent agents.Agent, policyID string, task tasks.Task) agents.WorkItem {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	item, err := agents.WorkItemFromTask(agents.WorkItemFromTaskOptions{
		Task: task, AssignedAgentID: agent.ID, CreatedBy: "test", Now: now,
	})
	if err != nil {
		t.Fatalf("WorkItemFromTask() error = %v", err)
	}
	item.Status = agents.WorkItemStatusQueued
	item.BudgetPolicy = policyID
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
	_ = db.SaveTask(ctx, &task)
	return item
}

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
	item := seedQueuedWork(t, db, agent, policy.ID, task)

	svc := Service{Repo: db}
	result, err := svc.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("DispatchOnce() status = %q, want ok (%+v)", result.Status, result)
	}
	if !result.LeaseAcquired || !result.BudgetCommitted {
		t.Fatalf("expected lease acquired and budget committed, got %+v", result)
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
		ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
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
	item := seedQueuedWork(t, db, agent, policy.ID, task)

	svc := Service{Repo: db}
	result, err := svc.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if err != nil {
		t.Fatalf("DispatchOnce() error = %v", err)
	}
	if result.Status != "budget_blocked" || result.BudgetBlocked == nil {
		t.Fatalf("DispatchOnce() = %+v, want budget_blocked", result)
	}
	if !result.LeaseReleased {
		t.Fatalf("expected lease released on budget block, got %+v", result)
	}
	_ = workqueue.EventBudgetBlocked
}

func TestDispatchOnceAutoClaimAlreadyLeased(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "leased.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "agent-a", DisplayName: "A", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "p1", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly, Timezone: "UTC",
		HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000, MaxSingleRunMicroUSD: 3_000_000,
		ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-leased", Title: "Leased", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	claim, _ := db.ClaimWorkItem(ctx, agent.ID, item.ID, 15*time.Minute, now)
	other, _ := db.ClaimWorkItem(ctx, agent.ID, item.ID, 15*time.Minute, now)
	if other.Claimed {
		t.Fatal("expected second claim to fail")
	}
	_ = claim
	result, _ := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if result.Status != "not_started" || result.WorkerStarted {
		t.Fatalf("got %+v, want not_started without worker", result)
	}
}

func TestDispatchOnceRealModeBlocked(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "real.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	opts := testDispatchOpts(db, t, agents.WorkItem{ID: "work_x"}, "", NewFakeWorkerRunner())
	opts.Mode = ModeReal
	opts.ConfirmWorkerDispatch = true
	result, err := Service{Repo: db}.DispatchOnce(ctx, opts)
	if err == nil {
		t.Fatal("expected real mode error")
	}
	if result.Status != "blocked" || result.WorkerStarted {
		t.Fatalf("got %+v", result)
	}
}

func TestDispatchOnceNoBudgetPolicyBlocks(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "nobudget.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "nobudget", DisplayName: "N", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	task := tasks.Task{
		ID: "task-nobudget", Title: "No budget", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, "", task)
	called := false
	runner := workerFunc(func(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
		called = true
		return FakeWorkerRun(ctx, opts)
	})
	result, err := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", runner))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "blocked" || result.BlockedReason != BlockedBudgetPolicyRequired || called {
		t.Fatalf("got %+v called=%v", result, called)
	}
}

func TestDispatchRunnerErrorReleasesLease(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "fail.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "fail-agent", DisplayName: "F", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "p-fail", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly, Timezone: "UTC",
		HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000, MaxSingleRunMicroUSD: 3_000_000,
		ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-fake-failure", Title: "Fail", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	result, err := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if err == nil {
		t.Fatal("expected failure")
	}
	if !result.LeaseReleased || !result.BudgetReleased || result.Status != "failed" {
		t.Fatalf("got %+v", result)
	}
	leases, _ := db.ListLeases(ctx)
	for _, lease := range leases {
		if lease.WorkItemID == item.ID && lease.Status == workqueue.LeaseStatusActive {
			t.Fatalf("lease still active: %+v", lease)
		}
	}
}
