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

	_, err = s.db.ExecContext(ctx, `INSERT INTO tasks (
		id, title, domain, worker, goal, mode,
		workspace_strategy, workspace_path, memory_scope,
		allowed_paths, forbidden_paths, expected_outputs, definition_of_done
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		title = excluded.title,
		domain = excluded.domain,
		worker = excluded.worker,
		goal = excluded.goal,
		mode = excluded.mode,
		workspace_strategy = excluded.workspace_strategy,
		workspace_path = excluded.workspace_path,
		memory_scope = excluded.memory_scope,
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
		allowed_paths, forbidden_paths, expected_outputs, definition_of_done
		FROM tasks WHERE id = ?`, id)

	var task tasks.Task
	var allowedPaths, forbiddenPaths, expectedOutputs, definitionOfDone string
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
		id, task_id, status, worker, workspace_path, created_at, updated_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		task_id = excluded.task_id,
		status = excluded.status,
		worker = excluded.worker,
		workspace_path = excluded.workspace_path,
		created_at = excluded.created_at,
		updated_at = excluded.updated_at,
		finished_at = excluded.finished_at`,
		run.ID,
		run.TaskID,
		string(run.Status),
		run.Worker,
		run.WorkspacePath,
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
		id, task_id, status, worker, workspace_path, created_at, updated_at, finished_at
		FROM runs WHERE id = ?`, id)

	var run runs.Run
	var status string
	var createdAt, updatedAt string
	var finishedAt sql.NullString
	err := row.Scan(
		&run.ID,
		&run.TaskID,
		&status,
		&run.Worker,
		&run.WorkspacePath,
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
		id, run_id, path, kind, created_at
	) VALUES (?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		run_id = excluded.run_id,
		path = excluded.path,
		kind = excluded.kind,
		created_at = excluded.created_at`,
		artifact.ID,
		artifact.RunID,
		artifact.Path,
		string(artifact.Kind),
		formatTime(artifact.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("save artifact %q: %w", artifact.ID, err)
	}
	return nil
}

func (s *SQLiteStore) ArtifactsByRun(ctx context.Context, runID string) ([]artifacts.Artifact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, run_id, path, kind, created_at
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
		if err := rows.Scan(&artifact.ID, &artifact.RunID, &artifact.Path, &kind, &createdAt); err != nil {
			return nil, fmt.Errorf("scan artifact for run %q: %w", runID, err)
		}
		artifact.Kind = artifacts.Kind(kind)
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

func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalTime(value *time.Time) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(*value), Valid: true}
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
