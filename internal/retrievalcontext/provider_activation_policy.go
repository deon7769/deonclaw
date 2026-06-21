package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"gopkg.in/yaml.v3"
)

type ProviderActivationPolicyConfig struct {
	ProviderActivationPolicy ProviderActivationPolicy `yaml:"provider_activation_policy" json:"provider_activation_policy"`
}

type ProviderActivationPolicy struct {
	Enabled                          bool   `yaml:"enabled" json:"enabled"`
	Provider                         string `yaml:"provider" json:"provider"`
	RequireActivationReadinessAudit  bool   `yaml:"require_activation_readiness_audit" json:"require_activation_readiness_audit"`
	RequireRealCallProposal          bool   `yaml:"require_real_call_proposal" json:"require_real_call_proposal"`
	RequireCredentialPolicy          bool   `yaml:"require_credential_policy" json:"require_credential_policy"`
	RequireExecutionSimulationReport bool   `yaml:"require_execution_simulation_report" json:"require_execution_simulation_report"`
	RequireReleaseGate               bool   `yaml:"require_release_gate" json:"require_release_gate"`
	RequireManualOperatorApproval    bool   `yaml:"require_manual_operator_approval" json:"require_manual_operator_approval"`
	AllowProviderCall                bool   `yaml:"allow_provider_call" json:"allow_provider_call"`
	AllowNetwork                     bool   `yaml:"allow_network" json:"allow_network"`
	AllowSecretRead                  bool   `yaml:"allow_secret_read" json:"allow_secret_read"`
	AllowTransport                   bool   `yaml:"allow_transport" json:"allow_transport"`
	AllowWorkspaceWrite              bool   `yaml:"allow_workspace_write" json:"allow_workspace_write"`
	AllowDiffApply                   bool   `yaml:"allow_diff_apply" json:"allow_diff_apply"`
	AllowWorkerExecution             bool   `yaml:"allow_worker_execution" json:"allow_worker_execution"`
	AllowPromptInjection             bool   `yaml:"allow_prompt_injection" json:"allow_prompt_injection"`
	BlockedReason                    string `yaml:"blocked_reason" json:"blocked_reason"`
	MaxTotalChars                    int    `yaml:"max_total_chars" json:"max_total_chars"`
	MaxPayloadBytes                  int    `yaml:"max_payload_bytes" json:"max_payload_bytes"`
}

type ProviderActivationPolicyValidateResult struct {
	Status                    string   `json:"status"`
	ActivationPolicyValidated bool     `json:"activation_policy_validated"`
	ActivationAllowedNow      bool     `json:"activation_allowed_now"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	SecretValuesRead          bool     `json:"secret_values_read"`
	TransportCalled           bool     `json:"transport_called"`
	WorkspaceModified         bool     `json:"workspace_modified"`
	BlockedReason             string   `json:"blocked_reason"`
	Provider                  string   `json:"provider"`
	MaxTotalChars             int      `json:"max_total_chars"`
	MaxPayloadBytes           int      `json:"max_payload_bytes"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

type ProviderActivationPolicyPlanOptions struct {
	ConfigPath string
	OutputPath string
}

type ProviderActivationPolicyPlanResult struct {
	Status                    string   `json:"status"`
	ActivationPolicyPlanReady bool     `json:"activation_policy_plan_ready"`
	ActivationPolicyValidated bool     `json:"activation_policy_validated"`
	ActivationAllowedNow      bool     `json:"activation_allowed_now"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	SecretValuesRead          bool     `json:"secret_values_read"`
	TransportCalled           bool     `json:"transport_called"`
	WorkspaceModified         bool     `json:"workspace_modified"`
	BlockedReason             string   `json:"blocked_reason"`
	Provider                  string   `json:"provider"`
	ConfigSHA256              string   `json:"config_sha256"`
	MaxTotalChars             int      `json:"max_total_chars"`
	MaxPayloadBytes           int      `json:"max_payload_bytes"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func LoadProviderActivationPolicyConfig(path string) (ProviderActivationPolicyConfig, error) {
	if err := validateRelativeSafePath("provider activation policy config path", path); err != nil {
		return ProviderActivationPolicyConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderActivationPolicyConfig{}, fmt.Errorf("read provider activation policy config %q: %w", path, err)
	}
	return ParseProviderActivationPolicyConfig(data)
}

func ParseProviderActivationPolicyConfig(data []byte) (ProviderActivationPolicyConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderActivationPolicyConfig{}, fmt.Errorf("provider activation policy config must not contain materialized preview text")
	}
	var cfg ProviderActivationPolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProviderActivationPolicyConfig{}, fmt.Errorf("parse provider activation policy yaml: %w", err)
	}
	cfg.ProviderActivationPolicy.Provider = strings.TrimSpace(cfg.ProviderActivationPolicy.Provider)
	cfg.ProviderActivationPolicy.BlockedReason = strings.TrimSpace(cfg.ProviderActivationPolicy.BlockedReason)
	return cfg, nil
}

func ValidateProviderActivationPolicyConfig(cfg ProviderActivationPolicyConfig) error {
	p := cfg.ProviderActivationPolicy
	if p.Enabled {
		return fmt.Errorf("provider_activation_policy.enabled must be false")
	}
	if p.Provider != ProviderCallExecutorProvider {
		return fmt.Errorf("provider_activation_policy.provider %q must be %q", p.Provider, ProviderCallExecutorProvider)
	}
	for _, check := range []struct {
		name  string
		value bool
	}{
		{"require_activation_readiness_audit", p.RequireActivationReadinessAudit},
		{"require_real_call_proposal", p.RequireRealCallProposal},
		{"require_credential_policy", p.RequireCredentialPolicy},
		{"require_execution_simulation_report", p.RequireExecutionSimulationReport},
		{"require_release_gate", p.RequireReleaseGate},
		{"require_manual_operator_approval", p.RequireManualOperatorApproval},
	} {
		if !check.value {
			return fmt.Errorf("provider_activation_policy.%s must be true", check.name)
		}
	}
	for _, check := range []struct {
		name  string
		value bool
	}{
		{"allow_provider_call", p.AllowProviderCall},
		{"allow_network", p.AllowNetwork},
		{"allow_secret_read", p.AllowSecretRead},
		{"allow_transport", p.AllowTransport},
		{"allow_workspace_write", p.AllowWorkspaceWrite},
		{"allow_diff_apply", p.AllowDiffApply},
		{"allow_worker_execution", p.AllowWorkerExecution},
		{"allow_prompt_injection", p.AllowPromptInjection},
	} {
		if check.value {
			return fmt.Errorf("provider_activation_policy.%s must be false", check.name)
		}
	}
	if p.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("provider_activation_policy.blocked_reason %q must be %q", p.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	if p.MaxTotalChars <= 0 {
		return fmt.Errorf("provider_activation_policy.max_total_chars must be > 0")
	}
	if p.MaxPayloadBytes <= 0 {
		return fmt.Errorf("provider_activation_policy.max_payload_bytes must be > 0")
	}
	return nil
}

func ProviderActivationPolicyValidate(cfg ProviderActivationPolicyConfig) (ProviderActivationPolicyValidateResult, error) {
	p := cfg.ProviderActivationPolicy
	result := ProviderActivationPolicyValidateResult{
		Status:               lancedbpolicy.StatusOK,
		ActivationAllowedNow: false,
		ProviderCall:         false,
		NetworkCall:          false,
		SecretValuesRead:     false,
		TransportCalled:      false,
		WorkspaceModified:    false,
		BlockedReason:        p.BlockedReason,
		Provider:             p.Provider,
		MaxTotalChars:        p.MaxTotalChars,
		MaxPayloadBytes:      p.MaxPayloadBytes,
	}
	if err := ValidateProviderActivationPolicyConfig(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
		return result, nil
	}
	result.ActivationPolicyValidated = true
	return result, nil
}

func ProviderActivationPolicyPlan(opts ProviderActivationPolicyPlanOptions) (ProviderActivationPolicyPlanResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"config path", opts.ConfigPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderActivationPolicyPlanResult{}, err
		}
	}
	cfgData, err := readArtifactBytesNoTextExcerpt("provider activation policy config", opts.ConfigPath)
	if err != nil {
		return ProviderActivationPolicyPlanResult{}, err
	}
	cfg, err := ParseProviderActivationPolicyConfig(cfgData)
	if err != nil {
		return ProviderActivationPolicyPlanResult{}, err
	}
	validateResult, err := ProviderActivationPolicyValidate(cfg)
	if err != nil {
		return ProviderActivationPolicyPlanResult{}, err
	}
	result := ProviderActivationPolicyPlanResult{
		Status:                    lancedbpolicy.StatusOK,
		ActivationPolicyValidated: validateResult.ActivationPolicyValidated,
		ActivationAllowedNow:      false,
		ProviderCall:              false,
		NetworkCall:               false,
		SecretValuesRead:          false,
		TransportCalled:           false,
		WorkspaceModified:         false,
		BlockedReason:             validateResult.BlockedReason,
		Provider:                  validateResult.Provider,
		ConfigSHA256:              sha256Hex(cfgData),
		MaxTotalChars:             validateResult.MaxTotalChars,
		MaxPayloadBytes:           validateResult.MaxPayloadBytes,
		Failures:                  append([]string(nil), validateResult.Failures...),
	}
	if validateResult.Status == lancedbpolicy.StatusFailed {
		result.Status = lancedbpolicy.StatusFailed
		return result, nil
	}
	result.ActivationPolicyPlanReady = true
	if err := writeProviderActivationPolicyPlanJSON(opts.OutputPath, result); err != nil {
		return ProviderActivationPolicyPlanResult{}, err
	}
	return result, nil
}

func LoadProviderActivationPolicyPlan(path string) (ProviderActivationPolicyPlanResult, []byte, error) {
	if err := validateRelativeSafePath("provider activation policy plan path", path); err != nil {
		return ProviderActivationPolicyPlanResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("provider activation policy plan", path)
	if err != nil {
		return ProviderActivationPolicyPlanResult{}, nil, err
	}
	var result ProviderActivationPolicyPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderActivationPolicyPlanResult{}, nil, fmt.Errorf("parse provider activation policy plan json: %w", err)
	}
	return result, data, nil
}

func writeProviderActivationPolicyPlanJSON(path string, result ProviderActivationPolicyPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation policy plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation policy plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation policy plan must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider activation policy plan %q: %w", path, err)
	}
	return nil
}

func WriteProviderActivationPolicyValidateText(result ProviderActivationPolicyValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_policy_validate:"); err != nil {
		return err
	}
	return writeProviderActivationPolicyBlockedLines(result.ActivationPolicyValidated, result.ActivationAllowedNow, result.ProviderCall, result.NetworkCall, result.SecretValuesRead, result.TransportCalled, result.WorkspaceModified, result.BlockedReason, result.Status, out)
}

func WriteProviderActivationPolicyValidateJSON(result ProviderActivationPolicyValidateResult, out io.Writer) error {
	return writeProviderActivationPolicyValidateJSON(result, out)
}

func WriteProviderActivationPolicyPlanText(result ProviderActivationPolicyPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_policy_plan:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "activation_policy_plan_ready: %t\n", result.ActivationPolicyPlanReady); err != nil {
		return err
	}
	return writeProviderActivationPolicyBlockedLines(result.ActivationPolicyValidated, result.ActivationAllowedNow, result.ProviderCall, result.NetworkCall, result.SecretValuesRead, result.TransportCalled, result.WorkspaceModified, result.BlockedReason, result.Status, out)
}

func WriteProviderActivationPolicyPlanJSON(result ProviderActivationPolicyPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation policy plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation policy plan json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}

func writeProviderActivationPolicyValidateJSON(result ProviderActivationPolicyValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation policy validate json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation policy validate json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}

func writeProviderActivationPolicyBlockedLines(validated, allowedNow, providerCall, networkCall, secretRead, transportCalled, workspaceModified bool, blockedReason, status string, out io.Writer) error {
	lines := []struct {
		label string
		value string
	}{
		{"status", status},
		{"activation_policy_validated", fmt.Sprintf("%t", validated)},
		{"activation_allowed_now", fmt.Sprintf("%t", allowedNow)},
		{"provider_call", fmt.Sprintf("%t", providerCall)},
		{"network_call", fmt.Sprintf("%t", networkCall)},
		{"secret_values_read", fmt.Sprintf("%t", secretRead)},
		{"transport_called", fmt.Sprintf("%t", transportCalled)},
		{"workspace_modified", fmt.Sprintf("%t", workspaceModified)},
		{"blocked_reason", blockedReason},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	return nil
}
