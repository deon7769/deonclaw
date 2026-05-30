package runner

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestOpenCodeRunnerRunUsesCommonHarnessAndPersistsArtifacts(t *testing.T) {
	cleanupCalled := false
	validationCalled := false
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	wantWorkspace := filepath.Join(artifactsDir, "run-opencode-001", "workspace")

	runner := testOpenCodeRunner(t, testOpenCodeRunnerOptions{
		RunID: "run-opencode-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if !strings.HasSuffix(spec.Workspace, filepath.Join("run-opencode-001", "workspace")) {
						t.Fatalf("worker workspace = %q, want isolated run workspace", spec.Workspace)
					}
					return &workers.RunResult{
						Worker:    "opencode",
						Workspace: spec.Workspace,
						Command:   []string{"opencode", "run", "--cwd", spec.Workspace, "-"},
						Events: []workers.WorkerEvent{
							{Type: "message", Worker: "opencode", Payload: []byte("{\"type\":\"message\",\"text\":\"ok\"}")},
						},
						Artifacts: []artifacts.Artifact{
							{Path: "stdout.log", Kind: artifacts.KindLog, Content: []byte("opencode stdout\n")},
						},
						Stderr: "opencode stderr\n",
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			if workspace != wantWorkspace {
				t.Fatalf("validation workspace = %q, want %q", workspace, wantWorkspace)
			}
			if len(commands) != 1 || commands[0].Name != "go-test" {
				t.Fatalf("validation commands = %#v, want configured command", commands)
			}
			return ValidationResult{Status: ValidationPassed, CommandCount: len(commands), Commands: []ValidationCommandResult{}}
		},
		WorkspaceManager: fakeWorkspacePreparer{
			cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
				cleanupCalled = true
				if workspace.Path != wantWorkspace {
					t.Fatalf("cleanup workspace path = %q, want %q", workspace.Path, wantWorkspace)
				}
				return nil
			},
		},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), OpenCodeRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "opencode"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !validationCalled {
		t.Fatal("validation was not called")
	}
	if !cleanupCalled {
		t.Fatal("workspace cleanup was not called for succeeded run")
	}
	if !strings.Contains(stdout.String(), "command: opencode run --cwd "+wantWorkspace+" -") {
		t.Fatalf("stdout = %q, want opencode command", stdout.String())
	}

	runDir := filepath.Join(artifactsDir, "run-opencode-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.log"), "opencode stdout\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "opencode stderr\n")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), "{\"type\":\"message\",\"text\":\"ok\"}\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker: opencode")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: passed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: removed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	gotRun, err := db.Run(context.Background(), "run-opencode-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Worker != "opencode" || gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run = %#v, want opencode succeeded", gotRun)
	}
	assertPersistedArtifactWithMetadata(t, db, "run-opencode-001", filepath.Join(runDir, "summary.md"))
}

func TestOpenCodeRunnerRunAppliesPathPolicyAndKeepsWorkspace(t *testing.T) {
	cleanupCalled := false
	runner := testOpenCodeRunner(t, testOpenCodeRunnerOptions{
		RunID: "run-opencode-policy-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{runResult: &workers.RunResult{Worker: "opencode", Command: []string{"opencode", "run"}}}
		},
		Baseline: &git.Snapshot{},
		PostRun: &git.Snapshot{
			Entries: []git.FileEntry{{Path: "secrets/leaked.key", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked}},
		},
		WorkspaceManager: fakeWorkspacePreparer{cleanupFunc: func(context.Context, *runtime.Workspace) error {
			cleanupCalled = true
			return nil
		}},
	})
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), OpenCodeRunOptions{
		TaskPath:     writeTaskFileWithPolicy(t, "opencode", "workspace_write", []string{"internal/**"}, []string{"secrets/**"}),
		StorePath:    filepath.Join(tempDir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if cleanupCalled {
		t.Fatal("workspace cleanup should not run for policy_failed")
	}
	if !strings.Contains(stderr.String(), "policy failed") {
		t.Fatalf("stderr = %q, want policy failure", stderr.String())
	}
	runDir := filepath.Join(tempDir, "artifacts", "run-opencode-policy-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: policy_failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
}

func TestOpenCodeRunnerRunKeepsWorkspaceOnWorkerFailure(t *testing.T) {
	cleanupCalled := false
	runner := testOpenCodeRunner(t, testOpenCodeRunnerOptions{
		RunID: "run-opencode-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{Worker: "opencode", Command: []string{"opencode", "run"}, Stderr: "failed stderr\n"},
				runErr:    errors.New("worker failed"),
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		WorkspaceManager: fakeWorkspacePreparer{cleanupFunc: func(context.Context, *runtime.Workspace) error {
			cleanupCalled = true
			return nil
		}},
	})
	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), OpenCodeRunOptions{
		TaskPath:     writeTaskFile(t, "opencode"),
		StorePath:    filepath.Join(tempDir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if cleanupCalled {
		t.Fatal("workspace cleanup should not run for failed run")
	}
	runDir := filepath.Join(tempDir, "artifacts", "run-opencode-failed-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "failed stderr\n")
}

func testOpenCodeRunner(t *testing.T, opts testOpenCodeRunnerOptions) OpenCodeRunner {
	t.Helper()
	workerFactory := opts.WorkerFactory
	if workerFactory == nil {
		workerFactory = func() workers.Worker {
			return fakeWorker{runResult: &workers.RunResult{Worker: "opencode", Command: []string{"opencode", "run"}}}
		}
	}
	workspaceManager := opts.WorkspaceManager
	if workspaceManager == nil {
		workspaceManager = fakeWorkspacePreparer{}
	}
	callCount := 0
	gitSnapshotRunner := opts.GitSnapshotRunner
	if gitSnapshotRunner == nil {
		gitSnapshotRunner = func(ctx context.Context, workspace string) (*git.Snapshot, error) {
			callCount++
			if callCount == 1 {
				return opts.Baseline, nil
			}
			return opts.PostRun, nil
		}
	}
	return OpenCodeRunner{
		WorkerFactory: workerFactory,
		RunIDFactory: func() string {
			return opts.RunID
		},
		GitDiffRunner: func(context.Context, string) ([]byte, error) {
			return []byte(opts.Diff), nil
		},
		GitSnapshotRunner: gitSnapshotRunner,
		ValidationRunner:  opts.ValidationRunner,
		WorkspaceManagerFactory: func() WorkspacePreparer {
			return workspaceManager
		},
	}
}

type testOpenCodeRunnerOptions struct {
	RunID             string
	WorkerFactory     func() workers.Worker
	Diff              string
	Baseline          *git.Snapshot
	PostRun           *git.Snapshot
	GitSnapshotRunner GitSnapshotRunner
	ValidationRunner  ValidationRunner
	WorkspaceManager  WorkspacePreparer
}
