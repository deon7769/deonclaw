package budget

import (
	"fmt"
	"sort"
	"strings"
)

type StatusEntry struct {
	PolicyID          string `json:"policy_id"`
	WindowID          string `json:"window_id"`
	AgentID           string `json:"agent_id,omitempty"`
	HardLimitMicroUSD int64  `json:"hard_limit_microusd"`
	ReservedMicroUSD  int64  `json:"reserved_microusd"`
	CommittedMicroUSD int64  `json:"committed_microusd"`
	AvailableMicroUSD int64  `json:"available_microusd"`
	WarningLevel      string `json:"warning_level,omitempty"`
	Status            string `json:"status"`
}

type StatusReport struct {
	AgentID string        `json:"agent_id,omitempty"`
	Entries []StatusEntry `json:"entries"`
}

type PlanEntry struct {
	PolicyID                string `json:"policy_id"`
	Scope                   string `json:"scope"`
	HardLimitMicroUSD       int64  `json:"hard_limit_microusd"`
	DefaultEstimateMicroUSD int64  `json:"default_estimate_microusd"`
	MaxSingleRunMicroUSD    int64  `json:"max_single_run_microusd"`
	ReserveBeforeRun        bool   `json:"reserve_before_run"`
	OnExhausted             string `json:"on_exhausted"`
}

type PlanReport struct {
	AgentID string      `json:"agent_id,omitempty"`
	Entries []PlanEntry `json:"entries"`
}

func BuildStatusReport(agentID string, windows []Window, policies map[string]Policy) StatusReport {
	report := StatusReport{AgentID: strings.TrimSpace(agentID)}
	for _, window := range windows {
		policy := policies[window.PolicyID]
		if agentID != "" && policy.Scope == ScopeAgent && policy.AgentID != agentID {
			continue
		}
		available := window.HardLimitMicroUSD - window.CommittedMicroUSD - window.ReservedMicroUSD
		if available < 0 {
			available = 0
		}
		entry := StatusEntry{
			PolicyID:          window.PolicyID,
			WindowID:          window.ID,
			AgentID:           policy.AgentID,
			HardLimitMicroUSD: window.HardLimitMicroUSD,
			ReservedMicroUSD:  window.ReservedMicroUSD,
			CommittedMicroUSD: window.CommittedMicroUSD,
			AvailableMicroUSD: available,
			Status:            window.Status,
			WarningLevel:      warningLevel(window, policy),
		}
		report.Entries = append(report.Entries, entry)
	}
	sort.Slice(report.Entries, func(i, j int) bool { return report.Entries[i].PolicyID < report.Entries[j].PolicyID })
	return report
}

func BuildPlanReport(cfg Config, agentID string) PlanReport {
	report := PlanReport{AgentID: strings.TrimSpace(agentID)}
	for id, policy := range cfg.BudgetPolicies {
		if agentID != "" && policy.Scope == ScopeAgent && policy.AgentID != agentID {
			continue
		}
		if agentID != "" && policy.Scope == ScopeGlobal {
			// include global policies for any agent plan
		}
		report.Entries = append(report.Entries, PlanEntry{
			PolicyID:                id,
			Scope:                   policy.Scope,
			HardLimitMicroUSD:       policy.HardLimitMicroUSD,
			DefaultEstimateMicroUSD: policy.DefaultEstimateMicroUSD,
			MaxSingleRunMicroUSD:    policy.MaxSingleRunMicroUSD,
			ReserveBeforeRun:        policy.ReserveBeforeRun,
			OnExhausted:             policy.OnExhausted,
		})
	}
	sort.Slice(report.Entries, func(i, j int) bool { return report.Entries[i].PolicyID < report.Entries[j].PolicyID })
	return report
}

func warningLevel(window Window, policy Policy) string {
	if window.HardLimitMicroUSD <= 0 {
		return ""
	}
	used := window.CommittedMicroUSD + window.ReservedMicroUSD
	ratioBps := used * 10000 / window.HardLimitMicroUSD
	thresholds := append([]int(nil), policy.WarningThresholdBasisPoints...)
	sort.Slice(thresholds, func(i, j int) bool { return thresholds[i] > thresholds[j] })
	for _, threshold := range thresholds {
		if ratioBps >= int64(threshold) {
			return fmt.Sprintf("%dbps", threshold)
		}
	}
	return ""
}
