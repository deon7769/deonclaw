package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/heartbeat"
	"github.com/deon7769/deonclaw/internal/hooks"
	"github.com/deon7769/deonclaw/internal/schedule"
	"github.com/deon7769/deonclaw/internal/store"
)

func TestRunOnceProcessesDueWakeup(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "proactive.db")
	sqlite, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer sqlite.Close()

	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "backend-engineer", DisplayName: "Backend", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	if err := sqlite.SaveAgent(ctx, agent); err != nil {
		t.Fatalf("SaveAgent() error = %v", err)
	}

	sched := schedule.Schedule{
		ID: "hb", Kind: schedule.KindHeartbeat, Name: "hb", Status: schedule.StatusActive,
		AgentID: agent.ID, HeartbeatPolicyID: "backend-default", Every: "30m",
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
		NextDueAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
	}
	if err := sqlite.SaveSchedule(ctx, sched); err != nil {
		t.Fatalf("SaveSchedule() error = %v", err)
	}

	hbCfg, err := heartbeat.ParseConfig([]byte(`
heartbeat_policies:
  backend-default:
    enabled: true
    every: 30m
    no_op_token: HEARTBEAT_OK
`))
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	hooksCfg, _ := hooks.ParseConfig([]byte(`hooks: []`))

	repo := store.ProactiveRepo{Store: sqlite, Ctx: ctx}
	result, err := RunOnce(repo, RunOnceOptions{
		Now: now, HeartbeatConfig: hbCfg, HooksConfig: hooksCfg,
	})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Processed+result.Skipped == 0 && result.Materialized == 0 {
		t.Fatalf("run-once did nothing: %+v", result)
	}
}

func TestRunOnceSkipsPausedAgent(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "paused.db")
	sqlite, err := store.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer sqlite.Close()

	now := time.Now().UTC()
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID: "paused-agent", DisplayName: "Paused", Role: "engineer", DefaultWorker: "codex",
	}, agents.StatusPaused, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	_ = sqlite.SaveAgent(ctx, agent)
	sched := schedule.Schedule{
		ID: "task", Kind: schedule.KindEvery, Name: "task", Status: schedule.StatusActive,
		AgentID: agent.ID, Every: "30m",
		CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano),
		NextDueAt: now.Add(-time.Minute).Format(time.RFC3339Nano),
	}
	_ = sqlite.SaveSchedule(ctx, sched)

	repo := store.ProactiveRepo{Store: sqlite, Ctx: ctx}
	result, err := RunOnce(repo, RunOnceOptions{Now: now, HooksConfig: hooks.Config{}})
	if err != nil {
		t.Fatalf("RunOnce() error = %v", err)
	}
	if result.Skipped == 0 && result.Processed > 0 {
		t.Fatalf("paused agent should not process runs: %+v", result)
	}
}
