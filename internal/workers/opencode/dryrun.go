package opencode

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/workers"
)

type Worker struct {
	command string
}

func New() *Worker {
	return &Worker{command: "opencode"}
}

func (w *Worker) DryRun(ctx context.Context, spec workers.RunSpec) (*workers.WorkerEvent, error) {
	return w.plan(ctx, spec)
}

func (w *Worker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	plan, err := w.plan(ctx, spec)
	if err != nil {
		return nil, err
	}
	return &workers.RunResult{
		Worker:    "opencode",
		Command:   plan.Command,
		Workspace: plan.Workspace,
		Sandbox:   plan.Sandbox,
		Events:    []workers.WorkerEvent{*plan},
	}, errors.New("opencode run is not implemented")
}

func (w *Worker) plan(ctx context.Context, spec workers.RunSpec) (*workers.WorkerEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.Task == nil {
		return nil, errors.New("task is required")
	}

	policy, err := policyForMode(spec.Task.Mode)
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

	command := []string{w.command, "run", "--cwd", workspace, "-"}
	return &workers.WorkerEvent{
		Type:      workers.EventDryRunPlanned,
		Worker:    "opencode",
		Command:   command,
		Workspace: workspace,
		Sandbox:   policy,
	}, nil
}

func policyForMode(mode string) (string, error) {
	switch mode {
	case "read_only":
		return "read-only", nil
	case "workspace_write":
		return "workspace-write", nil
	default:
		return "", fmt.Errorf("unsupported task mode %q", mode)
	}
}
