package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/deon7769/deonclaw/internal/agents"
)

func (s *SQLiteStore) bootstrapTaskSnapshots(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS work_item_task_snapshots (
			work_item_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			task_json TEXT NOT NULL,
			task_sha256 TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (work_item_id) REFERENCES work_items(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_work_item_task_snapshots_sha ON work_item_task_snapshots(task_sha256)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap task snapshots: %w", err)
		}
	}
	return nil
}

func (s *SQLiteStore) SaveWorkItemTaskSnapshot(ctx context.Context, snapshot agents.WorkItemTaskSnapshot) error {
	if err := agents.ValidateWorkItemTaskSnapshot(snapshot); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO work_item_task_snapshots (work_item_id, task_id, task_json, task_sha256, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(work_item_id) DO UPDATE SET
			task_id = excluded.task_id,
			task_json = excluded.task_json,
			task_sha256 = excluded.task_sha256,
			created_at = excluded.created_at`,
		snapshot.WorkItemID, snapshot.TaskID, snapshot.TaskJSON, snapshot.TaskSHA256, snapshot.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save task snapshot %q: %w", snapshot.WorkItemID, err)
	}
	return nil
}

func (s *SQLiteStore) WorkItemTaskSnapshot(ctx context.Context, workItemID string) (agents.WorkItemTaskSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `SELECT work_item_id, task_id, task_json, task_sha256, created_at
		FROM work_item_task_snapshots WHERE work_item_id = ?`, workItemID)
	var snapshot agents.WorkItemTaskSnapshot
	if err := row.Scan(&snapshot.WorkItemID, &snapshot.TaskID, &snapshot.TaskJSON, &snapshot.TaskSHA256, &snapshot.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.WorkItemTaskSnapshot{}, fmt.Errorf("task snapshot for work item %q: %w", workItemID, ErrNotFound)
		}
		return agents.WorkItemTaskSnapshot{}, err
	}
	return snapshot, nil
}

func (s *SQLiteStore) ListWorkItemTaskSnapshotIDs(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT work_item_id FROM work_item_task_snapshots`)
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

func saveWorkItemTaskSnapshotTx(ctx context.Context, tx *sql.Tx, snapshot agents.WorkItemTaskSnapshot) error {
	if err := agents.ValidateWorkItemTaskSnapshot(snapshot); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO work_item_task_snapshots (work_item_id, task_id, task_json, task_sha256, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		snapshot.WorkItemID, snapshot.TaskID, snapshot.TaskJSON, snapshot.TaskSHA256, snapshot.CreatedAt,
	)
	return err
}
