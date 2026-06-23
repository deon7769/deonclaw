package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/heartbeat"
	"github.com/deon7769/deonclaw/internal/schedule"
	"github.com/deon7769/deonclaw/internal/wakeup"
)

func (s *SQLiteStore) bootstrapProactive(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS schedules (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			config_json TEXT NOT NULL,
			next_due_at TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_schedules_agent ON schedules(agent_id, status)`,
		`CREATE TABLE IF NOT EXISTS wakeups (
			id TEXT PRIMARY KEY,
			schedule_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			due_at TEXT NOT NULL,
			status TEXT NOT NULL,
			idempotency_key TEXT NOT NULL UNIQUE,
			attempt INTEGER NOT NULL DEFAULT 0,
			run_id TEXT,
			config_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			claimed_at TEXT,
			finished_at TEXT,
			FOREIGN KEY (schedule_id) REFERENCES schedules(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wakeups_due ON wakeups(status, due_at)`,
		`CREATE TABLE IF NOT EXISTS heartbeat_state (
			agent_id TEXT NOT NULL,
			policy_id TEXT NOT NULL,
			last_due_at TEXT,
			last_run_at TEXT,
			last_status TEXT,
			last_summary TEXT,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (agent_id, policy_id)
		)`,
		`CREATE TABLE IF NOT EXISTS daemon_state (
			key TEXT PRIMARY KEY,
			value_json TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS hook_definitions (
			id TEXT PRIMARY KEY,
			event TEXT NOT NULL,
			action TEXT NOT NULL,
			status TEXT NOT NULL,
			config_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap proactive tables: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SaveSchedule(ctx context.Context, sched schedule.Schedule) error {
	data, err := json.Marshal(sched)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO schedules (
		id, kind, status, agent_id, config_json, next_due_at, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		kind = excluded.kind,
		status = excluded.status,
		agent_id = excluded.agent_id,
		config_json = excluded.config_json,
		next_due_at = excluded.next_due_at,
		updated_at = excluded.updated_at`,
		sched.ID, sched.Kind, sched.Status, sched.AgentID, string(data), nullIfEmpty(sched.NextDueAt),
		sched.CreatedAt, sched.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save schedule %q: %w", sched.ID, err)
	}
	return nil
}

func (s *SQLiteStore) Schedule(ctx context.Context, id string) (schedule.Schedule, error) {
	row := s.db.QueryRowContext(ctx, `SELECT config_json FROM schedules WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return schedule.Schedule{}, fmt.Errorf("schedule %q: %w", id, ErrNotFound)
		}
		return schedule.Schedule{}, err
	}
	return decodeSchedule(raw)
}

func (s *SQLiteStore) ListSchedules(ctx context.Context) ([]schedule.Schedule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config_json FROM schedules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]schedule.Schedule, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		sched, err := decodeSchedule(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, sched)
	}
	return out, rows.Err()
}

func decodeSchedule(raw string) (schedule.Schedule, error) {
	var sched schedule.Schedule
	if err := json.Unmarshal([]byte(raw), &sched); err != nil {
		return schedule.Schedule{}, err
	}
	return sched, nil
}

func (s *SQLiteStore) SaveWakeup(ctx context.Context, w wakeup.Wakeup) error {
	data, err := json.Marshal(map[string]string{
		"skip_reason":     w.SkipReason,
		"work_item_id":    w.WorkItemID,
		"terminal_run_id": w.TerminalRunID,
	})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO wakeups (
		id, schedule_id, agent_id, due_at, status, idempotency_key, attempt, run_id, config_json, created_at, claimed_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		status = excluded.status,
		attempt = excluded.attempt,
		run_id = excluded.run_id,
		config_json = excluded.config_json,
		claimed_at = excluded.claimed_at,
		finished_at = excluded.finished_at`,
		w.ID, w.ScheduleID, w.AgentID, w.DueAt, w.Status, w.IdempotencyKey, w.Attempt,
		nullIfEmpty(w.RunID), string(data), w.CreatedAt, nullIfEmpty(w.ClaimedAt), nullIfEmpty(w.FinishedAt),
	)
	if err != nil {
		return fmt.Errorf("save wakeup %q: %w", w.ID, err)
	}
	return nil
}

// ClaimNextDueWakeup atomically claims the earliest queued wakeup with due_at <= now.
func (s *SQLiteStore) ClaimNextDueWakeup(ctx context.Context, now time.Time) (wakeup.Wakeup, bool, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wakeup.Wakeup{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	dueCutoff := now.UTC().Format(time.RFC3339Nano)
	row := tx.QueryRowContext(ctx, `SELECT id, schedule_id, agent_id, due_at, status, idempotency_key, attempt, run_id, config_json, created_at, claimed_at, finished_at
		FROM wakeups
		WHERE status = ? AND due_at <= ?
		ORDER BY due_at ASC, id ASC
		LIMIT 1`,
		wakeup.StatusQueued, dueCutoff,
	)
	candidate, err := scanWakeup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return wakeup.Wakeup{}, false, nil
	}
	if err != nil {
		return wakeup.Wakeup{}, false, err
	}

	claimedAt := now.UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE wakeups
		SET status = ?, claimed_at = ?, attempt = attempt + 1
		WHERE id = ? AND status = ?`,
		wakeup.StatusClaimed, claimedAt, candidate.ID, wakeup.StatusQueued,
	)
	if err != nil {
		return wakeup.Wakeup{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return wakeup.Wakeup{}, false, err
	}
	if affected == 0 {
		return wakeup.Wakeup{}, false, nil
	}
	if err := tx.Commit(); err != nil {
		return wakeup.Wakeup{}, false, err
	}
	candidate.Status = wakeup.StatusClaimed
	candidate.ClaimedAt = claimedAt
	candidate.Attempt++
	return candidate, true, nil
}

func (s *SQLiteStore) ListWakeups(ctx context.Context) ([]wakeup.Wakeup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, schedule_id, agent_id, due_at, status, idempotency_key, attempt, run_id, config_json, created_at, claimed_at, finished_at FROM wakeups ORDER BY due_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]wakeup.Wakeup, 0)
	for rows.Next() {
		w, err := scanWakeup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func scanWakeup(scanner interface{ Scan(...any) error }) (wakeup.Wakeup, error) {
	var w wakeup.Wakeup
	var runID, claimedAt, finishedAt sql.NullString
	var configJSON string
	if err := scanner.Scan(&w.ID, &w.ScheduleID, &w.AgentID, &w.DueAt, &w.Status, &w.IdempotencyKey, &w.Attempt, &runID, &configJSON, &w.CreatedAt, &claimedAt, &finishedAt); err != nil {
		return wakeup.Wakeup{}, err
	}
	w.RunID = runID.String
	w.ClaimedAt = claimedAt.String
	w.FinishedAt = finishedAt.String
	var extra map[string]string
	_ = json.Unmarshal([]byte(configJSON), &extra)
	w.SkipReason = extra["skip_reason"]
	w.WorkItemID = extra["work_item_id"]
	w.TerminalRunID = extra["terminal_run_id"]
	return w, nil
}

func (s *SQLiteStore) GetDaemonState(ctx context.Context, key string) (string, error) {
	row := s.db.QueryRowContext(ctx, `SELECT value_json FROM daemon_state WHERE key = ?`, key)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return raw, nil
}

func (s *SQLiteStore) SetDaemonState(ctx context.Context, key string, value string, updatedAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO daemon_state (key, value_json, updated_at) VALUES (?, ?, ?)
	ON CONFLICT(key) DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at`,
		key, value, formatTime(updatedAt),
	)
	return err
}

func (s *SQLiteStore) SaveHeartbeatState(ctx context.Context, state heartbeat.State) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO heartbeat_state (
		agent_id, policy_id, last_due_at, last_run_at, last_status, last_summary, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(agent_id, policy_id) DO UPDATE SET
		last_due_at = excluded.last_due_at,
		last_run_at = excluded.last_run_at,
		last_status = excluded.last_status,
		last_summary = excluded.last_summary,
		updated_at = excluded.updated_at`,
		state.AgentID, state.PolicyID, nullIfEmpty(state.LastDueAt), nullIfEmpty(state.LastRunAt),
		nullIfEmpty(state.LastStatus), nullIfEmpty(state.LastSummary), state.UpdatedAt,
	)
	return err
}

func (s *SQLiteStore) HeartbeatState(ctx context.Context, agentID string, policyID string) (heartbeat.State, error) {
	row := s.db.QueryRowContext(ctx, `SELECT agent_id, policy_id, last_due_at, last_run_at, last_status, last_summary, updated_at
		FROM heartbeat_state WHERE agent_id = ? AND policy_id = ?`, agentID, policyID)
	var state heartbeat.State
	var lastDue, lastRun, lastStatus, lastSummary sql.NullString
	if err := row.Scan(&state.AgentID, &state.PolicyID, &lastDue, &lastRun, &lastStatus, &lastSummary, &state.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return heartbeat.State{}, fmt.Errorf("heartbeat state: %w", ErrNotFound)
		}
		return heartbeat.State{}, err
	}
	state.LastDueAt = lastDue.String
	state.LastRunAt = lastRun.String
	state.LastStatus = lastStatus.String
	state.LastSummary = lastSummary.String
	return state, nil
}

// ProactiveRepo adapts SQLiteStore to daemon.Repository.
type ProactiveRepo struct {
	Store *SQLiteStore
	Ctx   context.Context
}

func (r ProactiveRepo) GetDaemonState(key string) (string, error) {
	return r.Store.GetDaemonState(r.Ctx, key)
}

func (r ProactiveRepo) SetDaemonState(key string, value string, updatedAt time.Time) error {
	return r.Store.SetDaemonState(r.Ctx, key, value, updatedAt)
}

func (r ProactiveRepo) ListSchedules() ([]schedule.Schedule, error) {
	return r.Store.ListSchedules(r.Ctx)
}

func (r ProactiveRepo) SaveSchedule(sched schedule.Schedule) error {
	return r.Store.SaveSchedule(r.Ctx, sched)
}

func (r ProactiveRepo) ListWakeups() ([]wakeup.Wakeup, error) {
	return r.Store.ListWakeups(r.Ctx)
}

func (r ProactiveRepo) SaveWakeup(w wakeup.Wakeup) error {
	return r.Store.SaveWakeup(r.Ctx, w)
}

func (r ProactiveRepo) ClaimNextDueWakeup(now time.Time) (wakeup.Wakeup, bool, error) {
	return r.Store.ClaimNextDueWakeup(r.Ctx, now)
}

func (r ProactiveRepo) GetAgent(id string) (agents.Agent, error) {
	return r.Store.Agent(r.Ctx, id)
}

func (r ProactiveRepo) GetHeartbeatState(agentID string) (heartbeat.State, error) {
	// policy id resolved by caller via schedule; return latest for agent if single
	rows, err := r.Store.db.QueryContext(r.Ctx, `SELECT agent_id, policy_id, last_due_at, last_run_at, last_status, last_summary, updated_at FROM heartbeat_state WHERE agent_id = ? ORDER BY updated_at DESC LIMIT 1`, agentID)
	if err != nil {
		return heartbeat.State{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return heartbeat.State{AgentID: agentID}, nil
	}
	var state heartbeat.State
	var lastDue, lastRun, lastStatus, lastSummary sql.NullString
	if err := rows.Scan(&state.AgentID, &state.PolicyID, &lastDue, &lastRun, &lastStatus, &lastSummary, &state.UpdatedAt); err != nil {
		return heartbeat.State{}, err
	}
	state.LastDueAt = lastDue.String
	state.LastRunAt = lastRun.String
	state.LastStatus = lastStatus.String
	state.LastSummary = lastSummary.String
	return state, nil
}

func (r ProactiveRepo) SaveHeartbeatState(state heartbeat.State) error {
	return r.Store.SaveHeartbeatState(r.Ctx, state)
}

func (r ProactiveRepo) MaterializeWorkFromWakeup(w wakeup.Wakeup, sched schedule.Schedule, now time.Time) (string, bool, error) {
	result, err := r.Store.MaterializeWorkFromWakeup(r.Ctx, MaterializeWorkFromWakeupOptions{
		Wakeup: w, Schedule: sched, Now: now,
	})
	if err != nil {
		return "", false, err
	}
	return result.WorkItemID, result.Created, nil
}

func (s *SQLiteStore) SaveHookDefinitions(ctx context.Context, hooksJSON []byte, now time.Time) error {
	type hookRow struct {
		ID      string `json:"id"`
		Event   string `json:"event"`
		Action  string `json:"action"`
		Enabled bool   `json:"enabled"`
		Policy  string `json:"policy"`
	}
	var payload struct {
		Hooks []hookRow `json:"hooks"`
	}
	if err := json.Unmarshal(hooksJSON, &payload); err != nil {
		return err
	}
	stamp := formatTime(now)
	for _, hook := range payload.Hooks {
		cfg, _ := json.Marshal(hook)
		status := "disabled"
		if hook.Enabled {
			status = "active"
		}
		_, err := s.db.ExecContext(ctx, `INSERT INTO hook_definitions (id, event, action, status, config_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET event = excluded.event, action = excluded.action, status = excluded.status, config_json = excluded.config_json, updated_at = excluded.updated_at`,
			hook.ID, hook.Event, hook.Action, status, string(cfg), stamp, stamp,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
