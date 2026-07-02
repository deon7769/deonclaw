package dispatch

import (
	"context"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/skills"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/usage"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

const (
	ModeFake = "fake"
	ModeReal = "real"

	BlockedBudgetPolicyRequired         = "budget_policy_required"
	BlockedRealModeUnsupported          = "real_dispatch_not_wired"
	BlockedRealModeConfirmationRequired = "real_dispatch_requires_confirmation"
	BlockedRealModeCI                   = "real_dispatch_blocked_in_ci"

	StepValidateOptions      = "01_validate_options"
	StepLoadWorkItem         = "02_load_work_item"
	StepValidateWorkItem     = "03_validate_work_item"
	StepLoadAgent            = "04_load_agent"
	StepValidateAgent        = "05_validate_agent"
	StepLoadTaskSnapshot     = "06_load_task_snapshot"
	StepValidateTaskSnapshot = "07_validate_task_snapshot"
	StepResolveBudgetPolicy  = "08_resolve_budget_policy"
	StepGetOrCreateWindow    = "09_get_or_create_window"
	StepBudgetPreflight      = "10_budget_preflight"
	StepReserveBudget        = "11_reserve_budget"
	StepVerifyLease          = "12_verify_lease"
	StepCreateRun            = "13_create_run"
	StepBindLease            = "14_bind_lease"
	StepMarkWorkRunning      = "15_mark_work_running"
	StepExecuteWorker        = "16_execute_worker"
	StepNormalizeUsage       = "17_normalize_usage"
	StepCalculateCost        = "18_calculate_cost"
	StepCommitBudget         = "19_commit_budget"
	StepPersistUsage         = "20_persist_usage"
	StepFinalizeRun          = "21_finalize_run"
	StepBuildEvidence        = "22_build_evidence"
	StepEnqueueInsightReview = "23_enqueue_insight_review"
	StepComplete             = "24_complete"
)

type Repository interface {
	WorkItem(ctx context.Context, id string) (agents.WorkItem, error)
	SaveWorkItem(ctx context.Context, item agents.WorkItem) error
	Agent(ctx context.Context, id string) (agents.Agent, error)
	SaveAgent(ctx context.Context, agent agents.Agent) error
	WorkItemTaskSnapshot(ctx context.Context, workItemID string) (agents.WorkItemTaskSnapshot, error)
	ClaimWorkItem(ctx context.Context, agentID string, workItemID string, ttl time.Duration, now time.Time) (workqueue.ClaimResult, error)
	Lease(ctx context.Context, id string) (workqueue.Lease, error)
	SaveLease(ctx context.Context, lease workqueue.Lease) error
	ReleaseLease(ctx context.Context, leaseID string, reason string, requeue bool, now time.Time) error
	ListLeases(ctx context.Context) ([]workqueue.Lease, error)
	SaveSession(ctx context.Context, session agents.Session) error
	SaveRun(ctx context.Context, run *runs.Run) error
	SaveTask(ctx context.Context, task *tasks.Task) error
	AppendWorkQueueEvent(ctx context.Context, event workqueue.QueueEvent) error
	BudgetPolicy(ctx context.Context, id string) (budget.Policy, error)
	GetOrCreateWindow(ctx context.Context, policy budget.Policy, now time.Time) (budget.Window, error)
	ReserveBudget(ctx context.Context, opts budget.ReserveBudgetOptions) (budget.ReserveBudgetResult, error)
	CommitReservation(ctx context.Context, reservationID string, event usage.Event, actualMicroUSD int64, now time.Time) (budget.Reservation, error)
	CommitReservationWithUsage(ctx context.Context, reservationID string, event usage.Event, actualMicroUSD int64, now time.Time) (budget.Reservation, error)
	ReleaseReservation(ctx context.Context, reservationID string, reason string, now time.Time) (budget.Reservation, error)
	ApplyBudgetExhausted(ctx context.Context, agent agents.Agent, policy budget.Policy, now time.Time) (agents.Agent, string, error)
	SaveUsageEvent(ctx context.Context, event usage.Event) error
	ModelPrice(ctx context.Context, id string) (usage.ModelPrice, error)
	SaveWorkItemTaskSnapshot(ctx context.Context, snapshot agents.WorkItemTaskSnapshot) error
	SaveInboxItem(ctx context.Context, item agents.InboxItem) error
}

type WorkerRunner interface {
	Run(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error)
}

type WorkerRunOptions struct {
	TaskJSON     string
	TaskID       string
	Worker       string
	ModelProfile string
	RunID        string
	Mode         string
	ConfirmReal  bool
}

type WorkerRunResult struct {
	Status       runs.RunStatus
	UsageMeta    map[string]any
	DurationMS   int64
	ErrorMessage string
}

type BudgetConfigLoader func() (budget.Config, error)

type PricingLoader func(ctx context.Context, repo Repository, modelProfile string, at time.Time) (usage.ModelPrice, error)

type EvidenceBuilder func(ctx context.Context, repo Repository, runID string, now time.Time) (insights.EvidenceBundle, error)

type OnceOptions struct {
	WorkItemID               string
	LeaseID                  string
	LeaseTTL                 time.Duration
	Mode                     string
	ConfirmWorkerDispatch    bool
	RunningInCI              bool
	ArtifactsDir             string
	RegistryRoot             string
	SkillPolicy              skills.Policy
	ReviewerResponsePath     string
	InsightPolicy            insights.Policy
	LearningApprovalDecision string
	LearningApprovalReason   string
	LearningApprovalReviewer string
	LearningConfirmApply     bool
	BudgetConfigLoader       BudgetConfigLoader
	PricingLoader            PricingLoader
	EvidenceBuilder          EvidenceBuilder
	CodexRunner              WorkerRunner
	OpenCodeRunner           WorkerRunner
	Now                      time.Time
}

type BudgetBlockedContract struct {
	Status            string `json:"status"`
	BlockedReason     string `json:"blocked_reason"`
	WorkItemID        string `json:"work_item_id"`
	AgentID           string `json:"agent_id,omitempty"`
	PolicyID          string `json:"policy_id,omitempty"`
	EstimatedMicroUSD int64  `json:"estimated_microusd,omitempty"`
	ProviderCall      bool   `json:"provider_call"`
	NetworkCall       bool   `json:"network_call"`
}
