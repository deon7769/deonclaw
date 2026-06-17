package retrievalcontext_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/runs"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestBuildRetrievalReportListsRunWithTraceFields(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storePath := filepath.Join(root, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	runID := "run-retrieval-report-001"
	createdAt := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	if err := db.SaveTask(ctx, &tasks.Task{
		ID: "task-retrieval-001", Title: "Retrieval task", Domain: "general", Worker: "codex",
		Goal: "Inspect retrieval report", Mode: "read_only",
		Workspace:        tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."},
		Memory:           tasks.MemorySpec{Scope: "none"},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"done"},
	}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := db.SaveRun(ctx, &runs.Run{
		ID: runID, TaskID: "task-retrieval-001", Status: runs.StatusSucceeded,
		Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt,
	}); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	tracePath := filepath.Join(root, "artifacts", runID, "execution-trace.json")
	retrievalJSONPath := filepath.Join(root, "artifacts", runID, "retrieval-context.json")
	retrievalMDPath := filepath.Join(root, "artifacts", runID, "retrieval-context.md")
	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	traceJSON := `{
  "task_id": "task-retrieval-001",
  "retrieval_context_attached": true,
  "retrieval_context_count": 1,
  "retrieval_context_sha256": "abc123",
  "retrieval_context_status": "ok"
}`
	if err := os.WriteFile(tracePath, []byte(traceJSON), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(retrievalJSONPath, validRetrievalArtifactJSON(t), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(retrievalMDPath, []byte("# Retrieved context metadata only\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	for _, artifact := range []artifacts.Artifact{
		{ID: runID + "-trace", RunID: runID, Path: tracePath, Kind: artifacts.KindOther, SizeBytes: int64(len(traceJSON)), SHA256: strings.Repeat("a", 64), CreatedAt: createdAt},
		{ID: runID + "-retrieval-json", RunID: runID, Path: retrievalJSONPath, Kind: artifacts.KindOther, SizeBytes: 10, SHA256: strings.Repeat("b", 64), CreatedAt: createdAt},
		{ID: runID + "-retrieval-md", RunID: runID, Path: retrievalMDPath, Kind: artifacts.KindOther, SizeBytes: 10, SHA256: strings.Repeat("c", 64), CreatedAt: createdAt},
	} {
		if err := db.SaveArtifact(ctx, &artifact); err != nil {
			t.Fatalf("SaveArtifact() error = %v", err)
		}
	}

	report, err := retrievalcontext.BuildRetrievalReport(ctx, db, retrievalcontext.RetrievalReportOptions{})
	if err != nil {
		t.Fatalf("BuildRetrievalReport() error = %v", err)
	}
	if len(report.Runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(report.Runs))
	}
	entry := report.Runs[0]
	if entry.RunID != runID || entry.TaskID != "task-retrieval-001" || !entry.RetrievalContextAttached {
		t.Fatalf("entry = %#v, want retrieval run metadata", entry)
	}
	if entry.RetrievalContextStatus != "ok" || entry.RetrievalContextCount != 1 || entry.RetrievalContextSHA256 != "abc123" {
		t.Fatalf("entry trace fields = %#v", entry)
	}
	if len(entry.ArtifactPaths) != 2 {
		t.Fatalf("artifact_paths = %#v, want retrieval-context json/md", entry.ArtifactPaths)
	}
}

func TestBuildRetrievalReportDoesNotInvokeSearchPreflight(t *testing.T) {
	t.Setenv("DEONCLAW_LANCEDB_SEARCH_SCRIPT", "/definitely/missing/lancedb_search_smoke.py")
	ctx := context.Background()
	root := t.TempDir()
	storePath := filepath.Join(root, "deonclaw.db")
	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	runID := "run-retrieval-report-env-001"
	createdAt := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	if err := db.SaveTask(ctx, &tasks.Task{
		ID: "task-retrieval-001", Title: "Retrieval task", Domain: "general", Worker: "codex",
		Goal: "Inspect retrieval report", Mode: "read_only",
		Workspace:        tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."},
		Memory:           tasks.MemorySpec{Scope: "none"},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"done"},
	}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := db.SaveRun(ctx, &runs.Run{
		ID: runID, TaskID: "task-retrieval-001", Status: runs.StatusSucceeded,
		Worker: "codex", WorkspacePath: "workspace", CreatedAt: createdAt, UpdatedAt: createdAt,
	}); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
	tracePath := filepath.Join(root, "artifacts", runID, "execution-trace.json")
	if err := os.MkdirAll(filepath.Dir(tracePath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	traceJSON := `{"retrieval_context_attached":true,"retrieval_context_count":1,"retrieval_context_status":"ok"}`
	if err := os.WriteFile(tracePath, []byte(traceJSON), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := db.SaveArtifact(ctx, &artifacts.Artifact{
		ID: runID + "-trace", RunID: runID, Path: tracePath, Kind: artifacts.KindOther,
		SizeBytes: int64(len(traceJSON)), SHA256: strings.Repeat("d", 64), CreatedAt: createdAt,
	}); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}

	if _, err := retrievalcontext.BuildRetrievalReport(ctx, db, retrievalcontext.RetrievalReportOptions{}); err != nil {
		t.Fatalf("BuildRetrievalReport() error = %v, want store-only report without search preflight", err)
	}
}
