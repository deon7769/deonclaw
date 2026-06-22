package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

func SyncFromConfig(cfg Config, now time.Time) []Agent {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stamp := now.Format(time.RFC3339Nano)
	agents := make([]Agent, 0, len(cfg.Agents.List))
	for _, item := range cfg.Agents.List {
		agents = append(agents, AgentFromConfig(item, StatusActive, stamp, stamp))
	}
	return agents
}

func EncodeAgentConfig(agent Agent) (string, error) {
	data, err := json.Marshal(agent)
	if err != nil {
		return "", fmt.Errorf("encode agent config: %w", err)
	}
	return string(data), nil
}

func EncodeWorkItemConfig(item WorkItem) (string, error) {
	data, err := json.Marshal(item)
	if err != nil {
		return "", fmt.Errorf("encode work item config: %w", err)
	}
	return string(data), nil
}

type Repository interface {
	SaveAgent(ctx context.Context, agent Agent) error
	Agent(ctx context.Context, id string) (Agent, error)
	ListAgents(ctx context.Context) ([]Agent, error)
	UpdateAgentStatus(ctx context.Context, id string, status string, updatedAt time.Time) error
	SaveLifecycleEvent(ctx context.Context, event LifecycleEvent) error
	SaveSession(ctx context.Context, session Session) error
	Session(ctx context.Context, id string) (Session, error)
	ListSessionsByAgent(ctx context.Context, agentID string) ([]Session, error)
	SaveWorkItem(ctx context.Context, item WorkItem) error
	WorkItem(ctx context.Context, id string) (WorkItem, error)
	SaveInboxItem(ctx context.Context, item InboxItem) error
	InboxItem(ctx context.Context, id string) (InboxItem, error)
	ListInboxByAgent(ctx context.Context, agentID string) ([]InboxItem, error)
	UpdateInboxItem(ctx context.Context, item InboxItem) error
}
