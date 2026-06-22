package wakeup

import "time"

const (
	StatusQueued    = "queued"
	StatusClaimed   = "claimed"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusSkipped   = "skipped"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
	StatusLost      = "lost"
)

type Wakeup struct {
	ID             string `json:"id"`
	ScheduleID     string `json:"schedule_id"`
	AgentID        string `json:"agent_id"`
	DueAt          string `json:"due_at"`
	Status         string `json:"status"`
	IdempotencyKey string `json:"idempotency_key"`
	Attempt        int    `json:"attempt"`
	RunID          string `json:"run_id,omitempty"`
	CreatedAt      string `json:"created_at"`
	ClaimedAt      string `json:"claimed_at,omitempty"`
	FinishedAt     string `json:"finished_at,omitempty"`
	SkipReason     string `json:"skip_reason,omitempty"`
}

type ClaimResult struct {
	Wakeup  Wakeup `json:"wakeup"`
	Claimed bool   `json:"claimed"`
}

type FinishOptions struct {
	Wakeup     Wakeup
	Status     string
	RunID      string
	SkipReason string
	Now        time.Time
}

type RecoveryFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	WakeupID string `json:"wakeup_id,omitempty"`
}

type RecoveryReport struct {
	Status   string            `json:"status"`
	Findings []RecoveryFinding `json:"findings"`
}

const DefaultClaimTTL = 15 * 60 // seconds, used by doctor only as constant reference
