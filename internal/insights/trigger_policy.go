package insights

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ReviewerCodex    = "codex"
	ReviewerOpenCode = "opencode"

	TriggerRunCompleted          = "run_completed"
	TriggerCommitCreated         = "commit_created"
	TriggerValidationFailed      = "validation_failed"
	TriggerUserCorrection        = "user_correction"
	TriggerApprovalDenied        = "approval_denied"
	TriggerRepeatedError         = "repeated_error"
	TriggerCostThresholdExceeded = "cost_threshold_exceeded"
	TriggerManualInsightRequest  = "manual_insight_request"
	TriggerHeartbeatSummary      = "heartbeat_summary"
)

var allowedTriggers = map[string]struct{}{
	TriggerRunCompleted:          {},
	TriggerCommitCreated:         {},
	TriggerValidationFailed:      {},
	TriggerUserCorrection:        {},
	TriggerApprovalDenied:        {},
	TriggerRepeatedError:         {},
	TriggerCostThresholdExceeded: {},
	TriggerManualInsightRequest:  {},
	TriggerHeartbeatSummary:      {},
}

var allowedReviewers = map[string]struct{}{
	ReviewerCodex:    {},
	ReviewerOpenCode: {},
}

type Config struct {
	InsightPolicy Policy `yaml:"insight_policy" json:"insight_policy"`
}

type Policy struct {
	Enabled              bool           `yaml:"enabled" json:"enabled"`
	EvaluateAfterRuns    int            `yaml:"evaluate_after_runs" json:"evaluate_after_runs"`
	EvaluateAfterCommits int            `yaml:"evaluate_after_commits" json:"evaluate_after_commits"`
	EvaluateAfterTurns   int            `yaml:"evaluate_after_turns" json:"evaluate_after_turns"`
	AlwaysOn             []string       `yaml:"always_on" json:"always_on"`
	Reviewer             ReviewerConfig `yaml:"reviewer" json:"reviewer"`
	AutoPropose          bool           `yaml:"auto_propose" json:"auto_propose"`
	AutoApply            bool           `yaml:"auto_apply" json:"auto_apply"`
}

type ReviewerConfig struct {
	Preferred string `yaml:"preferred" json:"preferred"`
	Fallback  string `yaml:"fallback" json:"fallback"`
}

func LoadPolicy(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read insight policy %q: %w", path, err)
	}
	return ParsePolicy(data)
}

func ParsePolicy(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse insight policy yaml: %w", err)
	}
	normalizePolicy(&cfg)
	return cfg, nil
}

func ValidatePolicy(cfg Config) error {
	return validatePolicy(cfg)
}

func normalizePolicy(cfg *Config) {
	p := &cfg.InsightPolicy
	p.Reviewer.Preferred = strings.TrimSpace(p.Reviewer.Preferred)
	p.Reviewer.Fallback = strings.TrimSpace(p.Reviewer.Fallback)
	p.AlwaysOn = trimNonEmpty(p.AlwaysOn)
}

func trimNonEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func validatePolicy(cfg Config) error {
	var errs []error
	p := cfg.InsightPolicy

	if p.Enabled {
		if p.EvaluateAfterRuns < 0 {
			errs = append(errs, errors.New("insight_policy.evaluate_after_runs must be >= 0"))
		}
		if p.EvaluateAfterCommits < 0 {
			errs = append(errs, errors.New("insight_policy.evaluate_after_commits must be >= 0"))
		}
		if p.EvaluateAfterTurns < 0 {
			errs = append(errs, errors.New("insight_policy.evaluate_after_turns must be >= 0"))
		}
		if p.EvaluateAfterRuns == 0 && p.EvaluateAfterCommits == 0 && p.EvaluateAfterTurns == 0 && len(p.AlwaysOn) == 0 {
			errs = append(errs, errors.New("insight_policy must define evaluate_after_* thresholds or always_on triggers when enabled"))
		}
	}

	for i, trigger := range p.AlwaysOn {
		if err := validateTriggerType(fmt.Sprintf("insight_policy.always_on[%d]", i), trigger); err != nil {
			errs = append(errs, err)
		}
	}

	if p.Reviewer.Preferred != "" {
		if _, ok := allowedReviewers[p.Reviewer.Preferred]; !ok {
			errs = append(errs, fmt.Errorf("insight_policy.reviewer.preferred %q is not allowed", p.Reviewer.Preferred))
		}
	}
	if p.Reviewer.Fallback != "" {
		if _, ok := allowedReviewers[p.Reviewer.Fallback]; !ok {
			errs = append(errs, fmt.Errorf("insight_policy.reviewer.fallback %q is not allowed", p.Reviewer.Fallback))
		}
	}
	if p.Enabled && p.Reviewer.Preferred == "" {
		errs = append(errs, errors.New("insight_policy.reviewer.preferred is required when insight_policy.enabled is true"))
	}

	if p.AutoApply && !p.AutoPropose {
		errs = append(errs, errors.New("insight_policy.auto_apply requires auto_propose=true"))
	}
	if err := ValidateProposalPolicy(p); err != nil {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

func validateTriggerType(field string, trigger string) error {
	trigger = strings.TrimSpace(trigger)
	if trigger == "" {
		return fmt.Errorf("%s is required", field)
	}
	if _, ok := allowedTriggers[trigger]; !ok {
		return fmt.Errorf("%s %q is not an allowed trigger", field, trigger)
	}
	return nil
}

func IsAlwaysOnTrigger(policy Policy, trigger string) bool {
	trigger = strings.TrimSpace(trigger)
	for _, candidate := range policy.AlwaysOn {
		if candidate == trigger {
			return true
		}
	}
	return false
}
