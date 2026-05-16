package tasks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseValidFixture(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "valid.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	task, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if task.ID != "test-task-001" {
		t.Errorf("ID = %q, want test-task-001", task.ID)
	}
	if task.Worker != "codex" {
		t.Errorf("Worker = %q, want codex", task.Worker)
	}
	if task.Mode != "read_only" {
		t.Errorf("Mode = %q, want read_only", task.Mode)
	}
}

func TestLoadCodexSmokeExample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "tasks", "codex-smoke.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("example task not found at %s", path)
	}

	task, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}
	if task.ID != "codex-smoke-001" {
		t.Errorf("ID = %q, want codex-smoke-001", task.ID)
	}
	if err := Validate(task); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
