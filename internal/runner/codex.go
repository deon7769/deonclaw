package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/policy"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
	"github.com/deon7769/deonclaw/internal/workers/codex"
)

type WorkerFactory func() workers.Worker

type RunIDFactory func() string

type GitDiffRunner func(context.Context, string) ([]byte, error)

type GitSnapshotRunner func(context.Context, string) (*git.Snapshot, error)

type WorkspacePreparer interface {
	Prepare(context.Context, runtime.WorkspaceSpec) (*runtime.Workspace, error)
	Cleanup(context.Context, *runtime.Workspace) error
}

type WorkspaceManagerFactory func() WorkspacePreparer

type CodexRunOptions struct {
	TaskPath     string
	StorePath    string
	ArtifactsDir string
}

type CodexRunner struct {
	WorkerFactory           WorkerFactory
	RunIDFactory            RunIDFactory
	GitDiffRunner           GitDiffRunner
	GitSnapshotRunner       GitSnapshotRunner
	WorkspaceManagerFactory WorkspaceManagerFactory
}

func NewCodexRunner() CodexRunner {
	return CodexRunner{
		WorkerFactory: func() workers.Worker {
			return codex.New()
		},
		RunIDFactory: func() string {
			return "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		},
		GitDiffRunner:     CaptureGitDiff,
		GitSnapshotRunner: git.TakeSnapshot,
		WorkspaceManagerFactory: func() WorkspacePreparer {
			return runtime.NewWorkspaceManager()
		},
	}
}

func (r CodexRunner) Run(ctx context.Context, opts CodexRunOptions, stdout io.Writer, stderr io.Writer) int {
	r = r.withDefaults()

	task, err := tasks.LoadFromFile(opts.TaskPath)
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

	runID := r.RunIDFactory()
	runDir := filepath.Join(opts.ArtifactsDir, runID)
	now := time.Now().UTC()

	workspace := strings.TrimSpace(task.Workspace.Path)
	if workspace == "" {
		workspace = "."
	}

	baseline, snapErr := r.GitSnapshotRunner(ctx, workspace)
	if snapErr != nil {
		fmt.Fprintf(stderr, "capture pre-run snapshot: %v\n", snapErr)
		return 1
	}
	if !baseline.IsClean() {
		fmt.Fprintln(stderr, "workspace is dirty before run")
		return 1
	}

	workspaceManager := r.WorkspaceManagerFactory()
	preparedWorkspace, err := workspaceManager.Prepare(ctx, runtime.WorkspaceSpec{
		RunID:      runID,
		SourcePath: workspace,
		RootDir:    opts.ArtifactsDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "prepare workspace failed: %v\n", err)
		return 1
	}
	workspace = preparedWorkspace.Path

	runRecord := &runs.Run{
		ID:            runID,
		TaskID:        task.ID,
		Status:        runs.StatusRunning,
		Worker:        "codex",
		WorkspacePath: workspace,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	db, err := store.OpenSQLite(opts.StorePath)
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

	worker := r.WorkerFactory()
	result, runErr := worker.Run(ctx, workers.RunSpec{
		Task:      task,
		Workspace: workspace,
	})
	if result == nil {
		result = &workers.RunResult{
			Worker:    "codex",
			Workspace: workspace,
		}
	}
	result.Workspace = workspace

	diffPatch, diffErr := r.GitDiffRunner(ctx, workspace)
	if diffErr != nil && runErr == nil {
		runErr = fmt.Errorf("capture git diff: %w", diffErr)
	}

	postRun, snapPostErr := r.GitSnapshotRunner(ctx, workspace)
	if snapPostErr != nil && runErr == nil {
		runErr = fmt.Errorf("capture post-run snapshot: %w", snapPostErr)
	}

	var changedPaths []string
	var changedFiles []ChangedFile
	if snapPostErr == nil {
		changeDiff := git.Diff{Before: baseline, After: postRun}
		changedPaths = changeDiff.ChangedPaths()
		changedFiles = changedFilesFromSnapshot(baseline, postRun)
		diffPatch = appendUntrackedMetadata(diffPatch, changedFiles)
	} else {
		changedPaths = policy.ChangedPathsFromGitDiff(diffPatch)
	}

	policyResult := policy.EvaluateChangedPaths(
		task.Mode,
		changedPaths,
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
	cleanup := cleanupWorkspace(ctx, workspaceManager, preparedWorkspace, runRecord.Status)
	runRecord.UpdatedAt = finishedAt
	runRecord.FinishedAt = &finishedAt

	if err := db.SaveRun(ctx, runRecord); err != nil {
		fmt.Fprintf(stderr, "save run failed: %v\n", err)
		return 1
	}

	runArtifacts, err := writeCodexRunArtifacts(runDir, runID, task, result, runRecord.Status, runErr, policySummary, len(changedPaths), cleanup, diffPatch, changedFiles, finishedAt)
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
	if cleanup.Warning != "" {
		fmt.Fprintf(stderr, "workspace cleanup warning: %s\n", cleanup.Warning)
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

func (r CodexRunner) withDefaults() CodexRunner {
	defaults := NewCodexRunner()
	if r.WorkerFactory == nil {
		r.WorkerFactory = defaults.WorkerFactory
	}
	if r.RunIDFactory == nil {
		r.RunIDFactory = defaults.RunIDFactory
	}
	if r.GitDiffRunner == nil {
		r.GitDiffRunner = defaults.GitDiffRunner
	}
	if r.GitSnapshotRunner == nil {
		r.GitSnapshotRunner = defaults.GitSnapshotRunner
	}
	if r.WorkspaceManagerFactory == nil {
		r.WorkspaceManagerFactory = defaults.WorkspaceManagerFactory
	}
	return r
}

func CaptureGitDiff(ctx context.Context, workspace string) ([]byte, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace = "."
	}

	unstaged, err := captureGitDiffWithArgs(ctx, workspace, []string{"diff", "--binary"})
	if err != nil {
		return nil, err
	}
	staged, err := captureGitDiffWithArgs(ctx, workspace, []string{"diff", "--cached", "--binary"})
	if err != nil {
		return nil, err
	}

	var diff bytes.Buffer
	diff.Write(unstaged)
	if len(unstaged) > 0 && len(staged) > 0 && !bytes.HasSuffix(unstaged, []byte("\n")) {
		diff.WriteByte('\n')
	}
	diff.Write(staged)
	return diff.Bytes(), nil
}

func captureGitDiffWithArgs(ctx context.Context, workspace string, args []string) ([]byte, error) {
	commandArgs := append([]string{"-C", workspace}, args...)
	cmd := exec.CommandContext(ctx, "git", commandArgs...)
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
