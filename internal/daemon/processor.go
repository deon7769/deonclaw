package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/heartbeat"
	"github.com/deon7769/deonclaw/internal/hooks"
	"github.com/deon7769/deonclaw/internal/schedule"
	"github.com/deon7769/deonclaw/internal/wakeup"
)

const (
	StateKeyStatus = "daemon.status"
	StatusStopped  = "stopped"
	StatusRunning  = "running"
)

type State struct {
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	PID       int    `json:"pid,omitempty"`
}

type Repository interface {
	GetDaemonState(key string) (string, error)
	SetDaemonState(key string, value string, updatedAt time.Time) error
	ListSchedules() ([]schedule.Schedule, error)
	SaveSchedule(schedule.Schedule) error
	ListWakeups() ([]wakeup.Wakeup, error)
	SaveWakeup(wakeup.Wakeup) error
	ClaimNextDueWakeup(now time.Time) (wakeup.Wakeup, bool, error)
	GetAgent(id string) (agents.Agent, error)
	GetHeartbeatState(agentID string) (heartbeat.State, error)
	SaveHeartbeatState(state heartbeat.State) error
}

type RunOnceOptions struct {
	Now              time.Time
	ClaimTTL         time.Duration
	HeartbeatConfig  heartbeat.Config
	HooksConfig      hooks.Config
	AgentBusyChecker func(agentID string) bool
}

type RunOnceResult struct {
	Materialized int             `json:"materialized"`
	Processed    int             `json:"processed"`
	Skipped      int             `json:"skipped"`
	Wakeups      []wakeup.Wakeup `json:"wakeups"`
	HookPlans    []hooks.Plan    `json:"hook_plans,omitempty"`
}

type DoctorReport struct {
	Status         string                `json:"status"`
	DaemonState    State                 `json:"daemon_state"`
	ScheduleCount  int                   `json:"schedule_count"`
	QueuedWakeups  int                   `json:"queued_wakeups"`
	WakeupRecovery wakeup.RecoveryReport `json:"wakeup_recovery"`
}

func ParseState(raw string) (State, error) {
	if strings.TrimSpace(raw) == "" {
		return State{Status: StatusStopped}, nil
	}
	var state State
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return State{}, err
	}
	return state, nil
}

func EncodeState(state State) (string, error) {
	data, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func Doctor(repo Repository, now time.Time, claimTTL time.Duration) (DoctorReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	raw, _ := repo.GetDaemonState(StateKeyStatus)
	state, _ := ParseState(raw)
	schedules, err := repo.ListSchedules()
	if err != nil {
		return DoctorReport{}, err
	}
	wakeups, err := repo.ListWakeups()
	if err != nil {
		return DoctorReport{}, err
	}
	updated, recovery := wakeup.RecoverStaleClaims(wakeups, now, claimTTL)
	for _, w := range updated {
		for i, original := range wakeups {
			if original.ID == w.ID && original.Status != w.Status {
				if err := repo.SaveWakeup(w); err != nil {
					return DoctorReport{}, err
				}
				wakeups[i] = w
			}
		}
	}
	queued := 0
	for _, w := range wakeups {
		if w.Status == wakeup.StatusQueued {
			queued++
		}
	}
	status := "ok"
	if recovery.Status != "ok" {
		status = recovery.Status
	}
	return DoctorReport{
		Status:         status,
		DaemonState:    state,
		ScheduleCount:  len(schedules),
		QueuedWakeups:  queued,
		WakeupRecovery: recovery,
	}, nil
}

func RunOnce(repo Repository, opts RunOnceOptions) (RunOnceResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if opts.ClaimTTL == 0 {
		opts.ClaimTTL = 15 * time.Minute
	}
	schedules, err := repo.ListSchedules()
	if err != nil {
		return RunOnceResult{}, err
	}
	existing, err := repo.ListWakeups()
	if err != nil {
		return RunOnceResult{}, err
	}
	result := RunOnceResult{}
	for _, s := range schedules {
		newOnes, err := schedule.MaterializeWakeups(schedule.MaterializeOptions{
			Schedule: s,
			Now:      now,
			Existing: existing,
		})
		if err != nil {
			return RunOnceResult{}, fmt.Errorf("schedule %q: %w", s.ID, err)
		}
		hasRunning := wakeup.HasActiveForSchedule(existing, s.ID)
		allowed, skipped := schedule.ApplyConcurrencyPolicy(s.ConcurrencyPolicy, newOnes, hasRunning)
		for _, w := range skipped {
			if err := repo.SaveWakeup(w); err != nil {
				return RunOnceResult{}, err
			}
			existing = append(existing, w)
			result.Skipped++
		}
		for _, w := range allowed {
			if err := repo.SaveWakeup(w); err != nil {
				return RunOnceResult{}, err
			}
			existing = append(existing, w)
			result.Materialized++
		}
	}
	var candidate *wakeup.Wakeup
	claimed, ok, err := repo.ClaimNextDueWakeup(now)
	if err != nil {
		return RunOnceResult{}, err
	}
	if ok {
		candidate = &claimed
	}
	if candidate == nil {
		return result, nil
	}
	agent, err := repo.GetAgent(candidate.AgentID)
	if err != nil {
		w, finishErr := wakeup.Finish(wakeup.FinishOptions{
			Wakeup:     *candidate,
			Status:     wakeup.StatusSkipped,
			SkipReason: "agent not found",
			Now:        now,
		})
		if finishErr != nil {
			return RunOnceResult{}, finishErr
		}
		if err := repo.SaveWakeup(w); err != nil {
			return RunOnceResult{}, err
		}
		result.Skipped++
		return result, nil
	}
	if err := agents.CanStartRun(agent); err != nil {
		w, finishErr := wakeup.Finish(wakeup.FinishOptions{
			Wakeup:     *candidate,
			Status:     wakeup.StatusSkipped,
			SkipReason: err.Error(),
			Now:        now,
		})
		if finishErr != nil {
			return RunOnceResult{}, finishErr
		}
		if saveErr := repo.SaveWakeup(w); saveErr != nil {
			return RunOnceResult{}, saveErr
		}
		result.Skipped++
		return result, nil
	}
	running := wakeup.MarkRunning(*candidate, now)
	if err := repo.SaveWakeup(running); err != nil {
		return RunOnceResult{}, err
	}
	var sched schedule.Schedule
	for _, s := range schedules {
		if s.ID == running.ScheduleID {
			sched = s
			break
		}
	}
	if sched.Kind == schedule.KindHeartbeat {
		policyID := sched.HeartbeatPolicyID
		policy, ok := heartbeat.PolicyByID(opts.HeartbeatConfig, policyID)
		if !ok {
			w, _ := wakeup.Finish(wakeup.FinishOptions{Wakeup: running, Status: wakeup.StatusFailed, SkipReason: "heartbeat policy not found", Now: now})
			_ = repo.SaveWakeup(w)
			result.Skipped++
			return result, nil
		}
		busy := false
		if opts.AgentBusyChecker != nil {
			busy = opts.AgentBusyChecker(agent.ID)
		}
		plan := heartbeat.PlanDryRun(agent.ID, policyID, policy, busy)
		if !plan.WouldRun {
			w, _ := wakeup.Finish(wakeup.FinishOptions{Wakeup: running, Status: wakeup.StatusSkipped, SkipReason: plan.BlockedReason, Now: now})
			_ = repo.SaveWakeup(w)
			_ = repo.SaveHeartbeatState(heartbeat.State{
				AgentID: agent.ID, PolicyID: policyID, LastDueAt: running.DueAt,
				LastRunAt: now.Format(time.RFC3339Nano), LastStatus: wakeup.StatusSkipped,
				LastSummary: plan.BlockedReason, UpdatedAt: now.Format(time.RFC3339Nano),
			})
			result.Skipped++
			return result, nil
		}
		eval := heartbeat.EvaluateResult(policy, policy.NoOpToken)
		w, _ := wakeup.Finish(wakeup.FinishOptions{
			Wakeup: running, Status: wakeup.StatusSucceeded, Now: now,
		})
		_ = repo.SaveWakeup(w)
		_ = repo.SaveHeartbeatState(heartbeat.State{
			AgentID: agent.ID, PolicyID: policyID, LastDueAt: running.DueAt,
			LastRunAt: now.Format(time.RFC3339Nano), LastStatus: wakeup.StatusSucceeded,
			LastSummary: eval.Summary, UpdatedAt: now.Format(time.RFC3339Nano),
		})
		result.Processed++
		result.Wakeups = append(result.Wakeups, w)
	} else {
		w, _ := wakeup.Finish(wakeup.FinishOptions{
			Wakeup: running, Status: wakeup.StatusSucceeded, SkipReason: "dry-run: work item materialization only", Now: now,
		})
		_ = repo.SaveWakeup(w)
		result.Processed++
		result.Wakeups = append(result.Wakeups, w)
	}
	if sched.ID != "" {
		advanced, err := schedule.AdvanceScheduleAfterRun(sched, now)
		if err == nil {
			_ = repo.SaveSchedule(advanced)
		}
	}
	result.HookPlans = hooks.PlanForEvent(opts.HooksConfig, hooks.EventScheduleDue)
	return result, nil
}
