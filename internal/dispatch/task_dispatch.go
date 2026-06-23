package dispatch

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TaskFromSnapshot(snapshot agents.WorkItemTaskSnapshot) (tasks.Task, error) {
	if err := agents.ValidateWorkItemTaskSnapshot(snapshot); err != nil {
		return tasks.Task{}, err
	}
	var task tasks.Task
	if err := json.Unmarshal([]byte(snapshot.TaskJSON), &task); err != nil {
		return tasks.Task{}, fmt.Errorf("decode task snapshot: %w", err)
	}
	if err := tasks.Validate(&task); err != nil {
		return tasks.Task{}, err
	}
	return task, nil
}

func ValidateDispatchableWorkItem(item agents.WorkItem, leaseID string) error {
	switch item.Status {
	case agents.WorkItemStatusQueued, agents.WorkItemStatusLeased:
	default:
		return fmt.Errorf("work item %q status %q is not dispatchable", item.ID, item.Status)
	}
	if strings.TrimSpace(item.AssignedAgentID) == "" {
		return fmt.Errorf("work item %q has no assigned agent", item.ID)
	}
	return nil
}

func SelectWorkerRunner(agent agents.Agent, codexRunner, openCodeRunner WorkerRunner) (string, WorkerRunner, error) {
	worker := strings.TrimSpace(agent.DefaultWorker)
	switch worker {
	case "codex":
		if codexRunner == nil {
			return "", nil, fmt.Errorf("codex runner is not configured")
		}
		return worker, codexRunner, nil
	case "opencode":
		if openCodeRunner == nil {
			return "", nil, fmt.Errorf("opencode runner is not configured")
		}
		return worker, openCodeRunner, nil
	default:
		return "", nil, fmt.Errorf("unsupported worker %q", worker)
	}
}
