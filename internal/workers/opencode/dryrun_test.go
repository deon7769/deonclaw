package opencode

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestDryRunBuildsReadOnlyCommand(t *testing.T) {
	worker := New()
	event, err := worker.DryRun(context.Background(), workers.RunSpec{
		Task:      opencodeTaskWithMode("read_only"),
		Workspace: ".",
	})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	wantCommand := []string{"opencode", "run", "--cwd", ".", "-"}
	if !reflect.DeepEqual(event.Command, wantCommand) {
		t.Fatalf("Command = %#v, want %#v", event.Command, wantCommand)
	}
	if event.Worker != "opencode" {
		t.Fatalf("Worker = %q, want opencode", event.Worker)
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
		Task:      opencodeTaskWithMode("workspace_write"),
		Workspace: "workspaces/run-001",
	})
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	wantCommand := []string{"opencode", "run", "--cwd", "workspaces/run-001", "-"}
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
		Task:      opencodeTaskWithMode("danger-full-access"),
		Workspace: ".",
	})
	if err == nil {
		t.Fatal("DryRun() expected error, got nil")
	}
	if !strings.Contains(err.Error(), `unsupported task mode "danger-full-access"`) {
		t.Fatalf("error = %v, want unsupported mode", err)
	}
}

func opencodeTaskWithMode(mode string) *tasks.Task {
	return &tasks.Task{
		ID:     "task-001",
		Worker: "opencode",
		Goal:   "default prompt",
		Mode:   mode,
		Workspace: tasks.WorkspaceSpec{
			Path: ".",
		},
	}
}
