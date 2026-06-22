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
	WorkItemStatusQueued    = "queued"
	WorkItemStatusAssigned  = "assigned"
	WorkItemStatusRunning   = "running"
	WorkItemStatusCompleted = "completed"
	WorkItemStatusCancelled = "cancelled"
	WorkItemStatusBlocked   = "blocked"
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
