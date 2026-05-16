package codex

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestDryRunBuildsReadOnlyCommand(t *testing.T) {
	worker := New()
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      taskWithMode("read_only"),
		Workspace: ".",
	})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	wantCommand := []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"}
	if !reflect.DeepEqual(event.Command, wantCommand) {
		t.Fatalf("Command = %#v, want %#v", event.Command, wantCommand)
	}
	if event.Workspace != "." {
		t.Fatalf("Workspace = %q, want .", event.Workspace)
	}
	if event.Sandbox != "read-only" {
		t.Fatalf("Sandbox = %q, want read-only", event.Sandbox)
	}
}

func TestDryRunBuildsWorkspaceWriteCommand(t *testing.T) {
	worker := New()
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      taskWithMode("workspace_write"),
		Workspace: "workspaces/run-001",
	})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	wantCommand := []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", "workspaces/run-001", "-"}
	if !reflect.DeepEqual(event.Command, wantCommand) {
		t.Fatalf("Command = %#v, want %#v", event.Command, wantCommand)
	}
	if event.Sandbox != "workspace-write" {
		t.Fatalf("Sandbox = %q, want workspace-write", event.Sandbox)
	}
}

func TestDryRunRejectsUnsupportedMode(t *testing.T) {
	worker := New()
	_, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      taskWithMode("danger-full-access"),
		Workspace: ".",
	})
	if err == nil {
		t.Fatal("DryRun() expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported task mode "danger-full-access"`) {
		t.Fatalf("error = %v, want unsupported mode", err)
	}
}

func TestRunExecutesCommandAndCapturesJSONL(t *testing.T) {
	var gotCommand string
	var gotArgs []string
	var gotPrompt string

	worker := newWithRunner("codex", func(ctx context.Context, command string, args []string, prompt string) ([]byte, string, error) {
		gotCommand = command
		gotArgs = append(gotArgs, args...)
		gotPrompt = prompt

		stdout := []byte(`{"type":"message","text":"hello"}` + "\n" + `{"type":"done"}` + "\n")
		return stdout, "stderr line\n", nil
	})

	result, err := worker.Run(context.Background(), workers.RunSpec{
		Task:      taskWithMode("workspace_write"),
		Workspace: "workspaces/run-001",
		Prompt:    "summarize this repository",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if gotCommand != "codex" {
		t.Fatalf("command = %q, want codex", gotCommand)
	}
	wantArgs := []string{"exec", "--json", "--sandbox", "workspace-write", "--cd", "workspaces/run-001", "-"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
	if gotPrompt != "summarize this repository" {
		t.Fatalf("prompt = %q, want prompt sent on stdin", gotPrompt)
	}
	if result.Stderr != "stderr line\n" {
		t.Fatalf("Stderr = %q, want captured stderr", result.Stderr)
	}
	if len(result.Events) != 2 {
		t.Fatalf("len(Events) = %d, want 2", len(result.Events))
	}
	if result.Events[0].Type != "message" {
		t.Fatalf("event type = %q, want message", result.Events[0].Type)
	}
	var payload map[string]string
	if err := json.Unmarshal(result.Events[0].Payload, &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["type"] != "message" || payload["text"] != "hello" {
		t.Fatalf("payload = %#v, want message payload", payload)
	}
}

func TestRunHandlesLargeJSONLLine(t *testing.T) {
	largeText := strings.Repeat("x", 70*1024)
	stdout := []byte(`{"type":"message","text":"` + largeText + `"}` + "\n")
	worker := newWithRunner("codex", func(ctx context.Context, command string, args []string, prompt string) ([]byte, string, error) {
		return stdout, "", nil
	})

	result, err := worker.Run(context.Background(), workers.RunSpec{
		Task:      taskWithMode("read_only"),
		Workspace: ".",
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Events) != 1 {
		t.Fatalf("len(Events) = %d, want 1", len(result.Events))
	}
	if result.Events[0].Type != "message" {
		t.Fatalf("event type = %q, want message", result.Events[0].Type)
	}
}

func TestRunCreatesStdoutAndStderrArtifactsOnCommandError(t *testing.T) {
	worker := newWithRunner("codex", func(ctx context.Context, command string, args []string, prompt string) ([]byte, string, error) {
		return []byte(`{"type":"message","text":"partial"}` + "\n"), "boom\n", errors.New("exit 1")
	})

	result, err := worker.Run(context.Background(), workers.RunSpec{
		Task:      taskWithMode("read_only"),
		Workspace: ".",
	})
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if result == nil {
		t.Fatal("Run() result = nil, want partial result with artifacts")
	}

	want := map[string]struct {
		kind    artifacts.Kind
		content string
	}{
		"artifacts/stdout.jsonl": {kind: artifacts.KindEvents, content: `{"type":"message","text":"partial"}` + "\n"},
		"artifacts/stderr.log":   {kind: artifacts.KindLog, content: "boom\n"},
	}
	if len(result.Artifacts) != len(want) {
		t.Fatalf("len(Artifacts) = %d, want %d", len(result.Artifacts), len(want))
	}
	for _, artifact := range result.Artifacts {
		expected, ok := want[artifact.Path]
		if !ok {
			t.Fatalf("unexpected artifact path %q", artifact.Path)
		}
		if artifact.Kind != expected.kind {
			t.Fatalf("artifact %s kind = %q, want %q", artifact.Path, artifact.Kind, expected.kind)
		}
		if string(artifact.Content) != expected.content {
			t.Fatalf("artifact %s content = %q, want %q", artifact.Path, artifact.Content, expected.content)
		}
	}
}

func TestRunUsesTaskGoalAsDefaultPrompt(t *testing.T) {
	var gotPrompt string
	worker := newWithRunner("codex", func(ctx context.Context, command string, args []string, prompt string) ([]byte, string, error) {
		gotPrompt = prompt
		return []byte(`{"type":"done"}` + "\n"), "", nil
	})

	task := taskWithMode("read_only")
	task.Goal = "use the task goal"

	if _, err := worker.Run(context.Background(), workers.RunSpec{
		Task:      task,
		Workspace: ".",
	}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if gotPrompt != "use the task goal" {
		t.Fatalf("prompt = %q, want task goal", gotPrompt)
	}
}

func TestRunRejectsInvalidJSONL(t *testing.T) {
	worker := newWithRunner("codex", func(ctx context.Context, command string, args []string, prompt string) ([]byte, string, error) {
		return []byte("{not-json}\n"), "", nil
	})

	_, err := worker.Run(context.Background(), workers.RunSpec{
		Task:      taskWithMode("read_only"),
		Workspace: ".",
	})
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse codex stdout jsonl line 1") {
		t.Fatalf("error = %v, want JSONL parse error", err)
	}
}

func taskWithMode(mode string) *tasks.Task {
	return &tasks.Task{
		ID:     "task-001",
		Worker: "codex",
		Goal:   "default prompt",
		Mode:   mode,
		Workspace: tasks.WorkspaceSpec{
			Path: ".",
		},
	}
}
