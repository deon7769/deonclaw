package runreport

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestBuildGroupsByModelProfileAndHandlesLegacyRuns(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := store.OpenSQLite(filepath.Join(tempDir, "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	saveReportTask(t, ctx, db)
	createdAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	saveReportRun(t, ctx, db, "run-succeeded", runs.StatusSucceeded, "opencode", createdAt)
	saveReportRun(t, ctx, db, "run-failed", runs.StatusFailed, "opencode", createdAt.Add(time.Second))
	saveReportRun(t, ctx, db, "run-policy-legacy", runs.StatusPolicyFailed, "codex", createdAt.Add(2*time.Second))
	saveTraceArtifact(t, ctx, db, tempDir, "run-succeeded", "{\n  \"worker\": \"opencode\",\n  \"model_profile\": \"opencode-zai-glm-5-1\",\n  \"duration_ms\": 100,\n  \"parsed_events\": 4,\n  \"parse_warnings\": 1,\n  \"validation_status\": \"passed\",\n  \"changed_paths_count\": 2\n}")
	saveTraceArtifact(t, ctx, db, tempDir, "run-failed", "{\n  \"worker\": \"opencode\",\n  \"model_profile\": \"opencode-zai-glm-5-1\",\n  \"duration_ms\": 300,\n  \"parsed_events\": 2,\n  \"parse_warnings\": 2,\n  \"validation_status\": \"failed\",\n  \"changed_paths_count\": 4\n}")

	report, err := Build(ctx, db, Options{GroupBy: GroupByModelProfile})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.TotalRuns != 3 || report.Succeeded != 1 || report.Failed != 1 || report.PolicyFailed != 1 {
		t.Fatalf("status counts = %#v, want total/succeeded/failed/policy 3/1/1/1", report)
	}
	if report.ValidationFailed != 1 {
		t.Fatalf("validation_failed = %d, want 1", report.ValidationFailed)
	}
	if report.AvgDurationMS != 200 || report.AvgParsedEvents != 3 || report.TotalParseWarnings != 3 || report.AvgChangedPathsCount != 3 {
		t.Fatalf("averages = %#v, want duration=200 parsed=3 warnings=3 changed=3", report)
	}
	if report.LegacyRuns != 1 {
		t.Fatalf("legacy_runs = %d, want 1", report.LegacyRuns)
	}
	assertGroup(t, report.ByWorker, "opencode", 2, 1, 1, 0, 1)
	assertGroup(t, report.ByWorker, "codex", 1, 0, 0, 1, 0)
	assertGroup(t, report.ByModelProfile, "opencode-zai-glm-5-1", 2, 1, 1, 0, 1)
	assertGroup(t, report.ByModelProfile, UnknownLegacyGroup, 1, 0, 0, 1, 0)
}

func TestBuildRunWithoutTraceDoesNotBreak(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := store.OpenSQLite(filepath.Join(tempDir, "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	saveReportTask(t, ctx, db)
	createdAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	saveReportRun(t, ctx, db, "run-legacy", runs.StatusSucceeded, "codex", createdAt)

	report, err := Build(ctx, db, Options{})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.TotalRuns != 1 || report.LegacyRuns != 1 || report.AvgDurationMS != 0 || report.TotalParseWarnings != 0 {
		t.Fatalf("report = %#v, want one legacy run with zero trace metrics", report)
	}

	var text bytes.Buffer
	if err := WriteText(report, &text); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	if !strings.Contains(text.String(), "runs_report:") || !strings.Contains(text.String(), "legacy_runs: 1") {
		t.Fatalf("text report = %q, want legacy report", text.String())
	}
}

func TestBuildSplitsTraceWithoutModelProfile(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := store.OpenSQLite(filepath.Join(tempDir, "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	saveReportTask(t, ctx, db)
	createdAt := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	saveReportRun(t, ctx, db, "run-legacy", runs.StatusSucceeded, "codex", createdAt)
	saveReportRun(t, ctx, db, "run-no-profile", runs.StatusSucceeded, "opencode", createdAt.Add(time.Second))
	saveTraceArtifact(t, ctx, db, tempDir, "run-no-profile", "{\n  \"worker\": \"opencode\",\n  \"model_profile\": \"\",\n  \"duration_ms\": 100,\n  \"parsed_events\": 1,\n  \"parse_warnings\": 0,\n  \"validation_status\": \"passed\",\n  \"changed_paths_count\": 1\n}")

	report, err := Build(ctx, db, Options{GroupBy: GroupByModelProfile})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	assertGroup(t, report.ByModelProfile, UnknownLegacyGroup, 1, 1, 0, 0, 0)
	assertGroup(t, report.ByModelProfile, NoModelProfileGroup, 1, 1, 0, 0, 0)
}

func TestBuildFiltersWorkerStatusAndSince(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()
	db, err := store.OpenSQLite(filepath.Join(tempDir, "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	saveReportTask(t, ctx, db)
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	saveReportRun(t, ctx, db, "run-old", runs.StatusSucceeded, "opencode", base)
	saveReportRun(t, ctx, db, "run-codex", runs.StatusSucceeded, "codex", base.Add(24*time.Hour))
	saveReportRun(t, ctx, db, "run-failed", runs.StatusFailed, "opencode", base.Add(48*time.Hour))
	saveReportRun(t, ctx, db, "run-want", runs.StatusSucceeded, "opencode", base.Add(72*time.Hour))
	for _, runID := range []string{"run-old", "run-codex", "run-failed", "run-want"} {
		saveTraceArtifact(t, ctx, db, tempDir, runID, "{\n  \"worker\": \"opencode\",\n  \"model_profile\": \"opencode-zai-glm-5-1\",\n  \"duration_ms\": 100,\n  \"parsed_events\": 1,\n  \"parse_warnings\": 0,\n  \"validation_status\": \"passed\",\n  \"changed_paths_count\": 1\n}")
	}
	since := base.Add(48 * time.Hour)

	report, err := Build(ctx, db, Options{
		GroupBy: GroupByModelProfile,
		Worker:  "opencode",
		Status:  runs.StatusSucceeded,
		Since:   &since,
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if report.TotalRuns != 1 || len(report.ByModelProfile) != 1 || report.ByModelProfile[0].Group != "opencode-zai-glm-5-1" {
		t.Fatalf("report = %#v, want only filtered opencode succeeded run", report)
	}
}

func TestWriteJSONValid(t *testing.T) {
	var output bytes.Buffer
	report := Report{TotalRuns: 1, Succeeded: 1, ByWorker: []GroupReport{{Group: "codex", TotalRuns: 1, Succeeded: 1}}}
	if err := WriteJSON(report, &output); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	var decoded Report
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%s", err, output.String())
	}
	if decoded.TotalRuns != 1 || len(decoded.ByWorker) != 1 || decoded.ByWorker[0].Group != "codex" {
		t.Fatalf("decoded = %#v, want codex report", decoded)
	}
}

func saveReportTask(t *testing.T, ctx context.Context, db store.Store) {
	t.Helper()
	task := &tasks.Task{
		ID:     "task-report-001",
		Title:  "Report task",
		Domain: "general",
		Worker: "opencode",
		Goal:   "Do not execute",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"report is generated"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
}

func saveReportRun(t *testing.T, ctx context.Context, db store.Store, id string, status runs.RunStatus, worker string, createdAt time.Time) {
	t.Helper()
	run := &runs.Run{
		ID:            id,
		TaskID:        "task-report-001",
		Status:        status,
		Worker:        worker,
		WorkspacePath: "workspace",
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}
	if err := db.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun(%q) error = %v", id, err)
	}
}

func saveTraceArtifact(t *testing.T, ctx context.Context, db store.Store, root string, runID string, traceJSON string) {
	t.Helper()
	tracePath := filepath.Join(root, "artifacts", runID, "execution-trace.json")
	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(tracePath, []byte(traceJSON), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	artifact := &artifacts.Artifact{
		ID:        runID + "-trace",
		RunID:     runID,
		Path:      tracePath,
		Kind:      artifacts.KindOther,
		SizeBytes: int64(len(traceJSON)),
		SHA256:    strings.Repeat("a", 64),
		CreatedAt: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC),
	}
	if err := db.SaveArtifact(ctx, artifact); err != nil {
		t.Fatalf("SaveArtifact(%q) error = %v", artifact.ID, err)
	}
}

func assertGroup(t *testing.T, groups []GroupReport, name string, total int, succeeded int, failed int, policyFailed int, validationFailed int) {
	t.Helper()
	for _, group := range groups {
		if group.Group != name {
			continue
		}
		if group.TotalRuns != total || group.Succeeded != succeeded || group.Failed != failed || group.PolicyFailed != policyFailed || group.ValidationFailed != validationFailed {
			t.Fatalf("group %q = %#v, want total=%d succeeded=%d failed=%d policy_failed=%d validation_failed=%d", name, group, total, succeeded, failed, policyFailed, validationFailed)
		}
		return
	}
	t.Fatalf("group %q not found in %#v", name, groups)
}
