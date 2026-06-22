package schedule

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read schedules config %q: %w", path, err)
	}
	return ParseConfig(data)
}

func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse schedules yaml: %w", err)
	}
	normalizeConfig(&cfg)
	return cfg, nil
}

func normalizeConfig(cfg *Config) {
	for i := range cfg.Schedules {
		s := &cfg.Schedules[i]
		s.ID = strings.TrimSpace(s.ID)
		s.Kind = strings.TrimSpace(strings.ToLower(s.Kind))
		s.Name = strings.TrimSpace(s.Name)
		s.Status = strings.TrimSpace(strings.ToLower(s.Status))
		if s.Status == "" {
			s.Status = StatusActive
		}
		s.AgentID = strings.TrimSpace(s.AgentID)
		s.Timezone = strings.TrimSpace(s.Timezone)
		if s.Timezone == "" {
			s.Timezone = "UTC"
		}
		s.ConcurrencyPolicy = strings.TrimSpace(s.ConcurrencyPolicy)
		if s.ConcurrencyPolicy == "" {
			s.ConcurrencyPolicy = ConcurrencySkipIfRunning
		}
		s.CatchUpPolicy = strings.TrimSpace(s.CatchUpPolicy)
		if s.CatchUpPolicy == "" {
			s.CatchUpPolicy = CatchUpNextOnly
		}
	}
}

func ValidateConfig(cfg Config) error {
	var errs []error
	ids := map[string]struct{}{}
	for _, s := range cfg.Schedules {
		if s.ID == "" {
			errs = append(errs, errors.New("schedule id is required"))
			continue
		}
		if _, ok := ids[s.ID]; ok {
			errs = append(errs, fmt.Errorf("duplicate schedule id %q", s.ID))
		}
		ids[s.ID] = struct{}{}
		if s.Name == "" {
			errs = append(errs, fmt.Errorf("schedule %q name is required", s.ID))
		}
		if s.AgentID == "" {
			errs = append(errs, fmt.Errorf("schedule %q agent_id is required", s.ID))
		}
		if err := validateKindFields(s); err != nil {
			errs = append(errs, fmt.Errorf("schedule %q: %w", s.ID, err))
		}
		if err := validateConcurrencyPolicy(s.ConcurrencyPolicy); err != nil {
			errs = append(errs, fmt.Errorf("schedule %q: %w", s.ID, err))
		}
	}
	return errors.Join(errs...)
}

func validateKindFields(s Schedule) error {
	switch s.Kind {
	case KindAt:
		if strings.TrimSpace(s.At) == "" {
			return errors.New("at is required for kind at")
		}
		if _, err := time.Parse(time.RFC3339, s.At); err != nil {
			return fmt.Errorf("at must be RFC3339: %w", err)
		}
	case KindEvery:
		if strings.TrimSpace(s.Every) == "" {
			return errors.New("every is required for kind every")
		}
		if _, err := ParseEveryDuration(s.Every); err != nil {
			return err
		}
	case KindCron:
		if strings.TrimSpace(s.Cron) == "" {
			return errors.New("cron is required for kind cron")
		}
		if _, err := ParseCronExpression(s.Cron, s.Timezone); err != nil {
			return err
		}
	case KindHeartbeat:
		if strings.TrimSpace(s.HeartbeatPolicyID) == "" {
			return errors.New("heartbeat_policy_id is required for kind heartbeat")
		}
	case KindWebhook, KindHook:
		return nil
	default:
		return fmt.Errorf("kind %q is not supported", s.Kind)
	}
	return nil
}

func validateConcurrencyPolicy(policy string) error {
	switch policy {
	case ConcurrencyAllowParallel, ConcurrencySkipIfRunning, ConcurrencyQueueAfterRunning, ConcurrencyReplacePending, ConcurrencyCoalesce:
		return nil
	default:
		return fmt.Errorf("concurrency_policy %q is not supported", policy)
	}
}

func SyncFromConfig(cfg Config, now time.Time) []Schedule {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.Format(time.RFC3339Nano)
	out := make([]Schedule, 0, len(cfg.Schedules))
	for _, s := range cfg.Schedules {
		if strings.TrimSpace(s.CreatedAt) == "" {
			s.CreatedAt = stamp
		}
		s.UpdatedAt = stamp
		if s.NextDueAt == "" {
			if next, err := NextDue(s, now); err == nil {
				s.NextDueAt = next.Format(time.RFC3339Nano)
			}
		}
		out = append(out, s)
	}
	return out
}
