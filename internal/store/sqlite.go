package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

const currentSchemaVersion = 3

func OpenSQLite(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite store: %w", err)
	}
	db.SetMaxOpenConns(1)

	store := &SQLiteStore{db: db}
	if err := store.bootstrap(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) bootstrap(ctx context.Context) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS tasks (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			domain TEXT NOT NULL,
			worker TEXT NOT NULL,
			goal TEXT NOT NULL,
			mode TEXT NOT NULL,
			workspace_strategy TEXT NOT NULL,
			workspace_path TEXT NOT NULL,
			memory_scope TEXT NOT NULL,
			validation_commands TEXT NOT NULL DEFAULT 'null',
			allowed_paths TEXT NOT NULL,
			forbidden_paths TEXT NOT NULL,
			expected_outputs TEXT NOT NULL,
			definition_of_done TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS runs (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			worker TEXT NOT NULL,
			workspace_path TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT,
			FOREIGN KEY (task_id) REFERENCES tasks(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_runs_task_id ON runs(task_id)`,
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			type TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			payload TEXT,
			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_run_timestamp ON events(run_id, timestamp, id)`,
		`CREATE TABLE IF NOT EXISTS artifacts (
			id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			path TEXT NOT NULL,
			kind TEXT NOT NULL,
			size_bytes INTEGER NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			keep INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			FOREIGN KEY (run_id) REFERENCES runs(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_artifacts_run_created ON artifacts(run_id, created_at, id)`,
	}

	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("bootstrap sqlite store: %w", err)
		}
	}
	if err := s.ensureColumn(ctx, "tasks", "validation_commands", "TEXT NOT NULL DEFAULT 'null'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "artifacts", "size_bytes", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "artifacts", "sha256", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "artifacts", "keep", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.bootstrapAgents(ctx); err != nil {
		return err
	}
	if err := s.bootstrapProactive(ctx); err != nil {
		return err
	}
	if err := s.recordSchemaMigration(ctx, currentSchemaVersion); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) recordSchemaMigration(ctx context.Context, version int) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, formatTime(time.Now().UTC()))
	if err != nil {
		return fmt.Errorf("record schema migration %d: %w", version, err)
	}
	return nil
}

func (s *SQLiteStore) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect sqlite table %s: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan sqlite table %s info: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect sqlite table %s: %w", table, err)
	}

	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("migrate sqlite table %s column %s: %w", table, column, err)
	}
	return nil
}

func (s *SQLiteStore) SaveTask(ctx context.Context, task *tasks.Task) error {
	allowedPaths, err := encodeStrings(task.AllowedPaths)
	if err != nil {
		return err
	}
	forbiddenPaths, err := encodeStrings(task.ForbiddenPaths)
	if err != nil {
		return err
	}
	expectedOutputs, err := encodeStrings(task.ExpectedOutputs)
	if err != nil {
		return err
	}
	definitionOfDone, err := encodeStrings(task.DefinitionOfDone)
	if err != nil {
		return err
	}
	validationCommands, err := encodeValidationCommands(task.Validation.Commands)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `INSERT INTO tasks (
		id, title, domain, worker, goal, mode,
		workspace_strategy, workspace_path, memory_scope,
		validation_commands, allowed_paths, forbidden_paths, expected_outputs, definition_of_done
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		domain = excluded.domain,
		worker = excluded.worker,
		goal = excluded.goal,
		mode = excluded.mode,
		workspace_strategy = excluded.workspace_strategy,
		workspace_path = excluded.workspace_path,
		memory_scope = excluded.memory_scope,
		validation_commands = excluded.validation_commands,
		allowed_paths = excluded.allowed_paths,
		forbidden_paths = excluded.forbidden_paths,
		expected_outputs = excluded.expected_outputs,
		definition_of_done = excluded.definition_of_done`,
		task.ID,
		task.Title,
		task.Domain,
		task.Worker,
		task.Goal,
		task.Mode,
		task.Workspace.Strategy,
		task.Workspace.Path,
		task.Memory.Scope,
		validationCommands,
		allowedPaths,
		forbiddenPaths,
		expectedOutputs,
		definitionOfDone,
	)
	if err != nil {
		return fmt.Errorf("save task %q: %w", task.ID, err)
	}
	return nil
}

func (s *SQLiteStore) Task(ctx context.Context, id string) (*tasks.Task, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, title, domain, worker, goal, mode,
		workspace_strategy, workspace_path, memory_scope,
		validation_commands, allowed_paths, forbidden_paths, expected_outputs, definition_of_done
		FROM tasks WHERE id = ?`, id)

	var task tasks.Task
	var validationCommands, allowedPaths, forbiddenPaths, expectedOutputs, definitionOfDone string
	err := row.Scan(
		&task.ID,
		&task.Title,
		&task.Domain,
		&task.Worker,
		&task.Goal,
		&task.Mode,
		&task.Workspace.Strategy,
		&task.Workspace.Path,
		&task.Memory.Scope,
		&validationCommands,
		&allowedPaths,
		&forbiddenPaths,
		&expectedOutputs,
		&definitionOfDone,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("task %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("load task %q: %w", id, err)
	}

	if err := decodeStrings(allowedPaths, &task.AllowedPaths); err != nil {
		return nil, err
	}
	if err := decodeValidationCommands(validationCommands, &task.Validation.Commands); err != nil {
		return nil, err
	}
	if err := decodeStrings(forbiddenPaths, &task.ForbiddenPaths); err != nil {
		return nil, err
	}
	if err := decodeStrings(expectedOutputs, &task.ExpectedOutputs); err != nil {
		return nil, err
	}
	if err := decodeStrings(definitionOfDone, &task.DefinitionOfDone); err != nil {
		return nil, err
	}
	return &task, nil
}

func (s *SQLiteStore) SaveRun(ctx context.Context, run *runs.Run) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runs (
		id, task_id, status, worker, workspace_path, agent_id, session_id, work_item_id, created_at, updated_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		task_id = excluded.task_id,
		status = excluded.status,
		worker = excluded.worker,
		workspace_path = excluded.workspace_path,
		agent_id = excluded.agent_id,
		session_id = excluded.session_id,
		work_item_id = excluded.work_item_id,
		created_at = excluded.created_at,
		updated_at = excluded.updated_at,
		finished_at = excluded.finished_at`,
		run.ID,
		run.TaskID,
		string(run.Status),
		run.Worker,
		run.WorkspacePath,
		run.AgentID,
		run.SessionID,
		run.WorkItemID,
		formatTime(run.CreatedAt),
		formatTime(run.UpdatedAt),
		formatOptionalTime(run.FinishedAt),
	)
	if err != nil {
		return fmt.Errorf("save run %q: %w", run.ID, err)
	}
	return nil
}

func (s *SQLiteStore) Run(ctx context.Context, id string) (*runs.Run, error) {
	row := s.db.QueryRowContext(ctx, `SELECT
		id, task_id, status, worker, workspace_path, agent_id, session_id, work_item_id, created_at, updated_at, finished_at
		FROM runs WHERE id = ?`, id)

	var run runs.Run
	var status string
	var createdAt, updatedAt string
	var finishedAt sql.NullString
	var agentID, sessionID, workItemID sql.NullString
	err := row.Scan(
		&run.ID,
		&run.TaskID,
		&status,
		&run.Worker,
		&run.WorkspacePath,
		&agentID,
		&sessionID,
		&workItemID,
		&createdAt,
		&updatedAt,
		&finishedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("run %q: %w", id, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("load run %q: %w", id, err)
	}

	run.Status = runs.RunStatus(status)
	run.AgentID = agentID.String
	run.SessionID = sessionID.String
	run.WorkItemID = workItemID.String
	if run.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if run.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}
	if run.FinishedAt, err = parseOptionalTime(finishedAt); err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *SQLiteStore) ListRuns(ctx context.Context) ([]runs.Run, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		id, task_id, status, worker, workspace_path, agent_id, session_id, work_item_id, created_at, updated_at, finished_at
		FROM runs ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var result []runs.Run
	for rows.Next() {
		var run runs.Run
		var status string
		var createdAt, updatedAt string
		var finishedAt sql.NullString
		var agentID, sessionID, workItemID sql.NullString
		if err := rows.Scan(
			&run.ID,
			&run.TaskID,
			&status,
			&run.Worker,
			&run.WorkspacePath,
			&agentID,
			&sessionID,
			&workItemID,
			&createdAt,
			&updatedAt,
			&finishedAt,
		); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		run.Status = runs.RunStatus(status)
		run.AgentID = agentID.String
		run.SessionID = sessionID.String
		run.WorkItemID = workItemID.String
		if run.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		if run.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return nil, err
		}
		if run.FinishedAt, err = parseOptionalTime(finishedAt); err != nil {
			return nil, err
		}
		result = append(result, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return result, nil
}

func (s *SQLiteStore) SaveEvent(ctx context.Context, event *events.Event) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO events (
		id, run_id, type, timestamp, payload
	) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		run_id = excluded.run_id,
		type = excluded.type,
		timestamp = excluded.timestamp,
		payload = excluded.payload`,
		event.ID,
		event.RunID,
		string(event.Type),
		formatTime(event.Timestamp),
		[]byte(event.Payload),
	)
	if err != nil {
		return fmt.Errorf("save event %q: %w", event.ID, err)
	}
	return nil
}

func (s *SQLiteStore) EventsByRun(ctx context.Context, runID string) ([]events.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, type, timestamp, payload
		FROM events WHERE run_id = ? ORDER BY timestamp, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list events for run %q: %w", runID, err)
	}
	defer rows.Close()

	var result []events.Event
	for rows.Next() {
		var event events.Event
		var eventType string
		var timestamp string
		var payload []byte
		if err := rows.Scan(&event.ID, &event.RunID, &eventType, &timestamp, &payload); err != nil {
			return nil, fmt.Errorf("scan event for run %q: %w", runID, err)
		}
		event.Type = events.EventType(eventType)
		event.Timestamp, err = parseTime(timestamp)
		if err != nil {
			return nil, err
		}
		event.Payload = append(event.Payload, payload...)
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list events for run %q: %w", runID, err)
	}
	return result, nil
}

func (s *SQLiteStore) SaveArtifact(ctx context.Context, artifact *artifacts.Artifact) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO artifacts (
		id, run_id, path, kind, size_bytes, sha256, keep, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		run_id = excluded.run_id,
		path = excluded.path,
		kind = excluded.kind,
		size_bytes = excluded.size_bytes,
		sha256 = excluded.sha256,
		keep = excluded.keep,
		created_at = excluded.created_at`,
		artifact.ID,
		artifact.RunID,
		artifact.Path,
		string(artifact.Kind),
		artifact.SizeBytes,
		artifact.SHA256,
		boolToInt(artifact.Keep),
		formatTime(artifact.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("save artifact %q: %w", artifact.ID, err)
	}
	return nil
}

func (s *SQLiteStore) ArtifactsByRun(ctx context.Context, runID string) ([]artifacts.Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, path, kind, size_bytes, sha256, keep, created_at
		FROM artifacts WHERE run_id = ? ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list artifacts for run %q: %w", runID, err)
	}
	defer rows.Close()

	var result []artifacts.Artifact
	for rows.Next() {
		var artifact artifacts.Artifact
		var kind string
		var createdAt string
		var keep int
		if err := rows.Scan(&artifact.ID, &artifact.RunID, &artifact.Path, &kind, &artifact.SizeBytes, &artifact.SHA256, &keep, &createdAt); err != nil {
			return nil, fmt.Errorf("scan artifact for run %q: %w", runID, err)
		}
		artifact.Kind = artifacts.Kind(kind)
		artifact.Keep = keep != 0
		artifact.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifacts for run %q: %w", runID, err)
	}
	return result, nil
}

func (s *SQLiteStore) ListArtifacts(ctx context.Context, filter ArtifactListFilter) ([]artifacts.Artifact, error) {
	status := string(filter.Status)
	rows, err := s.db.QueryContext(ctx, `SELECT
		a.id, a.run_id, a.path, a.kind, a.size_bytes, a.sha256, a.keep, a.created_at
		FROM artifacts a
		INNER JOIN runs r ON r.id = a.run_id
		WHERE (? = '' OR a.run_id = ?)
		  AND (? = '' OR r.status = ?)
		ORDER BY a.created_at, a.id`,
		filter.RunID,
		filter.RunID,
		status,
		status,
	)
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()

	var result []artifacts.Artifact
	for rows.Next() {
		var artifact artifacts.Artifact
		var kind string
		var createdAt string
		var keep int
		if err := rows.Scan(&artifact.ID, &artifact.RunID, &artifact.Path, &kind, &artifact.SizeBytes, &artifact.SHA256, &keep, &createdAt); err != nil {
			return nil, fmt.Errorf("scan artifact: %w", err)
		}
		artifact.Kind = artifacts.Kind(kind)
		artifact.Keep = keep != 0
		artifact.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	return result, nil
}

func (s *SQLiteStore) PrunableArtifacts(ctx context.Context, cutoff time.Time) ([]artifacts.PruneCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT
		a.id, a.run_id, a.path, a.kind, a.size_bytes, a.sha256, a.keep, a.created_at, r.status
		FROM artifacts a
		INNER JOIN runs r ON r.id = a.run_id
		WHERE a.created_at < ?
		  AND a.keep = 0
		  AND r.status = ?
		ORDER BY a.created_at, a.id`,
		formatTime(cutoff),
		string(runs.StatusSucceeded),
	)
	if err != nil {
		return nil, fmt.Errorf("list prunable artifacts: %w", err)
	}
	defer rows.Close()

	var result []artifacts.PruneCandidate
	for rows.Next() {
		var artifact artifacts.Artifact
		var kind string
		var createdAt string
		var keep int
		var status string
		if err := rows.Scan(&artifact.ID, &artifact.RunID, &artifact.Path, &kind, &artifact.SizeBytes, &artifact.SHA256, &keep, &createdAt, &status); err != nil {
			return nil, fmt.Errorf("scan prunable artifact: %w", err)
		}
		artifact.Kind = artifacts.Kind(kind)
		artifact.Keep = keep != 0
		artifact.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, artifacts.PruneCandidate{
			Artifact:  artifact,
			RunStatus: runs.RunStatus(status),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list prunable artifacts: %w", err)
	}
	return result, nil
}

func (s *SQLiteStore) DeleteArtifacts(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete artifacts: %w", err)
	}
	defer tx.Rollback()

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM artifacts WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete artifact %q: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete artifacts: %w", err)
	}
	return nil
}

func encodeStrings(values []string) (string, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode string list: %w", err)
	}
	return string(data), nil
}

func decodeStrings(data string, target *[]string) error {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		return fmt.Errorf("decode string list: %w", err)
	}
	return nil
}

func encodeValidationCommands(values []tasks.ValidationCommand) (string, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode validation commands: %w", err)
	}
	return string(data), nil
}

func decodeValidationCommands(data string, target *[]tasks.ValidationCommand) error {
	if err := json.Unmarshal([]byte(data), target); err != nil {
		return fmt.Errorf("decode validation commands: %w", err)
	}
	return nil
}

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*value), Valid: true}
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value, err)
	}
	return parsed, nil
}

func parseOptionalTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	parsed, err := parseTime(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
