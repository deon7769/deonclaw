package schedule

import (
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/wakeup"
)

func TestValidateConfigOK(t *testing.T) {
	cfg, err := ParseConfig([]byte(exampleSchedulesYAML))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}

func TestCronParseWithTimezone(t *testing.T) {
	sched, err := ParseCronExpression("0 9 * * 1-5", "America/Sao_Paulo")
	if err != nil {
		t.Fatalf("ParseCronExpression() error = %v", err)
	}
	next := sched.Next(time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC))
	if next.IsZero() {
		t.Fatal("next due is zero")
	}
}

func TestMaterializeOneShotCreatesOneWakeup(t *testing.T) {
	due := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	s := Schedule{
		ID: "one-shot", Kind: KindAt, Status: StatusActive, AgentID: "legacy-manual",
		At: due.Format(time.RFC3339), NextDueAt: due.Format(time.RFC3339Nano),
	}
	wakeups, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: due})
	if err != nil {
		t.Fatalf("MaterializeWakeups() error = %v", err)
	}
	if len(wakeups) != 1 {
		t.Fatalf("wakeup count = %d", len(wakeups))
	}
}

func TestIdempotencyPreventsDuplicateWakeup(t *testing.T) {
	now := time.Date(2026, 6, 22, 8, 0, 0, 0, time.UTC)
	s := Schedule{
		ID: "every-task", Kind: KindEvery, Status: StatusActive, AgentID: "backend-engineer", Every: "30m",
		NextDueAt: now.Format(time.RFC3339Nano),
	}
	first, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: now})
	if err != nil || len(first) != 1 {
		t.Fatalf("first materialize failed: %v len=%d", err, len(first))
	}
	second, err := MaterializeWakeups(MaterializeOptions{Schedule: s, Now: now, Existing: first})
	if err != nil {
		t.Fatalf("MaterializeWakeups() error = %v", err)
	}
	if len(second) != 0 {
		t.Fatalf("duplicate wakeup materialized")
	}
}

func TestSkipIfRunning(t *testing.T) {
	due := []wakeup.Wakeup{{ID: "w1", Status: wakeup.StatusQueued}}
	allowed, skipped := ApplyConcurrencyPolicy(ConcurrencySkipIfRunning, due, true)
	if len(allowed) != 0 || len(skipped) != 1 {
		t.Fatalf("skip_if_running mismatch allowed=%d skipped=%d", len(allowed), len(skipped))
	}
}

func TestCoalesceMergesWakeups(t *testing.T) {
	items := []wakeup.Wakeup{
		{ID: "a", DueAt: "2026-06-22T08:00:00Z"},
		{ID: "b", DueAt: "2026-06-22T09:00:00Z"},
	}
	out := CoalesceWakeups(items)
	if len(out) != 1 || out[0].ID != "b" {
		t.Fatalf("coalesce result = %+v", out)
	}
}

const exampleSchedulesYAML = `
schedules:
  - id: daily-review
    kind: cron
    name: Daily repository review
    status: active
    agent_id: engineering-manager
    cron: "0 9 * * 1-5"
    timezone: UTC
`
