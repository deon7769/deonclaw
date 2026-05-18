package runner

import (
	"context"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestRunValidationCommandsUsesWorkspaceAndCapturesOutput(t *testing.T) {
	workspace := t.TempDir()

	result := RunValidationCommands(context.Background(), workspace, []tasks.ValidationCommand{
		{
			Name:    "pwd",
			Command: "pwd",
		},
	})

	if result.Status != ValidationPassed {
		t.Fatalf("validation status = %q, want %q: %#v", result.Status, ValidationPassed, result)
	}
	if result.CommandCount != 1 {
		t.Fatalf("validation command_count = %d, want 1", result.CommandCount)
	}
	if len(result.Commands) != 1 {
		t.Fatalf("len(commands) = %d, want 1", len(result.Commands))
	}
	if strings.TrimSpace(result.Commands[0].Stdout) != workspace {
		t.Fatalf("validation stdout = %q, want workspace %q", result.Commands[0].Stdout, workspace)
	}
	if result.Commands[0].ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.Commands[0].ExitCode)
	}
	if result.Commands[0].TimeoutSeconds != defaultValidationTimeoutSeconds {
		t.Fatalf("timeout = %d, want default", result.Commands[0].TimeoutSeconds)
	}
}
