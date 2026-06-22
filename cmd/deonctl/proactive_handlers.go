package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/deon7769/deonclaw/internal/daemon"
	"github.com/deon7769/deonclaw/internal/heartbeat"
	"github.com/deon7769/deonclaw/internal/hooks"
	"github.com/deon7769/deonclaw/internal/schedule"
	"github.com/deon7769/deonclaw/internal/store"
)

func runDaemonStatus(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon status failed: %v\n", err)
		return 1
	}
	defer db.Close()
	raw, err := db.GetDaemonState(context.Background(), daemon.StateKeyStatus)
	if err != nil {
		fmt.Fprintf(stderr, "daemon status failed: %v\n", err)
		return 1
	}
	state, _ := daemon.ParseState(raw)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(state)
	return 0
}

func runDaemonDoctor(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon doctor failed: %v\n", err)
		return 1
	}
	defer db.Close()
	repo := store.ProactiveRepo{Store: db, Ctx: context.Background()}
	report, err := daemon.Doctor(repo, time.Now().UTC(), 15*time.Minute)
	if err != nil {
		fmt.Fprintf(stderr, "daemon doctor failed: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return 1
	}
	if report.Status != "ok" {
		return 1
	}
	return 0
}

func runDaemonStart(storePath string, stdout io.Writer, stderr io.Writer) int {
	return setDaemonStatus(storePath, daemon.StatusRunning, stdout, stderr)
}

func runDaemonStop(storePath string, stdout io.Writer, stderr io.Writer) int {
	return setDaemonStatus(storePath, daemon.StatusStopped, stdout, stderr)
}

func setDaemonStatus(storePath string, status string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon state update failed: %v\n", err)
		return 1
	}
	defer db.Close()
	now := time.Now().UTC()
	state := daemon.State{Status: status, UpdatedAt: now.Format(time.RFC3339Nano), PID: os.Getpid()}
	encoded, err := daemon.EncodeState(state)
	if err != nil {
		fmt.Fprintf(stderr, "daemon state update failed: %v\n", err)
		return 1
	}
	if err := db.SetDaemonState(context.Background(), daemon.StateKeyStatus, encoded, now); err != nil {
		fmt.Fprintf(stderr, "daemon state update failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "daemon: status=%s\n", status)
	return 0
}

type daemonRunOnceOptions struct {
	storePath       string
	heartbeatConfig string
	hooksConfig     string
}

func runDaemonRunOnce(opts daemonRunOnceOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "daemon run-once failed: %v\n", err)
		return 1
	}
	defer db.Close()
	hbCfg := heartbeat.Config{HeartbeatPolicies: map[string]heartbeat.Policy{}}
	if strings.TrimSpace(opts.heartbeatConfig) != "" {
		hbCfg, err = heartbeat.LoadConfig(opts.heartbeatConfig)
		if err != nil {
			fmt.Fprintf(stderr, "daemon run-once failed: %v\n", err)
			return 1
		}
	}
	hooksCfg := hooks.Config{}
	if strings.TrimSpace(opts.hooksConfig) != "" {
		hooksCfg, err = hooks.LoadConfig(opts.hooksConfig)
		if err != nil {
			fmt.Fprintf(stderr, "daemon run-once failed: %v\n", err)
			return 1
		}
	}
	repo := store.ProactiveRepo{Store: db, Ctx: context.Background()}
	result, err := daemon.RunOnce(repo, daemon.RunOnceOptions{
		Now:             time.Now().UTC(),
		HeartbeatConfig: hbCfg,
		HooksConfig:     hooksCfg,
	})
	if err != nil {
		fmt.Fprintf(stderr, "daemon run-once failed: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
	return 0
}

func runSchedulesValidate(configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := schedule.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "schedules validate failed: %v\n", err)
		return 1
	}
	if err := schedule.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "schedules validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "schedules validate: ok")
	return 0
}

func runSchedulesSync(configPath string, storePath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := schedule.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "schedules sync failed: %v\n", err)
		return 1
	}
	if err := schedule.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "schedules sync failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "schedules sync failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	for _, sched := range schedule.SyncFromConfig(cfg, time.Now().UTC()) {
		if err := db.SaveSchedule(ctx, sched); err != nil {
			fmt.Fprintf(stderr, "schedules sync failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "schedules sync: ok count=%d\n", len(cfg.Schedules))
	return 0
}

func runSchedulesList(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		return 1
	}
	defer db.Close()
	items, err := db.ListSchedules(context.Background())
	if err != nil {
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "id\tkind\tstatus\tagent\tnext_due_at")
	for _, s := range items {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", s.ID, s.Kind, s.Status, s.AgentID, s.NextDueAt)
	}
	_ = writer.Flush()
	return 0
}

func runSchedulesDue(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		return 1
	}
	defer db.Close()
	items, err := db.ListSchedules(context.Background())
	if err != nil {
		return 1
	}
	fmt.Fprint(stdout, schedule.FormatDueReport(items, time.Now().UTC()))
	return 0
}

func runHeartbeatValidate(configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := heartbeat.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "heartbeat validate failed: %v\n", err)
		return 1
	}
	if err := heartbeat.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "heartbeat validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "heartbeat validate: ok")
	return 0
}

func runHeartbeatDryRun(agentID string, policyID string, configPath string, agentBusy bool, stdout io.Writer, stderr io.Writer) int {
	cfg, err := heartbeat.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "heartbeat dry-run failed: %v\n", err)
		return 1
	}
	policy, ok := heartbeat.PolicyByID(cfg, policyID)
	if !ok {
		fmt.Fprintf(stderr, "heartbeat dry-run failed: policy %q not found\n", policyID)
		return 1
	}
	plan := heartbeat.PlanDryRun(agentID, policyID, policy, agentBusy)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(plan)
	return 0
}

func runHooksValidate(configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := hooks.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "hooks validate failed: %v\n", err)
		return 1
	}
	if err := hooks.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "hooks validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "hooks validate: ok")
	return 0
}

func runHooksPlan(configPath string, event string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := hooks.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "hooks plan failed: %v\n", err)
		return 1
	}
	plans := hooks.PlanForEvent(cfg, event)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(plans)
	return 0
}
