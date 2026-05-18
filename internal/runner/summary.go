package runner

import (
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func codexRunSummary(runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup) []byte {
	workspace := result.Workspace
	if workspace == "" {
		workspace = task.Workspace.Path
	}

	lines := []string{
		fmt.Sprintf("# Codex run %s", runID),
		"",
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Task: %s", task.ID),
		"Worker: codex",
		fmt.Sprintf("Workspace: %s", workspace),
		fmt.Sprintf("Command: %s", strings.Join(result.Command, " ")),
		fmt.Sprintf("Events: %d", len(result.Events)),
		fmt.Sprintf("Policy: %s", policySummary),
		fmt.Sprintf("Changed paths: %d", changedPathCount),
		fmt.Sprintf("Workspace cleanup: %s", cleanup.Action),
		fmt.Sprintf("Cleanup reason: %s", cleanup.Reason),
	}
	if runErr != nil {
		lines = append(lines, fmt.Sprintf("Error: %v", runErr))
	}
	if cleanup.Warning != "" {
		lines = append(lines, fmt.Sprintf("Cleanup warning: %s", cleanup.Warning))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
