package dispatch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/skills"
)

type SkillPrepareOptions struct {
	Agent        agents.Agent
	SessionID    string
	Workspace    string
	RegistryRoot string
	SkillPolicy  skills.Policy
	Now          time.Time
}

type SkillPrepareResult struct {
	Session            agents.Session
	Snapshot           skills.SessionSnapshot
	SnapshotSHA256     string
	Materialized       bool
	MaterializedSkills []string
}

func PrepareSkills(ctx context.Context, repo Repository, opts SkillPrepareOptions) (SkillPrepareResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	registryRoot := strings.TrimSpace(opts.RegistryRoot)
	if registryRoot == "" {
		return SkillPrepareResult{}, fmt.Errorf("registry root is required for dispatch")
	}
	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		workspace = filepath.Join(os.TempDir(), "deonclaw-dispatch-workspace")
	}
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return SkillPrepareResult{}, err
	}

	session, err := agents.CreateSession(agents.CreateSessionOptions{
		Agent: opts.Agent, Kind: agents.SessionKindMain, SessionID: opts.SessionID,
		WorkspacePath: workspace, Now: now,
	})
	if err != nil {
		return SkillPrepareResult{}, err
	}

	snapshot, err := skills.BuildSnapshot(skills.SnapshotOptions{
		AgentID: opts.Agent.ID, SessionID: session.ID, RegistryRoot: registryRoot,
		Policy: opts.SkillPolicy, Now: now,
	})
	if err != nil {
		return SkillPrepareResult{}, err
	}
	snapshotPath := filepath.Join(registryRoot, "snapshots", session.ID+".json")
	if err := os.MkdirAll(filepath.Dir(snapshotPath), 0o755); err != nil {
		return SkillPrepareResult{}, err
	}
	if err := skills.WriteSnapshotJSON(snapshot, snapshotPath); err != nil {
		return SkillPrepareResult{}, err
	}
	session.SkillSnapshotPath = snapshotPath
	if err := repo.SaveSession(ctx, session); err != nil {
		return SkillPrepareResult{}, err
	}

	mat, err := skills.MaterializeSnapshot(skills.MaterializeOptions{
		Snapshot: snapshot, Workspace: workspace, RegistryRoot: registryRoot,
	})
	if err != nil {
		return SkillPrepareResult{}, err
	}
	return SkillPrepareResult{
		Session: session, Snapshot: snapshot, SnapshotSHA256: snapshot.SHA256,
		Materialized: true, MaterializedSkills: mat.Materialized,
	}, nil
}
