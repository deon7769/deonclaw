package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
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
