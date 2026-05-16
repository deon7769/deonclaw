package workers

import (
	"context"
	"encoding/json"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const EventDryRunPlanned = "worker.dry_run.planned"
const EventStdoutJSON = "worker.stdout.json"

type RunSpec struct {
	Task      *tasks.Task
	Workspace string
	Prompt    string
}

type WorkerEvent struct {
	Type      string          `json:"type"`
	Worker    string          `json:"worker"`
	Command   []string        `json:"command,omitempty"`
	Workspace string          `json:"workspace,omitempty"`
	Sandbox   string          `json:"sandbox,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type RunResult struct {
	Worker    string               `json:"worker"`
	Command   []string             `json:"command"`
	Workspace string               `json:"workspace"`
	Sandbox   string               `json:"sandbox"`
	Events    []WorkerEvent        `json:"events"`
	Artifacts []artifacts.Artifact `json:"artifacts,omitempty"`
	Stderr    string               `json:"stderr,omitempty"`
}

type Worker interface {
	DryRun(context.Context, RunSpec) (*WorkerEvent, error)
	Run(context.Context, RunSpec) (*RunResult, error)
}
