package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"github.com/deon7769/deonclaw/internal/workers"
)

const (
	WorkerRuntimeLocal  = "local"
	WorkerRuntimeDocker = "docker"

	maxDockerWorkerJSONLLineSize = 10 * 1024 * 1024
)

func runWorkerWithRuntime(ctx context.Context, worker workers.Worker, spec workers.RunSpec, runtimeName string, runtimeConfig *runtimeconfig.Config) (*workers.RunResult, error) {
	switch strings.TrimSpace(runtimeName) {
	case "", WorkerRuntimeLocal:
		return worker.Run(ctx, spec)
	case WorkerRuntimeDocker:
		if runtimeConfig == nil {
			return nil, errors.New("worker runtime docker requires --runtime-config")
		}
		return runDockerWorker(ctx, worker, spec, *runtimeConfig)
	default:
		return nil, fmt.Errorf("worker runtime %q is not supported", runtimeName)
	}
}

func normalizeWorkerRuntime(runtimeName string) (string, error) {
	switch strings.TrimSpace(runtimeName) {
	case "", WorkerRuntimeLocal:
		return WorkerRuntimeLocal, nil
	case WorkerRuntimeDocker:
		return WorkerRuntimeDocker, nil
	default:
		return "", fmt.Errorf("worker runtime %q is not supported", runtimeName)
	}
}

func runDockerWorker(ctx context.Context, worker workers.Worker, spec workers.RunSpec, cfg runtimeconfig.Config) (*workers.RunResult, error) {
	planned, err := worker.DryRun(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("plan docker worker command: %w", err)
	}
	if planned == nil || len(planned.Command) == 0 || strings.TrimSpace(planned.Command[0]) == "" {
		return nil, errors.New("docker worker command must not be empty")
	}

	plan, err := runtimeconfig.PlanDockerExec(cfg, spec.Workspace, planned.Command)
	result := &workers.RunResult{
		Worker:    firstNonEmpty(planned.Worker, workerNameFromSpec(spec)),
		Command:   append([]string(nil), planned.Command...),
		Workspace: spec.Workspace,
		Sandbox:   planned.Sandbox,
	}
	if err != nil {
		return result, err
	}
	result.Command = append([]string(nil), plan.Command...)

	prompt := spec.Prompt
	if strings.TrimSpace(prompt) == "" && spec.Task != nil {
		prompt = spec.Task.Goal
	}

	cmd := exec.CommandContext(ctx, plan.Command[0], plan.Command[1:]...)
	cmd.Dir = spec.Workspace
	cmd.Stdin = strings.NewReader(prompt)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now().UTC()
	runErr := cmd.Run()
	result.Events = parseDockerWorkerEvents(stdout.Bytes(), result.Worker, result.Command, spec.Workspace, result.Sandbox)
	result.Artifacts = dockerWorkerArtifacts(stdout.Bytes(), stderr.String(), started)
	result.Stderr = stderr.String()
	if runErr != nil {
		return result, fmt.Errorf("docker worker command failed with exit code %d", commandExitCode(runErr))
	}
	return result, nil
}

func parseDockerWorkerEvents(stdout []byte, workerName string, command []string, workspace string, sandbox string) []workers.WorkerEvent {
	var events []workers.WorkerEvent
	scanner := bufio.NewScanner(bytes.NewReader(stdout))
	scanner.Buffer(make([]byte, 64*1024), maxDockerWorkerJSONLLineSize)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || !json.Valid(line) {
			continue
		}
		payload := append([]byte(nil), line...)
		events = append(events, workers.WorkerEvent{
			Type:      eventTypeFromPayload(payload),
			Worker:    workerName,
			Command:   append([]string(nil), command...),
			Workspace: workspace,
			Sandbox:   sandbox,
			Payload:   json.RawMessage(payload),
		})
	}
	return events
}

func dockerWorkerArtifacts(stdout []byte, stderr string, createdAt time.Time) []artifacts.Artifact {
	return []artifacts.Artifact{
		{
			ID:        "stdout",
			Path:      "artifacts/stdout.jsonl",
			Kind:      artifacts.KindEvents,
			Content:   append([]byte(nil), stdout...),
			CreatedAt: createdAt,
		},
		{
			ID:        "stderr",
			Path:      "artifacts/stderr.log",
			Kind:      artifacts.KindLog,
			Content:   []byte(stderr),
			CreatedAt: createdAt,
		},
	}
}

func eventTypeFromPayload(payload []byte) string {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return workers.EventStdoutJSON
	}
	if strings.TrimSpace(envelope.Type) == "" {
		return workers.EventStdoutJSON
	}
	return envelope.Type
}

func workerNameFromSpec(spec workers.RunSpec) string {
	if spec.Task == nil {
		return ""
	}
	return strings.TrimSpace(spec.Task.Worker)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
