package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/runs"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestRunVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "deonclaw ") {
		t.Fatalf("stdout = %q, want version output", stdout.String())
	}
}

func TestRunTaskValidate(t *testing.T) {
	taskPath := writeTaskFile(t, "codex")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"task", "validate", taskPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "task worker-mismatch-001: valid") {
		t.Fatalf("stdout = %q, want task valid output", stdout.String())
	}
}

func TestRunDomainsValidate(t *testing.T) {
	configPath := writeDomainsConfigFile(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "validate", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "domains config valid: 2 domains") {
		t.Fatalf("stdout = %q, want domains valid output", stdout.String())
	}
}

func TestRunDomainsValidateRejectsBadArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "validate"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run() exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "missing --config") {
		t.Fatalf("stderr = %q, want missing --config", stderr.String())
	}
}

func TestRunDomainsList(t *testing.T) {
	configPath := writeDomainsConfigFile(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run([]string{"domains", "list", "--config", configPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{
		"name",
		"type",
		"root",
		"default",
		"isolated",
		"default_agent",
		"general",
		"canonical_memory",
		"/vault/mysecondbrain",
		"escalasoft",
		"isolated_domain",
		"/domains/escalasoft_brain",
		"escalasoft-agent",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

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

func TestRunWorkerCodexRunRejectsBadArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "missing store",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--artifacts-dir", t.TempDir()},
			want: "missing --store",
		},
		{
			name: "missing artifacts dir",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--store", filepath.Join(t.TempDir(), "deonclaw.db")},
			want: "missing --artifacts-dir",
		},
		{
			name: "unknown argument",
			args: []string{"worker", "codex", "run", writeTaskFile(t, "codex"), "--store", "db.sqlite", "--artifacts-dir", "artifacts", "--unknown"},
			want: `unknown argument "--unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("run() exit code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tt.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
	}
}

func TestParseCodexRunOptions(t *testing.T) {
	opts, err := parseCodexRunOptions([]string{"task.yaml", "--store", "deonclaw.db", "--artifacts-dir", "artifacts"})
	if err != nil {
		t.Fatalf("parseCodexRunOptions() error = %v", err)
	}
	if opts.taskPath != "task.yaml" {
		t.Fatalf("taskPath = %q, want task.yaml", opts.taskPath)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
}

func TestRunArtifactsPruneDryRun(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	artifactPath := filepath.Join(artifactsDir, "run-old", "summary.md")
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(artifactPath, []byte("summary\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Task",
		Domain: "general",
		Worker: "codex",
		Goal:   "Test prune",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"dry-run lists artifacts"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	oldTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	runRecord := &runs.Run{
		ID:            "run-old",
		TaskID:        task.ID,
		Status:        runs.StatusSucceeded,
		Worker:        "codex",
		WorkspacePath: "workspace",
		CreatedAt:     oldTime,
		UpdatedAt:     oldTime,
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	if err := db.SaveArtifact(ctx, &artifacts.Artifact{
		ID:        "artifact-old",
		RunID:     runRecord.ID,
		Path:      artifactPath,
		Kind:      artifacts.KindSummary,
		SizeBytes: 8,
		CreatedAt: oldTime,
	}); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"artifacts",
		"prune",
		"--store",
		storePath,
		"--artifacts-dir",
		artifactsDir,
		"--older-than",
		"30d",
		"--dry-run",
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}

	output := stdout.String()
	for _, want := range []string{"dry_run: true", "older_than: 30d", "candidates: 1", "deleted: 0", "skipped: 0"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
	if _, err := os.Stat(artifactPath); err != nil {
		t.Fatalf("artifact file was removed in dry-run: %v", err)
	}

	db, err = storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	gotArtifacts, err := db.ArtifactsByRun(ctx, runRecord.ID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	if len(gotArtifacts) != 1 {
		t.Fatalf("len(artifacts) = %d, want 1 after dry-run", len(gotArtifacts))
	}
}

func TestRunArtifactsList(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}

	task := &tasks.Task{
		ID:     "task-001",
		Title:  "Task",
		Domain: "general",
		Worker: "codex",
		Goal:   "List artifacts",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"artifacts are listed"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	createdAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	runRecords := []runs.Run{
		{ID: "run-succeeded", TaskID: task.ID, Status: runs.StatusSucceeded, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "run-failed", TaskID: task.ID, Status: runs.StatusFailed, Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	for i := range runRecords {
		if err := db.SaveRun(ctx, &runRecords[i]); err != nil {
			t.Fatalf("SaveRun() error = %v", err)
		}
	}
	artifactRecords := []artifacts.Artifact{
		{
			ID:        "artifact-summary",
			RunID:     "run-succeeded",
			Path:      filepath.Join("artifacts", "run-succeeded", "summary.md"),
			Kind:      artifacts.KindSummary,
			SizeBytes: 12,
			SHA256:    strings.Repeat("a", 64),
			CreatedAt: createdAt,
		},
		{
			ID:        "artifact-stderr",
			RunID:     "run-failed",
			Path:      filepath.Join("artifacts", "run-failed", "stderr.log"),
			Kind:      artifacts.KindLog,
			SizeBytes: 9,
			SHA256:    strings.Repeat("b", 64),
			Keep:      true,
			CreatedAt: createdAt.Add(time.Second),
		},
	}
	for i := range artifactRecords {
		if err := db.SaveArtifact(ctx, &artifactRecords[i]); err != nil {
			t.Fatalf("SaveArtifact() error = %v", err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"artifacts", "list", "--store", storePath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"artifact_id", "run_id", "kind", "path", "size_bytes", "sha256", "keep", "created_at", "artifact-summary", "artifact-stderr"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"artifacts", "list", "--store", storePath, "--run", "run-succeeded"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--run) exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "artifact-summary") || strings.Contains(stdout.String(), "artifact-stderr") {
		t.Fatalf("stdout with --run = %q, want only artifact-summary", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"artifacts", "list", "--store", storePath, "--status", string(runs.StatusFailed)}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run(--status) exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "artifact-stderr") || strings.Contains(stdout.String(), "artifact-summary") {
		t.Fatalf("stdout with --status = %q, want only artifact-stderr", stdout.String())
	}
}

func TestParseArtifactPruneOptions(t *testing.T) {
	opts, err := parseArtifactPruneOptions([]string{"--store", "deonclaw.db", "--artifacts-dir", "artifacts", "--older-than", "30d", "--dry-run"})
	if err != nil {
		t.Fatalf("parseArtifactPruneOptions() error = %v", err)
	}
	if opts.storePath != "deonclaw.db" {
		t.Fatalf("storePath = %q, want deonclaw.db", opts.storePath)
	}
	if opts.artifactsDir != "artifacts" {
		t.Fatalf("artifactsDir = %q, want artifacts", opts.artifactsDir)
	}
	if opts.olderThan != 30*24*time.Hour {
		t.Fatalf("olderThan = %s, want 720h", opts.olderThan)
	}
	if !opts.dryRun {
		t.Fatal("dryRun = false, want true")
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

func writeDomainsConfigFile(t *testing.T) string {
	t.Helper()

	content := `domains:
  general:
    type: canonical_memory
    root: /vault/mysecondbrain
    default: true
    read_only_for_workers: true

  escalasoft:
    type: isolated_domain
    root: /domains/escalasoft_brain
    default: false
    read_only_for_general_agents: true
    bridge_files:
      - /vault/mysecondbrain/memory/context/escalasoft-operacao.md
    structured_data:
      historical_sqlite: /data/escalasoft-sqlite/escalasoft_docs.db
    staging:
      - /tmp/drive-escalasoft-docs
    default_agent: escalasoft-agent
`
	path := filepath.Join(t.TempDir(), "domains.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write domains config file: %v", err)
	}
	return path
}
