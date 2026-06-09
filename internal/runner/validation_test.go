package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestRunValidationCommandsUsesWorkspaceAndCapturesOutput(t *testing.T) {
	workspace := t.TempDir()

	result := RunValidationCommands(context.Background(), workspace, []tasks.ValidationCommand{
		{
			Name:    "print-working-directory",
			Command: os.Args[0],
			// Use the test binary as a helper instead of shell built-ins such as
			// pwd. RunValidationCommands uses exec.Command directly, so PowerShell
			// aliases/functions are not available on Windows.
			Args: []string{"-test.run=TestValidationCommandHelperProcess", "--", "print-working-directory"},
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
	if filepath.Clean(strings.TrimSpace(result.Commands[0].Stdout)) != filepath.Clean(workspace) {
		t.Fatalf("validation stdout = %q, want workspace %q", result.Commands[0].Stdout, workspace)
	}
	if result.Commands[0].ExitCode != 0 {
		t.Fatalf("exit code = %d, want 0", result.Commands[0].ExitCode)
	}
	if result.Commands[0].TimeoutSeconds != defaultValidationTimeoutSeconds {
		t.Fatalf("timeout = %d, want default", result.Commands[0].TimeoutSeconds)
	}
}

func TestRunValidationCommandsTruncatesLargeOutput(t *testing.T) {
	workspace := t.TempDir()
	originalBytes := defaultValidationOutputLimitBytes + 5

	result := RunValidationCommands(context.Background(), workspace, []tasks.ValidationCommand{
		{
			Name:    "large-output",
			Command: os.Args[0],
			Args: []string{
				"-test.run=TestValidationCommandHelperProcess",
				"--",
				"emit-large-output",
				strconv.Itoa(originalBytes),
			},
		},
	})

	if result.Status != ValidationPassed {
		t.Fatalf("validation status = %q, want %q: %#v", result.Status, ValidationPassed, result)
	}
	command := result.Commands[0]
	if len(command.Stdout) != defaultValidationOutputLimitBytes {
		t.Fatalf("stdout len = %d, want %d", len(command.Stdout), defaultValidationOutputLimitBytes)
	}
	if len(command.Stderr) != defaultValidationOutputLimitBytes {
		t.Fatalf("stderr len = %d, want %d", len(command.Stderr), defaultValidationOutputLimitBytes)
	}
	if !command.StdoutTruncated || !command.StderrTruncated {
		t.Fatalf("truncated flags = stdout:%t stderr:%t, want both true", command.StdoutTruncated, command.StderrTruncated)
	}
	if command.StdoutOriginalBytes != originalBytes || command.StderrOriginalBytes != originalBytes {
		t.Fatalf("original bytes = stdout:%d stderr:%d, want %d", command.StdoutOriginalBytes, command.StderrOriginalBytes, originalBytes)
	}

	log := string(validationLog(result))
	if !strings.Contains(log, "Stdout truncated: true") || !strings.Contains(log, "Stderr truncated: true") {
		t.Fatalf("validation log = %q, want truncation markers", log)
	}

	data, err := validationJSON(result)
	if err != nil {
		t.Fatalf("validationJSON() error = %v", err)
	}
	var decoded ValidationResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("validation JSON unmarshal error = %v", err)
	}
	if !decoded.Commands[0].StdoutTruncated || decoded.Commands[0].StdoutOriginalBytes != originalBytes {
		t.Fatalf("validation JSON command = %#v, want stdout truncation metadata", decoded.Commands[0])
	}
}

func TestRunDockerValidationCommandsUsesFakeDockerAndCapturesOutput(t *testing.T) {
	workspace := t.TempDir()
	argsPath := installRunnerFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
printf 'docker validation stdout\n'
printf 'docker validation stderr\n' >&2
exit 0
`)

	result := RunDockerValidationCommands(context.Background(), workspace, []tasks.ValidationCommand{
		{
			Name:    "go-test",
			Command: "go",
			Args:    []string{"test", "./..."},
		},
	}, validDockerValidationConfig())

	if result.Status != ValidationPassed {
		t.Fatalf("validation status = %q, want %q: %#v", result.Status, ValidationPassed, result)
	}
	command := result.Commands[0]
	if command.Runtime != tasks.ValidationRuntimeDocker {
		t.Fatalf("runtime = %q, want docker", command.Runtime)
	}
	if !strings.Contains(command.Stdout, "docker validation stdout") {
		t.Fatalf("stdout = %q, want fake docker stdout", command.Stdout)
	}
	if !strings.Contains(command.Stderr, "docker validation stderr") {
		t.Fatalf("stderr = %q, want fake docker stderr", command.Stderr)
	}
	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	for _, want := range []string{"run", "--rm", "--network", "none", "--read-only", "deonclaw-runner:latest", "go", "test", "./..."} {
		if !containsString(args, want) {
			t.Fatalf("docker args = %#v, want %q", args, want)
		}
	}
	if strings.Contains(strings.Join(args, " "), "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}
	if args[len(args)-3] != "go" || args[len(args)-2] != "test" || args[len(args)-1] != "./..." {
		t.Fatalf("docker args tail = %#v, want validation command appended after image", args)
	}
}

func TestRunDockerValidationCommandsRespectsTimeout(t *testing.T) {
	installRunnerFakeDocker(t, `#!/bin/sh
sleep 2
exit 0
`)

	start := time.Now()
	result := RunDockerValidationCommands(context.Background(), t.TempDir(), []tasks.ValidationCommand{
		{
			Name:           "slow",
			Command:        "go",
			Args:           []string{"test"},
			TimeoutSeconds: 1,
		},
	}, validDockerValidationConfig())
	elapsed := time.Since(start)

	if result.Status != ValidationFailed {
		t.Fatalf("validation status = %q, want failed: %#v", result.Status, result)
	}
	if len(result.Commands) != 1 || result.Commands[0].Status != validationTimedOut {
		t.Fatalf("commands = %#v, want timed out command", result.Commands)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("elapsed = %s, want timeout near 1s", elapsed)
	}
}

func TestRunDockerValidationCommandsRecordsNonZeroExit(t *testing.T) {
	installRunnerFakeDocker(t, `#!/bin/sh
printf 'bad stdout\n'
printf 'bad stderr\n' >&2
exit 23
`)

	result := RunDockerValidationCommands(context.Background(), t.TempDir(), []tasks.ValidationCommand{
		{Name: "failing", Command: "go", Args: []string{"test"}},
	}, validDockerValidationConfig())

	if result.Status != ValidationFailed {
		t.Fatalf("validation status = %q, want failed: %#v", result.Status, result)
	}
	command := result.Commands[0]
	if command.ExitCode != 23 {
		t.Fatalf("exit code = %d, want 23", command.ExitCode)
	}
	if !strings.Contains(command.Stdout, "bad stdout") || !strings.Contains(command.Stderr, "bad stderr") {
		t.Fatalf("command = %#v, want stdout/stderr captured", command)
	}
}

func TestRunDockerValidationCommandsInvalidConfigFailsBeforeDocker(t *testing.T) {
	argsPath := installRunnerFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	cfg := validDockerValidationConfig()
	cfg.Runtime.Mode = "podman"

	result := RunDockerValidationCommands(context.Background(), t.TempDir(), []tasks.ValidationCommand{
		{Name: "go-test", Command: "go", Args: []string{"test"}},
	}, cfg)

	if result.Status != ValidationFailed {
		t.Fatalf("validation status = %q, want failed: %#v", result.Status, result)
	}
	if !strings.Contains(result.Error, `runtime.mode "podman" is not supported`) {
		t.Fatalf("error = %q, want invalid config", result.Error)
	}
	assertRunnerFileEmptyOrMissing(t, argsPath)
}

func TestRunDockerValidationCommandsDangerousMountFailsBeforeDocker(t *testing.T) {
	argsPath := installRunnerFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	cfg := validDockerValidationConfig()
	cfg.Runtime.Docker.Mounts = []runtimeconfig.MountSpec{{Source: "/", Target: "/workspace", Mode: "ro"}}

	result := RunDockerValidationCommands(context.Background(), t.TempDir(), []tasks.ValidationCommand{
		{Name: "go-test", Command: "go", Args: []string{"test"}},
	}, cfg)

	if result.Status != ValidationFailed {
		t.Fatalf("validation status = %q, want failed: %#v", result.Status, result)
	}
	if !strings.Contains(result.Error, `runtime.docker.mounts[0].source "/" is not allowed`) {
		t.Fatalf("error = %q, want dangerous mount", result.Error)
	}
	assertRunnerFileEmptyOrMissing(t, argsPath)
}

func TestLimitedOutputReturnsOriginalWriteLengthWhenTruncating(t *testing.T) {
	output := newLimitedOutput(3)

	n, err := output.Write([]byte("abcdef"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if n != 6 {
		t.Fatalf("Write() bytes = %d, want original length 6", n)
	}
	if output.String() != "abc" {
		t.Fatalf("String() = %q, want truncated payload", output.String())
	}
	if !output.Truncated() || output.OriginalBytes() != 6 {
		t.Fatalf("truncation metadata = truncated:%t original:%d, want true/6", output.Truncated(), output.OriginalBytes())
	}
}

func installRunnerFakeDocker(t *testing.T, script string) string {
	t.Helper()
	tempDir := t.TempDir()
	fakeDocker := filepath.Join(tempDir, "docker")
	argsPath := filepath.Join(tempDir, "docker.args")
	if err := os.WriteFile(fakeDocker, []byte(script), 0o700); err != nil {
		t.Fatalf("WriteFile(fake docker) error = %v", err)
	}
	t.Setenv("DEONCLAW_FAKE_DOCKER_ARGS", argsPath)
	t.Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsPath
}

func validDockerValidationConfig() runtimeconfig.Config {
	return runtimeconfig.Config{
		Runtime: runtimeconfig.Runtime{
			Mode: runtimeconfig.ModeDocker,
			Docker: runtimeconfig.DockerConfig{
				Image:        "deonclaw-runner:latest",
				Workdir:      "/workspace",
				Network:      "none",
				ReadOnlyRoot: true,
				MemoryLimit:  "2g",
				CPUs:         "2",
				Mounts: []runtimeconfig.MountSpec{
					{Source: ".", Target: "/workspace", Mode: "rw"},
				},
			},
		},
	}
}

func assertRunnerFileEmptyOrMissing(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if len(data) != 0 {
		t.Fatalf("%s = %q, want empty or missing", path, string(data))
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestValidationCommandHelperProcess(t *testing.T) {
	args := validationHelperArgs()
	if len(args) == 0 {
		return
	}

	switch args[0] {
	case "print-working-directory":
		printWorkingDirectory()
	case "emit-large-output":
		if len(args) != 2 {
			fmt.Fprintln(os.Stderr, "emit-large-output requires byte count")
			os.Exit(2)
		}
		byteCount, err := strconv.Atoi(args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Fprint(os.Stdout, strings.Repeat("o", byteCount))
		fmt.Fprint(os.Stderr, strings.Repeat("e", byteCount))
		os.Exit(0)
	default:
		return
	}
}

func validationHelperArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1:]
		}
	}
	return nil
}

func printWorkingDirectory() {
	workspace, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Fprintln(os.Stdout, workspace)
	os.Exit(0)
}
