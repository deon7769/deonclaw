package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/usage"
)

func TestCommitReservationWithUsageInsertsUsage(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "budget-commit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	policy := budget.Policy{
		ID: "agent-monthly", Scope: budget.ScopeAgent, AgentID: "agent-a", Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 10_000_000, DefaultEstimateMicroUSD: 300_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	if err := db.SyncBudgetPolicies(ctx, []budget.Policy{policy}); err != nil {
		t.Fatal(err)
	}
	reserve, err := db.ReserveBudget(ctx, budget.ReserveBudgetOptions{
		PolicyID: policy.ID, WorkItemID: "work_a", RunID: "run_a", AgentID: "agent-a",
		EstimatedMicroUSD: 300_000, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	event := usage.Event{
		ID: "usage_run_a", RunID: "run_a", WorkItemID: "work_a", AgentID: "agent-a",
		Worker: "codex", Source: usage.SourceFakeFixture, Confidence: usage.ConfidenceEstimated,
		EstimatedCostMicroUSD: 180_000, ActualCostMicroUSD: 180_000, CreatedAt: now.Format(time.RFC3339Nano),
	}
	committed, err := db.CommitReservationWithUsage(ctx, reserve.Reservation.ID, event, 180_000, now)
	if err != nil {
		t.Fatalf("CommitReservationWithUsage() error = %v", err)
	}
	if committed.Status != budget.ReservationCommitted {
		t.Fatalf("reservation status = %q", committed.Status)
	}
	saved, err := db.UsageEvent(ctx, event.ID)
	if err != nil {
		t.Fatalf("UsageEvent() error = %v", err)
	}
	if saved.ActualCostMicroUSD != 180_000 {
		t.Fatalf("usage actual = %d", saved.ActualCostMicroUSD)
	}
}

func TestCommitReservationWithUsageOverage(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "budget-overage.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	policy := budget.Policy{
		ID: "agent-monthly", Scope: budget.ScopeAgent, AgentID: "agent-a", Period: budget.PeriodMonthly,
		Timezone: "UTC", HardLimitMicroUSD: 10_000_000, DefaultEstimateMicroUSD: 300_000,
		MaxSingleRunMicroUSD: 3_000_000, ReserveBeforeRun: true, OnExhausted: budget.OnExhaustedBlockWork,
	}
	if err := db.SyncBudgetPolicies(ctx, []budget.Policy{policy}); err != nil {
		t.Fatal(err)
	}
	reserve, err := db.ReserveBudget(ctx, budget.ReserveBudgetOptions{
		PolicyID: policy.ID, WorkItemID: "work_b", RunID: "run_b", AgentID: "agent-a",
		EstimatedMicroUSD: 300_000, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	event := usage.Event{
		ID: "usage_run_b", RunID: "run_b", WorkItemID: "work_b", AgentID: "agent-a",
		Worker: "codex", Source: usage.SourceFakeFixture, Confidence: usage.ConfidenceEstimated,
		EstimatedCostMicroUSD: 450_000, ActualCostMicroUSD: 450_000, CreatedAt: now.Format(time.RFC3339Nano),
	}
	committed, err := db.CommitReservationWithUsage(ctx, reserve.Reservation.ID, event, 450_000, now)
	if err != nil {
		t.Fatalf("CommitReservationWithUsage() overage error = %v", err)
	}
	if committed.Status != budget.ReservationOverBudget {
		t.Fatalf("reservation status = %q, want over_budget", committed.Status)
	}
	if committed.CommittedMicroUSD != 450_000 {
		t.Fatalf("committed = %d", committed.CommittedMicroUSD)
	}
}
