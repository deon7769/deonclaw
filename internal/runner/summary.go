package runner

import (
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func codexRunSummary(workerName string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup, validation ValidationResult, contextPackWarnings []string, memoryProposal memoryProposalCheck, artifactCount int) []byte {
	workspace := result.Workspace
	if workspace == "" {
		workspace = task.Workspace.Path
	}
	if workerName == "" {
		workerName = result.Worker
	}

	lines := []string{
		fmt.Sprintf("# %s run %s", workerName, runID),
		"",
		fmt.Sprintf("Status: %s", status),
		fmt.Sprintf("Task: %s", task.ID),
		fmt.Sprintf("Worker: %s", workerName),
		fmt.Sprintf("Workspace: %s", workspace),
		fmt.Sprintf("Command: %s", strings.Join(result.Command, " ")),
		fmt.Sprintf("Events: %d", len(result.Events)),
		fmt.Sprintf("Policy: %s", policySummary),
		fmt.Sprintf("Changed paths: %d", changedPathCount),
		fmt.Sprintf("Validation: %s", validation.Status),
		fmt.Sprintf("Validation commands: %d", validation.CommandCount),
		fmt.Sprintf("Memory proposal: %s", memoryProposal.Status),
		fmt.Sprintf("Memory proposal violations: %d", len(memoryProposal.Violations)),
		fmt.Sprintf("Memory proposal warnings: %d", len(memoryProposal.Warnings)),
		fmt.Sprintf("Artifacts: %d", artifactCount),
		fmt.Sprintf("Workspace cleanup: %s", cleanup.Action),
		fmt.Sprintf("Cleanup reason: %s", cleanup.Reason),
	}
	if validation.Error != "" {
		lines = append(lines, fmt.Sprintf("Validation error: %s", validation.Error))
	}
	if memoryProposal.ProposalID != "" {
		lines = append(lines, fmt.Sprintf("Memory proposal id: %s", memoryProposal.ProposalID))
	}
	lines = appendSmallDetails(lines, "Memory proposal violation", memoryProposal.Violations)
	lines = appendSmallDetails(lines, "Memory proposal warning", memoryProposal.Warnings)
	if len(contextPackWarnings) > 0 {
		lines = append(lines, fmt.Sprintf("Context pack warnings: %d", len(contextPackWarnings)))
		for _, warning := range contextPackWarnings {
			lines = append(lines, fmt.Sprintf("Context pack warning: %s", warning))
		}
	}
	if runErr != nil {
		lines = append(lines, fmt.Sprintf("Error: %v", runErr))
	}
	if cleanup.Warning != "" {
		lines = append(lines, fmt.Sprintf("Cleanup warning: %s", cleanup.Warning))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func appendSmallDetails(lines []string, label string, details []string) []string {
	const maxSummaryDetails = 5
	if len(details) > maxSummaryDetails {
		return lines
	}
	for _, detail := range details {
		lines = append(lines, fmt.Sprintf("%s: %s", label, detail))
	}
	return lines
}
