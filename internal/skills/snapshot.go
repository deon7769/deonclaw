package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type SnapshotOptions struct {
	AgentID      string
	SessionID    string
	RegistryRoot string
	Policy       Policy
	Now          time.Time
}

func BuildSnapshot(opts SnapshotOptions) (SessionSnapshot, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	agentID := strings.TrimSpace(opts.AgentID)
	if agentID == "" {
		return SessionSnapshot{}, fmt.Errorf("agent id is required")
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		return SessionSnapshot{}, fmt.Errorf("session id is required")
	}
	registry, err := LoadRegistry(opts.RegistryRoot)
	if err != nil {
		return SessionSnapshot{}, err
	}

	skills := make([]SnapshotSkill, 0)
	for name, entry := range registry.Skills {
		if entry.State != LifecycleActive && entry.State != LifecycleVerified {
			continue
		}
		if !opts.Policy.AgentMayUse(agentID, name) && !registryAgentMayUse(registry, agentID, name) {
			continue
		}
		revisionDir := RevisionDir(opts.RegistryRoot, name, entry.CurrentRevision)
		sidecar, _ := LoadSidecar(revisionDir)
		skills = append(skills, SnapshotSkill{
			Name:             name,
			Revision:         entry.CurrentRevision,
			SHA256:           entry.ContentSHA256,
			Source:           "managed",
			MaterializedPath: AgentsSkillsDir + "/" + name + "/" + MaterializedSKILLFilename,
			Permissions:      DefaultPermissions(opts.Policy, sidecar),
		})
	}

	snapshot := SessionSnapshot{
		SessionID: sessionID,
		AgentID:   agentID,
		CreatedAt: now.Format(time.RFC3339Nano),
		Skills:    skills,
	}
	hash, err := snapshotHash(snapshot)
	if err != nil {
		return SessionSnapshot{}, err
	}
	snapshot.SHA256 = hash
	return snapshot, nil
}

func registryAgentMayUse(registry Registry, agentID string, skillName string) bool {
	skills, ok := registry.AgentAllowlists[agentID]
	if !ok {
		return false
	}
	for _, allowed := range skills {
		if allowed == skillName {
			return true
		}
	}
	return false
}

func snapshotHash(snapshot SessionSnapshot) (string, error) {
	copy := snapshot
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func WriteSnapshotJSON(snapshot SessionSnapshot, path string) error {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot %q: %w", path, err)
	}
	return nil
}

func ReadSnapshotJSON(path string) (SessionSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionSnapshot{}, fmt.Errorf("read snapshot %q: %w", path, err)
	}
	var snapshot SessionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return SessionSnapshot{}, fmt.Errorf("parse snapshot %q: %w", path, err)
	}
	return snapshot, nil
}
