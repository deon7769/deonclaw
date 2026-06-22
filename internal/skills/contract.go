package skills

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	SourceTypeLocal = "local"

	LifecycleDiscovered       = "discovered"
	LifecycleStaged           = "staged"
	LifecycleVerified         = "verified"
	LifecyclePendingApproval  = "pending_approval"
	LifecycleActive           = "active"
	LifecycleDeprecated       = "deprecated"
	LifecycleArchived         = "archived"
	LifecycleRejected         = "rejected"
	LifecycleQuarantined      = "quarantined"
	PolicyStateDraft          = "draft"
	PolicyStateActive         = "active"
	PolicyStateDeprecated     = "deprecated"
	PolicyStateArchived       = "archived"
	RegistryVersion           = 1
	AgentsSkillsDir           = ".agents/skills"
	MaterializedSKILLFilename = "SKILL.md"
)

var skillNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)

type SkillFrontmatter struct {
	Name          string `yaml:"name" json:"name"`
	Description   string `yaml:"description" json:"description"`
	License       string `yaml:"license,omitempty" json:"license,omitempty"`
	Compatibility string `yaml:"compatibility,omitempty" json:"compatibility,omitempty"`
}

type ParsedSkill struct {
	Name          string           `json:"name"`
	Description   string           `json:"description"`
	License       string           `json:"license,omitempty"`
	Compatibility string           `json:"compatibility,omitempty"`
	Body          string           `json:"body"`
	SourcePath    string           `json:"source_path"`
	SKILLPath     string           `json:"skill_md_path"`
	Files         []string         `json:"files"`
	Scripts       []string         `json:"scripts"`
	Sidecar       *DeonClawSidecar `json:"sidecar,omitempty"`
}

type DeonClawSidecar struct {
	Source struct {
		Type           string `yaml:"type" json:"type"`
		Repository     string `yaml:"repository,omitempty" json:"repository,omitempty"`
		Ref            string `yaml:"ref,omitempty" json:"ref,omitempty"`
		ResolvedCommit string `yaml:"resolved_commit,omitempty" json:"resolved_commit,omitempty"`
		SHA256         string `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	} `yaml:"source" json:"source"`
	Trust struct {
		Level      string `yaml:"level" json:"level"`
		ReviewedBy string `yaml:"reviewed_by,omitempty" json:"reviewed_by,omitempty"`
		ReviewedAt string `yaml:"reviewed_at,omitempty" json:"reviewed_at,omitempty"`
	} `yaml:"trust" json:"trust"`
	Permissions map[string]string `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Lifecycle   struct {
		State      string `yaml:"state" json:"state"`
		Channel    string `yaml:"channel,omitempty" json:"channel,omitempty"`
		AutoUpdate bool   `yaml:"auto_update" json:"auto_update"`
	} `yaml:"lifecycle" json:"lifecycle"`
	CompatibleWorkers []string `yaml:"compatible_workers,omitempty" json:"compatible_workers,omitempty"`
}

type ScanFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
}

type ScanReport struct {
	Status   string        `json:"status"`
	Findings []ScanFinding `json:"findings"`
}

type InspectReport struct {
	SourceType    string      `json:"source_type"`
	SourceRef     string      `json:"source_ref"`
	Skill         ParsedSkill `json:"skill"`
	Scan          ScanReport  `json:"scan"`
	ContentSHA256 string      `json:"content_sha256"`
	ReadyToStage  bool        `json:"ready_to_stage"`
}

type Provenance struct {
	SourceType    string `json:"source_type"`
	SourceRef     string `json:"source_ref"`
	ResolvedPath  string `json:"resolved_path,omitempty"`
	ContentSHA256 string `json:"content_sha256"`
	InstalledAt   string `json:"installed_at"`
	SkillName     string `json:"skill_name"`
	RevisionID    string `json:"revision_id"`
}

type SkillRevision struct {
	RevisionID    string     `json:"revision_id"`
	SkillName     string     `json:"skill_name"`
	State         string     `json:"state"`
	Provenance    Provenance `json:"provenance"`
	ContentSHA256 string     `json:"content_sha256"`
	Materialized  bool       `json:"materialized"`
}

type RegistrySkill struct {
	Name            string `json:"name"`
	CurrentRevision string `json:"current_revision"`
	State           string `json:"state"`
	ContentSHA256   string `json:"content_sha256"`
	InstalledAt     string `json:"installed_at"`
}

type Registry struct {
	Version         int                      `json:"version"`
	RegistryRoot    string                   `json:"registry_root"`
	Skills          map[string]RegistrySkill `json:"skills"`
	AgentAllowlists map[string][]string      `json:"agent_allowlists"`
	SHA256          string                   `json:"sha256"`
}

type SnapshotSkill struct {
	Name             string            `json:"name"`
	Revision         string            `json:"revision"`
	SHA256           string            `json:"sha256"`
	Source           string            `json:"source"`
	MaterializedPath string            `json:"materialized_path"`
	Permissions      map[string]string `json:"permissions,omitempty"`
}

type SessionSnapshot struct {
	SessionID string          `json:"session_id"`
	AgentID   string          `json:"agent_id"`
	CreatedAt string          `json:"created_at"`
	Skills    []SnapshotSkill `json:"skills"`
	SHA256    string          `json:"sha256"`
}

type ImportPlan struct {
	SourceType    string        `json:"source_type"`
	SourceRef     string        `json:"source_ref"`
	SkillName     string        `json:"skill_name"`
	Inspect       InspectReport `json:"inspect"`
	WouldInstall  bool          `json:"would_install"`
	BlockedReason string        `json:"blocked_reason,omitempty"`
}

func ValidateSkillName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("skill name is required")
	}
	if len(name) > 64 {
		return errors.New("skill name exceeds 64 characters")
	}
	if !skillNamePattern.MatchString(name) {
		return fmt.Errorf("skill name %q is invalid; use lowercase kebab-case", name)
	}
	return nil
}

func normalizeLifecycleState(state string) string {
	return strings.TrimSpace(strings.ToLower(state))
}

func allowedRegistryState(state string) bool {
	switch normalizeLifecycleState(state) {
	case LifecycleVerified, LifecycleActive, LifecycleDeprecated, LifecycleArchived, LifecycleQuarantined:
		return true
	default:
		return false
	}
}
