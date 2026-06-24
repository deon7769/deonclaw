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
	"github.com/deon7769/deonclaw/internal/workqueue"
)

func (s *SQLiteStore) bootstrapWorkQueue(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS execution_leases (
			id TEXT PRIMARY KEY,
			work_item_id TEXT NOT NULL,
			agent_id TEXT NOT NULL,
			run_id TEXT,
			status TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			heartbeat_at TEXT,
			config_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (work_item_id) REFERENCES work_items(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_leases_work ON execution_leases(work_item_id, status)`,
		`CREATE TABLE IF NOT EXISTS work_queue_events (
			id TEXT PRIMARY KEY,
			work_item_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (work_item_id) REFERENCES work_items(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_work_queue_events_work ON work_queue_events(work_item_id, created_at)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_execution_leases_one_active_per_work ON execution_leases(work_item_id) WHERE status = 'active'`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap work queue tables: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) ListWorkItems(ctx context.Context) ([]agents.WorkItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config_json FROM work_items ORDER BY priority DESC, created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]agents.WorkItem, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item agents.WorkItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) SaveLease(ctx context.Context, lease workqueue.Lease) error {
	data, err := json.Marshal(map[string]string{
		"release_reason": lease.ReleaseReason,
		"ttl_seconds":    fmt.Sprintf("%d", lease.TTLSeconds),
	})
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO execution_leases (
		id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		run_id = excluded.run_id,
		status = excluded.status,
		expires_at = excluded.expires_at,
		heartbeat_at = excluded.heartbeat_at,
		config_json = excluded.config_json,
		updated_at = excluded.updated_at`,
		lease.ID, lease.WorkItemID, lease.AgentID, nullIfEmpty(lease.RunID), lease.Status,
		lease.ExpiresAt, nullIfEmpty(lease.HeartbeatAt), string(data), lease.CreatedAt, lease.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save lease %q: %w", lease.ID, err)
	}
	return nil
}

func (s *SQLiteStore) ListLeases(ctx context.Context) ([]workqueue.Lease, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at
		FROM execution_leases ORDER BY created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]workqueue.Lease, 0)
	for rows.Next() {
		lease, err := scanLease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lease)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) Lease(ctx context.Context, id string) (workqueue.Lease, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at
		FROM execution_leases WHERE id = ?`, id)
	lease, err := scanLease(row)
	if errors.Is(err, sql.ErrNoRows) {
		return workqueue.Lease{}, fmt.Errorf("lease %q: %w", id, ErrNotFound)
	}
	return lease, err
}

func scanLease(scanner interface{ Scan(...any) error }) (workqueue.Lease, error) {
	var lease workqueue.Lease
	var runID, heartbeatAt sql.NullString
	var configJSON string
	if err := scanner.Scan(&lease.ID, &lease.WorkItemID, &lease.AgentID, &runID, &lease.Status,
		&lease.ExpiresAt, &heartbeatAt, &configJSON, &lease.CreatedAt, &lease.UpdatedAt); err != nil {
		return workqueue.Lease{}, err
	}
	lease.RunID = runID.String
	lease.HeartbeatAt = heartbeatAt.String
	var extra map[string]string
	_ = json.Unmarshal([]byte(configJSON), &extra)
	lease.ReleaseReason = extra["release_reason"]
	if v := extra["ttl_seconds"]; v != "" {
		var ttl int
		_, _ = fmt.Sscanf(v, "%d", &ttl)
		lease.TTLSeconds = ttl
	}
	return lease, nil
}

func (s *SQLiteStore) AppendWorkQueueEvent(ctx context.Context, event workqueue.QueueEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO work_queue_events (id, work_item_id, event_type, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		event.ID, event.WorkItemID, event.EventType, event.Payload, event.CreatedAt,
	)
	return err
}

// ClaimWorkItem acquires a lease for the next or specific queued work item atomically.
func (s *SQLiteStore) ClaimWorkItem(ctx context.Context, agentID string, workItemID string, ttl time.Duration, now time.Time) (workqueue.ClaimResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	items, err := listWorkItemsTx(ctx, tx)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	inboxRows, err := listAllInboxTx(ctx, tx)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	snapshotIDs, err := listTaskSnapshotIDsTx(ctx, tx)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}

	var candidate agents.WorkItem
	if stringsTrim(workItemID) != "" {
		for _, item := range items {
			if item.ID == workItemID {
				candidate = item
				break
			}
		}
		if candidate.ID == "" {
			return workqueue.ClaimResult{}, fmt.Errorf("work item %q: %w", workItemID, ErrNotFound)
		}
		if agentID != "" && candidate.AssignedAgentID != agentID {
			return workqueue.ClaimResult{}, fmt.Errorf("work item %q is assigned to agent %q", workItemID, candidate.AssignedAgentID)
		}
	} else {
		var ok bool
		candidate, ok = workqueue.SelectNextCandidate(items, agentID)
		if !ok {
			return workqueue.ClaimResult{Claimed: false}, nil
		}
	}
	claimAgentID := candidate.AssignedAgentID
	if agentID != "" {
		claimAgentID = agentID
	}
	agent, err := agentTx(ctx, tx, claimAgentID)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	leases, err := listLeasesTx(ctx, tx)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	autoAccept := candidate.AssignmentMode == "auto_accept"
	_, hasSnapshot := snapshotIDs[candidate.ID]
	if err := workqueue.ValidateClaim(workqueue.ClaimContext{
		Agent:            agent,
		WorkItem:         candidate,
		InboxStatus:      workqueue.InboxStatusForWork(inboxRows, candidate.ID),
		AutoAccept:       autoAccept,
		ActiveLeaseCount: workqueue.CountActiveLeasesForAgent(leases, agent.ID),
		ItemsByID:        workqueue.ItemsByID(items),
		HasTaskSnapshot:  hasSnapshot,
		Now:              now,
	}); err != nil {
		return workqueue.ClaimResult{}, err
	}
	if workqueue.HasActiveLeaseForWork(leases, candidate.ID) {
		return workqueue.ClaimResult{Claimed: false}, nil
	}

	stamp := now.UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `UPDATE work_items SET status = ?, updated_at = ?
		WHERE id = ? AND status = ?`,
		agents.WorkItemStatusLeased, stamp, candidate.ID, agents.WorkItemStatusQueued,
	)
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	if affected == 0 {
		return workqueue.ClaimResult{Claimed: false}, nil
	}

	candidate = workqueue.MarkLeased(candidate, stamp)
	if err := saveWorkItemTx(ctx, tx, candidate); err != nil {
		return workqueue.ClaimResult{}, err
	}

	lease, err := workqueue.BuildLease(workqueue.LeaseOptions{
		WorkItemID: candidate.ID,
		AgentID:    candidate.AssignedAgentID,
		TTL:        ttl,
		Now:        now,
	})
	if err != nil {
		return workqueue.ClaimResult{}, err
	}
	leaseData, _ := json.Marshal(map[string]string{"ttl_seconds": fmt.Sprintf("%d", lease.TTLSeconds)})
	if _, err := tx.ExecContext(ctx, `INSERT INTO execution_leases (
		id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		lease.ID, lease.WorkItemID, lease.AgentID, "", lease.Status, lease.ExpiresAt, lease.HeartbeatAt,
		string(leaseData), lease.CreatedAt, lease.UpdatedAt,
	); err != nil {
		return workqueue.ClaimResult{}, err
	}
	if err := appendWorkQueueEventTx(ctx, tx, workqueueEventRow{
		ID: "wqe_claimed_" + lease.ID, WorkItemID: candidate.ID, EventType: workqueue.EventClaimed,
		Payload: fmt.Sprintf(`{"lease_id":%q}`, lease.ID), CreatedAt: stamp,
	}); err != nil {
		return workqueue.ClaimResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return workqueue.ClaimResult{}, err
	}
	return workqueue.ClaimResult{Lease: lease, Claimed: true, WorkItem: candidate.ID}, nil
}

func stringsTrim(v string) string {
	return strings.TrimSpace(v)
}

// ReleaseLease releases an active lease and optionally requeues the work item.
func (s *SQLiteStore) ReleaseLease(ctx context.Context, leaseID string, reason string, requeue bool, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	lease, err := leaseTx(ctx, tx, leaseID)
	if err != nil {
		return err
	}
	if lease.Status != workqueue.LeaseStatusActive {
		return fmt.Errorf("lease %q status %q is not active", leaseID, lease.Status)
	}
	item, err := workItemTx(ctx, tx, lease.WorkItemID)
	if err != nil {
		return err
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	lease = workqueue.ReleaseLease(lease, reason, now)
	item = workqueue.MarkReleased(item, stamp, requeue)
	if err := saveLeaseTx(ctx, tx, lease); err != nil {
		return err
	}
	if err := saveWorkItemTx(ctx, tx, item); err != nil {
		return err
	}
	eventType := workqueue.EventReleased
	if item.Status == agents.WorkItemStatusDeadLetter {
		eventType = workqueue.EventDeadLettered
	}
	if err := appendWorkQueueEventTx(ctx, tx, workqueueEventRow{
		ID: "wqe_release_" + lease.ID, WorkItemID: item.ID, EventType: eventType,
		Payload: fmt.Sprintf(`{"lease_id":%q,"reason":%q,"requeue":%t}`, lease.ID, reason, requeue), CreatedAt: stamp,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// RecoverWorkQueue marks expired active leases as lost and requeues or dead-letters work items.
func (s *SQLiteStore) RecoverWorkQueue(ctx context.Context, now time.Time) (workqueue.RecoveryReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workqueue.RecoveryReport{}, err
	}
	defer func() { _ = tx.Rollback() }()

	leases, err := listLeasesTx(ctx, tx)
	if err != nil {
		return workqueue.RecoveryReport{}, err
	}
	updated, report := workqueue.RecoverStaleLeases(leases, now)
	stamp := now.UTC().Format(time.RFC3339Nano)
	for _, lease := range updated {
		original := findLease(leases, lease.ID)
		if original == nil || original.Status == lease.Status {
			continue
		}
		if err := saveLeaseTx(ctx, tx, lease); err != nil {
			return workqueue.RecoveryReport{}, err
		}
		item, err := workItemTx(ctx, tx, lease.WorkItemID)
		if err != nil {
			return workqueue.RecoveryReport{}, err
		}
		requeue := !workqueue.ShouldDeadLetterOnRecovery(item)
		if requeue {
			item = workqueue.MarkReleased(item, stamp, true)
		} else {
			item = workqueue.MarkDeadLetter(item, stamp)
		}
		if err := saveWorkItemTx(ctx, tx, item); err != nil {
			return workqueue.RecoveryReport{}, err
		}
		if err := appendWorkQueueEventTx(ctx, tx, workqueueEventRow{
			ID: "wqe_recover_" + lease.ID, WorkItemID: item.ID, EventType: workqueue.EventRecovered,
			Payload: fmt.Sprintf(`{"lease_id":%q,"dead_letter":%t}`, lease.ID, !requeue), CreatedAt: stamp,
		}); err != nil {
			return workqueue.RecoveryReport{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return workqueue.RecoveryReport{}, err
	}
	return report, nil
}

func (s *SQLiteStore) RenewLease(ctx context.Context, leaseID string, ttl time.Duration, now time.Time) (workqueue.Lease, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return workqueue.Lease{}, err
	}
	defer func() { _ = tx.Rollback() }()

	lease, err := leaseTx(ctx, tx, leaseID)
	if err != nil {
		return workqueue.Lease{}, err
	}
	renewed, err := workqueue.RenewLease(lease, ttl, now)
	if err != nil {
		return workqueue.Lease{}, err
	}
	if err := saveLeaseTx(ctx, tx, renewed); err != nil {
		return workqueue.Lease{}, err
	}
	if err := appendWorkQueueEventTx(ctx, tx, workqueueEventRow{
		ID: "wqe_renew_" + renewed.ID, WorkItemID: renewed.WorkItemID, EventType: workqueue.EventLeaseRenewed,
		Payload: fmt.Sprintf(`{"lease_id":%q,"expires_at":%q}`, renewed.ID, renewed.ExpiresAt), CreatedAt: renewed.UpdatedAt,
	}); err != nil {
		return workqueue.Lease{}, err
	}
	if err := tx.Commit(); err != nil {
		return workqueue.Lease{}, err
	}
	return renewed, nil
}

func (s *SQLiteStore) WorkDoctor(ctx context.Context, now time.Time) (workqueue.DoctorReport, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	items, err := s.ListWorkItems(ctx)
	if err != nil {
		return workqueue.DoctorReport{}, err
	}
	leases, err := s.ListLeases(ctx)
	if err != nil {
		return workqueue.DoctorReport{}, err
	}
	inbox, err := s.ListAllInbox(ctx)
	if err != nil {
		return workqueue.DoctorReport{}, err
	}
	snapshotIDs, err := s.ListWorkItemTaskSnapshotIDs(ctx)
	if err != nil {
		return workqueue.DoctorReport{}, err
	}
	report := workqueue.Diagnose(workqueue.DoctorInput{
		Items: items, Leases: leases, Inbox: inbox, SnapshotWorkIDs: snapshotIDs, Now: now,
	})
	agentsByID := map[string]agents.Agent{}
	for _, item := range items {
		if item.AssignedAgentID == "" {
			continue
		}
		if _, ok := agentsByID[item.AssignedAgentID]; ok {
			continue
		}
		agent, err := s.Agent(ctx, item.AssignedAgentID)
		if err != nil {
			report.Findings = append(report.Findings, workqueue.DoctorFinding{
				Severity: "error", Code: "assigned_agent_missing", Message: err.Error(), WorkItemID: item.ID,
			})
			continue
		}
		agentsByID[item.AssignedAgentID] = agent
	}
	for _, item := range items {
		if item.Status != agents.WorkItemStatusQueued && item.Status != agents.WorkItemStatusLeased {
			continue
		}
		agent, ok := agentsByID[item.AssignedAgentID]
		if !ok {
			continue
		}
		if err := agents.CanStartRun(agent); err != nil {
			report.Findings = append(report.Findings, workqueue.DoctorFinding{
				Severity: "error", Code: "assigned_agent_not_runnable", Message: err.Error(), WorkItemID: item.ID,
			})
		}
	}
	if report.Status == "ok" {
		for _, finding := range report.Findings {
			if finding.Severity == "error" {
				report.Status = "failed"
				break
			}
			if finding.Severity == "warning" {
				report.Status = "warning"
			}
		}
	}
	return report, nil
}

func findLease(leases []workqueue.Lease, id string) *workqueue.Lease {
	for i := range leases {
		if leases[i].ID == id {
			return &leases[i]
		}
	}
	return nil
}

func listWorkItemsTx(ctx context.Context, tx *sql.Tx) ([]agents.WorkItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT config_json FROM work_items ORDER BY priority DESC, created_at, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]agents.WorkItem, 0)
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var item agents.WorkItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func listLeasesTx(ctx context.Context, tx *sql.Tx) ([]workqueue.Lease, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at FROM execution_leases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]workqueue.Lease, 0)
	for rows.Next() {
		lease, err := scanLease(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, lease)
	}
	return out, rows.Err()
}

func listAllInboxTx(ctx context.Context, tx *sql.Tx) ([]agents.InboxItem, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, agent_id, work_item_id, status, created_at, updated_at FROM agent_inbox`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]agents.InboxItem, 0)
	for rows.Next() {
		var item agents.InboxItem
		if err := rows.Scan(&item.ID, &item.AgentID, &item.WorkItemID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func listTaskSnapshotIDsTx(ctx context.Context, tx *sql.Tx) (map[string]struct{}, error) {
	rows, err := tx.QueryContext(ctx, `SELECT work_item_id FROM work_item_task_snapshots`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = struct{}{}
	}
	return out, rows.Err()
}

func agentTx(ctx context.Context, tx *sql.Tx, id string) (agents.Agent, error) {
	row := tx.QueryRowContext(ctx, `SELECT config_json FROM agents WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.Agent{}, fmt.Errorf("agent %q: %w", id, ErrNotFound)
		}
		return agents.Agent{}, err
	}
	var agent agents.Agent
	if err := json.Unmarshal([]byte(raw), &agent); err != nil {
		return agents.Agent{}, err
	}
	return agent, nil
}

func leaseTx(ctx context.Context, tx *sql.Tx, id string) (workqueue.Lease, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at
		FROM execution_leases WHERE id = ?`, id)
	lease, err := scanLease(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return workqueue.Lease{}, fmt.Errorf("lease %q: %w", id, ErrNotFound)
		}
		return workqueue.Lease{}, err
	}
	return lease, nil
}

func saveLeaseTx(ctx context.Context, tx *sql.Tx, lease workqueue.Lease) error {
	data, err := json.Marshal(map[string]string{
		"release_reason": lease.ReleaseReason,
		"ttl_seconds":    fmt.Sprintf("%d", lease.TTLSeconds),
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO execution_leases (
		id, work_item_id, agent_id, run_id, status, expires_at, heartbeat_at, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		run_id = excluded.run_id,
		status = excluded.status,
		expires_at = excluded.expires_at,
		heartbeat_at = excluded.heartbeat_at,
		config_json = excluded.config_json,
		updated_at = excluded.updated_at`,
		lease.ID, lease.WorkItemID, lease.AgentID, nullIfEmpty(lease.RunID), lease.Status,
		lease.ExpiresAt, nullIfEmpty(lease.HeartbeatAt), string(data), lease.CreatedAt, lease.UpdatedAt,
	)
	return err
}
