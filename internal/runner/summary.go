package runner

import (
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func codexRunSummary(workerName string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup, validation ValidationResult, workerRuntime string, contextPackWarnings []string, mcpContextCount int, mcpToolProposal mcpToolProposalCheck, memoryProposal memoryProposalCheck, artifactCount int) []byte {
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
		fmt.Sprintf("Worker runtime: %s", workerRuntimeForSummary(workerRuntime)),
		fmt.Sprintf("Workspace: %s", workspace),
		fmt.Sprintf("Command: %s", strings.Join(result.Command, " ")),
		fmt.Sprintf("Events: %d", len(result.Events)),
		fmt.Sprintf("Policy: %s", policySummary),
		fmt.Sprintf("Changed paths: %d", changedPathCount),
		fmt.Sprintf("Validation: %s", validation.Status),
		fmt.Sprintf("Validation runtime: %s", validationRuntimeForSummary(validation)),
		fmt.Sprintf("Validation commands: %d", validation.CommandCount),
		fmt.Sprintf("MCP tool proposal: %s", mcpToolProposalStatusForSummary(mcpToolProposal)),
		fmt.Sprintf("Memory proposal: %s", memoryProposal.Status),
		fmt.Sprintf("Memory proposal violations: %d", len(memoryProposal.Violations)),
		fmt.Sprintf("Memory proposal warnings: %d", len(memoryProposal.Warnings)),
		fmt.Sprintf("Artifacts: %d", artifactCount),
		"Execution trace: execution-trace.json",
		fmt.Sprintf("Workspace cleanup: %s", cleanup.Action),
		fmt.Sprintf("Cleanup reason: %s", cleanup.Reason),
	}
	if validation.Error != "" {
		lines = append(lines, fmt.Sprintf("Validation error: %s", validation.Error))
	}
	if memoryProposal.ProposalID != "" {
		lines = append(lines, fmt.Sprintf("Memory proposal id: %s", memoryProposal.ProposalID))
	}
	if mcpToolProposal.ProposalSHA256 != "" {
		lines = append(lines, fmt.Sprintf("MCP tool proposal sha256: %s", mcpToolProposal.ProposalSHA256))
	}
	if workerName == "opencode" {
		lines = append(lines, opencodeStdoutSummaryLines(result)...)
	}
	if mcpContextCount > 0 {
		lines = append(lines, fmt.Sprintf("MCP context attachments: %d", mcpContextCount))
	}
	lines = appendSmallDetails(lines, "MCP tool proposal violation", mcpToolProposal.Violations)
	lines = appendSmallDetails(lines, "MCP tool proposal warning", mcpToolProposal.Warnings)
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

func mcpToolProposalStatusForSummary(check mcpToolProposalCheck) string {
	if status := strings.TrimSpace(check.Status); status != "" {
		return status
	}
	return mcpToolProposalStatusNone
}

func workerRuntimeForSummary(workerRuntime string) string {
	if runtime := strings.TrimSpace(workerRuntime); runtime != "" {
		return runtime
	}
	return WorkerRuntimeLocal
}

func validationRuntimeForSummary(validation ValidationResult) string {
	if runtime := strings.TrimSpace(validation.Runtime); runtime != "" {
		return runtime
	}
	return tasks.ValidationRuntimeLocal
}

func opencodeStdoutSummaryLines(result *workers.RunResult) []string {
	metadata := result.Metadata
	lines := modelProfileSummaryLines(metadata)
	stdoutFormat := metadata["opencode.stdout_format"]
	if stdoutFormat == "" {
		stdoutFormat = "unknown"
	}
	parsedEvents := metadata["opencode.parsed_events"]
	if parsedEvents == "" {
		parsedEvents = "0"
	}
	parseWarnings := metadata["opencode.parse_warnings"]
	if parseWarnings == "" {
		parseWarnings = "0"
	}
	lines = append(lines,
		fmt.Sprintf("OpenCode stdout format: %s", stdoutFormat),
		fmt.Sprintf("OpenCode parsed events: %s", parsedEvents),
		fmt.Sprintf("OpenCode parse warnings: %s", parseWarnings),
	)
	return lines
}

func modelProfileSummaryLines(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	lines := []string{}
	if value := metadata["model_profile"]; value != "" {
		lines = append(lines, fmt.Sprintf("Model profile: %s", value))
	}
	if value := metadata["model_strategy"]; value != "" {
		lines = append(lines, fmt.Sprintf("Model strategy: %s", value))
	}
	if value := metadata["selected_model_profile"]; value != "" {
		lines = append(lines, fmt.Sprintf("Selected model profile: %s", value))
	}
	if value := metadata["provider"]; value != "" {
		lines = append(lines, fmt.Sprintf("Provider: %s", value))
	}
	if value := metadata["model"]; value != "" {
		lines = append(lines, fmt.Sprintf("Model: %s", value))
	}
	if value := metadata["model_arg"]; value != "" {
		lines = append(lines, fmt.Sprintf("Model arg: %s", value))
	}
	return lines
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
