package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/contextpack"
	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/mcpcontext"
	"github.com/deon7769/deonclaw/internal/policy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workerconfig"
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
	TaskPath          string
	StorePath         string
	ArtifactsDir      string
	DomainsPath       string
	MemoryPolicyPath  string
	EnvRequirements   []workerconfig.EnvRequirementCheck
	WorkerRuntime     string
	ValidationRuntime string
	RuntimeConfig     *runtimeconfig.Config
	RuntimeConfigPath string
}

type CodexRunner struct {
	WorkerName              string
	WorkerFactory           WorkerFactory
	RunIDFactory            RunIDFactory
	GitDiffRunner           GitDiffRunner
	GitSnapshotRunner       GitSnapshotRunner
	ValidationRunner        ValidationRunner
	WorkspaceManagerFactory WorkspaceManagerFactory
}

func NewCodexRunner() CodexRunner {
	return CodexRunner{
		WorkerName: "codex",
		WorkerFactory: func() workers.Worker {
			return codex.New()
		},
		RunIDFactory: func() string {
			return "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
		},
		GitDiffRunner:     CaptureGitDiff,
		GitSnapshotRunner: git.TakeSnapshot,
		ValidationRunner:  RunValidationCommands,
		WorkspaceManagerFactory: func() WorkspacePreparer {
			return runtime.NewWorkspaceManager()
		},
	}
}

func (r CodexRunner) Run(ctx context.Context, opts CodexRunOptions, stdout io.Writer, stderr io.Writer) int {
	r = r.withDefaults()
	workerName := r.WorkerName
	startedAt := time.Now().UTC()
	timeline := newExecutionTimeline()

	task, err := tasks.LoadFromFile(opts.TaskPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	timeline.Mark("task_loaded")
	if err := tasks.Validate(task); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	if _, err := retrievalcontext.ValidateTaskMaterializedInjection(task.RetrievalContext.MaterializedInjection); err != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", err)
		return 1
	}
	if err := ensureTaskWorker(task, workerName); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	timeline.Mark("task_validated")
	if strings.TrimSpace(task.ModelProfile) != "" {
		timeline.Mark("model_profile_resolved")
	} else {
		timeline.MarkStatus("model_profile_resolved", "skipped")
	}
	timeline.Mark("env_required_checked")

	var contextPackMarkdown []byte
	var contextPackWarnings []string
	var prompt string
	if strings.TrimSpace(opts.DomainsPath) != "" {
		pack, err := (contextpack.Builder{}).Build(ctx, contextpack.BuildOptions{
			TaskPath:    opts.TaskPath,
			DomainsPath: opts.DomainsPath,
		})
		if err != nil {
			fmt.Fprintf(stderr, "context pack failed: %v\n", err)
			return 1
		}
		contextPackMarkdown = pack.Markdown()
		contextPackWarnings = append(contextPackWarnings, pack.Warnings...)
		timeline.Mark("context_pack_built")
	} else {
		timeline.MarkStatus("context_pack_built", "skipped")
	}
	var mcpContextMarkdown []byte
	var mcpContextCount int
	if len(task.MCPContext.Attachments) > 0 {
		mcpContext, err := mcpcontext.Load(task.MCPContext)
		if err != nil {
			fmt.Fprintf(stderr, "mcp context failed: %v\n", err)
			return 1
		}
		mcpContextMarkdown = mcpContext.Markdown()
		mcpContextCount = mcpContext.Count()
		timeline.Mark("mcp_context_loaded")
	} else {
		timeline.MarkStatus("mcp_context_loaded", "skipped")
	}
	var retrievalContextMarkdown []byte
	var retrievalContextJSON []byte
	var retrievalContextCount int
	var retrievalContextStatus string
	if len(task.RetrievalContext.Attachments) > 0 {
		retrievalContext, err := retrievalcontext.Load(task.RetrievalContext)
		if err != nil {
			fmt.Fprintf(stderr, "retrieval context failed: %v\n", err)
			return 1
		}
		retrievalContextMarkdown = retrievalContext.Markdown()
		retrievalContextJSON, err = retrievalContext.JSON()
		if err != nil {
			fmt.Fprintf(stderr, "retrieval context failed: %v\n", err)
			return 1
		}
		retrievalContextCount = retrievalContext.Count()
		retrievalContextStatus = retrievalContext.Status
		timeline.Mark("retrieval_context_loaded")
	} else {
		retrievalContextStatus = "skipped"
		timeline.MarkStatus("retrieval_context_loaded", "skipped")
	}
	prompt = buildRunnerPrompt(task.Goal, contextPackMarkdown, mcpContextMarkdown, retrievalContextMarkdown)

	runID := r.RunIDFactory()
	runDir := filepath.Join(opts.ArtifactsDir, runID)
	now := time.Now().UTC()

	workspace := strings.TrimSpace(task.Workspace.Path)
	if workspace == "" {
		workspace = "."
	}
	validationRunner, validationRuntime, err := resolveValidationRunner(r.ValidationRunner, opts.ValidationRuntime, task.Validation.Runtime, opts.RuntimeConfig, workspace)
	if err != nil {
		fmt.Fprintf(stderr, "validation runtime failed: %v\n", err)
		return 1
	}
	workerRuntime, err := normalizeWorkerRuntime(opts.WorkerRuntime)
	if err != nil {
		fmt.Fprintf(stderr, "worker runtime failed: %v\n", err)
		return 1
	}
	if workerRuntime == WorkerRuntimeDocker {
		if opts.RuntimeConfig == nil {
			fmt.Fprintln(stderr, "worker runtime failed: worker runtime docker requires --runtime-config")
			return 1
		}
		if _, err := runtimeconfig.PlanDocker(*opts.RuntimeConfig, workspace); err != nil {
			fmt.Fprintf(stderr, "worker runtime failed: %v\n", err)
			return 1
		}
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
	timeline.Mark("workspace_prepared")
	workspace = preparedWorkspace.Path

	runRecord := &runs.Run{
		ID:            runID,
		TaskID:        task.ID,
		Status:        runs.StatusRunning,
		Worker:        workerName,
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
	workerTask := taskForWorker(task)
	timeline.Mark("worker_started")
	result, runErr := runWorkerWithRuntime(ctx, worker, workers.RunSpec{
		Task:      workerTask,
		Workspace: workspace,
		Prompt:    prompt,
	}, workerRuntime, opts.RuntimeConfig)
	if runErr != nil {
		timeline.MarkStatus("worker_finished", "failed")
	} else {
		timeline.Mark("worker_finished")
	}
	if result == nil {
		result = &workers.RunResult{
			Worker:    workerName,
			Workspace: workspace,
		}
	}
	if result.Worker == "" {
		result.Worker = workerName
	}
	result.Workspace = workspace
	mcpToolProposalCheck, mcpProposalErr := checkMCPToolProposal(result, task.MCPProposalPolicy)
	if mcpProposalErr != nil && runErr == nil {
		runErr = mcpProposalErr
	}
	timeline.MarkStatus("mcp_tool_proposal_checked", mcpToolProposalCheck.Status)

	validationResult := skippedValidation(task.Validation.Commands, "worker failed")
	validationResult.Runtime = validationRuntime
	var validationErr error
	if runErr == nil {
		validationResult = validationRunner(ctx, workspace, task.Validation.Commands)
		validationErr = validationFailureError(validationResult)
		if validationErr != nil {
			runErr = validationErr
		}
	}
	timeline.MarkStatus("validation_completed", validationResult.Status)

	diffPatch, diffErr := r.GitDiffRunner(ctx, workspace)
	if diffErr != nil && runErr == nil {
		runErr = fmt.Errorf("capture git diff: %w", diffErr)
	}
	if diffErr != nil {
		timeline.MarkStatus("diff_captured", "failed")
	} else {
		timeline.Mark("diff_captured")
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
	if policyResult.OK() {
		timeline.MarkStatus("path_policy_completed", "ok")
	} else {
		timeline.MarkStatus("path_policy_completed", "failed")
	}

	runRecord.Status = runs.StatusSucceeded
	if runErr != nil {
		runRecord.Status = runs.StatusFailed
	}
	if !policyResult.OK() {
		runRecord.Status = runs.StatusPolicyFailed
	}
	cleanup := cleanupWorkspace(ctx, workspaceManager, preparedWorkspace, runRecord.Status)
	timeline.MarkStatus("workspace_cleanup", cleanup.Action)
	finishedAt := time.Now().UTC()
	runRecord.UpdatedAt = finishedAt
	runRecord.FinishedAt = &finishedAt

	if err := db.SaveRun(ctx, runRecord); err != nil {
		fmt.Fprintf(stderr, "save run failed: %v\n", err)
		return 1
	}

	memoryProposalCheck, err := checkMemoryProposal(runDir, result, opts.MemoryPolicyPath)
	if err != nil {
		fmt.Fprintf(stderr, "memory proposal check failed: %v\n", err)
		return 1
	}

	timeline.Mark("artifacts_written")
	trace := executionTraceOptions{
		RunID:                    runID,
		WorkerName:               workerName,
		Task:                     task,
		Result:                   result,
		Status:                   runRecord.Status,
		StartedAt:                startedAt,
		FinishedAt:               finishedAt,
		Prompt:                   prompt,
		ContextPackMarkdown:      contextPackMarkdown,
		MCPContextMarkdown:       mcpContextMarkdown,
		RetrievalContextMarkdown: retrievalContextMarkdown,
		RetrievalContextAttached: len(task.RetrievalContext.Attachments) > 0 && retrievalContextStatus == "ok",
		RetrievalContextCount:    retrievalContextCount,
		RetrievalContextStatus:   retrievalContextStatus,
		MemoryPolicyPath:         opts.MemoryPolicyPath,
		RuntimeConfigPath:        opts.RuntimeConfigPath,
		MCPToolProposal:          mcpToolProposalCheck,
		EnvRequirements:          opts.EnvRequirements,
		WorkerRuntime:            workerRuntime,
		Validation:               validationResult,
		ValidationRuntime:        validationRuntime,
		PolicyOK:                 policyResult.OK(),
		ChangedPathCount:         len(changedPaths),
		Cleanup:                  cleanup,
		Timeline:                 timeline.Events(),
	}
	runArtifacts, err := writeCodexRunArtifacts(workerName, runDir, runID, task, result, runRecord.Status, runErr, policySummary, len(changedPaths), cleanup, diffPatch, changedFiles, validationResult, contextPackMarkdown, contextPackWarnings, mcpContextMarkdown, mcpContextCount, retrievalContextMarkdown, retrievalContextJSON, retrievalContextCount, mcpToolProposalCheck, memoryProposalCheck, trace, finishedAt)
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
	if validationErr != nil {
		fmt.Fprintf(stderr, "validation failed: %v\n", validationErr)
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

func resolveValidationRunner(defaultRunner ValidationRunner, overrideRuntime string, taskRuntime string, runtimeConfig *runtimeconfig.Config, workspace string) (ValidationRunner, string, error) {
	runtimeName := strings.TrimSpace(overrideRuntime)
	if runtimeName == "" {
		runtimeName = strings.TrimSpace(taskRuntime)
	}
	switch runtimeName {
	case "", tasks.ValidationRuntimeLocal:
		return defaultRunner, tasks.ValidationRuntimeLocal, nil
	case tasks.ValidationRuntimeDocker:
		if runtimeConfig == nil {
			return nil, "", errors.New("validation.runtime docker requires --runtime-config")
		}
		if _, err := runtimeconfig.PlanDocker(*runtimeConfig, workspace); err != nil {
			return nil, "", err
		}
		return NewDockerValidationRunner(*runtimeConfig), tasks.ValidationRuntimeDocker, nil
	default:
		return nil, "", fmt.Errorf("validation.runtime %q is not supported", runtimeName)
	}
}

func taskForWorker(task *tasks.Task) *tasks.Task {
	if task == nil {
		return nil
	}
	copied := *task
	copied.MCPContext = tasks.MCPContextSpec{}
	copied.MCPProposalPolicy = tasks.MCPProposalPolicySpec{}
	copied.RetrievalContext = tasks.RetrievalContextSpec{}
	return &copied
}

func buildRunnerPrompt(goal string, contextPackMarkdown []byte, mcpContextMarkdown []byte, retrievalContextMarkdown []byte) string {
	if len(contextPackMarkdown) == 0 && len(mcpContextMarkdown) == 0 && len(retrievalContextMarkdown) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("# Task Goal\n")
	builder.WriteString(goal)
	builder.WriteString("\n")
	if len(contextPackMarkdown) > 0 {
		builder.WriteString("\n# Context Pack\n")
		builder.Write(contextPackMarkdown)
		if !strings.HasSuffix(string(contextPackMarkdown), "\n") {
			builder.WriteByte('\n')
		}
	}
	if len(mcpContextMarkdown) > 0 {
		builder.WriteByte('\n')
		builder.Write(mcpContextMarkdown)
		if !strings.HasSuffix(string(mcpContextMarkdown), "\n") {
			builder.WriteByte('\n')
		}
	}
	if len(retrievalContextMarkdown) > 0 {
		builder.WriteByte('\n')
		builder.Write(retrievalContextMarkdown)
		if !strings.HasSuffix(string(retrievalContextMarkdown), "\n") {
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

func (r CodexRunner) withDefaults() CodexRunner {
	defaults := NewCodexRunner()
	if r.WorkerName == "" {
		r.WorkerName = defaults.WorkerName
	}
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
	if r.ValidationRunner == nil {
		r.ValidationRunner = defaults.ValidationRunner
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
