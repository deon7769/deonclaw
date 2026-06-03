package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/workers"
)

type Worker struct {
	command string
}

const (
	stdoutFormatJSONL = "jsonl"
	stdoutFormatMixed = "mixed"
	stdoutFormatText  = "text"
	stdoutFormatEmpty = "empty"

	maxEventPayloadBytes = 64 * 1024
)

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

	cmd := exec.CommandContext(ctx, plan.Command[0], commandArgsWithPrompt(plan.Command[1:], workerPrompt(spec))...)
	cmd.Dir = plan.Workspace

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	stdoutBytes := stdout.Bytes()
	stderrBytes := []byte(stderr.String())
	stdoutAnalysis := analyzeStdout(stdoutBytes, plan.Command, plan.Workspace)
	result := &workers.RunResult{
		Worker:    "opencode",
		Command:   plan.Command,
		Workspace: plan.Workspace,
		Sandbox:   plan.Sandbox,
		Events:    stdoutAnalysis.Events,
		Artifacts: append(stdoutAnalysis.Artifacts, artifacts.Artifact{Path: "stderr.log", Kind: artifacts.KindLog, Content: stderrBytes}),
		Stderr:    string(stderrBytes),
		Metadata: map[string]string{
			"opencode.stdout_format":  stdoutAnalysis.Format,
			"opencode.parsed_events":  strconv.Itoa(stdoutAnalysis.ParsedEvents),
			"opencode.parse_warnings": strconv.Itoa(len(stdoutAnalysis.ParseWarnings)),
		},
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

	command := []string{w.command, "run", "--dir", workspace, "--format", "json", "<prompt>"}
	return &workers.WorkerEvent{
		Type:      workers.EventDryRunPlanned,
		Worker:    "opencode",
		Command:   command,
		Workspace: workspace,
		Sandbox:   policy,
	}, nil
}

func commandArgsWithPrompt(args []string, prompt string) []string {
	if len(args) == 0 {
		return nil
	}
	commandArgs := append([]string(nil), args...)
	for i, arg := range commandArgs {
		if arg == "<prompt>" {
			commandArgs[i] = prompt
		}
	}
	return commandArgs
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

type stdoutAnalysis struct {
	Format        string
	Events        []workers.WorkerEvent
	Artifacts     []artifacts.Artifact
	ParsedEvents  int
	ParseWarnings []string
}

func analyzeStdout(stdout []byte, command []string, workspace string) stdoutAnalysis {
	lines := bytes.Split(stdout, []byte("\n"))
	events := make([]workers.WorkerEvent, 0, len(lines))
	warnings := []string{}
	invalidLines := 0
	nonEmptyLines := 0
	for i, line := range lines {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		nonEmptyLines++
		if !json.Valid(line) {
			invalidLines++
			continue
		}
		events = append(events, jsonEventFromLine(line, i+1, command, workspace, &warnings))
	}

	format := stdoutFormatText
	if nonEmptyLines == 0 {
		format = stdoutFormatEmpty
	} else if invalidLines == 0 {
		format = stdoutFormatJSONL
	} else if len(events) > 0 {
		format = stdoutFormatMixed
		warnings = append(warnings, fmt.Sprintf("stdout contained %d non-JSON line(s) alongside %d JSON event line(s)", invalidLines, len(events)))
	} else {
		format = stdoutFormatText
	}

	parsedEvents := len(events)
	if format == stdoutFormatText {
		events = append(events, textEvent(stdout, command, workspace))
	}
	for _, warning := range warnings {
		events = append(events, warningEvent(warning, command, workspace))
	}
	return stdoutAnalysis{
		Format:        format,
		Events:        events,
		Artifacts:     stdoutArtifacts(format, stdout),
		ParsedEvents:  parsedEvents,
		ParseWarnings: warnings,
	}
}

func jsonEventFromLine(line []byte, lineNumber int, command []string, workspace string, warnings *[]string) workers.WorkerEvent {
	payload := append([]byte(nil), line...)
	eventType := eventTypeFromPayload(payload)
	if len(payload) > maxEventPayloadBytes {
		*warnings = append(*warnings, fmt.Sprintf("stdout JSON line %d exceeded event payload limit and was truncated in events", lineNumber))
		payload = safeTextPayload("worker.stdout.json.truncated", payload)
		eventType = "worker.stdout.json.truncated"
	}
	return workers.WorkerEvent{
		Type:      eventType,
		Worker:    "opencode",
		Command:   append([]string(nil), command...),
		Workspace: workspace,
		Payload:   payload,
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

func textEvent(stdout []byte, command []string, workspace string) workers.WorkerEvent {
	return workers.WorkerEvent{
		Type:      "worker.stdout.log",
		Worker:    "opencode",
		Command:   append([]string(nil), command...),
		Workspace: workspace,
		Payload:   safeTextPayload("worker.stdout.log", stdout),
	}
}

func warningEvent(warning string, command []string, workspace string) workers.WorkerEvent {
	payload, err := json.Marshal(map[string]string{
		"type":    "worker.stdout.parse_warning",
		"warning": warning,
	})
	if err != nil {
		payload = []byte(`{"type":"worker.stdout.parse_warning","warning":"marshal failed"}`)
	}
	return workers.WorkerEvent{
		Type:      "worker.stdout.parse_warning",
		Worker:    "opencode",
		Command:   append([]string(nil), command...),
		Workspace: workspace,
		Payload:   payload,
	}
}

func safeTextPayload(eventType string, content []byte) []byte {
	preview := content
	truncated := false
	previewLimit := maxEventPayloadBytes / 2
	if len(preview) > previewLimit {
		preview = preview[:previewLimit]
		truncated = true
	}
	payload, err := json.Marshal(struct {
		Type          string `json:"type"`
		Text          string `json:"text"`
		Truncated     bool   `json:"truncated"`
		OriginalBytes int    `json:"original_bytes"`
	}{
		Type:          eventType,
		Text:          string(preview),
		Truncated:     truncated,
		OriginalBytes: len(content),
	})
	if err != nil {
		return []byte(`{"type":"worker.stdout.log","text":"","truncated":true,"original_bytes":0}`)
	}
	return payload
}

func stdoutArtifacts(format string, stdout []byte) []artifacts.Artifact {
	if format == stdoutFormatJSONL {
		return []artifacts.Artifact{{Path: "stdout.jsonl", Kind: artifacts.KindEvents, Content: append([]byte(nil), stdout...)}}
	}
	return []artifacts.Artifact{{Path: "stdout.log", Kind: artifacts.KindLog, Content: append([]byte(nil), stdout...)}}
}
