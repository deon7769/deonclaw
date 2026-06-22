package schedule

const (
	KindAt        = "at"
	KindEvery     = "every"
	KindCron      = "cron"
	KindWebhook   = "webhook"
	KindHeartbeat = "heartbeat"
	KindHook      = "hook"

	StatusActive    = "active"
	StatusPaused    = "paused"
	StatusCancelled = "cancelled"

	ConcurrencyAllowParallel     = "allow_parallel"
	ConcurrencySkipIfRunning     = "skip_if_running"
	ConcurrencyQueueAfterRunning = "queue_after_running"
	ConcurrencyReplacePending    = "replace_pending"
	ConcurrencyCoalesce          = "coalesce"

	CatchUpNextOnly = "next_only"
	CatchUpAll      = "all"
)

type ActiveHours struct {
	Start    string `yaml:"start" json:"start"`
	End      string `yaml:"end" json:"end"`
	Timezone string `yaml:"timezone,omitempty" json:"timezone,omitempty"`
}

type Schedule struct {
	ID                 string      `yaml:"id" json:"id"`
	Kind               string      `yaml:"kind" json:"kind"`
	Name               string      `yaml:"name" json:"name"`
	Status             string      `yaml:"status" json:"status"`
	AgentID            string      `yaml:"agent_id" json:"agent_id"`
	WorkTemplateID     string      `yaml:"work_template_id,omitempty" json:"work_template_id,omitempty"`
	At                 string      `yaml:"at,omitempty" json:"at,omitempty"`
	Every              string      `yaml:"every,omitempty" json:"every,omitempty"`
	Cron               string      `yaml:"cron,omitempty" json:"cron,omitempty"`
	Timezone           string      `yaml:"timezone,omitempty" json:"timezone,omitempty"`
	ActiveHours        ActiveHours `yaml:"active_hours,omitempty" json:"active_hours,omitempty"`
	ConcurrencyPolicy  string      `yaml:"concurrency_policy,omitempty" json:"concurrency_policy,omitempty"`
	CatchUpPolicy      string      `yaml:"catch_up_policy,omitempty" json:"catch_up_policy,omitempty"`
	MaxLatenessSeconds int         `yaml:"max_lateness_seconds,omitempty" json:"max_lateness_seconds,omitempty"`
	JitterSeconds      int         `yaml:"jitter_seconds,omitempty" json:"jitter_seconds,omitempty"`
	DeliveryPolicy     string      `yaml:"delivery_policy,omitempty" json:"delivery_policy,omitempty"`
	HeartbeatPolicyID  string      `yaml:"heartbeat_policy_id,omitempty" json:"heartbeat_policy_id,omitempty"`
	NextDueAt          string      `yaml:"next_due_at,omitempty" json:"next_due_at,omitempty"`
	CreatedAt          string      `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt          string      `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
}

type Config struct {
	Schedules []Schedule `yaml:"schedules" json:"schedules"`
}
