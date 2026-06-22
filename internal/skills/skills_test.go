package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const exampleSkillMarkdown = `---
name: test-skill
description: Example skill for unit tests
license: MIT
---

## When to use

Use during tests.
`

func writeExampleSkill(t *testing.T, root string, extraFiles map[string]string) string {
	t.Helper()
	skillDir := filepath.Join(root, "test-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(exampleSkillMarkdown), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	for rel, content := range extraFiles {
		path := filepath.Join(skillDir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}
	return skillDir
}

func TestParseValidSKILLMarkdown(t *testing.T) {
	skill, err := ParseSKILLContent(exampleSkillMarkdown, "SKILL.md")
	if err != nil {
		t.Fatalf("ParseSKILLContent() error = %v", err)
	}
	if skill.Name != "test-skill" {
		t.Fatalf("name = %q", skill.Name)
	}
}

func TestParseSKILLRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := ParseSKILLFile(filepath.Join(dir, "SKILL.md"))
	if err == nil {
		t.Fatal("ParseSKILLFile() expected error for missing SKILL.md")
	}
}

func TestValidateSkillNameRejectsInvalid(t *testing.T) {
	if err := ValidateSkillName("Bad_Name"); err == nil {
		t.Fatal("ValidateSkillName() expected error for invalid name")
	}
}

func TestInspectLocalSkill(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeExampleSkill(t, dir, nil)
	report, err := InspectLocal(skillDir)
	if err != nil {
		t.Fatalf("InspectLocal() error = %v", err)
	}
	if !report.ReadyToStage {
		t.Fatalf("ready_to_stage = false, scan=%s", report.Scan.Status)
	}
	if report.Skill.Name != "test-skill" {
		t.Fatalf("skill name = %q", report.Skill.Name)
	}
}

func TestScanRejectsPathTraversalName(t *testing.T) {
	dir := t.TempDir()
	bad := `---
name: ../escape
description: bad
---
body
`
	skillDir := filepath.Join(dir, "bad")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(bad), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	scan, _, err := ScanSkillDirectory(skillDir)
	if err != nil {
		t.Fatalf("ScanSkillDirectory() error = %v", err)
	}
	if scan.Status != ScanStatusFailed {
		t.Fatalf("scan status = %q, want failed", scan.Status)
	}
}

func TestScanRejectsSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeExampleSkill(t, dir, nil)
	outside := filepath.Join(dir, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(skillDir, "escape-link")); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	scan, _, err := ScanSkillDirectory(skillDir)
	if err != nil {
		t.Fatalf("ScanSkillDirectory() error = %v", err)
	}
	if scan.Status != ScanStatusFailed {
		t.Fatalf("scan status = %q, want failed", scan.Status)
	}
}

func TestInstallLocalSkillWithProvenance(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeExampleSkill(t, dir, map[string]string{
		"scripts/check.sh": "#!/bin/sh\necho ok\n",
	})
	registryRoot := filepath.Join(dir, "registry")
	policy := Policy{
		DefaultVisibility: "deny",
		AllowSources:      []string{"local", "managed"},
	}
	result, _, err := InstallLocal(InstallOptions{
		SourcePath:   skillDir,
		RegistryRoot: registryRoot,
		Policy:       policy,
	})
	if err != nil {
		t.Fatalf("InstallLocal() error = %v", err)
	}
	if result.SkillName != "test-skill" {
		t.Fatalf("skill_name = %q", result.SkillName)
	}
	if _, err := os.Stat(filepath.Join(registryRoot, "registry.json")); err != nil {
		t.Fatalf("registry.json missing: %v", err)
	}
}

func TestSnapshotRespectsAgentAllowlist(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeExampleSkill(t, dir, nil)
	registryRoot := filepath.Join(dir, "registry")
	policy := Policy{
		DefaultVisibility: "deny",
		AllowSources:      []string{"local"},
		AgentAllowlists: map[string][]string{
			"legacy-manual": {"test-skill"},
		},
	}
	if _, registry, err := InstallLocal(InstallOptions{
		SourcePath:   skillDir,
		RegistryRoot: registryRoot,
		Policy:       policy,
	}); err != nil {
		t.Fatalf("InstallLocal() error = %v", err)
	} else {
		registry, err = EnableSkillForAgent(registry, "legacy-manual", "test-skill")
		if err != nil {
			t.Fatalf("EnableSkillForAgent() error = %v", err)
		}
		if err := SaveRegistry(registry); err != nil {
			t.Fatalf("SaveRegistry() error = %v", err)
		}
	}

	snapshot, err := BuildSnapshot(SnapshotOptions{
		AgentID:      "legacy-manual",
		SessionID:    "ses_test",
		RegistryRoot: registryRoot,
		Policy:       policy,
	})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if len(snapshot.Skills) != 1 {
		t.Fatalf("snapshot skill count = %d, want 1", len(snapshot.Skills))
	}
	if snapshot.Skills[0].Permissions["network"] != "deny" {
		t.Fatalf("permissions = %#v", snapshot.Skills[0].Permissions)
	}

	snapshot2, err := BuildSnapshot(SnapshotOptions{
		AgentID:      "other-agent",
		SessionID:    "ses_other",
		RegistryRoot: registryRoot,
		Policy:       policy,
	})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if len(snapshot2.Skills) != 0 {
		t.Fatalf("denied agent snapshot count = %d, want 0", len(snapshot2.Skills))
	}
}

func TestMaterializeCreatesAgentsSkillsPath(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeExampleSkill(t, dir, nil)
	registryRoot := filepath.Join(dir, "registry")
	workspace := filepath.Join(dir, "workspace")
	policy := Policy{
		DefaultVisibility: "deny",
		AllowSources:      []string{"local"},
		AgentAllowlists: map[string][]string{
			"legacy-manual": {"test-skill"},
		},
	}
	_, registry, err := InstallLocal(InstallOptions{
		SourcePath:   skillDir,
		RegistryRoot: registryRoot,
		Policy:       policy,
	})
	if err != nil {
		t.Fatalf("InstallLocal() error = %v", err)
	}
	registry, err = EnableSkillForAgent(registry, "legacy-manual", "test-skill")
	if err != nil {
		t.Fatalf("EnableSkillForAgent() error = %v", err)
	}
	if err := SaveRegistry(registry); err != nil {
		t.Fatalf("SaveRegistry() error = %v", err)
	}

	snapshot, err := BuildSnapshot(SnapshotOptions{
		AgentID:      "legacy-manual",
		SessionID:    "ses_test",
		RegistryRoot: registryRoot,
		Policy:       policy,
	})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	result, err := MaterializeSnapshot(MaterializeOptions{
		Snapshot:     snapshot,
		Workspace:    workspace,
		RegistryRoot: registryRoot,
	})
	if err != nil {
		t.Fatalf("MaterializeSnapshot() error = %v", err)
	}
	if len(result.Materialized) != 1 {
		t.Fatalf("materialized count = %d", len(result.Materialized))
	}
	skillPath := filepath.Join(workspace, AgentsSkillsDir, "test-skill", MaterializedSKILLFilename)
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("materialized SKILL.md missing: %v", err)
	}
}

func TestValidatePolicyOK(t *testing.T) {
	cfg, err := ParsePolicy([]byte(exampleSkillPolicyYAML))
	if err != nil {
		t.Fatalf("ParsePolicy() error = %v", err)
	}
	if err := ValidatePolicy(cfg); err != nil {
		t.Fatalf("ValidatePolicy() error = %v", err)
	}
}

func TestBuildImportPlanBlocksGitInThisSprint(t *testing.T) {
	policy := Policy{AllowSources: []string{"local", "git"}}
	plan, err := BuildImportPlan("git:owner/repo@v1.0.0", policy)
	if err != nil {
		t.Fatalf("BuildImportPlan() error = %v", err)
	}
	if plan.WouldInstall {
		t.Fatal("WouldInstall = true for git plan-only source")
	}
	if !strings.Contains(plan.BlockedReason, "clone") {
		t.Fatalf("blocked_reason = %q", plan.BlockedReason)
	}
}

const exampleSkillPolicyYAML = `
skill_policy:
  default_visibility: deny
  default_update_mode: proposal_only
  allow_sources:
    - local
    - managed
  deny_sources:
    - archive
  states:
    - draft
    - active
`

func TestEnableRequiresApprovalBeforeActive(t *testing.T) {
	registry := NewRegistry(t.TempDir())
	registry.Skills["pending-skill"] = RegistrySkill{
		Name: "pending-skill", CurrentRevision: "rev1", State: LifecyclePendingApproval,
	}
	_, err := EnableSkillForAgent(registry, "legacy-manual", "pending-skill")
	if err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("EnableSkillForAgent() error = %v, want approval required", err)
	}
	registry, err = ApproveSkillInstallation(registry, "pending-skill")
	if err != nil {
		t.Fatalf("ApproveSkillInstallation() error = %v", err)
	}
	registry, err = EnableSkillForAgent(registry, "legacy-manual", "pending-skill")
	if err != nil {
		t.Fatalf("EnableSkillForAgent() after approve error = %v", err)
	}
	if registry.Skills["pending-skill"].State != LifecycleActive {
		t.Fatalf("state = %q, want active", registry.Skills["pending-skill"].State)
	}
}

func TestValidateSnapshotAgainstRegistryRejectsHashMismatch(t *testing.T) {
	dir := t.TempDir()
	skillDir := writeExampleSkill(t, dir, nil)
	registryRoot := filepath.Join(dir, "registry")
	policy := Policy{DefaultVisibility: "allow", AllowSources: []string{"local"}}
	if _, registry, err := InstallLocal(InstallOptions{SourcePath: skillDir, RegistryRoot: registryRoot, Policy: policy}); err != nil {
		t.Fatalf("InstallLocal() error = %v", err)
	} else {
		registry, err = EnableSkillForAgent(registry, "legacy-manual", "test-skill")
		if err != nil {
			t.Fatalf("EnableSkillForAgent() error = %v", err)
		}
		_ = SaveRegistry(registry)
	}
	snapshot, err := BuildSnapshot(SnapshotOptions{
		AgentID: "legacy-manual", SessionID: "ses_test", RegistryRoot: registryRoot, Policy: policy,
	})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	snapshot.Skills[0].SHA256 = "deadbeef"
	if err := ValidateSnapshotAgainstRegistry(snapshot, registryRoot); err == nil {
		t.Fatal("ValidateSnapshotAgainstRegistry() expected hash mismatch error")
	}
}
