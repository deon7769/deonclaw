package mcpapproval

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/mcpconfig"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
)

const (
	ProposalStatusProposed = "proposed"

	SourceTypeManual    = "manual"
	SourceTypeDiscovery = "discovery"

	ApprovalDecisionApproved = "approved"
	ApprovalDecisionRejected = "rejected"

	PreflightStatusPassed = "passed"
	PreflightStatusFailed = "failed"
)

type MCPToolCallProposal struct {
	ID                  string            `json:"id"`
	CreatedAt           time.Time         `json:"created_at"`
	Server              string            `json:"server"`
	Tool                string            `json:"tool"`
	Arguments           json.RawMessage   `json:"arguments"`
	ArgumentsSHA256     string            `json:"arguments_sha256"`
	Reason              string            `json:"reason"`
	RequestedBy         string            `json:"requested_by"`
	Source              MCPToolCallSource `json:"source"`
	PolicyPath          string            `json:"policy_path"`
	ConfigPath          string            `json:"config_path,omitempty"`
	ConfigSHA256        string            `json:"config_sha256,omitempty"`
	Runtime             string            `json:"runtime"`
	RuntimeConfigPath   string            `json:"runtime_config_path,omitempty"`
	RuntimeConfigSHA256 string            `json:"runtime_config_sha256,omitempty"`
	Workspace           string            `json:"workspace,omitempty"`
	Status              string            `json:"status"`
}

type MCPToolCallSource struct {
	Type              string `json:"type"`
	DiscoveryArtifact string `json:"discovery_artifact,omitempty"`
}

type MCPToolCallApproval struct {
	ProposalID          string    `json:"proposal_id"`
	Decision            string    `json:"decision"`
	ApprovedAt          time.Time `json:"approved_at"`
	ApprovedBy          string    `json:"approved_by"`
	Reason              string    `json:"reason"`
	ArgumentsSHA256     string    `json:"arguments_sha256"`
	PolicySHA256        string    `json:"policy_sha256"`
	ConfigSHA256        string    `json:"config_sha256,omitempty"`
	RuntimeConfigSHA256 string    `json:"runtime_config_sha256,omitempty"`
	ConfirmReadOnly     bool      `json:"confirm_read_only"`
}

type MCPToolCallPreflight struct {
	ProposalID          string                     `json:"proposal_id"`
	Status              string                     `json:"status"`
	Checks              MCPToolCallPreflightChecks `json:"checks"`
	PolicySHA256        string                     `json:"policy_sha256,omitempty"`
	ConfigSHA256        string                     `json:"config_sha256,omitempty"`
	RuntimeConfigSHA256 string                     `json:"runtime_config_sha256,omitempty"`
	Warnings            []string                   `json:"warnings"`
	Failures            []string                   `json:"failures,omitempty"`
}

type MCPToolCallPreflightChecks struct {
	PolicyLoaded         bool `json:"policy_loaded"`
	ServerAllowlisted    bool `json:"server_allowlisted"`
	ToolAllowlisted      bool `json:"tool_allowlisted"`
	ReadOnlyCapability   bool `json:"read_only_capability"`
	DockerRequired       bool `json:"docker_required"`
	EnvAvailable         bool `json:"env_available"`
	ArgumentsWithinLimit bool `json:"arguments_within_limit"`
	NoWriteExec          bool `json:"no_write_exec"`
}

type LintResult struct {
	ProposalID string   `json:"proposal_id"`
	Status     string   `json:"status"`
	Violations []string `json:"violations"`
	Warnings   []string `json:"warnings"`
}

type MCPToolCallExecutionBundle struct {
	ProposalID          string            `json:"proposal_id"`
	ApprovalSHA256      string            `json:"approval_sha256"`
	ProposalSHA256      string            `json:"proposal_sha256"`
	PolicySHA256        string            `json:"policy_sha256"`
	ConfigSHA256        string            `json:"config_sha256,omitempty"`
	RuntimeConfigSHA256 string            `json:"runtime_config_sha256,omitempty"`
	PreflightStatus     string            `json:"preflight_status"`
	Server              string            `json:"server"`
	Tool                string            `json:"tool"`
	ArgumentsSHA256     string            `json:"arguments_sha256"`
	Runtime             string            `json:"runtime"`
	Artifacts           map[string]string `json:"artifacts"`
	Status              string            `json:"status"`
	StartedAt           string            `json:"started_at"`
	FinishedAt          string            `json:"finished_at"`
	ResponseTruncated   bool              `json:"response_truncated"`
	ToolCalls           int               `json:"tool_calls"`
}

type NewProposalOptions struct {
	ID                string
	CreatedAt         time.Time
	Server            string
	Tool              string
	Arguments         []byte
	Reason            string
	RequestedBy       string
	SourceType        string
	DiscoveryArtifact string
	PolicyPath        string
	ConfigPath        string
	Runtime           string
	RuntimeConfigPath string
	Workspace         string
}

type NewApprovalOptions struct {
	Decision        string
	ApprovedAt      time.Time
	ApprovedBy      string
	Reason          string
	PolicyPath      string
	ConfirmReadOnly bool
}

type ValidationOptions struct {
	Config        mcpconfig.Config
	Policy        mcpsmoke.CallPolicy
	RuntimeConfig *runtimeconfig.Config
}

func NewProposal(opts NewProposalOptions) (MCPToolCallProposal, error) {
	createdAt := opts.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	id := strings.TrimSpace(opts.ID)
	if id == "" {
		id = "mcp-call-" + createdAt.UTC().Format("20060102T150405.000000000Z")
	}
	requestedBy := strings.TrimSpace(opts.RequestedBy)
	if requestedBy == "" {
		requestedBy = "manual"
	}
	sourceType := strings.TrimSpace(opts.SourceType)
	if sourceType == "" {
		sourceType = SourceTypeManual
	}
	arguments, err := canonicalArguments(opts.Arguments)
	if err != nil {
		return MCPToolCallProposal{}, err
	}
	configPath := strings.TrimSpace(opts.ConfigPath)
	configSHA256, err := optionalFileSHA256(configPath)
	if err != nil {
		return MCPToolCallProposal{}, err
	}
	runtimeConfigPath := strings.TrimSpace(opts.RuntimeConfigPath)
	runtimeConfigSHA256, err := optionalFileSHA256(runtimeConfigPath)
	if err != nil {
		return MCPToolCallProposal{}, err
	}
	proposal := MCPToolCallProposal{
		ID:              id,
		CreatedAt:       createdAt.UTC(),
		Server:          strings.TrimSpace(opts.Server),
		Tool:            strings.TrimSpace(opts.Tool),
		Arguments:       arguments,
		ArgumentsSHA256: ArgumentsSHA256(arguments),
		Reason:          strings.TrimSpace(opts.Reason),
		RequestedBy:     requestedBy,
		Source: MCPToolCallSource{
			Type:              sourceType,
			DiscoveryArtifact: strings.TrimSpace(opts.DiscoveryArtifact),
		},
		PolicyPath:          strings.TrimSpace(opts.PolicyPath),
		ConfigPath:          configPath,
		ConfigSHA256:        configSHA256,
		Runtime:             normalizeRuntime(opts.Runtime),
		RuntimeConfigPath:   runtimeConfigPath,
		RuntimeConfigSHA256: runtimeConfigSHA256,
		Workspace:           strings.TrimSpace(opts.Workspace),
		Status:              ProposalStatusProposed,
	}
	if err := proposal.Validate(); err != nil {
		return MCPToolCallProposal{}, err
	}
	return proposal, nil
}

func (p MCPToolCallProposal) Validate() error {
	var errs []error
	requireNonEmpty := func(field string, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}
	requireNonEmpty("id", p.ID)
	requireNonEmpty("server", p.Server)
	requireNonEmpty("tool", p.Tool)
	requireNonEmpty("reason", p.Reason)
	requireNonEmpty("requested_by", p.RequestedBy)
	requireNonEmpty("policy_path", p.PolicyPath)
	requireNonEmpty("runtime", p.Runtime)
	if p.CreatedAt.IsZero() {
		errs = append(errs, errors.New("created_at is required"))
	}
	if p.Status != ProposalStatusProposed {
		errs = append(errs, fmt.Errorf("status must be %q", ProposalStatusProposed))
	}
	switch p.Source.Type {
	case SourceTypeManual:
		if p.Source.DiscoveryArtifact != "" {
			errs = append(errs, errors.New("source.discovery_artifact is only valid for discovery source"))
		}
	case SourceTypeDiscovery:
	default:
		errs = append(errs, fmt.Errorf("source.type %q is not supported", p.Source.Type))
	}
	switch p.Runtime {
	case mcpsmoke.RuntimeLocal, mcpsmoke.RuntimeDocker:
	default:
		errs = append(errs, fmt.Errorf("runtime %q is not supported", p.Runtime))
	}
	arguments, err := canonicalArguments(p.Arguments)
	if err != nil {
		errs = append(errs, err)
	} else if p.ArgumentsSHA256 != ArgumentsSHA256(arguments) {
		errs = append(errs, errors.New("arguments_sha256 does not match arguments"))
	}
	if strings.TrimSpace(p.ArgumentsSHA256) == "" {
		errs = append(errs, errors.New("arguments_sha256 is required"))
	}
	if strings.TrimSpace(p.ConfigSHA256) != "" && strings.TrimSpace(p.ConfigPath) == "" {
		errs = append(errs, errors.New("config_path is required when config_sha256 is set"))
	}
	if strings.TrimSpace(p.RuntimeConfigSHA256) != "" && strings.TrimSpace(p.RuntimeConfigPath) == "" {
		errs = append(errs, errors.New("runtime_config_path is required when runtime_config_sha256 is set"))
	}
	return errors.Join(errs...)
}

func (p MCPToolCallProposal) JSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func LoadProposal(path string) (MCPToolCallProposal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MCPToolCallProposal{}, fmt.Errorf("read MCP tool call proposal %q: %w", path, err)
	}
	return ParseProposalJSON(data)
}

func ParseProposalJSON(data []byte) (MCPToolCallProposal, error) {
	var proposal MCPToolCallProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		return MCPToolCallProposal{}, fmt.Errorf("parse MCP tool call proposal json: %w", err)
	}
	return proposal, nil
}

func LintProposal(proposal MCPToolCallProposal, opts ValidationOptions) LintResult {
	preflight := BuildPreflight(proposal, opts)
	result := LintResult{
		ProposalID: proposal.ID,
		Status:     PreflightStatusPassed,
		Warnings:   append([]string(nil), preflight.Warnings...),
	}
	for _, failure := range preflight.Failures {
		if strings.Contains(failure, "env ") || strings.Contains(failure, "runtime.docker.env") || strings.Contains(failure, "runtime config") {
			continue
		}
		result.Violations = append(result.Violations, failure)
	}
	if len(result.Violations) > 0 {
		result.Status = PreflightStatusFailed
	}
	return result
}

func BuildPreflight(proposal MCPToolCallProposal, opts ValidationOptions) MCPToolCallPreflight {
	preflight := MCPToolCallPreflight{
		ProposalID: proposal.ID,
		Status:     PreflightStatusPassed,
		Warnings:   []string{},
		Failures:   []string{},
	}
	preflight.PolicySHA256 = hashFileIfReadable(proposal.PolicyPath)
	preflight.ConfigSHA256 = hashFileIfReadable(proposal.ConfigPath)
	preflight.RuntimeConfigSHA256 = hashFileIfReadable(proposal.RuntimeConfigPath)

	if err := proposal.Validate(); err != nil {
		preflight.addFailure(err.Error())
	}
	if err := mcpsmoke.ValidateCallPolicy(opts.Policy); err != nil {
		preflight.addFailure(fmt.Sprintf("policy invalid: %v", err))
	} else {
		preflight.Checks.PolicyLoaded = true
	}
	if _, err := mcpconfig.Validate(opts.Config); err != nil {
		preflight.addFailure(fmt.Sprintf("mcp config invalid: %v", err))
	}
	server, ok := opts.Config.MCP.Servers[proposal.Server]
	if !ok {
		preflight.addFailure(fmt.Sprintf("server %q not found", proposal.Server))
		return preflight.finish()
	}

	policy := opts.Policy.MCPCallPolicy
	if hasString(policy.AllowedServers, proposal.Server) {
		preflight.Checks.ServerAllowlisted = true
	} else {
		preflight.addFailure(fmt.Sprintf("server %q is not allowlisted", proposal.Server))
	}
	if hasString(policy.AllowedTools, proposal.Tool) {
		preflight.Checks.ToolAllowlisted = true
	} else {
		preflight.addFailure(fmt.Sprintf("tool %q is not allowlisted", proposal.Tool))
	}

	preflight.Checks.NoWriteExec = true
	preflight.Checks.ReadOnlyCapability = true
	if server.Enabled {
		preflight.addFailure("server enabled=true is refused")
	}
	if server.Protocol != mcpconfig.ProtocolStdio {
		preflight.addFailure(fmt.Sprintf("server protocol must be %s", mcpconfig.ProtocolStdio))
	}
	if len(server.Capabilities) == 0 {
		preflight.Checks.ReadOnlyCapability = false
		preflight.addFailure("server requires read capability")
	}
	for _, capability := range server.Capabilities {
		switch capability {
		case mcpconfig.CapabilityRead:
			if !hasString(policy.AllowedCapabilities, capability) {
				preflight.Checks.ReadOnlyCapability = false
				preflight.addFailure(fmt.Sprintf("capability %q is not allowlisted", capability))
			}
		case mcpconfig.CapabilityWrite, mcpconfig.CapabilityExec:
			preflight.Checks.NoWriteExec = false
			preflight.Checks.ReadOnlyCapability = false
			preflight.addFailure("server has write or exec capability")
		default:
			preflight.Checks.ReadOnlyCapability = false
			preflight.addFailure(fmt.Sprintf("capability %q is not supported", capability))
		}
	}

	preflight.Checks.DockerRequired = true
	if !server.TestOnly && policy.RequireDockerForReal && proposal.Runtime != mcpsmoke.RuntimeDocker {
		preflight.Checks.DockerRequired = false
		preflight.addFailure("real read-only server requires runtime docker")
	}
	if proposal.Runtime == mcpsmoke.RuntimeDocker && opts.RuntimeConfig == nil {
		preflight.Checks.DockerRequired = false
		preflight.addFailure("runtime docker requires runtime config")
	}

	preflight.Checks.ArgumentsWithinLimit = true
	arguments, err := canonicalArguments(proposal.Arguments)
	if err != nil {
		preflight.Checks.ArgumentsWithinLimit = false
		preflight.addFailure(err.Error())
	} else if len(arguments) > policy.MaxArgumentsBytes {
		preflight.Checks.ArgumentsWithinLimit = false
		preflight.addFailure(fmt.Sprintf("arguments too large: %d bytes exceeds %d", len(arguments), policy.MaxArgumentsBytes))
	}

	if envMissing := missingServerEnv(server); len(envMissing) > 0 {
		preflight.addFailure("env passthrough missing: " + strings.Join(envMissing, ","))
	} else {
		preflight.Checks.EnvAvailable = true
	}
	if opts.RuntimeConfig != nil {
		if envMissing := missingRuntimeEnv(*opts.RuntimeConfig); len(envMissing) > 0 {
			preflight.Checks.EnvAvailable = false
			preflight.addFailure("runtime env passthrough missing: " + strings.Join(envMissing, ","))
		}
	}

	return preflight.finish()
}

func BuildApproval(proposal MCPToolCallProposal, opts NewApprovalOptions) (MCPToolCallApproval, error) {
	if !opts.ConfirmReadOnly {
		return MCPToolCallApproval{}, errors.New("confirm_read_only is required")
	}
	if err := proposal.Validate(); err != nil {
		return MCPToolCallApproval{}, err
	}
	policySHA256, err := PolicySHA256FromFile(opts.PolicyPath)
	if err != nil {
		return MCPToolCallApproval{}, err
	}
	configSHA256 := proposal.ConfigSHA256
	if configSHA256 == "" && proposal.ConfigPath != "" {
		configSHA256, err = optionalFileSHA256(proposal.ConfigPath)
		if err != nil {
			return MCPToolCallApproval{}, err
		}
	}
	runtimeConfigSHA256 := proposal.RuntimeConfigSHA256
	if runtimeConfigSHA256 == "" && proposal.RuntimeConfigPath != "" {
		runtimeConfigSHA256, err = optionalFileSHA256(proposal.RuntimeConfigPath)
		if err != nil {
			return MCPToolCallApproval{}, err
		}
	}
	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}
	approvedBy := strings.TrimSpace(opts.ApprovedBy)
	if approvedBy == "" {
		approvedBy = "manual"
	}
	approval := MCPToolCallApproval{
		ProposalID:          proposal.ID,
		Decision:            strings.TrimSpace(opts.Decision),
		ApprovedAt:          approvedAt.UTC(),
		ApprovedBy:          approvedBy,
		Reason:              strings.TrimSpace(opts.Reason),
		ArgumentsSHA256:     proposal.ArgumentsSHA256,
		PolicySHA256:        policySHA256,
		ConfigSHA256:        configSHA256,
		RuntimeConfigSHA256: runtimeConfigSHA256,
		ConfirmReadOnly:     true,
	}
	if err := approval.Validate(); err != nil {
		return MCPToolCallApproval{}, err
	}
	return approval, nil
}

func (a MCPToolCallApproval) Validate() error {
	var errs []error
	requireNonEmpty := func(field string, value string) {
		if strings.TrimSpace(value) == "" {
			errs = append(errs, fmt.Errorf("%s is required", field))
		}
	}
	requireNonEmpty("proposal_id", a.ProposalID)
	requireNonEmpty("approved_by", a.ApprovedBy)
	requireNonEmpty("reason", a.Reason)
	requireNonEmpty("arguments_sha256", a.ArgumentsSHA256)
	requireNonEmpty("policy_sha256", a.PolicySHA256)
	if a.ApprovedAt.IsZero() {
		errs = append(errs, errors.New("approved_at is required"))
	}
	switch a.Decision {
	case ApprovalDecisionApproved, ApprovalDecisionRejected:
	default:
		errs = append(errs, fmt.Errorf("decision %q is not supported", a.Decision))
	}
	if !a.ConfirmReadOnly {
		errs = append(errs, errors.New("confirm_read_only must be true"))
	}
	return errors.Join(errs...)
}

func (a MCPToolCallApproval) JSON() ([]byte, error) {
	if err := a.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func LoadApproval(path string) (MCPToolCallApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MCPToolCallApproval{}, fmt.Errorf("read MCP tool call approval %q: %w", path, err)
	}
	return ParseApprovalJSON(data)
}

func ParseApprovalJSON(data []byte) (MCPToolCallApproval, error) {
	var approval MCPToolCallApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return MCPToolCallApproval{}, fmt.Errorf("parse MCP tool call approval json: %w", err)
	}
	return approval, nil
}

func (p MCPToolCallPreflight) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (p *MCPToolCallPreflight) addFailure(failure string) {
	p.Failures = append(p.Failures, failure)
}

func (p MCPToolCallPreflight) finish() MCPToolCallPreflight {
	if len(p.Failures) > 0 {
		p.Status = PreflightStatusFailed
	}
	return p
}

func ValidateExecution(proposal MCPToolCallProposal, approval MCPToolCallApproval, policyPath string, configPath string, runtimeConfigPath string, preflight MCPToolCallPreflight) error {
	if err := proposal.Validate(); err != nil {
		return err
	}
	if err := approval.Validate(); err != nil {
		return err
	}
	if approval.Decision != ApprovalDecisionApproved {
		return fmt.Errorf("approval decision must be %q, got %q", ApprovalDecisionApproved, approval.Decision)
	}
	if approval.ProposalID != proposal.ID {
		return fmt.Errorf("approval proposal_id %q does not match proposal id %q", approval.ProposalID, proposal.ID)
	}
	if approval.ArgumentsSHA256 != proposal.ArgumentsSHA256 {
		return errors.New("approval arguments_sha256 does not match proposal arguments_sha256")
	}
	policySHA256, err := PolicySHA256FromFile(policyPath)
	if err != nil {
		return err
	}
	if approval.PolicySHA256 != policySHA256 {
		return errors.New("approval policy_sha256 does not match current policy")
	}
	if approval.ConfigSHA256 != "" {
		if strings.TrimSpace(configPath) == "" {
			return errors.New("approval config_sha256 is present but no config path was provided")
		}
		configSHA256, err := FileSHA256(configPath)
		if err != nil {
			return err
		}
		if approval.ConfigSHA256 != configSHA256 {
			return errors.New("approval config_sha256 does not match current config")
		}
	}
	if approval.RuntimeConfigSHA256 != "" {
		if strings.TrimSpace(runtimeConfigPath) == "" {
			return errors.New("approval runtime_config_sha256 is present but no runtime config path was provided")
		}
		runtimeConfigSHA256, err := FileSHA256(runtimeConfigPath)
		if err != nil {
			return err
		}
		if approval.RuntimeConfigSHA256 != runtimeConfigSHA256 {
			return errors.New("approval runtime_config_sha256 does not match current runtime config")
		}
	}
	if !approval.ConfirmReadOnly {
		return errors.New("approval confirm_read_only must be true")
	}
	if preflight.Status != PreflightStatusPassed {
		return errors.New("proposal preflight failed")
	}
	return nil
}

func BuildExecutionBundle(proposal MCPToolCallProposal, approval MCPToolCallApproval, preflight MCPToolCallPreflight, result mcpsmoke.CallResult, policyPath string, configPath string, runtimeConfigPath string) (MCPToolCallExecutionBundle, error) {
	proposalSHA256, err := JSONSHA256(proposal)
	if err != nil {
		return MCPToolCallExecutionBundle{}, err
	}
	approvalSHA256, err := JSONSHA256(approval)
	if err != nil {
		return MCPToolCallExecutionBundle{}, err
	}
	policySHA256, err := FileSHA256(policyPath)
	if err != nil {
		return MCPToolCallExecutionBundle{}, err
	}
	configSHA256 := ""
	if strings.TrimSpace(configPath) != "" {
		configSHA256, err = FileSHA256(configPath)
		if err != nil {
			return MCPToolCallExecutionBundle{}, err
		}
	}
	runtimeConfigSHA256 := ""
	if strings.TrimSpace(runtimeConfigPath) != "" {
		runtimeConfigSHA256, err = FileSHA256(runtimeConfigPath)
		if err != nil {
			return MCPToolCallExecutionBundle{}, err
		}
	}
	bundle := MCPToolCallExecutionBundle{
		ProposalID:          proposal.ID,
		ApprovalSHA256:      approvalSHA256,
		ProposalSHA256:      proposalSHA256,
		PolicySHA256:        policySHA256,
		ConfigSHA256:        configSHA256,
		RuntimeConfigSHA256: runtimeConfigSHA256,
		PreflightStatus:     preflight.Status,
		Server:              proposal.Server,
		Tool:                proposal.Tool,
		ArgumentsSHA256:     proposal.ArgumentsSHA256,
		Runtime:             proposal.Runtime,
		Artifacts: map[string]string{
			"mcp-call-smoke-summary.md": result.SummaryPath,
			"mcp-call-transcript.jsonl": result.TranscriptPath,
			"mcp-call-result.json":      result.ResultPath,
			"mcp-call-stdout.log":       result.StdoutPath,
			"mcp-call-stderr.log":       result.StderrPath,
			"mcp-call-response.json":    result.ResponsePath,
		},
		Status:            result.Status,
		StartedAt:         result.StartedAt,
		FinishedAt:        result.FinishedAt,
		ResponseTruncated: result.ResponseTruncated,
		ToolCalls:         result.ToolCalls,
	}
	return bundle, nil
}

func (b MCPToolCallExecutionBundle) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func ArgumentsSHA256(raw json.RawMessage) string {
	canonical, err := canonicalArguments(raw)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

func PolicySHA256FromFile(path string) (string, error) {
	return FileSHA256(path)
}

func FileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file for sha256 %q: %w", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func JSONSHA256(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func optionalFileSHA256(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	sha256, err := FileSHA256(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return sha256, nil
}

func hashFileIfReadable(path string) string {
	sha256, err := optionalFileSHA256(path)
	if err != nil {
		return ""
	}
	return sha256
}

func canonicalArguments(raw []byte) (json.RawMessage, error) {
	raw = []byte(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, errors.New("arguments are required")
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("arguments must be a valid JSON object: %w", err)
	}
	if object == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(canonical), nil
}

func normalizeRuntime(runtime string) string {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return mcpsmoke.RuntimeDocker
	}
	return runtime
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func missingServerEnv(server mcpconfig.ServerConfig) []string {
	var missing []string
	for _, name := range server.Env.Passthrough {
		if !envIsSet(name) {
			missing = append(missing, name)
		}
	}
	return missing
}

func missingRuntimeEnv(cfg runtimeconfig.Config) []string {
	var missing []string
	for _, name := range cfg.Runtime.Docker.Env.Passthrough {
		if !envIsSet(name) {
			missing = append(missing, name)
		}
	}
	return missing
}

func envIsSet(name string) bool {
	value, ok := os.LookupEnv(name)
	return ok && value != ""
}
