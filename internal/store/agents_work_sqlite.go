package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

type AssignWorkItemWithSnapshotOptions struct {
	Agent     agents.Agent
	Task      tasks.Task
	CreatedBy string
	Now       time.Time
}

func (s *SQLiteStore) AssignWorkItemWithSnapshot(ctx context.Context, opts AssignWorkItemWithSnapshotOptions) (agents.AssignTaskResult, agents.WorkItemTaskSnapshot, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	workItem, err := agents.WorkItemFromTask(agents.WorkItemFromTaskOptions{
		Task:            opts.Task,
		AssignedAgentID: opts.Agent.ID,
		CreatedBy:       opts.CreatedBy,
		Now:             now,
	})
	if err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	agents.ApplyWorkItemDefaults(&workItem, opts.Agent)
	result, err := agents.AssignWorkItem(agents.AssignTaskOptions{Agent: opts.Agent, WorkItem: workItem, CreatedBy: opts.CreatedBy, Now: now})
	if err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	snapshot, err := agents.BuildWorkItemTaskSnapshot(result.WorkItem.ID, opts.Task, result.WorkItem.CreatedAt)
	if err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	result.WorkItem.TaskSnapshotSHA256 = snapshot.TaskSHA256

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := saveWorkItemTx(ctx, tx, result.WorkItem); err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	if err := saveInboxItemTx(ctx, tx, result.InboxItem); err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	if err := saveWorkItemTaskSnapshotTx(ctx, tx, snapshot); err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	stamp := result.WorkItem.UpdatedAt
	if err := appendWorkQueueEventTx(ctx, tx, workqueueEventRow{
		ID: "wqe_assigned_" + result.WorkItem.ID, WorkItemID: result.WorkItem.ID,
		EventType: workqueue.EventAssigned, Payload: fmt.Sprintf(`{"inbox_id":%q}`, result.InboxItem.ID), CreatedAt: stamp,
	}); err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return agents.AssignTaskResult{}, agents.WorkItemTaskSnapshot{}, err
	}
	return result, snapshot, nil
}

func (s *SQLiteStore) AcceptInboxAndQueueWork(ctx context.Context, inboxItemID string, now time.Time) (agents.InboxItem, agents.WorkItem, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	defer func() { _ = tx.Rollback() }()

	inbox, err := inboxItemTx(ctx, tx, inboxItemID)
	if err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	workItem, err := workItemTx(ctx, tx, inbox.WorkItemID)
	if err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	acceptedInbox, queuedWork, err := agents.AcceptInboxAndQueueWork(inbox, workItem, now)
	if err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	if err := saveInboxItemTx(ctx, tx, acceptedInbox); err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	if err := saveWorkItemTx(ctx, tx, queuedWork); err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	stamp := queuedWork.UpdatedAt
	for _, evt := range []workqueueEventRow{
		{ID: "wqe_accepted_" + acceptedInbox.ID, WorkItemID: queuedWork.ID, EventType: workqueue.EventAccepted, Payload: fmt.Sprintf(`{"inbox_id":%q}`, acceptedInbox.ID), CreatedAt: stamp},
		{ID: "wqe_queued_" + queuedWork.ID, WorkItemID: queuedWork.ID, EventType: workqueue.EventQueued, Payload: "{}", CreatedAt: stamp},
	} {
		if err := appendWorkQueueEventTx(ctx, tx, evt); err != nil {
			return agents.InboxItem{}, agents.WorkItem{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return agents.InboxItem{}, agents.WorkItem{}, err
	}
	return acceptedInbox, queuedWork, nil
}

func saveWorkItemTx(ctx context.Context, tx *sql.Tx, item agents.WorkItem) error {
	configJSON, err := agents.EncodeWorkItemConfig(item)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO work_items (
		id, title, status, assigned_agent_id, parent_work_item_id, task_id, priority, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		status = excluded.status,
		assigned_agent_id = excluded.assigned_agent_id,
		parent_work_item_id = excluded.parent_work_item_id,
		task_id = excluded.task_id,
		priority = excluded.priority,
		config_json = excluded.config_json,
		updated_at = excluded.updated_at`,
		item.ID, item.Title, item.Status, nullIfEmpty(item.AssignedAgentID), nullIfEmpty(item.ParentWorkItemID),
		nullIfEmpty(item.TaskID), item.Priority, configJSON, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func saveInboxItemTx(ctx context.Context, tx *sql.Tx, item agents.InboxItem) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO agent_inbox (id, agent_id, work_item_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET status = excluded.status, updated_at = excluded.updated_at`,
		item.ID, item.AgentID, item.WorkItemID, item.Status, item.CreatedAt, item.UpdatedAt,
	)
	return err
}

func inboxItemTx(ctx context.Context, tx *sql.Tx, id string) (agents.InboxItem, error) {
	row := tx.QueryRowContext(ctx, `SELECT id, agent_id, work_item_id, status, created_at, updated_at FROM agent_inbox WHERE id = ?`, id)
	var item agents.InboxItem
	if err := row.Scan(&item.ID, &item.AgentID, &item.WorkItemID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return agents.InboxItem{}, err
	}
	return item, nil
}

func workItemTx(ctx context.Context, tx *sql.Tx, id string) (agents.WorkItem, error) {
	row := tx.QueryRowContext(ctx, `SELECT config_json FROM work_items WHERE id = ?`, id)
	var raw string
	if err := row.Scan(&raw); err != nil {
		return agents.WorkItem{}, err
	}
	var item agents.WorkItem
	if err := json.Unmarshal([]byte(raw), &item); err != nil {
		return agents.WorkItem{}, err
	}
	return item, nil
}

func appendWorkQueueEventTx(ctx context.Context, tx *sql.Tx, event workqueueEventRow) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO work_queue_events (id, work_item_id, event_type, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		event.ID, event.WorkItemID, event.EventType, event.Payload, event.CreatedAt,
	)
	return err
}

type workqueueEventRow struct {
	ID         string
	WorkItemID string
	EventType  string
	Payload    string
	CreatedAt  string
}
