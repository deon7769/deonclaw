package runs

import "time"

type RunStatus string

const (
	StatusPending      RunStatus = "pending"
	StatusRunning      RunStatus = "running"
	StatusSucceeded    RunStatus = "succeeded"
	StatusFailed       RunStatus = "failed"
	StatusPolicyFailed RunStatus = "policy_failed"
	StatusCancelled    RunStatus = "cancelled"
)

type Run struct {
	ID            string     `json:"id"`
	TaskID        string     `json:"task_id"`
	Status        RunStatus  `json:"status"`
	Worker        string     `json:"worker"`
	WorkspacePath string     `json:"workspace_path"`
	AgentID       string     `json:"agent_id,omitempty"`
	SessionID     string     `json:"session_id,omitempty"`
	WorkItemID    string     `json:"work_item_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	FinishedAt    *time.Time `json:"finished_at,omitempty"`
}

func (s RunStatus) IsTerminal() bool {
	switch s {
	case StatusSucceeded, StatusFailed, StatusPolicyFailed, StatusCancelled:
		return true
	default:
		return false
	}
}
