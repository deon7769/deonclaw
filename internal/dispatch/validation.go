package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/runner"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func runDispatchValidation(ctx context.Context, opts OnceOptions, task tasks.Task) (runner.ValidationResult, []artifacts.Artifact, error) {
	if len(task.Validation.Commands) == 0 {
		return runner.ValidationResult{}, nil, nil
	}
	validationRunner := opts.ValidationRunner
	if validationRunner == nil {
		if strings.TrimSpace(task.Validation.Runtime) != "" && task.Validation.Runtime != tasks.ValidationRuntimeLocal {
			result := runner.ValidationResult{
				Status:       runner.ValidationFailed,
				Runtime:      task.Validation.Runtime,
				CommandCount: len(task.Validation.Commands),
				Commands:     []runner.ValidationCommandResult{},
				Error:        fmt.Sprintf("dispatch validation runtime %q is not supported", task.Validation.Runtime),
			}
			validationArtifacts, artifactErr := dispatchValidationArtifacts(result)
			if artifactErr != nil {
				return result, nil, artifactErr
			}
			return result, validationArtifacts, dispatchValidationFailureError(result)
		}
		validationRunner = runner.RunValidationCommands
	}
	result := validationRunner(ctx, task.Workspace.Path, task.Validation.Commands)
	if strings.TrimSpace(result.Runtime) == "" {
		result.Runtime = tasks.ValidationRuntimeLocal
	}
	validationArtifacts, err := dispatchValidationArtifacts(result)
	if err != nil {
		return result, nil, err
	}
	return result, validationArtifacts, dispatchValidationFailureError(result)
}

func dispatchValidationFailureError(result runner.ValidationResult) error {
	if result.Status != runner.ValidationFailed {
		return nil
	}
	if strings.TrimSpace(result.Error) == "" {
		return errors.New("validation failed")
	}
	return errors.New(result.Error)
}

func dispatchValidationArtifacts(result runner.ValidationResult) ([]artifacts.Artifact, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return []artifacts.Artifact{
		{Path: "validation/validation.json", Kind: artifacts.KindOther, Content: data},
		{Path: "validation/validation.log", Kind: artifacts.KindLog, Content: dispatchValidationLog(result)},
	}, nil
}

func dispatchValidationLog(result runner.ValidationResult) []byte {
	var output strings.Builder
	output.WriteString("Validation: ")
	output.WriteString(result.Status)
	output.WriteByte('\n')
	if result.Runtime != "" {
		output.WriteString("Runtime: ")
		output.WriteString(result.Runtime)
		output.WriteByte('\n')
	}
	output.WriteString("Validation commands: ")
	output.WriteString(strconv.Itoa(result.CommandCount))
	output.WriteByte('\n')
	if result.SkippedReason != "" {
		output.WriteString("Skipped reason: ")
		output.WriteString(result.SkippedReason)
		output.WriteByte('\n')
	}
	if result.Error != "" {
		output.WriteString("Validation error: ")
		output.WriteString(result.Error)
		output.WriteByte('\n')
	}
	for _, command := range result.Commands {
		output.WriteByte('\n')
		output.WriteString("Command: ")
		output.WriteString(command.Name)
		output.WriteByte('\n')
		output.WriteString("Exec: ")
		output.WriteString(strings.TrimSpace(command.Command + " " + strings.Join(command.Args, " ")))
		output.WriteByte('\n')
		output.WriteString("Status: ")
		output.WriteString(command.Status)
		output.WriteByte('\n')
		output.WriteString("Exit code: ")
		output.WriteString(strconv.Itoa(command.ExitCode))
		output.WriteByte('\n')
		if command.Error != "" {
			output.WriteString("Error: ")
			output.WriteString(command.Error)
			output.WriteByte('\n')
		}
	}
	return []byte(output.String())
}
