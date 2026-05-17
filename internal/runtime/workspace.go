package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type WorkspaceMethod string

const (
	MethodGitWorktree WorkspaceMethod = "git_worktree"
	MethodCopy        WorkspaceMethod = "copy"
)

type WorkspaceSpec struct {
	RunID      string
	SourcePath string
	RootDir    string
}

type Workspace struct {
	Path       string
	SourcePath string
	Method     WorkspaceMethod
}

type commandRunner func(context.Context, string, []string) ([]byte, string, error)

type WorkspaceManager struct {
	method WorkspaceMethod
	runner commandRunner
}

func NewWorkspaceManager() *WorkspaceManager {
	return newWorkspaceManager(MethodGitWorktree, runCommand)
}

func newWorkspaceManager(method WorkspaceMethod, runner commandRunner) *WorkspaceManager {
	return &WorkspaceManager{
		method: method,
		runner: runner,
	}
}

func (m *WorkspaceManager) Prepare(ctx context.Context, spec WorkspaceSpec) (*Workspace, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	runID := strings.TrimSpace(spec.RunID)
	if runID == "" {
		return nil, errors.New("run id is required")
	}

	sourcePath := strings.TrimSpace(spec.SourcePath)
	if sourcePath == "" {
		sourcePath = "."
	}
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("resolve source workspace: %w", err)
	}

	rootDir := strings.TrimSpace(spec.RootDir)
	if rootDir == "" {
		return nil, errors.New("workspace root dir is required")
	}
	rootDir, err = filepath.Abs(rootDir)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root dir: %w", err)
	}

	workspacePath := filepath.Join(rootDir, runID, "workspace")
	if samePath(sourcePath, workspacePath) {
		return nil, errors.New("isolated workspace path must differ from source workspace")
	}

	switch m.method {
	case MethodGitWorktree:
		if err := m.prepareGitWorktree(ctx, sourcePath, workspacePath); err != nil {
			return nil, err
		}
	case MethodCopy:
		return nil, errors.New("copy workspace method is not implemented")
	default:
		return nil, fmt.Errorf("unsupported workspace method %q", m.method)
	}

	return &Workspace{
		Path:       workspacePath,
		SourcePath: sourcePath,
		Method:     m.method,
	}, nil
}

func (m *WorkspaceManager) prepareGitWorktree(ctx context.Context, sourcePath string, workspacePath string) error {
	if err := os.MkdirAll(filepath.Dir(workspacePath), 0o755); err != nil {
		return fmt.Errorf("create workspace parent: %w", err)
	}

	_, stderr, err := m.runner(ctx, "git", []string{
		"-C", sourcePath,
		"worktree", "add",
		"--detach",
		workspacePath,
		"HEAD",
	})
	if err != nil {
		message := strings.TrimSpace(stderr)
		if message != "" {
			return fmt.Errorf("create git worktree workspace: %w: %s", err, message)
		}
		return fmt.Errorf("create git worktree workspace: %w", err)
	}
	return nil
}

func runCommand(ctx context.Context, command string, args []string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, command, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

func samePath(a string, b string) bool {
	rel, err := filepath.Rel(a, b)
	return err == nil && rel == "."
}
