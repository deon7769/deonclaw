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
	Worker                string                   `json:"worker,omitempty"`
	WorkerStarted         bool                     `json:"worker_started"`
	SkillSnapshotApplied  bool                     `json:"skill_snapshot_applied,omitempty"`
	BudgetReserved        bool                     `json:"budget_reserved"`
	UsageRecorded         bool                     `json:"usage_recorded"`
	BudgetCommitted       bool                     `json:"budget_committed"`
	WorkStatus            string                   `json:"work_status,omitempty"`
	LeaseStatus           string                   `json:"lease_status,omitempty"`
	EvidenceBundleCreated bool                     `json:"evidence_bundle_created"`
	ReviewWorkQueued      bool                     `json:"review_work_queued"`
	StepsCompleted        []string                 `json:"steps_completed"`
	RunStatus             runs.RunStatus           `json:"run_status,omitempty"`
	BudgetBlocked         *BudgetBlockedContract   `json:"budget_blocked,omitempty"`
	EvidenceBundle        *insights.EvidenceBundle `json:"evidence_bundle,omitempty"`
	InsightReview         *InsightReviewResult     `json:"insight_review,omitempty"`
	Error                 string                   `json:"error,omitempty"`
	ProviderCall          bool                     `json:"provider_call"`
	NetworkCall           bool                     `json:"network_call"`
	SecretValuesRead      bool                     `json:"secret_values_read"`
}

func NewSuccessResult(opts OnceOptions, steps []string, runID string, status runs.RunStatus, meta SuccessMeta) OnceResult {
	return OnceResult{
		Status:                "ok",
		RunID:                 runID,
		WorkItemID:            opts.WorkItemID,
		LeaseID:               meta.LeaseID,
		ReservationID:         meta.ReservationID,
		Mode:                  opts.Mode,
		Worker:                meta.Worker,
		WorkerStarted:         true,
		SkillSnapshotApplied:  meta.SkillSnapshotApplied,
		BudgetReserved:        meta.BudgetReserved,
		UsageRecorded:         meta.UsageRecorded,
		BudgetCommitted:       meta.BudgetCommitted,
		WorkStatus:            meta.WorkStatus,
		LeaseStatus:           meta.LeaseStatus,
		EvidenceBundleCreated: meta.EvidenceBundleCreated,
		ReviewWorkQueued:      meta.ReviewWorkQueued,
		StepsCompleted:        steps,
		RunStatus:             status,
		ProviderCall:          false,
		NetworkCall:           false,
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
	WorkStatus            string
	LeaseStatus           string
	EvidenceBundleCreated bool
	ReviewWorkQueued      bool
}

func NewBudgetBlockedResult(opts OnceOptions, steps []string, contract BudgetBlockedContract) OnceResult {
	return OnceResult{
		Status:         "budget_blocked",
		WorkItemID:     opts.WorkItemID,
		Mode:           opts.Mode,
		StepsCompleted: steps,
		BudgetBlocked:  &contract,
		ProviderCall:   false,
		NetworkCall:    false,
	}
}

func NewErrorResult(opts OnceOptions, steps []string, err error) OnceResult {
	return OnceResult{
		Status:         "failed",
		WorkItemID:     opts.WorkItemID,
		Mode:           opts.Mode,
		StepsCompleted: steps,
		Error:          err.Error(),
		ProviderCall:   false,
		NetworkCall:    false,
	}
}
