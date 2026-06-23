package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/usage"
)

const budgetPolicyStatusActive = "active"

func (s *SQLiteStore) bootstrapBudget(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS budget_policies (
			id TEXT PRIMARY KEY,
			scope TEXT NOT NULL,
			config_json TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS budget_windows (
			id TEXT PRIMARY KEY,
			policy_id TEXT NOT NULL,
			period_start TEXT NOT NULL,
			period_end TEXT NOT NULL,
			hard_limit_microusd INTEGER NOT NULL,
			reserved_microusd INTEGER NOT NULL DEFAULT 0,
			committed_microusd INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			config_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (policy_id) REFERENCES budget_policies(id)
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_budget_windows_policy_period ON budget_windows(policy_id, period_start)`,
		`CREATE TABLE IF NOT EXISTS budget_reservations (
			id TEXT PRIMARY KEY,
			idempotency_key TEXT NOT NULL UNIQUE,
			policy_id TEXT NOT NULL,
			window_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			run_id TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			estimated_microusd INTEGER NOT NULL,
			committed_microusd INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL,
			config_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (window_id) REFERENCES budget_windows(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_budget_reservations_work ON budget_reservations(work_item_id, status)`,
		`CREATE TABLE IF NOT EXISTS budget_override_approvals (
			id TEXT PRIMARY KEY,
			policy_id TEXT NOT NULL,
			approval_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap budget tables: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SyncBudgetPolicies(ctx context.Context, policies []budget.Policy) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	now := formatTime(time.Now().UTC())
	for _, policy := range policies {
		data, err := json.Marshal(policy)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO budget_policies (id, scope, config_json, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET scope = excluded.scope, config_json = excluded.config_json, status = excluded.status, updated_at = excluded.updated_at`,
			policy.ID, policy.Scope, string(data), budgetPolicyStatusActive, now, now,
		)
		if err != nil {
			return fmt.Errorf("sync budget policy %q: %w", policy.ID, err)
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) BudgetPolicy(ctx context.Context, id string) (budget.Policy, error) {
	row := s.db.QueryRowContext(ctx, `SELECT config_json FROM budget_policies WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return budget.Policy{}, fmt.Errorf("budget policy %q: %w", id, ErrNotFound)
		}
		return budget.Policy{}, err
	}
	var policy budget.Policy
	if err := json.Unmarshal([]byte(raw), &policy); err != nil {
		return budget.Policy{}, err
	}
	return policy, nil
}

func (s *SQLiteStore) ListBudgetWindows(ctx context.Context) ([]budget.Window, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, policy_id, period_start, period_end, hard_limit_microusd, reserved_microusd, committed_microusd, status
		FROM budget_windows ORDER BY period_start, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]budget.Window, 0)
	for rows.Next() {
		var window budget.Window
		if err := rows.Scan(&window.ID, &window.PolicyID, &window.PeriodStart, &window.PeriodEnd,
			&window.HardLimitMicroUSD, &window.ReservedMicroUSD, &window.CommittedMicroUSD, &window.Status); err != nil {
			return nil, err
		}
		out = append(out, window)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) GetOrCreateWindow(ctx context.Context, policy budget.Policy, now time.Time) (budget.Window, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return budget.Window{}, err
	}
	defer func() { _ = tx.Rollback() }()

	window, err := budget.BuildWindow(policy, now)
	if err != nil {
		return budget.Window{}, err
	}
	stamp := formatTime(now.UTC())
	_, err = tx.ExecContext(ctx, `INSERT INTO budget_windows (
		id, policy_id, period_start, period_end, hard_limit_microusd, reserved_microusd, committed_microusd, status, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, 0, 0, ?, '{}', ?, ?)
	ON CONFLICT(id) DO NOTHING`,
		window.ID, window.PolicyID, window.PeriodStart, window.PeriodEnd, window.HardLimitMicroUSD, window.Status, stamp, stamp,
	)
	if err != nil {
		return budget.Window{}, err
	}
	row := tx.QueryRowContext(ctx, `SELECT id, policy_id, period_start, period_end, hard_limit_microusd, reserved_microusd, committed_microusd, status
		FROM budget_windows WHERE id = ?`, window.ID)
	if err := row.Scan(&window.ID, &window.PolicyID, &window.PeriodStart, &window.PeriodEnd,
		&window.HardLimitMicroUSD, &window.ReservedMicroUSD, &window.CommittedMicroUSD, &window.Status); err != nil {
		return budget.Window{}, err
	}
	window.Timezone = policy.Timezone
	if err := tx.Commit(); err != nil {
		return budget.Window{}, err
	}
	return window, nil
}

func (s *SQLiteStore) ReserveBudget(ctx context.Context, opts budget.ReserveBudgetOptions) (budget.ReserveBudgetResult, error) {
	if err := budget.ValidateReserveOptions(opts); err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	idempotencyKey := strings.TrimSpace(opts.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = budget.ReservationIdempotencyKey(opts.WorkItemID, opts.RunID)
	}

	if existing, err := s.budgetReservationByIdempotency(ctx, idempotencyKey); err == nil {
		return budget.ReserveBudgetResult{Reservation: existing, Created: false}, nil
	} else if !errors.Is(err, ErrNotFound) {
		return budget.ReserveBudgetResult{}, err
	}

	policy, err := s.BudgetPolicy(ctx, opts.PolicyID)
	if err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	window, err := s.GetOrCreateWindow(ctx, policy, now)
	if err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	if err := budget.CanReserve(window, policy, opts.EstimatedMicroUSD); err != nil {
		return budget.ReserveBudgetResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `UPDATE budget_windows
		SET reserved_microusd = reserved_microusd + ?, updated_at = ?
		WHERE id = ? AND committed_microusd + reserved_microusd + ? <= hard_limit_microusd`,
		opts.EstimatedMicroUSD, stamp, window.ID, opts.EstimatedMicroUSD,
	)
	if err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	if affected == 0 {
		return budget.ReserveBudgetResult{}, fmt.Errorf("budget exhausted")
	}

	reservation := budget.Reservation{
		ID:                budget.NewReservationID(opts.WorkItemID, now),
		IdempotencyKey:    idempotencyKey,
		PolicyID:          opts.PolicyID,
		WindowID:          window.ID,
		WorkItemID:        opts.WorkItemID,
		RunID:             opts.RunID,
		AgentID:           opts.AgentID,
		EstimatedMicroUSD: opts.EstimatedMicroUSD,
		Status:            budget.ReservationReserved,
		CreatedAt:         stamp,
		UpdatedAt:         stamp,
	}
	if err := saveReservationTx(ctx, tx, reservation); err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return budget.ReserveBudgetResult{}, err
	}
	return budget.ReserveBudgetResult{Reservation: reservation, Created: true}, nil
}

func (s *SQLiteStore) CommitReservation(ctx context.Context, reservationID string, event usage.Event, actualMicroUSD int64, now time.Time) (budget.Reservation, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return budget.Reservation{}, err
	}
	defer func() { _ = tx.Rollback() }()

	reservation, err := reservationTx(ctx, tx, reservationID)
	if err != nil {
		return budget.Reservation{}, err
	}
	if reservation.Status == budget.ReservationCommitted {
		if err := tx.Commit(); err != nil {
			return budget.Reservation{}, err
		}
		return reservation, nil
	}
	updated, releaseReserved, commitAmount, err := budget.CommitAmount(reservation, actualMicroUSD)
	if err != nil {
		return budget.Reservation{}, err
	}
	_ = event
	result, err := tx.ExecContext(ctx, `UPDATE budget_windows
		SET reserved_microusd = reserved_microusd - ?, committed_microusd = committed_microusd + ?, updated_at = ?
		WHERE id = ? AND reserved_microusd >= ?`,
		releaseReserved, commitAmount, stamp, reservation.WindowID, releaseReserved,
	)
	if err != nil {
		return budget.Reservation{}, err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return budget.Reservation{}, fmt.Errorf("budget window update failed for reservation %q", reservationID)
	}
	updated.UpdatedAt = stamp
	if err := saveReservationTx(ctx, tx, updated); err != nil {
		return budget.Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return budget.Reservation{}, err
	}
	return updated, nil
}

func (s *SQLiteStore) ReleaseReservation(ctx context.Context, reservationID string, reason string, now time.Time) (budget.Reservation, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return budget.Reservation{}, err
	}
	defer func() { _ = tx.Rollback() }()

	reservation, err := reservationTx(ctx, tx, reservationID)
	if err != nil {
		return budget.Reservation{}, err
	}
	if reservation.Status == budget.ReservationReleased || reservation.Status == budget.ReservationCommitted {
		if err := tx.Commit(); err != nil {
			return budget.Reservation{}, err
		}
		return reservation, nil
	}
	amount := budget.ReleaseAmount(reservation)
	if amount > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE budget_windows
			SET reserved_microusd = reserved_microusd - ?, updated_at = ?
			WHERE id = ? AND reserved_microusd >= ?`,
			amount, stamp, reservation.WindowID, amount,
		); err != nil {
			return budget.Reservation{}, err
		}
	}
	reservation.Status = budget.ReservationReleased
	reservation.UpdatedAt = stamp
	configJSON, _ := json.Marshal(map[string]string{"release_reason": budget.ReservationReleaseReason(reason)})
	if _, err := tx.ExecContext(ctx, `UPDATE budget_reservations SET status = ?, updated_at = ?, config_json = ? WHERE id = ?`,
		reservation.Status, reservation.UpdatedAt, string(configJSON), reservation.ID,
	); err != nil {
		return budget.Reservation{}, err
	}
	if err := tx.Commit(); err != nil {
		return budget.Reservation{}, err
	}
	return reservation, nil
}

func (s *SQLiteStore) BudgetReservation(ctx context.Context, id string) (budget.Reservation, error) {
	return reservationByID(ctx, s.db, id)
}

func (s *SQLiteStore) BudgetStatus(ctx context.Context, agentID string) (budget.StatusReport, error) {
	windows, err := s.ListBudgetWindows(ctx)
	if err != nil {
		return budget.StatusReport{}, err
	}
	policies := map[string]budget.Policy{}
	rows, err := s.db.QueryContext(ctx, `SELECT id, config_json FROM budget_policies`)
	if err != nil {
		return budget.StatusReport{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, raw string
		if err := rows.Scan(&id, &raw); err != nil {
			return budget.StatusReport{}, err
		}
		var policy budget.Policy
		if err := json.Unmarshal([]byte(raw), &policy); err != nil {
			return budget.StatusReport{}, err
		}
		policy.ID = id
		policies[id] = policy
	}
	if err := rows.Err(); err != nil {
		return budget.StatusReport{}, err
	}
	return budget.BuildStatusReport(agentID, windows, policies), nil
}

func (s *SQLiteStore) SaveBudgetOverrideApproval(ctx context.Context, approval budget.OverrideApproval) error {
	if err := budget.ValidateOverrideApproval(approval); err != nil {
		return err
	}
	data, err := json.Marshal(approval)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO budget_override_approvals (id, policy_id, approval_json, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET approval_json = excluded.approval_json`,
		approval.ApprovalID, approval.PolicyID, string(data), approval.CreatedAt,
	)
	return err
}

func (s *SQLiteStore) BudgetOverrideApproval(ctx context.Context, id string) (budget.OverrideApproval, error) {
	row := s.db.QueryRowContext(ctx, `SELECT approval_json FROM budget_override_approvals WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return budget.OverrideApproval{}, fmt.Errorf("budget override approval %q: %w", id, ErrNotFound)
		}
		return budget.OverrideApproval{}, err
	}
	var approval budget.OverrideApproval
	if err := json.Unmarshal([]byte(raw), &approval); err != nil {
		return budget.OverrideApproval{}, err
	}
	return approval, nil
}

func (s *SQLiteStore) ApplyBudgetExhausted(ctx context.Context, agent agents.Agent, policy budget.Policy, now time.Time) (agents.Agent, string, error) {
	updated, eventType, err := budget.ApplyOnExhausted(agent, policy, now)
	if err != nil {
		return agent, "", err
	}
	if err := s.SaveAgent(ctx, updated); err != nil {
		return agent, "", err
	}
	return updated, eventType, nil
}

func reservationByID(ctx context.Context, db queryer, id string) (budget.Reservation, error) {
	row := db.QueryRowContext(ctx, `SELECT id, idempotency_key, policy_id, window_id, work_item_id, run_id, agent_id,
		estimated_microusd, committed_microusd, status, created_at, updated_at FROM budget_reservations WHERE id = ?`, id)
	return scanReservation(row)
}

func (s *SQLiteStore) budgetReservationByIdempotency(ctx context.Context, key string) (budget.Reservation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, idempotency_key, policy_id, window_id, work_item_id, run_id, agent_id,
		estimated_microusd, committed_microusd, status, created_at, updated_at FROM budget_reservations WHERE idempotency_key = ?`, key)
	reservation, err := scanReservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return budget.Reservation{}, ErrNotFound
	}
	return reservation, err
}

func reservationByIdempotencyTx(ctx context.Context, tx *sql.Tx, key string) (budget.Reservation, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, idempotency_key, policy_id, window_id, work_item_id, run_id, agent_id,
		estimated_microusd, committed_microusd, status, created_at, updated_at FROM budget_reservations WHERE idempotency_key = ?`, key)
	reservation, err := scanReservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return budget.Reservation{}, ErrNotFound
	}
	return reservation, err
}

func reservationTx(ctx context.Context, tx *sql.Tx, id string) (budget.Reservation, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, idempotency_key, policy_id, window_id, work_item_id, run_id, agent_id,
		estimated_microusd, committed_microusd, status, created_at, updated_at FROM budget_reservations WHERE id = ?`, id)
	reservation, err := scanReservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return budget.Reservation{}, fmt.Errorf("budget reservation %q: %w", id, ErrNotFound)
	}
	return reservation, err
}

func scanReservation(scanner interface{ Scan(...any) error }) (budget.Reservation, error) {
	var reservation budget.Reservation
	if err := scanner.Scan(&reservation.ID, &reservation.IdempotencyKey, &reservation.PolicyID, &reservation.WindowID,
		&reservation.WorkItemID, &reservation.RunID, &reservation.AgentID, &reservation.EstimatedMicroUSD,
		&reservation.CommittedMicroUSD, &reservation.Status, &reservation.CreatedAt, &reservation.UpdatedAt); err != nil {
		return budget.Reservation{}, err
	}
	return reservation, nil
}

func saveReservationTx(ctx context.Context, tx *sql.Tx, reservation budget.Reservation) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO budget_reservations (
		id, idempotency_key, policy_id, window_id, work_item_id, run_id, agent_id,
		estimated_microusd, committed_microusd, status, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '{}', ?, ?)
	ON CONFLICT(idempotency_key) DO NOTHING`,
		reservation.ID, reservation.IdempotencyKey, reservation.PolicyID, reservation.WindowID, reservation.WorkItemID,
		nullIfEmpty(reservation.RunID), nullIfEmpty(reservation.AgentID), reservation.EstimatedMicroUSD,
		reservation.CommittedMicroUSD, reservation.Status, reservation.CreatedAt, reservation.UpdatedAt,
	)
	if err != nil {
		return err
	}
	row := tx.QueryRowContext(ctx, `SELECT id, idempotency_key, policy_id, window_id, work_item_id, run_id, agent_id,
		estimated_microusd, committed_microusd, status, created_at, updated_at FROM budget_reservations WHERE idempotency_key = ?`,
		reservation.IdempotencyKey,
	)
	existing, err := scanReservation(row)
	if err != nil {
		return err
	}
	reservation.ID = existing.ID
	return nil
}

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
