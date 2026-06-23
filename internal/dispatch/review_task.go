package dispatch

import (
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func BuildInsightReviewTask(parentRunID string, evidence insights.EvidenceBundle, reviewer agents.Agent) (tasks.Task, error) {
	parentRunID = strings.TrimSpace(parentRunID)
	if parentRunID == "" {
		return tasks.Task{}, fmt.Errorf("parent run id is required")
	}
	taskID := "insight-review-" + parentRunID
	return tasks.Task{
		ID:     taskID,
		Title:  "Insight review for run " + parentRunID,
		Domain: "general",
		Worker: reviewer.DefaultWorker,
		Goal:   "Review the evidence bundle and produce an insight report with learning proposals. Do not modify project files.",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{
			"artifacts/**",
		},
		ForbiddenPaths: []string{
			"mysecondbrain/**",
			"secrets/**",
			".env",
		},
		ExpectedOutputs: []string{
			"artifacts/insight-report.json",
		},
		DefinitionOfDone: []string{
			"insight report artifact produced",
		},
	}, nil
}
