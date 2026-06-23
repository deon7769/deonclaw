package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

func (s *SQLiteStore) bootstrapAgents(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			supervisor_id TEXT,
			default_worker TEXT NOT NULL,
			model_profile TEXT,
			config_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS agent_sessions (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			worker TEXT,
			model_profile TEXT,
			workspace_path TEXT,
			skill_snapshot_path TEXT,
			memory_snapshot_id TEXT,
			created_at TEXT NOT NULL,
			last_active_at TEXT NOT NULL,
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_sessions_agent ON agent_sessions(agent_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS work_items (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			status TEXT NOT NULL,
			assigned_agent_id TEXT,
			parent_work_item_id TEXT,
			task_id TEXT,
			priority INTEGER NOT NULL DEFAULT 50,
			config_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (assigned_agent_id) REFERENCES agents(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_work_items_agent ON work_items(assigned_agent_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS agent_inbox (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			work_item_id TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY (agent_id) REFERENCES agents(id),
			FOREIGN KEY (work_item_id) REFERENCES work_items(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_inbox_agent ON agent_inbox(agent_id, status, created_at)`,
		`CREATE TABLE IF NOT EXISTS agent_lifecycle_events (
			id TEXT PRIMARY KEY,
			agent_id TEXT NOT NULL,
			event_type TEXT NOT NULL,
			actor TEXT NOT NULL,
			payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY (agent_id) REFERENCES agents(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_lifecycle_agent ON agent_lifecycle_events(agent_id, created_at)`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap agents tables: %w", err)
		}
	}
	if err := s.ensureColumn(ctx, "runs", "agent_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "runs", "session_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "runs", "work_item_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) SaveAgent(ctx context.Context, agent agents.Agent) error {
	configJSON, err := agents.EncodeAgentConfig(agent)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO agents (
		id, display_name, role, status, supervisor_id, default_worker, model_profile, config_json, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		display_name = excluded.display_name,
		role = excluded.role,
		status = excluded.status,
		supervisor_id = excluded.supervisor_id,
		default_worker = excluded.default_worker,
		model_profile = excluded.model_profile,
		config_json = excluded.config_json,
		updated_at = excluded.updated_at`,
		agent.ID,
		agent.DisplayName,
		agent.Role,
		agent.Status,
		nullIfEmpty(agent.SupervisorID),
		agent.DefaultWorker,
		nullIfEmpty(agent.ModelProfile),
		configJSON,
		agent.CreatedAt,
		agent.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save agent %q: %w", agent.ID, err)
	}
	return nil
}

func (s *SQLiteStore) Agent(ctx context.Context, id string) (agents.Agent, error) {
	row := s.db.QueryRowContext(ctx, `SELECT config_json FROM agents WHERE id = ?`, id)
	var configJSON string
	if err := row.Scan(&configJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.Agent{}, fmt.Errorf("agent %q: %w", id, ErrNotFound)
		}
		return agents.Agent{}, fmt.Errorf("load agent %q: %w", id, err)
	}
	return decodeAgent(configJSON)
}

func (s *SQLiteStore) ListAgents(ctx context.Context) ([]agents.Agent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT config_json FROM agents ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()
	result := make([]agents.Agent, 0)
	for rows.Next() {
		var configJSON string
		if err := rows.Scan(&configJSON); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		agent, err := decodeAgent(configJSON)
		if err != nil {
			return nil, err
		}
		result = append(result, agent)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpdateAgentStatus(ctx context.Context, id string, status string, updatedAt time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE agents SET status = ?, updated_at = ? WHERE id = ?`, status, formatTime(updatedAt), id)
	if err != nil {
		return fmt.Errorf("update agent status %q: %w", id, err)
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("agent %q: %w", id, ErrNotFound)
	}
	return nil
}

func (s *SQLiteStore) SaveLifecycleEvent(ctx context.Context, event agents.LifecycleEvent) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_lifecycle_events (
		id, agent_id, event_type, actor, payload_json, created_at
	) VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		agent_id = excluded.agent_id,
		event_type = excluded.event_type,
		actor = excluded.actor,
		payload_json = excluded.payload_json,
		created_at = excluded.created_at`,
		event.ID,
		event.AgentID,
		event.EventType,
		event.Actor,
		event.Payload,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save lifecycle event %q: %w", event.ID, err)
	}
	return nil
}

func (s *SQLiteStore) SaveSession(ctx context.Context, session agents.Session) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_sessions (
		id, agent_id, kind, status, worker, model_profile, workspace_path, skill_snapshot_path, memory_snapshot_id, created_at, last_active_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		status = excluded.status,
		worker = excluded.worker,
		model_profile = excluded.model_profile,
		workspace_path = excluded.workspace_path,
		skill_snapshot_path = excluded.skill_snapshot_path,
		memory_snapshot_id = excluded.memory_snapshot_id,
		last_active_at = excluded.last_active_at`,
		session.ID,
		session.AgentID,
		session.Kind,
		session.Status,
		nullIfEmpty(session.Worker),
		nullIfEmpty(session.ModelProfile),
		nullIfEmpty(session.WorkspacePath),
		nullIfEmpty(session.SkillSnapshotPath),
		nullIfEmpty(session.MemorySnapshotID),
		session.CreatedAt,
		session.LastActiveAt,
	)
	if err != nil {
		return fmt.Errorf("save session %q: %w", session.ID, err)
	}
	return nil
}

func (s *SQLiteStore) Session(ctx context.Context, id string) (agents.Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, agent_id, kind, status, worker, model_profile, workspace_path, skill_snapshot_path, memory_snapshot_id, created_at, last_active_at
		FROM agent_sessions WHERE id = ?`, id)
	return scanSession(row)
}

func (s *SQLiteStore) ListSessionsByAgent(ctx context.Context, agentID string) ([]agents.Session, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, agent_id, kind, status, worker, model_profile, workspace_path, skill_snapshot_path, memory_snapshot_id, created_at, last_active_at
		FROM agent_sessions WHERE agent_id = ? ORDER BY created_at, id`, agentID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	result := make([]agents.Session, 0)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, session)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) SaveWorkItem(ctx context.Context, item agents.WorkItem) error {
	configJSON, err := agents.EncodeWorkItemConfig(item)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO work_items (
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
		item.ID,
		item.Title,
		item.Status,
		item.AssignedAgentID,
		nullIfEmpty(item.ParentWorkItemID),
		nullIfEmpty(item.TaskID),
		item.Priority,
		configJSON,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save work item %q: %w", item.ID, err)
	}
	return nil
}

func (s *SQLiteStore) WorkItem(ctx context.Context, id string) (agents.WorkItem, error) {
	row := s.db.QueryRowContext(ctx, `SELECT config_json FROM work_items WHERE id = ?`, id)
	var configJSON string
	if err := row.Scan(&configJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.WorkItem{}, fmt.Errorf("work item %q: %w", id, ErrNotFound)
		}
		return agents.WorkItem{}, fmt.Errorf("load work item %q: %w", id, err)
	}
	var item agents.WorkItem
	if err := json.Unmarshal([]byte(configJSON), &item); err != nil {
		return agents.WorkItem{}, fmt.Errorf("decode work item: %w", err)
	}
	return item, nil
}

func (s *SQLiteStore) SaveInboxItem(ctx context.Context, item agents.InboxItem) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO agent_inbox (
		id, agent_id, work_item_id, status, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		status = excluded.status,
		updated_at = excluded.updated_at`,
		item.ID,
		item.AgentID,
		item.WorkItemID,
		item.Status,
		item.CreatedAt,
		item.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save inbox item %q: %w", item.ID, err)
	}
	return nil
}

func (s *SQLiteStore) InboxItem(ctx context.Context, id string) (agents.InboxItem, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, agent_id, work_item_id, status, created_at, updated_at FROM agent_inbox WHERE id = ?`, id)
	var item agents.InboxItem
	if err := row.Scan(&item.ID, &item.AgentID, &item.WorkItemID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.InboxItem{}, fmt.Errorf("inbox item %q: %w", id, ErrNotFound)
		}
		return agents.InboxItem{}, fmt.Errorf("load inbox item %q: %w", id, err)
	}
	return item, nil
}

func (s *SQLiteStore) ListInboxByAgent(ctx context.Context, agentID string) ([]agents.InboxItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, agent_id, work_item_id, status, created_at, updated_at
		FROM agent_inbox WHERE agent_id = ? ORDER BY created_at, id`, agentID)
	if err != nil {
		return nil, fmt.Errorf("list inbox: %w", err)
	}
	defer rows.Close()
	result := make([]agents.InboxItem, 0)
	for rows.Next() {
		var item agents.InboxItem
		if err := rows.Scan(&item.ID, &item.AgentID, &item.WorkItemID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) ListAllInbox(ctx context.Context) ([]agents.InboxItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, agent_id, work_item_id, status, created_at, updated_at
		FROM agent_inbox ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list all inbox: %w", err)
	}
	defer rows.Close()
	result := make([]agents.InboxItem, 0)
	for rows.Next() {
		var item agents.InboxItem
		if err := rows.Scan(&item.ID, &item.AgentID, &item.WorkItemID, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *SQLiteStore) UpdateInboxItem(ctx context.Context, item agents.InboxItem) error {
	return s.SaveInboxItem(ctx, item)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSession(scanner rowScanner) (agents.Session, error) {
	var session agents.Session
	var worker, modelProfile, workspacePath, skillSnapshotPath, memorySnapshotID sql.NullString
	if err := scanner.Scan(
		&session.ID,
		&session.AgentID,
		&session.Kind,
		&session.Status,
		&worker,
		&modelProfile,
		&workspacePath,
		&skillSnapshotPath,
		&memorySnapshotID,
		&session.CreatedAt,
		&session.LastActiveAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return agents.Session{}, ErrNotFound
		}
		return agents.Session{}, err
	}
	session.Worker = worker.String
	session.ModelProfile = modelProfile.String
	session.WorkspacePath = workspacePath.String
	session.SkillSnapshotPath = skillSnapshotPath.String
	session.MemorySnapshotID = memorySnapshotID.String
	return session, nil
}

func decodeAgent(configJSON string) (agents.Agent, error) {
	var agent agents.Agent
	if err := json.Unmarshal([]byte(configJSON), &agent); err != nil {
		return agents.Agent{}, fmt.Errorf("decode agent: %w", err)
	}
	return agent, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
