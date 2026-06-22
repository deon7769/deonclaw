package heartbeat

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ActiveHours struct {
	Start    string `yaml:"start" json:"start"`
	End      string `yaml:"end" json:"end"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}

type Policy struct {
	Enabled         bool        `yaml:"enabled" json:"enabled"`
	Every           string      `yaml:"every" json:"every"`
	Target          string      `yaml:"target,omitempty" json:"target,omitempty"`
	ActiveHours     ActiveHours `yaml:"active_hours,omitempty" json:"active_hours,omitempty"`
	SkipWhenBusy    bool        `yaml:"skip_when_busy" json:"skip_when_busy"`
	IsolatedSession bool        `yaml:"isolated_session" json:"isolated_session"`
	LightContext    bool        `yaml:"light_context" json:"light_context"`
	TimeoutSeconds  int         `yaml:"timeout_seconds,omitempty" json:"timeout_seconds,omitempty"`
	NoOpToken       string      `yaml:"no_op_token" json:"no_op_token"`
	AckMaxChars     int         `yaml:"ack_max_chars,omitempty" json:"ack_max_chars,omitempty"`
	Prompt          string      `yaml:"prompt,omitempty" json:"prompt,omitempty"`
}

type Config struct {
	HeartbeatPolicies map[string]Policy `yaml:"heartbeat_policies" json:"heartbeat_policies"`
}

type DryRunPlan struct {
	AgentID       string `json:"agent_id"`
	PolicyID      string `json:"policy_id"`
	Enabled       bool   `json:"enabled"`
	SkipWhenBusy  bool   `json:"skip_when_busy"`
	NoOpToken     string `json:"no_op_token"`
	PromptPreview string `json:"prompt_preview"`
	WouldRun      bool   `json:"would_run"`
	BlockedReason string `json:"blocked_reason,omitempty"`
}

type Result struct {
	Notify    bool   `json:"notify"`
	Summary   string `json:"summary"`
	NoOp      bool   `json:"no_op"`
	NoOpToken string `json:"no_op_token,omitempty"`
}

type State struct {
	AgentID     string `json:"agent_id"`
	PolicyID    string `json:"policy_id"`
	LastDueAt   string `json:"last_due_at,omitempty"`
	LastRunAt   string `json:"last_run_at,omitempty"`
	LastStatus  string `json:"last_status,omitempty"`
	LastSummary string `json:"last_summary,omitempty"`
	UpdatedAt   string `json:"updated_at"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read heartbeat config %q: %w", path, err)
	}
	return ParseConfig(data)
}

func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse heartbeat yaml: %w", err)
	}
	normalizeConfig(&cfg)
	return cfg, nil
}

func normalizeConfig(cfg *Config) {
	if cfg.HeartbeatPolicies == nil {
		cfg.HeartbeatPolicies = map[string]Policy{}
	}
	normalized := make(map[string]Policy, len(cfg.HeartbeatPolicies))
	for id, policy := range cfg.HeartbeatPolicies {
		if strings.TrimSpace(policy.NoOpToken) == "" {
			policy.NoOpToken = "HEARTBEAT_OK"
		}
		if policy.AckMaxChars == 0 {
			policy.AckMaxChars = 32
		}
		if policy.TimeoutSeconds == 0 {
			policy.TimeoutSeconds = 120
		}
		normalized[strings.TrimSpace(id)] = policy
	}
	cfg.HeartbeatPolicies = normalized
}

func ValidateConfig(cfg Config) error {
	var errs []error
	if len(cfg.HeartbeatPolicies) == 0 {
		errs = append(errs, errors.New("heartbeat_policies is required"))
	}
	for id, policy := range cfg.HeartbeatPolicies {
		if strings.TrimSpace(id) == "" {
			errs = append(errs, errors.New("heartbeat policy id is required"))
		}
		if policy.Enabled && strings.TrimSpace(policy.Every) != "" {
			if policy.Every == "0" || policy.Every == "0m" {
				errs = append(errs, fmt.Errorf("heartbeat policy %q every must be greater than zero when enabled", id))
			}
		}
		if strings.TrimSpace(policy.NoOpToken) == "" {
			errs = append(errs, fmt.Errorf("heartbeat policy %q no_op_token is required", id))
		}
	}
	return errors.Join(errs...)
}

func PolicyByID(cfg Config, id string) (Policy, bool) {
	policy, ok := cfg.HeartbeatPolicies[strings.TrimSpace(id)]
	return policy, ok
}

func PlanDryRun(agentID string, policyID string, policy Policy, agentBusy bool) DryRunPlan {
	plan := DryRunPlan{
		AgentID:      agentID,
		PolicyID:     policyID,
		Enabled:      policy.Enabled,
		SkipWhenBusy: policy.SkipWhenBusy,
		NoOpToken:    policy.NoOpToken,
		WouldRun:     policy.Enabled,
	}
	if len(policy.Prompt) > 160 {
		plan.PromptPreview = strings.TrimSpace(policy.Prompt[:160]) + "..."
	} else {
		plan.PromptPreview = strings.TrimSpace(policy.Prompt)
	}
	if !policy.Enabled {
		plan.WouldRun = false
		plan.BlockedReason = "heartbeat policy disabled"
	}
	if policy.Enabled && policy.SkipWhenBusy && agentBusy {
		plan.WouldRun = false
		plan.BlockedReason = "agent is busy"
	}
	return plan
}

func EvaluateResult(policy Policy, text string) Result {
	text = strings.TrimSpace(text)
	token := strings.TrimSpace(policy.NoOpToken)
	ackMax := policy.AckMaxChars
	if ackMax <= 0 {
		ackMax = 32
	}
	noOp := false
	if text == "" {
		noOp = true
	} else if strings.EqualFold(text, token) {
		noOp = true
	} else if strings.HasPrefix(strings.ToUpper(text), strings.ToUpper(token)) && len(text) <= ackMax {
		noOp = true
	} else if strings.HasSuffix(strings.ToUpper(text), strings.ToUpper(token)) && len(text) <= ackMax {
		noOp = true
	}
	return Result{
		Notify:    !noOp,
		Summary:   text,
		NoOp:      noOp,
		NoOpToken: token,
	}
}
