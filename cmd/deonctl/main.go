package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/config"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/policy"
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

var gitDiffRunner = captureGitDiff

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

	workspace := result.Workspace
	if workspace == "" {
		workspace = task.Workspace.Path
	}
	diffPatch, diffErr := gitDiffRunner(ctx, workspace)
	if diffErr != nil && runErr == nil {
		runErr = fmt.Errorf("capture git diff: %w", diffErr)
	}
	policyResult := policy.EvaluateChangedPaths(
		task.Mode,
		policy.ChangedPathsFromGitDiff(diffPatch),
		task.AllowedPaths,
		task.ForbiddenPaths,
	)
	policySummary := policyResult.Summary()

	finishedAt := time.Now().UTC()
	runRecord.Status = runs.StatusSucceeded
	if runErr != nil {
		runRecord.Status = runs.StatusFailed
	}
	if !policyResult.OK() {
		runRecord.Status = runs.StatusPolicyFailed
	}
	runRecord.UpdatedAt = finishedAt
	runRecord.FinishedAt = &finishedAt

	if err := db.SaveRun(ctx, runRecord); err != nil {
		fmt.Fprintf(stderr, "save run failed: %v\n", err)
		return 1
	}

	runArtifacts, err := writeCodexRunArtifacts(runDir, runID, task, result, runRecord.Status, runErr, policySummary, diffPatch, finishedAt)
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
	if !policyResult.OK() {
		fmt.Fprintf(stderr, "policy failed: %s\n", policySummary)
		return 1
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

func captureGitDiff(ctx context.Context, workspace string) ([]byte, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace = "."
	}

	cmd := exec.CommandContext(ctx, "git", "-C", workspace, "diff", "--binary")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	diff, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return nil, fmt.Errorf("%w: %s", err, message)
		}
		return nil, err
	}
	return diff, nil
}

func writeCodexRunArtifacts(runDir string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, diffPatch []byte, createdAt time.Time) ([]artifacts.Artifact, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}

	rawStdout, _ := workerArtifactContent(result.Artifacts, "stdout.jsonl")
	rawStderr, ok := workerArtifactContent(result.Artifacts, "stderr.log")
	if !ok {
		rawStderr = []byte(result.Stderr)
	}

	writer := runArtifactWriter{
		runDir:    runDir,
		runID:     runID,
		createdAt: createdAt,
		written:   make(map[string]struct{}),
	}
	if err := writer.write("stdout", "stdout.jsonl", artifacts.KindEvents, rawStdout); err != nil {
		return nil, err
	}
	if err := writer.write("events", "events.jsonl", artifacts.KindEvents, workerEventsJSONL(result.Events)); err != nil {
		return nil, err
	}
	if err := writer.write("stderr", "stderr.log", artifacts.KindLog, rawStderr); err != nil {
		return nil, err
	}
	if err := writer.write("diff", "diff.patch", artifacts.KindDiff, diffPatch); err != nil {
		return nil, err
	}
	if err := writer.write("summary", "summary.md", artifacts.KindSummary, codexRunSummary(runID, task, result, status, runErr, policySummary)); err != nil {
		return nil, err
	}

	for i, artifact := range result.Artifacts {
		name := artifactFileName(artifact.Path)
		if name == "" || isCLIOwnedArtifact(name) {
			continue
		}
		kind := artifact.Kind
		if kind == "" {
			kind = artifacts.KindOther
		}
		if err := writer.write(fmt.Sprintf("worker-%03d", i+1), name, kind, artifact.Content); err != nil {
			return nil, err
		}
	}
	return writer.artifacts, nil
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

func codexRunSummary(runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string) []byte {
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
		fmt.Sprintf("Policy: %s", policySummary),
	}
	if runErr != nil {
		lines = append(lines, fmt.Sprintf("Error: %v", runErr))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

type runArtifactWriter struct {
	runDir    string
	runID     string
	createdAt time.Time
	written   map[string]struct{}
	artifacts []artifacts.Artifact
}

func (w *runArtifactWriter) write(idSuffix string, name string, kind artifacts.Kind, content []byte) error {
	name = w.uniqueName(name)
	path := filepath.Join(w.runDir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return err
	}
	w.artifacts = append(w.artifacts, artifacts.Artifact{
		ID:        fmt.Sprintf("%s-artifact-%s", w.runID, idSuffix),
		RunID:     w.runID,
		Path:      path,
		Kind:      kind,
		CreatedAt: w.createdAt,
	})
	return nil
}

func (w *runArtifactWriter) uniqueName(name string) string {
	if _, ok := w.written[name]; !ok {
		w.written[name] = struct{}{}
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, ok := w.written[candidate]; !ok {
			w.written[candidate] = struct{}{}
			return candidate
		}
	}
}

func workerArtifactContent(workerArtifacts []artifacts.Artifact, name string) ([]byte, bool) {
	for _, artifact := range workerArtifacts {
		if artifactFileName(artifact.Path) == name {
			return append([]byte(nil), artifact.Content...), true
		}
	}
	return nil, false
}

func artifactFileName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func isCLIOwnedArtifact(name string) bool {
	switch name {
	case "stdout.jsonl", "stderr.log", "events.jsonl", "diff.patch", "summary.md":
		return true
	default:
		return false
	}
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
