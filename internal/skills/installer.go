package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type InstallOptions struct {
	SourcePath   string
	RegistryRoot string
	AsName       string
	Policy       Policy
	Now          time.Time
}

type InstallResult struct {
	SkillName  string        `json:"skill_name"`
	RevisionID string        `json:"revision_id"`
	State      string        `json:"state"`
	Provenance Provenance    `json:"provenance"`
	Inspect    InspectReport `json:"inspect"`
}

type VerifyResult struct {
	SkillName string     `json:"skill_name"`
	Revision  string     `json:"revision"`
	Scan      ScanReport `json:"scan"`
	Status    string     `json:"status"`
}

func BuildImportPlan(sourceRef string, policy Policy) (ImportPlan, error) {
	inspect, err := InspectSourceRef(sourceRef)
	if err != nil {
		return ImportPlan{}, err
	}
	plan := ImportPlan{
		SourceType: inspect.SourceType,
		SourceRef:  sourceRef,
		SkillName:  inspect.Skill.Name,
		Inspect:    inspect,
	}
	if !policy.SourceAllowed(inspect.SourceType) {
		plan.BlockedReason = fmt.Sprintf("source type %q is denied by policy", inspect.SourceType)
		return plan, nil
	}
	if inspect.SourceType == SourceTypeLocal && !inspect.ReadyToStage {
		plan.BlockedReason = "inspect scan failed"
		return plan, nil
	}
	if inspect.SourceType == "git" {
		plan.BlockedReason = "git import requires local clone before install in this sprint"
		return plan, nil
	}
	if policy.IsProtected(plan.SkillName) {
		plan.BlockedReason = fmt.Sprintf("skill %q is protected by policy", plan.SkillName)
		return plan, nil
	}
	plan.WouldInstall = true
	return plan, nil
}

func InstallLocal(opts InstallOptions) (InstallResult, Registry, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	inspect, err := InspectLocal(opts.SourcePath)
	if err != nil {
		return InstallResult{}, Registry{}, err
	}
	if !opts.Policy.SourceAllowed(inspect.SourceType) {
		return InstallResult{}, Registry{}, fmt.Errorf("source type %q is denied by policy", inspect.SourceType)
	}
	if !inspect.ReadyToStage {
		return InstallResult{}, Registry{}, fmt.Errorf("skill scan failed")
	}

	skillName := strings.TrimSpace(opts.AsName)
	if skillName == "" {
		skillName = inspect.Skill.Name
	}
	if err := ValidateSkillName(skillName); err != nil {
		return InstallResult{}, Registry{}, err
	}
	if opts.Policy.IsProtected(skillName) {
		return InstallResult{}, Registry{}, fmt.Errorf("skill %q is protected by policy", skillName)
	}

	registryRoot := strings.TrimSpace(opts.RegistryRoot)
	if registryRoot == "" {
		return InstallResult{}, Registry{}, fmt.Errorf("registry root is required")
	}
	registry, err := LoadRegistry(registryRoot)
	if err != nil {
		return InstallResult{}, Registry{}, err
	}

	revisionID := NewRevisionID(now, inspect.ContentSHA256)
	destDir := RevisionDir(registryRoot, skillName, revisionID)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return InstallResult{}, Registry{}, err
	}
	if err := copySkillTree(opts.SourcePath, destDir); err != nil {
		return InstallResult{}, Registry{}, err
	}

	state := LifecycleVerified
	if opts.Policy.RequiresApproval(inspect.SourceType) {
		state = LifecyclePendingApproval
	}

	provenance := Provenance{
		SourceType:    inspect.SourceType,
		SourceRef:     inspect.SourceRef,
		ResolvedPath:  destDir,
		ContentSHA256: inspect.ContentSHA256,
		InstalledAt:   now.Format(time.RFC3339Nano),
		SkillName:     skillName,
		RevisionID:    revisionID,
	}

	registry.Skills[skillName] = RegistrySkill{
		Name:            skillName,
		CurrentRevision: revisionID,
		State:           state,
		ContentSHA256:   inspect.ContentSHA256,
		InstalledAt:     provenance.InstalledAt,
	}
	if err := updateCurrentLink(registryRoot, skillName, revisionID); err != nil {
		return InstallResult{}, Registry{}, err
	}
	if err := SaveRegistry(registry); err != nil {
		return InstallResult{}, Registry{}, err
	}

	return InstallResult{
		SkillName:  skillName,
		RevisionID: revisionID,
		State:      state,
		Provenance: provenance,
		Inspect:    inspect,
	}, registry, nil
}

func updateCurrentLink(registryRoot string, skillName string, revisionID string) error {
	linkPath := CurrentLinkPath(registryRoot, skillName)
	_ = os.Remove(linkPath)
	target := filepath.Join("revisions", revisionID)
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, linkPath)
}

func VerifySkill(registryRoot string, skillName string) (VerifyResult, error) {
	registry, err := LoadRegistry(registryRoot)
	if err != nil {
		return VerifyResult{}, err
	}
	entry, ok := registry.Skills[skillName]
	if !ok {
		return VerifyResult{}, fmt.Errorf("skill %q not found in registry", skillName)
	}
	revisionDir := RevisionDir(registryRoot, skillName, entry.CurrentRevision)
	scan, _, err := ScanSkillDirectory(revisionDir)
	if err != nil {
		return VerifyResult{}, err
	}
	return VerifyResult{
		SkillName: skillName,
		Revision:  entry.CurrentRevision,
		Scan:      scan,
		Status:    scan.Status,
	}, nil
}

func SetSkillState(registry Registry, skillName string, state string) (Registry, error) {
	entry, ok := registry.Skills[skillName]
	if !ok {
		return Registry{}, fmt.Errorf("skill %q not found in registry", skillName)
	}
	state = normalizeLifecycleState(state)
	if !allowedRegistryState(state) {
		return Registry{}, fmt.Errorf("state %q is not allowed", state)
	}
	entry.State = state
	registry.Skills[skillName] = entry
	return registry, nil
}

func EnableSkillForAgent(registry Registry, agentID string, skillName string) (Registry, error) {
	agentID = strings.TrimSpace(agentID)
	skillName = strings.TrimSpace(skillName)
	if agentID == "" {
		return Registry{}, fmt.Errorf("agent id is required")
	}
	if _, ok := registry.Skills[skillName]; !ok {
		return Registry{}, fmt.Errorf("skill %q not found in registry", skillName)
	}
	registry, err := SetSkillState(registry, skillName, LifecycleActive)
	if err != nil {
		return Registry{}, err
	}
	allowlist := registry.AgentAllowlists[agentID]
	for _, existing := range allowlist {
		if existing == skillName {
			return registry, nil
		}
	}
	registry.AgentAllowlists[agentID] = append(allowlist, skillName)
	return registry, nil
}

func DisableSkillForAgent(registry Registry, agentID string, skillName string) (Registry, error) {
	agentID = strings.TrimSpace(agentID)
	skillName = strings.TrimSpace(skillName)
	allowlist := registry.AgentAllowlists[agentID]
	filtered := make([]string, 0, len(allowlist))
	for _, existing := range allowlist {
		if existing != skillName {
			filtered = append(filtered, existing)
		}
	}
	registry.AgentAllowlists[agentID] = filtered
	return registry, nil
}
