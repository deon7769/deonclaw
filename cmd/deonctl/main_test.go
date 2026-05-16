package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
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
