package schedule

import (
	"fmt"
	"strings"
	"time"
)

func ParseScheduleTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("schedule time is empty")
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("schedule time %q is not RFC3339", raw)
}

// ResolveNextDueAt returns the persisted next due time when set, otherwise computes it.
func ResolveNextDueAt(s Schedule, reference time.Time) (time.Time, error) {
	if strings.TrimSpace(s.NextDueAt) != "" {
		return ParseScheduleTime(s.NextDueAt)
	}
	return ComputeNextDueAt(s, reference)
}

// ComputeNextDueAt computes the next scheduled occurrence at or after reference.
func ComputeNextDueAt(s Schedule, reference time.Time) (time.Time, error) {
	if reference.IsZero() {
		reference = time.Now().UTC()
	}
	switch s.Kind {
	case KindHeartbeat:
		every := defaultEvery(s.Every, "30m")
		d, err := ParseEveryDuration(every)
		if err != nil {
			return time.Time{}, err
		}
		return reference.UTC().Truncate(d), nil
	default:
		return NextDue(s, reference)
	}
}

// ComputeNextDueAfterRun returns the next occurrence strictly after a completed run.
func ComputeNextDueAfterRun(s Schedule, ranAt time.Time) (time.Time, error) {
	if ranAt.IsZero() {
		ranAt = time.Now().UTC()
	}
	switch s.Kind {
	case KindAt:
		return time.Time{}, nil
	case KindEvery:
		d, err := ParseEveryDuration(s.Every)
		if err != nil {
			return time.Time{}, err
		}
		return ranAt.UTC().Add(d), nil
	case KindCron:
		sched, err := ParseCronExpression(s.Cron, s.Timezone)
		if err != nil {
			return time.Time{}, err
		}
		return sched.Next(ranAt), nil
	case KindHeartbeat:
		every := defaultEvery(s.Every, "30m")
		d, err := ParseEveryDuration(every)
		if err != nil {
			return time.Time{}, err
		}
		base := ranAt.UTC().Truncate(d)
		if !ranAt.After(base) {
			return base.Add(d), nil
		}
		return base.Add(d), nil
	default:
		return time.Time{}, fmt.Errorf("kind %q does not support next due after run", s.Kind)
	}
}

func dueWithinLateness(due time.Time, now time.Time, maxLatenessSeconds int) bool {
	if maxLatenessSeconds <= 0 {
		return true
	}
	lateness := now.Sub(due)
	if lateness < 0 {
		return false
	}
	return lateness <= time.Duration(maxLatenessSeconds)*time.Second
}

// DueTimesForMaterialize returns concrete due timestamps that should become wakeups now.
// Only includes due_at <= now and respects catch_up_policy and max_lateness_seconds.
func DueTimesForMaterialize(s Schedule, now time.Time) ([]time.Time, error) {
	if s.Status != StatusActive {
		return nil, nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if !WithinActiveHours(s, now) {
		return nil, nil
	}

	first, err := ResolveNextDueAt(s, now)
	if err != nil {
		return nil, err
	}
	if first.IsZero() || first.After(now) {
		return nil, nil
	}

	catchUp := s.CatchUpPolicy
	if catchUp == "" {
		catchUp = CatchUpNextOnly
	}

	out := make([]time.Time, 0, 1)
	current := first.UTC()
	for {
		if current.After(now) {
			break
		}
		if dueWithinLateness(current, now, s.MaxLatenessSeconds) {
			out = append(out, current)
		}
		if catchUp != CatchUpAll {
			break
		}
		next, err := ComputeNextDueAfterRun(s, current)
		if err != nil {
			return nil, err
		}
		if next.IsZero() || !next.After(current) {
			break
		}
		current = next.UTC()
	}
	return out, nil
}
