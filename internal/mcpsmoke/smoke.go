package mcpsmoke

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/mcpconfig"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
)

const (
	RuntimeLocal  = "local"
	RuntimeDocker = "docker"

	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type Options struct {
	Config         mcpconfig.Config
	Server         string
	ArtifactsDir   string
	Timeout        time.Duration
	Runtime        string
	RuntimeConfig  *runtimeconfig.Config
	Workspace      string
	StartedAtClock func() time.Time
}

type Result struct {
	Server         string   `json:"server"`
	Status         string   `json:"status"`
	Runtime        string   `json:"runtime"`
	PlanOnly       bool     `json:"plan_only"`
	TestOnly       bool     `json:"test_only"`
	Protocol       string   `json:"protocol"`
	ToolCalls      int      `json:"tool_calls"`
	Command        []string `json:"command"`
	ArtifactsDir   string   `json:"artifacts_dir"`
	SummaryPath    string   `json:"summary_path"`
	TranscriptPath string   `json:"transcript_path"`
	StdoutPath     string   `json:"stdout_path"`
	StderrPath     string   `json:"stderr_path"`
	ResultPath     string   `json:"result_path"`
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at"`
	DurationMS     int64    `json:"duration_ms"`
	Error          string   `json:"error,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type transcriptEntry struct {
	Direction string          `json:"direction"`
	Method    string          `json:"method"`
	ID        json.RawMessage `json:"id,omitempty"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func RunFakeServer(ctx context.Context, stdin io.Reader, stdout io.Writer, stderr io.Writer) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	encoder := json.NewEncoder(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var request rpcMessage
		if err := json.Unmarshal(line, &request); err != nil {
			fmt.Fprintf(stderr, "fake MCP server parse error: %v\n", err)
			continue
		}
		switch request.Method {
		case "initialize":
			if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: request.ID, Result: mustRawJSON(map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]any{
					"tools": map[string]any{"listChanged": false},
				},
				"serverInfo": map[string]any{
					"name":    "deonclaw-fake-mcp",
					"version": "test",
				},
			})}); err != nil {
				return err
			}
		case "tools/list":
			if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: request.ID, Result: mustRawJSON(map[string]any{
				"tools": []any{},
			})}); err != nil {
				return err
			}
		case "shutdown":
			if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: request.ID, Result: mustRawJSON(nil)}); err != nil {
				return err
			}
		case "exit":
			return nil
		default:
			if len(request.ID) > 0 {
				if err := encoder.Encode(rpcMessage{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{Code: -32601, Message: "method not found"}}); err != nil {
					return err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func Smoke(ctx context.Context, opts Options) (Result, error) {
	startWall := time.Now()
	clock := opts.StartedAtClock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	startedAt := clock().UTC()
	result := Result{
		Server:       strings.TrimSpace(opts.Server),
		Runtime:      normalizeRuntime(opts.Runtime),
		PlanOnly:     false,
		TestOnly:     true,
		Protocol:     mcpconfig.ProtocolStdio,
		ArtifactsDir: opts.ArtifactsDir,
		StartedAt:    startedAt.Format(time.RFC3339Nano),
	}
	if opts.Timeout <= 0 {
		return result, errors.New("mcp smoke timeout must be greater than zero")
	}
	if opts.ArtifactsDir == "" {
		return result, errors.New("mcp smoke requires artifacts dir")
	}
	if _, err := mcpconfig.Validate(opts.Config); err != nil {
		return result, err
	}
	server, err := selectSmokeServer(opts.Config, opts.Server)
	if err != nil {
		return result, err
	}
	if err := validateSmokeServer(server); err != nil {
		return result, err
	}
	result.Protocol = server.Protocol
	result.TestOnly = server.TestOnly
	result.Command = append([]string{server.Command}, server.Args...)
	switch result.Runtime {
	case RuntimeLocal:
	case RuntimeDocker:
		return result, errors.New("mcp smoke runtime docker is not implemented in Task 21.2; use runtime local")
	default:
		return result, fmt.Errorf("mcp smoke runtime %q is not supported", result.Runtime)
	}

	if err := os.MkdirAll(opts.ArtifactsDir, 0o755); err != nil {
		return result, fmt.Errorf("create mcp smoke artifacts dir: %w", err)
	}
	result.SummaryPath = filepath.Join(opts.ArtifactsDir, "mcp-smoke-summary.md")
	result.TranscriptPath = filepath.Join(opts.ArtifactsDir, "mcp-transcript.jsonl")
	result.StdoutPath = filepath.Join(opts.ArtifactsDir, "mcp-stdout.log")
	result.StderrPath = filepath.Join(opts.ArtifactsDir, "mcp-stderr.log")
	result.ResultPath = filepath.Join(opts.ArtifactsDir, "mcp-smoke-result.json")

	transcript, rawStdout, rawStderr, runErr := runLocalSmoke(ctx, server, opts.Timeout, clock)
	finishedAt := clock().UTC()
	result.FinishedAt = finishedAt.Format(time.RFC3339Nano)
	result.DurationMS = time.Since(startWall).Milliseconds()
	if runErr != nil {
		result.Status = StatusFailed
		result.Error = runErr.Error()
		_ = writeSmokeArtifacts(result, transcript, rawStdout, rawStderr)
		return result, runErr
	}
	result.Status = StatusSucceeded
	if err := writeSmokeArtifacts(result, transcript, rawStdout, rawStderr); err != nil {
		return result, err
	}
	return result, nil
}

func selectSmokeServer(cfg mcpconfig.Config, name string) (mcpconfig.ServerConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return mcpconfig.ServerConfig{}, errors.New("mcp smoke requires --server")
	}
	server, ok := cfg.MCP.Servers[name]
	if !ok {
		return mcpconfig.ServerConfig{}, fmt.Errorf("mcp server %q not found", name)
	}
	return server, nil
}

func validateSmokeServer(server mcpconfig.ServerConfig) error {
	if server.Enabled {
		return errors.New("mcp smoke requires enabled=false for test fake servers")
	}
	if !server.TestOnly {
		return errors.New("mcp smoke requires server.test_only=true")
	}
	if server.Protocol != mcpconfig.ProtocolStdio {
		return fmt.Errorf("mcp smoke requires protocol=%s", mcpconfig.ProtocolStdio)
	}
	if hasCapability(server.Capabilities, mcpconfig.CapabilityWrite) || hasCapability(server.Capabilities, mcpconfig.CapabilityExec) {
		return errors.New("mcp smoke refuses servers with write or exec capability")
	}
	return nil
}

func runLocalSmoke(ctx context.Context, server mcpconfig.ServerConfig, timeout time.Duration, clock func() time.Time) ([]transcriptEntry, []byte, string, error) {
	smokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := append([]string{server.Command}, server.Args...)
	cmd := exec.CommandContext(smokeCtx, command[0], command[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, "", err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, "", err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, "", err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, "", err
	}

	var stderr bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&stderr, stderrPipe)
		close(stderrDone)
	}()

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	writer := bufio.NewWriter(stdin)
	var stdout bytes.Buffer
	var transcript []transcriptEntry

	send := func(id int, method string) error {
		request := rpcMessage{
			JSONRPC: "2.0",
			ID:      mustRawJSON(id),
			Method:  method,
			Params:  mustRawJSON(map[string]any{}),
		}
		payload := mustRawJSON(request)
		transcript = append(transcript, transcriptEntry{
			Direction: "request",
			Method:    method,
			ID:        request.ID,
			Timestamp: clock().UTC().Format(time.RFC3339Nano),
			Payload:   payload,
		})
		if _, err := writer.Write(append(payload, '\n')); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if !scanner.Scan() {
			if smokeCtx.Err() != nil {
				return fmt.Errorf("mcp smoke timed out waiting for %s response", method)
			}
			if err := scanner.Err(); err != nil {
				return err
			}
			return fmt.Errorf("mcp smoke server closed stdout before %s response", method)
		}
		line := append([]byte(nil), bytes.TrimSpace(scanner.Bytes())...)
		stdout.Write(line)
		stdout.WriteByte('\n')
		var response rpcMessage
		if err := json.Unmarshal(line, &response); err != nil {
			return fmt.Errorf("parse %s response: %w", method, err)
		}
		transcript = append(transcript, transcriptEntry{
			Direction: "response",
			Method:    method,
			ID:        response.ID,
			Timestamp: clock().UTC().Format(time.RFC3339Nano),
			Payload:   append(json.RawMessage(nil), line...),
		})
		return nil
	}

	runErr := send(1, "initialize")
	if runErr == nil {
		runErr = send(2, "tools/list")
	}
	if runErr == nil {
		runErr = send(3, "shutdown")
	}
	exitNotification := rpcMessage{JSONRPC: "2.0", Method: "exit"}
	exitPayload := mustRawJSON(exitNotification)
	transcript = append(transcript, transcriptEntry{
		Direction: "request",
		Method:    "exit",
		Timestamp: clock().UTC().Format(time.RFC3339Nano),
		Payload:   exitPayload,
	})
	if _, err := writer.Write(append(exitPayload, '\n')); err != nil && runErr == nil {
		runErr = err
	}
	if err := writer.Flush(); err != nil && runErr == nil {
		runErr = err
	}
	_ = stdin.Close()
	waitErr := cmd.Wait()
	<-stderrDone
	if smokeCtx.Err() != nil {
		return transcript, stdout.Bytes(), stderr.String(), errors.New("mcp smoke timed out")
	}
	if runErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), runErr
	}
	if waitErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), fmt.Errorf("mcp smoke server exited non-zero: %w", waitErr)
	}
	return transcript, stdout.Bytes(), stderr.String(), nil
}

func writeSmokeArtifacts(result Result, transcript []transcriptEntry, stdout []byte, stderr string) error {
	if err := os.WriteFile(result.SummaryPath, smokeSummary(result), 0o600); err != nil {
		return err
	}
	transcriptJSONL, err := transcriptJSONL(transcript)
	if err != nil {
		return err
	}
	if err := os.WriteFile(result.TranscriptPath, transcriptJSONL, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(result.StdoutPath, stdout, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(result.StderrPath, []byte(stderr), 0o600); err != nil {
		return err
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(result.ResultPath, append(data, '\n'), 0o600)
}

func smokeSummary(result Result) []byte {
	var builder strings.Builder
	builder.WriteString("# MCP Smoke\n\n")
	builder.WriteString("Mode: fake/test smoke only\n")
	builder.WriteString("Server: " + result.Server + "\n")
	builder.WriteString("Status: " + result.Status + "\n")
	builder.WriteString("Runtime: " + result.Runtime + "\n")
	builder.WriteString("Protocol: " + result.Protocol + "\n")
	builder.WriteString(fmt.Sprintf("Tool calls: %d\n", result.ToolCalls))
	if result.Error != "" {
		builder.WriteString("Error: " + result.Error + "\n")
	}
	return []byte(builder.String())
}

func transcriptJSONL(entries []transcriptEntry) ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

func normalizeRuntime(runtime string) string {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return RuntimeLocal
	}
	return runtime
}

func hasCapability(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustRawJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}
