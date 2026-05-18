package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/policy"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestCaptureGitDiffIncludesStagedAndUnstagedChanges(t *testing.T) {
	repo := t.TempDir()
	runGitTestCommand(t, repo, "init")
	runGitTestCommand(t, repo, "config", "user.email", "test@example.com")
	runGitTestCommand(t, repo, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGitTestCommand(t, repo, "add", "tracked.txt")
	runGitTestCommand(t, repo, "commit", "-m", "initial")

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("staged\n"), 0o600); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	runGitTestCommand(t, repo, "add", "staged.txt")

	diff, err := CaptureGitDiff(context.Background(), repo)
	if err != nil {
		t.Fatalf("CaptureGitDiff() error = %v", err)
	}
	diffText := string(diff)
	if !strings.Contains(diffText, "diff --git a/tracked.txt b/tracked.txt") {
		t.Fatalf("diff = %q, want unstaged tracked diff", diffText)
	}
	if !strings.Contains(diffText, "diff --git a/staged.txt b/staged.txt") {
		t.Fatalf("diff = %q, want staged file diff", diffText)
	}
}

func TestCodexRunnerRun(t *testing.T) {
	cleanupCalled := false
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	wantWorkspace := filepath.Join(artifactsDir, "run-test-001", "workspace")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-test-001",
		WorkerFactory: func() workers.Worker {
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
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
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
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
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
	assertFileContent(t, filepath.Join(runDir, "changed-files.json"), "[]\n")
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Validation: skipped")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationSkipped, 0)
	assertArtifactManifestContains(t, filepath.Join(runDir, "artifact-manifest.json"), filepath.Join(runDir, "validation.json"), artifacts.KindOther)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: skipped")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation commands: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Artifacts: 9")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: removed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: succeeded")
	if !cleanupCalled {
		t.Fatal("workspace cleanup was not called for succeeded run")
	}

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
		filepath.Join(runDir, "stdout.jsonl"):           false,
		filepath.Join(runDir, "events.jsonl"):           false,
		filepath.Join(runDir, "stderr.log"):             false,
		filepath.Join(runDir, "diff.patch"):             false,
		filepath.Join(runDir, "changed-files.json"):     false,
		filepath.Join(runDir, "validation.log"):         false,
		filepath.Join(runDir, "validation.json"):        false,
		filepath.Join(runDir, "summary.md"):             false,
		filepath.Join(runDir, "artifact-manifest.json"): false,
	}
	for _, artifact := range gotArtifacts {
		if _, ok := wantArtifactPaths[artifact.Path]; !ok {
			t.Fatalf("unexpected artifact path %q", artifact.Path)
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("artifact %q sha256 = %q, want 64 hex chars", artifact.Path, artifact.SHA256)
		}
		if artifact.Path == filepath.Join(runDir, "validation.log") && artifact.SizeBytes == 0 {
			t.Fatalf("validation.log size_bytes = 0, want persisted non-zero size")
		}
		wantArtifactPaths[artifact.Path] = true
	}
	for path, seen := range wantArtifactPaths {
		if !seen {
			t.Fatalf("artifact path %q was not persisted", path)
		}
	}
}

func TestCodexRunnerRunValidationCommandSuccess(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	wantWorkspace := filepath.Join(artifactsDir, "run-validation-success-001", "workspace")
	validationCalled := false

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-success-001",
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			if workspace != wantWorkspace {
				t.Fatalf("validation workspace = %q, want %q", workspace, wantWorkspace)
			}
			if len(commands) != 1 || commands[0].Name != "go-test" {
				t.Fatalf("validation commands = %#v, want go-test command", commands)
			}
			return validationPassed(commands, "go-test")
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !validationCalled {
		t.Fatal("validation runner was not called")
	}

	runDir := filepath.Join(artifactsDir, "run-validation-success-001")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationPassed, 1)
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Validation: passed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: passed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-validation-success-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
	assertPersistedArtifact(t, db, "run-validation-success-001", filepath.Join(runDir, "validation.json"))
	assertPersistedArtifact(t, db, "run-validation-success-001", filepath.Join(runDir, "artifact-manifest.json"))
}

func TestCodexRunnerRunValidationCommandFailure(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-failed-001",
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			return validationFailed(commands, "go-test")
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `validation failed: validation command "go-test" failed with exit code 1`) {
		t.Fatalf("stderr = %q, want validation failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-validation-failed-001")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationFailed, 1)
	assertFileContains(t, filepath.Join(runDir, "validation.log"), `Validation error: validation command "go-test" failed with exit code 1`)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), `Validation error: validation command "go-test" failed with exit code 1`)

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-validation-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusFailed)
	}
}

func TestCodexRunnerRunSkipsValidationWhenWorkerFails(t *testing.T) {
	validationCalled := false
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-skipped-worker-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Stderr:    "boom\n",
				},
				runErr: errors.New("exit 1"),
			}
		},
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			return validationPassed(commands, "go-test")
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if validationCalled {
		t.Fatal("validation runner was called after worker failure")
	}

	runDir := filepath.Join(artifactsDir, "run-validation-skipped-worker-failed-001")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationSkipped, 1)
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Skipped reason: worker failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: skipped")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation commands: 1")
}

func TestCodexRunnerRunPolicyFailureOverridesValidationFailure(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	validationCalled := false
	snapshotCalls := 0

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-policy-failed-001",
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			return validationFailed(commands, "go-test")
		},
		GitSnapshotRunner: func(ctx context.Context, workspace string) (*git.Snapshot, error) {
			snapshotCalls++
			if snapshotCalls == 1 {
				if validationCalled {
					t.Fatal("validation ran before baseline snapshot")
				}
				return &git.Snapshot{}, nil
			}
			if !validationCalled {
				t.Fatal("post-run snapshot was captured before validation")
			}
			return &git.Snapshot{
				Entries: []git.FileEntry{
					{Path: "secrets/from-validation.txt", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
				},
			}, nil
		},
	})
	taskPath := writeTaskFileWithValidationAndPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `policy failed: secrets/from-validation.txt matches forbidden path "secrets/**"`) {
		t.Fatalf("stderr = %q, want policy failure", stderr.String())
	}
	if strings.Contains(stderr.String(), "validation failed:") {
		t.Fatalf("stderr = %q, validation failure should not outrank policy failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-validation-policy-failed-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: policy_failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: failed")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "secrets/from-validation.txt", string(git.StatusUntracked), string(git.StatusUntracked), "snapshot")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-validation-policy-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusPolicyFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusPolicyFailed)
	}
}

func TestCodexRunnerRunPreservesRawStdoutWhenJSONLIsInvalid(t *testing.T) {
	rawStdout := "{not-json}\n"
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-invalid-json-001",
		WorkerFactory: func() workers.Worker {
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
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		WorkspaceManager: fakeWorkspacePreparer{
			cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
				t.Fatal("workspace cleanup should not run for failed run")
				return nil
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
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

func TestCodexRunnerRunKeepsStatusWhenCleanupFails(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-cleanup-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		WorkspaceManager: fakeWorkspacePreparer{
			cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
				return errors.New("remove failed")
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "workspace cleanup warning: workspace cleanup failed: remove failed") {
		t.Fatalf("stderr = %q, want cleanup warning", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-cleanup-failed-001")
	assertFileContains(t, filepath.Join(runDir, "stderr.log"), "workspace cleanup warning: workspace cleanup failed: remove failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup warning: workspace cleanup failed: remove failed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-cleanup-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
}

func TestCodexRunnerRunMarksPolicyFailedForDiffViolations(t *testing.T) {
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
			changedInDiff := policy.ChangedPathsFromGitDiff([]byte(tt.diff))
			postSnapshot := &git.Snapshot{
				Entries: make([]git.FileEntry, 0, len(changedInDiff)),
			}
			for _, p := range changedInDiff {
				postSnapshot.Entries = append(postSnapshot.Entries, git.FileEntry{
					Path:     p,
					Staged:   git.StatusUnmodified,
					Unstaged: git.StatusModified,
				})
			}
			runner := testCodexRunner(t, testCodexRunnerOptions{
				RunID: tt.runID,
				WorkerFactory: func() workers.Worker {
					return fakeWorker{
						runResult: &workers.RunResult{
							Workspace: ".",
							Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
							Events: []workers.WorkerEvent{
								{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
							},
						},
					}
				},
				Diff:     tt.diff,
				Baseline: &git.Snapshot{},
				PostRun:  postSnapshot,
				WorkspaceManager: fakeWorkspacePreparer{
					cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
						t.Fatal("workspace cleanup should not run for policy_failed run")
						return nil
					},
				},
			})

			tempDir := t.TempDir()
			storePath := filepath.Join(tempDir, "deonclaw.db")
			artifactsDir := filepath.Join(tempDir, "artifacts")

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runner.Run(context.Background(), CodexRunOptions{
				TaskPath:     tt.taskPath,
				StorePath:    storePath,
				ArtifactsDir: artifactsDir,
			}, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("Run() exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "policy failed: "+tt.want) {
				t.Fatalf("stderr = %q, want policy failure %q", stderr.String(), tt.want)
			}

			runDir := filepath.Join(artifactsDir, tt.runID)
			assertFileContains(t, filepath.Join(runDir, "diff.patch"), tt.diff)
			assertFileContains(t, filepath.Join(runDir, "changed-files.json"), `"source": "snapshot"`)
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: policy_failed")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Policy: "+tt.want)
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 1")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: policy_failed")

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
			wantChangedFilesPath := filepath.Join(runDir, "changed-files.json")
			var sawDiff bool
			var sawChangedFiles bool
			for _, artifact := range gotArtifacts {
				if artifact.Path == wantDiffPath {
					sawDiff = true
					if artifact.Kind != artifacts.KindDiff {
						t.Fatalf("diff artifact kind = %q, want %q", artifact.Kind, artifacts.KindDiff)
					}
				}
				if artifact.Path == wantChangedFilesPath {
					sawChangedFiles = true
					if artifact.Kind != artifacts.KindOther {
						t.Fatalf("changed-files artifact kind = %q, want %q", artifact.Kind, artifacts.KindOther)
					}
				}
			}
			if !sawDiff {
				t.Fatalf("diff artifact path %q was not persisted", wantDiffPath)
			}
			if !sawChangedFiles {
				t.Fatalf("changed-files artifact path %q was not persisted", wantChangedFilesPath)
			}
		})
	}
}

func TestCodexRunnerRunPersistsFailureArtifacts(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-failed-001",
		WorkerFactory: func() workers.Worker {
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
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "run failed: exit 1") {
		t.Fatalf("stderr = %q, want run failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-failed-001")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"partial"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "boom\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: failed")

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

func TestCodexRunnerRunRejectsMismatchedTaskWorker(t *testing.T) {
	tempDir := t.TempDir()
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID:    "run-mismatch-001",
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "opencode"),
		StorePath:    filepath.Join(tempDir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	want := `task worker "opencode" does not match requested worker "codex"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCodexRunnerRunDetectsChangedPathsFromSnapshotDiff(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-snapshot-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun: &git.Snapshot{
			Entries: []git.FileEntry{
				{Path: "internal/tasks/task.go", Staged: git.StatusModified, Unstaged: git.StatusUnmodified},
				{Path: "newfile.go", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	taskPath := writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**", "newfile.go"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-snapshot-001")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "internal/tasks/task.go", string(git.StatusModified), string(git.StatusUnmodified), "snapshot")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "newfile.go", string(git.StatusUntracked), string(git.StatusUntracked), "snapshot")
	assertFileContains(t, filepath.Join(runDir, "diff.patch"), "# Untracked files from snapshot")
	assertFileContains(t, filepath.Join(runDir, "diff.patch"), "# path: newfile.go staged: ? unstaged: ? source: snapshot")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 2")

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

func TestCodexRunnerRunDetectsDirtyBaseline(t *testing.T) {
	workerCalled := false
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-dirty-001",
		WorkerFactory: func() workers.Worker {
			workerCalled = true
			return fakeWorker{}
		},
		Baseline: &git.Snapshot{
			Entries: []git.FileEntry{
				{Path: "existing-dirty.go", Staged: git.StatusModified, Unstaged: git.StatusUnmodified},
			},
		},
		PostRun: &git.Snapshot{},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
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

func TestCodexRunnerRunPolicyUsesSnapshotPathsNotJustDiff(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-untracked-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun: &git.Snapshot{
			Entries: []git.FileEntry{
				{Path: "secrets/leaked.key", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	taskPath := writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1 (policy failure)", code)
	}
	if !strings.Contains(stderr.String(), `secrets/leaked.key matches forbidden path "secrets/**"`) {
		t.Fatalf("stderr = %q, want untracked forbidden path violation", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-untracked-001")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "secrets/leaked.key", string(git.StatusUntracked), string(git.StatusUntracked), "snapshot")
	assertFileContains(t, filepath.Join(runDir, "diff.patch"), "# path: secrets/leaked.key staged: ? unstaged: ? source: snapshot")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 1")

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

type testCodexRunnerOptions struct {
	RunID             string
	WorkerFactory     func() workers.Worker
	Diff              string
	Baseline          *git.Snapshot
	PostRun           *git.Snapshot
	GitSnapshotRunner GitSnapshotRunner
	ValidationRunner  ValidationRunner
	WorkspaceManager  WorkspacePreparer
}

func testCodexRunner(t *testing.T, opts testCodexRunnerOptions) CodexRunner {
	t.Helper()
	workerFactory := opts.WorkerFactory
	if workerFactory == nil {
		workerFactory = func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
				},
			}
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
	return CodexRunner{
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

type fakeWorkspacePreparer struct {
	prepareFunc func(context.Context, runtime.WorkspaceSpec) (*runtime.Workspace, error)
	cleanupFunc func(context.Context, *runtime.Workspace) error
}

func (f fakeWorkspacePreparer) Prepare(ctx context.Context, spec runtime.WorkspaceSpec) (*runtime.Workspace, error) {
	if f.prepareFunc != nil {
		return f.prepareFunc(ctx, spec)
	}
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
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return nil, err
	}
	return &runtime.Workspace{
		Path:       workspacePath,
		SourcePath: sourcePath,
		Method:     runtime.MethodGitWorktree,
	}, nil
}

func (f fakeWorkspacePreparer) Cleanup(ctx context.Context, workspace *runtime.Workspace) error {
	if f.cleanupFunc != nil {
		return f.cleanupFunc(ctx, workspace)
	}
	return nil
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

func writeTaskFileWithValidation(t *testing.T, worker string) string {
	t.Helper()
	return writeTaskFileContent(t, `id: validation-task-001
title: "Validation task"
domain: general
worker: `+worker+`
goal: "Do not execute"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
validation:
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
      timeout_seconds: 300
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - validation executes
`)
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

func writeTaskFileWithValidationAndPolicy(t *testing.T, worker string, mode string, allowedPaths []string, forbiddenPaths []string) string {
	t.Helper()

	content := `id: task-validation-` + strings.ReplaceAll(mode, "_", "-") + `-001
title: "Validation policy task"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: ` + mode + `
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
validation:
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
      timeout_seconds: 300
allowed_paths:
` + yamlStringList(allowedPaths) + `forbidden_paths:
` + yamlStringList(forbiddenPaths) + `expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - validation executes
`
	return writeTaskFileContent(t, content)
}

func writeTaskFileContent(t *testing.T, content string) string {
	t.Helper()
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

func assertChangedFile(t *testing.T, path string, wantPath string, wantStaged string, wantUnstaged string, wantSource string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var changedFiles []ChangedFile
	if err := json.Unmarshal(got, &changedFiles); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	for _, changed := range changedFiles {
		if changed.Path != wantPath {
			continue
		}
		if changed.Staged != wantStaged || changed.Unstaged != wantUnstaged || changed.Source != wantSource {
			t.Fatalf("changed file %+v, want staged=%q unstaged=%q source=%q", changed, wantStaged, wantUnstaged, wantSource)
		}
		return
	}
	t.Fatalf("changed file %q not found in %q: %s", wantPath, path, got)
}

func assertValidationStatus(t *testing.T, path string, wantStatus string, wantCommandCount int) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var result ValidationResult
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	if result.Status != wantStatus {
		t.Fatalf("validation status = %q, want %q", result.Status, wantStatus)
	}
	if result.CommandCount != wantCommandCount {
		t.Fatalf("validation command_count = %d, want %d", result.CommandCount, wantCommandCount)
	}
}

func assertArtifactManifestContains(t *testing.T, path string, wantPath string, wantKind artifacts.Kind) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var manifest artifactManifest
	if err := json.Unmarshal(got, &manifest); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.Path != wantPath {
			continue
		}
		if artifact.Kind != wantKind {
			t.Fatalf("manifest artifact kind = %q, want %q", artifact.Kind, wantKind)
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("manifest artifact sha256 = %q, want 64 hex chars", artifact.SHA256)
		}
		return
	}
	t.Fatalf("artifact %q not found in manifest %q", wantPath, path)
}

func assertPersistedArtifact(t *testing.T, db *storepkg.SQLiteStore, runID string, wantPath string) {
	t.Helper()
	gotArtifacts, err := db.ArtifactsByRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	for _, artifact := range gotArtifacts {
		if artifact.Path != wantPath {
			continue
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("artifact %q sha256 = %q, want 64 hex chars", artifact.Path, artifact.SHA256)
		}
		return
	}
	t.Fatalf("artifact %q was not persisted", wantPath)
}

func validationPassed(commands []tasks.ValidationCommand, name string) ValidationResult {
	return ValidationResult{
		Status:       ValidationPassed,
		CommandCount: len(commands),
		Commands: []ValidationCommandResult{
			{
				Name:           name,
				Command:        "go",
				Args:           []string{"test", "./..."},
				TimeoutSeconds: defaultValidationTimeoutSeconds,
				ExitCode:       0,
				DurationMS:     1,
				Status:         ValidationPassed,
				Stdout:         "ok\n",
			},
		},
	}
}

func validationFailed(commands []tasks.ValidationCommand, name string) ValidationResult {
	return ValidationResult{
		Status:       ValidationFailed,
		CommandCount: len(commands),
		Error:        `validation command "` + name + `" failed with exit code 1`,
		Commands: []ValidationCommandResult{
			{
				Name:           name,
				Command:        "go",
				Args:           []string{"test", "./..."},
				TimeoutSeconds: defaultValidationTimeoutSeconds,
				ExitCode:       1,
				DurationMS:     1,
				Status:         ValidationFailed,
				Stdout:         "fail\n",
				Stderr:         "boom\n",
				Error:          "exit status 1",
			},
		},
	}
}

func runGitTestCommand(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
