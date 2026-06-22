package insights

import (
	"fmt"
	"strings"
)

type TriggerCounters struct {
	RunsSinceLastInsight    int `json:"runs_since_last_insight"`
	CommitsSinceLastInsight int `json:"commits_since_last_insight"`
	TurnsSinceLastInsight   int `json:"turns_since_last_insight"`
}

type TriggerDecision struct {
	ShouldEvaluate bool   `json:"should_evaluate"`
	Trigger        string `json:"trigger"`
	Reason         string `json:"reason"`
	PolicyEnabled  bool   `json:"policy_enabled"`
}

func EvaluateTrigger(policy Policy, trigger string, counters TriggerCounters) (TriggerDecision, error) {
	trigger = strings.TrimSpace(trigger)
	if err := validateTriggerType("trigger", trigger); err != nil {
		return TriggerDecision{}, err
	}

	decision := TriggerDecision{
		Trigger:       trigger,
		PolicyEnabled: policy.Enabled,
	}
	if !policy.Enabled {
		decision.Reason = "insight policy disabled"
		return decision, nil
	}

	if IsAlwaysOnTrigger(policy, trigger) {
		decision.ShouldEvaluate = true
		decision.Reason = "always_on trigger"
		return decision, nil
	}

	switch trigger {
	case TriggerRunCompleted:
		if policy.EvaluateAfterRuns > 0 && counters.RunsSinceLastInsight >= policy.EvaluateAfterRuns {
			decision.ShouldEvaluate = true
			decision.Reason = fmt.Sprintf("runs_since_last_insight=%d >= evaluate_after_runs=%d", counters.RunsSinceLastInsight, policy.EvaluateAfterRuns)
		} else {
			decision.Reason = "run threshold not met"
		}
	case TriggerCommitCreated:
		if policy.EvaluateAfterCommits > 0 && counters.CommitsSinceLastInsight >= policy.EvaluateAfterCommits {
			decision.ShouldEvaluate = true
			decision.Reason = fmt.Sprintf("commits_since_last_insight=%d >= evaluate_after_commits=%d", counters.CommitsSinceLastInsight, policy.EvaluateAfterCommits)
		} else {
			decision.Reason = "commit threshold not met"
		}
	case TriggerHeartbeatSummary:
		if policy.EvaluateAfterTurns > 0 && counters.TurnsSinceLastInsight >= policy.EvaluateAfterTurns {
			decision.ShouldEvaluate = true
			decision.Reason = fmt.Sprintf("turns_since_last_insight=%d >= evaluate_after_turns=%d", counters.TurnsSinceLastInsight, policy.EvaluateAfterTurns)
		} else {
			decision.Reason = "turn threshold not met"
		}
	default:
		decision.Reason = "trigger is not configured for threshold evaluation"
	}

	return decision, nil
}
