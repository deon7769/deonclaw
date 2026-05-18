package runner

import (
	"context"
	"fmt"

	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
)

type workspaceCleanup struct {
	Action  string
	Reason  runs.RunStatus
	Warning string
}

func cleanupWorkspace(ctx context.Context, manager WorkspacePreparer, workspace *runtime.Workspace, status runs.RunStatus) workspaceCleanup {
	cleanup := workspaceCleanup{
		Action: "kept",
		Reason: status,
	}
	if status != runs.StatusSucceeded {
		return cleanup
	}

	if err := manager.Cleanup(ctx, workspace); err != nil {
		cleanup.Warning = fmt.Sprintf("workspace cleanup failed: %v", err)
		return cleanup
	}
	cleanup.Action = "removed"
	return cleanup
}
