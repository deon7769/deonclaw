package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"github.com/deon7769/deonclaw/internal/tasks"
)

const defaultValidationTimeoutSeconds = 300
const defaultValidationOutputLimitBytes = 1024 * 1024

const (
	ValidationPassed  = "passed"
	ValidationFailed  = "failed"
	ValidationSkipped = "skipped"
)

const validationTimedOut = "timed_out"

type ValidationRunner func(context.Context, string, []tasks.ValidationCommand) ValidationResult

type ValidationResult struct {
	Status        string                    `json:"status"`
	Runtime       string                    `json:"runtime,omitempty"`
	CommandCount  int                       `json:"command_count"`
	Commands      []ValidationCommandResult `json:"commands"`
	Error         string                    `json:"error,omitempty"`
	SkippedReason string                    `json:"skipped_reason,omitempty"`
}

type ValidationCommandResult struct {
	Name                string   `json:"name"`
	Runtime             string   `json:"runtime,omitempty"`
	Command             string   `json:"command"`
	Args                []string `json:"args,omitempty"`
	TimeoutSeconds      int      `json:"timeout_seconds"`
	ExitCode            int      `json:"exit_code"`
	DurationMS          int64    `json:"duration_ms"`
	Status              string   `json:"status"`
	Stdout              string   `json:"stdout"`
	Stderr              string   `json:"stderr"`
	StdoutTruncated     bool     `json:"stdout_truncated,omitempty"`
	StderrTruncated     bool     `json:"stderr_truncated,omitempty"`
	StdoutOriginalBytes int      `json:"stdout_original_bytes,omitempty"`
	StderrOriginalBytes int      `json:"stderr_original_bytes,omitempty"`
	Error               string   `json:"error,omitempty"`
}

func RunValidationCommands(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
	return runValidationCommands(ctx, workspace, commands, tasks.ValidationRuntimeLocal, runLocalValidationCommand)
}

func NewDockerValidationRunner(cfg runtimeconfig.Config) ValidationRunner {
	return func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
		return RunDockerValidationCommands(ctx, workspace, commands, cfg)
	}
}

func RunDockerValidationCommands(ctx context.Context, workspace string, commands []tasks.ValidationCommand, cfg runtimeconfig.Config) ValidationResult {
	if len(commands) == 0 {
		result := skippedValidation(commands, "no validation commands configured")
		result.Runtime = tasks.ValidationRuntimeDocker
		return result
	}
	if _, err := runtimeconfig.PlanDocker(cfg, workspace); err != nil {
		return ValidationResult{
			Status:       ValidationFailed,
			Runtime:      tasks.ValidationRuntimeDocker,
			CommandCount: len(commands),
			Commands:     []ValidationCommandResult{},
			Error:        fmt.Sprintf("docker validation runtime config invalid: %v", err),
		}
	}
	return runValidationCommands(ctx, workspace, commands, tasks.ValidationRuntimeDocker, func(ctx context.Context, workspace string, command tasks.ValidationCommand) ValidationCommandResult {
		return runDockerValidationCommand(ctx, workspace, command, cfg)
	})
}

type validationCommandRunner func(context.Context, string, tasks.ValidationCommand) ValidationCommandResult

func runValidationCommands(ctx context.Context, workspace string, commands []tasks.ValidationCommand, runtime string, runCommand validationCommandRunner) ValidationResult {
	if len(commands) == 0 {
		result := skippedValidation(commands, "no validation commands configured")
		result.Runtime = runtime
		return result
	}

	result := ValidationResult{
		Status:       ValidationPassed,
		Runtime:      runtime,
		CommandCount: len(commands),
		Commands:     make([]ValidationCommandResult, 0, len(commands)),
	}
	for _, command := range commands {
		commandResult := runCommand(ctx, workspace, command)
		commandResult.Runtime = runtime
		result.Commands = append(result.Commands, commandResult)
		if commandResult.Status != ValidationPassed {
			result.Status = ValidationFailed
			result.Error = validationCommandError(commandResult)
			return result
		}
	}
	return result
}

func skippedValidation(commands []tasks.ValidationCommand, reason string) ValidationResult {
	return ValidationResult{
		Status:        ValidationSkipped,
		CommandCount:  len(commands),
		Commands:      []ValidationCommandResult{},
		SkippedReason: reason,
	}
}

func runLocalValidationCommand(ctx context.Context, workspace string, command tasks.ValidationCommand) ValidationCommandResult {
	timeoutSeconds := command.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultValidationTimeoutSeconds
	}

	commandCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, command.Command, command.Args...)
	cmd.Dir = workspace

	stdout := newLimitedOutput(defaultValidationOutputLimitBytes)
	stderr := newLimitedOutput(defaultValidationOutputLimitBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	commandResult := ValidationCommandResult{
		Name:           command.Name,
		Command:        command.Command,
		Args:           append([]string(nil), command.Args...),
		TimeoutSeconds: timeoutSeconds,
		ExitCode:       0,
		DurationMS:     duration.Milliseconds(),
		Status:         ValidationPassed,
		Stdout:         stdout.String(),
		Stderr:         stderr.String(),
	}
	if stdout.Truncated() {
		commandResult.StdoutTruncated = true
		commandResult.StdoutOriginalBytes = stdout.OriginalBytes()
	}
	if stderr.Truncated() {
		commandResult.StderrTruncated = true
		commandResult.StderrOriginalBytes = stderr.OriginalBytes()
	}
	if err == nil {
		return commandResult
	}

	commandResult.Status = ValidationFailed
	commandResult.ExitCode = commandExitCode(err)
	commandResult.Error = err.Error()
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		commandResult.Status = validationTimedOut
		commandResult.ExitCode = -1
		commandResult.Error = fmt.Sprintf("validation command timed out after %d seconds", timeoutSeconds)
	}
	return commandResult
}

func runDockerValidationCommand(ctx context.Context, workspace string, command tasks.ValidationCommand, cfg runtimeconfig.Config) ValidationCommandResult {
	timeoutSeconds := command.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = defaultValidationTimeoutSeconds
	}

	commandResult := ValidationCommandResult{
		Name:           command.Name,
		Command:        command.Command,
		Args:           append([]string(nil), command.Args...),
		TimeoutSeconds: timeoutSeconds,
		ExitCode:       0,
		Status:         ValidationPassed,
	}

	execCommand := append([]string{command.Command}, command.Args...)
	plan, err := runtimeconfig.PlanDockerExec(cfg, workspace, execCommand)
	if err != nil {
		commandResult.Status = ValidationFailed
		commandResult.ExitCode = -1
		commandResult.Error = err.Error()
		return commandResult
	}

	commandCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(commandCtx, plan.Command[0], plan.Command[1:]...)
	cmd.Dir = workspace

	stdout := newLimitedOutput(defaultValidationOutputLimitBytes)
	stderr := newLimitedOutput(defaultValidationOutputLimitBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	commandResult.DurationMS = duration.Milliseconds()
	commandResult.Stdout = stdout.String()
	commandResult.Stderr = stderr.String()
	if stdout.Truncated() {
		commandResult.StdoutTruncated = true
		commandResult.StdoutOriginalBytes = stdout.OriginalBytes()
	}
	if stderr.Truncated() {
		commandResult.StderrTruncated = true
		commandResult.StderrOriginalBytes = stderr.OriginalBytes()
	}
	if err == nil {
		return commandResult
	}

	commandResult.Status = ValidationFailed
	commandResult.ExitCode = commandExitCode(err)
	commandResult.Error = err.Error()
	if errors.Is(commandCtx.Err(), context.DeadlineExceeded) {
		commandResult.Status = validationTimedOut
		commandResult.ExitCode = -1
		commandResult.Error = fmt.Sprintf("validation command timed out after %d seconds", timeoutSeconds)
	}
	return commandResult
}

func commandExitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func validationCommandError(result ValidationCommandResult) string {
	if result.Status == validationTimedOut {
		return fmt.Sprintf("validation command %q timed out after %d seconds", result.Name, result.TimeoutSeconds)
	}
	if result.ExitCode == -1 && strings.TrimSpace(result.Error) != "" {
		return result.Error
	}
	return fmt.Sprintf("validation command %q failed with exit code %d", result.Name, result.ExitCode)
}

func validationFailureError(result ValidationResult) error {
	if result.Status != ValidationFailed {
		return nil
	}
	if strings.TrimSpace(result.Error) == "" {
		return errors.New("validation failed")
	}
	return errors.New(result.Error)
}

func validationJSON(result ValidationResult) ([]byte, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func validationLog(result ValidationResult) []byte {
	var output strings.Builder
	output.WriteString("Validation: ")
	output.WriteString(result.Status)
	output.WriteByte('\n')
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
		if command.Runtime != "" {
			output.WriteString("Runtime: ")
			output.WriteString(command.Runtime)
			output.WriteByte('\n')
		}
		output.WriteString("Exec: ")
		output.WriteString(displayCommand(command.Command, command.Args))
		output.WriteByte('\n')
		output.WriteString("Status: ")
		output.WriteString(command.Status)
		output.WriteByte('\n')
		output.WriteString("Exit code: ")
		output.WriteString(strconv.Itoa(command.ExitCode))
		output.WriteByte('\n')
		output.WriteString("Duration ms: ")
		output.WriteString(strconv.FormatInt(command.DurationMS, 10))
		output.WriteByte('\n')
		if command.StdoutTruncated {
			output.WriteString("Stdout truncated: true\n")
			output.WriteString("Stdout original bytes: ")
			output.WriteString(strconv.Itoa(command.StdoutOriginalBytes))
			output.WriteByte('\n')
		}
		if command.StderrTruncated {
			output.WriteString("Stderr truncated: true\n")
			output.WriteString("Stderr original bytes: ")
			output.WriteString(strconv.Itoa(command.StderrOriginalBytes))
			output.WriteByte('\n')
		}
		if command.Error != "" {
			output.WriteString("Error: ")
			output.WriteString(command.Error)
			output.WriteByte('\n')
		}
		output.WriteString("Stdout:\n")
		output.WriteString(command.Stdout)
		if command.Stdout != "" && !strings.HasSuffix(command.Stdout, "\n") {
			output.WriteByte('\n')
		}
		output.WriteString("Stderr:\n")
		output.WriteString(command.Stderr)
		if command.Stderr != "" && !strings.HasSuffix(command.Stderr, "\n") {
			output.WriteByte('\n')
		}
	}
	return []byte(output.String())
}

func displayCommand(command string, args []string) string {
	parts := []string{command}
	for _, arg := range args {
		parts = append(parts, strconv.Quote(arg))
	}
	return strings.Join(parts, " ")
}

type limitedOutput struct {
	limit int
	data  []byte
	bytes int
}

func newLimitedOutput(limit int) limitedOutput {
	return limitedOutput{
		limit: limit,
		data:  make([]byte, 0, limit),
	}
}

func (o *limitedOutput) Write(p []byte) (int, error) {
	written := len(p)
	o.bytes += written
	remaining := o.limit - len(o.data)
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		o.data = append(o.data, p...)
	}
	return written, nil
}

func (o *limitedOutput) String() string {
	return string(o.data)
}

func (o *limitedOutput) Truncated() bool {
	return o.bytes > len(o.data)
}

func (o *limitedOutput) OriginalBytes() int {
	return o.bytes
}
