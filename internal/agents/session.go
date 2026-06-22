package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

type Session struct {
	ID                string `json:"id"`
	AgentID           string `json:"agent_id"`
	Kind              string `json:"kind"`
	Status            string `json:"status"`
	Worker            string `json:"worker,omitempty"`
	ModelProfile      string `json:"model_profile,omitempty"`
	WorkspacePath     string `json:"workspace_path,omitempty"`
	SkillSnapshotPath string `json:"skill_snapshot_path,omitempty"`
	MemorySnapshotID  string `json:"memory_snapshot_id,omitempty"`
	CreatedAt         string `json:"created_at"`
	LastActiveAt      string `json:"last_active_at"`
}

type CreateSessionOptions struct {
	Agent             Agent
	Kind              string
	SessionID         string
	Worker            string
	ModelProfile      string
	WorkspacePath     string
	SkillSnapshotPath string
	Now               time.Time
}

func CreateSession(opts CreateSessionOptions) (Session, error) {
	if err := CanStartRun(opts.Agent); err != nil {
		return Session{}, err
	}
	kind := strings.TrimSpace(opts.Kind)
	if kind == "" {
		kind = SessionKindMain
	}
	worker := strings.TrimSpace(opts.Worker)
	if worker == "" {
		worker = opts.Agent.DefaultWorker
	}
	modelProfile := strings.TrimSpace(opts.ModelProfile)
	if modelProfile == "" {
		modelProfile = opts.Agent.ModelProfile
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		sessionID = newSessionID(opts.Agent.ID, kind, now)
	}
	return Session{
		ID:                sessionID,
		AgentID:           opts.Agent.ID,
		Kind:              kind,
		Status:            SessionStatusActive,
		Worker:            worker,
		ModelProfile:      modelProfile,
		WorkspacePath:     strings.TrimSpace(opts.WorkspacePath),
		SkillSnapshotPath: strings.TrimSpace(opts.SkillSnapshotPath),
		CreatedAt:         now.Format(time.RFC3339Nano),
		LastActiveAt:      now.Format(time.RFC3339Nano),
	}, nil
}

func newSessionID(agentID string, kind string, now time.Time) string {
	sum := sha256.Sum256([]byte(agentID + "|" + kind + "|" + now.Format(time.RFC3339Nano)))
	return "ses_" + hex.EncodeToString(sum[:8])
}

func ValidateSessionImmutable(existing Session, updated Session) error {
	if existing.ID != updated.ID {
		return errors.New("session id mismatch")
	}
	if existing.AgentID != updated.AgentID {
		return errors.New("session agent_id is immutable")
	}
	if existing.Kind != updated.Kind {
		return errors.New("session kind is immutable")
	}
	if existing.SkillSnapshotPath != "" && updated.SkillSnapshotPath != "" && existing.SkillSnapshotPath != updated.SkillSnapshotPath {
		return errors.New("session skill snapshot is immutable")
	}
	return nil
}
