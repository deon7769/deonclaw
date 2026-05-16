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
	"github.com/deon7769/deonclaw/internal/runs"
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
			runResult: &workers.RunResult{
				Workspace: ".",
				Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
				Events: []workers.WorkerEvent{
					{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
				},
				Stderr: "stderr line\n",
			},
		}
	})
	setRunID(t, "run-test-001")

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

	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "run_id: run-test-001") {
		t.Fatalf("stdout = %q, want run id", output)
	}
	if !strings.Contains(output, "workspace: .") {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "command: codex exec --json --sandbox read-only --cd . -") {
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

type fakeWorker struct {
	dryRunEvent *workers.WorkerEvent
	runResult   *workers.RunResult
	runErr      error
}

func (f fakeWorker) DryRun(context.Context, workers.RunSpec) (*workers.WorkerEvent, error) {
	return f.dryRunEvent, nil
}

func (f fakeWorker) Run(context.Context, workers.RunSpec) (*workers.RunResult, error) {
	return f.runResult, f.runErr
}
