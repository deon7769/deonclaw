package events

import (
	"encoding/json"
	"time"
)

type EventType string

const (
	TypeRunStarted           EventType = "run.started"
	TypeRunCompleted         EventType = "run.completed"
	TypeRunFailed            EventType = "run.failed"
	TypeWorkerStdout         EventType = "worker.stdout"
	TypeWorkerStderr         EventType = "worker.stderr"
	TypeWorkerMessage        EventType = "worker.message"
	TypeWorkerCommandStarted EventType = "worker.command.started"
	TypeWorkerCommandDone    EventType = "worker.command.completed"
	TypeWorkerFileChanged    EventType = "worker.file.changed"
	TypeWorkerToolCalled     EventType = "worker.tool.called"
	TypePolicyFailed         EventType = "policy.failed"
	TypeArtifactCreated      EventType = "artifact.created"
)

type Event struct {
	ID        string          `json:"id"`
	RunID     string          `json:"run_id"`
	Type      EventType       `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}
