package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/deon7769/deonclaw/internal/workers"
)

type commandRunner func(context.Context, string, []string, string) ([]byte, string, error)

type Worker struct {
	command string
	runner  commandRunner
}

func New() *Worker {
	return newWithRunner("codex", runCommand)
}

func newWithRunner(command string, runner commandRunner) *Worker {
	return &Worker{
		command: command,
		runner:  runner,
	}
}

func (w *Worker) DryRun(ctx context.Context, spec workers.RunSpec) (*workers.WorkerEvent, error) {
	return w.plan(ctx, spec)
}

func (w *Worker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	plan, err := w.plan(ctx, spec)
	if err != nil {
		return nil, err
	}

	prompt := spec.Prompt
	if prompt == "" {
		prompt = spec.Task.Goal
	}

	stdout, stderr, runErr := w.runner(ctx, plan.Command[0], plan.Command[1:], prompt)
	events, parseErr := parseStdoutJSONL(stdout)
	result := &workers.RunResult{
		Worker:    "codex",
		Command:   plan.Command,
		Workspace: plan.Workspace,
		Sandbox:   plan.Sandbox,
		Events:    events,
		Stderr:    stderr,
	}
	if parseErr != nil {
		return result, parseErr
	}
	if runErr != nil {
		return result, fmt.Errorf("run codex command: %w", runErr)
	}
	return result, nil
}

func (w *Worker) plan(ctx context.Context, spec workers.RunSpec) (*workers.WorkerEvent, error) {
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

	command := []string{w.command, "exec", "--json", "--sandbox", sandbox, "--cd", workspace, "-"}
	return &workers.WorkerEvent{
		Type:      workers.EventDryRunPlanned,
		Worker:    "codex",
		Command:   command,
		Workspace: workspace,
		Sandbox:   sandbox,
	}, nil
}

func runCommand(ctx context.Context, command string, args []string, prompt string) ([]byte, string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Stdin = strings.NewReader(prompt)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.String(), err
}

func parseStdoutJSONL(stdout []byte) ([]workers.WorkerEvent, error) {
	var events []workers.WorkerEvent
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			return events, fmt.Errorf("parse codex stdout jsonl line %d: invalid JSON", lineNumber)
		}
		payload := append([]byte(nil), line...)
		events = append(events, workers.WorkerEvent{
			Type:    workers.EventStdoutJSON,
			Worker:  "codex",
			Payload: json.RawMessage(payload),
		})
	}
	if err := scanner.Err(); err != nil {
		return events, fmt.Errorf("scan codex stdout jsonl: %w", err)
	}
	return events, nil
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
