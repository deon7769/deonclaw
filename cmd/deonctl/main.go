package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/config"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
	"github.com/deon7769/deonclaw/internal/workers/codex"
)

const usage = `deonctl - DeonClaw control CLI

Usage:
  deonctl version
  deonctl task validate <path>
  deonctl worker codex dry-run <task-path>
  deonctl worker codex run <task-path> --store <path> --artifacts-dir <path>
`

var codexWorkerFactory = func() workers.Worker {
	return codex.New()
}

var runIDFactory = func() string {
	return "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "deonclaw %s\n", config.Version)
		return 0
	case "task":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			if len(args) != 3 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runTaskValidate(args[2], stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	case "worker":
		if len(args) < 4 || args[1] != "codex" {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[2] {
		case "dry-run":
			if len(args) != 4 {
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runCodexDryRun(args[3], stdout, stderr)
		case "run":
			opts, err := parseCodexRunOptions(args[3:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runCodexRun(opts, stdout, stderr)
		default:
			fmt.Fprint(stderr, usage)
			return 2
		}
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

func runTaskValidate(path string, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "task %s: valid\n", task.ID)
	return 0
}

func runCodexDryRun(path string, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	if err := ensureTaskWorker(task, "codex"); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	worker := codexWorkerFactory()
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if err != nil {
		fmt.Fprintf(stderr, "dry-run failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "workspace: %s\n", event.Workspace)
	fmt.Fprintf(stdout, "command: %s\n", strings.Join(event.Command, " "))
	return 0
}

type codexRunOptions struct {
	taskPath     string
	storePath    string
	artifactsDir string
}

func parseCodexRunOptions(args []string) (codexRunOptions, error) {
	if len(args) < 1 {
		return codexRunOptions{}, fmt.Errorf("missing task path")
	}

	opts := codexRunOptions{taskPath: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return codexRunOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		default:
			return codexRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return codexRunOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return codexRunOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	return opts, nil
}

func runCodexRun(opts codexRunOptions, stdout io.Writer, stderr io.Writer) int {
	task, err := tasks.LoadFromFile(opts.taskPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	if err := ensureTaskWorker(task, "codex"); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	ctx := context.Background()
	runID := runIDFactory()
	runDir := filepath.Join(opts.artifactsDir, runID)
	now := time.Now().UTC()
	runRecord := &runs.Run{
		ID:            runID,
		TaskID:        task.ID,
		Status:        runs.StatusRunning,
		Worker:        "codex",
		WorkspacePath: task.Workspace.Path,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	db, err := store.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	if err := db.SaveTask(ctx, task); err != nil {
		fmt.Fprintf(stderr, "save task failed: %v\n", err)
		return 1
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		fmt.Fprintf(stderr, "save run failed: %v\n", err)
		return 1
	}

	worker := codexWorkerFactory()
	result, runErr := worker.Run(ctx, workers.RunSpec{
		Task:      task,
		Workspace: task.Workspace.Path,
	})
	if result == nil {
		result = &workers.RunResult{
			Worker:    "codex",
			Workspace: task.Workspace.Path,
		}
	}

	finishedAt := time.Now().UTC()
	runRecord.Status = runs.StatusSucceeded
	if runErr != nil {
		runRecord.Status = runs.StatusFailed
	}
	runRecord.UpdatedAt = finishedAt
	runRecord.FinishedAt = &finishedAt

	if err := db.SaveRun(ctx, runRecord); err != nil {
		fmt.Fprintf(stderr, "save run failed: %v\n", err)
		return 1
	}

	runArtifacts, err := writeCodexRunArtifacts(runDir, runID, task, result, runRecord.Status, runErr, finishedAt)
	if err != nil {
		fmt.Fprintf(stderr, "write artifacts failed: %v\n", err)
		return 1
	}
	if err := saveWorkerEvents(ctx, db, runID, result.Events, finishedAt); err != nil {
		fmt.Fprintf(stderr, "save events failed: %v\n", err)
		return 1
	}
	for i := range runArtifacts {
		if err := db.SaveArtifact(ctx, &runArtifacts[i]); err != nil {
			fmt.Fprintf(stderr, "save artifact failed: %v\n", err)
			return 1
		}
	}

	if result != nil && result.Stderr != "" {
		fmt.Fprintf(stderr, "%s", result.Stderr)
	}
	if runErr != nil {
		fmt.Fprintf(stderr, "run failed: %v\n", runErr)
		return 1
	}

	fmt.Fprintf(stdout, "run_id: %s\n", runID)
	fmt.Fprintf(stdout, "workspace: %s\n", result.Workspace)
	fmt.Fprintf(stdout, "command: %s\n", strings.Join(result.Command, " "))
	fmt.Fprintf(stdout, "events: %d\n", len(result.Events))
	fmt.Fprintf(stdout, "artifacts_dir: %s\n", runDir)
	return 0
}

func writeCodexRunArtifacts(runDir string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, createdAt time.Time) ([]artifacts.Artifact, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}

	files := []struct {
		idSuffix string
		name     string
		kind     artifacts.Kind
		content  []byte
	}{
		{
			idSuffix: "events",
			name:     "events.jsonl",
			kind:     artifacts.KindEvents,
			content:  workerEventsJSONL(result.Events),
		},
		{
			idSuffix: "stderr",
			name:     "stderr.log",
			kind:     artifacts.KindLog,
			content:  []byte(result.Stderr),
		},
		{
			idSuffix: "summary",
			name:     "summary.md",
			kind:     artifacts.KindSummary,
			content:  codexRunSummary(runID, task, result, status, runErr),
		},
	}

	runArtifacts := make([]artifacts.Artifact, 0, len(files))
	for _, file := range files {
		path := filepath.Join(runDir, file.name)
		if err := os.WriteFile(path, file.content, 0o644); err != nil {
			return nil, err
		}
		runArtifacts = append(runArtifacts, artifacts.Artifact{
			ID:        fmt.Sprintf("%s-artifact-%s", runID, file.idSuffix),
			RunID:     runID,
			Path:      path,
			Kind:      file.kind,
			CreatedAt: createdAt,
		})
	}
	return runArtifacts, nil
}

func workerEventsJSONL(workerEvents []workers.WorkerEvent) []byte {
	var output bytes.Buffer
	for _, event := range workerEvents {
		line := event.Payload
		if len(line) == 0 {
			marshaled, err := json.Marshal(event)
			if err != nil {
				continue
			}
			line = marshaled
		}
		output.Write(line)
		output.WriteByte('\n')
	}
	return output.Bytes()
}

func codexRunSummary(runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error) []byte {
	workspace := result.Workspace
	if workspace == "" {
		workspace = task.Workspace.Path
	}

	lines := []string{
		fmt.Sprintf("# Codex run %s", runID),
		"",
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Task: %s", task.ID),
		"Worker: codex",
		fmt.Sprintf("Workspace: %s", workspace),
		fmt.Sprintf("Command: %s", strings.Join(result.Command, " ")),
		fmt.Sprintf("Events: %d", len(result.Events)),
	}
	if runErr != nil {
		lines = append(lines, fmt.Sprintf("Error: %v", runErr))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func saveWorkerEvents(ctx context.Context, db store.Store, runID string, workerEvents []workers.WorkerEvent, timestamp time.Time) error {
	for i, workerEvent := range workerEvents {
		payload := workerEvent.Payload
		if len(payload) == 0 {
			marshaled, err := json.Marshal(workerEvent)
			if err != nil {
				return err
			}
			payload = marshaled
		}
		eventType := workerEvent.Type
		if eventType == "" {
			eventType = workers.EventStdoutJSON
		}
		event := &events.Event{
			ID:        fmt.Sprintf("%s-event-%06d", runID, i+1),
			RunID:     runID,
			Type:      events.EventType(eventType),
			Timestamp: timestamp.Add(time.Duration(i) * time.Nanosecond),
			Payload:   payload,
		}
		if err := db.SaveEvent(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

func ensureTaskWorker(task *tasks.Task, requested string) error {
	if task.Worker != requested {
		return fmt.Errorf("task worker %q does not match requested worker %q", task.Worker, requested)
	}
	return nil
}
