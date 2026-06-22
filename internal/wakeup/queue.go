package wakeup

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

func Claim(w Wakeup, now time.Time) (ClaimResult, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	switch w.Status {
	case StatusQueued:
		w.Status = StatusClaimed
		w.ClaimedAt = now.Format(time.RFC3339Nano)
		w.Attempt++
		return ClaimResult{Wakeup: w, Claimed: true}, nil
	case StatusClaimed, StatusRunning:
		return ClaimResult{Wakeup: w, Claimed: false}, nil
	default:
		return ClaimResult{}, fmt.Errorf("wakeup %q status %q cannot be claimed", w.ID, w.Status)
	}
}

func MarkRunning(w Wakeup, now time.Time) Wakeup {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	w.Status = StatusRunning
	if w.ClaimedAt == "" {
		w.ClaimedAt = now.Format(time.RFC3339Nano)
	}
	return w
}

func Finish(opts FinishOptions) (Wakeup, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	status := strings.TrimSpace(opts.Status)
	switch status {
	case StatusSucceeded, StatusFailed, StatusSkipped, StatusCancelled, StatusExpired, StatusLost:
	default:
		return Wakeup{}, fmt.Errorf("finish status %q is not allowed", status)
	}
	w := opts.Wakeup
	w.Status = status
	w.RunID = strings.TrimSpace(opts.RunID)
	w.SkipReason = strings.TrimSpace(opts.SkipReason)
	w.FinishedAt = now.Format(time.RFC3339Nano)
	return w, nil
}

func RecoverStaleClaims(items []Wakeup, now time.Time, ttl time.Duration) ([]Wakeup, RecoveryReport) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	findings := make([]RecoveryFinding, 0)
	updated := make([]Wakeup, 0, len(items))
	for _, w := range items {
		if w.Status != StatusClaimed && w.Status != StatusRunning {
			updated = append(updated, w)
			continue
		}
		if w.ClaimedAt == "" {
			updated = append(updated, w)
			continue
		}
		claimedAt, err := time.Parse(time.RFC3339Nano, w.ClaimedAt)
		if err != nil {
			updated = append(updated, w)
			continue
		}
		if now.Sub(claimedAt) <= ttl {
			updated = append(updated, w)
			continue
		}
		w.Status = StatusLost
		w.FinishedAt = now.Format(time.RFC3339Nano)
		w.SkipReason = "claim ttl exceeded"
		updated = append(updated, w)
		findings = append(findings, RecoveryFinding{
			Severity: "warning",
			Code:     "stale_claim",
			Message:  "claimed wakeup exceeded ttl and was marked lost",
			WakeupID: w.ID,
		})
	}
	status := "ok"
	if len(findings) > 0 {
		status = "warning"
	}
	return updated, RecoveryReport{Status: status, Findings: findings}
}

func HasActiveForSchedule(items []Wakeup, scheduleID string) bool {
	for _, w := range items {
		if w.ScheduleID != scheduleID {
			continue
		}
		switch w.Status {
		case StatusClaimed, StatusRunning:
			return true
		}
	}
	return false
}

func ValidateFinish(w Wakeup) error {
	if strings.TrimSpace(w.ID) == "" {
		return errors.New("wakeup id is required")
	}
	return nil
}
