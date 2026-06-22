package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func ParseEveryDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, fmt.Errorf("every duration is empty")
	}
	if value == "0" || value == "0m" {
		return 0, fmt.Errorf("every duration must be greater than zero")
	}
	if strings.HasSuffix(value, "m") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "m"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid every duration %q", value)
		}
		return time.Duration(n) * time.Minute, nil
	}
	if strings.HasSuffix(value, "h") {
		n, err := strconv.Atoi(strings.TrimSuffix(value, "h"))
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("invalid every duration %q", value)
		}
		return time.Duration(n) * time.Hour, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid every duration %q: %w", value, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("every duration must be greater than zero")
	}
	return d, nil
}

func ParseCronExpression(expression string, timezone string) (cron.Schedule, error) {
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("cron expression is empty")
	}
	loc := time.UTC
	if strings.TrimSpace(timezone) != "" {
		parsed, err := time.LoadLocation(timezone)
		if err != nil {
			return nil, fmt.Errorf("timezone %q: %w", timezone, err)
		}
		loc = parsed
	}
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expression)
	if err != nil {
		return nil, fmt.Errorf("cron expression %q: %w", expression, err)
	}
	return cronSchedule{inner: sched, loc: loc}, nil
}

type cronSchedule struct {
	inner cron.Schedule
	loc   *time.Location
}

func (c cronSchedule) Next(from time.Time) time.Time {
	return c.inner.Next(from.In(c.loc))
}

func NextDue(s Schedule, from time.Time) (time.Time, error) {
	if from.IsZero() {
		from = time.Now().UTC()
	}
	switch s.Kind {
	case KindAt:
		t, err := time.Parse(time.RFC3339, s.At)
		if err != nil {
			return time.Time{}, err
		}
		if t.Before(from) {
			return time.Time{}, nil
		}
		return t, nil
	case KindEvery:
		d, err := ParseEveryDuration(s.Every)
		if err != nil {
			return time.Time{}, err
		}
		return from.Add(d), nil
	case KindCron:
		sched, err := ParseCronExpression(s.Cron, s.Timezone)
		if err != nil {
			return time.Time{}, err
		}
		return sched.Next(from), nil
	case KindHeartbeat:
		return from, nil
	default:
		return time.Time{}, fmt.Errorf("kind %q does not support next due computation", s.Kind)
	}
}

func WithinActiveHours(s Schedule, at time.Time) bool {
	if strings.TrimSpace(s.ActiveHours.Start) == "" || strings.TrimSpace(s.ActiveHours.End) == "" {
		return true
	}
	loc := time.UTC
	tz := strings.TrimSpace(s.ActiveHours.Timezone)
	if tz == "" {
		tz = s.Timezone
	}
	if tz != "" {
		if parsed, err := time.LoadLocation(tz); err == nil {
			loc = parsed
		}
	}
	local := at.In(loc)
	start, err := parseClock(s.ActiveHours.Start)
	if err != nil {
		return true
	}
	end, err := parseClock(s.ActiveHours.End)
	if err != nil {
		return true
	}
	clock := local.Hour()*60 + local.Minute()
	return clock >= start && clock <= end
}

func parseClock(value string) (int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid clock %q", value)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}
