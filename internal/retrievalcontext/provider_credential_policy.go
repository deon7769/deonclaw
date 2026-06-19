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

type ProviderCredentialPolicyConfig struct {
	ProviderCredentialPolicy ProviderCredentialPolicy `yaml:"provider_credential_policy" json:"provider_credential_policy"`
}

type ProviderCredentialPolicy struct {
	Enabled                bool     `yaml:"enabled" json:"enabled"`
	Provider               string   `yaml:"provider" json:"provider"`
	CredentialCheckEnabled bool     `yaml:"credential_check_enabled" json:"credential_check_enabled"`
	AllowSecretRead        bool     `yaml:"allow_secret_read" json:"allow_secret_read"`
	AllowNetwork           bool     `yaml:"allow_network" json:"allow_network"`
	BlockedReason          string   `yaml:"blocked_reason" json:"blocked_reason"`
	AllowedEnvVarNames     []string `yaml:"allowed_env_var_names" json:"allowed_env_var_names"`
}

type ProviderCredentialPolicyValidateResult struct {
	Status                    string   `json:"status"`
	CredentialPolicyValidated bool     `json:"credential_policy_validated"`
	CredentialCheckEnabled    bool     `json:"credential_check_enabled"`
	SecretValuesRead          bool     `json:"secret_values_read"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	BlockedReason             string   `json:"blocked_reason"`
	Provider                  string   `json:"provider"`
	AllowedEnvVarCount        int      `json:"allowed_env_var_count"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

type ProviderCredentialPolicyPlanOptions struct {
	ConfigPath string
	OutputPath string
}

type ProviderCredentialPolicyPlanResult struct {
	Status                    string   `json:"status"`
	CredentialPolicyPlanReady bool     `json:"credential_policy_plan_ready"`
	CredentialPolicyValidated bool     `json:"credential_policy_validated"`
	CredentialCheckEnabled    bool     `json:"credential_check_enabled"`
	SecretValuesRead          bool     `json:"secret_values_read"`
	ProviderCall              bool     `json:"provider_call"`
	NetworkCall               bool     `json:"network_call"`
	BlockedReason             string   `json:"blocked_reason"`
	Provider                  string   `json:"provider"`
	ConfigSHA256              string   `json:"config_sha256"`
	AllowedEnvVarNames        []string `json:"allowed_env_var_names"`
	Warnings                  []string `json:"warnings,omitempty"`
	Failures                  []string `json:"failures,omitempty"`
}

func LoadProviderCredentialPolicyConfig(path string) (ProviderCredentialPolicyConfig, error) {
	if err := validateRelativeSafePath("provider credential policy config path", path); err != nil {
		return ProviderCredentialPolicyConfig{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCredentialPolicyConfig{}, fmt.Errorf("read provider credential policy config %q: %w", path, err)
	}
	return ParseProviderCredentialPolicyConfig(data)
}

func ParseProviderCredentialPolicyConfig(data []byte) (ProviderCredentialPolicyConfig, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCredentialPolicyConfig{}, fmt.Errorf("provider credential policy config must not contain materialized preview text")
	}
	var cfg ProviderCredentialPolicyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return ProviderCredentialPolicyConfig{}, fmt.Errorf("parse provider credential policy yaml: %w", err)
	}
	cfg.ProviderCredentialPolicy.Provider = strings.TrimSpace(cfg.ProviderCredentialPolicy.Provider)
	cfg.ProviderCredentialPolicy.BlockedReason = strings.TrimSpace(cfg.ProviderCredentialPolicy.BlockedReason)
	return cfg, nil
}

func ValidateProviderCredentialPolicyConfig(cfg ProviderCredentialPolicyConfig) error {
	p := cfg.ProviderCredentialPolicy
	if p.Enabled {
		return fmt.Errorf("provider_credential_policy.enabled must be false")
	}
	if p.Provider != ProviderCallExecutorProvider {
		return fmt.Errorf("provider_credential_policy.provider %q must be %q", p.Provider, ProviderCallExecutorProvider)
	}
	if p.CredentialCheckEnabled {
		return fmt.Errorf("provider_credential_policy.credential_check_enabled must be false")
	}
	if p.AllowSecretRead {
		return fmt.Errorf("provider_credential_policy.allow_secret_read must be false")
	}
	if p.AllowNetwork {
		return fmt.Errorf("provider_credential_policy.allow_network must be false")
	}
	if p.BlockedReason != ProviderCallExecutorBlockedReason {
		return fmt.Errorf("provider_credential_policy.blocked_reason %q must be %q", p.BlockedReason, ProviderCallExecutorBlockedReason)
	}
	if len(p.AllowedEnvVarNames) == 0 {
		return fmt.Errorf("provider_credential_policy.allowed_env_var_names must not be empty")
	}
	for _, name := range p.AllowedEnvVarNames {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("provider_credential_policy.allowed_env_var_names must not contain empty names")
		}
	}
	return nil
}

func ProviderCredentialPolicyValidate(cfg ProviderCredentialPolicyConfig) (ProviderCredentialPolicyValidateResult, error) {
	p := cfg.ProviderCredentialPolicy
	result := ProviderCredentialPolicyValidateResult{
		Status:                 lancedbpolicy.StatusOK,
		CredentialCheckEnabled: p.CredentialCheckEnabled,
		SecretValuesRead:       false,
		ProviderCall:           false,
		NetworkCall:            false,
		BlockedReason:          p.BlockedReason,
		Provider:               p.Provider,
		AllowedEnvVarCount:     len(p.AllowedEnvVarNames),
	}
	if err := ValidateProviderCredentialPolicyConfig(cfg); err != nil {
		result.Status = lancedbpolicy.StatusFailed
		result.Failures = []string{err.Error()}
		return result, nil
	}
	result.CredentialPolicyValidated = true
	return result, nil
}

func ProviderCredentialPolicyPlan(opts ProviderCredentialPolicyPlanOptions) (ProviderCredentialPolicyPlanResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"config path", opts.ConfigPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCredentialPolicyPlanResult{}, err
		}
	}

	cfgData, err := readArtifactBytesNoTextExcerpt("provider credential policy config", opts.ConfigPath)
	if err != nil {
		return ProviderCredentialPolicyPlanResult{}, err
	}
	cfg, err := ParseProviderCredentialPolicyConfig(cfgData)
	if err != nil {
		return ProviderCredentialPolicyPlanResult{}, err
	}
	validateResult, err := ProviderCredentialPolicyValidate(cfg)
	if err != nil {
		return ProviderCredentialPolicyPlanResult{}, err
	}

	result := ProviderCredentialPolicyPlanResult{
		Status:                    lancedbpolicy.StatusOK,
		CredentialPolicyValidated: validateResult.CredentialPolicyValidated,
		CredentialCheckEnabled:    validateResult.CredentialCheckEnabled,
		SecretValuesRead:          false,
		ProviderCall:              false,
		NetworkCall:               false,
		BlockedReason:             validateResult.BlockedReason,
		Provider:                  validateResult.Provider,
		ConfigSHA256:              sha256Hex(cfgData),
		AllowedEnvVarNames:        append([]string(nil), cfg.ProviderCredentialPolicy.AllowedEnvVarNames...),
		Failures:                  append([]string(nil), validateResult.Failures...),
	}
	if validateResult.Status == lancedbpolicy.StatusFailed {
		result.Status = lancedbpolicy.StatusFailed
	} else {
		result.CredentialPolicyPlanReady = true
		if err := writeProviderCredentialPolicyPlanJSON(opts.OutputPath, result); err != nil {
			return ProviderCredentialPolicyPlanResult{}, err
		}
	}
	return result, nil
}

func LoadProviderCredentialPolicyPlan(path string) (ProviderCredentialPolicyPlanResult, []byte, error) {
	data, err := readArtifactBytesNoTextExcerpt("credential policy plan", path)
	if err != nil {
		return ProviderCredentialPolicyPlanResult{}, nil, err
	}
	var result ProviderCredentialPolicyPlanResult
	if err := json.Unmarshal(data, &result); err != nil {
		return ProviderCredentialPolicyPlanResult{}, nil, fmt.Errorf("parse credential policy plan json: %w", err)
	}
	return result, data, nil
}

func writeProviderCredentialPolicyPlanJSON(path string, result ProviderCredentialPolicyPlanResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create credential policy plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credential policy plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("credential policy plan must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderCredentialPolicyValidateText(result ProviderCredentialPolicyValidateResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_credential_policy_validate:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"credential_policy_validated", fmt.Sprintf("%t", result.CredentialPolicyValidated)},
		{"credential_check_enabled", fmt.Sprintf("%t", result.CredentialCheckEnabled)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"blocked_reason", result.BlockedReason},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteProviderCredentialPolicyValidateJSON(result ProviderCredentialPolicyValidateResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credential policy validate json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}

func WriteProviderCredentialPolicyPlanText(result ProviderCredentialPolicyPlanResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_credential_policy_plan:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"credential_policy_plan_ready", fmt.Sprintf("%t", result.CredentialPolicyPlanReady)},
		{"credential_policy_validated", fmt.Sprintf("%t", result.CredentialPolicyValidated)},
		{"secret_values_read", fmt.Sprintf("%t", result.SecretValuesRead)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"blocked_reason", result.BlockedReason},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	return nil
}

func WriteProviderCredentialPolicyPlanJSON(result ProviderCredentialPolicyPlanResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credential policy plan json: %w", err)
	}
	data = append(data, '\n')
	_, err = out.Write(data)
	return err
}
