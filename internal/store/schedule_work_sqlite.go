package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/schedule"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/wakeup"
	"github.com/deon7769/deonclaw/internal/workqueue"
	"github.com/deon7769/deonclaw/internal/worktemplates"
)

type MaterializeWorkFromWakeupOptions struct {
	Wakeup   wakeup.Wakeup
	Schedule schedule.Schedule
	Now      time.Time
}

type MaterializeWorkFromWakeupResult struct {
	WorkItemID string `json:"work_item_id"`
	Created    bool   `json:"created"`
}

func (s *SQLiteStore) MaterializeWorkFromWakeup(ctx context.Context, opts MaterializeWorkFromWakeupOptions) (MaterializeWorkFromWakeupResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if stringsTrim(opts.Wakeup.ID) == "" {
		return MaterializeWorkFromWakeupResult{}, fmt.Errorf("wakeup id is required")
	}
	if stringsTrim(opts.Schedule.WorkTemplateID) == "" {
		return MaterializeWorkFromWakeupResult{}, fmt.Errorf("schedule %q work_template_id is required", opts.Schedule.ID)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	w, err := wakeupTx(ctx, tx, opts.Wakeup.ID)
	if err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	if stringsTrim(w.WorkItemID) != "" {
		if err := tx.Commit(); err != nil {
			return MaterializeWorkFromWakeupResult{}, err
		}
		return MaterializeWorkFromWakeupResult{WorkItemID: w.WorkItemID, Created: false}, nil
	}

	tmpl, err := workTemplateTx(ctx, tx, opts.Schedule.WorkTemplateID)
	if err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	taskPtr, err := tasks.LoadFromFile(tmpl.TaskPath)
	if err != nil {
		return MaterializeWorkFromWakeupResult{}, fmt.Errorf("load task %q: %w", tmpl.TaskPath, err)
	}
	if err := tasks.Validate(taskPtr); err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	task := *taskPtr

	materialized, err := worktemplates.MaterializeWorkItem(worktemplates.MaterializeOptions{
		Template: tmpl,
		Task:     task,
		Wakeup:   w,
		Now:      now,
	})
	if err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	if err := saveWorkItemTx(ctx, tx, materialized.WorkItem); err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	if err := saveInboxItemTx(ctx, tx, materialized.Inbox); err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	if err := saveWorkItemTaskSnapshotTx(ctx, tx, materialized.Snapshot); err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}

	stamp := materialized.WorkItem.UpdatedAt
	events := []workqueueEventRow{
		{ID: "wqe_materialized_" + materialized.WorkItem.ID, WorkItemID: materialized.WorkItem.ID, EventType: workqueue.EventAssigned, Payload: fmt.Sprintf(`{"wakeup_id":%q}`, w.ID), CreatedAt: stamp},
	}
	if materialized.WorkItem.Status == agents.WorkItemStatusQueued {
		events = append(events,
			workqueueEventRow{ID: "wqe_queued_" + materialized.WorkItem.ID, WorkItemID: materialized.WorkItem.ID, EventType: workqueue.EventQueued, Payload: "{}", CreatedAt: stamp},
			workqueueEventRow{ID: "wqe_enqueued_" + materialized.WorkItem.ID, WorkItemID: materialized.WorkItem.ID, EventType: workqueue.EventEnqueued, Payload: fmt.Sprintf(`{"wakeup_id":%q}`, w.ID), CreatedAt: stamp},
		)
	}
	for _, event := range events {
		if err := appendWorkQueueEventTx(ctx, tx, event); err != nil {
			return MaterializeWorkFromWakeupResult{}, err
		}
	}

	w.WorkItemID = materialized.WorkItem.ID
	w.Status = wakeup.StatusEnqueued
	if err := saveWakeupTx(ctx, tx, w); err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return MaterializeWorkFromWakeupResult{}, err
	}
	return MaterializeWorkFromWakeupResult{WorkItemID: materialized.WorkItem.ID, Created: true}, nil
}

func wakeupTx(ctx context.Context, tx *sql.Tx, id string) (wakeup.Wakeup, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, schedule_id, agent_id, due_at, status, idempotency_key, attempt, run_id, config_json, created_at, claimed_at, finished_at
		FROM wakeups WHERE id = ?`, id)
	w, err := scanWakeup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return wakeup.Wakeup{}, fmt.Errorf("wakeup %q: %w", id, ErrNotFound)
	}
	return w, err
}

func workTemplateTx(ctx context.Context, tx *sql.Tx, id string) (worktemplates.Template, error) {
	row := tx.QueryRowContext(ctx, `SELECT config_json FROM work_templates WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return worktemplates.Template{}, fmt.Errorf("work template %q: %w", id, ErrNotFound)
		}
		return worktemplates.Template{}, err
	}
	var tmpl worktemplates.Template
	if err := json.Unmarshal([]byte(raw), &tmpl); err != nil {
		return worktemplates.Template{}, err
	}
	return tmpl, nil
}

func saveWakeupTx(ctx context.Context, tx *sql.Tx, w wakeup.Wakeup) error {
	data, err := json.Marshal(map[string]string{
		"skip_reason":     w.SkipReason,
		"work_item_id":    w.WorkItemID,
		"terminal_run_id": w.TerminalRunID,
	})
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE wakeups SET status = ?, run_id = ?, config_json = ?, claimed_at = ?, finished_at = ?
		WHERE id = ?`,
		w.Status, nullIfEmpty(w.RunID), string(data), nullIfEmpty(w.ClaimedAt), nullIfEmpty(w.FinishedAt), w.ID,
	)
	return err
}
