package runner

import (
	"context"
	"io"
	"time"

	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/runtime"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"github.com/deon7769/deonclaw/internal/workerconfig"
	"github.com/deon7769/deonclaw/internal/workers"
	"github.com/deon7769/deonclaw/internal/workers/opencode"
)

type OpenCodeRunOptions struct {
	TaskPath          string
	StorePath         string
	ArtifactsDir      string
	DomainsPath       string
	MemoryPolicyPath  string
	EnvRequirements   []workerconfig.EnvRequirementCheck
	ValidationRuntime string
	RuntimeConfig     *runtimeconfig.Config
}

type OpenCodeRunner struct {
	WorkerFactory           WorkerFactory
	RunIDFactory            RunIDFactory
	GitDiffRunner           GitDiffRunner
	GitSnapshotRunner       GitSnapshotRunner
	ValidationRunner        ValidationRunner
	WorkspaceManagerFactory WorkspaceManagerFactory
}

func NewOpenCodeRunner() OpenCodeRunner {
	return OpenCodeRunner{
		WorkerFactory: func() workers.Worker {
			return opencode.New()
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

func (r OpenCodeRunner) Run(ctx context.Context, opts OpenCodeRunOptions, stdout io.Writer, stderr io.Writer) int {
	r = r.withDefaults()
	codexRunner := CodexRunner{
		WorkerName:              "opencode",
		WorkerFactory:           r.WorkerFactory,
		RunIDFactory:            r.RunIDFactory,
		GitDiffRunner:           r.GitDiffRunner,
		GitSnapshotRunner:       r.GitSnapshotRunner,
		ValidationRunner:        r.ValidationRunner,
		WorkspaceManagerFactory: r.WorkspaceManagerFactory,
	}
	return codexRunner.Run(ctx, CodexRunOptions{
		TaskPath:          opts.TaskPath,
		StorePath:         opts.StorePath,
		ArtifactsDir:      opts.ArtifactsDir,
		DomainsPath:       opts.DomainsPath,
		MemoryPolicyPath:  opts.MemoryPolicyPath,
		EnvRequirements:   opts.EnvRequirements,
		ValidationRuntime: opts.ValidationRuntime,
		RuntimeConfig:     opts.RuntimeConfig,
	}, stdout, stderr)
}

func (r OpenCodeRunner) withDefaults() OpenCodeRunner {
	defaults := NewOpenCodeRunner()
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
