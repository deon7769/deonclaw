package budget

import (
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/usage"
)

type ReserveBudgetOptions struct {
	PolicyID          string
	WorkItemID        string
	RunID             string
	AgentID           string
	EstimatedMicroUSD int64
	IdempotencyKey    string
	Now               time.Time
}

type ReserveBudgetResult struct {
	Reservation Reservation `json:"reservation"`
	Created     bool        `json:"created"`
}

type CommitReservationOptions struct {
	ReservationID  string
	Usage          usage.Event
	ActualMicroUSD int64
	Now            time.Time
}

func ValidateReserveOptions(opts ReserveBudgetOptions) error {
	if strings.TrimSpace(opts.PolicyID) == "" {
		return fmt.Errorf("budget policy id is required")
	}
	if strings.TrimSpace(opts.WorkItemID) == "" {
		return fmt.Errorf("work item id is required")
	}
	if opts.EstimatedMicroUSD <= 0 {
		return fmt.Errorf("estimated_microusd must be positive")
	}
	return nil
}

func CommitAmount(reservation Reservation, actualMicroUSD int64) (Reservation, int64, int64, int64, error) {
	if actualMicroUSD < 0 {
		return Reservation{}, 0, 0, 0, fmt.Errorf("actual_microusd must be non-negative")
	}
	if reservation.Status != ReservationReserved {
		return Reservation{}, 0, 0, 0, fmt.Errorf("reservation %q status %q is not reserved", reservation.ID, reservation.Status)
	}
	releaseReserved := reservation.EstimatedMicroUSD
	overage := int64(0)
	if actualMicroUSD > reservation.EstimatedMicroUSD {
		overage = actualMicroUSD - reservation.EstimatedMicroUSD
	}
	reservation.CommittedMicroUSD = actualMicroUSD
	reservation.OverageMicroUSD = overage
	reservation.Status = ReservationCommitted
	if overage > 0 {
		reservation.Status = ReservationOverBudget
	}
	return reservation, releaseReserved, actualMicroUSD, overage, nil
}

func ReleaseAmount(reservation Reservation) int64 {
	if reservation.Status != ReservationReserved {
		return 0
	}
	return reservation.EstimatedMicroUSD
}

func ReservationReleaseReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "released"
	}
	return reason
}
