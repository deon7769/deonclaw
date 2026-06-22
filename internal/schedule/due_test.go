package schedule

import (
	"testing"
	"time"
)

func TestMaterializeUsesPersistedNextDueAt(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	due := now.Add(-5 * time.Minute)
	s := Schedule{
		ID: "hb", Kind: KindHeartbeat, Status: StatusActive, AgentID: "backend-engineer",
		HeartbeatPolicyID: "backend-default", Every: "30m",
		NextDueAt: due.Format(time.RFC3339Nano),
	}
	wakeups, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: now})
	if err != nil {
		t.Fatalf("MaterializeWakeups() error = %v", err)
	}
	if len(wakeups) != 1 {
		t.Fatalf("wakeup count = %d, want 1", len(wakeups))
	}
	if wakeups[0].DueAt != due.Format(time.RFC3339Nano) {
		t.Fatalf("due_at = %q, want %q", wakeups[0].DueAt, due.Format(time.RFC3339Nano))
	}
}

func TestMaterializeSkipsFutureCron(t *testing.T) {
	now := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	future := time.Date(2026, 6, 22, 9, 0, 0, 0, time.UTC)
	s := Schedule{
		ID: "daily", Kind: KindCron, Status: StatusActive, AgentID: "engineering-manager",
		Cron: "0 9 * * 1-5", Timezone: "UTC",
		NextDueAt: future.Format(time.RFC3339Nano),
	}
	wakeups, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: now})
	if err != nil {
		t.Fatalf("MaterializeWakeups() error = %v", err)
	}
	if len(wakeups) != 0 {
		t.Fatalf("future cron materialized early: %+v", wakeups)
	}
}

func TestMaterializeOneShotWaitsUntilDue(t *testing.T) {
	due := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	s := Schedule{
		ID: "one-shot", Kind: KindAt, Status: StatusActive, AgentID: "legacy-manual",
		At: due.Format(time.RFC3339), NextDueAt: due.Format(time.RFC3339Nano),
	}
	before, err := MaterializeWakeups(MaterializeOptions{
		Schedule: s,
		Now:      due.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("MaterializeWakeups() before due error = %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("one-shot materialized before due: %+v", before)
	}
	after, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: due})
	if err != nil || len(after) != 1 {
		t.Fatalf("one-shot at due failed: err=%v len=%d", err, len(after))
	}
}

func TestMaterializeMaxLatenessSkipsStaleDue(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	stale := now.Add(-2 * time.Hour)
	s := Schedule{
		ID: "every-task", Kind: KindEvery, Status: StatusActive, AgentID: "backend-engineer",
		Every: "30m", MaxLatenessSeconds: 900,
		NextDueAt: stale.Format(time.RFC3339Nano),
	}
	wakeups, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: now})
	if err != nil {
		t.Fatalf("MaterializeWakeups() error = %v", err)
	}
	if len(wakeups) != 0 {
		t.Fatalf("stale due within max lateness should not materialize: %+v", wakeups)
	}
}

func TestCatchUpAllMaterializesMultipleWithinLateness(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	first := now.Add(-45 * time.Minute)
	second := now.Add(-15 * time.Minute)
	s := Schedule{
		ID: "every-task", Kind: KindEvery, Status: StatusActive, AgentID: "backend-engineer",
		Every: "30m", CatchUpPolicy: CatchUpAll, MaxLatenessSeconds: 3600,
		NextDueAt: first.Format(time.RFC3339Nano),
	}
	wakeups, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: now})
	if err != nil {
		t.Fatalf("MaterializeWakeups() error = %v", err)
	}
	if len(wakeups) != 2 {
		t.Fatalf("wakeup count = %d, want 2", len(wakeups))
	}
	if wakeups[0].DueAt != first.Format(time.RFC3339Nano) {
		t.Fatalf("first due = %q", wakeups[0].DueAt)
	}
	if wakeups[1].DueAt != second.Format(time.RFC3339Nano) {
		t.Fatalf("second due = %q", wakeups[1].DueAt)
	}
}
