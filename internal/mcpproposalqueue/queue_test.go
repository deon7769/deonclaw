package mcpproposalqueue_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/mcpproposalqueue"
	"github.com/deon7769/deonclaw/internal/mcpsmoke"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/store"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
)

func TestListEmptyStore(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)

	result, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Proposals) != 0 {
		t.Fatalf("proposals = %d, want 0", len(result.Proposals))
	}
}

func TestListFindsProposalLintPreflight(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	root := t.TempDir()
	runID := "run-mcp-queue-001"

	saveTestTask(t, ctx, db, "task-mcp-queue-001")
	saveTestRun(t, ctx, db, runID, "task-mcp-queue-001", "codex")

	proposalPath := writeTestProposalArtifact(t, root, runID)
	lintPath := writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalLintArtifactName, root, `{
  "proposal_id": "mcp-call-test-001",
  "status": "passed",
  "violations": [],
  "warnings": []
}`)
	preflightPath := writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalPreflightArtifactName, root, `{
  "proposal_id": "mcp-call-test-001",
  "status": "passed",
  "warnings": [],
  "failures": []
}`)
	writeTestArtifact(t, db, ctx, runID, "mcp-tool-call-proposal.json", root, readFileString(t, proposalPath))

	result, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Proposals) != 1 {
		t.Fatalf("proposals = %d, want 1", len(result.Proposals))
	}
	entry := result.Proposals[0]
	if entry.RunID != runID || entry.TaskID != "task-mcp-queue-001" || entry.Worker != "codex" {
		t.Fatalf("entry = %#v, want run/task/worker metadata", entry)
	}
	if entry.ProposalStatus != mcpproposalqueue.StatusPreflightPassed {
		t.Fatalf("proposal_status = %q, want %q", entry.ProposalStatus, mcpproposalqueue.StatusPreflightPassed)
	}
	if entry.ProposalID != "mcp-call-test-001" || entry.Server != "fake-stdio" || entry.Tool != "deonclaw.fake.echo" {
		t.Fatalf("entry = %#v, want proposal metadata", entry)
	}
	if entry.PreflightStatus != mcpapproval.PreflightStatusPassed {
		t.Fatalf("preflight_status = %q, want passed", entry.PreflightStatus)
	}
	if entry.ArtifactPath == "" || entry.ProposalSHA256 == "" {
		t.Fatalf("entry = %#v, want artifact path and sha256", entry)
	}
	_ = lintPath
	_ = preflightPath
}

func TestListFiltersByStatusWorkerTask(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	root := t.TempDir()

	saveTestTask(t, ctx, db, "task-a")
	saveTestTask(t, ctx, db, "task-b")
	saveTestRun(t, ctx, db, "run-valid", "task-a", "codex")
	saveTestRun(t, ctx, db, "run-invalid", "task-b", "opencode")

	writeValidProposalRun(t, db, ctx, root, "run-valid")
	writeInvalidProposalRun(t, db, ctx, root, "run-invalid")

	byStatus, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{Status: mcpproposalqueue.StatusValid})
	if err != nil {
		t.Fatalf("List(status) error = %v", err)
	}
	if len(byStatus.Proposals) != 1 || byStatus.Proposals[0].RunID != "run-valid" {
		t.Fatalf("by status = %#v, want run-valid only", byStatus.Proposals)
	}

	byWorker, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{Worker: "opencode"})
	if err != nil {
		t.Fatalf("List(worker) error = %v", err)
	}
	if len(byWorker.Proposals) != 1 || byWorker.Proposals[0].RunID != "run-invalid" {
		t.Fatalf("by worker = %#v, want run-invalid only", byWorker.Proposals)
	}

	byTask, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{TaskID: "task-a"})
	if err != nil {
		t.Fatalf("List(task) error = %v", err)
	}
	if len(byTask.Proposals) != 1 || byTask.Proposals[0].RunID != "run-valid" {
		t.Fatalf("by task = %#v, want run-valid only", byTask.Proposals)
	}
}

func TestShowOmitsRawArguments(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	root := t.TempDir()
	runID := "run-mcp-show-001"

	saveTestTask(t, ctx, db, "task-show-001")
	saveTestRun(t, ctx, db, runID, "task-show-001", "codex")
	proposalPath := writeTestProposalArtifact(t, root, runID)
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalArtifactName, root, readFileString(t, proposalPath))
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalLintArtifactName, root, `{
  "proposal_id": "mcp-call-test-001",
  "status": "passed",
  "violations": [],
  "warnings": ["skipped_policy: example"]
}`)

	detail, err := mcpproposalqueue.Show(ctx, db, runID)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if detail.ProposalID != "mcp-call-test-001" || detail.Server != "fake-stdio" || detail.Tool != "deonclaw.fake.echo" {
		t.Fatalf("detail = %#v, want proposal metadata", detail)
	}
	if detail.ArgumentsSHA256 == "" {
		t.Fatalf("detail = %#v, want arguments_sha256", detail)
	}

	var stdout strings.Builder
	if err := mcpproposalqueue.WriteShowText(detail, &stdout); err != nil {
		t.Fatalf("WriteShowText() error = %v", err)
	}
	output := stdout.String()
	if strings.Contains(output, "super-secret-argument") || strings.Contains(output, `"arguments"`) {
		t.Fatalf("show text leaked raw arguments: %q", output)
	}
	if !strings.Contains(output, "arguments_sha256:") {
		t.Fatalf("show text = %q, want arguments_sha256", output)
	}

	jsonBytes, err := json.Marshal(detail)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(jsonBytes), "super-secret-argument") {
		t.Fatalf("show json leaked raw arguments: %s", string(jsonBytes))
	}
}

func TestExportPreservesProposalJSON(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	root := t.TempDir()
	runID := "run-mcp-export-001"

	saveTestTask(t, ctx, db, "task-export-001")
	saveTestRun(t, ctx, db, runID, "task-export-001", "codex")
	proposalPath := writeTestProposalArtifact(t, root, runID)
	original := readFileBytes(t, proposalPath)
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalArtifactName, root, string(original))

	exportPath := filepath.Join(root, "exported-proposal.json")
	if err := mcpproposalqueue.Export(ctx, db, runID, exportPath); err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	exported := readFileBytes(t, exportPath)
	if string(exported) != string(original) {
		t.Fatalf("exported content differs from original")
	}
}

func TestListAndShowPreflightFailed(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	root := t.TempDir()
	runID := "run-mcp-preflight-fail-001"

	saveTestTask(t, ctx, db, "task-preflight-fail")
	saveTestRun(t, ctx, db, runID, "task-preflight-fail", "codex")
	proposalPath := writeTestProposalArtifact(t, root, runID)
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalArtifactName, root, readFileString(t, proposalPath))
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalPreflightArtifactName, root, `{
  "proposal_id": "mcp-call-test-001",
  "status": "failed",
  "warnings": [],
  "failures": ["tool not allowlisted"]
}`)

	list, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list.Proposals) != 1 || list.Proposals[0].ProposalStatus != mcpproposalqueue.StatusPreflightFailed {
		t.Fatalf("list = %#v, want preflight_failed", list.Proposals)
	}

	detail, err := mcpproposalqueue.Show(ctx, db, runID)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if detail.ProposalStatus != mcpproposalqueue.StatusPreflightFailed || detail.PreflightStatus != mcpapproval.PreflightStatusFailed {
		t.Fatalf("detail = %#v, want preflight_failed", detail)
	}
	if len(detail.PreflightFails) == 0 {
		t.Fatalf("detail = %#v, want preflight failures", detail)
	}
}

func TestListAndShowWorkerApprovalRefused(t *testing.T) {
	ctx := context.Background()
	db := openTestStore(t)
	root := t.TempDir()
	runID := "run-mcp-approval-refused-001"

	saveTestTask(t, ctx, db, "task-approval-refused")
	saveTestRun(t, ctx, db, runID, "task-approval-refused", "codex")
	writeTestArtifact(t, db, ctx, runID, "mcp-tool-call-approval.json", root, `{"decision":"approved"}`)

	list, err := mcpproposalqueue.List(ctx, db, mcpproposalqueue.ListOptions{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list.Proposals) != 1 || list.Proposals[0].ProposalStatus != mcpproposalqueue.StatusRefused {
		t.Fatalf("list = %#v, want refused", list.Proposals)
	}

	detail, err := mcpproposalqueue.Show(ctx, db, runID)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if detail.ProposalStatus != mcpproposalqueue.StatusRefused || !detail.ApprovalRefused {
		t.Fatalf("detail = %#v, want refused approval", detail)
	}
}

func TestWriteListJSONValid(t *testing.T) {
	result := mcpproposalqueue.ListResult{
		Proposals: []mcpproposalqueue.ListEntry{{
			RunID:          "run-1",
			TaskID:         "task-1",
			Worker:         "codex",
			ProposalStatus: mcpproposalqueue.StatusValid,
		}},
	}
	var stdout strings.Builder
	if err := mcpproposalqueue.WriteListJSON(result, &stdout); err != nil {
		t.Fatalf("WriteListJSON() error = %v", err)
	}
	if !json.Valid([]byte(stdout.String())) {
		t.Fatalf("stdout is not valid JSON: %s", stdout.String())
	}
}

func openTestStore(t *testing.T) store.Store {
	t.Helper()
	db, err := storepkg.OpenSQLite(filepath.Join(t.TempDir(), "deonclaw.db"))
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func saveTestTask(t *testing.T, ctx context.Context, db store.Store, taskID string) {
	t.Helper()
	task := &tasks.Task{
		ID:     taskID,
		Title:  "MCP queue task",
		Domain: "general",
		Worker: "codex",
		Goal:   "queue test",
		Mode:   "read_only",
		Workspace: tasks.WorkspaceSpec{
			Strategy: "local_repo",
			Path:     ".",
		},
		Memory: tasks.MemorySpec{
			Scope: "none",
		},
		AllowedPaths:     []string{"."},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"done"},
	}
	if err := db.SaveTask(ctx, task); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
}

func saveTestRun(t *testing.T, ctx context.Context, db store.Store, runID string, taskID string, worker string) {
	t.Helper()
	now := time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC)
	runRecord := &runs.Run{
		ID:            runID,
		TaskID:        taskID,
		Status:        runs.StatusFailed,
		Worker:        worker,
		WorkspacePath: "workspace",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.SaveRun(ctx, runRecord); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}
}

func writeTestProposalArtifact(t *testing.T, root string, runID string) string {
	t.Helper()
	proposal, err := mcpapproval.NewProposal(mcpapproval.NewProposalOptions{
		ID:          "mcp-call-test-001",
		Server:      "fake-stdio",
		Tool:        "deonclaw.fake.echo",
		Arguments:   []byte(`{"text":"super-secret-argument"}`),
		Reason:      "worker suggestion",
		RequestedBy: "worker",
		PolicyPath:  "configs/examples/mcp-call-policy-fake.yaml",
		ConfigPath:  "configs/examples/mcp-fake.yaml",
		Runtime:     mcpsmoke.RuntimeLocal,
	})
	if err != nil {
		t.Fatalf("NewProposal() error = %v", err)
	}
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	path := filepath.Join(root, "artifacts", runID, mcpproposalqueue.ProposalArtifactName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func writeTestArtifact(t *testing.T, db store.Store, ctx context.Context, runID string, name string, root string, content string) string {
	t.Helper()
	path := filepath.Join(root, "artifacts", runID, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	artifact := &artifacts.Artifact{
		ID:        runID + "-" + name,
		RunID:     runID,
		Path:      path,
		Kind:      artifacts.KindOther,
		SizeBytes: int64(len(content)),
		SHA256:    strings.Repeat("a", 64),
		CreatedAt: time.Date(2026, 6, 17, 12, 0, 0, 0, time.UTC),
	}
	if err := db.SaveArtifact(ctx, artifact); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
	return path
}

func writeValidProposalRun(t *testing.T, db store.Store, ctx context.Context, root string, runID string) {
	t.Helper()
	proposalPath := writeTestProposalArtifact(t, root, runID)
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalArtifactName, root, readFileString(t, proposalPath))
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalLintArtifactName, root, `{
  "proposal_id": "mcp-call-test-001",
  "status": "skipped_policy",
  "violations": [],
  "warnings": ["skipped_policy"]
}`)
}

func writeInvalidProposalRun(t *testing.T, db store.Store, ctx context.Context, root string, runID string) {
	t.Helper()
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalArtifactName, root, `{"id":"bad"}`)
	writeTestArtifact(t, db, ctx, runID, mcpproposalqueue.ProposalLintArtifactName, root, `{
  "proposal_id": "",
  "status": "invalid",
  "violations": ["parse error"],
  "warnings": []
}`)
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	return string(readFileBytes(t, path))
}

func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}
