package budget

import (
	"fmt"
	"strings"
	"time"
)

const (
	ScopeGlobal       = "global"
	ScopeAgent        = "agent"
	ScopeModelProfile = "model_profile"

	PeriodMonthly = "monthly"

	OnExhaustedBlockWork  = "block_work"
	OnExhaustedPauseAgent = "pause_agent"

	ReservationReserved   = "reserved"
	ReservationCommitted  = "committed"
	ReservationReleased   = "released"
	ReservationExpired    = "expired"
	ReservationCancelled  = "cancelled"
	ReservationOverBudget = "over_budget"

	WindowOpen = "open"
)

type Policy struct {
	ID                          string `yaml:"-" json:"id"`
	Scope                       string `yaml:"scope" json:"scope"`
	AgentID                     string `yaml:"agent_id,omitempty" json:"agent_id,omitempty"`
	ModelProfile                string `yaml:"model_profile,omitempty" json:"model_profile,omitempty"`
	Period                      string `yaml:"period" json:"period"`
	Timezone                    string `yaml:"timezone" json:"timezone"`
	HardLimitMicroUSD           int64  `yaml:"hard_limit_microusd" json:"hard_limit_microusd"`
	WarningThresholdBasisPoints []int  `yaml:"warning_threshold_basis_points" json:"warning_threshold_basis_points"`
	ReserveBeforeRun            bool   `yaml:"reserve_before_run" json:"reserve_before_run"`
	DefaultEstimateMicroUSD     int64  `yaml:"default_estimate_microusd" json:"default_estimate_microusd"`
	MaxSingleRunMicroUSD        int64  `yaml:"max_single_run_microusd" json:"max_single_run_microusd"`
	OnExhausted                 string `yaml:"on_exhausted" json:"on_exhausted"`
	AllowOperatorOverride       bool   `yaml:"allow_operator_override" json:"allow_operator_override"`
}

type Window struct {
	ID                string `json:"id"`
	PolicyID          string `json:"policy_id"`
	PeriodStart       string `json:"period_start"`
	PeriodEnd         string `json:"period_end"`
	Timezone          string `json:"timezone"`
	HardLimitMicroUSD int64  `json:"hard_limit_microusd"`
	ReservedMicroUSD  int64  `json:"reserved_microusd"`
	CommittedMicroUSD int64  `json:"committed_microusd"`
	Status            string `json:"status"`
}

type Reservation struct {
	ID                string `json:"id"`
	IdempotencyKey    string `json:"idempotency_key"`
	PolicyID          string `json:"policy_id"`
	WindowID          string `json:"window_id"`
	WorkItemID        string `json:"work_item_id"`
	RunID             string `json:"run_id,omitempty"`
	AgentID           string `json:"agent_id,omitempty"`
	EstimatedMicroUSD int64  `json:"estimated_microusd"`
	CommittedMicroUSD int64  `json:"committed_microusd"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

func NewReservationID(workItemID string, now time.Time) string {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return fmt.Sprintf("budres_%s_%d", workItemID, now.UnixNano())
}

func ReservationIdempotencyKey(workItemID, runID string) string {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return "work:" + workItemID
	}
	return "run:" + runID
}
