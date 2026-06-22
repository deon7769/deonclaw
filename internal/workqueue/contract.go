package workqueue

const (
	LeaseStatusActive    = "active"
	LeaseStatusReleased  = "released"
	LeaseStatusLost      = "lost"
	LeaseStatusCancelled = "cancelled"

	DefaultLeaseTTLSeconds = 900

	EventClaimed   = "work.claimed"
	EventReleased  = "work.released"
	EventRecovered = "work.lease.recovered"
	EventEnqueued  = "work.enqueued"
)

type Lease struct {
	ID            string `json:"id"`
	WorkItemID    string `json:"work_item_id"`
	AgentID       string `json:"agent_id"`
	RunID         string `json:"run_id,omitempty"`
	Status        string `json:"status"`
	TTLSeconds    int    `json:"ttl_seconds"`
	ExpiresAt     string `json:"expires_at"`
	HeartbeatAt   string `json:"heartbeat_at,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	ReleaseReason string `json:"release_reason,omitempty"`
}

type QueueEvent struct {
	ID         string `json:"id"`
	WorkItemID string `json:"work_item_id"`
	EventType  string `json:"event_type"`
	Payload    string `json:"payload_json"`
	CreatedAt  string `json:"created_at"`
}

type ClaimResult struct {
	Lease    Lease  `json:"lease"`
	Claimed  bool   `json:"claimed"`
	WorkItem string `json:"work_item_id,omitempty"`
}

type RecoveryFinding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	LeaseID  string `json:"lease_id,omitempty"`
}

type RecoveryReport struct {
	Status   string            `json:"status"`
	Findings []RecoveryFinding `json:"findings"`
}

type DoctorReport struct {
	Status        string         `json:"status"`
	QueuedWork    int            `json:"queued_work"`
	LeasedWork    int            `json:"leased_work"`
	ActiveLeases  int            `json:"active_leases"`
	LeaseRecovery RecoveryReport `json:"lease_recovery"`
}
