package tasks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deon7769/deonclaw/internal/config"
)

func Validate(task *Task) error {
	if task == nil {
		return errors.New("task is nil")
	}

	var errs []error

	requireNonEmpty := func(field, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}

	requireNonEmpty("id", task.ID)
	requireNonEmpty("title", task.Title)
	requireNonEmpty("domain", task.Domain)
	requireNonEmpty("worker", task.Worker)
	requireNonEmpty("goal", task.Goal)
	requireNonEmpty("mode", task.Mode)
	requireNonEmpty("workspace.strategy", task.Workspace.Strategy)
	requireNonEmpty("workspace.path", task.Workspace.Path)
	requireNonEmpty("memory.scope", task.Memory.Scope)

	if task.Domain != "" && !config.IsKnownDomain(task.Domain) {
		errs = append(errs, fmt.Errorf("domain %q is not supported", task.Domain))
	}
	if task.Worker != "" && !config.IsKnownWorker(task.Worker) {
		errs = append(errs, fmt.Errorf("worker %q is not supported", task.Worker))
	}
	if task.Mode != "" && !config.IsKnownMode(task.Mode) {
		errs = append(errs, fmt.Errorf("mode %q is not supported", task.Mode))
	}
	if task.Mode == "workspace_write" && len(task.AllowedPaths) == 0 {
		errs = append(errs, errors.New("allowed_paths must not be empty for workspace_write mode"))
	}
	if task.Workspace.Strategy != "" && !config.IsKnownWorkspaceStrategy(task.Workspace.Strategy) {
		errs = append(errs, fmt.Errorf("workspace.strategy %q is not supported", task.Workspace.Strategy))
	}
	if task.Memory.Scope != "" && !config.IsKnownMemoryScope(task.Memory.Scope) {
		errs = append(errs, fmt.Errorf("memory.scope %q is not supported", task.Memory.Scope))
	}
	if task.ModelProfile != "" && task.ModelStrategy != nil {
		errs = append(errs, errors.New("model_profile and model_strategy are mutually exclusive"))
	}
	if task.ModelStrategy != nil && len(task.ModelStrategy.Preferred) == 0 {
		errs = append(errs, errors.New("model_strategy.preferred must not be empty"))
	}
	for i, command := range task.Validation.Commands {
		prefix := fmt.Sprintf("validation.commands[%d]", i)
		if strings.TrimSpace(command.Name) == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", prefix))
		}
		if strings.TrimSpace(command.Command) == "" {
			errs = append(errs, fmt.Errorf("%s.command is required", prefix))
		}
		if command.TimeoutSeconds < 0 {
			errs = append(errs, fmt.Errorf("%s.timeout_seconds must not be negative", prefix))
		}
	}
	if len(task.ForbiddenPaths) == 0 {
		errs = append(errs, errors.New("forbidden_paths must not be empty"))
	}
	if len(task.ExpectedOutputs) == 0 {
		errs = append(errs, errors.New("expected_outputs must not be empty"))
	}
	if len(task.DefinitionOfDone) == 0 {
		errs = append(errs, errors.New("definition_of_done must not be empty"))
	}

	return errors.Join(errs...)
}
