package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	artifactspkg "github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/config"
	"github.com/deon7769/deonclaw/internal/domains"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/runner"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
	"github.com/deon7769/deonclaw/internal/workers/codex"
)

const usage = `deonctl - DeonClaw control CLI

Usage:
  deonctl version
  deonctl task validate <path>
  deonctl domains validate --config <path>
  deonctl domains list --config <path>
  deonctl worker codex dry-run <task-path>
  deonctl worker codex run <task-path> --store <path> --artifacts-dir <path>
  deonctl artifacts list --store <path> [--run <run-id>] [--status <status>]
  deonctl artifacts prune --store <path> --artifacts-dir <path> --older-than <duration> [--dry-run]
`

var codexWorkerFactory = func() workers.Worker {
	return codex.New()
}

var runIDFactory = func() string {
	return "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
}

var gitDiffRunner = captureGitDiff

var gitSnapshotRunner = git.TakeSnapshot

type workspacePreparer = runner.WorkspacePreparer

type changedFile = runner.ChangedFile

var workspaceManagerFactory = func() workspacePreparer {
	return runtime.NewWorkspaceManager()
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
	case "domains":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		opts, err := parseDomainsOptions(args[2:])
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "validate":
			return runDomainsValidate(opts, stdout, stderr)
		case "list":
			return runDomainsList(opts, stdout, stderr)
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
	case "artifacts":
		if len(args) < 2 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		switch args[1] {
		case "list":
			opts, err := parseArtifactListOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runArtifactList(opts, stdout, stderr)
		case "prune":
			opts, err := parseArtifactPruneOptions(args[2:])
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				fmt.Fprint(stderr, usage)
				return 2
			}
			return runArtifactPrune(opts, stdout, stderr)
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

type domainsOptions struct {
	configPath string
}

func parseDomainsOptions(args []string) (domainsOptions, error) {
	var opts domainsOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return domainsOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return domainsOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return domainsOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func loadAndValidateDomains(path string) (*domains.DomainsConfig, error) {
	config, err := domains.LoadFromFile(path)
	if err != nil {
		return nil, err
	}
	if err := domains.Validate(config); err != nil {
		return nil, err
	}
	return config, nil
}

func runDomainsValidate(opts domainsOptions, stdout io.Writer, stderr io.Writer) int {
	config, err := loadAndValidateDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "domains validation failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "domains config valid: %d domains\n", len(config.Domains))
	return 0
}

func runDomainsList(opts domainsOptions, stdout io.Writer, stderr io.Writer) int {
	config, err := loadAndValidateDomains(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "domains validation failed: %v\n", err)
		return 1
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "name\ttype\troot\tdefault\tisolated\tdefault_agent")
	for _, domain := range config.List() {
		fmt.Fprintf(table, "%s\t%s\t%s\t%t\t%t\t%s\n",
			domain.Name,
			domain.Type,
			domain.Root,
			domain.Default,
			domain.IsIsolated(),
			domain.DefaultAgent,
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "domains list failed: %v\n", err)
		return 1
	}
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
	codexRunner := runner.CodexRunner{
		WorkerFactory:           codexWorkerFactory,
		RunIDFactory:            runIDFactory,
		GitDiffRunner:           gitDiffRunner,
		GitSnapshotRunner:       gitSnapshotRunner,
		WorkspaceManagerFactory: workspaceManagerFactory,
	}
	return codexRunner.Run(context.Background(), runner.CodexRunOptions{
		TaskPath:     opts.taskPath,
		StorePath:    opts.storePath,
		ArtifactsDir: opts.artifactsDir,
	}, stdout, stderr)
}

type artifactListOptions struct {
	storePath string
	runID     string
	status    runs.RunStatus
}

func parseArtifactListOptions(args []string) (artifactListOptions, error) {
	var opts artifactListOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--status":
			if i+1 >= len(args) {
				return artifactListOptions{}, fmt.Errorf("missing value for --status")
			}
			opts.status = runs.RunStatus(args[i+1])
			i++
		default:
			return artifactListOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return artifactListOptions{}, fmt.Errorf("missing --store")
	}
	return opts, nil
}

type artifactPruneOptions struct {
	storePath    string
	artifactsDir string
	olderThan    time.Duration
	olderThanRaw string
	dryRun       bool
}

func parseArtifactPruneOptions(args []string) (artifactPruneOptions, error) {
	var opts artifactPruneOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--artifacts-dir":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --artifacts-dir")
			}
			opts.artifactsDir = args[i+1]
			i++
		case "--older-than":
			if i+1 >= len(args) {
				return artifactPruneOptions{}, fmt.Errorf("missing value for --older-than")
			}
			duration, err := parseRetentionDuration(args[i+1])
			if err != nil {
				return artifactPruneOptions{}, err
			}
			opts.olderThan = duration
			opts.olderThanRaw = args[i+1]
			i++
		case "--dry-run":
			opts.dryRun = true
		default:
			return artifactPruneOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return artifactPruneOptions{}, fmt.Errorf("missing --store")
	}
	if opts.artifactsDir == "" {
		return artifactPruneOptions{}, fmt.Errorf("missing --artifacts-dir")
	}
	if opts.olderThan == 0 {
		return artifactPruneOptions{}, fmt.Errorf("missing --older-than")
	}
	return opts, nil
}

func runArtifactList(opts artifactListOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := db.ListArtifacts(context.Background(), storepkg.ArtifactListFilter{
		RunID:  opts.runID,
		Status: opts.status,
	})
	if err != nil {
		fmt.Fprintf(stderr, "artifact list failed: %v\n", err)
		return 1
	}

	table := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "artifact_id\trun_id\tkind\tpath\tsize_bytes\tsha256\tkeep\tcreated_at")
	for _, artifact := range result {
		fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%d\t%s\t%t\t%s\n",
			artifact.ID,
			artifact.RunID,
			artifact.Kind,
			artifact.Path,
			artifact.SizeBytes,
			artifact.SHA256,
			artifact.Keep,
			artifact.CreatedAt.Format(time.RFC3339Nano),
		)
	}
	if err := table.Flush(); err != nil {
		fmt.Fprintf(stderr, "artifact list failed: %v\n", err)
		return 1
	}
	return 0
}

func parseRetentionDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("missing --older-than")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return 0, fmt.Errorf("invalid --older-than %q", value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid --older-than %q", value)
	}
	return duration, nil
}

func runArtifactPrune(opts artifactPruneOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	result, err := artifactspkg.Prune(context.Background(), db, artifactspkg.RetentionConfig{
		ArtifactsDir: opts.artifactsDir,
		OlderThan:    opts.olderThan,
		DryRun:       opts.dryRun,
	})
	if err != nil {
		fmt.Fprintf(stderr, "artifact prune failed: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "dry_run: %t\n", result.DryRun)
	fmt.Fprintf(stdout, "older_than: %s\n", opts.olderThanRaw)
	fmt.Fprintf(stdout, "cutoff: %s\n", result.Cutoff.Format(time.RFC3339Nano))
	fmt.Fprintf(stdout, "candidates: %d\n", result.Candidates)
	fmt.Fprintf(stdout, "deleted: %d\n", result.Deleted)
	fmt.Fprintf(stdout, "skipped: %d\n", result.Skipped)
	fmt.Fprintf(stdout, "bytes: %d\n", result.SizeBytes)
	return 0
}

func captureGitDiff(ctx context.Context, workspace string) ([]byte, error) {
	return runner.CaptureGitDiff(ctx, workspace)
}

func ensureTaskWorker(task *tasks.Task, requested string) error {
	if task.Worker != requested {
		return fmt.Errorf("task worker %q does not match requested worker %q", task.Worker, requested)
	}
	return nil
}
