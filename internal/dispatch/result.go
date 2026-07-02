package dispatch

import (
	"github.com/deon7769/deonclaw/internal/insights"
	"github.com/deon7769/deonclaw/internal/runs"
)

type OnceResult struct {
	Status                string                   `json:"status"`
	RunID                 string                   `json:"run_id,omitempty"`
	WorkItemID            string                   `json:"work_item_id"`
	LeaseID               string                   `json:"lease_id,omitempty"`
	ReservationID         string                   `json:"reservation_id,omitempty"`
	Mode                  string                   `json:"mode"`
	LeaseRequired         bool                     `json:"lease_required"`
	LeaseAcquired         bool                     `json:"lease_acquired"`
	DispatchStarted       bool                     `json:"dispatch_started"`
	Worker                string                   `json:"worker,omitempty"`
	WorkerStarted         bool                     `json:"worker_started"`
	SkillSnapshotApplied  bool                     `json:"skill_snapshot_applied,omitempty"`
	BudgetReserved        bool                     `json:"budget_reserved"`
	UsageRecorded         bool                     `json:"usage_recorded"`
	BudgetCommitted       bool                     `json:"budget_committed"`
	LeaseReleased         bool                     `json:"lease_released,omitempty"`
	BudgetReleased        bool                     `json:"budget_released,omitempty"`
	WorkStatus            string                   `json:"work_status,omitempty"`
	LeaseStatus           string                   `json:"lease_status,omitempty"`
	EvidenceBundleCreated bool                     `json:"evidence_bundle_created"`
	ReviewWorkQueued      bool                     `json:"review_work_queued"`
	BlockedReason         string                   `json:"blocked_reason,omitempty"`
	StepsCompleted        []string                 `json:"steps_completed"`
	RunStatus             runs.RunStatus           `json:"run_status,omitempty"`
	BudgetBlocked         *BudgetBlockedContract   `json:"budget_blocked,omitempty"`
	EvidenceBundle        *insights.EvidenceBundle `json:"evidence_bundle,omitempty"`
	InsightReview         *InsightReviewResult     `json:"insight_review,omitempty"`
	LearningLoop          *LearningLoopResult      `json:"learning_loop,omitempty"`
	Error                 string                   `json:"error,omitempty"`
	ProviderCall          bool                     `json:"provider_call"`
	NetworkCall           bool                     `json:"network_call"`
	SecretValuesRead      bool                     `json:"secret_values_read"`
}

func NewSuccessResult(opts OnceOptions, steps []string, runID string, status runs.RunStatus, meta SuccessMeta) OnceResult {
	realDispatch := opts.Mode == ModeReal
	return OnceResult{
		Status:                "ok",
		RunID:                 runID,
		WorkItemID:            opts.WorkItemID,
		LeaseID:               meta.LeaseID,
		ReservationID:         meta.ReservationID,
		Mode:                  opts.Mode,
		LeaseRequired:         true,
		LeaseAcquired:         true,
		DispatchStarted:       true,
		Worker:                meta.Worker,
		WorkerStarted:         true,
		SkillSnapshotApplied:  meta.SkillSnapshotApplied,
		BudgetReserved:        meta.BudgetReserved,
		UsageRecorded:         meta.UsageRecorded,
		BudgetCommitted:       meta.BudgetCommitted,
		LeaseReleased:         meta.LeaseReleased,
		WorkStatus:            meta.WorkStatus,
		LeaseStatus:           meta.LeaseStatus,
		EvidenceBundleCreated: meta.EvidenceBundleCreated,
		ReviewWorkQueued:      meta.ReviewWorkQueued,
		StepsCompleted:        steps,
		RunStatus:             status,
		ProviderCall:          realDispatch,
		NetworkCall:           realDispatch,
		SecretValuesRead:      false,
	}
}

type SuccessMeta struct {
	LeaseID               string
	ReservationID         string
	Worker                string
	SkillSnapshotApplied  bool
	BudgetReserved        bool
	UsageRecorded         bool
	BudgetCommitted       bool
	LeaseReleased         bool
	WorkStatus            string
	LeaseStatus           string
	EvidenceBundleCreated bool
	ReviewWorkQueued      bool
}

func NewBudgetBlockedResult(opts OnceOptions, steps []string, contract BudgetBlockedContract) OnceResult {
	return OnceResult{
		Status:          "budget_blocked",
		WorkItemID:      opts.WorkItemID,
		Mode:            opts.Mode,
		LeaseRequired:   true,
		DispatchStarted: false,
		WorkerStarted:   false,
		StepsCompleted:  steps,
		BudgetBlocked:   &contract,
		BlockedReason:   contract.BlockedReason,
		ProviderCall:    false,
		NetworkCall:     false,
	}
}

func NewBlockedResult(opts OnceOptions, steps []string, reason, message string) OnceResult {
	return OnceResult{
		Status:          "blocked",
		WorkItemID:      opts.WorkItemID,
		Mode:            opts.Mode,
		LeaseRequired:   true,
		DispatchStarted: false,
		WorkerStarted:   false,
		BlockedReason:   reason,
		StepsCompleted:  steps,
		Error:           message,
		ProviderCall:    false,
		NetworkCall:     false,
	}
}

func NewNotStartedResult(opts OnceOptions, steps []string, leaseID string) OnceResult {
	return OnceResult{
		Status:          "not_started",
		WorkItemID:      opts.WorkItemID,
		LeaseID:         leaseID,
		Mode:            opts.Mode,
		LeaseRequired:   true,
		LeaseAcquired:   false,
		DispatchStarted: false,
		WorkerStarted:   false,
		StepsCompleted:  steps,
		ProviderCall:    false,
		NetworkCall:     false,
	}
}

func NewErrorResult(opts OnceOptions, steps []string, err error) OnceResult {
	return OnceResult{
		Status:          "failed",
		WorkItemID:      opts.WorkItemID,
		Mode:            opts.Mode,
		LeaseRequired:   true,
		DispatchStarted: false,
		WorkerStarted:   false,
		StepsCompleted:  steps,
		Error:           err.Error(),
		ProviderCall:    false,
		NetworkCall:     false,
	}
}
