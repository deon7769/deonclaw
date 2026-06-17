package runner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workerconfig"
	"github.com/deon7769/deonclaw/internal/workers"
)

var requiredExecutionTraceEvents = []string{
	"task_loaded",
	"task_validated",
	"model_profile_resolved",
	"env_required_checked",
	"context_pack_built",
	"workspace_prepared",
	"worker_started",
	"worker_finished",
	"validation_completed",
	"diff_captured",
	"path_policy_completed",
	"artifacts_written",
	"workspace_cleanup",
}

type executionTraceOptions struct {
	RunID                    string
	WorkerName               string
	Task                     *tasks.Task
	Result                   *workers.RunResult
	Status                   runs.RunStatus
	StartedAt                time.Time
	FinishedAt               time.Time
	Prompt                   string
	ContextPackMarkdown      []byte
	MCPContextMarkdown       []byte
	RetrievalContextMarkdown []byte
	RetrievalContextAttached bool
	RetrievalContextCount    int
	RetrievalContextStatus   string
	MemoryPolicyPath         string
	RuntimeConfigPath        string
	MCPToolProposal          mcpToolProposalCheck
	EnvRequirements          []workerconfig.EnvRequirementCheck
	WorkerRuntime            string
	Validation               ValidationResult
	ValidationRuntime        string
	PolicyOK                 bool
	ChangedPathCount         int
	Cleanup                  workspaceCleanup
	Timeline                 []executionTraceEvent
}

type executionTrace struct {
	RunID                    string                             `json:"run_id"`
	TaskID                   string                             `json:"task_id"`
	Worker                   string                             `json:"worker"`
	ModelProfile             string                             `json:"model_profile"`
	ModelStrategy            string                             `json:"model_strategy,omitempty"`
	SelectedModelProfile     string                             `json:"selected_model_profile,omitempty"`
	Provider                 string                             `json:"provider"`
	Model                    string                             `json:"model"`
	ModelArg                 string                             `json:"model_arg"`
	CommandDisplay           string                             `json:"command_display"`
	PromptSHA256             string                             `json:"prompt_sha256"`
	ContextPackSHA256        string                             `json:"context_pack_sha256,omitempty"`
	MCPContextSHA256         string                             `json:"mcp_context_sha256,omitempty"`
	RetrievalContextAttached bool                               `json:"retrieval_context_attached"`
	RetrievalContextCount    int                                `json:"retrieval_context_count"`
	RetrievalContextSHA256   string                             `json:"retrieval_context_sha256,omitempty"`
	RetrievalContextStatus   string                             `json:"retrieval_context_status"`
	MCPToolProposalStatus    string                             `json:"mcp_tool_proposal_status"`
	MCPToolProposalSHA256    string                             `json:"mcp_tool_proposal_sha256,omitempty"`
	MemoryPolicySHA256       string                             `json:"memory_policy_sha256,omitempty"`
	RuntimeConfigSHA256      string                             `json:"runtime_config_sha256,omitempty"`
	EnvRequirements          []workerconfig.EnvRequirementCheck `json:"env_requirements"`
	WorkerRuntime            string                             `json:"worker_runtime"`
	StartedAt                string                             `json:"started_at"`
	FinishedAt               string                             `json:"finished_at"`
	DurationMS               int64                              `json:"duration_ms"`
	Status                   string                             `json:"status"`
	StdoutFormat             string                             `json:"stdout_format"`
	ParsedEvents             int                                `json:"parsed_events"`
	ParseWarnings            int                                `json:"parse_warnings"`
	ValidationStatus         string                             `json:"validation_status"`
	ValidationRuntime        string                             `json:"validation_runtime"`
	PolicyStatus             string                             `json:"policy_status"`
	ChangedPathsCount        int                                `json:"changed_paths_count"`
	CleanupAction            string                             `json:"cleanup_action"`
	CleanupReason            string                             `json:"cleanup_reason"`
	Timeline                 []executionTraceEvent              `json:"timeline"`
}

type executionTraceEvent struct {
	Event  string `json:"event"`
	At     string `json:"at,omitempty"`
	Status string `json:"status,omitempty"`
}

type executionTimeline struct {
	events []executionTraceEvent
}

func newExecutionTimeline() *executionTimeline {
	return &executionTimeline{}
}

func (t *executionTimeline) Mark(event string) {
	t.MarkStatus(event, "completed")
}

func (t *executionTimeline) MarkStatus(event string, status string) {
	if t == nil {
		return
	}
	event = strings.TrimSpace(event)
	if event == "" {
		return
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "completed"
	}
	t.events = append(t.events, executionTraceEvent{
		Event:  event,
		At:     time.Now().UTC().Format(time.RFC3339Nano),
		Status: status,
	})
}

func (t *executionTimeline) Events() []executionTraceEvent {
	if t == nil {
		return nil
	}
	return append([]executionTraceEvent(nil), t.events...)
}

func executionTraceJSON(opts executionTraceOptions) ([]byte, error) {
	result := opts.Result
	if result == nil {
		result = &workers.RunResult{}
	}
	task := opts.Task
	metadata := result.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}

	taskID := ""
	modelProfile := metadata["model_profile"]
	if task != nil {
		taskID = task.ID
		if modelProfile == "" {
			modelProfile = task.ModelProfile
		}
	}
	worker := strings.TrimSpace(opts.WorkerName)
	if worker == "" {
		worker = strings.TrimSpace(result.Worker)
	}

	prompt := opts.Prompt
	if strings.TrimSpace(prompt) == "" && task != nil {
		prompt = task.Goal
	}
	trace := executionTrace{
		RunID:                 opts.RunID,
		TaskID:                taskID,
		Worker:                worker,
		ModelProfile:          modelProfile,
		ModelStrategy:         metadata["model_strategy"],
		SelectedModelProfile:  metadata["selected_model_profile"],
		Provider:              metadata["provider"],
		Model:                 metadata["model"],
		ModelArg:              metadata["model_arg"],
		CommandDisplay:        strings.Join(result.Command, " "),
		PromptSHA256:          sha256Hex([]byte(prompt)),
		MCPToolProposalStatus: mcpToolProposalStatusForTrace(opts.MCPToolProposal),
		MCPToolProposalSHA256: opts.MCPToolProposal.ProposalSHA256,
		EnvRequirements:       append([]workerconfig.EnvRequirementCheck(nil), opts.EnvRequirements...),
		WorkerRuntime:         workerRuntimeForTrace(opts),
		StartedAt:             opts.StartedAt.Format(time.RFC3339Nano),
		FinishedAt:            opts.FinishedAt.Format(time.RFC3339Nano),
		DurationMS:            durationMillis(opts.StartedAt, opts.FinishedAt),
		Status:                string(opts.Status),
		StdoutFormat:          metadataDefault(metadata, "opencode.stdout_format", "unknown"),
		ParsedEvents:          metadataInt(metadata, "opencode.parsed_events"),
		ParseWarnings:         metadataInt(metadata, "opencode.parse_warnings"),
		ValidationStatus:      opts.Validation.Status,
		ValidationRuntime:     validationRuntimeForTrace(opts),
		PolicyStatus:          policyTraceStatus(opts.PolicyOK),
		ChangedPathsCount:     opts.ChangedPathCount,
		CleanupAction:         opts.Cleanup.Action,
		CleanupReason:         string(opts.Cleanup.Reason),
		Timeline:              completedTraceTimeline(opts.Timeline),
	}
	if len(opts.ContextPackMarkdown) > 0 {
		trace.ContextPackSHA256 = sha256Hex(opts.ContextPackMarkdown)
	}
	if len(opts.MCPContextMarkdown) > 0 {
		trace.MCPContextSHA256 = sha256Hex(opts.MCPContextMarkdown)
	}
	trace.RetrievalContextAttached = opts.RetrievalContextAttached
	trace.RetrievalContextCount = opts.RetrievalContextCount
	trace.RetrievalContextStatus = strings.TrimSpace(opts.RetrievalContextStatus)
	if trace.RetrievalContextStatus == "" {
		trace.RetrievalContextStatus = "skipped"
	}
	if len(opts.RetrievalContextMarkdown) > 0 {
		trace.RetrievalContextSHA256 = sha256Hex(opts.RetrievalContextMarkdown)
	}
	if hash := fileSHA256IfReadable(opts.MemoryPolicyPath); hash != "" {
		trace.MemoryPolicySHA256 = hash
	}
	if hash := fileSHA256IfReadable(opts.RuntimeConfigPath); hash != "" {
		trace.RuntimeConfigSHA256 = hash
	}
	if trace.EnvRequirements == nil {
		trace.EnvRequirements = []workerconfig.EnvRequirementCheck{}
	}

	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(trace); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func mcpToolProposalStatusForTrace(check mcpToolProposalCheck) string {
	if strings.TrimSpace(check.Status) != "" {
		return check.Status
	}
	return mcpToolProposalStatusNone
}

func workerRuntimeForTrace(opts executionTraceOptions) string {
	if runtime := strings.TrimSpace(opts.WorkerRuntime); runtime != "" {
		return runtime
	}
	return WorkerRuntimeLocal
}

func validationRuntimeForTrace(opts executionTraceOptions) string {
	if runtime := strings.TrimSpace(opts.Validation.Runtime); runtime != "" {
		return runtime
	}
	if runtime := strings.TrimSpace(opts.ValidationRuntime); runtime != "" {
		return runtime
	}
	return tasks.ValidationRuntimeLocal
}

func completedTraceTimeline(events []executionTraceEvent) []executionTraceEvent {
	completed := append([]executionTraceEvent(nil), events...)
	seen := make(map[string]struct{}, len(completed))
	for _, event := range completed {
		if event.Event != "" {
			seen[event.Event] = struct{}{}
		}
	}
	for _, event := range requiredExecutionTraceEvents {
		if _, ok := seen[event]; ok {
			continue
		}
		completed = append(completed, executionTraceEvent{
			Event:  event,
			Status: "not_reached",
		})
	}
	return completed
}

func sha256Hex(content []byte) string {
	hash := sha256.Sum256(content)
	return hex.EncodeToString(hash[:])
}

func fileSHA256IfReadable(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return sha256Hex(content)
}

func metadataDefault(metadata map[string]string, key string, fallback string) string {
	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return fallback
	}
	return value
}

func metadataInt(metadata map[string]string, key string) int {
	value := strings.TrimSpace(metadata[key])
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func durationMillis(startedAt time.Time, finishedAt time.Time) int64 {
	if startedAt.IsZero() || finishedAt.IsZero() || finishedAt.Before(startedAt) {
		return 0
	}
	return finishedAt.Sub(startedAt).Milliseconds()
}

func policyTraceStatus(ok bool) string {
	if ok {
		return "ok"
	}
	return "failed"
}
