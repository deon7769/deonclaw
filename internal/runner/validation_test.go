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
