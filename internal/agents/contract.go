package agents

const LegacyManualAgentID = "legacy-manual"

const (
	StatusActive      = "active"
	StatusPaused      = "paused"
	StatusDraining    = "draining"
	StatusTerminated  = "terminated"
	StatusQuarantined = "quarantined"
)

const (
	SessionKindMain      = "main"
	SessionKindHeartbeat = "heartbeat"
	SessionKindScratch   = "scratch"
	SessionStatusActive  = "active"
	SessionStatusClosed  = "closed"
)

const (
	WorkItemStatusQueued         = "queued"
	WorkItemStatusAssigned       = "assigned"
	WorkItemStatusLeased         = "leased"
	WorkItemStatusRunning        = "running"
	WorkItemStatusReviewRequired = "review_required"
	WorkItemStatusCompleted      = "completed"
	WorkItemStatusSucceeded      = "succeeded"
	WorkItemStatusFailed         = "failed"
	WorkItemStatusCancelled      = "cancelled"
	WorkItemStatusBlocked        = "blocked"
	WorkItemStatusDeadLetter     = "dead_letter"
)

const (
	InboxStatusPending  = "pending"
	InboxStatusAccepted = "accepted"
	InboxStatusDeferred = "deferred"
)

const (
	EventAgentCreated        = "agent.created"
	EventAgentUpdated        = "agent.updated"
	EventAgentPaused         = "agent.paused"
	EventAgentResumed        = "agent.resumed"
	EventAgentTerminated     = "agent.terminated"
	EventAgentAssigned       = "agent.assigned"
	EventAgentDelegated      = "agent.delegated"
	EventAgentSessionCreated = "agent.session.created"
	EventInboxAccepted       = "agent.inbox.accepted"
	EventInboxDeferred       = "agent.inbox.deferred"
)

const DefaultMaxDelegationDepth = 2
