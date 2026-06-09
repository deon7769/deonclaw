package workerconfig

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/deon7769/deonclaw/internal/tasks"
	"gopkg.in/yaml.v3"
)

const (
	EnvRequirementRequired = "required"

	EnvStateSetMasked = "set_masked"
	EnvStateMissing   = "missing"
)

type Config struct {
	Workers       map[string]Worker       `yaml:"workers" json:"workers"`
	ModelProfiles map[string]ModelProfile `yaml:"model_profiles,omitempty" json:"model_profiles,omitempty"`
}

type Worker struct {
	Command  string            `yaml:"command" json:"command"`
	Provider string            `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model    string            `yaml:"model,omitempty" json:"model,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

type ModelProfile struct {
	Worker   string            `yaml:"worker" json:"worker"`
	Provider string            `yaml:"provider,omitempty" json:"provider,omitempty"`
	Model    string            `yaml:"model,omitempty" json:"model,omitempty"`
	ModelArg string            `yaml:"model_arg,omitempty" json:"model_arg,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
	Tags     []string          `yaml:"tags,omitempty" json:"tags,omitempty"`
}

type ResolvedModelProfile struct {
	Name    string       `json:"name"`
	Profile ModelProfile `json:"profile"`
}

type ResolvedModelStrategy struct {
	Preferred      []ResolvedModelProfile `json:"preferred"`
	Fallback       []ResolvedModelProfile `json:"fallback,omitempty"`
	RequireTags    []string               `json:"require_tags,omitempty"`
	FallbackPolicy tasks.FallbackPolicy   `json:"fallback_policy"`
}

func (s ResolvedModelStrategy) PlannedModelProfile() (ResolvedModelProfile, bool) {
	if len(s.Preferred) == 0 {
		return ResolvedModelProfile{}, false
	}
	return s.Preferred[0], true
}

type EnvRequirementCheck struct {
	Name        string `json:"name"`
	Requirement string `json:"requirement"`
	State       string `json:"state"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read workers config %q: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse workers config yaml: %w", err)
	}
	normalize(&cfg)
	return cfg, nil
}

func Default() Config {
	return Config{
		Workers: map[string]Worker{
			"codex":    {Command: "codex"},
			"opencode": {Command: "opencode"},
		},
	}
}

func KnownWorkers() []string {
	return []string{"codex", "opencode"}
}

func (c Config) Command(worker string) string {
	worker = strings.TrimSpace(worker)
	if worker == "" {
		return ""
	}
	if configured, ok := c.Workers[worker]; ok && strings.TrimSpace(configured.Command) != "" {
		return configured.Command
	}
	return Default().Workers[worker].Command
}

func (c Config) Worker(worker string) Worker {
	worker = strings.TrimSpace(worker)
	if worker == "" {
		return Worker{}
	}
	configured, ok := c.Workers[worker]
	if !ok {
		configured = Default().Workers[worker]
	}
	if strings.TrimSpace(configured.Command) == "" {
		configured.Command = Default().Workers[worker].Command
	}
	return configured
}

func (c Config) EnvRequirementChecks(worker string) []EnvRequirementCheck {
	return c.Worker(worker).EnvRequirementChecks()
}

func (c Config) EnvRequirementChecksFor(worker string, profileName string) ([]EnvRequirementCheck, error) {
	merged := map[string]string{}
	for name, requirement := range c.Worker(worker).Env {
		merged[name] = requirement
	}

	profileName = strings.TrimSpace(profileName)
	if profileName != "" {
		profile, err := c.ResolveModelProfile(worker, profileName)
		if err != nil {
			return nil, err
		}
		for name, requirement := range profile.Env {
			merged[name] = requirement
		}
	}

	return envRequirementChecks(merged), nil
}

func (c Config) FallbackEnvRequirementChecksFor(worker string, strategy *tasks.ModelStrategy) ([]EnvRequirementCheck, error) {
	resolved, err := c.ResolveModelStrategy(worker, strategy)
	if err != nil {
		return nil, err
	}
	if len(resolved.Fallback) == 0 {
		return nil, nil
	}

	merged := map[string]string{}
	for name, requirement := range c.Worker(worker).Env {
		merged[name] = requirement
	}
	for _, profile := range resolved.Fallback {
		for name, requirement := range profile.Profile.Env {
			merged[name] = requirement
		}
	}
	return envRequirementChecks(merged), nil
}

func (c Config) MissingRequiredEnv(worker string) []EnvRequirementCheck {
	return MissingRequiredEnv(c.EnvRequirementChecks(worker))
}

func (c Config) MissingRequiredEnvFor(worker string, profileName string) ([]EnvRequirementCheck, error) {
	checks, err := c.EnvRequirementChecksFor(worker, profileName)
	if err != nil {
		return nil, err
	}
	return MissingRequiredEnv(checks), nil
}

func (c Config) ValidateRequiredEnv(worker string) error {
	missing := c.MissingRequiredEnv(worker)
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for _, check := range missing {
		names = append(names, check.Name)
	}
	return fmt.Errorf("worker %s missing required env: %s", worker, strings.Join(names, ", "))
}

func (c Config) ValidateRequiredEnvFor(worker string, profileName string) error {
	missing, err := c.MissingRequiredEnvFor(worker, profileName)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		return nil
	}
	names := make([]string, 0, len(missing))
	for _, check := range missing {
		names = append(names, check.Name)
	}
	return fmt.Errorf("worker %s missing required env: %s", worker, strings.Join(names, ", "))
}

func (c Config) ResolveModelProfile(worker string, profileName string) (ModelProfile, error) {
	worker = strings.TrimSpace(worker)
	profileName = strings.TrimSpace(profileName)
	if profileName == "" {
		return ModelProfile{}, nil
	}
	profile, ok := c.ModelProfiles[profileName]
	if !ok {
		return ModelProfile{}, fmt.Errorf("model profile %q not found", profileName)
	}
	if profile.Worker == "" {
		return ModelProfile{}, fmt.Errorf("model profile %q worker is required", profileName)
	}
	if profile.Worker != worker {
		return ModelProfile{}, fmt.Errorf("model profile %q worker %q does not match task worker %q", profileName, profile.Worker, worker)
	}
	return profile, nil
}

func (c Config) ResolveModelStrategy(worker string, strategy *tasks.ModelStrategy) (ResolvedModelStrategy, error) {
	worker = strings.TrimSpace(worker)
	if strategy == nil {
		return ResolvedModelStrategy{}, nil
	}
	resolved := ResolvedModelStrategy{
		RequireTags:    normalizeTags(strategy.RequireTags),
		FallbackPolicy: tasks.EffectiveFallbackPolicy(strategy.FallbackPolicy),
	}
	if err := tasks.ValidateFallbackPolicy(strategy.FallbackPolicy); err != nil {
		return ResolvedModelStrategy{}, err
	}
	preferredNames := normalizeTags(strategy.Preferred)
	if len(preferredNames) == 0 {
		return ResolvedModelStrategy{}, errors.New("model_strategy.preferred must not be empty")
	}
	preferred, preferredErrs := c.resolveStrategyProfiles(worker, "preferred", preferredNames, resolved.RequireTags)
	fallback, fallbackErrs := c.resolveStrategyProfiles(worker, "fallback", strategy.Fallback, resolved.RequireTags)
	resolved.Preferred = preferred
	resolved.Fallback = fallback

	errs := append(preferredErrs, fallbackErrs...)
	if len(errs) > 0 {
		return ResolvedModelStrategy{}, errors.Join(errs...)
	}
	return resolved, nil
}

func (c Config) resolveStrategyProfiles(worker string, section string, names []string, requiredTags []string) ([]ResolvedModelProfile, []error) {
	names = normalizeTags(names)
	resolved := make([]ResolvedModelProfile, 0, len(names))
	var errs []error
	for i, name := range names {
		prefix := fmt.Sprintf("model_strategy.%s[%d] profile %q", section, i, name)
		profile, ok := c.ModelProfiles[name]
		if !ok {
			errs = append(errs, fmt.Errorf("%s not found", prefix))
			continue
		}
		if profile.Worker == "" {
			errs = append(errs, fmt.Errorf("%s worker is required", prefix))
			continue
		}
		if profile.Worker != worker {
			errs = append(errs, fmt.Errorf("%s worker %q does not match task worker %q; automatic worker switching is not implemented", prefix, profile.Worker, worker))
			continue
		}
		for _, tag := range requiredTags {
			if !hasTag(profile.Tags, tag) {
				errs = append(errs, fmt.Errorf("%s missing required tag %q", prefix, tag))
			}
		}
		resolved = append(resolved, ResolvedModelProfile{Name: name, Profile: profile})
	}
	return resolved, errs
}

func hasTag(tags []string, want string) bool {
	for _, tag := range tags {
		if tag == want {
			return true
		}
	}
	return false
}

func (w Worker) EnvRequirementChecks() []EnvRequirementCheck {
	return envRequirementChecks(w.Env)
}

func (p ModelProfile) EnvRequirementChecks() []EnvRequirementCheck {
	return envRequirementChecks(p.Env)
}

func envRequirementChecks(env map[string]string) []EnvRequirementCheck {
	if len(env) == 0 {
		return nil
	}
	names := make([]string, 0, len(env))
	for name := range env {
		names = append(names, name)
	}
	sort.Strings(names)

	checks := make([]EnvRequirementCheck, 0, len(names))
	for _, name := range names {
		requirement := normalizeEnvRequirement(env[name])
		state := EnvStateMissing
		if _, ok := os.LookupEnv(name); ok {
			state = EnvStateSetMasked
		}
		checks = append(checks, EnvRequirementCheck{
			Name:        name,
			Requirement: requirement,
			State:       state,
		})
	}
	return checks
}

func MissingRequiredEnv(checks []EnvRequirementCheck) []EnvRequirementCheck {
	missing := []EnvRequirementCheck{}
	for _, check := range checks {
		if check.Requirement == EnvRequirementRequired && check.State == EnvStateMissing {
			missing = append(missing, check)
		}
	}
	return missing
}

func normalize(cfg *Config) {
	if cfg.Workers == nil {
		cfg.Workers = map[string]Worker{}
	}
	for name, worker := range cfg.Workers {
		normalizedName := strings.TrimSpace(name)
		worker.Command = strings.TrimSpace(worker.Command)
		worker.Provider = strings.TrimSpace(worker.Provider)
		worker.Model = strings.TrimSpace(worker.Model)
		worker.Env = normalizeEnv(worker.Env)
		if normalizedName != name {
			delete(cfg.Workers, name)
		}
		cfg.Workers[normalizedName] = worker
	}
	if cfg.ModelProfiles != nil {
		for name, profile := range cfg.ModelProfiles {
			normalizedName := strings.TrimSpace(name)
			if normalizedName == "" {
				delete(cfg.ModelProfiles, name)
				continue
			}
			profile.Worker = strings.TrimSpace(profile.Worker)
			profile.Provider = strings.TrimSpace(profile.Provider)
			profile.Model = strings.TrimSpace(profile.Model)
			profile.ModelArg = strings.TrimSpace(profile.ModelArg)
			profile.Env = normalizeEnv(profile.Env)
			profile.Tags = normalizeTags(profile.Tags)
			if normalizedName != name {
				delete(cfg.ModelProfiles, name)
			}
			cfg.ModelProfiles[normalizedName] = profile
		}
		if len(cfg.ModelProfiles) == 0 {
			cfg.ModelProfiles = nil
		}
	}
}

func normalizeEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(env))
	for name, value := range env {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		normalized[name] = normalizeEnvRequirement(value)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeEnvRequirement(requirement string) string {
	return strings.ToLower(strings.TrimSpace(requirement))
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		normalized = append(normalized, tag)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
