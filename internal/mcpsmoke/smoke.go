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

	FakeEchoToolName        = "deonclaw.fake.echo"
	MaxToolArgumentsBytes   = 64 * 1024
	MaxToolSchemaBytes      = 16 * 1024
	DefaultMaxResponseBytes = 1024 * 1024
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

type DiscoveryOptions struct {
	Config         mcpconfig.Config
	Server         string
	ArtifactsDir   string
	Timeout        time.Duration
	Runtime        string
	RuntimeConfig  *runtimeconfig.Config
	Workspace      string
	Policy         *DiscoveryPolicy
	StartedAtClock func() time.Time
}

type DiscoveryResult struct {
	Server         string   `json:"server"`
	Status         string   `json:"status"`
	Runtime        string   `json:"runtime"`
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
	ToolsListPath  string   `json:"tools_list_path"`
	ToolCount      int      `json:"tool_count"`
	ToolNames      []string `json:"tool_names,omitempty"`
	StartedAt      string   `json:"started_at"`
	FinishedAt     string   `json:"finished_at"`
	DurationMS     int64    `json:"duration_ms"`
	Error          string   `json:"error,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

type CallOptions struct {
	Config         mcpconfig.Config
	Server         string
	Tool           string
	Arguments      []byte
	ArtifactsDir   string
	Timeout        time.Duration
	Runtime        string
	RuntimeConfig  *runtimeconfig.Config
	Workspace      string
	Policy         *CallPolicy
	StartedAtClock func() time.Time
}

type CallResult struct {
	Server            string   `json:"server"`
	Status            string   `json:"status"`
	Runtime           string   `json:"runtime"`
	TestOnly          bool     `json:"test_only"`
	Protocol          string   `json:"protocol"`
	Tool              string   `json:"tool"`
	ToolCalls         int      `json:"tool_calls"`
	Command           []string `json:"command"`
	ArtifactsDir      string   `json:"artifacts_dir"`
	SummaryPath       string   `json:"summary_path"`
	TranscriptPath    string   `json:"transcript_path"`
	StdoutPath        string   `json:"stdout_path"`
	StderrPath        string   `json:"stderr_path"`
	ResultPath        string   `json:"result_path"`
	ResponsePath      string   `json:"response_path"`
	ResponseBytes     int      `json:"response_bytes"`
	MaxResponseBytes  int      `json:"max_response_bytes"`
	ResponseTruncated bool     `json:"response_truncated"`
	StartedAt         string   `json:"started_at"`
	FinishedAt        string   `json:"finished_at"`
	DurationMS        int64    `json:"duration_ms"`
	Error             string   `json:"error,omitempty"`
	Warnings          []string `json:"warnings,omitempty"`
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

type DiscoveryPolicy struct {
	MCPDiscoveryPolicy MCPDiscoveryPolicy `yaml:"mcp_discovery_policy" json:"mcp_discovery_policy"`
}

type MCPDiscoveryPolicy struct {
	AllowRealReadonly    bool     `yaml:"allow_real_readonly" json:"allow_real_readonly"`
	MaxToolCalls         int      `yaml:"max_tool_calls" json:"max_tool_calls"`
	AllowedServers       []string `yaml:"allowed_servers,omitempty" json:"allowed_servers,omitempty"`
	AllowedCapabilities  []string `yaml:"allowed_capabilities,omitempty" json:"allowed_capabilities,omitempty"`
	RequireDockerForReal bool     `yaml:"require_docker_for_real" json:"require_docker_for_real"`
}

type CallPolicy struct {
	MCPCallPolicy MCPCallPolicy `yaml:"mcp_call_policy" json:"mcp_call_policy"`
}

type MCPCallPolicy struct {
	AllowRealReadonly    bool     `yaml:"allow_real_readonly" json:"allow_real_readonly"`
	RequireDockerForReal bool     `yaml:"require_docker_for_real" json:"require_docker_for_real"`
	MaxToolCalls         int      `yaml:"max_tool_calls" json:"max_tool_calls"`
	AllowedServers       []string `yaml:"allowed_servers,omitempty" json:"allowed_servers,omitempty"`
	AllowedTools         []string `yaml:"allowed_tools,omitempty" json:"allowed_tools,omitempty"`
	AllowedCapabilities  []string `yaml:"allowed_capabilities,omitempty" json:"allowed_capabilities,omitempty"`
	MaxArgumentsBytes    int      `yaml:"max_arguments_bytes" json:"max_arguments_bytes"`
	MaxResponseBytes     int      `yaml:"max_response_bytes" json:"max_response_bytes"`
}

type CallResponseArtifact struct {
	Tool                  string          `json:"tool"`
	Response              json.RawMessage `json:"response,omitempty"`
	ResponsePreview       string          `json:"response_preview,omitempty"`
	ResponseTruncated     bool            `json:"response_truncated"`
	ResponseOriginalBytes int             `json:"response_original_bytes"`
	ResponseStoredBytes   int             `json:"response_stored_bytes"`
	MaxResponseBytes      int             `json:"max_response_bytes"`
}

type ToolsListArtifact struct {
	ToolCount  int                  `json:"tool_count"`
	ToolNames  []string             `json:"tool_names"`
	Tools      []DiscoveredToolMeta `json:"tools"`
	Truncation ToolsListTruncation  `json:"truncation"`
}

type DiscoveredToolMeta struct {
	Name                     string          `json:"name"`
	Description              string          `json:"description,omitempty"`
	InputSchema              json.RawMessage `json:"input_schema,omitempty"`
	InputSchemaPreview       string          `json:"input_schema_preview,omitempty"`
	InputSchemaTruncated     bool            `json:"input_schema_truncated,omitempty"`
	InputSchemaOriginalBytes int             `json:"input_schema_original_bytes,omitempty"`
	InputSchemaStoredBytes   int             `json:"input_schema_stored_bytes,omitempty"`
}

type ToolsListTruncation struct {
	SchemaLimitBytes int  `json:"schema_limit_bytes"`
	Applied          bool `json:"applied"`
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

func Discover(ctx context.Context, opts DiscoveryOptions) (DiscoveryResult, error) {
	startWall := time.Now()
	clock := opts.StartedAtClock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	startedAt := clock().UTC()
	result := DiscoveryResult{
		Server:       strings.TrimSpace(opts.Server),
		Runtime:      normalizeRuntime(opts.Runtime),
		Protocol:     mcpconfig.ProtocolStdio,
		ArtifactsDir: opts.ArtifactsDir,
		StartedAt:    startedAt.Format(time.RFC3339Nano),
	}
	if opts.Timeout <= 0 {
		return result, errors.New("mcp discover timeout must be greater than zero")
	}
	if opts.ArtifactsDir == "" {
		return result, errors.New("mcp discover requires artifacts dir")
	}
	if opts.Policy == nil {
		return result, errors.New("mcp discover requires --policy")
	}
	policy := *opts.Policy
	if err := ValidateDiscoveryPolicy(policy); err != nil {
		return result, err
	}
	if _, err := mcpconfig.Validate(opts.Config); err != nil {
		return result, err
	}
	server, err := selectSmokeServer(opts.Config, opts.Server)
	if err != nil {
		return result, err
	}
	if err := validateDiscoveryServer(policy, result.Server, server, result.Runtime); err != nil {
		return result, err
	}
	if err := validateServerEnvPassthrough(server); err != nil {
		return result, err
	}
	secretValues := discoveryRedactionValues(server, opts.RuntimeConfig)
	result.Protocol = server.Protocol
	result.TestOnly = server.TestOnly
	serverCommand := append([]string{server.Command}, server.Args...)
	runCommand := append([]string(nil), serverCommand...)
	switch result.Runtime {
	case RuntimeLocal:
	case RuntimeDocker:
		if opts.RuntimeConfig == nil {
			return result, errors.New("mcp discover runtime docker requires --runtime-config")
		}
		plan, err := mcpconfig.PlanDockerLaunch(opts.Config, opts.Server, *opts.RuntimeConfig, opts.Workspace)
		if err != nil {
			return result, err
		}
		runCommand = append([]string(nil), plan.Command...)
		result.Warnings = append(result.Warnings, plan.Warnings...)
	default:
		return result, fmt.Errorf("mcp discover runtime %q is not supported", result.Runtime)
	}
	result.Command = append([]string(nil), runCommand...)

	if err := os.MkdirAll(opts.ArtifactsDir, 0o755); err != nil {
		return result, fmt.Errorf("create mcp discover artifacts dir: %w", err)
	}
	result.SummaryPath = filepath.Join(opts.ArtifactsDir, "mcp-discovery-summary.md")
	result.TranscriptPath = filepath.Join(opts.ArtifactsDir, "mcp-discovery-transcript.jsonl")
	result.StdoutPath = filepath.Join(opts.ArtifactsDir, "mcp-discovery-stdout.log")
	result.StderrPath = filepath.Join(opts.ArtifactsDir, "mcp-discovery-stderr.log")
	result.ResultPath = filepath.Join(opts.ArtifactsDir, "mcp-discovery-result.json")
	result.ToolsListPath = filepath.Join(opts.ArtifactsDir, "mcp-tools-list.json")

	transcript, rawStdout, rawStderr, toolsListRaw, runErr := runCommandDiscovery(ctx, runCommand, opts.Timeout, clock)
	toolsListArtifact := buildToolsListArtifact(toolsListRaw)
	result.ToolCount = toolsListArtifact.ToolCount
	result.ToolNames = append([]string(nil), toolsListArtifact.ToolNames...)
	finishedAt := clock().UTC()
	result.FinishedAt = finishedAt.Format(time.RFC3339Nano)
	result.DurationMS = time.Since(startWall).Milliseconds()
	if runErr != nil {
		result.Status = StatusFailed
		result.Error = runErr.Error()
		_ = writeDiscoveryArtifacts(result, toolsListArtifact, transcript, rawStdout, rawStderr, secretValues)
		return result, runErr
	}
	result.Status = StatusSucceeded
	result.ToolCalls = 0
	if err := writeDiscoveryArtifacts(result, toolsListArtifact, transcript, rawStdout, rawStderr, secretValues); err != nil {
		return result, err
	}
	return result, nil
}

func CallSmoke(ctx context.Context, opts CallOptions) (CallResult, error) {
	startWall := time.Now()
	clock := opts.StartedAtClock
	if clock == nil {
		clock = func() time.Time { return time.Now().UTC() }
	}
	startedAt := clock().UTC()
	result := CallResult{
		Server:       strings.TrimSpace(opts.Server),
		Runtime:      normalizeRuntime(opts.Runtime),
		Protocol:     mcpconfig.ProtocolStdio,
		Tool:         strings.TrimSpace(opts.Tool),
		ArtifactsDir: opts.ArtifactsDir,
		StartedAt:    startedAt.Format(time.RFC3339Nano),
	}
	if opts.Timeout <= 0 {
		return result, errors.New("mcp call-smoke timeout must be greater than zero")
	}
	if opts.ArtifactsDir == "" {
		return result, errors.New("mcp call-smoke requires artifacts dir")
	}
	if result.Tool == "" {
		return result, errors.New("mcp call-smoke requires --tool")
	}
	if opts.Policy == nil {
		return result, errors.New("mcp call-smoke requires --policy")
	}
	policy := *opts.Policy
	if err := ValidateCallPolicy(policy); err != nil {
		return result, err
	}
	arguments, err := validateCallArguments(opts.Arguments, policy.MCPCallPolicy.MaxArgumentsBytes)
	if err != nil {
		return result, err
	}
	if _, err := mcpconfig.Validate(opts.Config); err != nil {
		return result, err
	}
	server, err := selectSmokeServer(opts.Config, opts.Server)
	if err != nil {
		return result, err
	}
	if err := validateCallServer(policy, result.Server, server, result.Tool, result.Runtime); err != nil {
		return result, err
	}
	if err := validateServerEnvPassthrough(server); err != nil {
		return result, err
	}
	secretValues := discoveryRedactionValues(server, opts.RuntimeConfig)
	result.Protocol = server.Protocol
	result.TestOnly = server.TestOnly
	result.MaxResponseBytes = policy.MCPCallPolicy.MaxResponseBytes
	serverCommand := append([]string{server.Command}, server.Args...)
	runCommand := append([]string(nil), serverCommand...)
	switch result.Runtime {
	case RuntimeLocal:
	case RuntimeDocker:
		if opts.RuntimeConfig == nil {
			return result, errors.New("mcp call-smoke runtime docker requires --runtime-config")
		}
		plan, err := mcpconfig.PlanDockerLaunch(opts.Config, opts.Server, *opts.RuntimeConfig, opts.Workspace)
		if err != nil {
			return result, err
		}
		runCommand = append([]string(nil), plan.Command...)
		result.Warnings = append(result.Warnings, plan.Warnings...)
	default:
		return result, fmt.Errorf("mcp call-smoke runtime %q is not supported", result.Runtime)
	}
	result.Command = append([]string(nil), runCommand...)

	if err := os.MkdirAll(opts.ArtifactsDir, 0o755); err != nil {
		return result, fmt.Errorf("create mcp call-smoke artifacts dir: %w", err)
	}
	result.SummaryPath = filepath.Join(opts.ArtifactsDir, "mcp-call-smoke-summary.md")
	result.TranscriptPath = filepath.Join(opts.ArtifactsDir, "mcp-call-transcript.jsonl")
	result.StdoutPath = filepath.Join(opts.ArtifactsDir, "mcp-call-stdout.log")
	result.StderrPath = filepath.Join(opts.ArtifactsDir, "mcp-call-stderr.log")
	result.ResultPath = filepath.Join(opts.ArtifactsDir, "mcp-call-result.json")
	result.ResponsePath = filepath.Join(opts.ArtifactsDir, "mcp-call-response.json")

	transcript, rawStdout, rawStderr, toolResponse, runErr := runCommandCallSmoke(ctx, runCommand, opts.Timeout, clock, result.Tool, arguments)
	responseArtifact := buildCallResponseArtifact(result.Tool, toolResponse, policy.MCPCallPolicy.MaxResponseBytes)
	result.ResponseBytes = responseArtifact.ResponseOriginalBytes
	result.ResponseTruncated = responseArtifact.ResponseTruncated
	finishedAt := clock().UTC()
	result.FinishedAt = finishedAt.Format(time.RFC3339Nano)
	result.DurationMS = time.Since(startWall).Milliseconds()
	if runErr != nil {
		result.Status = StatusFailed
		result.Error = runErr.Error()
		_ = writeCallArtifacts(result, responseArtifact, transcript, rawStdout, rawStderr, secretValues)
		return result, runErr
	}
	result.Status = StatusSucceeded
	result.ToolCalls = 1
	if err := writeCallArtifacts(result, responseArtifact, transcript, rawStdout, rawStderr, secretValues); err != nil {
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

func LoadDiscoveryPolicy(path string) (DiscoveryPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DiscoveryPolicy{}, fmt.Errorf("read MCP discovery policy %q: %w", path, err)
	}
	var policy DiscoveryPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return DiscoveryPolicy{}, fmt.Errorf("parse MCP discovery policy yaml: %w", err)
	}
	normalizeDiscoveryPolicy(&policy)
	return policy, nil
}

func ValidateDiscoveryPolicy(policy DiscoveryPolicy) error {
	var errs []error
	p := policy.MCPDiscoveryPolicy
	if !p.AllowRealReadonly {
		errs = append(errs, errors.New("mcp_discovery_policy.allow_real_readonly must be true"))
	}
	if p.MaxToolCalls != 0 {
		errs = append(errs, errors.New("mcp_discovery_policy.max_tool_calls must be 0"))
	}
	if len(p.AllowedServers) == 0 {
		errs = append(errs, errors.New("mcp_discovery_policy.allowed_servers must not be empty"))
	}
	if len(p.AllowedCapabilities) == 0 {
		errs = append(errs, errors.New("mcp_discovery_policy.allowed_capabilities must not be empty"))
	}
	for _, capability := range p.AllowedCapabilities {
		switch capability {
		case mcpconfig.CapabilityRead:
		case mcpconfig.CapabilityWrite, mcpconfig.CapabilityExec:
			errs = append(errs, fmt.Errorf("mcp_discovery_policy.allowed_capabilities %q is not permitted", capability))
		default:
			errs = append(errs, fmt.Errorf("mcp_discovery_policy.allowed_capabilities %q is not supported", capability))
		}
	}
	return errors.Join(errs...)
}

func LoadCallPolicy(path string) (CallPolicy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CallPolicy{}, fmt.Errorf("read MCP call policy %q: %w", path, err)
	}
	var policy CallPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return CallPolicy{}, fmt.Errorf("parse MCP call policy yaml: %w", err)
	}
	normalizeCallPolicy(&policy)
	return policy, nil
}

func ValidateCallPolicy(policy CallPolicy) error {
	var errs []error
	p := policy.MCPCallPolicy
	if !p.AllowRealReadonly {
		errs = append(errs, errors.New("mcp_call_policy.allow_real_readonly must be true"))
	}
	if p.MaxToolCalls != 1 {
		errs = append(errs, errors.New("mcp_call_policy.max_tool_calls must be 1"))
	}
	if len(p.AllowedServers) == 0 {
		errs = append(errs, errors.New("mcp_call_policy.allowed_servers must not be empty"))
	}
	if len(p.AllowedTools) == 0 {
		errs = append(errs, errors.New("mcp_call_policy.allowed_tools must not be empty"))
	}
	if len(p.AllowedCapabilities) == 0 {
		errs = append(errs, errors.New("mcp_call_policy.allowed_capabilities must not be empty"))
	}
	for _, capability := range p.AllowedCapabilities {
		switch capability {
		case mcpconfig.CapabilityRead:
		case mcpconfig.CapabilityWrite, mcpconfig.CapabilityExec:
			errs = append(errs, fmt.Errorf("mcp_call_policy.allowed_capabilities %q is not permitted", capability))
		default:
			errs = append(errs, fmt.Errorf("mcp_call_policy.allowed_capabilities %q is not supported", capability))
		}
	}
	if p.MaxArgumentsBytes < 1 {
		errs = append(errs, errors.New("mcp_call_policy.max_arguments_bytes must be >= 1"))
	}
	if p.MaxResponseBytes < 1 {
		errs = append(errs, errors.New("mcp_call_policy.max_response_bytes must be >= 1"))
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

func validateDiscoveryServer(policy DiscoveryPolicy, serverName string, server mcpconfig.ServerConfig, runtime string) error {
	p := policy.MCPDiscoveryPolicy
	if server.Enabled {
		return errors.New("mcp discover requires enabled=false")
	}
	if server.Protocol != mcpconfig.ProtocolStdio {
		return fmt.Errorf("mcp discover requires protocol=%s", mcpconfig.ProtocolStdio)
	}
	if len(p.AllowedServers) > 0 && !hasString(p.AllowedServers, serverName) {
		return fmt.Errorf("mcp discover server %q is not allowlisted", serverName)
	}
	if len(server.Capabilities) == 0 {
		return errors.New("mcp discover requires read capability")
	}
	for _, capability := range server.Capabilities {
		switch capability {
		case mcpconfig.CapabilityRead:
			if len(p.AllowedCapabilities) > 0 && !hasString(p.AllowedCapabilities, capability) {
				return fmt.Errorf("mcp discover capability %q is not allowlisted", capability)
			}
		case mcpconfig.CapabilityWrite, mcpconfig.CapabilityExec:
			return errors.New("mcp discover refuses servers with write or exec capability")
		default:
			return fmt.Errorf("mcp discover capability %q is not supported", capability)
		}
	}
	if !server.TestOnly {
		if !p.AllowRealReadonly {
			return errors.New("mcp discover real read-only servers require allow_real_readonly=true")
		}
		if p.RequireDockerForReal && runtime != RuntimeDocker {
			return errors.New("mcp discover requires --runtime docker for real read-only servers")
		}
	}
	return nil
}

func validateCallServer(policy CallPolicy, serverName string, server mcpconfig.ServerConfig, tool string, runtime string) error {
	p := policy.MCPCallPolicy
	if server.Enabled {
		return errors.New("mcp call-smoke requires enabled=false")
	}
	if server.Protocol != mcpconfig.ProtocolStdio {
		return fmt.Errorf("mcp call-smoke requires protocol=%s", mcpconfig.ProtocolStdio)
	}
	if len(p.AllowedServers) > 0 && !hasString(p.AllowedServers, serverName) {
		return fmt.Errorf("mcp call-smoke server %q is not allowlisted", serverName)
	}
	if len(p.AllowedTools) > 0 && !hasString(p.AllowedTools, tool) {
		return fmt.Errorf("mcp call-smoke tool %q is not allowlisted", tool)
	}
	if len(server.Capabilities) == 0 {
		return errors.New("mcp call-smoke requires read capability")
	}
	for _, capability := range server.Capabilities {
		switch capability {
		case mcpconfig.CapabilityRead:
			if len(p.AllowedCapabilities) > 0 && !hasString(p.AllowedCapabilities, capability) {
				return fmt.Errorf("mcp call-smoke capability %q is not allowlisted", capability)
			}
		case mcpconfig.CapabilityWrite, mcpconfig.CapabilityExec:
			return errors.New("mcp call-smoke refuses servers with write or exec capability")
		default:
			return fmt.Errorf("mcp call-smoke capability %q is not supported", capability)
		}
	}
	if !server.TestOnly {
		if !p.AllowRealReadonly {
			return errors.New("mcp call-smoke real read-only servers require allow_real_readonly=true")
		}
		if p.RequireDockerForReal && runtime != RuntimeDocker {
			return errors.New("mcp call-smoke requires --runtime docker for real read-only servers")
		}
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

func runCommandDiscovery(ctx context.Context, command []string, timeout time.Duration, clock func() time.Time) ([]transcriptEntry, []byte, string, json.RawMessage, error) {
	smokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, nil, "", nil, errors.New("mcp discover command must not be empty")
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
				return rpcMessage{}, fmt.Errorf("mcp discover timed out waiting for %s response", method)
			}
			if err := scanner.Err(); err != nil {
				return rpcMessage{}, err
			}
			return rpcMessage{}, fmt.Errorf("mcp discover server closed stdout before %s response", method)
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
			return response, fmt.Errorf("mcp discover %s failed: %s", method, response.Error.Message)
		}
		return response, nil
	}

	var toolsList json.RawMessage
	runErr := func() error {
		if _, err := send(1, "initialize", map[string]any{}); err != nil {
			return err
		}
		response, err := send(2, "tools/list", map[string]any{})
		if err != nil {
			return err
		}
		toolsList = append(json.RawMessage(nil), response.Result...)
		if _, err := send(3, "shutdown", map[string]any{}); err != nil {
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
		return transcript, stdout.Bytes(), stderr.String(), toolsList, errors.New("mcp discover timed out")
	}
	if runErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), toolsList, runErr
	}
	if waitErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), toolsList, fmt.Errorf("mcp discover process exited non-zero: %w", waitErr)
	}
	return transcript, stdout.Bytes(), stderr.String(), toolsList, nil
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

func runCommandCallSmoke(ctx context.Context, command []string, timeout time.Duration, clock func() time.Time, tool string, arguments json.RawMessage) ([]transcriptEntry, []byte, string, json.RawMessage, error) {
	smokeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, nil, "", nil, errors.New("mcp call-smoke command must not be empty")
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
				return rpcMessage{}, fmt.Errorf("mcp call-smoke timed out waiting for %s response", method)
			}
			if err := scanner.Err(); err != nil {
				return rpcMessage{}, err
			}
			return rpcMessage{}, fmt.Errorf("mcp call-smoke server closed stdout before %s response", method)
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
			return response, fmt.Errorf("mcp call-smoke %s failed: %s", method, response.Error.Message)
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
			return fmt.Errorf("mcp call-smoke tool %q was not listed by server", tool)
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
		return transcript, stdout.Bytes(), stderr.String(), toolResponse, errors.New("mcp call-smoke timed out")
	}
	if runErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), toolResponse, runErr
	}
	if waitErr != nil {
		return transcript, stdout.Bytes(), stderr.String(), toolResponse, fmt.Errorf("mcp call-smoke process exited non-zero: %w", waitErr)
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

func writeDiscoveryArtifacts(result DiscoveryResult, toolsList ToolsListArtifact, transcript []transcriptEntry, stdout []byte, stderr string, redactions []string) error {
	if err := os.WriteFile(result.SummaryPath, redactBytes(discoverySummary(result), redactions), 0o600); err != nil {
		return err
	}
	transcriptJSONL, err := transcriptJSONL(transcript)
	if err != nil {
		return err
	}
	if err := os.WriteFile(result.TranscriptPath, redactBytes(transcriptJSONL, redactions), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(result.StdoutPath, redactBytes(stdout, redactions), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(result.StderrPath, redactBytes([]byte(stderr), redactions), 0o600); err != nil {
		return err
	}
	resultData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(result.ResultPath, append(redactBytes(resultData, redactions), '\n'), 0o600); err != nil {
		return err
	}
	toolsData, err := json.MarshalIndent(toolsList, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(result.ToolsListPath, append(redactBytes(toolsData, redactions), '\n'), 0o600)
}

func writeCallArtifacts(result CallResult, response CallResponseArtifact, transcript []transcriptEntry, stdout []byte, stderr string, redactions []string) error {
	if err := writeRedactedCallArtifact(result.SummaryPath, "mcp-call-smoke-summary.md", callSummary(result), redactions); err != nil {
		return err
	}
	transcriptJSONL, err := transcriptJSONL(transcript)
	if err != nil {
		return err
	}
	if err := writeRedactedCallArtifact(result.TranscriptPath, "mcp-call-transcript.jsonl", transcriptJSONL, redactions); err != nil {
		return err
	}
	if err := writeRedactedCallArtifact(result.StdoutPath, "mcp-call-stdout.log", stdout, redactions); err != nil {
		return err
	}
	if err := writeRedactedCallArtifact(result.StderrPath, "mcp-call-stderr.log", []byte(stderr), redactions); err != nil {
		return err
	}
	resultData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	if err := writeRedactedCallArtifact(result.ResultPath, "mcp-call-result.json", append(resultData, '\n'), redactions); err != nil {
		return err
	}
	responseData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return err
	}
	return writeRedactedCallArtifact(result.ResponsePath, "mcp-call-response.json", append(responseData, '\n'), redactions)
}

func writeRedactedCallArtifact(path string, label string, data []byte, redactions []string) error {
	redacted := redactBytes(data, redactions)
	if err := scanRedactionLeaks(label, redacted, redactions); err != nil {
		return err
	}
	return os.WriteFile(path, redacted, 0o600)
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

func callSummary(result CallResult) []byte {
	var builder strings.Builder
	builder.WriteString("# MCP Call Smoke\n\n")
	builder.WriteString("Mode: read-only call smoke only; no worker integration\n")
	builder.WriteString("Server: " + result.Server + "\n")
	builder.WriteString("Status: " + result.Status + "\n")
	builder.WriteString("Runtime: " + result.Runtime + "\n")
	builder.WriteString("Protocol: " + result.Protocol + "\n")
	builder.WriteString("Tool: " + result.Tool + "\n")
	builder.WriteString(fmt.Sprintf("Tool calls: %d\n", result.ToolCalls))
	builder.WriteString(fmt.Sprintf("tool_call_count: %d\n", result.ToolCalls))
	builder.WriteString(fmt.Sprintf("Response bytes: %d\n", result.ResponseBytes))
	builder.WriteString(fmt.Sprintf("Max response bytes: %d\n", result.MaxResponseBytes))
	builder.WriteString(fmt.Sprintf("Response truncated: %t\n", result.ResponseTruncated))
	builder.WriteString(fmt.Sprintf("response_truncated: %t\n", result.ResponseTruncated))
	if result.Error != "" {
		builder.WriteString("Error: " + result.Error + "\n")
	}
	return []byte(builder.String())
}

func discoverySummary(result DiscoveryResult) []byte {
	var builder strings.Builder
	builder.WriteString("# MCP Discovery Smoke\n\n")
	builder.WriteString("Mode: read-only discovery only\n")
	builder.WriteString("Server: " + result.Server + "\n")
	builder.WriteString("Status: " + result.Status + "\n")
	builder.WriteString("Runtime: " + result.Runtime + "\n")
	builder.WriteString("Protocol: " + result.Protocol + "\n")
	builder.WriteString(fmt.Sprintf("Tool calls: %d\n", result.ToolCalls))
	builder.WriteString(fmt.Sprintf("Tool count: %d\n", result.ToolCount))
	if len(result.ToolNames) > 0 {
		builder.WriteString("Tools: " + strings.Join(result.ToolNames, ",") + "\n")
	}
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

func validateCallArguments(raw []byte, maxBytes int) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil, errors.New("mcp call-smoke requires --arguments")
	}
	if maxBytes < 1 {
		return nil, errors.New("mcp call-smoke max arguments bytes must be >= 1")
	}
	if len(raw) > maxBytes {
		return nil, fmt.Errorf("mcp call-smoke arguments too large: %d bytes exceeds %d", len(raw), maxBytes)
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("mcp call-smoke arguments must be valid JSON object: %w", err)
	}
	if object == nil {
		return nil, errors.New("mcp call-smoke arguments must be a JSON object")
	}
	return append(json.RawMessage(nil), raw...), nil
}

func buildToolsListArtifact(raw json.RawMessage) ToolsListArtifact {
	artifact := ToolsListArtifact{
		ToolNames:  []string{},
		Tools:      []DiscoveredToolMeta{},
		Truncation: ToolsListTruncation{SchemaLimitBytes: MaxToolSchemaBytes},
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return artifact
	}
	var decoded struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return artifact
	}
	artifact.ToolCount = len(decoded.Tools)
	for _, tool := range decoded.Tools {
		meta := DiscoveredToolMeta{
			Name:        tool.Name,
			Description: tool.Description,
		}
		if len(tool.InputSchema) > 0 {
			meta.InputSchemaOriginalBytes = len(tool.InputSchema)
			if len(tool.InputSchema) > MaxToolSchemaBytes {
				meta.InputSchemaPreview = string(tool.InputSchema[:MaxToolSchemaBytes])
				meta.InputSchemaStoredBytes = MaxToolSchemaBytes
				meta.InputSchemaTruncated = true
				artifact.Truncation.Applied = true
			} else {
				meta.InputSchema = append(json.RawMessage(nil), tool.InputSchema...)
				meta.InputSchemaStoredBytes = len(tool.InputSchema)
			}
		}
		artifact.ToolNames = append(artifact.ToolNames, tool.Name)
		artifact.Tools = append(artifact.Tools, meta)
	}
	return artifact
}

func buildCallResponseArtifact(tool string, raw json.RawMessage, maxBytes int) CallResponseArtifact {
	if maxBytes < 1 {
		maxBytes = DefaultMaxResponseBytes
	}
	artifact := CallResponseArtifact{
		Tool:                  tool,
		ResponseOriginalBytes: len(raw),
		MaxResponseBytes:      maxBytes,
	}
	if len(raw) > maxBytes {
		artifact.ResponsePreview = string(raw[:maxBytes])
		artifact.ResponseStoredBytes = maxBytes
		artifact.ResponseTruncated = true
		return artifact
	}
	artifact.Response = append(json.RawMessage(nil), raw...)
	artifact.ResponseStoredBytes = len(raw)
	return artifact
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

func normalizeCallPolicy(policy *CallPolicy) {
	p := &policy.MCPCallPolicy
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

func normalizeDiscoveryPolicy(policy *DiscoveryPolicy) {
	p := &policy.MCPDiscoveryPolicy
	for i := range p.AllowedServers {
		p.AllowedServers[i] = strings.TrimSpace(p.AllowedServers[i])
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

func discoveryRedactionValues(server mcpconfig.ServerConfig, runtimeCfg *runtimeconfig.Config) []string {
	var names []string
	names = append(names, server.Env.Passthrough...)
	if runtimeCfg != nil {
		names = append(names, runtimeCfg.Runtime.Docker.Env.Passthrough...)
	}
	values := make([]string, 0, len(names))
	seen := map[string]bool{}
	for _, name := range names {
		value, ok := os.LookupEnv(name)
		if !ok || value == "" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	return values
}

func redactBytes(data []byte, values []string) []byte {
	if len(data) == 0 || len(values) == 0 {
		return data
	}
	redacted := append([]byte(nil), data...)
	for _, value := range values {
		if value == "" {
			continue
		}
		redacted = bytes.ReplaceAll(redacted, []byte(value), []byte("[redacted]"))
	}
	return redacted
}

func scanRedactionLeaks(label string, data []byte, values []string) error {
	for _, value := range values {
		if value == "" {
			continue
		}
		if bytes.Contains(data, []byte(value)) {
			return fmt.Errorf("%s contains unredacted env passthrough value", label)
		}
	}
	return nil
}

func mustRawJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return json.RawMessage(data)
}
