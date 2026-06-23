package budget

import (
	"fmt"
	"time"

	"github.com/deon7769/deonclaw/internal/agents"
)

func WindowBounds(policy Policy, now time.Time) (time.Time, time.Time, error) {
	loc, err := time.LoadLocation(policy.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("timezone %q: %w", policy.Timezone, err)
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 1, 0)
	return start, end, nil
}

func WindowID(policyID string, periodStart time.Time) string {
	return fmt.Sprintf("budwin_%s_%s", policyID, periodStart.Format("200601"))
}

func BuildWindow(policy Policy, now time.Time) (Window, error) {
	start, end, err := WindowBounds(policy, now)
	if err != nil {
		return Window{}, err
	}
	return Window{
		ID:                WindowID(policy.ID, start),
		PolicyID:          policy.ID,
		PeriodStart:       start.UTC().Format(time.RFC3339Nano),
		PeriodEnd:         end.UTC().Format(time.RFC3339Nano),
		Timezone:          policy.Timezone,
		HardLimitMicroUSD: policy.HardLimitMicroUSD,
		Status:            WindowOpen,
	}, nil
}

func CanReserve(window Window, policy Policy, requested int64) error {
	if requested <= 0 {
		return fmt.Errorf("requested amount must be positive")
	}
	if requested > policy.MaxSingleRunMicroUSD {
		return fmt.Errorf("requested amount exceeds max_single_run_microusd")
	}
	if window.CommittedMicroUSD+window.ReservedMicroUSD+requested > window.HardLimitMicroUSD {
		return fmt.Errorf("budget exhausted")
	}
	return nil
}

func ApplyOnExhausted(agent agents.Agent, policy Policy, now time.Time) (agents.Agent, string, error) {
	switch policy.OnExhausted {
	case OnExhaustedPauseAgent:
		target, err := agents.PauseTargetStatus(agent.Status)
		if err != nil {
			return agent, "", err
		}
		agent.Status = target
		if now.IsZero() {
			now = time.Now().UTC()
		}
		agent.UpdatedAt = now.Format(time.RFC3339Nano)
		return agent, agents.EventAgentPaused, nil
	case OnExhaustedBlockWork:
		return agent, "", nil
	default:
		return agent, "", fmt.Errorf("unsupported on_exhausted %q", policy.OnExhausted)
	}
}
