package skills

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	SkillPolicy Policy `yaml:"skill_policy" json:"skill_policy"`
}

type Policy struct {
	DefaultVisibility      string              `yaml:"default_visibility" json:"default_visibility"`
	DefaultUpdateMode      string              `yaml:"default_update_mode" json:"default_update_mode"`
	AllowSources           []string            `yaml:"allow_sources" json:"allow_sources"`
	DenySources            []string            `yaml:"deny_sources" json:"deny_sources"`
	RequireApprovalFor     []string            `yaml:"require_approval_for" json:"require_approval_for"`
	States                 []string            `yaml:"states" json:"states"`
	ProtectedSkills        []string            `yaml:"protected_skills" json:"protected_skills"`
	RenewalTriggers        []string            `yaml:"renewal_triggers" json:"renewal_triggers"`
	ProposalRequiredFields []string            `yaml:"proposal_required_fields" json:"proposal_required_fields"`
	Permissions            map[string]string   `yaml:"permissions" json:"permissions"`
	AgentAllowlists        map[string][]string `yaml:"agent_allowlists" json:"agent_allowlists"`
}

func LoadPolicy(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read skill policy %q: %w", path, err)
	}
	return ParsePolicy(data)
}

func ParsePolicy(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse skill policy yaml: %w", err)
	}
	normalizePolicy(&cfg)
	return cfg, nil
}

func normalizePolicy(cfg *Config) {
	cfg.SkillPolicy.DefaultVisibility = strings.TrimSpace(strings.ToLower(cfg.SkillPolicy.DefaultVisibility))
	if cfg.SkillPolicy.DefaultVisibility == "" {
		cfg.SkillPolicy.DefaultVisibility = "deny"
	}
	cfg.SkillPolicy.DefaultUpdateMode = strings.TrimSpace(strings.ToLower(cfg.SkillPolicy.DefaultUpdateMode))
	if cfg.SkillPolicy.DefaultUpdateMode == "" {
		cfg.SkillPolicy.DefaultUpdateMode = "proposal_only"
	}
	for i, source := range cfg.SkillPolicy.AllowSources {
		cfg.SkillPolicy.AllowSources[i] = strings.TrimSpace(strings.ToLower(source))
	}
	for i, source := range cfg.SkillPolicy.DenySources {
		cfg.SkillPolicy.DenySources[i] = strings.TrimSpace(strings.ToLower(source))
	}
	for i, item := range cfg.SkillPolicy.RequireApprovalFor {
		cfg.SkillPolicy.RequireApprovalFor[i] = strings.TrimSpace(strings.ToLower(item))
	}
	for agent, skills := range cfg.SkillPolicy.AgentAllowlists {
		delete(cfg.SkillPolicy.AgentAllowlists, agent)
		trimmedAgent := strings.TrimSpace(agent)
		normalized := make([]string, 0, len(skills))
		for _, skill := range skills {
			normalized = append(normalized, strings.TrimSpace(skill))
		}
		cfg.SkillPolicy.AgentAllowlists[trimmedAgent] = normalized
	}
}

func ValidatePolicy(cfg Config) error {
	var errs []error
	if cfg.SkillPolicy.DefaultVisibility != "deny" && cfg.SkillPolicy.DefaultVisibility != "allow" {
		errs = append(errs, fmt.Errorf("default_visibility %q is not supported", cfg.SkillPolicy.DefaultVisibility))
	}
	if cfg.SkillPolicy.DefaultUpdateMode != "proposal_only" && cfg.SkillPolicy.DefaultUpdateMode != "manual" {
		errs = append(errs, fmt.Errorf("default_update_mode %q is not supported", cfg.SkillPolicy.DefaultUpdateMode))
	}
	for _, state := range cfg.SkillPolicy.States {
		switch strings.TrimSpace(strings.ToLower(state)) {
		case PolicyStateDraft, PolicyStateActive, PolicyStateDeprecated, PolicyStateArchived:
		default:
			errs = append(errs, fmt.Errorf("state %q is not supported", state))
		}
	}
	return errors.Join(errs...)
}

func (p Policy) SourceAllowed(sourceType string) bool {
	sourceType = strings.TrimSpace(strings.ToLower(sourceType))
	for _, denied := range p.DenySources {
		if denied == sourceType {
			return false
		}
	}
	if len(p.AllowSources) == 0 {
		return sourceType == SourceTypeLocal || sourceType == "managed" || sourceType == "workspace"
	}
	for _, allowed := range p.AllowSources {
		if allowed == sourceType {
			return true
		}
	}
	return false
}

func (p Policy) RequiresApproval(sourceType string) bool {
	sourceType = strings.TrimSpace(strings.ToLower(sourceType))
	for _, item := range p.RequireApprovalFor {
		if item == sourceType {
			return true
		}
	}
	return false
}

func (p Policy) AgentMayUse(agentID string, skillName string) bool {
	agentID = strings.TrimSpace(agentID)
	skillName = strings.TrimSpace(skillName)
	skills, ok := p.AgentAllowlists[agentID]
	if !ok {
		return p.DefaultVisibility == "allow"
	}
	for _, allowed := range skills {
		if allowed == skillName {
			return true
		}
	}
	return false
}

func (p Policy) IsProtected(skillName string) bool {
	for _, protected := range p.ProtectedSkills {
		if strings.TrimSpace(protected) == strings.TrimSpace(skillName) {
			return true
		}
	}
	return false
}

func DefaultPermissions(policy Policy, sidecar *DeonClawSidecar) map[string]string {
	if sidecar != nil && len(sidecar.Permissions) > 0 {
		copy := make(map[string]string, len(sidecar.Permissions))
		for key, value := range sidecar.Permissions {
			copy[key] = value
		}
		return copy
	}
	if len(policy.Permissions) > 0 {
		copy := make(map[string]string, len(policy.Permissions))
		for key, value := range policy.Permissions {
			copy[key] = value
		}
		return copy
	}
	return map[string]string{
		"network": "deny",
		"shell":   "ask",
		"secrets": "deny",
	}
}
