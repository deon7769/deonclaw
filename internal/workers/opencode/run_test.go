package opencode

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestRunFailsClearlyWhenBinaryMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-opencode")
	result, err := (&Worker{command: missing}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("Run() error = nil, want missing binary failure")
	}
	if result == nil || result.Worker != "opencode" {
		t.Fatalf("result = %#v, want opencode result", result)
	}
	if !strings.Contains(err.Error(), "opencode command failed") {
		t.Fatalf("error = %q, want clear opencode command failure", err)
	}
}

func TestRunCapturesPlainStdoutAndStderr(t *testing.T) {
	script := writeExecutableScript(t, "printf 'plain stdout\\n'; printf 'plain stderr\\n' >&2")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stderr != "plain stderr\n" {
		t.Fatalf("stderr = %q, want captured stderr", result.Stderr)
	}
	if len(result.Artifacts) != 1 || filepath.Base(result.Artifacts[0].Path) != "stdout.log" {
		t.Fatalf("artifacts = %#v, want stdout.log", result.Artifacts)
	}
	if string(result.Artifacts[0].Content) != "plain stdout\n" {
		t.Fatalf("stdout artifact = %q, want stdout", result.Artifacts[0].Content)
	}
	if len(result.Events) != 1 || result.Events[0].Type != "worker.stdout.log" {
		t.Fatalf("events = %#v, want stdout log event", result.Events)
	}
}

func TestRunCapturesJSONLStdoutAsEvents(t *testing.T) {
	script := writeExecutableScript(t, "printf '%s\\n' '{\"type\":\"message\",\"text\":\"ok\"}'")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Events) != 1 || string(result.Events[0].Payload) != "{\"type\":\"message\",\"text\":\"ok\"}" {
		t.Fatalf("events = %#v, want JSONL event", result.Events)
	}
	if len(result.Artifacts) != 1 || filepath.Base(result.Artifacts[0].Path) != "stdout.jsonl" {
		t.Fatalf("artifacts = %#v, want stdout.jsonl", result.Artifacts)
	}
}

func writeExecutableScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode-fake.sh")
	content := strings.Join([]string{
		"#!/bin/sh",
		"shift 4",
		body,
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	return path
}
