package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestSQLiteStorePersistsTaskRunEventAndArtifact(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Persist a task",
		Domain: "general",
		Worker: "codex",
		Goal:   "Exercise store persistence",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		Validation: tasks.ValidationSpec{
			Commands: []tasks.ValidationCommand{
				{
					Name:           "go-test",
					Command:        "go",
					Args:           []string{"test", "./..."},
					TimeoutSeconds: 300,
				},
			},
		},
		AllowedPaths:     []string{"internal/store/**"},
		ForbiddenPaths:   []string{"mysecondbrain/**", "secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"records round trip"},
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	gotTask, err := store.Task(ctx, task.ID)
	if err != nil {
		t.Fatalf("Task() error = %v", err)
	}
	if !reflect.DeepEqual(gotTask, task) {
		t.Fatalf("Task() = %#v, want %#v", gotTask, task)
	}

	createdAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	finishedAt := updatedAt.Add(time.Minute)
	run := &runs.Run{
		ID:            "run-001",
		TaskID:        task.ID,
		Status:        runs.StatusSucceeded,
		Worker:        "codex",
		WorkspacePath: filepath.Join("workspaces", "run-001"),
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		FinishedAt:    &finishedAt,
	}
	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	gotRun, err := store.Run(ctx, run.ID)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflect.DeepEqual(gotRun, run) {
		t.Fatalf("Run() = %#v, want %#v", gotRun, run)
	}

	event := &events.Event{
		ID:        "event-001",
		RunID:     run.ID,
		Type:      events.TypeRunStarted,
		Timestamp: createdAt,
		Payload:   json.RawMessage(`{"worker":"codex"}`),
	}
	if err := store.SaveEvent(ctx, event); err != nil {
		t.Fatalf("SaveEvent() error = %v", err)
	}

	gotEvents, err := store.EventsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("EventsByRun() error = %v", err)
	}
	if !reflect.DeepEqual(gotEvents, []events.Event{*event}) {
		t.Fatalf("EventsByRun() = %#v, want %#v", gotEvents, []events.Event{*event})
	}

	artifact := &artifacts.Artifact{
		ID:        "artifact-001",
		RunID:     run.ID,
		Path:      filepath.Join("artifacts", "summary.md"),
		Kind:      artifacts.KindSummary,
		SizeBytes: 12,
		SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Keep:      true,
		CreatedAt: createdAt,
	}
	if err := store.SaveArtifact(ctx, artifact); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}

	gotArtifacts, err := store.ArtifactsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	if !reflect.DeepEqual(gotArtifacts, []artifacts.Artifact{*artifact}) {
		t.Fatalf("ArtifactsByRun() = %#v, want %#v", gotArtifacts, []artifacts.Artifact{*artifact})
	}
}

func TestSQLiteStoreBootstrapsSchemaMigrations(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	var appliedAt string
	err = store.db.QueryRowContext(ctx, `SELECT applied_at FROM schema_migrations WHERE version = ?`, currentSchemaVersion).Scan(&appliedAt)
	if err != nil {
		t.Fatalf("schema_migrations current version query error = %v", err)
	}
	if appliedAt == "" {
		t.Fatal("schema_migrations applied_at is empty")
	}
}

func TestSQLiteStoreKeepsEnsureColumnCompatibilityWithLegacyArtifactsTable(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "deonclaw.db")
	rawDB, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	if _, err := rawDB.ExecContext(ctx, `CREATE TABLE artifacts (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL,
		path TEXT NOT NULL,
		kind TEXT NOT NULL,
		created_at TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("create legacy artifacts table: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("legacy db Close() error = %v", err)
	}

	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	for _, column := range []string{"size_bytes", "sha256", "keep"} {
		if !sqliteColumnExists(t, store.db, "artifacts", column) {
			t.Fatalf("legacy artifacts table missing migrated column %q", column)
		}
	}
}

func TestSQLiteStorePrunableArtifactsAndDeleteArtifacts(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	task := minimalTask()
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	oldTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC)
	runsToSave := []runs.Run{
		{ID: "run-succeeded", TaskID: task.ID, Status: runs.StatusSucceeded, Worker: "codex", WorkspacePath: "workspace", CreatedAt: oldTime, UpdatedAt: oldTime},
		{ID: "run-failed", TaskID: task.ID, Status: runs.StatusFailed, Worker: "codex", WorkspacePath: "workspace", CreatedAt: oldTime, UpdatedAt: oldTime},
		{ID: "run-policy", TaskID: task.ID, Status: runs.StatusPolicyFailed, Worker: "codex", WorkspacePath: "workspace", CreatedAt: oldTime, UpdatedAt: oldTime},
	}
	for i := range runsToSave {
		if err := store.SaveRun(ctx, &runsToSave[i]); err != nil {
			t.Fatalf("SaveRun(%q) error = %v", runsToSave[i].ID, err)
		}
	}

	artifactsToSave := []artifacts.Artifact{
		{ID: "old-succeeded", RunID: "run-succeeded", Path: "artifacts/old-succeeded/summary.md", Kind: artifacts.KindSummary, CreatedAt: oldTime},
		{ID: "new-succeeded", RunID: "run-succeeded", Path: "artifacts/new-succeeded/summary.md", Kind: artifacts.KindSummary, CreatedAt: newTime},
		{ID: "old-keep", RunID: "run-succeeded", Path: "artifacts/old-keep/summary.md", Kind: artifacts.KindSummary, Keep: true, CreatedAt: oldTime},
		{ID: "old-failed", RunID: "run-failed", Path: "artifacts/old-failed/summary.md", Kind: artifacts.KindSummary, CreatedAt: oldTime},
		{ID: "old-policy", RunID: "run-policy", Path: "artifacts/old-policy/summary.md", Kind: artifacts.KindSummary, CreatedAt: oldTime},
	}
	for i := range artifactsToSave {
		if err := store.SaveArtifact(ctx, &artifactsToSave[i]); err != nil {
			t.Fatalf("SaveArtifact(%q) error = %v", artifactsToSave[i].ID, err)
		}
	}

	prunable, err := store.PrunableArtifacts(ctx, cutoff)
	if err != nil {
		t.Fatalf("PrunableArtifacts() error = %v", err)
	}
	if len(prunable) != 1 {
		t.Fatalf("len(prunable) = %d, want 1: %#v", len(prunable), prunable)
	}
	if prunable[0].Artifact.ID != "old-succeeded" {
		t.Fatalf("prunable artifact = %q, want old-succeeded", prunable[0].Artifact.ID)
	}

	if err := store.DeleteArtifacts(ctx, []string{"old-succeeded"}); err != nil {
		t.Fatalf("DeleteArtifacts() error = %v", err)
	}
	gotArtifacts, err := store.ArtifactsByRun(ctx, "run-succeeded")
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	for _, artifact := range gotArtifacts {
		if artifact.ID == "old-succeeded" {
			t.Fatalf("old-succeeded still exists after DeleteArtifacts")
		}
	}
}

func TestSQLiteStoreListArtifactsFiltersByRunAndStatus(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	task := minimalTask()
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	createdAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	runsToSave := []runs.Run{
		{ID: "run-succeeded", TaskID: task.ID, Status: runs.StatusSucceeded, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "run-failed", TaskID: task.ID, Status: runs.StatusFailed, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	for i := range runsToSave {
		if err := store.SaveRun(ctx, &runsToSave[i]); err != nil {
			t.Fatalf("SaveRun(%q) error = %v", runsToSave[i].ID, err)
		}
	}

	artifactsToSave := []artifacts.Artifact{
		{ID: "artifact-succeeded", RunID: "run-succeeded", Path: "artifacts/run-succeeded/summary.md", Kind: artifacts.KindSummary, SizeBytes: 12, SHA256: strings.Repeat("a", 64), CreatedAt: createdAt},
		{ID: "artifact-failed", RunID: "run-failed", Path: "artifacts/run-failed/stderr.log", Kind: artifacts.KindLog, SizeBytes: 9, SHA256: strings.Repeat("b", 64), Keep: true, CreatedAt: createdAt.Add(time.Second)},
	}
	for i := range artifactsToSave {
		if err := store.SaveArtifact(ctx, &artifactsToSave[i]); err != nil {
			t.Fatalf("SaveArtifact(%q) error = %v", artifactsToSave[i].ID, err)
		}
	}

	allArtifacts, err := store.ListArtifacts(ctx, ArtifactListFilter{})
	if err != nil {
		t.Fatalf("ListArtifacts() error = %v", err)
	}
	if len(allArtifacts) != 2 {
		t.Fatalf("len(allArtifacts) = %d, want 2", len(allArtifacts))
	}

	runArtifacts, err := store.ListArtifacts(ctx, ArtifactListFilter{RunID: "run-succeeded"})
	if err != nil {
		t.Fatalf("ListArtifacts(run) error = %v", err)
	}
	if len(runArtifacts) != 1 || runArtifacts[0].ID != "artifact-succeeded" {
		t.Fatalf("runArtifacts = %#v, want artifact-succeeded", runArtifacts)
	}

	failedArtifacts, err := store.ListArtifacts(ctx, ArtifactListFilter{Status: runs.StatusFailed})
	if err != nil {
		t.Fatalf("ListArtifacts(status) error = %v", err)
	}
	if len(failedArtifacts) != 1 || failedArtifacts[0].ID != "artifact-failed" {
		t.Fatalf("failedArtifacts = %#v, want artifact-failed", failedArtifacts)
	}
}

func TestSQLiteStoreReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	if _, err := store.Task(ctx, "missing-task"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Task() error = %v, want ErrNotFound", err)
	}
	if _, err := store.Run(ctx, "missing-run"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Run() error = %v, want ErrNotFound", err)
	}
}

func sqliteColumnExists(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s) error = %v", table, err)
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
			t.Fatalf("scan table info: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table info rows error = %v", err)
	}
	return false
}

func minimalTask() *tasks.Task {
	return &tasks.Task{
		ID:     "task-001",
		Title:  "Persist a task",
		Domain: "general",
		Worker: "codex",
		Goal:   "Exercise store persistence",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"records round trip"},
	}
}
