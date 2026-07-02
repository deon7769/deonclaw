package dispatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/skills"
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
				Trigger:          insights.TriggerRunCompleted,
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

func TestDispatchOnceRealModeRequiresConfirmation(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "real.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	opts := testDispatchOpts(db, t, agents.WorkItem{ID: "work_x"}, "", NewFakeWorkerRunner())
	opts.Mode = ModeReal
	opts.ConfirmWorkerDispatch = false
	result, err := Service{Repo: db}.DispatchOnce(ctx, opts)
	if err != nil {
		t.Fatalf("real mode without confirmation should return blocked result without error: %v", err)
	}
	if result.Status != "blocked" || result.BlockedReason != BlockedRealModeConfirmationRequired || result.WorkerStarted {
		t.Fatalf("got %+v", result)
	}
}

func TestDispatchOnceRealModeBlockedInCI(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "real-ci.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	called := false
	runner := workerFunc(func(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
		called = true
		return FakeWorkerRun(ctx, opts)
	})
	opts := testDispatchOpts(db, t, agents.WorkItem{ID: "work_ci"}, "", runner)
	opts.Mode = ModeReal
	opts.ConfirmWorkerDispatch = true
	opts.RunningInCI = true
	result, err := Service{Repo: db}.DispatchOnce(ctx, opts)
	if err != nil {
		t.Fatalf("real mode in CI should return blocked result without error: %v", err)
	}
	if result.Status != "blocked" || result.BlockedReason != BlockedRealModeCI || result.WorkerStarted || called {
		t.Fatalf("got %+v called=%v", result, called)
	}
}

func TestDispatchOnceRealModeWithConfirmationUsesConfiguredRunner(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "real-confirmed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "real-agent", DisplayName: "Real", Role: "engineer", DefaultWorker: "codex",
		ModelProfile: "opencode-zai-glm-5-1",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "real-budget", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-real-success", Title: "Real success", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	called := false
	var captured WorkerRunOptions
	runner := workerFunc(func(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
		called = true
		captured = opts
		return FakeWorkerRun(ctx, opts)
	})
	opts := testDispatchOpts(db, t, item, "", runner)
	opts.Mode = ModeReal
	opts.ConfirmWorkerDispatch = true
	opts.RegistryRoot = filepath.Join(t.TempDir(), "skills-registry")
	result, err := Service{Repo: db}.DispatchOnce(ctx, opts)
	if err != nil {
		t.Fatalf("real dispatch error = %v result=%+v", err, result)
	}
	if result.Status != "ok" || result.Mode != ModeReal || !result.WorkerStarted || !result.BudgetCommitted || !result.SkillSnapshotApplied {
		t.Fatalf("real dispatch result = %+v", result)
	}
	if !result.ProviderCall || !result.NetworkCall {
		t.Fatalf("real dispatch should expose provider/network call boundary: %+v", result)
	}
	if !called || captured.Mode != ModeReal || !captured.ConfirmReal || captured.TaskID != task.ID || captured.Worker != "codex" {
		t.Fatalf("runner called=%v captured=%+v", called, captured)
	}
}

func TestDispatchOnceRealModePersistsWorkerArtifactsAndEvents(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "real-artifacts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "real-artifact-agent", DisplayName: "Real Artifact", Role: "engineer", DefaultWorker: "codex",
		ModelProfile: "opencode-zai-glm-5-1",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "real-artifact-budget", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-real-artifacts", Title: "Real artifacts", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	runner := workerFunc(func(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
		return WorkerRunResult{
			Status: runs.StatusSucceeded,
			Artifacts: []artifacts.Artifact{{
				ID:      "stdout",
				Path:    "artifacts/stdout.log",
				Kind:    artifacts.KindLog,
				Content: []byte("worker output\n"),
			}},
			Events: []events.Event{{
				ID:      "worker-message",
				Type:    events.TypeWorkerMessage,
				Payload: json.RawMessage(`{"text":"ok"}`),
			}},
		}, nil
	})
	opts := testDispatchOpts(db, t, item, "", runner)
	opts.Mode = ModeReal
	opts.ConfirmWorkerDispatch = true
	opts.RegistryRoot = filepath.Join(t.TempDir(), "skills-registry")
	result, err := Service{Repo: db}.DispatchOnce(ctx, opts)
	if err != nil {
		t.Fatalf("real dispatch error = %v result=%+v", err, result)
	}
	storedArtifacts, err := db.ArtifactsByRun(ctx, result.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(storedArtifacts) != 1 || storedArtifacts[0].RunID != result.RunID || storedArtifacts[0].Kind != artifacts.KindLog || storedArtifacts[0].SizeBytes != int64(len("worker output\n")) || storedArtifacts[0].SHA256 == "" {
		t.Fatalf("stored artifacts = %+v", storedArtifacts)
	}
	if storedArtifacts[0].ID != "art_"+result.RunID+"_worker_001" || filepath.Dir(storedArtifacts[0].Path) != filepath.Join(opts.ArtifactsDir, "worker", result.RunID, "artifacts") {
		t.Fatalf("stored artifact identity/path = %+v", storedArtifacts[0])
	}
	content, err := os.ReadFile(storedArtifacts[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "worker output\n" {
		t.Fatalf("artifact content = %q", content)
	}
	storedEvents, err := db.EventsByRun(ctx, result.RunID)
	if err != nil {
		t.Fatal(err)
	}
	if len(storedEvents) != 1 || storedEvents[0].RunID != result.RunID || storedEvents[0].Type != events.TypeWorkerMessage || string(storedEvents[0].Payload) != `{"text":"ok"}` {
		t.Fatalf("stored events = %+v", storedEvents)
	}
	if storedEvents[0].ID != "evt_"+result.RunID+"_worker_001" {
		t.Fatalf("stored event id = %q", storedEvents[0].ID)
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
	if !result.LeaseReleased {
		t.Fatalf("expected lease released on budget policy block, got %+v", result)
	}
}

func TestDispatchOnceInvalidLeaseFails(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "bad-lease.db"))
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
		ID: "task-lease-invalid", Title: "Invalid lease", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	called := false
	runner := workerFunc(func(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
		called = true
		return FakeWorkerRun(ctx, opts)
	})
	_, err = Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "lease_missing", runner))
	if err == nil {
		t.Fatal("expected invalid lease error")
	}
	if called {
		t.Fatal("worker must not run with invalid lease")
	}
}

func TestDispatchOnceLeaseWrongWorkItemFails(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "wrong-lease.db"))
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
	taskA := tasks.Task{
		ID: "task-a", Title: "A", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	taskB := tasks.Task{
		ID: "task-b", Title: "B", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	itemA := seedQueuedWork(t, db, agent, policy.ID, taskA)
	itemB := seedQueuedWork(t, db, agent, policy.ID, taskB)
	claim, err := db.ClaimWorkItem(ctx, agent.ID, itemA.ID, 15*time.Minute, now)
	if err != nil || !claim.Claimed {
		t.Fatalf("ClaimWorkItem() = %+v err=%v", claim, err)
	}
	called := false
	runner := workerFunc(func(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
		called = true
		return FakeWorkerRun(ctx, opts)
	})
	_, err = Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, itemB, claim.Lease.ID, runner))
	if err == nil {
		t.Fatal("expected lease/work item mismatch error")
	}
	if called {
		t.Fatal("worker must not run with lease for another work item")
	}
}

func TestDispatchOnceInsightReviewFakeDispatch(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "review.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
		ModelProfile: "opencode-zai-glm-5-1",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "backend-monthly", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-fake-success", Title: "Parent", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	parent, err := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if err != nil || parent.Status != "ok" || parent.InsightReview == nil {
		t.Fatalf("parent dispatch = %+v err=%v", parent, err)
	}
	reviewID := parent.InsightReview.WorkItemID
	snapshot, err := db.WorkItemTaskSnapshot(ctx, reviewID)
	if err != nil {
		t.Fatalf("review task snapshot missing: %v", err)
	}
	if snapshot.WorkItemID != reviewID {
		t.Fatalf("snapshot work item = %q, want %q", snapshot.WorkItemID, reviewID)
	}
	reviewItem, err := db.WorkItem(ctx, reviewID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewItem.Kind != agents.WorkItemKindInsightReview {
		t.Fatalf("review kind = %q", reviewItem.Kind)
	}
	reviewResult, err := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, reviewItem, "", NewFakeWorkerRunner()))
	if err != nil {
		t.Fatalf("review dispatch error = %v", err)
	}
	if reviewResult.Status != "ok" || !reviewResult.WorkerStarted {
		t.Fatalf("review dispatch = %+v", reviewResult)
	}
	if reviewResult.LearningLoop != nil && reviewResult.LearningLoop.Materialized {
		t.Fatalf("review without reviewer response should not materialize learning loop: %+v", reviewResult.LearningLoop)
	}
}

func TestDispatchOnceInsightReviewMaterializesReportAndProposals(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "review-loop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
		ModelProfile: "opencode-zai-glm-5-1",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "backend-monthly", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-fake-success", Title: "Parent", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	parent, err := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if err != nil || parent.Status != "ok" || parent.InsightReview == nil || parent.EvidenceBundle == nil {
		t.Fatalf("parent dispatch = %+v err=%v", parent, err)
	}
	reviewItem, err := db.WorkItem(ctx, parent.InsightReview.WorkItemID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(reviewItem.EvidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := insights.WriteEvidenceJSON(*parent.EvidenceBundle, reviewItem.EvidencePath); err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(t.TempDir(), "reviewer-response.json")
	if err := os.WriteFile(responsePath, []byte(`{"observations":["reviewed evidence"],"what_worked":["evidence exists"],"what_failed":[],"reusable_lessons":["keep evidence-linked proposals"],"uncertainties":[],"risk_notes":[],"proposals":[{"type":"documentation","target":"docs/INSIGHT_LEARNING_LOOP.md","reason":"Document fake insight review materialization.","proposed_change_summary":"Add fixture E2E behavior to the learning loop docs.","confidence":0.91}],"action_required":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewOpts := testDispatchOpts(db, t, reviewItem, "", NewFakeWorkerRunner())
	reviewOpts.ReviewerResponsePath = responsePath
	reviewOpts.InsightPolicy = insights.Policy{Enabled: true, AutoPropose: true, Reviewer: insights.ReviewerConfig{Preferred: insights.ReviewerCodex}}
	reviewResult, err := Service{Repo: db}.DispatchOnce(ctx, reviewOpts)
	if err != nil {
		t.Fatalf("review dispatch error = %v", err)
	}
	if reviewResult.LearningLoop == nil || !reviewResult.LearningLoop.Materialized {
		t.Fatalf("expected learning loop materialized, got %+v", reviewResult.LearningLoop)
	}
	report, err := insights.ReadReportJSON(reviewResult.LearningLoop.InsightReportPath)
	if err != nil {
		t.Fatal(err)
	}
	if report.EvidenceBundleSHA256 != parent.EvidenceBundle.SHA256 || report.Reviewer != insights.ReviewerCodex {
		t.Fatalf("report = %+v, parent evidence=%s", report, parent.EvidenceBundle.SHA256)
	}
	proposals, err := insights.ReadProposalBundleJSON(reviewResult.LearningLoop.ProposalBundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(proposals.Proposals) != 1 || proposals.Proposals[0].EvidenceBundleSHA256 != parent.EvidenceBundle.SHA256 {
		t.Fatalf("proposal bundle = %+v", proposals)
	}
}

func TestDispatchOnceInsightReviewWritesApprovalAndApplyArtifacts(t *testing.T) {
	ctx := context.Background()
	db, err := openTestStoreAdapter(filepath.Join(t.TempDir(), "review-approval.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
		ModelProfile: "opencode-zai-glm-5-1",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = db.SaveAgent(ctx, agent)
	policy := budget.Policy{
		ID: "backend-monthly", Scope: budget.ScopeAgent, AgentID: agent.ID, Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 50_000_000, DefaultEstimateMicroUSD: 250_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	_ = db.SyncBudgetPolicies(ctx, []budget.Policy{policy})
	task := tasks.Task{
		ID: "task-fake-success", Title: "Parent", Domain: "general", Worker: "codex", Goal: "noop", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	item := seedQueuedWork(t, db, agent, policy.ID, task)
	parent, err := Service{Repo: db}.DispatchOnce(ctx, testDispatchOpts(db, t, item, "", NewFakeWorkerRunner()))
	if err != nil || parent.Status != "ok" || parent.InsightReview == nil || parent.EvidenceBundle == nil {
		t.Fatalf("parent dispatch = %+v err=%v", parent, err)
	}
	reviewItem, err := db.WorkItem(ctx, parent.InsightReview.WorkItemID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(reviewItem.EvidencePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := insights.WriteEvidenceJSON(*parent.EvidenceBundle, reviewItem.EvidencePath); err != nil {
		t.Fatal(err)
	}
	responsePath := filepath.Join(t.TempDir(), "reviewer-response.json")
	if err := os.WriteFile(responsePath, []byte(`{"observations":["reviewed evidence"],"what_worked":["evidence exists"],"what_failed":[],"reusable_lessons":["keep evidence-linked proposals"],"uncertainties":[],"risk_notes":[],"proposals":[{"type":"documentation","target":"docs/INSIGHT_LEARNING_LOOP.md","reason":"Document approval fixture behavior.","proposed_change_summary":"Add explicit approval artifact to the fake learning loop fixture.","confidence":0.93}],"action_required":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reviewOpts := testDispatchOpts(db, t, reviewItem, "", NewFakeWorkerRunner())
	reviewOpts.RegistryRoot = filepath.Join(t.TempDir(), "skills-registry")
	reviewOpts.ReviewerResponsePath = responsePath
	reviewOpts.InsightPolicy = insights.Policy{Enabled: true, AutoPropose: true, Reviewer: insights.ReviewerConfig{Preferred: insights.ReviewerCodex}}
	reviewOpts.LearningApprovalDecision = insights.ApprovalDecisionApproved
	reviewOpts.LearningApprovalReason = "Operator approved fixture proposal for 23.22 smoke."
	reviewOpts.LearningApprovalReviewer = insights.ReviewerOpenCode
	reviewOpts.LearningConfirmApply = true
	reviewResult, err := Service{Repo: db}.DispatchOnce(ctx, reviewOpts)
	if err != nil {
		t.Fatalf("review dispatch error = %v", err)
	}
	if reviewResult.LearningLoop == nil || reviewResult.LearningLoop.ApprovalCount != 1 {
		t.Fatalf("expected one approval artifact, got %+v", reviewResult.LearningLoop)
	}
	if len(reviewResult.LearningLoop.ApprovalPaths) != 1 {
		t.Fatalf("approval paths = %+v", reviewResult.LearningLoop.ApprovalPaths)
	}
	proposals, err := insights.ReadProposalBundleJSON(reviewResult.LearningLoop.ProposalBundlePath)
	if err != nil {
		t.Fatal(err)
	}
	approval, err := insights.ReadApprovalJSON(reviewResult.LearningLoop.ApprovalPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	if approval.Decision != insights.ApprovalDecisionApproved || approval.ProposalID != proposals.Proposals[0].ProposalID {
		t.Fatalf("approval = %+v proposal = %+v", approval, proposals.Proposals[0])
	}
	if approval.Reviewer != insights.ReviewerOpenCode {
		t.Fatalf("approval reviewer = %q, want %q", approval.Reviewer, insights.ReviewerOpenCode)
	}
	if err := insights.ValidateApprovalAgainstProposal(approval, proposals.Proposals[0]); err != nil {
		t.Fatalf("approval should bind to proposal: %v", err)
	}
	if reviewResult.LearningLoop.ApplyCount != 1 {
		t.Fatalf("expected one apply artifact, got %+v", reviewResult.LearningLoop)
	}
	if len(reviewResult.LearningLoop.ApplyResultPaths) != 1 || len(reviewResult.LearningLoop.ApplyPreviewPaths) != 1 {
		t.Fatalf("apply paths = result=%+v preview=%+v", reviewResult.LearningLoop.ApplyResultPaths, reviewResult.LearningLoop.ApplyPreviewPaths)
	}
	applyResult, err := insights.ReadApplyExecuteJSON(reviewResult.LearningLoop.ApplyResultPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	if !applyResult.Executed || applyResult.ProposalID != proposals.Proposals[0].ProposalID || applyResult.AppliedArtifact != reviewResult.LearningLoop.ApplyPreviewPaths[0] {
		t.Fatalf("apply result = %+v", applyResult)
	}
	if reviewResult.LearningLoop.EffectivenessCount != 1 || len(reviewResult.LearningLoop.EffectivenessPaths) != 1 {
		t.Fatalf("effectiveness artifacts = %+v", reviewResult.LearningLoop)
	}
	effectiveness, err := insights.ReadEffectivenessBundleJSON(reviewResult.LearningLoop.EffectivenessPaths[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(effectiveness.Records) != 1 || effectiveness.Records[0].ProposalID != proposals.Proposals[0].ProposalID || effectiveness.Records[0].RunID != reviewResult.RunID {
		t.Fatalf("effectiveness = %+v", effectiveness)
	}
	if reviewResult.LearningLoop.SessionRefreshPath == "" || reviewResult.LearningLoop.SessionSnapshotPath == "" {
		t.Fatalf("session refresh artifacts missing: %+v", reviewResult.LearningLoop)
	}
	snapshot, err := skills.ReadSnapshotJSON(reviewResult.LearningLoop.SessionSnapshotPath)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AgentID != agent.ID || snapshot.SessionID == "" || snapshot.SHA256 != reviewResult.LearningLoop.SessionSnapshotSHA256 {
		t.Fatalf("session snapshot = %+v learning_loop=%+v", snapshot, reviewResult.LearningLoop)
	}
	refreshData, err := os.ReadFile(reviewResult.LearningLoop.SessionRefreshPath)
	if err != nil {
		t.Fatal(err)
	}
	var refresh LearningSessionRefreshArtifact
	if err := json.Unmarshal(refreshData, &refresh); err != nil {
		t.Fatal(err)
	}
	if refresh.Status != "planned" || refresh.SessionID != snapshot.SessionID || refresh.SkillSnapshotPath != reviewResult.LearningLoop.SessionSnapshotPath || refresh.SkillSnapshotSHA256 != snapshot.SHA256 {
		t.Fatalf("session refresh = %+v snapshot=%+v", refresh, snapshot)
	}
	if len(refresh.ProposalIDs) != 1 || refresh.ProposalIDs[0] != proposals.Proposals[0].ProposalID || len(refresh.ApplyResultPaths) != 1 || refresh.ApplyResultPaths[0] != reviewResult.LearningLoop.ApplyResultPaths[0] {
		t.Fatalf("session refresh links = %+v", refresh)
	}
	sessions, err := db.ListSessionsByAgent(ctx, agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, session := range sessions {
		if session.ID == refresh.SessionID {
			t.Fatalf("learning refresh session should be planned-only, found persisted session %+v", session)
		}
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
