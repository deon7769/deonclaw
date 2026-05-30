package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/workers"
)

type Worker struct {
	command string
}

func New() *Worker {
	return &Worker{command: "opencode"}
}

func NewWithCommand(command string) *Worker {
	command = strings.TrimSpace(command)
	if command == "" {
		command = "opencode"
	}
	return &Worker{command: command}
}

func (w *Worker) DryRun(ctx context.Context, spec workers.RunSpec) (*workers.WorkerEvent, error) {
	return w.plan(ctx, spec)
}

func (w *Worker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	plan, err := w.plan(ctx, spec)
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, plan.Command[0], plan.Command[1:]...)
	cmd.Dir = plan.Workspace
	cmd.Stdin = strings.NewReader(workerPrompt(spec))

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	stdoutBytes := stdout.Bytes()
	result := &workers.RunResult{
		Worker:    "opencode",
		Command:   plan.Command,
		Workspace: plan.Workspace,
		Sandbox:   plan.Sandbox,
		Events:    eventsFromStdout(stdoutBytes, plan.Command, plan.Workspace),
		Artifacts: stdoutArtifacts(stdoutBytes),
		Stderr:    stderr.String(),
	}
	if runErr != nil {
		return result, fmt.Errorf("opencode command failed: %w", runErr)
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

func workerPrompt(spec workers.RunSpec) string {
	if strings.TrimSpace(spec.Prompt) != "" {
		return spec.Prompt
	}
	if spec.Task == nil {
		return ""
	}
	return spec.Task.Goal
}

func eventsFromStdout(stdout []byte, command []string, workspace string) []workers.WorkerEvent {
	lines := bytes.Split(stdout, []byte("\n"))
	events := make([]workers.WorkerEvent, 0, len(lines))
	for _, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if !json.Valid(line) {
			continue
		}
		events = append(events, workers.WorkerEvent{
			Type:      workers.EventStdoutJSON,
			Worker:    "opencode",
			Command:   append([]string(nil), command...),
			Workspace: workspace,
			Payload:   append([]byte(nil), line...),
		})
	}
	if len(events) > 0 {
		return events
	}
	payload, err := json.Marshal(map[string]string{
		"type": "worker.stdout.log",
		"text": string(stdout),
	})
	if err != nil {
		return nil
	}
	return []workers.WorkerEvent{
		{
			Type:      "worker.stdout.log",
			Worker:    "opencode",
			Command:   append([]string(nil), command...),
			Workspace: workspace,
			Payload:   payload,
		},
	}
}

func stdoutArtifacts(stdout []byte) []artifacts.Artifact {
	if stdoutIsJSONL(stdout) {
		return []artifacts.Artifact{{Path: "stdout.jsonl", Kind: artifacts.KindEvents, Content: append([]byte(nil), stdout...)}}
	}
	return []artifacts.Artifact{{Path: "stdout.log", Kind: artifacts.KindLog, Content: append([]byte(nil), stdout...)}}
}

func stdoutIsJSONL(stdout []byte) bool {
	hasLine := false
	for _, line := range bytes.Split(stdout, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		hasLine = true
		if !json.Valid(line) {
			return false
		}
	}
	return hasLine
}
