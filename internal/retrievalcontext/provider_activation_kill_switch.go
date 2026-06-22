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

type ProviderActivationKillSwitchConfig struct {
	ProviderActivationKillSwitch ProviderActivationKillSwitch `yaml:"provider_activation_kill_switch" json:"provider_activation_kill_switch"`
}

type ProviderActivationKillSwitch struct {
	GlobalDisabled       bool   `yaml:"global_disabled" json:"global_disabled"`
	BlockProviderCall    bool   `yaml:"block_provider_call" json:"block_provider_call"`
	BlockNetwork         bool   `yaml:"block_network" json:"block_network"`
	BlockSecretRead      bool   `yaml:"block_secret_read" json:"block_secret_read"`
	BlockTransport       bool   `yaml:"block_transport" json:"block_transport"`
	BlockWorkspaceWrite  bool   `yaml:"block_workspace_write" json:"block_workspace_write"`
	BlockDiffApply       bool   `yaml:"block_diff_apply" json:"block_diff_apply"`
	BlockWorkerExecution bool   `yaml:"block_worker_execution" json:"block_worker_execution"`
	BlockPromptInjection bool   `yaml:"block_prompt_injection" json:"block_prompt_injection"`
	BlockedReason        string `yaml:"blocked_reason" json:"blocked_reason"`
}

type ProviderActivationKillSwitchValidateResult struct {
	Status                string   `json:"status"`
	KillSwitchValidated   bool     `json:"kill_switch_validated"`
	GlobalDisabled        bool     `json:"global_disabled"`
	ProviderCallBlocked   bool     `json:"provider_call_blocked"`
	NetworkBlocked        bool     `json:"network_blocked"`
	SecretReadBlocked     bool     `json:"secret_read_blocked"`
	TransportBlocked      bool     `json:"transport_blocked"`
	WorkspaceWriteBlocked bool     `json:"workspace_write_blocked"`
	ActivationAllowedNow  bool     `json:"activation_allowed_now"`
	BlockedReason         string   `json:"blocked_reason"`
	Warnings              []string `json:"warnings,omitempty"`
	Failures              []string `json:"failures,omitempty"`
}

type ProviderActivationKillSwitchPlanOptions struct {
	ConfigPath string
	OutputPath string
}

type ProviderActivationKillSwitchPlanResult struct {
	Status                string   `json:"status"`
	KillSwitchPlanReady   bool     `json:"kill_switch_plan_ready"`
	KillSwitchValidated   bool     `json:"kill_switch_validated"`
	GlobalDisabled        bool     `json:"global_disabled"`
	ProviderCallBlocked   bool     `json:"provider_call_blocked"`
	NetworkBlocked        bool     `json:"network_blocked"`
	SecretReadBlocked     bool     `json:"secret_read_blocked"`
	TransportBlocked      bool     `json:"transport_blocked"`
	WorkspaceWriteBlocked bool     `json:"workspace_write_blocked"`
	ActivationAllowedNow  bool     `json:"activation_allowed_now"`
	BlockedReason         string   `json:"blocked_reason"`
	ConfigSHA256          string   `json:"config_sha256"`
	Warnings              []string `json:"warnings,omitempty"`
	Failures              []string `json:"failures,omitempty"`
}

func LoadProviderActivationKillSwitchConfig(path string) (ProviderActivationKillSwitchConfig, error) {
	if err := validateRelativeSafePath("provider activation kill switch config path", path); err != nil {
		return ProviderActivationKillSwitchConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderActivationKillSwitchConfig{}, fmt.Errorf("read provider activation kill switch config %q: %w", path, err)
	}
	return ParseProviderActivationKillSwitchConfig(data)
}

func ParseProviderActivationKillSwitchConfig(data []byte) (ProviderActivationKillSwitchConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderActivationKillSwitchConfig{}, fmt.Errorf("provider activation kill switch config must not contain materialized preview text")
	}
	var cfg ProviderActivationKillSwitchConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProviderActivationKillSwitchConfig{}, fmt.Errorf("parse provider activation kill switch yaml: %w", err)
	}
	cfg.ProviderActivationKillSwitch.BlockedReason = strings.TrimSpace(cfg.ProviderActivationKillSwitch.BlockedReason)
	return cfg, nil
}

func ValidateProviderActivationKillSwitchConfig(cfg ProviderActivationKillSwitchConfig) error {
	ks := cfg.ProviderActivationKillSwitch
	if !ks.GlobalDisabled {
		return fmt.Errorf("provider_activation_kill_switch.global_disabled must be true")
	}
	blockChecks := []struct {
		name  string
		value bool
	}{
		{"block_provider_call", ks.BlockProviderCall},
		{"block_network", ks.BlockNetwork},
		{"block_secret_read", ks.BlockSecretRead},
		{"block_transport", ks.BlockTransport},
		{"block_workspace_write", ks.BlockWorkspaceWrite},
		{"block_diff_apply", ks.BlockDiffApply},
		{"block_worker_execution", ks.BlockWorkerExecution},
		{"block_prompt_injection", ks.BlockPromptInjection},
	}
	for _, check := range blockChecks {
		if !check.value {
			return fmt.Errorf("provider_activation_kill_switch.%s must be true", check.name)
		}
	}
	if ks.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("provider_activation_kill_switch.blocked_reason %q must be %q", ks.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	return nil
}

func ProviderActivationKillSwitchValidate(cfg ProviderActivationKillSwitchConfig) (ProviderActivationKillSwitchValidateResult, error) {
	ks := cfg.ProviderActivationKillSwitch
	result := ProviderActivationKillSwitchValidateResult{
		Status:                lancedbpolicy.StatusOK,
		GlobalDisabled:        ks.GlobalDisabled,
		ProviderCallBlocked:   ks.BlockProviderCall,
		NetworkBlocked:        ks.BlockNetwork,
		SecretReadBlocked:     ks.BlockSecretRead,
		TransportBlocked:      ks.BlockTransport,
		WorkspaceWriteBlocked: ks.BlockWorkspaceWrite,
		ActivationAllowedNow:  false,
		BlockedReason:         ks.BlockedReason,
	}
	if err := ValidateProviderActivationKillSwitchConfig(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
		return result, nil
	}
	result.KillSwitchValidated = true
	return result, nil
}

func ProviderActivationKillSwitchPlan(opts ProviderActivationKillSwitchPlanOptions) (ProviderActivationKillSwitchPlanResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"config path", opts.ConfigPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderActivationKillSwitchPlanResult{}, err
		}
	}
	cfgData, err := readArtifactBytesNoTextExcerpt("provider activation kill switch config", opts.ConfigPath)
	if err != nil {
		return ProviderActivationKillSwitchPlanResult{}, err
	}
	cfg, err := ParseProviderActivationKillSwitchConfig(cfgData)
	if err != nil {
		return ProviderActivationKillSwitchPlanResult{}, err
	}
	validateResult, err := ProviderActivationKillSwitchValidate(cfg)
	if err != nil {
		return ProviderActivationKillSwitchPlanResult{}, err
	}
	result := ProviderActivationKillSwitchPlanResult{
		Status:                validateResult.Status,
		KillSwitchValidated:   validateResult.KillSwitchValidated,
		GlobalDisabled:        validateResult.GlobalDisabled,
		ProviderCallBlocked:   validateResult.ProviderCallBlocked,
		NetworkBlocked:        validateResult.NetworkBlocked,
		SecretReadBlocked:     validateResult.SecretReadBlocked,
		TransportBlocked:      validateResult.TransportBlocked,
		WorkspaceWriteBlocked: validateResult.WorkspaceWriteBlocked,
		ActivationAllowedNow:  false,
		BlockedReason:         validateResult.BlockedReason,
		ConfigSHA256:          sha256Hex(cfgData),
		Failures:              append([]string(nil), validateResult.Failures...),
	}
	if validateResult.Status == lancedbpolicy.StatusFailed {
		return result, nil
	}
	result.KillSwitchPlanReady = true
	if err := writeProviderActivationKillSwitchPlanJSON(opts.OutputPath, result); err != nil {
		return ProviderActivationKillSwitchPlanResult{}, err
	}
	return result, nil
}

func LoadProviderActivationKillSwitchPlan(path string) (ProviderActivationKillSwitchPlanResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("provider activation kill switch plan", path)
	if err != nil {
		return ProviderActivationKillSwitchPlanResult{}, nil, err
	}
	var result ProviderActivationKillSwitchPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderActivationKillSwitchPlanResult{}, nil, fmt.Errorf("parse provider activation kill switch plan json: %w", err)
	}
	return result, data, nil
}

func writeProviderActivationKillSwitchPlanJSON(path string, result ProviderActivationKillSwitchPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider activation kill switch plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider activation kill switch plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider activation kill switch plan must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderActivationKillSwitchValidateJSON(result ProviderActivationKillSwitchValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderActivationKillSwitchValidateText(result ProviderActivationKillSwitchValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_kill_switch_validate:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  kill_switch_validated: %t\n  global_disabled: %t\n  activation_allowed_now: %t\n  blocked_reason: %s\n",
		result.Status, result.KillSwitchValidated, result.GlobalDisabled, result.ActivationAllowedNow, result.BlockedReason)
	return err
}

func WriteProviderActivationKillSwitchPlanJSON(result ProviderActivationKillSwitchPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderActivationKillSwitchPlanText(result ProviderActivationKillSwitchPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_activation_kill_switch_plan:"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "  status: %s\n  kill_switch_plan_ready: %t\n  kill_switch_validated: %t\n  global_disabled: %t\n  activation_allowed_now: %t\n  blocked_reason: %s\n",
		result.Status, result.KillSwitchPlanReady, result.KillSwitchValidated, result.GlobalDisabled, result.ActivationAllowedNow, result.BlockedReason)
	return err
}
