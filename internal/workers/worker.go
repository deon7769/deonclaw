package workers

import (
	"context"

	"github.com/deon7769/deonclaw/internal/tasks"
)

const EventDryRunPlanned = "worker.dry_run.planned"

type RunSpec struct {
	Task      *tasks.Task
	Workspace string
}

type WorkerEvent struct {
	Type      string   `json:"type"`
	Worker    string   `json:"worker"`
	Command   []string `json:"command"`
	Workspace string   `json:"workspace"`
	Sandbox   string   `json:"sandbox"`
}

type Worker interface {
	DryRun(context.Context, RunSpec) (*WorkerEvent, error)
}
