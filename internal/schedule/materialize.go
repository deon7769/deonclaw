package schedule

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/wakeup"
)

type MaterializeOptions struct {
	Schedule Schedule
	Now      time.Time
	Existing []wakeup.Wakeup
}

func MaterializeWakeups(opts MaterializeOptions) ([]wakeup.Wakeup, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	dueTimes, err := DueTimesForMaterialize(opts.Schedule, now)
	if err != nil {
		return nil, err
	}
	out := make([]wakeup.Wakeup, 0, len(dueTimes))
	for _, due := range dueTimes {
		key := IdempotencyKey(opts.Schedule.ID, due)
		duplicate := false
		for _, existing := range opts.Existing {
			if existing.IdempotencyKey == key {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
		out = append(out, wakeup.Wakeup{
			ID:             newWakeupID(opts.Schedule.ID, due),
			ScheduleID:     opts.Schedule.ID,
			AgentID:        opts.Schedule.AgentID,
			DueAt:          due.Format(time.RFC3339Nano),
			Status:         wakeup.StatusQueued,
			IdempotencyKey: key,
			Attempt:        0,
			CreatedAt:      now.Format(time.RFC3339Nano),
		})
	}
	return out, nil
}

func defaultEvery(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

func IdempotencyKey(scheduleID string, due time.Time) string {
	sum := sha256.Sum256([]byte(scheduleID + ":" + due.UTC().Format(time.RFC3339)))
	return "idem_" + hex.EncodeToString(sum[:12])
}

func newWakeupID(scheduleID string, due time.Time) string {
	sum := sha256.Sum256([]byte(scheduleID + "|" + due.Format(time.RFC3339Nano)))
	return "wakeup_" + hex.EncodeToString(sum[:8])
}

func AdvanceScheduleAfterRun(s Schedule, ranAt time.Time) (Schedule, error) {
	switch s.Kind {
	case KindAt:
		s.Status = StatusCancelled
	case KindEvery, KindCron, KindHeartbeat:
		next, err := ComputeNextDueAfterRun(s, ranAt)
		if err != nil {
			return Schedule{}, err
		}
		if !next.IsZero() {
			s.NextDueAt = next.Format(time.RFC3339Nano)
		}
	}
	s.UpdatedAt = ranAt.Format(time.RFC3339Nano)
	return s, nil
}

func CoalesceWakeups(items []wakeup.Wakeup) []wakeup.Wakeup {
	if len(items) <= 1 {
		return items
	}
	latest := items[0]
	for _, item := range items[1:] {
		if strings.Compare(item.DueAt, latest.DueAt) > 0 {
			latest = item
		}
	}
	return []wakeup.Wakeup{latest}
}

func ApplyConcurrencyPolicy(policy string, due []wakeup.Wakeup, hasRunning bool) ([]wakeup.Wakeup, []wakeup.Wakeup) {
	if len(due) == 0 {
		return nil, nil
	}
	switch policy {
	case ConcurrencyCoalesce:
		return CoalesceWakeups(due), nil
	case ConcurrencySkipIfRunning:
		if hasRunning {
			skipped := make([]wakeup.Wakeup, 0, len(due))
			for _, w := range due {
				copy := w
				copy.Status = wakeup.StatusSkipped
				skipped = append(skipped, copy)
			}
			return nil, skipped
		}
		return due, nil
	default:
		return due, nil
	}
}

func FormatDueReport(schedules []Schedule, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "schedules_due_report:\n")
	fmt.Fprintf(&b, "  now: %s\n", now.Format(time.RFC3339Nano))
	for _, s := range schedules {
		if s.Status != StatusActive {
			continue
		}
		due, err := ResolveNextDueAt(s, now)
		if err != nil {
			fmt.Fprintf(&b, "  - id: %s\n    error: %s\n", s.ID, err.Error())
			continue
		}
		dueLabel := "none"
		if !due.IsZero() {
			dueLabel = due.Format(time.RFC3339Nano)
		}
		fmt.Fprintf(&b, "  - id: %s\n    kind: %s\n    next_due_at: %s\n    due_now: %t\n", s.ID, s.Kind, dueLabel, !due.IsZero() && !due.After(now))
	}
	return b.String()
}
