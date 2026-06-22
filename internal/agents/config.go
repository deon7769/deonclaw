package agents

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/workerconfig"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Agents AgentsSection `yaml:"agents" json:"agents"`
}

type AgentsSection struct {
	Defaults Defaults      `yaml:"defaults" json:"defaults"`
	List     []AgentConfig `yaml:"list" json:"list"`
}

type Defaults struct {
	WorkspaceStrategy string `yaml:"workspace_strategy" json:"workspace_strategy"`
	MemoryScope       string `yaml:"memory_scope" json:"memory_scope"`
	SkillPolicy       string `yaml:"skill_policy" json:"skill_policy"`
	HeartbeatPolicy   string `yaml:"heartbeat_policy" json:"heartbeat_policy"`
}

type AgentConfig struct {
	ID                string   `yaml:"id" json:"id"`
	DisplayName       string   `yaml:"display_name" json:"display_name"`
	Role              string   `yaml:"role" json:"role"`
	Description       string   `yaml:"description,omitempty" json:"description,omitempty"`
	SupervisorID      string   `yaml:"supervisor_id,omitempty" json:"supervisor_id,omitempty"`
	DefaultWorker     string   `yaml:"default_worker" json:"default_worker"`
	FallbackWorkers   []string `yaml:"fallback_workers,omitempty" json:"fallback_workers,omitempty"`
	ModelProfile      string   `yaml:"model_profile,omitempty" json:"model_profile,omitempty"`
	WorkspacePolicy   string   `yaml:"workspace_policy,omitempty" json:"workspace_policy,omitempty"`
	MemoryScope       string   `yaml:"memory_scope,omitempty" json:"memory_scope,omitempty"`
	SkillPolicy       string   `yaml:"skill_policy,omitempty" json:"skill_policy,omitempty"`
	BudgetPolicy      string   `yaml:"budget_policy,omitempty" json:"budget_policy,omitempty"`
	HeartbeatPolicy   string   `yaml:"heartbeat_policy,omitempty" json:"heartbeat_policy,omitempty"`
	Skills            []string `yaml:"skills,omitempty" json:"skills,omitempty"`
	MaxConcurrentRuns int      `yaml:"max_concurrent_runs,omitempty" json:"max_concurrent_runs,omitempty"`
}

type Agent struct {
	ID                string   `json:"id"`
	DisplayName       string   `json:"display_name"`
	Role              string   `json:"role"`
	Description       string   `json:"description,omitempty"`
	Status            string   `json:"status"`
	SupervisorID      string   `json:"supervisor_id,omitempty"`
	DefaultWorker     string   `json:"default_worker"`
	FallbackWorkers   []string `json:"fallback_workers,omitempty"`
	ModelProfile      string   `json:"model_profile,omitempty"`
	WorkspacePolicy   string   `json:"workspace_policy,omitempty"`
	MemoryScope       string   `json:"memory_scope,omitempty"`
	SkillPolicy       string   `json:"skill_policy,omitempty"`
	BudgetPolicy      string   `json:"budget_policy,omitempty"`
	HeartbeatPolicy   string   `json:"heartbeat_policy,omitempty"`
	Skills            []string `json:"skills,omitempty"`
	MaxConcurrentRuns int      `json:"max_concurrent_runs,omitempty"`
	CreatedAt         string   `json:"created_at"`
	UpdatedAt         string   `json:"updated_at"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read agents config %q: %w", path, err)
	}
	return ParseConfig(data)
}

func ParseConfig(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse agents config yaml: %w", err)
	}
	normalizeConfig(&cfg)
	return cfg, nil
}

func normalizeConfig(cfg *Config) {
	for i := range cfg.Agents.List {
		agent := &cfg.Agents.List[i]
		agent.ID = strings.TrimSpace(agent.ID)
		agent.DisplayName = strings.TrimSpace(agent.DisplayName)
		agent.Role = strings.TrimSpace(agent.Role)
		agent.SupervisorID = strings.TrimSpace(agent.SupervisorID)
		agent.DefaultWorker = strings.TrimSpace(agent.DefaultWorker)
		agent.ModelProfile = strings.TrimSpace(agent.ModelProfile)
		if agent.WorkspacePolicy == "" {
			agent.WorkspacePolicy = strings.TrimSpace(cfg.Agents.Defaults.WorkspaceStrategy)
		}
		if agent.MemoryScope == "" {
			agent.MemoryScope = strings.TrimSpace(cfg.Agents.Defaults.MemoryScope)
		}
		if agent.SkillPolicy == "" {
			agent.SkillPolicy = strings.TrimSpace(cfg.Agents.Defaults.SkillPolicy)
		}
		if agent.HeartbeatPolicy == "" {
			agent.HeartbeatPolicy = strings.TrimSpace(cfg.Agents.Defaults.HeartbeatPolicy)
		}
		if agent.MaxConcurrentRuns == 0 {
			agent.MaxConcurrentRuns = 1
		}
	}
}

func ValidateConfig(cfg Config, workers workerconfig.Config) error {
	var errs []error
	if len(cfg.Agents.List) == 0 {
		errs = append(errs, errors.New("agents.list is required"))
	}

	ids := map[string]struct{}{}
	for _, agent := range cfg.Agents.List {
		if agent.ID == "" {
			errs = append(errs, errors.New("agent id is required"))
			continue
		}
		if _, exists := ids[agent.ID]; exists {
			errs = append(errs, fmt.Errorf("duplicate agent id %q", agent.ID))
		}
		ids[agent.ID] = struct{}{}
		if agent.DisplayName == "" {
			errs = append(errs, fmt.Errorf("agent %q display_name is required", agent.ID))
		}
		if agent.Role == "" {
			errs = append(errs, fmt.Errorf("agent %q role is required", agent.ID))
		}
		if err := validateWorkerRef(agent.DefaultWorker, workers); err != nil {
			errs = append(errs, fmt.Errorf("agent %q: %w", agent.ID, err))
		}
		for _, fallback := range agent.FallbackWorkers {
			if err := validateWorkerRef(fallback, workers); err != nil {
				errs = append(errs, fmt.Errorf("agent %q fallback %q: %w", agent.ID, fallback, err))
			}
		}
		if agent.ModelProfile != "" {
			if _, ok := workers.ModelProfiles[agent.ModelProfile]; !ok {
				errs = append(errs, fmt.Errorf("agent %q model_profile %q is unknown", agent.ID, agent.ModelProfile))
			}
		}
	}

	for _, agent := range cfg.Agents.List {
		if agent.SupervisorID == "" {
			continue
		}
		if agent.SupervisorID == agent.ID {
			errs = append(errs, fmt.Errorf("agent %q cannot supervise itself", agent.ID))
		}
		if _, ok := ids[agent.SupervisorID]; !ok {
			errs = append(errs, fmt.Errorf("agent %q supervisor_id %q is unknown", agent.ID, agent.SupervisorID))
		}
	}

	return errors.Join(errs...)
}

func validateWorkerRef(worker string, workers workerconfig.Config) error {
	worker = strings.TrimSpace(worker)
	if worker == "" {
		return errors.New("worker is required")
	}
	if workers.Command(worker) == "" {
		return fmt.Errorf("worker %q is unknown", worker)
	}
	return nil
}

func AgentFromConfig(cfg AgentConfig, status string, createdAt string, updatedAt string) Agent {
	return Agent{
		ID:                cfg.ID,
		DisplayName:       cfg.DisplayName,
		Role:              cfg.Role,
		Description:       cfg.Description,
		Status:            status,
		SupervisorID:      cfg.SupervisorID,
		DefaultWorker:     cfg.DefaultWorker,
		FallbackWorkers:   append([]string(nil), cfg.FallbackWorkers...),
		ModelProfile:      cfg.ModelProfile,
		WorkspacePolicy:   cfg.WorkspacePolicy,
		MemoryScope:       cfg.MemoryScope,
		SkillPolicy:       cfg.SkillPolicy,
		BudgetPolicy:      cfg.BudgetPolicy,
		HeartbeatPolicy:   cfg.HeartbeatPolicy,
		Skills:            append([]string(nil), cfg.Skills...),
		MaxConcurrentRuns: cfg.MaxConcurrentRuns,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}
}
