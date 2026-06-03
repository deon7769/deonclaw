package runner

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/memory"
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
						Command:   []string{"opencode", "run", "--dir", spec.Workspace, "--format", "json", "<prompt>"},
						Events: []workers.WorkerEvent{
							{Type: "message", Worker: "opencode", Payload: []byte("{\"type\":\"message\",\"text\":\"ok\"}")},
						},
						Artifacts: []artifacts.Artifact{
							{Path: "stdout.log", Kind: artifacts.KindLog, Content: []byte("opencode stdout\n")},
						},
						Stderr: "opencode stderr\n",
						Metadata: map[string]string{
							"opencode.stdout_format":  "text",
							"opencode.parsed_events":  "0",
							"opencode.parse_warnings": "0",
						},
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
	if !strings.Contains(stdout.String(), "command: opencode run --dir "+wantWorkspace+" --format json <prompt>") {
		t.Fatalf("stdout = %q, want opencode command", stdout.String())
	}

	runDir := filepath.Join(artifactsDir, "run-opencode-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.log"), "opencode stdout\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "opencode stderr\n")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), "{\"type\":\"message\",\"text\":\"ok\"}\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker: opencode")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "OpenCode stdout format: text")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "OpenCode parsed events: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "OpenCode parse warnings: 0")
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

func TestOpenCodeRunnerRunWithoutDomainsKeepsPromptUnset(t *testing.T) {
	runner := testOpenCodeRunner(t, testOpenCodeRunnerOptions{
		RunID: "run-opencode-no-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if spec.Prompt != "" {
						t.Fatalf("prompt = %q, want empty prompt when --domains is not configured", spec.Prompt)
					}
					return &workers.RunResult{
						Worker:    "opencode",
						Workspace: spec.Workspace,
						Command:   []string{"opencode", "run", "--dir", spec.Workspace, "--format", "json", "<prompt>"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), OpenCodeRunOptions{
		TaskPath:     writeTaskFile(t, "opencode"),
		StorePath:    filepath.Join(tempDir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tempDir, "artifacts", "run-opencode-no-context-001", "context-pack.md")); !os.IsNotExist(err) {
		t.Fatalf("context-pack.md exists without --domains: %v", err)
	}
}

func TestOpenCodeRunnerRunWithGeneralContextPack(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	bridgePath := filepath.Join(tempDir, "escalasoft-bridge.md")
	if err := os.WriteFile(bridgePath, []byte("Escalasoft isolated bridge content\n"), 0o600); err != nil {
		t.Fatalf("write bridge file: %v", err)
	}
	domainsPath := writeDomainsConfigWithBridge(t, bridgePath)

	runner := testOpenCodeRunner(t, testOpenCodeRunnerOptions{
		RunID: "run-opencode-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if !strings.Contains(spec.Prompt, "# Task Goal\nDo not execute\n\n# Context Pack\n") {
						t.Fatalf("prompt = %q, want task goal and context pack sections", spec.Prompt)
					}
					if strings.Contains(spec.Prompt, "Escalasoft isolated bridge content") {
						t.Fatalf("prompt leaked isolated bridge content for general task: %q", spec.Prompt)
					}
					return &workers.RunResult{
						Worker:    "opencode",
						Workspace: spec.Workspace,
						Command:   []string{"opencode", "run", "--dir", spec.Workspace, "--format", "json", "<prompt>"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), OpenCodeRunOptions{
		TaskPath:     writeTaskFile(t, "opencode"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
		DomainsPath:  domainsPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-opencode-context-001")
	contextPath := filepath.Join(runDir, "context-pack.md")
	assertFileContains(t, contextPath, "domain: general")
	assertFileNotContains(t, contextPath, "Escalasoft isolated bridge content")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker: opencode")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Artifacts: 10")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	assertPersistedArtifactWithMetadata(t, db, "run-opencode-context-001", contextPath)
}

func TestOpenCodeRunnerRunMemoryProposalLintOK(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	proposalJSON := memoryProposalJSON(t, "mem-opencode-ok", "escalasoft", "/domains/escalasoft_brain/cases/case-001.md", memory.OperationUpdate)

	runner := testOpenCodeRunner(t, testOpenCodeRunnerOptions{
		RunID: "run-opencode-memory-proposal-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Worker:    "opencode",
					Workspace: ".",
					Command:   []string{"opencode", "run", "--dir", ".", "--format", "json", "<prompt>"},
					Artifacts: []artifacts.Artifact{
						{Path: "artifacts/memory-proposal.json", Kind: artifacts.KindOther, Content: proposalJSON},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), OpenCodeRunOptions{
		TaskPath:         writeTaskFile(t, "opencode"),
		StorePath:        storePath,
		ArtifactsDir:     artifactsDir,
		MemoryPolicyPath: filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-opencode-memory-proposal-001")
	assertMemoryProposalLintStatus(t, filepath.Join(runDir, "memory-proposal-lint.json"), "ok", 0, 0)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker: opencode")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal: ok")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal id: mem-opencode-ok")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	assertPersistedArtifactWithMetadata(t, db, "run-opencode-memory-proposal-001", filepath.Join(runDir, "memory-proposal.json"))
	assertPersistedArtifactWithMetadata(t, db, "run-opencode-memory-proposal-001", filepath.Join(runDir, "memory-proposal-lint.json"))
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
