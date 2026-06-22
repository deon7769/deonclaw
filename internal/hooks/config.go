package hooks

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	EventRunCompleted            = "run.completed"
	EventRunFailed               = "run.failed"
	EventScheduleDue             = "schedule.due"
	EventDaemonStarted           = "daemon.started"
	EventLearningProposalCreated = "learning.proposal.created"
)

type Hook struct {
	ID      string `yaml:"id" json:"id"`
	Event   string `yaml:"event" json:"event"`
	Action  string `yaml:"action" json:"action"`
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Policy  string `yaml:"policy,omitempty" json:"policy,omitempty"`
}

type Config struct {
	Hooks []Hook `yaml:"hooks" json:"hooks"`
}

type Plan struct {
	HookID   string `json:"hook_id"`
	Event    string `json:"event"`
	Action   string `json:"action"`
	Enabled  bool   `json:"enabled"`
	Policy   string `json:"policy,omitempty"`
	WouldRun bool   `json:"would_run"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read hooks config %q: %w", path, err)
	}
	return ParseConfig(data)
}

func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse hooks yaml: %w", err)
	}
	normalizeConfig(&cfg)
	return cfg, nil
}

func normalizeConfig(cfg *Config) {
	for i := range cfg.Hooks {
		cfg.Hooks[i].ID = strings.TrimSpace(cfg.Hooks[i].ID)
		cfg.Hooks[i].Event = strings.TrimSpace(cfg.Hooks[i].Event)
		cfg.Hooks[i].Action = strings.TrimSpace(cfg.Hooks[i].Action)
		cfg.Hooks[i].Policy = strings.TrimSpace(cfg.Hooks[i].Policy)
		if cfg.Hooks[i].Policy == "" {
			cfg.Hooks[i].Policy = "internal-only"
		}
	}
}

func ValidateConfig(cfg Config) error {
	var errs []error
	ids := map[string]struct{}{}
	for _, hook := range cfg.Hooks {
		if hook.ID == "" {
			errs = append(errs, errors.New("hook id is required"))
			continue
		}
		if _, ok := ids[hook.ID]; ok {
			errs = append(errs, fmt.Errorf("duplicate hook id %q", hook.ID))
		}
		ids[hook.ID] = struct{}{}
		if hook.Event == "" {
			errs = append(errs, fmt.Errorf("hook %q event is required", hook.ID))
		}
		if hook.Action == "" {
			errs = append(errs, fmt.Errorf("hook %q action is required", hook.ID))
		}
		if strings.Contains(hook.Action, "shell") {
			errs = append(errs, fmt.Errorf("hook %q shell actions are disabled in this sprint", hook.ID))
		}
	}
	return errors.Join(errs...)
}

func PlanForEvent(cfg Config, event string) []Plan {
	event = strings.TrimSpace(event)
	plans := make([]Plan, 0)
	for _, hook := range cfg.Hooks {
		if hook.Event != event {
			continue
		}
		plans = append(plans, Plan{
			HookID:   hook.ID,
			Event:    hook.Event,
			Action:   hook.Action,
			Enabled:  hook.Enabled,
			Policy:   hook.Policy,
			WouldRun: hook.Enabled && hook.Policy == "internal-only",
		})
	}
	return plans
}
