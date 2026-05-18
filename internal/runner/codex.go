package runner

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

func writeCodexRunArtifacts(runDir string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup, diffPatch []byte, changedFiles []ChangedFile, createdAt time.Time) ([]artifacts.Artifact, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}

	rawStdout, _ := workerArtifactContent(result.Artifacts, "stdout.jsonl")
	rawStderr, ok := workerArtifactContent(result.Artifacts, "stderr.log")
	if !ok {
		rawStderr = []byte(result.Stderr)
	}
	if cleanup.Warning != "" {
		rawStderr = append(rawStderr, []byte(fmt.Sprintf("workspace cleanup warning: %s\n", cleanup.Warning))...)
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
	if changedFiles == nil {
		changedFiles = []ChangedFile{}
	}
	changedFilesJSON, err := json.MarshalIndent(changedFiles, "", "  ")
	if err != nil {
		return nil, err
	}
	changedFilesJSON = append(changedFilesJSON, '\n')
	if err := writer.write("changed-files", "changed-files.json", artifacts.KindOther, changedFilesJSON); err != nil {
		return nil, err
	}
	if err := writer.write("summary", "summary.md", artifacts.KindSummary, codexRunSummary(runID, task, result, status, runErr, policySummary, changedPathCount, cleanup)); err != nil {
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

type workspaceCleanup struct {
	Action  string
	Reason  runs.RunStatus
	Warning string
}

func cleanupWorkspace(ctx context.Context, manager WorkspacePreparer, workspace *runtime.Workspace, status runs.RunStatus) workspaceCleanup {
	cleanup := workspaceCleanup{
		Action: "kept",
		Reason: status,
	}
	if status != runs.StatusSucceeded {
		return cleanup
	}

	if err := manager.Cleanup(ctx, workspace); err != nil {
		cleanup.Warning = fmt.Sprintf("workspace cleanup failed: %v", err)
		return cleanup
	}
	cleanup.Action = "removed"
	return cleanup
}

type ChangedFile struct {
	Path     string `json:"path"`
	Staged   string `json:"staged"`
	Unstaged string `json:"unstaged"`
	Source   string `json:"source"`
}

func changedFilesFromSnapshot(before *git.Snapshot, after *git.Snapshot) []ChangedFile {
	if after == nil {
		return nil
	}

	beforeSet := make(map[string]struct{})
	if before != nil {
		for _, entry := range before.Entries {
			beforeSet[entry.Path] = struct{}{}
		}
	}

	seen := make(map[string]struct{})
	changedFiles := make([]ChangedFile, 0, len(after.Entries))
	for _, entry := range after.Entries {
		if _, inBefore := beforeSet[entry.Path]; inBefore {
			continue
		}
		if _, already := seen[entry.Path]; already {
			continue
		}
		seen[entry.Path] = struct{}{}
		changedFiles = append(changedFiles, ChangedFile{
			Path:     entry.Path,
			Staged:   string(entry.Staged),
			Unstaged: string(entry.Unstaged),
			Source:   "snapshot",
		})
	}
	return changedFiles
}

func appendUntrackedMetadata(diffPatch []byte, changedFiles []ChangedFile) []byte {
	var untracked []ChangedFile
	for _, changed := range changedFiles {
		if changed.Staged == string(git.StatusUntracked) && changed.Unstaged == string(git.StatusUntracked) {
			untracked = append(untracked, changed)
		}
	}
	if len(untracked) == 0 {
		return diffPatch
	}

	var output bytes.Buffer
	output.Write(diffPatch)
	if len(diffPatch) > 0 && !bytes.HasSuffix(diffPatch, []byte("\n")) {
		output.WriteByte('\n')
	}
	if len(diffPatch) > 0 {
		output.WriteByte('\n')
	}
	output.WriteString("# Untracked files from snapshot\n")
	for _, changed := range untracked {
		output.WriteString("# path: ")
		output.WriteString(changed.Path)
		output.WriteString(" staged: ")
		output.WriteString(changed.Staged)
		output.WriteString(" unstaged: ")
		output.WriteString(changed.Unstaged)
		output.WriteString(" source: ")
		output.WriteString(changed.Source)
		output.WriteByte('\n')
	}
	return output.Bytes()
}

func codexRunSummary(runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup) []byte {
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
		fmt.Sprintf("Changed paths: %d", changedPathCount),
		fmt.Sprintf("Workspace cleanup: %s", cleanup.Action),
		fmt.Sprintf("Cleanup reason: %s", cleanup.Reason),
	}
	if runErr != nil {
		lines = append(lines, fmt.Sprintf("Error: %v", runErr))
	}
	if cleanup.Warning != "" {
		lines = append(lines, fmt.Sprintf("Cleanup warning: %s", cleanup.Warning))
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
	case "stdout.jsonl", "stderr.log", "events.jsonl", "diff.patch", "changed-files.json", "summary.md":
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
