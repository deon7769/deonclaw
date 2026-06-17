package tasks

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/deon7769/deonclaw/internal/config"
)

const (
	FallbackRetryWorkerFailed       = "worker_failed"
	FallbackRetryValidationFailed   = "validation_failed"
	FallbackNeverRetryPolicyFailed  = "policy_failed"
	FallbackNeverRetryMemoryPolicy  = "memory_policy_failed"
	DefaultFallbackPolicyMaxAttempt = 1

	ValidationRuntimeLocal  = "local"
	ValidationRuntimeDocker = "docker"
)

var allowedFallbackRetryOn = []string{
	FallbackRetryWorkerFailed,
	FallbackRetryValidationFailed,
}

var allowedFallbackNeverRetryOn = []string{
	FallbackNeverRetryPolicyFailed,
	FallbackNeverRetryMemoryPolicy,
}

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
	if task.ModelStrategy != nil {
		errs = append(errs, validateFallbackPolicy(task.ModelStrategy.FallbackPolicy)...)
	}
	if task.Validation.Runtime != "" && task.Validation.Runtime != ValidationRuntimeLocal && task.Validation.Runtime != ValidationRuntimeDocker {
		errs = append(errs, fmt.Errorf("validation.runtime %q is not supported", task.Validation.Runtime))
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
	for i, attachment := range task.MCPContext.Attachments {
		prefix := fmt.Sprintf("mcp_context.attachments[%d]", i)
		if strings.TrimSpace(attachment.Name) == "" {
			errs = append(errs, fmt.Errorf("%s.name is required", prefix))
		}
		switch strings.TrimSpace(attachment.Kind) {
		case "discovery", "call":
		default:
			errs = append(errs, fmt.Errorf("%s.kind %q is not supported", prefix, attachment.Kind))
		}
		if strings.TrimSpace(attachment.Path) == "" {
			errs = append(errs, fmt.Errorf("%s.path is required", prefix))
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

func ValidateFallbackPolicy(policy *FallbackPolicy) error {
	return errors.Join(validateFallbackPolicy(policy)...)
}

func EffectiveFallbackPolicy(policy *FallbackPolicy) FallbackPolicy {
	effective := FallbackPolicy{
		Enabled:      false,
		MaxAttempts:  DefaultFallbackPolicyMaxAttempt,
		RetryOn:      append([]string(nil), allowedFallbackRetryOn...),
		NeverRetryOn: append([]string(nil), allowedFallbackNeverRetryOn...),
	}
	if policy == nil {
		return effective
	}
	effective.Enabled = policy.Enabled
	if policy.MaxAttempts != 0 {
		effective.MaxAttempts = policy.MaxAttempts
	}
	if len(policy.RetryOn) > 0 {
		effective.RetryOn = append([]string(nil), policy.RetryOn...)
	}
	if len(policy.NeverRetryOn) > 0 {
		effective.NeverRetryOn = append([]string(nil), policy.NeverRetryOn...)
	}
	return effective
}

func validateFallbackPolicy(policy *FallbackPolicy) []error {
	if policy == nil {
		return nil
	}
	var errs []error
	if policy.Enabled && policy.MaxAttempts <= 0 {
		errs = append(errs, errors.New("model_strategy.fallback_policy.max_attempts must be greater than zero when fallback_policy.enabled is true"))
	}
	for i, reason := range policy.RetryOn {
		if !slices.Contains(allowedFallbackRetryOn, reason) {
			errs = append(errs, fmt.Errorf("model_strategy.fallback_policy.retry_on[%d] %q is not supported", i, reason))
		}
	}
	for i, reason := range policy.NeverRetryOn {
		if !slices.Contains(allowedFallbackNeverRetryOn, reason) {
			errs = append(errs, fmt.Errorf("model_strategy.fallback_policy.never_retry_on[%d] %q is not supported", i, reason))
		}
	}
	return errs
}
