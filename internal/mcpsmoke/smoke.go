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
	"gopkg.in/yaml.v3"
)

const (
	RuntimeLocal  = "local"
	RuntimeDocker = "docker"

	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"

	FakeEchoToolName      = "deonclaw.fake.echo"
	MaxToolArgumentsBytes = 64 * 1024
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

type ToolOptions struct {
	Config         mcpconfig.Config
	Server         string
	Tool           string
	Arguments      []byte
	ArtifactsDir   string
	Timeout        time.Duration
	Runtime        string
	RuntimeConfig  *runtimeconfig.Config
	Workspace      string
	Policy         *ToolPolicy
	StartedAtClock func() time.Time
}

type ToolResult struct {
	Server         string          `json:"server"`
	Status         string          `json:"status"`
	Runtime        string          `json:"runtime"`
	TestOnly       bool            `json:"test_only"`
	Protocol       string          `json:"protocol"`
	Tool           string          `json:"tool"`
	ToolCalls      int             `json:"tool_calls"`
	Command        []string        `json:"command"`
	ArtifactsDir   string          `json:"artifacts_dir"`
	SummaryPath    string          `json:"summary_path"`
	TranscriptPath string          `json:"transcript_path"`
	StdoutPath     string          `json:"stdout_path"`
	StderrPath     string          `json:"stderr_path"`
	ResultPath     string          `json:"result_path"`
	ToolResponse   json.RawMessage `json:"tool_response,omitempty"`
	StartedAt      string          `json:"started_at"`
	FinishedAt     string          `json:"finished_at"`
	DurationMS     int64           `json:"duration_ms"`
	Error          string          `json:"error,omitempty"`
	Warnings       []string        `json:"warnings,omitempty"`
}

type ToolPolicy struct {
	MCPToolPolicy MCPToolPolicy `yaml:"mcp_tool_policy" json:"mcp_tool_policy"`
}

type MCPToolPolicy struct {
	AllowTestOnly       bool     `yaml:"allow_test_only" json:"allow_test_only"`
	MaxToolCalls        int      `yaml:"max_tool_calls" json:"max_tool_calls"`
	AllowedServers      []string `yaml:"allowed_servers,omitempty" json:"allowed_servers,omitempty"`
	AllowedTools        []string `yaml:"allowed_tools,omitempty" json:"allowed_tools,omitempty"`
	AllowedCapabilities []string `yaml:"allowed_capabilities,omitempty" json:"allowed_capabilities,omitempty"`
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
				"tools": []any{
					map[string]any{
						"name":        FakeEchoToolName,
						"description": "echo test payload for MCP smoke only",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"text": map[string]any{"type": "string"},
							},
							"required": []string{"text"},
						},
					},
				},
			})}); err != nil {
				return err
			}
		case "tools/call":
			if err := encoder.Encode(fakeToolCallResponse(request)); err != nil {
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

func fakeToolCallResponse(request rpcMessage) rpcMessage {
	var params struct {
		Name      string `json:"name"`
		Arguments struct {
			Text string `json:"text"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return rpcMessage{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{Code: -32602, Message: "invalid tools/call params"}}
	}
	if params.Name != FakeEchoToolName {
		return rpcMessage{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{Code: -32602, Message: "unknown tool"}}
	}
	return rpcMessage{JSONRPC: "2.0", ID: request.ID, Result: mustRawJSON(map[string]any{
		"content": []any{
			map[string]any{
				"type": "text",
				"text": params.Arguments.Text,
			},
		},
	})}
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
	serverCommand := append([]string{server.Command}, server.Args...)
	runCommand := append([]string(nil), serverCommand...)
	switch result.Runtime {
	case RuntimeLocal:
	case RuntimeDocker:
		if opts.RuntimeConfig == nil {
			return result, errors.New("mcp smoke runtime docker requires --runtime-config")
		}
		if err := validateServerEnvPassthrough(server); err != nil {
			return result, err
		}
		plan, err := mcpconfig.PlanDockerLaunch(opts.Config, opts.Server, *opts.RuntimeConfig, opts.Workspace)
		if err != nil {
			return result, err
		}
		runCommand = append([]string(nil), plan.Command...)
		result.Warnings = append(result.Warnings, plan.Warnings...)
	default:
		return result, fmt.Errorf("mcp smoke runtime %q is not supported", result.Runtime)
	}
	result.Command = append([]string(nil), runCommand...)

	if err := os.MkdirAll(opts.ArtifactsDir, 0o755); err != nil {
		return result, fmt.Errorf("create mcp smoke artifacts dir: %w", err)
	}
	result.SummaryPath = filepath.Join(opts.ArtifactsDir, "mcp-smoke-summary.md")
	result.TranscriptPath = filepath.Join(opts.ArtifactsDir, "mcp-transcript.jsonl")
	result.StdoutPath = filepath.Join(opts.ArtifactsDir, "mcp-stdout.log")
	result.StderrPath = filepath.Join(opts.ArtifactsDir, "mcp-stderr.log")
	result.ResultPath = filepath.Join(opts.ArtifactsDir, "mcp-smoke-result.json")

	transcript, rawStdout, rawStderr, runErr := runCommandSmoke(ctx, runCommand, opts.Timeout, clock)
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

func ToolSmoke(ctx context.Context, opts ToolOptions) (ToolResult, error) {
	startWall := time.Now()
	clock := opts.StartedAtClock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	startedAt := clock().UTC()
	result := ToolResult{
		Server:       strings.TrimSpace(opts.Server),
		Runtime:      normalizeRuntime(opts.Runtime),
		TestOnly:     true,
		Protocol:     mcpconfig.ProtocolStdio,
		Tool:         strings.TrimSpace(opts.Tool),
		ArtifactsDir: opts.ArtifactsDir,
		StartedAt:    startedAt.Format(time.RFC3339Nano),
	}
	if opts.Timeout <= 0 {
		return result, errors.New("mcp tool-smoke timeout must be greater than zero")
	}
	if opts.ArtifactsDir == "" {
		return result, errors.New("mcp tool-smoke requires artifacts dir")
	}
	if result.Tool == "" {
		return result, errors.New("mcp tool-smoke requires --tool")
	}
	arguments, err := validateToolArguments(opts.Arguments)
	if err != nil {
		return result, err
	}
	policy := DefaultToolPolicy()
	if opts.Policy != nil {
		policy = *opts.Policy
	}
	if err := ValidateToolPolicy(policy); err != nil {
		return result, err
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
	if err := validateToolPolicyForCall(policy, result.Server, server, result.Tool); err != nil {
		return result, err
	}
	result.Protocol = server.Protocol
	result.TestOnly = server.TestOnly
	serverCommand := append([]string{server.Command}, server.Args...)
	runCommand := append([]string(nil), serverCommand...)
	switch result.Runtime {
	case RuntimeLocal:
	case RuntimeDocker:
		if opts.RuntimeConfig == nil {
			return result, errors.New("mcp tool-smoke runtime docker requires --runtime-config")
		}
		if err := validateServerEnvPassthrough(server); err != nil {
			return result, err
		}
		plan, err := mcpconfig.PlanDockerLaunch(opts.Config, opts.Server, *opts.RuntimeConfig, opts.Workspace)
		if err != nil {
			return result, err
		}
		runCommand = append([]string(nil), plan.Command...)
		result.Warnings = append(result.Warnings, plan.Warnings...)
	default:
		return result, fmt.Errorf("mcp tool-smoke runtime %q is not supported", result.Runtime)
	}
	result.Command = append([]string(nil), runCommand...)

	if err := os.MkdirAll(opts.ArtifactsDir, 0o755); err != nil {
		return result, fmt.Errorf("create mcp tool-smoke artifacts dir: %w", err)
	}
	result.SummaryPath = filepath.Join(opts.ArtifactsDir, "mcp-tool-smoke-summary.md")
	result.TranscriptPath = filepath.Join(opts.ArtifactsDir, "mcp-tool-transcript.jsonl")
	result.StdoutPath = filepath.Join(opts.ArtifactsDir, "mcp-tool-stdout.log")
	result.StderrPath = filepath.Join(opts.ArtifactsDir, "mcp-tool-stderr.log")
	result.ResultPath = filepath.Join(opts.ArtifactsDir, "mcp-tool-result.json")

	transcript, rawStdout, rawStderr, toolResponse, runErr := runCommandToolSmoke(ctx, runCommand, opts.Timeout, clock, result.Tool, arguments)
	finishedAt := clock().UTC()
	result.FinishedAt = finishedAt.Format(time.RFC3339Nano)
	result.DurationMS = time.Since(startWall).Milliseconds()
	result.ToolResponse = toolResponse
	if runErr != nil {
		result.Status = StatusFailed
		result.Error = runErr.Error()
		_ = writeToolArtifacts(result, transcript, rawStdout, rawStderr)
		return result, runErr
	}
	result.Status = StatusSucceeded
	result.ToolCalls = 1
	if err := writeToolArtifacts(result, transcript, rawStdout, rawStderr); err != nil {
		return result, err
	}
	return result, nil
}

func LoadToolPolicy(path string) (ToolPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ToolPolicy{}, fmt.Errorf("read MCP tool policy %q: %w", path, err)
	}
	var policy ToolPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return ToolPolicy{}, fmt.Errorf("parse MCP tool policy yaml: %w", err)
	}
	normalizeToolPolicy(&policy)
	return policy, nil
}

func DefaultToolPolicy() ToolPolicy {
	return ToolPolicy{
		MCPToolPolicy: MCPToolPolicy{
			AllowTestOnly:       true,
			MaxToolCalls:        1,
			AllowedServers:      []string{"fake-stdio"},
			AllowedTools:        []string{FakeEchoToolName},
			AllowedCapabilities: []string{mcpconfig.CapabilityRead},
		},
	}
}

func ValidateToolPolicy(policy ToolPolicy) error {
	var errs []error
	p := policy.MCPToolPolicy
	if !p.AllowTestOnly {
		errs = append(errs, errors.New("mcp_tool_policy.allow_test_only must be true"))
	}
	if p.MaxToolCalls < 1 {
		errs = append(errs, errors.New("mcp_tool_policy.max_tool_calls must be >= 1"))
	}
	if len(p.AllowedServers) == 0 {
		errs = append(errs, errors.New("mcp_tool_policy.allowed_servers must not be empty"))
	}
	if len(p.AllowedTools) == 0 {
		errs = append(errs, errors.New("mcp_tool_policy.allowed_tools must not be empty"))
	}
	if len(p.AllowedCapabilities) == 0 {
		errs = append(errs, errors.New("mcp_tool_policy.allowed_capabilities must not be empty"))
	}
	for _, capability := range p.AllowedCapabilities {
		switch capability {
		case mcpconfig.CapabilityRead:
		case mcpconfig.CapabilityWrite, mcpconfig.CapabilityExec:
			errs = append(errs, fmt.Errorf("mcp_tool_policy.allowed_capabilities %q is not permitted", capability))
		default:
			errs = append(errs, fmt.Errorf("mcp_tool_policy.allowed_capabilities %q is not supported", capability))
		}
	}
	return errors.Join(errs...)
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

func validateServerEnvPassthrough(server mcpconfig.ServerConfig) error {
	var errs []error
	for _, name := range server.Env.Passthrough {
		if !envIsSet(name) {
			errs = append(errs, fmt.Errorf("mcp server env passthrough %s is not set in process env", name))
		}
	}
	return errors.Join(errs...)
}

func validateToolPolicyForCall(policy ToolPolicy, serverName string, server mcpconfig.ServerConfig, tool string) error {
	p := policy.MCPToolPolicy
	if p.AllowTestOnly && !server.TestOnly {
		return errors.New("mcp tool-smoke requires server.test_only=true")
	}
	if len(p.AllowedServers) > 0 && !hasString(p.AllowedServers, serverName) {
		return fmt.Errorf("mcp tool-smoke server %q is not allowlisted", serverName)
	}
	if len(p.AllowedTools) > 0 && !hasString(p.AllowedTools, tool) {
		return fmt.Errorf("mcp tool-smoke tool %q is not allowlisted", tool)
	}
	for _, capability := range server.Capabilities {
		if capability == mcpconfig.CapabilityWrite || capability == mcpconfig.CapabilityExec {
			return errors.New("mcp tool-smoke refuses servers with write or exec capability")
		}
		if len(p.AllowedCapabilities) > 0 && !hasString(p.AllowedCapabilities, capability) {
			return fmt.Errorf("mcp tool-smoke capability %q is not allowlisted", capability)
		}
	}
	return nil
}

func runCommandSmoke(ctx context.Context, command []string, timeout time.Duration, clock func() time.Time) ([]transcriptEntry, []byte, string, error) {
	smokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, nil, "", errors.New("mcp smoke command must not be empty")
	}
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
		return transcript, stdout.Bytes(), stderr.String(), fmt.Errorf("mcp smoke process exited non-zero: %w", waitErr)
	}
	return transcript, stdout.Bytes(), stderr.String(), nil
}

func runCommandToolSmoke(ctx context.Context, command []string, timeout time.Duration, clock func() time.Time, tool string, arguments json.RawMessage) ([]transcriptEntry, []byte, string, json.RawMessage, error) {
	smokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, nil, "", nil, errors.New("mcp tool-smoke command must not be empty")
	}
	cmd := exec.CommandContext(smokeCtx, command[0], command[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, "", nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, "", nil, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, "", nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, "", nil, err
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

	send := func(id int, method string, params any) (rpcMessage, error) {
		request := rpcMessage{
			JSONRPC: "2.0",
			ID:      mustRawJSON(id),
			Method:  method,
			Params:  mustRawJSON(params),
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
			return rpcMessage{}, err
		}
		if err := writer.Flush(); err != nil {
			return rpcMessage{}, err
		}
		if !scanner.Scan() {
			if smokeCtx.Err() != nil {
				return rpcMessage{}, fmt.Errorf("mcp tool-smoke timed out waiting for %s response", method)
			}
			if err := scanner.Err(); err != nil {
				return rpcMessage{}, err
			}
			return rpcMessage{}, fmt.Errorf("mcp tool-smoke server closed stdout before %s response", method)
		}
		line := append([]byte(nil), bytes.TrimSpace(scanner.Bytes())...)
		stdout.Write(line)
		stdout.WriteByte('\n')
		var response rpcMessage
		if err := json.Unmarshal(line, &response); err != nil {
			return rpcMessage{}, fmt.Errorf("parse %s response: %w", method, err)
		}
		transcript = append(transcript, transcriptEntry{
			Direction: "response",
			Method:    method,
			ID:        response.ID,
			Timestamp: clock().UTC().Format(time.RFC3339Nano),
			Payload:   append(json.RawMessage(nil), line...),
		})
		if response.Error != nil {
			return response, fmt.Errorf("mcp tool-smoke %s failed: %s", method, response.Error.Message)
		}
		return response, nil
	}

	var toolResponse json.RawMessage
	runErr := func() error {
		if _, err := send(1, "initialize", map[string]any{}); err != nil {
			return err
		}
		toolsList, err := send(2, "tools/list", map[string]any{})
		if err != nil {
			return err
		}
		if !toolListed(toolsList.Result, tool) {
			return fmt.Errorf("mcp tool-smoke tool %q was not listed by fake server", tool)
		}
		call, err := send(3, "tools/call", map[string]any{
			"name":      tool,
			"arguments": arguments,
		})
		if err != nil {
			return err
		}
		toolResponse = append(json.RawMessage(nil), call.Result...)
		if _, err := send(4, "shutdown", map[string]any{}); err != nil {
			return err
		}
		return nil
	}()

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
		return transcript, stdout.Bytes(), stderr.String(), toolResponse, errors.New("mcp tool-smoke timed out")
	}
	if runErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), toolResponse, runErr
	}
	if waitErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), toolResponse, fmt.Errorf("mcp tool-smoke process exited non-zero: %w", waitErr)
	}
	return transcript, stdout.Bytes(), stderr.String(), toolResponse, nil
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

func writeToolArtifacts(result ToolResult, transcript []transcriptEntry, stdout []byte, stderr string) error {
	if err := os.WriteFile(result.SummaryPath, toolSummary(result), 0o600); err != nil {
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

func toolSummary(result ToolResult) []byte {
	var builder strings.Builder
	builder.WriteString("# MCP Tool Smoke\n\n")
	builder.WriteString("Mode: fake/test tool smoke only\n")
	builder.WriteString("Server: " + result.Server + "\n")
	builder.WriteString("Status: " + result.Status + "\n")
	builder.WriteString("Runtime: " + result.Runtime + "\n")
	builder.WriteString("Protocol: " + result.Protocol + "\n")
	builder.WriteString("Tool: " + result.Tool + "\n")
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

func validateToolArguments(raw []byte) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, errors.New("mcp tool-smoke requires --arguments")
	}
	if len(raw) > MaxToolArgumentsBytes {
		return nil, fmt.Errorf("mcp tool-smoke arguments too large: %d bytes exceeds %d", len(raw), MaxToolArgumentsBytes)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("mcp tool-smoke arguments must be valid JSON object: %w", err)
	}
	if object == nil {
		return nil, errors.New("mcp tool-smoke arguments must be a JSON object")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func toolListed(result json.RawMessage, name string) bool {
	var decoded struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(result, &decoded); err != nil {
		return false
	}
	for _, tool := range decoded.Tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func normalizeToolPolicy(policy *ToolPolicy) {
	p := &policy.MCPToolPolicy
	for i := range p.AllowedServers {
		p.AllowedServers[i] = strings.TrimSpace(p.AllowedServers[i])
	}
	for i := range p.AllowedTools {
		p.AllowedTools[i] = strings.TrimSpace(p.AllowedTools[i])
	}
	for i := range p.AllowedCapabilities {
		p.AllowedCapabilities[i] = strings.TrimSpace(p.AllowedCapabilities[i])
	}
}

func hasCapability(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func envIsSet(name string) bool {
	value, ok := os.LookupEnv(name)
	return ok && value != ""
}

func mustRawJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}
