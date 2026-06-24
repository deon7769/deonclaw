package usage

import (
	"fmt"
	"sort"
	"strings"
)

type ReportGroup struct {
	Key                    string `json:"key"`
	EventCount             int    `json:"event_count"`
	TotalActualMicroUSD    int64  `json:"total_actual_microusd"`
	TotalEstimatedMicroUSD int64  `json:"total_estimated_microusd"`
}

type Report struct {
	GroupBy string        `json:"group_by"`
	Groups  []ReportGroup `json:"groups"`
	Since   string        `json:"since,omitempty"`
}

func BuildReport(events []Event, groupBy string) Report {
	groups := map[string]*ReportGroup{}
	for _, event := range events {
		key := groupKey(event, groupBy)
		if key == "" {
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &ReportGroup{Key: key}
			groups[key] = g
		}
		g.EventCount++
		g.TotalActualMicroUSD += event.ActualCostMicroUSD
		g.TotalEstimatedMicroUSD += event.EstimatedCostMicroUSD
	}
	out := make([]ReportGroup, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return Report{GroupBy: groupBy, Groups: out}
}

func groupKey(event Event, groupBy string) string {
	switch strings.TrimSpace(groupBy) {
	case "agent":
		return event.AgentID
	case "worker":
		return event.Worker
	case "model_profile":
		return event.ModelProfile
	default:
		return ""
	}
}

func ValidateReportGroupBy(groupBy string) error {
	switch groupBy {
	case "agent", "worker", "model_profile":
		return nil
	default:
		return fmt.Errorf("unsupported report group %q", groupBy)
	}
}
