package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
