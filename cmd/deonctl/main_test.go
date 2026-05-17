package main

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
	"github.com/deon7769/deonclaw/internal/policy"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestRunWorkerCodexDryRun(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{
		"worker",
		"codex",
		"dry-run",
		filepath.Join("..", "..", "examples", "tasks", "codex-smoke.yaml"),
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "workspace: .") {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "command: codex exec --json --sandbox read-only --cd . -") {
		t.Fatalf("stdout = %q, want planned command", output)
	}
}

func TestRunWorkerCodexRun(t *testing.T) {
	setCodexWorkerFactory(t, func() workers.Worker {
		return fakeWorker{
			runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
				if spec.Workspace == "." {
					t.Fatal("worker received source repo workspace")
				}
				if !strings.HasSuffix(spec.Workspace, filepath.Join("run-test-001", "workspace")) {
					t.Fatalf("worker workspace = %q, want isolated run workspace", spec.Workspace)
				}
				return &workers.RunResult{
					Workspace: spec.Workspace,
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
					Stderr: "stderr line\n",
				}, nil
			},
		}
	})
	setRunID(t, "run-test-001")
	setGitDiff(t, "")
	setGitSnapshot(t, &git.Snapshot{}, &git.Snapshot{})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	wantWorkspace := filepath.Join(artifactsDir, "run-test-001", "workspace")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		writeTaskFile(t, "codex"),
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "run_id: run-test-001") {
		t.Fatalf("stdout = %q, want run id", output)
	}
	if !strings.Contains(output, "workspace: "+wantWorkspace) {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "command: codex exec --json --sandbox read-only --cd "+wantWorkspace+" -") {
		t.Fatalf("stdout = %q, want command", output)
	}
	if !strings.Contains(output, "events: 1") {
		t.Fatalf("stdout = %q, want event count", output)
	}
	if !strings.Contains(output, "artifacts_dir: "+filepath.Join(artifactsDir, "run-test-001")) {
		t.Fatalf("stdout = %q, want artifacts dir", output)
	}

	runDir := filepath.Join(artifactsDir, "run-test-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.jsonl"), "")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"ok"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "stderr line\n")
	assertFileContent(t, filepath.Join(runDir, "diff.patch"), "")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-test-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
	if gotRun.WorkspacePath != wantWorkspace {
		t.Fatalf("run workspace path = %q, want %q", gotRun.WorkspacePath, wantWorkspace)
	}
	if gotRun.TaskID != "worker-mismatch-001" {
		t.Fatalf("run task id = %q, want task id", gotRun.TaskID)
	}

	gotEvents, err := db.EventsByRun(context.Background(), "run-test-001")
	if err != nil {
		t.Fatalf("EventsByRun() error = %v", err)
	}
	if len(gotEvents) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(gotEvents))
	}
	if string(gotEvents[0].Type) != "message" {
		t.Fatalf("event type = %q, want message", gotEvents[0].Type)
	}

	gotArtifacts, err := db.ArtifactsByRun(context.Background(), "run-test-001")
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	wantArtifactPaths := map[string]bool{
		filepath.Join(runDir, "stdout.jsonl"): false,
		filepath.Join(runDir, "events.jsonl"): false,
		filepath.Join(runDir, "stderr.log"):   false,
		filepath.Join(runDir, "diff.patch"):   false,
		filepath.Join(runDir, "summary.md"):   false,
	}
	for _, artifact := range gotArtifacts {
		if _, ok := wantArtifactPaths[artifact.Path]; !ok {
			t.Fatalf("unexpected artifact path %q", artifact.Path)
		}
		wantArtifactPaths[artifact.Path] = true
	}
	for path, seen := range wantArtifactPaths {
		if !seen {
			t.Fatalf("artifact path %q was not persisted", path)
		}
	}
}

func TestRunWorkerCodexRunPreservesRawStdoutWhenJSONLIsInvalid(t *testing.T) {
	rawStdout := "{not-json}\n"
	setCodexWorkerFactory(t, func() workers.Worker {
		return fakeWorker{
			runResult: &workers.RunResult{
				Workspace: ".",
				Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
				Artifacts: []artifacts.Artifact{
					{ID: "stdout", Path: "artifacts/stdout.jsonl", Kind: artifacts.KindEvents, Content: []byte(rawStdout)},
					{ID: "stderr", Path: "artifacts/stderr.log", Kind: artifacts.KindLog, Content: []byte("parse warning\n")},
					{ID: "trace", Path: "artifacts/trace.txt", Kind: artifacts.KindOther, Content: []byte("raw trace\n")},
				},
				Stderr: "parse warning\n",
			},
			runErr: errors.New("parse codex stdout jsonl line 1: invalid JSON"),
		}
	})
	setRunID(t, "run-invalid-json-001")
	setGitDiff(t, "")
	setGitSnapshot(t, &git.Snapshot{}, &git.Snapshot{})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		writeTaskFile(t, "codex"),
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid JSON") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-invalid-json-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.jsonl"), rawStdout)
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "parse warning\n")
	assertFileContent(t, filepath.Join(runDir, "trace.txt"), "raw trace\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotArtifacts, err := db.ArtifactsByRun(context.Background(), "run-invalid-json-001")
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	wantStdoutPath := filepath.Join(runDir, "stdout.jsonl")
	wantTracePath := filepath.Join(runDir, "trace.txt")
	var sawStdout bool
	var sawTrace bool
	for _, artifact := range gotArtifacts {
		if artifact.Path == wantStdoutPath {
			sawStdout = true
			if artifact.Kind != artifacts.KindEvents {
				t.Fatalf("stdout artifact kind = %q, want %q", artifact.Kind, artifacts.KindEvents)
			}
		}
		if artifact.Path == wantTracePath {
			sawTrace = true
			if artifact.Kind != artifacts.KindOther {
				t.Fatalf("trace artifact kind = %q, want %q", artifact.Kind, artifacts.KindOther)
			}
		}
	}
	if !sawStdout {
		t.Fatalf("stdout artifact path %q was not persisted", wantStdoutPath)
	}
	if !sawTrace {
		t.Fatalf("trace artifact path %q was not persisted", wantTracePath)
	}
}

func TestRunWorkerCodexRunMarksPolicyFailedForDiffViolations(t *testing.T) {
	tests := []struct {
		name     string
		runID    string
		taskPath string
		diff     string
		want     string
	}{
		{
			name:     "forbidden path",
			runID:    "run-policy-forbidden-001",
			taskPath: writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"}),
			diff: `diff --git a/secrets/token.txt b/secrets/token.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/secrets/token.txt
@@ -0,0 +1 @@
+token
`,
			want: `secrets/token.txt matches forbidden path "secrets/**"`,
		},
		{
			name:     "outside allowed paths",
			runID:    "run-policy-allowed-001",
			taskPath: writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"}),
			diff: `diff --git a/README.md b/README.md
index 1111111..2222222 100644
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-old
+new
`,
			want: "README.md is outside allowed paths",
		},
		{
			name:     "read only diff",
			runID:    "run-policy-readonly-001",
			taskPath: writeTaskFileWithPolicy(t, "codex", "read_only", nil, []string{"secrets/**"}),
			diff: `diff --git a/internal/tasks/task.go b/internal/tasks/task.go
index 1111111..2222222 100644
--- a/internal/tasks/task.go
+++ b/internal/tasks/task.go
@@ -1 +1 @@
-old
+new
`,
			want: "read_only task changed files: internal/tasks/task.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setCodexWorkerFactory(t, func() workers.Worker {
				return fakeWorker{
					runResult: &workers.RunResult{
						Workspace: ".",
						Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
						Events: []workers.WorkerEvent{
							{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
						},
					},
				}
			})
			setRunID(t, tt.runID)
			setGitDiff(t, tt.diff)
			changedInDiff := policy.ChangedPathsFromGitDiff([]byte(tt.diff))
			postSnapshot := &git.Snapshot{
				Entries: make([]git.FileEntry, 0, len(changedInDiff)),
			}
			for _, p := range changedInDiff {
				postSnapshot.Entries = append(postSnapshot.Entries, git.FileEntry{
					Path:     p,
					Staged:   git.StatusUntracked,
					Unstaged: git.StatusUntracked,
				})
			}
			setGitSnapshot(t, &git.Snapshot{}, postSnapshot)

			tempDir := t.TempDir()
			storePath := filepath.Join(tempDir, "deonclaw.db")
			artifactsDir := filepath.Join(tempDir, "artifacts")

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run([]string{
				"worker",
				"codex",
				"run",
				tt.taskPath,
				"--store",
				storePath,
				"--artifacts-dir",
				artifactsDir,
			}, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("run() exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "policy failed: "+tt.want) {
				t.Fatalf("stderr = %q, want policy failure %q", stderr.String(), tt.want)
			}

			runDir := filepath.Join(artifactsDir, tt.runID)
			assertFileContent(t, filepath.Join(runDir, "diff.patch"), tt.diff)
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: policy_failed")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Policy: "+tt.want)

			db, err := storepkg.OpenSQLite(storePath)
			if err != nil {
				t.Fatalf("OpenSQLite() error = %v", err)
			}
			defer db.Close()

			gotRun, err := db.Run(context.Background(), tt.runID)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if gotRun.Status != runs.StatusPolicyFailed {
				t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusPolicyFailed)
			}

			gotArtifacts, err := db.ArtifactsByRun(context.Background(), tt.runID)
			if err != nil {
				t.Fatalf("ArtifactsByRun() error = %v", err)
			}
			wantDiffPath := filepath.Join(runDir, "diff.patch")
			var sawDiff bool
			for _, artifact := range gotArtifacts {
				if artifact.Path == wantDiffPath {
					sawDiff = true
					if artifact.Kind != artifacts.KindDiff {
						t.Fatalf("diff artifact kind = %q, want %q", artifact.Kind, artifacts.KindDiff)
					}
				}
			}
			if !sawDiff {
				t.Fatalf("diff artifact path %q was not persisted", wantDiffPath)
			}
		})
	}
}

func TestRunWorkerCodexRunPersistsFailureArtifacts(t *testing.T) {
	setCodexWorkerFactory(t, func() workers.Worker {
		return fakeWorker{
			runResult: &workers.RunResult{
				Workspace: ".",
				Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
				Events: []workers.WorkerEvent{
					{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"partial"}`)},
				},
				Stderr: "boom\n",
			},
			runErr: errors.New("exit 1"),
		}
	})
	setRunID(t, "run-failed-001")
	setGitDiff(t, "")
	setGitSnapshot(t, &git.Snapshot{}, &git.Snapshot{})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		writeTaskFile(t, "codex"),
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "run failed: exit 1") {
		t.Fatalf("stderr = %q, want run failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-failed-001")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"partial"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "boom\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusFailed)
	}

	gotEvents, err := db.EventsByRun(context.Background(), "run-failed-001")
	if err != nil {
		t.Fatalf("EventsByRun() error = %v", err)
	}
	if len(gotEvents) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(gotEvents))
	}
}

func TestRunWorkerCodexDryRunRejectsMismatchedTaskWorker(t *testing.T) {
	taskPath := writeTaskFile(t, "opencode")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"worker", "codex", "dry-run", taskPath}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	want := `task worker "opencode" does not match requested worker "codex"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestRunWorkerCodexRunRejectsMismatchedTaskWorker(t *testing.T) {
	taskPath := writeTaskFile(t, "opencode")
	tempDir := t.TempDir()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		taskPath,
		"--store",
		filepath.Join(tempDir, "deonclaw.db"),
		"--artifacts-dir",
		filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	want := `task worker "opencode" does not match requested worker "codex"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func writeTaskFile(t *testing.T, worker string) string {
	t.Helper()

	content := `id: worker-mismatch-001
title: "Worker mismatch"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func setCodexWorkerFactory(t *testing.T, factory func() workers.Worker) {
	t.Helper()
	oldFactory := codexWorkerFactory
	t.Cleanup(func() {
		codexWorkerFactory = oldFactory
	})
	codexWorkerFactory = factory
}

func setRunID(t *testing.T, runID string) {
	t.Helper()
	oldFactory := runIDFactory
	t.Cleanup(func() {
		runIDFactory = oldFactory
	})
	runIDFactory = func() string {
		return runID
	}
}

func setGitDiff(t *testing.T, diff string) {
	t.Helper()
	oldRunner := gitDiffRunner
	t.Cleanup(func() {
		gitDiffRunner = oldRunner
	})
	gitDiffRunner = func(context.Context, string) ([]byte, error) {
		return []byte(diff), nil
	}
}

func setGitSnapshot(t *testing.T, baseline *git.Snapshot, postRun *git.Snapshot) {
	t.Helper()
	oldRunner := gitSnapshotRunner
	oldWorkspaceFactory := workspaceManagerFactory
	t.Cleanup(func() {
		gitSnapshotRunner = oldRunner
		workspaceManagerFactory = oldWorkspaceFactory
	})
	callCount := 0
	gitSnapshotRunner = func(ctx context.Context, workspace string) (*git.Snapshot, error) {
		callCount++
		if callCount == 1 {
			return baseline, nil
		}
		return postRun, nil
	}
	workspaceManagerFactory = func() workspacePreparer {
		return fakeWorkspacePreparer{}
	}
}

type fakeWorkspacePreparer struct{}

func (fakeWorkspacePreparer) Prepare(ctx context.Context, spec runtime.WorkspaceSpec) (*runtime.Workspace, error) {
	sourcePath := strings.TrimSpace(spec.SourcePath)
	if sourcePath == "" {
		sourcePath = "."
	}
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, err
	}
	workspacePath, err := filepath.Abs(filepath.Join(spec.RootDir, spec.RunID, "workspace"))
	if err != nil {
		return nil, err
	}
	return &runtime.Workspace{
		Path:       workspacePath,
		SourcePath: sourcePath,
		Method:     runtime.MethodGitWorktree,
	}, nil
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if !strings.Contains(string(got), want) {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func writeTaskFileWithPolicy(t *testing.T, worker string, mode string, allowedPaths []string, forbiddenPaths []string) string {
	t.Helper()

	content := `id: task-` + strings.ReplaceAll(mode, "_", "-") + `-001
title: "Policy task"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: ` + mode + `
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths:
` + yamlStringList(allowedPaths) + `forbidden_paths:
` + yamlStringList(forbiddenPaths) + `expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func yamlStringList(values []string) string {
	if len(values) == 0 {
		return "  []\n"
	}
	var output strings.Builder
	for _, value := range values {
		output.WriteString("  - ")
		output.WriteString(value)
		output.WriteByte('\n')
	}
	return output.String()
}

type fakeWorker struct {
	dryRunEvent *workers.WorkerEvent
	runFunc     func(context.Context, workers.RunSpec) (*workers.RunResult, error)
	runResult   *workers.RunResult
	runErr      error
}

func (f fakeWorker) DryRun(context.Context, workers.RunSpec) (*workers.WorkerEvent, error) {
	return f.dryRunEvent, nil
}

func (f fakeWorker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, spec)
	}
	return f.runResult, f.runErr
}

func TestRunCodexRunDetectsChangedPathsFromSnapshotDiff(t *testing.T) {
	setCodexWorkerFactory(t, func() workers.Worker {
		return fakeWorker{
			runResult: &workers.RunResult{
				Workspace: ".",
				Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
				Events: []workers.WorkerEvent{
					{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
				},
			},
		}
	})
	setRunID(t, "run-snapshot-001")
	setGitDiff(t, "")
	postSnapshot := &git.Snapshot{
		Entries: []git.FileEntry{
			{Path: "internal/tasks/task.go", Staged: git.StatusModified, Unstaged: git.StatusUnmodified},
			{Path: "newfile.go", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
		},
	}
	setGitSnapshot(t, &git.Snapshot{}, postSnapshot)

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	taskPath := writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**", "newfile.go"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		taskPath,
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-snapshot-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
}

func TestRunCodexRunDetectsDirtyBaseline(t *testing.T) {
	workerCalled := false
	setCodexWorkerFactory(t, func() workers.Worker {
		workerCalled = true
		return fakeWorker{}
	})
	setRunID(t, "run-dirty-001")
	setGitDiff(t, "")
	dirtySnapshot := &git.Snapshot{
		Entries: []git.FileEntry{
			{Path: "existing-dirty.go", Staged: git.StatusModified, Unstaged: git.StatusUnmodified},
		},
	}
	setGitSnapshot(t, dirtySnapshot, &git.Snapshot{})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		writeTaskFile(t, "codex"),
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "workspace is dirty before run") {
		t.Fatalf("stderr = %q, want dirty baseline failure", stderr.String())
	}
	if workerCalled {
		t.Fatal("worker was called for dirty baseline")
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("store path exists after dirty baseline failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "run-dirty-001")); !os.IsNotExist(err) {
		t.Fatalf("run artifacts dir exists after dirty baseline failure: %v", err)
	}
}

func TestRunCodexRunPolicyUsesSnapshotPathsNotJustDiff(t *testing.T) {
	setCodexWorkerFactory(t, func() workers.Worker {
		return fakeWorker{
			runResult: &workers.RunResult{
				Workspace: ".",
				Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
				Events: []workers.WorkerEvent{
					{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
				},
			},
		}
	})
	setRunID(t, "run-untracked-001")
	setGitDiff(t, "")
	postSnapshot := &git.Snapshot{
		Entries: []git.FileEntry{
			{Path: "secrets/leaked.key", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
		},
	}
	setGitSnapshot(t, &git.Snapshot{}, postSnapshot)

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	taskPath := writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"worker",
		"codex",
		"run",
		taskPath,
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("run() exit code = %d, want 1 (policy failure)", code)
	}
	if !strings.Contains(stderr.String(), `secrets/leaked.key matches forbidden path "secrets/**"`) {
		t.Fatalf("stderr = %q, want untracked forbidden path violation", stderr.String())
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-untracked-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusPolicyFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusPolicyFailed)
	}
}
