package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type LifecycleEvent struct {
	ID        string `json:"id"`
	AgentID   string `json:"agent_id"`
	EventType string `json:"event_type"`
	Actor     string `json:"actor"`
	Payload   string `json:"payload_json"`
	CreatedAt string `json:"created_at"`
}

func NewLifecycleEvent(agentID string, eventType string, actor string, payload string, now time.Time) LifecycleEvent {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sum := sha256.Sum256([]byte(agentID + "|" + eventType + "|" + now.Format(time.RFC3339Nano)))
	return LifecycleEvent{
		ID:        "ale_" + hex.EncodeToString(sum[:8]),
		AgentID:   agentID,
		EventType: eventType,
		Actor:     actor,
		Payload:   payload,
		CreatedAt: now.Format(time.RFC3339Nano),
	}
}
