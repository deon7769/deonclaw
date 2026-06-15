package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func TestRunFailsClearlyWhenBinaryMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-opencode")
	result, err := (&Worker{command: missing}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err == nil {
		t.Fatal("Run() error = nil, want missing binary failure")
	}
	if result == nil || result.Worker != "opencode" {
		t.Fatalf("result = %#v, want opencode result", result)
	}
	if !strings.Contains(err.Error(), "opencode command failed") {
		t.Fatalf("error = %q, want clear opencode command failure", err)
	}
}

func TestRunCapturesPlainStdoutAndStderr(t *testing.T) {
	script := writeExecutableScript(t, "printf 'plain stdout\\n'; printf 'plain stderr\\n' >&2")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Stderr != "plain stderr\n" {
		t.Fatalf("stderr = %q, want captured stderr", result.Stderr)
	}
	assertWorkerArtifactContent(t, result, "stdout.log", "plain stdout\n")
	assertWorkerArtifactContent(t, result, "stderr.log", "plain stderr\n")
	if len(result.Events) != 1 || result.Events[0].Type != "worker.stdout.log" {
		t.Fatalf("events = %#v, want stdout log event", result.Events)
	}
	if got := result.Metadata["opencode.stdout_format"]; got != "text" {
		t.Fatalf("stdout format = %q, want text", got)
	}
}

func TestRunExecutesConfiguredModelAndKeepsPromptMaskedInResult(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	body := "printf '%s\\n' \"$*\" > " + shellQuote(argsPath) + "\nprintf '%s\\n' '{\"type\":\"done\"}'"
	script := writeRawExecutableScript(t, body)
	worker := NewWithOptions(Options{
		Command:      script,
		ModelProfile: "opencode-zai-glm-5-1",
		Provider:     "z-ai",
		Model:        "glm-5.1",
		ModelArg:     "z-ai/glm-5.1",
	})

	result, err := worker.Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do not leak this prompt", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	if !strings.Contains(string(args), "--model z-ai/glm-5.1") {
		t.Fatalf("args = %q, want --model", args)
	}
	if got := strings.Join(result.Command, " "); !strings.Contains(got, "--model z-ai/glm-5.1 <prompt>") {
		t.Fatalf("result command = %q, want masked model command", got)
	}
	if strings.Contains(strings.Join(result.Command, " "), "do not leak this prompt") {
		t.Fatalf("result command leaked prompt: %#v", result.Command)
	}
	if result.Metadata["model_profile"] != "opencode-zai-glm-5-1" || result.Metadata["provider"] != "z-ai" || result.Metadata["model"] != "glm-5.1" || result.Metadata["model_arg"] != "z-ai/glm-5.1" {
		t.Fatalf("metadata = %#v, want model profile metadata", result.Metadata)
	}
}

func TestRunCapturesJSONLStdoutAsEvents(t *testing.T) {
	script := writeExecutableScript(t, "printf '%s\\n' '{\"type\":\"message\",\"text\":\"ok\"}'")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].Type != "message" || string(result.Events[0].Payload) != "{\"type\":\"message\",\"text\":\"ok\"}" {
		t.Fatalf("events = %#v, want JSONL event", result.Events)
	}
	assertWorkerArtifactContent(t, result, "stdout.jsonl", "{\"type\":\"message\",\"text\":\"ok\"}\n")
	assertWorkerArtifactContent(t, result, "stderr.log", "")
	if got := result.Metadata["opencode.stdout_format"]; got != "jsonl" {
		t.Fatalf("stdout format = %q, want jsonl", got)
	}
}

func TestRunCapturesMixedStdoutWithoutFailing(t *testing.T) {
	script := writeExecutableScript(t, "printf '%s\\n' 'plain line' '{\"type\":\"message\",\"text\":\"ok\"}' '{not-json}'")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertWorkerArtifactContent(t, result, "stdout.log", "plain line\n{\"type\":\"message\",\"text\":\"ok\"}\n{not-json}\n")
	assertWorkerArtifactContent(t, result, "stderr.log", "")
	if len(result.Events) != 2 {
		t.Fatalf("events = %#v, want JSON event and parse warning", result.Events)
	}
	if result.Events[0].Type != "message" {
		t.Fatalf("first event = %#v, want message", result.Events[0])
	}
	if result.Events[1].Type != "worker.stdout.parse_warning" {
		t.Fatalf("second event = %#v, want parse warning", result.Events[1])
	}
	if got := result.Metadata["opencode.stdout_format"]; got != "mixed" {
		t.Fatalf("stdout format = %q, want mixed", got)
	}
	if got := result.Metadata["opencode.parsed_events"]; got != "1" {
		t.Fatalf("parsed events = %q, want 1", got)
	}
	if got := result.Metadata["opencode.parse_warnings"]; got != "1" {
		t.Fatalf("parse warnings = %q, want 1", got)
	}
}

func TestRunCapturesEmptyStdout(t *testing.T) {
	script := writeExecutableScript(t, ":")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertWorkerArtifactContent(t, result, "stdout.log", "")
	assertWorkerArtifactContent(t, result, "stderr.log", "")
	if len(result.Events) != 0 {
		t.Fatalf("events = %#v, want none for empty stdout", result.Events)
	}
	if got := result.Metadata["opencode.stdout_format"]; got != "empty" {
		t.Fatalf("stdout format = %q, want empty", got)
	}
}

func TestRunDoesNotFailOnInvalidJSONStdout(t *testing.T) {
	script := writeExecutableScript(t, "printf '%s\\n' '{not-json}'")
	result, err := (&Worker{command: script}).Run(context.Background(), workers.RunSpec{
		Task:      &tasks.Task{Goal: "do nothing", Mode: "read_only"},
		Workspace: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertWorkerArtifactContent(t, result, "stdout.log", "{not-json}\n")
	if len(result.Events) != 1 || result.Events[0].Type != "worker.stdout.log" {
		t.Fatalf("events = %#v, want stdout log event", result.Events)
	}
	if got := result.Metadata["opencode.stdout_format"]; got != "text" {
		t.Fatalf("stdout format = %q, want text", got)
	}
}

func TestLargeStdoutPayloadIsTruncatedInEventOnly(t *testing.T) {
	stdout := []byte(strings.Repeat("x", maxEventPayloadBytes*2))
	analysis := analyzeStdout(stdout, []string{"opencode", "run"}, ".")
	if analysis.Format != "text" {
		t.Fatalf("stdout format = %q, want text", analysis.Format)
	}
	if len(analysis.Artifacts) != 1 || string(analysis.Artifacts[0].Content) != string(stdout) {
		t.Fatalf("artifact content was not preserved raw")
	}
	if len(analysis.Events) != 1 {
		t.Fatalf("events = %#v, want stdout log event", analysis.Events)
	}
	if len(analysis.Events[0].Payload) > maxEventPayloadBytes {
		t.Fatalf("payload size = %d, want <= %d", len(analysis.Events[0].Payload), maxEventPayloadBytes)
	}
	var payload struct {
		Truncated     bool `json:"truncated"`
		OriginalBytes int  `json:"original_bytes"`
	}
	if err := json.Unmarshal(analysis.Events[0].Payload, &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if !payload.Truncated || payload.OriginalBytes != len(stdout) {
		t.Fatalf("payload = %#v, want truncated payload with original bytes", payload)
	}
}

func assertWorkerArtifactContent(t *testing.T, result *workers.RunResult, name string, want string) {
	t.Helper()
	for _, artifact := range result.Artifacts {
		if filepath.Base(artifact.Path) == name {
			if string(artifact.Content) != want {
				t.Fatalf("%s content = %q, want %q", name, artifact.Content, want)
			}
			return
		}
	}
	t.Fatalf("artifact %s not found in %#v", name, result.Artifacts)
}

func writeExecutableScript(t *testing.T, body string) string {
	t.Helper()
	return writeOpenCodeFakeCommand(t, openCodeFakeBehaviorFromBody(body))
}

func writeRawExecutableScript(t *testing.T, body string) string {
	t.Helper()
	behavior := openCodeFakeBehaviorFromBody(body)
	behavior.argsPath = openCodeFakeArgsPathFromBody(body)
	return writeOpenCodeFakeCommand(t, behavior)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

type openCodeFakeCommandBehavior struct {
	argsPath string
	stdout   string
	stderr   string
}

func writeOpenCodeFakeCommand(t *testing.T, behavior openCodeFakeCommandBehavior) string {
	t.Helper()
	t.Setenv("DEONCLAW_OPENCODE_FAKE_ARGS_PATH", behavior.argsPath)
	t.Setenv("DEONCLAW_OPENCODE_FAKE_STDOUT", behavior.stdout)
	t.Setenv("DEONCLAW_OPENCODE_FAKE_STDERR", behavior.stderr)

	dir := t.TempDir()
	path := filepath.Join(dir, "opencode-fake")
	if runtime.GOOS == "windows" {
		path += ".bat"
	}
	testBinary, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatalf("Abs(test binary) error = %v", err)
	}
	var content string
	if runtime.GOOS == "windows" {
		content = fmt.Sprintf("@echo off\r\nset DEONCLAW_OPENCODE_FAKE_HELPER=1\r\n\"%s\" -test.run=TestOpenCodeFakeCommandHelperProcess -- %%*\r\nexit /b %%ERRORLEVEL%%\r\n", testBinary)
	} else {
		content = "#!/bin/sh\nDEONCLAW_OPENCODE_FAKE_HELPER=1 exec " + shellQuote(testBinary) + " -test.run=TestOpenCodeFakeCommandHelperProcess -- \"$@\"\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("WriteFile(script) error = %v", err)
	}
	return path
}

func TestOpenCodeFakeCommandHelperProcess(t *testing.T) {
	if os.Getenv("DEONCLAW_OPENCODE_FAKE_HELPER") != "1" {
		return
	}
	if argsPath := os.Getenv("DEONCLAW_OPENCODE_FAKE_ARGS_PATH"); argsPath != "" {
		_ = os.WriteFile(argsPath, []byte(strings.Join(openCodeHelperArgs(), " ")+"\n"), 0o600)
	}
	_, _ = fmt.Fprint(os.Stdout, os.Getenv("DEONCLAW_OPENCODE_FAKE_STDOUT"))
	_, _ = fmt.Fprint(os.Stderr, os.Getenv("DEONCLAW_OPENCODE_FAKE_STDERR"))
	os.Exit(0)
}

func openCodeHelperArgs() []string {
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			return os.Args[i+1:]
		}
	}
	return nil
}

func openCodeFakeBehaviorFromBody(body string) openCodeFakeCommandBehavior {
	var behavior openCodeFakeCommandBehavior
	if strings.Contains(body, "plain stdout") {
		behavior.stdout += "plain stdout\n"
	}
	if strings.Contains(body, "plain stderr") {
		behavior.stderr += "plain stderr\n"
	}
	if strings.Contains(body, "plain line") {
		behavior.stdout += "plain line\n"
	}
	if strings.Contains(body, `{"type":"message","text":"ok"}`) {
		behavior.stdout += `{"type":"message","text":"ok"}` + "\n"
	}
	if strings.Contains(body, "{not-json}") {
		behavior.stdout += "{not-json}\n"
	}
	return behavior
}

func openCodeFakeArgsPathFromBody(body string) string {
	marker := "> "
	index := strings.Index(body, marker)
	if index < 0 {
		return ""
	}
	rest := strings.TrimSpace(body[index+len(marker):])
	line, _, _ := strings.Cut(rest, "\n")
	return strings.Trim(strings.TrimSpace(line), `'"`)
}
