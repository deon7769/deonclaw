package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

func TestAgentsBootstrapAndSync(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "agents.db")
	store, err := OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 6, 22, 16, 0, 0, 0, time.UTC)
	agent := agents.AgentFromConfig(agents.AgentConfig{
		ID:            agents.LegacyManualAgentID,
		DisplayName:   "Legacy Manual",
		Role:          "operator",
		DefaultWorker: "codex",
	}, agents.StatusActive, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))

	if err := store.SaveAgent(ctx, agent); err != nil {
		t.Fatalf("SaveAgent() error = %v", err)
	}
	loaded, err := store.Agent(ctx, agents.LegacyManualAgentID)
	if err != nil {
		t.Fatalf("Agent() error = %v", err)
	}
	if loaded.DisplayName != "Legacy Manual" {
		t.Fatalf("display_name = %q", loaded.DisplayName)
	}

	session, err := agents.CreateSession(agents.CreateSessionOptions{Agent: agent, Kind: agents.SessionKindMain, Now: now})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if err := store.SaveSession(ctx, session); err != nil {
		t.Fatalf("SaveSession() error = %v", err)
	}
	sessions, err := store.ListSessionsByAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("ListSessionsByAgent() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("session count = %d", len(sessions))
	}
}
