package runreport

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
)

const (
	GroupByNone         = ""
	GroupByModelProfile = "model_profile"

	UnknownLegacyGroup = "unknown/legacy"
)

type Options struct {
	GroupBy string
}

type Report struct {
	TotalRuns            int           `json:"total_runs"`
	Succeeded            int           `json:"succeeded"`
	Failed               int           `json:"failed"`
	PolicyFailed         int           `json:"policy_failed"`
	ValidationFailed     int           `json:"validation_failed"`
	AvgDurationMS        float64       `json:"avg_duration_ms"`
	AvgParsedEvents      float64       `json:"avg_parsed_events"`
	TotalParseWarnings   int           `json:"total_parse_warnings"`
	AvgChangedPathsCount float64       `json:"avg_changed_paths_count"`
	LegacyRuns           int           `json:"legacy_runs"`
	ByWorker             []GroupReport `json:"by_worker"`
	ByModelProfile       []GroupReport `json:"by_model_profile,omitempty"`
}

type GroupReport struct {
	Group                string  `json:"group"`
	TotalRuns            int     `json:"total_runs"`
	Succeeded            int     `json:"succeeded"`
	Failed               int     `json:"failed"`
	PolicyFailed         int     `json:"policy_failed"`
	ValidationFailed     int     `json:"validation_failed"`
	AvgDurationMS        float64 `json:"avg_duration_ms"`
	AvgParsedEvents      float64 `json:"avg_parsed_events"`
	TotalParseWarnings   int     `json:"total_parse_warnings"`
	AvgChangedPathsCount float64 `json:"avg_changed_paths_count"`
	LegacyRuns           int     `json:"legacy_runs"`
}

type executionTrace struct {
	Worker            string `json:"worker"`
	ModelProfile      string `json:"model_profile"`
	DurationMS        int64  `json:"duration_ms"`
	ParsedEvents      int    `json:"parsed_events"`
	ParseWarnings     int    `json:"parse_warnings"`
	ValidationStatus  string `json:"validation_status"`
	ChangedPathsCount int    `json:"changed_paths_count"`
}

type accumulator struct {
	group               string
	totalRuns           int
	succeeded           int
	failed              int
	policyFailed        int
	validationFailed    int
	durationSum         int64
	durationSamples     int
	parsedEventsSum     int
	parsedEventsSamples int
	totalParseWarnings  int
	changedPathsSum     int
	changedPathsSamples int
	legacyRuns          int
}

func Build(ctx context.Context, db store.Store, opts Options) (Report, error) {
	if opts.GroupBy != GroupByNone && opts.GroupBy != GroupByModelProfile {
		return Report{}, fmt.Errorf("unsupported --by %q", opts.GroupBy)
	}

	runRecords, err := db.ListRuns(ctx)
	if err != nil {
		return Report{}, err
	}

	overall := accumulator{group: "all"}
	workerGroups := map[string]*accumulator{}
	profileGroups := map[string]*accumulator{}

	for _, runRecord := range runRecords {
		trace, hasTrace, err := loadExecutionTrace(ctx, db, runRecord.ID)
		if err != nil {
			return Report{}, err
		}

		overall.add(runRecord, trace, hasTrace)

		workerKey := groupKey(runRecord.Worker)
		if workerKey == UnknownLegacyGroup && hasTrace {
			workerKey = groupKey(trace.Worker)
		}
		accumulateGroup(workerGroups, workerKey, runRecord, trace, hasTrace)

		if opts.GroupBy == GroupByModelProfile {
			profileKey := UnknownLegacyGroup
			if hasTrace {
				profileKey = groupKey(trace.ModelProfile)
			}
			accumulateGroup(profileGroups, profileKey, runRecord, trace, hasTrace)
		}
	}

	report := overall.report()
	report.ByWorker = groupReports(workerGroups)
	if opts.GroupBy == GroupByModelProfile {
		report.ByModelProfile = groupReports(profileGroups)
	}
	return report, nil
}

func WriteText(report Report, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "runs_report:"); err != nil {
		return err
	}
	if err := writeMetricLines(out, reportFields(report)); err != nil {
		return err
	}
	if err := writeGroups(out, "by_worker", report.ByWorker); err != nil {
		return err
	}
	if len(report.ByModelProfile) > 0 {
		if err := writeGroups(out, "by_model_profile", report.ByModelProfile); err != nil {
			return err
		}
	}
	return nil
}

func WriteJSON(report Report, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func loadExecutionTrace(ctx context.Context, db store.Store, runID string) (executionTrace, bool, error) {
	runArtifacts, err := db.ArtifactsByRun(ctx, runID)
	if err != nil {
		return executionTrace{}, false, err
	}
	for _, artifact := range runArtifacts {
		if artifactName(artifact) != "execution-trace.json" {
			continue
		}
		content, err := os.ReadFile(artifact.Path)
		if err != nil {
			return executionTrace{}, false, nil
		}
		var trace executionTrace
		if err := json.Unmarshal(content, &trace); err != nil {
			return executionTrace{}, false, nil
		}
		return trace, true, nil
	}
	return executionTrace{}, false, nil
}

func artifactName(artifact artifacts.Artifact) string {
	name := filepath.Base(filepath.Clean(artifact.Path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func (a *accumulator) add(runRecord runs.Run, trace executionTrace, hasTrace bool) {
	a.totalRuns++
	switch runRecord.Status {
	case runs.StatusSucceeded:
		a.succeeded++
	case runs.StatusFailed:
		a.failed++
	case runs.StatusPolicyFailed:
		a.policyFailed++
	}
	if !hasTrace {
		a.legacyRuns++
		return
	}
	if trace.ValidationStatus == "failed" {
		a.validationFailed++
	}
	a.durationSum += trace.DurationMS
	a.durationSamples++
	a.parsedEventsSum += trace.ParsedEvents
	a.parsedEventsSamples++
	a.totalParseWarnings += trace.ParseWarnings
	a.changedPathsSum += trace.ChangedPathsCount
	a.changedPathsSamples++
}

func (a accumulator) report() Report {
	return Report{
		TotalRuns:            a.totalRuns,
		Succeeded:            a.succeeded,
		Failed:               a.failed,
		PolicyFailed:         a.policyFailed,
		ValidationFailed:     a.validationFailed,
		AvgDurationMS:        averageInt64(a.durationSum, a.durationSamples),
		AvgParsedEvents:      averageInt(a.parsedEventsSum, a.parsedEventsSamples),
		TotalParseWarnings:   a.totalParseWarnings,
		AvgChangedPathsCount: averageInt(a.changedPathsSum, a.changedPathsSamples),
		LegacyRuns:           a.legacyRuns,
	}
}

func (a accumulator) groupReport() GroupReport {
	report := a.report()
	return GroupReport{
		Group:                a.group,
		TotalRuns:            report.TotalRuns,
		Succeeded:            report.Succeeded,
		Failed:               report.Failed,
		PolicyFailed:         report.PolicyFailed,
		ValidationFailed:     report.ValidationFailed,
		AvgDurationMS:        report.AvgDurationMS,
		AvgParsedEvents:      report.AvgParsedEvents,
		TotalParseWarnings:   report.TotalParseWarnings,
		AvgChangedPathsCount: report.AvgChangedPathsCount,
		LegacyRuns:           report.LegacyRuns,
	}
}

func accumulateGroup(groups map[string]*accumulator, key string, runRecord runs.Run, trace executionTrace, hasTrace bool) {
	group, ok := groups[key]
	if !ok {
		group = &accumulator{group: key}
		groups[key] = group
	}
	group.add(runRecord, trace, hasTrace)
}

func groupReports(groups map[string]*accumulator) []GroupReport {
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	reports := make([]GroupReport, 0, len(keys))
	for _, key := range keys {
		reports = append(reports, groups[key].groupReport())
	}
	return reports
}

func groupKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return UnknownLegacyGroup
	}
	return value
}

func averageInt(sum int, samples int) float64 {
	if samples == 0 {
		return 0
	}
	return float64(sum) / float64(samples)
}

func averageInt64(sum int64, samples int) float64 {
	if samples == 0 {
		return 0
	}
	return float64(sum) / float64(samples)
}

func writeGroups(out io.Writer, name string, groups []GroupReport) error {
	if _, err := fmt.Fprintf(out, "\n%s:\n", name); err != nil {
		return err
	}
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "group\ttotal_runs\tsucceeded\tfailed\tpolicy_failed\tvalidation_failed\tavg_duration_ms\tavg_parsed_events\ttotal_parse_warnings\tavg_changed_paths_count\tlegacy_runs"); err != nil {
		return err
	}
	for _, group := range groups {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%d\t%d\t%d\t%d\t%d\t%.2f\t%.2f\t%d\t%.2f\t%d\n",
			group.Group,
			group.TotalRuns,
			group.Succeeded,
			group.Failed,
			group.PolicyFailed,
			group.ValidationFailed,
			group.AvgDurationMS,
			group.AvgParsedEvents,
			group.TotalParseWarnings,
			group.AvgChangedPathsCount,
			group.LegacyRuns,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

type reportField struct {
	name  string
	value string
}

func reportFields(report Report) []reportField {
	return []reportField{
		{name: "total_runs", value: fmt.Sprintf("%d", report.TotalRuns)},
		{name: "succeeded", value: fmt.Sprintf("%d", report.Succeeded)},
		{name: "failed", value: fmt.Sprintf("%d", report.Failed)},
		{name: "policy_failed", value: fmt.Sprintf("%d", report.PolicyFailed)},
		{name: "validation_failed", value: fmt.Sprintf("%d", report.ValidationFailed)},
		{name: "avg_duration_ms", value: fmt.Sprintf("%.2f", report.AvgDurationMS)},
		{name: "avg_parsed_events", value: fmt.Sprintf("%.2f", report.AvgParsedEvents)},
		{name: "total_parse_warnings", value: fmt.Sprintf("%d", report.TotalParseWarnings)},
		{name: "avg_changed_paths_count", value: fmt.Sprintf("%.2f", report.AvgChangedPathsCount)},
		{name: "legacy_runs", value: fmt.Sprintf("%d", report.LegacyRuns)},
	}
}

func writeMetricLines(out io.Writer, fields []reportField) error {
	for _, field := range fields {
		if _, err := fmt.Fprintf(out, "%s: %s\n", field.name, field.value); err != nil {
			return err
		}
	}
	return nil
}
