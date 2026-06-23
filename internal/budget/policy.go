package budget

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BudgetPolicies map[string]Policy `yaml:"budget_policies"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read budgets config %q: %w", path, err)
	}
	return ParseConfig(data)
}

func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse budgets yaml: %w", err)
	}
	if cfg.BudgetPolicies == nil {
		cfg.BudgetPolicies = map[string]Policy{}
	}
	for id, policy := range cfg.BudgetPolicies {
		policy.ID = strings.TrimSpace(id)
		cfg.BudgetPolicies[id] = policy
	}
	return cfg, nil
}

func ValidateConfig(cfg Config) error {
	var errs []error
	for id, policy := range cfg.BudgetPolicies {
		if strings.TrimSpace(id) == "" {
			errs = append(errs, errors.New("budget policy id is required"))
			continue
		}
		if err := ValidatePolicy(policy); err != nil {
			errs = append(errs, fmt.Errorf("budget policy %q: %w", id, err))
		}
	}
	return errors.Join(errs...)
}

func ValidatePolicy(policy Policy) error {
	switch policy.Scope {
	case ScopeGlobal, ScopeAgent, ScopeModelProfile:
	default:
		return fmt.Errorf("scope %q is not supported", policy.Scope)
	}
	if policy.Scope == ScopeAgent && strings.TrimSpace(policy.AgentID) == "" {
		return errors.New("agent scope requires agent_id")
	}
	if policy.Scope == ScopeModelProfile && strings.TrimSpace(policy.ModelProfile) == "" {
		return errors.New("model_profile scope requires model_profile")
	}
	if policy.Period != PeriodMonthly {
		return fmt.Errorf("period %q is not supported", policy.Period)
	}
	if strings.TrimSpace(policy.Timezone) == "" {
		return errors.New("timezone is required")
	}
	if policy.HardLimitMicroUSD <= 0 {
		return errors.New("hard_limit_microusd must be positive")
	}
	if policy.DefaultEstimateMicroUSD <= 0 {
		return errors.New("default_estimate_microusd must be positive")
	}
	if policy.MaxSingleRunMicroUSD <= 0 {
		return errors.New("max_single_run_microusd must be positive")
	}
	if policy.DefaultEstimateMicroUSD > policy.MaxSingleRunMicroUSD {
		return errors.New("default_estimate_microusd cannot exceed max_single_run_microusd")
	}
	switch policy.OnExhausted {
	case OnExhaustedBlockWork, OnExhaustedPauseAgent:
	default:
		return fmt.Errorf("on_exhausted %q is not supported", policy.OnExhausted)
	}
	return nil
}

func ResolvePolicyForWork(cfg Config, policyID, agentID, modelProfile string) (Policy, error) {
	if strings.TrimSpace(policyID) != "" {
		policy, ok := cfg.BudgetPolicies[policyID]
		if !ok {
			return Policy{}, fmt.Errorf("budget policy %q not found", policyID)
		}
		return policy, nil
	}
	for _, policy := range cfg.BudgetPolicies {
		if policy.Scope == ScopeAgent && policy.AgentID == agentID {
			return policy, nil
		}
	}
	for _, policy := range cfg.BudgetPolicies {
		if policy.Scope == ScopeGlobal {
			return policy, nil
		}
	}
	return Policy{}, fmt.Errorf("no budget policy for agent %q", agentID)
}
