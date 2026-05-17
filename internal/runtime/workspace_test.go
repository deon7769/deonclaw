package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkspaceManagerPreparesGitWorktree(t *testing.T) {
	var gotCommand string
	var gotArgs []string

	manager := newWorkspaceManager(MethodGitWorktree, func(ctx context.Context, command string, args []string) ([]byte, string, error) {
		gotCommand = command
		gotArgs = append(gotArgs, args...)
		return nil, "", nil
	})

	sourcePath := t.TempDir()
	rootDir := t.TempDir()
	workspace, err := manager.Prepare(context.Background(), WorkspaceSpec{
		RunID:      "run-001",
		SourcePath: sourcePath,
		RootDir:    rootDir,
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	wantWorkspacePath := filepath.Join(rootDir, "run-001", "workspace")
	if workspace.Path != wantWorkspacePath {
		t.Fatalf("workspace path = %q, want %q", workspace.Path, wantWorkspacePath)
	}
	if workspace.SourcePath != sourcePath {
		t.Fatalf("source path = %q, want %q", workspace.SourcePath, sourcePath)
	}
	if workspace.Method != MethodGitWorktree {
		t.Fatalf("method = %q, want %q", workspace.Method, MethodGitWorktree)
	}

	wantArgs := []string{
		"-C", sourcePath,
		"worktree", "add",
		"--detach",
		wantWorkspacePath,
		"HEAD",
	}
	if gotCommand != "git" {
		t.Fatalf("command = %q, want git", gotCommand)
	}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestWorkspaceManagerRejectsMissingRootDir(t *testing.T) {
	manager := newWorkspaceManager(MethodGitWorktree, func(ctx context.Context, command string, args []string) ([]byte, string, error) {
		t.Fatal("runner should not be called")
		return nil, "", nil
	})

	_, err := manager.Prepare(context.Background(), WorkspaceSpec{
		RunID:      "run-001",
		SourcePath: ".",
	})
	if err == nil {
		t.Fatal("Prepare() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "workspace root dir is required") {
		t.Fatalf("error = %v, want root dir requirement", err)
	}
}

func TestWorkspaceManagerReturnsWorktreeError(t *testing.T) {
	manager := newWorkspaceManager(MethodGitWorktree, func(ctx context.Context, command string, args []string) ([]byte, string, error) {
		return nil, "fatal: not a git repository\n", errors.New("exit 128")
	})

	_, err := manager.Prepare(context.Background(), WorkspaceSpec{
		RunID:      "run-001",
		SourcePath: t.TempDir(),
		RootDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("Prepare() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "create git worktree workspace") {
		t.Fatalf("error = %v, want worktree context", err)
	}
	if !strings.Contains(err.Error(), "fatal: not a git repository") {
		t.Fatalf("error = %v, want git stderr", err)
	}
}

func TestWorkspaceManagerDocumentsCopyAsFutureMethod(t *testing.T) {
	manager := newWorkspaceManager(MethodCopy, func(ctx context.Context, command string, args []string) ([]byte, string, error) {
		t.Fatal("runner should not be called")
		return nil, "", nil
	})

	_, err := manager.Prepare(context.Background(), WorkspaceSpec{
		RunID:      "run-001",
		SourcePath: ".",
		RootDir:    t.TempDir(),
	})
	if err == nil {
		t.Fatal("Prepare() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "copy workspace method is not implemented") {
		t.Fatalf("error = %v, want copy future method message", err)
	}
}
