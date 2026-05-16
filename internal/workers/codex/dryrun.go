package codex

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/workers"
)

type Worker struct{}

func New() *Worker {
	return &Worker{}
}

func (w *Worker) DryRun(ctx context.Context, spec workers.RunSpec) (*workers.WorkerEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.Task == nil {
		return nil, errors.New("task is required")
	}

	sandbox, err := sandboxForMode(spec.Task.Mode)
	if err != nil {
		return nil, err
	}

	workspace := strings.TrimSpace(spec.Workspace)
	if workspace == "" {
		workspace = strings.TrimSpace(spec.Task.Workspace.Path)
	}
	if workspace == "" {
		return nil, errors.New("workspace is required")
	}

	command := []string{"codex", "exec", "--json", "--sandbox", sandbox, "--cd", workspace, "-"}
	return &workers.WorkerEvent{
		Type:      workers.EventDryRunPlanned,
		Worker:    "codex",
		Command:   command,
		Workspace: workspace,
		Sandbox:   sandbox,
	}, nil
}

func sandboxForMode(mode string) (string, error) {
	switch mode {
	case "read_only":
		return "read-only", nil
	case "workspace_write":
		return "workspace-write", nil
	default:
		return "", fmt.Errorf("unsupported task mode %q", mode)
	}
}
