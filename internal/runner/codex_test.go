package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/git"
	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/mcpapproval"
	"github.com/deon7769/deonclaw/internal/memory"
	"github.com/deon7769/deonclaw/internal/policy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/runtime"
	"github.com/deon7769/deonclaw/internal/runtimeconfig"
	storepkg "github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
	"gopkg.in/yaml.v3"
)

func TestCaptureGitDiffIncludesStagedAndUnstagedChanges(t *testing.T) {
	repo := t.TempDir()
	runGitTestCommand(t, repo, "init")
	runGitTestCommand(t, repo, "config", "user.email", "test@example.com")
	runGitTestCommand(t, repo, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGitTestCommand(t, repo, "add", "tracked.txt")
	runGitTestCommand(t, repo, "commit", "-m", "initial")

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "staged.txt"), []byte("staged\n"), 0o600); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	runGitTestCommand(t, repo, "add", "staged.txt")

	diff, err := CaptureGitDiff(context.Background(), repo)
	if err != nil {
		t.Fatalf("CaptureGitDiff() error = %v", err)
	}
	diffText := string(diff)
	if !strings.Contains(diffText, "diff --git a/tracked.txt b/tracked.txt") {
		t.Fatalf("diff = %q, want unstaged tracked diff", diffText)
	}
	if !strings.Contains(diffText, "diff --git a/staged.txt b/staged.txt") {
		t.Fatalf("diff = %q, want staged file diff", diffText)
	}
}

func TestCodexRunnerRun(t *testing.T) {
	cleanupCalled := false
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	wantWorkspace := filepath.Join(artifactsDir, "run-test-001", "workspace")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-test-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if spec.Workspace == "." {
						t.Fatal("worker received source repo workspace")
					}
					if !strings.HasSuffix(spec.Workspace, filepath.Join("run-test-001", "workspace")) {
						t.Fatalf("worker workspace = %q, want isolated run workspace", spec.Workspace)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
						Events: []workers.WorkerEvent{
							{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
						},
						Stderr: "stderr line\n",
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		WorkspaceManager: fakeWorkspacePreparer{
			cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
				cleanupCalled = true
				if workspace.Path != wantWorkspace {
					t.Fatalf("cleanup workspace path = %q, want %q", workspace.Path, wantWorkspace)
				}
				return nil
			},
		},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "run_id: run-test-001") {
		t.Fatalf("stdout = %q, want run id", output)
	}
	if !strings.Contains(output, "workspace: "+wantWorkspace) {
		t.Fatalf("stdout = %q, want workspace", output)
	}
	if !strings.Contains(output, "command: codex exec --json --sandbox read-only --cd "+wantWorkspace+" -") {
		t.Fatalf("stdout = %q, want command", output)
	}
	if !strings.Contains(output, "events: 1") {
		t.Fatalf("stdout = %q, want event count", output)
	}
	if !strings.Contains(output, "artifacts_dir: "+filepath.Join(artifactsDir, "run-test-001")) {
		t.Fatalf("stdout = %q, want artifacts dir", output)
	}

	runDir := filepath.Join(artifactsDir, "run-test-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.jsonl"), "")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"ok"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "stderr line\n")
	assertFileContent(t, filepath.Join(runDir, "diff.patch"), "")
	assertFileContent(t, filepath.Join(runDir, "changed-files.json"), "[]\n")
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Validation: skipped")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationSkipped, 0)
	assertArtifactManifestContains(t, filepath.Join(runDir, "artifact-manifest.json"), filepath.Join(runDir, "validation.json"), artifacts.KindOther)
	assertArtifactManifestContains(t, filepath.Join(runDir, "artifact-manifest.json"), filepath.Join(runDir, "execution-trace.json"), artifacts.KindOther)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker runtime: local")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: skipped")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation runtime: local")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation commands: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP tool proposal: none")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Artifacts: 10")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Execution trace: execution-trace.json")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: removed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: succeeded")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "run_id", "run-test-001")
	assertTraceString(t, trace, "task_id", "worker-mismatch-001")
	assertTraceString(t, trace, "worker", "codex")
	assertTraceString(t, trace, "worker_runtime", "local")
	assertTraceString(t, trace, "command_display", "codex exec --json --sandbox read-only --cd "+wantWorkspace+" -")
	assertTraceNonEmptyString(t, trace, "prompt_sha256")
	assertTraceString(t, trace, "validation_status", ValidationSkipped)
	assertTraceString(t, trace, "validation_runtime", "local")
	assertTraceString(t, trace, "mcp_tool_proposal_status", "none")
	assertTraceString(t, trace, "policy_status", "ok")
	assertTraceString(t, trace, "cleanup_action", "removed")
	assertTraceTimelineContains(t, trace, requiredExecutionTraceEvents...)
	assertFileNotContains(t, filepath.Join(runDir, "execution-trace.json"), "Do not execute")
	if !cleanupCalled {
		t.Fatal("workspace cleanup was not called for succeeded run")
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-test-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
	if gotRun.WorkspacePath != wantWorkspace {
		t.Fatalf("run workspace path = %q, want %q", gotRun.WorkspacePath, wantWorkspace)
	}
	if gotRun.TaskID != "worker-mismatch-001" {
		t.Fatalf("run task id = %q, want task id", gotRun.TaskID)
	}

	gotEvents, err := db.EventsByRun(context.Background(), "run-test-001")
	if err != nil {
		t.Fatalf("EventsByRun() error = %v", err)
	}
	if len(gotEvents) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(gotEvents))
	}
	if string(gotEvents[0].Type) != "message" {
		t.Fatalf("event type = %q, want message", gotEvents[0].Type)
	}

	gotArtifacts, err := db.ArtifactsByRun(context.Background(), "run-test-001")
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	wantArtifactPaths := map[string]bool{
		filepath.Join(runDir, "stdout.jsonl"):           false,
		filepath.Join(runDir, "events.jsonl"):           false,
		filepath.Join(runDir, "stderr.log"):             false,
		filepath.Join(runDir, "diff.patch"):             false,
		filepath.Join(runDir, "changed-files.json"):     false,
		filepath.Join(runDir, "validation.log"):         false,
		filepath.Join(runDir, "validation.json"):        false,
		filepath.Join(runDir, "execution-trace.json"):   false,
		filepath.Join(runDir, "summary.md"):             false,
		filepath.Join(runDir, "artifact-manifest.json"): false,
	}
	for _, artifact := range gotArtifacts {
		if _, ok := wantArtifactPaths[artifact.Path]; !ok {
			t.Fatalf("unexpected artifact path %q", artifact.Path)
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("artifact %q sha256 = %q, want 64 hex chars", artifact.Path, artifact.SHA256)
		}
		if artifact.Path == filepath.Join(runDir, "validation.log") && artifact.SizeBytes == 0 {
			t.Fatalf("validation.log size_bytes = 0, want persisted non-zero size")
		}
		wantArtifactPaths[artifact.Path] = true
	}
	for path, seen := range wantArtifactPaths {
		if !seen {
			t.Fatalf("artifact path %q was not persisted", path)
		}
	}
}

func TestCodexRunnerRunWithoutDomainsKeepsPromptUnset(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-no-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if spec.Prompt != "" {
						t.Fatalf("prompt = %q, want empty prompt when --domains is not configured", spec.Prompt)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	tempDir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    filepath.Join(tempDir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tempDir, "artifacts", "run-no-context-001", "context-pack.md")); !os.IsNotExist(err) {
		t.Fatalf("context-pack.md exists without --domains: %v", err)
	}
}

func TestCodexRunnerRunWithGeneralContextPack(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	bridgePath := filepath.Join(tempDir, "escalasoft-bridge.md")
	if err := os.WriteFile(bridgePath, []byte("Escalasoft isolated bridge content\n"), 0o600); err != nil {
		t.Fatalf("write bridge file: %v", err)
	}
	domainsPath := writeDomainsConfigWithBridge(t, bridgePath)

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-general-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if !strings.Contains(spec.Prompt, "# Task Goal\nDo not execute\n\n# Context Pack\n") {
						t.Fatalf("prompt = %q, want task goal and context pack sections", spec.Prompt)
					}
					if strings.Contains(spec.Prompt, "Escalasoft isolated bridge content") {
						t.Fatalf("prompt leaked isolated bridge content for general task: %q", spec.Prompt)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
		DomainsPath:  domainsPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-general-context-001")
	contextPath := filepath.Join(runDir, "context-pack.md")
	assertFileContains(t, contextPath, "domain: general")
	assertFileNotContains(t, contextPath, "Escalasoft isolated bridge content")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Artifacts: 11")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceNonEmptyString(t, trace, "context_pack_sha256")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	assertPersistedArtifactWithMetadata(t, db, "run-general-context-001", contextPath)
}

func TestCodexRunnerRunWithMCPDiscoveryContextAttachment(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	discoveryPath := writeRunnerMCPAttachment(t, tempDir, "mcp-tools-list.json", `{
  "tool_count": 1,
  "tool_names": ["fs.read"],
  "tools": [{"name":"fs.read","description":"super-secret-value"}]
}`)
	taskPath := writeTaskFileWithMCPContext(t, "codex", []tasks.MCPContextAttachment{{
		Name: "filesystem-discovery",
		Kind: "discovery",
		Path: discoveryPath,
	}})

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-mcp-discovery-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if len(spec.Task.MCPContext.Attachments) != 0 {
						t.Fatalf("worker received MCP context attachment paths: %#v", spec.Task.MCPContext.Attachments)
					}
					if !strings.Contains(spec.Prompt, "# MCP Context Attachments") ||
						!strings.Contains(spec.Prompt, "name: filesystem-discovery") ||
						!strings.Contains(spec.Prompt, "tool_count: 1") ||
						!strings.Contains(spec.Prompt, "fs.read") {
						t.Fatalf("prompt = %q, want passive discovery MCP summary", spec.Prompt)
					}
					if strings.Contains(spec.Prompt, "super-secret-value") {
						t.Fatalf("prompt leaked raw MCP attachment content: %q", spec.Prompt)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-mcp-discovery-context-001")
	mcpContextPath := filepath.Join(runDir, "mcp-context.md")
	assertFileContains(t, mcpContextPath, "# MCP Context Attachments")
	assertFileContains(t, mcpContextPath, "tool_count: 1")
	assertFileNotContains(t, mcpContextPath, "super-secret-value")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP context attachments: 1")
	assertArtifactManifestContains(t, filepath.Join(runDir, "artifact-manifest.json"), mcpContextPath, artifacts.KindOther)
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceNonEmptyString(t, trace, "mcp_context_sha256")
}

func TestCodexRunnerRunWithMCPCallContextAttachment(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	bundlePath := writeRunnerMCPAttachment(t, tempDir, "mcp-call-execution-bundle.json", runnerCallBundleJSON(`"preflight_status":"passed",
  "status":"succeeded",
  "tool_calls":1,
  "response_truncated":true,
  "response_preview":"super-secret-value"`))
	taskPath := writeTaskFileWithMCPContext(t, "codex", []tasks.MCPContextAttachment{{
		Name: "filesystem-call",
		Kind: "call",
		Path: bundlePath,
	}})

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-mcp-call-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if len(spec.Task.MCPContext.Attachments) != 0 {
						t.Fatalf("worker received MCP context attachment paths: %#v", spec.Task.MCPContext.Attachments)
					}
					if !strings.Contains(spec.Prompt, "server: filesystem-readonly") ||
						!strings.Contains(spec.Prompt, "tool: fs.read") ||
						!strings.Contains(spec.Prompt, "tool_calls: 1") ||
						!strings.Contains(spec.Prompt, "response_truncated: true") ||
						!strings.Contains(spec.Prompt, "mcp-call-response.json") {
						t.Fatalf("prompt = %q, want passive call bundle summary", spec.Prompt)
					}
					if strings.Contains(spec.Prompt, "super-secret-value") || strings.Contains(spec.Prompt, "response_preview") {
						t.Fatalf("prompt leaked raw MCP call payload: %q", spec.Prompt)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-mcp-call-context-001")
	mcpContextPath := filepath.Join(runDir, "mcp-context.md")
	assertFileContains(t, mcpContextPath, "status: succeeded")
	assertFileContains(t, mcpContextPath, "preflight_status: passed")
	assertFileContains(t, mcpContextPath, "tool_calls: 1")
	assertFileNotContains(t, mcpContextPath, "super-secret-value")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP context attachments: 1")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceNonEmptyString(t, trace, "mcp_context_sha256")
}

func TestCodexRunnerRunRejectsInvalidMCPContextBeforeWorker(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		content string
		wantErr string
	}{
		{
			name:    "missing path",
			path:    filepath.Join(t.TempDir(), "missing.json"),
			wantErr: "no such file",
		},
		{
			name: "failed bundle",
			content: runnerCallBundleJSON(`"preflight_status":"passed",
  "status":"failed",
  "tool_calls":1`),
			wantErr: "status",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			attachmentPath := tt.path
			if tt.content != "" {
				attachmentPath = writeRunnerMCPAttachment(t, tempDir, "bundle.json", tt.content)
			}
			taskPath := writeTaskFileWithMCPContext(t, "codex", []tasks.MCPContextAttachment{{
				Name: "filesystem-call",
				Kind: "call",
				Path: attachmentPath,
			}})
			workerCalled := false
			runner := testCodexRunner(t, testCodexRunnerOptions{
				RunID: "run-invalid-mcp-context-001",
				WorkerFactory: func() workers.Worker {
					return fakeWorker{
						runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
							workerCalled = true
							return &workers.RunResult{}, nil
						},
					}
				},
			})
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runner.Run(context.Background(), CodexRunOptions{
				TaskPath:     taskPath,
				StorePath:    filepath.Join(tempDir, "deonclaw.db"),
				ArtifactsDir: filepath.Join(tempDir, "artifacts"),
			}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("Run() exit code = %d, want 1", code)
			}
			if workerCalled {
				t.Fatal("worker was called for invalid MCP context")
			}
			if !strings.Contains(stderr.String(), "mcp context failed") || !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr = %q, want MCP context failure containing %q", stderr.String(), tt.wantErr)
			}
		})
	}
}

func TestCodexRunnerRunWithLanceDBRetrievalContextAttachment(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	resultPath, reportPath, policyPath := writeRunnerLanceDBRetrievalFixtures(t, runnerValidSearchEnvelope())
	taskPath := writeTaskFileWithRetrievalContext(t, "codex", []tasks.RetrievalContextAttachment{{
		Kind:       retrievalcontext.KindLanceDBSearchReport,
		Path:       resultPath,
		ReportPath: reportPath,
		Policy:     policyPath,
		MaxResults: 5,
	}})

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-lancedb-retrieval-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if len(spec.Task.RetrievalContext.Attachments) != 0 {
						t.Fatalf("worker received retrieval context config: %#v", spec.Task.RetrievalContext.Attachments)
					}
					if len(spec.Task.MCPContext.Attachments) != 0 {
						t.Fatalf("worker received MCP context config: %#v", spec.Task.MCPContext.Attachments)
					}
					if !strings.Contains(spec.Prompt, "Retrieved context metadata only") ||
						!strings.Contains(spec.Prompt, "chunk-a") ||
						!strings.Contains(spec.Prompt, "notes/a.md") {
						t.Fatalf("prompt = %q, want passive retrieval metadata", spec.Prompt)
					}
					for _, forbidden := range []string{"SECRET-CHUNK-TEXT", `"vector":`, "chunk_text:", `"text":`, `"content":`, `"embedding":`} {
						if strings.Contains(spec.Prompt, forbidden) {
							t.Fatalf("prompt leaked forbidden payload %q", forbidden)
						}
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-lancedb-retrieval-context-001")
	retrievalMD := filepath.Join(runDir, "retrieval-context.md")
	retrievalJSON := filepath.Join(runDir, "retrieval-context.json")
	assertFileContains(t, retrievalMD, "Retrieved context metadata only")
	assertFileContains(t, retrievalMD, "chunk-a")
	assertFileNotContains(t, retrievalMD, "SECRET-CHUNK-TEXT")
	assertFileContains(t, retrievalJSON, `"chunk_id": "chunk-a"`)
	assertFileNotContains(t, retrievalJSON, `"vector":`)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Retrieval context hits: 1")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	if trace["retrieval_context_attached"] != true {
		t.Fatalf("trace retrieval_context_attached = %v, want true", trace["retrieval_context_attached"])
	}
	if trace["retrieval_context_count"] != float64(1) {
		t.Fatalf("trace retrieval_context_count = %v, want 1", trace["retrieval_context_count"])
	}
	if trace["retrieval_context_status"] != "ok" {
		t.Fatalf("trace retrieval_context_status = %v, want ok", trace["retrieval_context_status"])
	}
	assertTraceNonEmptyString(t, trace, "retrieval_context_sha256")
}

func TestCodexRunnerRunWithMaterializedInjectionDeclarationDoesNotInjectPreviewText(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	resultPath, reportPath, policyPath := writeRunnerLanceDBRetrievalFixturesInDir(t, dir, runnerValidSearchEnvelope())
	writeRunnerGovernanceBundleJSON(t, "retrieval-context-injection-governance-bundle.json")
	if err := os.WriteFile("retrieval-context-prompt-preview.md", []byte("## preview\n- text_excerpt: alpha text\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	taskPath := filepath.Join(dir, "task.yaml")
	taskContent := `id: materialized-injection-task-001
title: "Materialized injection declaration"
domain: general
worker: codex
goal: "Declare future materialized injection"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
retrieval_context:
  materialized_injection:
    enabled: false
    governance_bundle: retrieval-context-injection-governance-bundle.json
    prompt_preview: retrieval-context-prompt-preview.md
    require_confirm_flag: true
    max_total_chars: 6000
  attachments:
    - kind: lancedb_search_report
      path: ` + filepath.ToSlash(filepath.Base(resultPath)) + `
      report_path: ` + filepath.ToSlash(filepath.Base(reportPath)) + `
      policy: ` + filepath.ToSlash(filepath.Base(policyPath)) + `
      max_results: 5
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - schema declared only
`
	if err := os.WriteFile(taskPath, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	storePath := filepath.Join(dir, "deonclaw.db")
	artifactsDir := filepath.Join(dir, "artifacts")
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-materialized-injection-declaration-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if spec.Task.RetrievalContext.MaterializedInjection != nil {
						t.Fatalf("worker received materialized_injection config: %#v", spec.Task.RetrievalContext.MaterializedInjection)
					}
					if strings.Contains(spec.Prompt, "alpha text") || strings.Contains(spec.Prompt, "text_excerpt:") {
						t.Fatalf("prompt leaked materialized preview text: %q", spec.Prompt)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout, stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	trace := readExecutionTrace(t, filepath.Join(artifactsDir, "run-materialized-injection-declaration-001", "execution-trace.json"))
	traceJSON, err := json.Marshal(trace)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(traceJSON), "text_excerpt") || strings.Contains(string(traceJSON), "alpha text") {
		t.Fatal("execution trace must not contain materialized preview text")
	}
}

func TestCodexRunnerRunRejectsMaterializedInjectionEnabledTrue(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	writeRunnerGovernanceBundleJSON(t, "retrieval-context-injection-governance-bundle.json")
	if err := os.WriteFile("retrieval-context-prompt-preview.md", []byte("preview\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	taskPath := filepath.Join(dir, "task.yaml")
	taskContent := `id: materialized-injection-task-001
title: "Materialized injection declaration"
domain: general
worker: codex
goal: "Declare future materialized injection"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
retrieval_context:
  materialized_injection:
    enabled: true
    governance_bundle: retrieval-context-injection-governance-bundle.json
    prompt_preview: retrieval-context-prompt-preview.md
    require_confirm_flag: true
    max_total_chars: 6000
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - schema declared only
`
	if err := os.WriteFile(taskPath, []byte(taskContent), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-materialized-injection-enabled-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(context.Context, workers.RunSpec) (*workers.RunResult, error) {
					t.Fatal("worker must not run when materialized injection is enabled")
					return nil, nil
				},
			}
		},
	})

	var stdout, stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    filepath.Join(dir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(dir, "artifacts"),
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatal("Run() expected validation failure")
	}
	if !strings.Contains(stderr.String(), tasks.MaterializedInjectionNotSupportedYet) {
		t.Fatalf("stderr = %q, want not supported yet", stderr.String())
	}
}

func TestCodexRunnerRunRejectsInvalidRetrievalContextBeforeWorker(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*runnerSearchEnvelopeFixture)
		wantErr string
	}{
		{
			name: "retrieval_performed false",
			mutate: func(envelope *runnerSearchEnvelopeFixture) {
				envelope.Summary.RetrievalPerformed = false
			},
			wantErr: "search-report",
		},
		{
			name: "forbidden text field",
			mutate: func(envelope *runnerSearchEnvelopeFixture) {
				raw := runnerMarshalSearchHit(lancedbpolicy.SearchHit{
					Rank: 1, ChunkID: "chunk-a", VectorID: "vec-a", Distance: 0.1,
				})
				var object map[string]any
				_ = json.Unmarshal(raw, &object)
				object["content"] = "SECRET-CHUNK-TEXT"
				raw, _ = json.Marshal(object)
				envelope.Search.Results = []json.RawMessage{raw}
			},
			wantErr: "search-report",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			envelope := runnerValidSearchEnvelope()
			resultPath, reportPath, policyPath := writeRunnerLanceDBRetrievalFixtures(t, envelope)
			if tt.mutate != nil {
				tt.mutate(&envelope)
				resultBytes, err := json.MarshalIndent(envelope, "", "  ")
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
				resultBytes = append(resultBytes, '\n')
				if err := os.WriteFile(resultPath, resultBytes, 0o644); err != nil {
					t.Fatalf("WriteFile() error = %v", err)
				}
			}
			taskPath := writeTaskFileWithRetrievalContext(t, "codex", []tasks.RetrievalContextAttachment{{
				Kind:       retrievalcontext.KindLanceDBSearchReport,
				Path:       resultPath,
				ReportPath: reportPath,
				Policy:     policyPath,
				MaxResults: 5,
			}})
			workerCalled := false
			runner := testCodexRunner(t, testCodexRunnerOptions{
				RunID: "run-invalid-retrieval-context-001",
				WorkerFactory: func() workers.Worker {
					return fakeWorker{
						runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
							workerCalled = true
							return &workers.RunResult{}, nil
						},
					}
				},
			})
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runner.Run(context.Background(), CodexRunOptions{
				TaskPath:     taskPath,
				StorePath:    filepath.Join(tempDir, "deonclaw.db"),
				ArtifactsDir: filepath.Join(tempDir, "artifacts"),
			}, &stdout, &stderr)
			if code != 1 {
				t.Fatalf("Run() exit code = %d, want 1", code)
			}
			if workerCalled {
				t.Fatal("worker was called for invalid retrieval context")
			}
			if !strings.Contains(stderr.String(), "retrieval context failed") || !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr = %q, want retrieval context failure containing %q", stderr.String(), tt.wantErr)
			}
		})
	}
}

func TestValidateRejectsInvalidRetrievalContextMaxResults(t *testing.T) {
	task := &tasks.Task{
		ID: "retrieval-task", Title: "t", Domain: "general", Worker: "codex", Goal: "g", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."},
		Memory:    tasks.MemorySpec{Scope: "none"},
		RetrievalContext: tasks.RetrievalContextSpec{Attachments: []tasks.RetrievalContextAttachment{{
			Kind: "lancedb_search_report", Path: "a.json", ReportPath: "b.json", Policy: "p.yaml", MaxResults: 0,
		}}},
		ForbiddenPaths:   []string{"secrets/**"},
		ExpectedOutputs:  []string{"artifacts/summary.md"},
		DefinitionOfDone: []string{"done"},
	}
	if err := tasks.Validate(task); err == nil || !strings.Contains(err.Error(), "max_results") {
		t.Fatalf("Validate() error = %v, want max_results validation failure", err)
	}
}

func TestCodexRunnerRunPreservesValidMCPToolProposalWithoutPolicy(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	secretArgument := "super-secret-value"
	proposalJSON := mcpToolProposalJSON(t, "fake-stdio", "deonclaw.fake.echo", `{"text":"`+secretArgument+`"}`, "configs/examples/mcp-call-policy-fake.yaml", "", "")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-mcp-tool-proposal-valid-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if spec.Task.MCPProposalPolicy.Config != "" || spec.Task.MCPProposalPolicy.Policy != "" {
						t.Fatalf("worker received MCP proposal policy paths: %#v", spec.Task.MCPProposalPolicy)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
						Artifacts: []artifacts.Artifact{
							{Path: "artifacts/" + mcpToolProposalArtifactName, Kind: artifacts.KindOther, Content: proposalJSON},
						},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-mcp-tool-proposal-valid-001")
	assertFileContent(t, filepath.Join(runDir, mcpToolProposalArtifactName), string(proposalJSON))
	assertMCPToolProposalLintStatus(t, filepath.Join(runDir, mcpToolProposalLintArtifactName), "skipped_policy", 0, 1)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP tool proposal: valid")
	assertFileNotContains(t, filepath.Join(runDir, "summary.md"), secretArgument)
	assertFileNotContains(t, filepath.Join(runDir, "execution-trace.json"), secretArgument)
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "mcp_tool_proposal_status", "valid")
	assertTraceNonEmptyString(t, trace, "mcp_tool_proposal_sha256")
	if _, err := os.Stat(filepath.Join(runDir, mcpToolProposalPreflightArtifactName)); !os.IsNotExist(err) {
		t.Fatalf("preflight artifact exists without mcp_proposal_policy: %v", err)
	}
}

func TestCodexRunnerRunFailsInvalidMCPToolProposal(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-mcp-tool-proposal-invalid-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Artifacts: []artifacts.Artifact{
						{Path: "artifacts/" + mcpToolProposalArtifactName, Kind: artifacts.KindOther, Content: []byte(`{"id":"bad"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "MCP tool proposal invalid") {
		t.Fatalf("stderr = %q, want MCP proposal failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-mcp-tool-proposal-invalid-001")
	assertMCPToolProposalLintStatus(t, filepath.Join(runDir, mcpToolProposalLintArtifactName), "invalid", 1, 0)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP tool proposal: invalid")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "mcp_tool_proposal_status", "invalid")
}

func TestCodexRunnerRunMCPToolProposalPreflightPassed(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	configPath, policyPath, runtimeConfigPath := writeRunnerMCPProposalPolicyFiles(t, tempDir, []string{"read"}, []string{"deonclaw.fake.echo"})
	proposalJSON := mcpToolProposalJSON(t, "fake-stdio", "deonclaw.fake.echo", `{"text":"hello"}`, policyPath, configPath, runtimeConfigPath)
	taskPath := writeTaskFileWithMCPProposalPolicy(t, "codex", configPath, policyPath, runtimeConfigPath, true)

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-mcp-tool-proposal-preflight-pass-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if spec.Task.MCPProposalPolicy.Config != "" || spec.Task.MCPProposalPolicy.Policy != "" || spec.Task.MCPProposalPolicy.RuntimeConfig != "" {
						t.Fatalf("worker received MCP proposal policy paths: %#v", spec.Task.MCPProposalPolicy)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
						Artifacts: []artifacts.Artifact{
							{Path: "artifacts/" + mcpToolProposalArtifactName, Kind: artifacts.KindOther, Content: proposalJSON},
						},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-mcp-tool-proposal-preflight-pass-001")
	assertMCPToolProposalLintStatus(t, filepath.Join(runDir, mcpToolProposalLintArtifactName), "passed", 0, 0)
	assertMCPToolPreflightStatus(t, filepath.Join(runDir, mcpToolProposalPreflightArtifactName), "passed", 0)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP tool proposal: preflight_passed")
	assertArtifactManifestContains(t, filepath.Join(runDir, "artifact-manifest.json"), filepath.Join(runDir, mcpToolProposalPreflightArtifactName), artifacts.KindOther)
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "mcp_tool_proposal_status", "preflight_passed")
	assertTraceNonEmptyString(t, trace, "mcp_tool_proposal_sha256")
}

func TestCodexRunnerRunMCPToolProposalPreflightFailureModes(t *testing.T) {
	tests := []struct {
		name             string
		requirePreflight bool
		capabilities     []string
		allowedTools     []string
		wantCode         int
	}{
		{
			name:             "required tool not allowlisted fails run",
			requirePreflight: true,
			capabilities:     []string{"read"},
			allowedTools:     []string{"other.tool"},
			wantCode:         1,
		},
		{
			name:             "optional tool not allowlisted records artifact",
			requirePreflight: false,
			capabilities:     []string{"read"},
			allowedTools:     []string{"other.tool"},
			wantCode:         0,
		},
		{
			name:             "write capability fails run",
			requirePreflight: true,
			capabilities:     []string{"read", "write"},
			allowedTools:     []string{"deonclaw.fake.echo"},
			wantCode:         1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			storePath := filepath.Join(tempDir, "deonclaw.db")
			artifactsDir := filepath.Join(tempDir, "artifacts")
			configPath, policyPath, runtimeConfigPath := writeRunnerMCPProposalPolicyFiles(t, tempDir, tt.capabilities, tt.allowedTools)
			proposalJSON := mcpToolProposalJSON(t, "fake-stdio", "deonclaw.fake.echo", `{"text":"hello"}`, policyPath, configPath, runtimeConfigPath)
			taskPath := writeTaskFileWithMCPProposalPolicy(t, "codex", configPath, policyPath, runtimeConfigPath, tt.requirePreflight)

			runner := testCodexRunner(t, testCodexRunnerOptions{
				RunID:    "run-mcp-tool-proposal-preflight-fail-001",
				Baseline: &git.Snapshot{},
				PostRun:  &git.Snapshot{},
				WorkerFactory: func() workers.Worker {
					return fakeWorker{
						runResult: &workers.RunResult{
							Workspace: ".",
							Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
							Artifacts: []artifacts.Artifact{
								{Path: "artifacts/" + mcpToolProposalArtifactName, Kind: artifacts.KindOther, Content: proposalJSON},
							},
						},
					}
				},
			})

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runner.Run(context.Background(), CodexRunOptions{
				TaskPath:     taskPath,
				StorePath:    storePath,
				ArtifactsDir: artifactsDir,
			}, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("Run() exit code = %d, want %d, stderr=%q", code, tt.wantCode, stderr.String())
			}

			runDir := filepath.Join(artifactsDir, "run-mcp-tool-proposal-preflight-fail-001")
			assertMCPToolPreflightStatus(t, filepath.Join(runDir, mcpToolProposalPreflightArtifactName), "failed", 1)
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP tool proposal: preflight_failed")
			trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
			assertTraceString(t, trace, "mcp_tool_proposal_status", "preflight_failed")
			if tt.requirePreflight && !strings.Contains(stderr.String(), "MCP proposal preflight failed") {
				t.Fatalf("stderr = %q, want preflight failure", stderr.String())
			}
		})
	}
}

func TestCodexRunnerRunFailsWorkerMCPApprovalArtifact(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-mcp-tool-approval-refused-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Artifacts: []artifacts.Artifact{
						{Path: "artifacts/mcp-tool-call-approval.json", Kind: artifacts.KindOther, Content: []byte(`{"decision":"approved"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "MCP proposal approval artifact refused") {
		t.Fatalf("stderr = %q, want refused approval artifact", stderr.String())
	}
	runDir := filepath.Join(artifactsDir, "run-mcp-tool-approval-refused-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "MCP tool proposal: invalid")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "mcp_tool_proposal_status", "invalid")
}

func TestCodexRunnerRunWithEscalasoftContextPack(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	bridgePath := filepath.Join(tempDir, "escalasoft-bridge.md")
	bridgeContent := "Escalasoft explicit bridge content\n"
	if err := os.WriteFile(bridgePath, []byte(bridgeContent), 0o600); err != nil {
		t.Fatalf("write bridge file: %v", err)
	}
	domainsPath := writeDomainsConfigWithBridge(t, bridgePath)

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-escalasoft-context-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runFunc: func(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
					if !strings.Contains(spec.Prompt, "# Task Goal\nReview Escalasoft task\n\n# Context Pack\n") {
						t.Fatalf("prompt = %q, want task goal and context pack sections", spec.Prompt)
					}
					if !strings.Contains(spec.Prompt, bridgeContent) {
						t.Fatalf("prompt = %q, want explicit bridge content", spec.Prompt)
					}
					return &workers.RunResult{
						Workspace: spec.Workspace,
						Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", spec.Workspace, "-"},
					}, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithDomainAndGoal(t, "codex", "escalasoft", "Review Escalasoft task"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
		DomainsPath:  domainsPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-escalasoft-context-001")
	contextPath := filepath.Join(runDir, "context-pack.md")
	assertFileContains(t, contextPath, "domain: escalasoft")
	assertFileContains(t, contextPath, bridgeContent)

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	assertPersistedArtifactWithMetadata(t, db, "run-escalasoft-context-001", contextPath)
}

func TestCodexRunnerRunContextPackWarningsAppearInSummary(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	missingBridgePath := filepath.Join(tempDir, "missing-bridge.md")
	domainsPath := writeDomainsConfigWithBridge(t, missingBridgePath)

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID:    "run-context-warning-001",
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithDomainAndGoal(t, "codex", "escalasoft", "Review Escalasoft task"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
		DomainsPath:  domainsPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-context-warning-001")
	assertFileContains(t, filepath.Join(runDir, "context-pack.md"), "bridge file not found")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Context pack warnings: 1")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Context pack warning: bridge file not found")
}

func TestCodexRunnerRunPreservesMemoryProposalWithoutPolicy(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	targetPath := filepath.Join(tempDir, "should-not-be-written.md")
	proposalJSON := memoryProposalJSON(t, "mem-run-not-checked", "general", targetPath, memory.OperationAppend)
	proposalMarkdown := []byte("# Memory Proposal mem-run-not-checked\n")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-memory-proposal-not-checked-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Artifacts: []artifacts.Artifact{
						{Path: "artifacts/memory-proposal.json", Kind: artifacts.KindOther, Content: proposalJSON},
						{Path: "artifacts/memory-proposal.md", Kind: artifacts.KindOther, Content: proposalMarkdown},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-memory-proposal-not-checked-001")
	assertFileContent(t, filepath.Join(runDir, "memory-proposal.json"), string(proposalJSON))
	assertFileContent(t, filepath.Join(runDir, "memory-proposal.md"), string(proposalMarkdown))
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal: not_checked")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal violations: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal warnings: 0")
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("proposal target exists, proposal was applied or created unexpectedly: %v", err)
	}

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	assertPersistedArtifactWithMetadata(t, db, "run-memory-proposal-not-checked-001", filepath.Join(runDir, "memory-proposal.json"))
}

func TestCodexRunnerRunMemoryProposalLintOK(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	proposalJSON := memoryProposalJSON(t, "mem-run-ok", "escalasoft", "/domains/escalasoft_brain/cases/case-001.md", memory.OperationUpdate)

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-memory-proposal-ok-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Artifacts: []artifacts.Artifact{
						{Path: "artifacts/memory-proposal.json", Kind: artifacts.KindOther, Content: proposalJSON},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:         writeTaskFile(t, "codex"),
		StorePath:        storePath,
		ArtifactsDir:     artifactsDir,
		MemoryPolicyPath: filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-memory-proposal-ok-001")
	assertMemoryProposalLintStatus(t, filepath.Join(runDir, "memory-proposal-lint.json"), "ok", 0, 0)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal: ok")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal id: mem-run-ok")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal violations: 0")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal warnings: 0")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	assertPersistedArtifactWithMetadata(t, db, "run-memory-proposal-ok-001", filepath.Join(runDir, "memory-proposal-lint.json"))
}

func TestCodexRunnerRunMemoryProposalLintFailedDoesNotFailRun(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	proposalJSON := memoryProposalJSON(t, "mem-run-failed", "escalasoft", "/vault/mysecondbrain/MEMORY.md", memory.OperationUpdate)

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-memory-proposal-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Artifacts: []artifacts.Artifact{
						{Path: "artifacts/memory-proposal.json", Kind: artifacts.KindOther, Content: proposalJSON},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:         writeTaskFile(t, "codex"),
		StorePath:        storePath,
		ArtifactsDir:     artifactsDir,
		MemoryPolicyPath: filepath.Join("..", "..", "configs", "examples", "memory-policy.yaml"),
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-memory-proposal-failed-001")
	assertMemoryProposalLintStatus(t, filepath.Join(runDir, "memory-proposal-lint.json"), "failed", 1, 0)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal id: mem-run-failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal violations:")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), `Memory proposal violation: target_path "/vault/mysecondbrain/MEMORY.md"`)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Memory proposal warnings: 0")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()
	gotRun, err := db.Run(context.Background(), "run-memory-proposal-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
}

func TestCodexRunnerRunValidationCommandSuccess(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	wantWorkspace := filepath.Join(artifactsDir, "run-validation-success-001", "workspace")
	validationCalled := false

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-success-001",
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			if workspace != wantWorkspace {
				t.Fatalf("validation workspace = %q, want %q", workspace, wantWorkspace)
			}
			if len(commands) != 1 || commands[0].Name != "go-test" {
				t.Fatalf("validation commands = %#v, want go-test command", commands)
			}
			return validationPassed(commands, "go-test")
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !validationCalled {
		t.Fatal("validation runner was not called")
	}

	runDir := filepath.Join(artifactsDir, "run-validation-success-001")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationPassed, 1)
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Validation: passed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: passed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-validation-success-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
	assertPersistedArtifact(t, db, "run-validation-success-001", filepath.Join(runDir, "validation.json"))
	assertPersistedArtifact(t, db, "run-validation-success-001", filepath.Join(runDir, "artifact-manifest.json"))
}

func TestCodexRunnerRunDockerValidationAuditFields(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	installRunnerFakeDocker(t, `#!/bin/sh
printf 'docker ok\n'
exit 0
`)
	runtimeConfigContent := []byte(`runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    mounts:
      - source: .
        target: /workspace
        mode: rw
`)
	runtimeConfigPath := filepath.Join(tempDir, "runtime.yaml")
	if err := os.WriteFile(runtimeConfigPath, runtimeConfigContent, 0o600); err != nil {
		t.Fatalf("WriteFile(runtime config) error = %v", err)
	}

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID:    "run-docker-validation-audit-001",
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:          writeTaskFileWithValidation(t, "codex"),
		StorePath:         storePath,
		ArtifactsDir:      artifactsDir,
		ValidationRuntime: tasks.ValidationRuntimeDocker,
		RuntimeConfig: &runtimeconfig.Config{
			Runtime: runtimeconfig.Runtime{
				Mode: runtimeconfig.ModeDocker,
				Docker: runtimeconfig.DockerConfig{
					Image:        "deonclaw-runner:latest",
					Workdir:      "/workspace",
					Network:      "none",
					ReadOnlyRoot: true,
					Mounts: []runtimeconfig.MountSpec{
						{Source: ".", Target: "/workspace", Mode: "rw"},
					},
				},
			},
		},
		RuntimeConfigPath: runtimeConfigPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-docker-validation-audit-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: passed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation runtime: docker")
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Runtime: docker")
	assertFileContains(t, filepath.Join(runDir, "validation.json"), `"runtime": "docker"`)
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "validation_runtime", "docker")
	assertTraceString(t, trace, "runtime_config_sha256", sha256Hex(runtimeConfigContent))
	assertFileNotContains(t, filepath.Join(runDir, "execution-trace.json"), "deonclaw-runner:latest")
	assertFileNotContains(t, filepath.Join(runDir, "execution-trace.json"), "/workspace")
}

func TestCodexRunnerRunDockerWorkerRuntimeWithFakeDocker(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	stdinPath := filepath.Join(tempDir, "docker.stdin")
	t.Setenv("DEONCLAW_FAKE_DOCKER_STDIN", stdinPath)
	argsPath := installRunnerFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
cat > "$DEONCLAW_FAKE_DOCKER_STDIN"
printf '{"type":"message","text":"docker worker ok"}\n'
printf 'docker worker stderr\n' >&2
exit 0
`)
	runtimeConfigContent := []byte(`runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    mounts:
      - source: .
        target: /workspace
        mode: rw
`)
	runtimeConfigPath := filepath.Join(tempDir, "runtime.yaml")
	if err := os.WriteFile(runtimeConfigPath, runtimeConfigContent, 0o600); err != nil {
		t.Fatalf("WriteFile(runtime config) error = %v", err)
	}

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-docker-worker-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				dryRunEvent: &workers.WorkerEvent{
					Type:      workers.EventDryRunPlanned,
					Worker:    "codex",
					Command:   []string{"fake-worker", "--json", "-"},
					Workspace: ".",
					Sandbox:   "read-only",
				},
				runFunc: func(context.Context, workers.RunSpec) (*workers.RunResult, error) {
					t.Fatal("local fake worker Run must not be called for docker worker runtime")
					return nil, nil
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:          writeTaskFile(t, "codex"),
		StorePath:         storePath,
		ArtifactsDir:      artifactsDir,
		WorkerRuntime:     WorkerRuntimeDocker,
		RuntimeConfig:     validDockerWorkerRuntimeConfig(),
		RuntimeConfigPath: runtimeConfigPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	for _, want := range []string{"run", "--rm", "--network", "none", "--read-only", "deonclaw-runner:latest", "fake-worker", "--json", "-"} {
		if !containsString(args, want) {
			t.Fatalf("docker args = %#v, want %q", args, want)
		}
	}
	if strings.Contains(strings.Join(args, " "), "sh -c") {
		t.Fatalf("docker args = %#v, must not use implicit shell", args)
	}
	assertFileContent(t, stdinPath, "Do not execute")

	runDir := filepath.Join(artifactsDir, "run-docker-worker-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.jsonl"), `{"type":"message","text":"docker worker ok"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"docker worker ok"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "docker worker stderr\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker runtime: docker")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "worker_runtime", "docker")
	assertTraceString(t, trace, "runtime_config_sha256", sha256Hex(runtimeConfigContent))
	workspaceMount := filepath.Join(artifactsDir, "run-docker-worker-001", "workspace") + ":/workspace:rw"
	assertTraceString(t, trace, "command_display", strings.Join([]string{
		"docker", "run", "--rm", "--network", "none", "--read-only", "-w", "/workspace",
		"-v", workspaceMount, "--label", "deonclaw.workspace=" + filepath.Join(artifactsDir, "run-docker-worker-001", "workspace"),
		"deonclaw-runner:latest", "fake-worker", "--json", "-",
	}, " "))
}

func TestCodexRunnerRunDockerWorkerRuntimeArgPlaceholderMasksPrompt(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	rawPrompt := "RAW_PROMPT_SECRET_2041"
	envSecret := "ENV_SECRET_2041"
	t.Setenv("ZAI_API_KEY", envSecret)
	argsPath := installRunnerFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
printf '{"type":"message","text":"arg placeholder ok"}\n'
printf 'arg placeholder stderr\n' >&2
exit 0
`)
	runtimeConfigContent := []byte(`runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    mounts:
      - source: .
        target: /workspace
        mode: rw
`)
	runtimeConfigPath := filepath.Join(tempDir, "runtime.yaml")
	if err := os.WriteFile(runtimeConfigPath, runtimeConfigContent, 0o600); err != nil {
		t.Fatalf("WriteFile(runtime config) error = %v", err)
	}
	runtimeCfg := *validDockerWorkerRuntimeConfig()
	runtimeCfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY"}

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-docker-worker-arg-placeholder-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				dryRunEvent: &workers.WorkerEvent{
					Type:              workers.EventDryRunPlanned,
					Worker:            "codex",
					Command:           []string{"fake-worker", "--prompt", workers.PromptPlaceholder},
					Workspace:         ".",
					Sandbox:           "read-only",
					PromptDelivery:    workers.PromptDeliveryArgPlaceholder,
					PromptPlaceholder: workers.PromptPlaceholder,
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:          writeTaskFileWithDomainAndGoal(t, "codex", "general", rawPrompt),
		StorePath:         storePath,
		ArtifactsDir:      artifactsDir,
		WorkerRuntime:     WorkerRuntimeDocker,
		RuntimeConfig:     &runtimeCfg,
		RuntimeConfigPath: runtimeConfigPath,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	argsData, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args) error = %v", err)
	}
	args := strings.Split(strings.TrimSpace(string(argsData)), "\n")
	if !containsString(args, rawPrompt) {
		t.Fatalf("docker args = %#v, want raw prompt passed to process args", args)
	}
	if strings.Contains(strings.Join(args, " "), envSecret) {
		t.Fatalf("docker args leaked env value: %#v", args)
	}

	runDir := filepath.Join(artifactsDir, "run-docker-worker-arg-placeholder-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker runtime: docker")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Command: docker run")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "--prompt <prompt>")
	assertFileNotContains(t, filepath.Join(runDir, "summary.md"), rawPrompt)
	assertFileNotContains(t, filepath.Join(runDir, "execution-trace.json"), rawPrompt)
	assertFileNotContains(t, filepath.Join(runDir, "stdout.jsonl"), rawPrompt)
	assertFileNotContains(t, filepath.Join(runDir, "stderr.log"), rawPrompt)
	assertFileNotContains(t, filepath.Join(runDir, "summary.md"), envSecret)
	assertFileNotContains(t, filepath.Join(runDir, "execution-trace.json"), envSecret)
	assertFileNotContains(t, filepath.Join(runDir, "stdout.jsonl"), envSecret)
	assertFileNotContains(t, filepath.Join(runDir, "stderr.log"), envSecret)
	trace := readExecutionTrace(t, filepath.Join(runDir, "execution-trace.json"))
	assertTraceString(t, trace, "worker_runtime", "docker")
	assertTraceString(t, trace, "runtime_config_sha256", sha256Hex(runtimeConfigContent))
	assertTraceNonEmptyString(t, trace, "prompt_sha256")
	workspaceMount := filepath.Join(artifactsDir, "run-docker-worker-arg-placeholder-001", "workspace") + ":/workspace:rw"
	assertTraceString(t, trace, "command_display", strings.Join([]string{
		"docker", "run", "--rm", "--network", "none", "--read-only", "-w", "/workspace", "-e", "ZAI_API_KEY",
		"-v", workspaceMount, "--label", "deonclaw.workspace=" + filepath.Join(artifactsDir, "run-docker-worker-arg-placeholder-001", "workspace"),
		"deonclaw-runner:latest", "fake-worker", "--prompt", "<prompt>",
	}, " "))
}

func TestCodexRunnerRunDockerWorkerRuntimeNonZeroFailsRun(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	installRunnerFakeDocker(t, `#!/bin/sh
printf 'bad stdout\n'
printf 'bad stderr\n' >&2
exit 19
`)
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-docker-worker-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				dryRunEvent: &workers.WorkerEvent{
					Worker:  "codex",
					Command: []string{"fake-worker"},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:      writeTaskFile(t, "codex"),
		StorePath:     storePath,
		ArtifactsDir:  artifactsDir,
		WorkerRuntime: WorkerRuntimeDocker,
		RuntimeConfig: validDockerWorkerRuntimeConfig(),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	runDir := filepath.Join(artifactsDir, "run-docker-worker-failed-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker runtime: docker")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Error: docker worker command failed with exit code 19")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "bad stderr\n")
}

func TestCodexRunnerRunDockerWorkerRuntimeMissingEnvFailsBeforeDocker(t *testing.T) {
	unsetRunnerEnvForTest(t, "ZAI_API_KEY")
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	argsPath := installRunnerFakeDocker(t, `#!/bin/sh
printf '%s\n' "$@" > "$DEONCLAW_FAKE_DOCKER_ARGS"
exit 0
`)
	cfg := *validDockerWorkerRuntimeConfig()
	cfg.Runtime.Docker.Env.Passthrough = []string{"ZAI_API_KEY"}
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-docker-worker-missing-env-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				dryRunEvent: &workers.WorkerEvent{
					Worker:  "codex",
					Command: []string{"fake-worker"},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:      writeTaskFile(t, "codex"),
		StorePath:     storePath,
		ArtifactsDir:  artifactsDir,
		WorkerRuntime: WorkerRuntimeDocker,
		RuntimeConfig: &cfg,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "ZAI_API_KEY") {
		t.Fatalf("stderr = %q, want missing env", stderr.String())
	}
	assertRunnerFileEmptyOrMissing(t, argsPath)
	runDir := filepath.Join(artifactsDir, "run-docker-worker-missing-env-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Worker runtime: docker")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "execution-trace.json"), `"worker_runtime": "docker"`)
}

func TestCodexRunnerRunValidationCommandFailure(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-failed-001",
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			return validationFailed(commands, "go-test")
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `validation failed: validation command "go-test" failed with exit code 1`) {
		t.Fatalf("stderr = %q, want validation failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-validation-failed-001")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationFailed, 1)
	assertFileContains(t, filepath.Join(runDir, "validation.log"), `Validation error: validation command "go-test" failed with exit code 1`)
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), `Validation error: validation command "go-test" failed with exit code 1`)

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-validation-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusFailed)
	}
}

func TestCodexRunnerRunSkipsValidationWhenWorkerFails(t *testing.T) {
	validationCalled := false
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-skipped-worker-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Stderr:    "boom\n",
				},
				runErr: errors.New("exit 1"),
			}
		},
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			return validationPassed(commands, "go-test")
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFileWithValidation(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if validationCalled {
		t.Fatal("validation runner was called after worker failure")
	}

	runDir := filepath.Join(artifactsDir, "run-validation-skipped-worker-failed-001")
	assertValidationStatus(t, filepath.Join(runDir, "validation.json"), ValidationSkipped, 1)
	assertFileContains(t, filepath.Join(runDir, "validation.log"), "Skipped reason: worker failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: skipped")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation commands: 1")
}

func TestCodexRunnerRunPolicyFailureOverridesValidationFailure(t *testing.T) {
	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	validationCalled := false
	snapshotCalls := 0

	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-validation-policy-failed-001",
		ValidationRunner: func(ctx context.Context, workspace string, commands []tasks.ValidationCommand) ValidationResult {
			validationCalled = true
			return validationFailed(commands, "go-test")
		},
		GitSnapshotRunner: func(ctx context.Context, workspace string) (*git.Snapshot, error) {
			snapshotCalls++
			if snapshotCalls == 1 {
				if validationCalled {
					t.Fatal("validation ran before baseline snapshot")
				}
				return &git.Snapshot{}, nil
			}
			if !validationCalled {
				t.Fatal("post-run snapshot was captured before validation")
			}
			return &git.Snapshot{
				Entries: []git.FileEntry{
					{Path: "secrets/from-validation.txt", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
				},
			}, nil
		},
	})
	taskPath := writeTaskFileWithValidationAndPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `policy failed: secrets/from-validation.txt matches forbidden path "secrets/**"`) {
		t.Fatalf("stderr = %q, want policy failure", stderr.String())
	}
	if strings.Contains(stderr.String(), "validation failed:") {
		t.Fatalf("stderr = %q, validation failure should not outrank policy failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-validation-policy-failed-001")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: policy_failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Validation: failed")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "secrets/from-validation.txt", string(git.StatusUntracked), string(git.StatusUntracked), "snapshot")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-validation-policy-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusPolicyFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusPolicyFailed)
	}
}

func TestCodexRunnerRunPreservesRawStdoutWhenJSONLIsInvalid(t *testing.T) {
	rawStdout := "{not-json}\n"
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-invalid-json-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Artifacts: []artifacts.Artifact{
						{ID: "stdout", Path: "artifacts/stdout.jsonl", Kind: artifacts.KindEvents, Content: []byte(rawStdout)},
						{ID: "stderr", Path: "artifacts/stderr.log", Kind: artifacts.KindLog, Content: []byte("parse warning\n")},
						{ID: "trace", Path: "artifacts/trace.txt", Kind: artifacts.KindOther, Content: []byte("raw trace\n")},
					},
					Stderr: "parse warning\n",
				},
				runErr: errors.New("parse codex stdout jsonl line 1: invalid JSON"),
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		WorkspaceManager: fakeWorkspacePreparer{
			cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
				t.Fatal("workspace cleanup should not run for failed run")
				return nil
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "invalid JSON") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-invalid-json-001")
	assertFileContent(t, filepath.Join(runDir, "stdout.jsonl"), rawStdout)
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "parse warning\n")
	assertFileContent(t, filepath.Join(runDir, "trace.txt"), "raw trace\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotArtifacts, err := db.ArtifactsByRun(context.Background(), "run-invalid-json-001")
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	wantStdoutPath := filepath.Join(runDir, "stdout.jsonl")
	wantTracePath := filepath.Join(runDir, "trace.txt")
	var sawStdout bool
	var sawTrace bool
	for _, artifact := range gotArtifacts {
		if artifact.Path == wantStdoutPath {
			sawStdout = true
			if artifact.Kind != artifacts.KindEvents {
				t.Fatalf("stdout artifact kind = %q, want %q", artifact.Kind, artifacts.KindEvents)
			}
		}
		if artifact.Path == wantTracePath {
			sawTrace = true
			if artifact.Kind != artifacts.KindOther {
				t.Fatalf("trace artifact kind = %q, want %q", artifact.Kind, artifacts.KindOther)
			}
		}
	}
	if !sawStdout {
		t.Fatalf("stdout artifact path %q was not persisted", wantStdoutPath)
	}
	if !sawTrace {
		t.Fatalf("trace artifact path %q was not persisted", wantTracePath)
	}
}

func TestCodexRunnerRunKeepsStatusWhenCleanupFails(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-cleanup-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
		WorkspaceManager: fakeWorkspacePreparer{
			cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
				return errors.New("remove failed")
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "workspace cleanup warning: workspace cleanup failed: remove failed") {
		t.Fatalf("stderr = %q, want cleanup warning", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-cleanup-failed-001")
	assertFileContains(t, filepath.Join(runDir, "stderr.log"), "workspace cleanup warning: workspace cleanup failed: remove failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: succeeded")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup warning: workspace cleanup failed: remove failed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-cleanup-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
}

func TestCodexRunnerRunMarksPolicyFailedForDiffViolations(t *testing.T) {
	tests := []struct {
		name     string
		runID    string
		taskPath string
		diff     string
		want     string
	}{
		{
			name:     "forbidden path",
			runID:    "run-policy-forbidden-001",
			taskPath: writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"}),
			diff: `diff --git a/secrets/token.txt b/secrets/token.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/secrets/token.txt
@@ -0,0 +1 @@
+token
`,
			want: `secrets/token.txt matches forbidden path "secrets/**"`,
		},
		{
			name:     "outside allowed paths",
			runID:    "run-policy-allowed-001",
			taskPath: writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"}),
			diff: `diff --git a/README.md b/README.md
index 1111111..2222222 100644
--- a/README.md
+++ b/README.md
@@ -1 +1 @@
-old
+new
`,
			want: "README.md is outside allowed paths",
		},
		{
			name:     "read only diff",
			runID:    "run-policy-readonly-001",
			taskPath: writeTaskFileWithPolicy(t, "codex", "read_only", nil, []string{"secrets/**"}),
			diff: `diff --git a/internal/tasks/task.go b/internal/tasks/task.go
index 1111111..2222222 100644
--- a/internal/tasks/task.go
+++ b/internal/tasks/task.go
@@ -1 +1 @@
-old
+new
`,
			want: "read_only task changed files: internal/tasks/task.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changedInDiff := policy.ChangedPathsFromGitDiff([]byte(tt.diff))
			postSnapshot := &git.Snapshot{
				Entries: make([]git.FileEntry, 0, len(changedInDiff)),
			}
			for _, p := range changedInDiff {
				postSnapshot.Entries = append(postSnapshot.Entries, git.FileEntry{
					Path:     p,
					Staged:   git.StatusUnmodified,
					Unstaged: git.StatusModified,
				})
			}
			runner := testCodexRunner(t, testCodexRunnerOptions{
				RunID: tt.runID,
				WorkerFactory: func() workers.Worker {
					return fakeWorker{
						runResult: &workers.RunResult{
							Workspace: ".",
							Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
							Events: []workers.WorkerEvent{
								{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
							},
						},
					}
				},
				Diff:     tt.diff,
				Baseline: &git.Snapshot{},
				PostRun:  postSnapshot,
				WorkspaceManager: fakeWorkspacePreparer{
					cleanupFunc: func(ctx context.Context, workspace *runtime.Workspace) error {
						t.Fatal("workspace cleanup should not run for policy_failed run")
						return nil
					},
				},
			})

			tempDir := t.TempDir()
			storePath := filepath.Join(tempDir, "deonclaw.db")
			artifactsDir := filepath.Join(tempDir, "artifacts")

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			code := runner.Run(context.Background(), CodexRunOptions{
				TaskPath:     tt.taskPath,
				StorePath:    storePath,
				ArtifactsDir: artifactsDir,
			}, &stdout, &stderr)

			if code != 1 {
				t.Fatalf("Run() exit code = %d, want 1", code)
			}
			if !strings.Contains(stderr.String(), "policy failed: "+tt.want) {
				t.Fatalf("stderr = %q, want policy failure %q", stderr.String(), tt.want)
			}

			runDir := filepath.Join(artifactsDir, tt.runID)
			assertFileContains(t, filepath.Join(runDir, "diff.patch"), tt.diff)
			assertFileContains(t, filepath.Join(runDir, "changed-files.json"), `"source": "snapshot"`)
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: policy_failed")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Policy: "+tt.want)
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 1")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
			assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: policy_failed")

			db, err := storepkg.OpenSQLite(storePath)
			if err != nil {
				t.Fatalf("OpenSQLite() error = %v", err)
			}
			defer db.Close()

			gotRun, err := db.Run(context.Background(), tt.runID)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if gotRun.Status != runs.StatusPolicyFailed {
				t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusPolicyFailed)
			}

			gotArtifacts, err := db.ArtifactsByRun(context.Background(), tt.runID)
			if err != nil {
				t.Fatalf("ArtifactsByRun() error = %v", err)
			}
			wantDiffPath := filepath.Join(runDir, "diff.patch")
			wantChangedFilesPath := filepath.Join(runDir, "changed-files.json")
			var sawDiff bool
			var sawChangedFiles bool
			for _, artifact := range gotArtifacts {
				if artifact.Path == wantDiffPath {
					sawDiff = true
					if artifact.Kind != artifacts.KindDiff {
						t.Fatalf("diff artifact kind = %q, want %q", artifact.Kind, artifacts.KindDiff)
					}
				}
				if artifact.Path == wantChangedFilesPath {
					sawChangedFiles = true
					if artifact.Kind != artifacts.KindOther {
						t.Fatalf("changed-files artifact kind = %q, want %q", artifact.Kind, artifacts.KindOther)
					}
				}
			}
			if !sawDiff {
				t.Fatalf("diff artifact path %q was not persisted", wantDiffPath)
			}
			if !sawChangedFiles {
				t.Fatalf("changed-files artifact path %q was not persisted", wantChangedFilesPath)
			}
		})
	}
}

func TestCodexRunnerRunPersistsFailureArtifacts(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-failed-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"partial"}`)},
					},
					Stderr: "boom\n",
				},
				runErr: errors.New("exit 1"),
			}
		},
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "run failed: exit 1") {
		t.Fatalf("stderr = %q, want run failure", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-failed-001")
	assertFileContent(t, filepath.Join(runDir, "events.jsonl"), `{"type":"message","text":"partial"}`+"\n")
	assertFileContent(t, filepath.Join(runDir, "stderr.log"), "boom\n")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Status: failed")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Workspace cleanup: kept")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Cleanup reason: failed")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-failed-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusFailed)
	}

	gotEvents, err := db.EventsByRun(context.Background(), "run-failed-001")
	if err != nil {
		t.Fatalf("EventsByRun() error = %v", err)
	}
	if len(gotEvents) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(gotEvents))
	}
}

func TestCodexRunnerRunRejectsMismatchedTaskWorker(t *testing.T) {
	tempDir := t.TempDir()
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID:    "run-mismatch-001",
		Baseline: &git.Snapshot{},
		PostRun:  &git.Snapshot{},
	})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "opencode"),
		StorePath:    filepath.Join(tempDir, "deonclaw.db"),
		ArtifactsDir: filepath.Join(tempDir, "artifacts"),
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	want := `task worker "opencode" does not match requested worker "codex"`
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
}

func TestCodexRunnerRunDetectsChangedPathsFromSnapshotDiff(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-snapshot-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun: &git.Snapshot{
			Entries: []git.FileEntry{
				{Path: "internal/tasks/task.go", Staged: git.StatusModified, Unstaged: git.StatusUnmodified},
				{Path: "newfile.go", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	taskPath := writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**", "newfile.go"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() exit code = %d, stderr = %q", code, stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-snapshot-001")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "internal/tasks/task.go", string(git.StatusModified), string(git.StatusUnmodified), "snapshot")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "newfile.go", string(git.StatusUntracked), string(git.StatusUntracked), "snapshot")
	assertFileContains(t, filepath.Join(runDir, "diff.patch"), "# Untracked files from snapshot")
	assertFileContains(t, filepath.Join(runDir, "diff.patch"), "# path: newfile.go staged: ? unstaged: ? source: snapshot")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 2")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-snapshot-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusSucceeded {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusSucceeded)
	}
}

func TestCodexRunnerRunDetectsDirtyBaseline(t *testing.T) {
	workerCalled := false
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-dirty-001",
		WorkerFactory: func() workers.Worker {
			workerCalled = true
			return fakeWorker{}
		},
		Baseline: &git.Snapshot{
			Entries: []git.FileEntry{
				{Path: "existing-dirty.go", Staged: git.StatusModified, Unstaged: git.StatusUnmodified},
			},
		},
		PostRun: &git.Snapshot{},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     writeTaskFile(t, "codex"),
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "workspace is dirty before run") {
		t.Fatalf("stderr = %q, want dirty baseline failure", stderr.String())
	}
	if workerCalled {
		t.Fatal("worker was called for dirty baseline")
	}
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatalf("store path exists after dirty baseline failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "run-dirty-001")); !os.IsNotExist(err) {
		t.Fatalf("run artifacts dir exists after dirty baseline failure: %v", err)
	}
}

func TestCodexRunnerRunPolicyUsesSnapshotPathsNotJustDiff(t *testing.T) {
	runner := testCodexRunner(t, testCodexRunnerOptions{
		RunID: "run-untracked-001",
		WorkerFactory: func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "workspace-write", "--cd", ".", "-"},
					Events: []workers.WorkerEvent{
						{Type: "message", Worker: "codex", Payload: []byte(`{"type":"message","text":"ok"}`)},
					},
				},
			}
		},
		Baseline: &git.Snapshot{},
		PostRun: &git.Snapshot{
			Entries: []git.FileEntry{
				{Path: "secrets/leaked.key", Staged: git.StatusUntracked, Unstaged: git.StatusUntracked},
			},
		},
	})

	tempDir := t.TempDir()
	storePath := filepath.Join(tempDir, "deonclaw.db")
	artifactsDir := filepath.Join(tempDir, "artifacts")
	taskPath := writeTaskFileWithPolicy(t, "codex", "workspace_write", []string{"internal/**"}, []string{"secrets/**"})

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := runner.Run(context.Background(), CodexRunOptions{
		TaskPath:     taskPath,
		StorePath:    storePath,
		ArtifactsDir: artifactsDir,
	}, &stdout, &stderr)

	if code != 1 {
		t.Fatalf("Run() exit code = %d, want 1 (policy failure)", code)
	}
	if !strings.Contains(stderr.String(), `secrets/leaked.key matches forbidden path "secrets/**"`) {
		t.Fatalf("stderr = %q, want untracked forbidden path violation", stderr.String())
	}

	runDir := filepath.Join(artifactsDir, "run-untracked-001")
	assertChangedFile(t, filepath.Join(runDir, "changed-files.json"), "secrets/leaked.key", string(git.StatusUntracked), string(git.StatusUntracked), "snapshot")
	assertFileContains(t, filepath.Join(runDir, "diff.patch"), "# path: secrets/leaked.key staged: ? unstaged: ? source: snapshot")
	assertFileContains(t, filepath.Join(runDir, "summary.md"), "Changed paths: 1")

	db, err := storepkg.OpenSQLite(storePath)
	if err != nil {
		t.Fatalf("OpenSQLite() error = %v", err)
	}
	defer db.Close()

	gotRun, err := db.Run(context.Background(), "run-untracked-001")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if gotRun.Status != runs.StatusPolicyFailed {
		t.Fatalf("run status = %q, want %q", gotRun.Status, runs.StatusPolicyFailed)
	}
}

type testCodexRunnerOptions struct {
	RunID             string
	WorkerFactory     func() workers.Worker
	Diff              string
	Baseline          *git.Snapshot
	PostRun           *git.Snapshot
	GitSnapshotRunner GitSnapshotRunner
	ValidationRunner  ValidationRunner
	WorkspaceManager  WorkspacePreparer
}

func testCodexRunner(t *testing.T, opts testCodexRunnerOptions) CodexRunner {
	t.Helper()
	workerFactory := opts.WorkerFactory
	if workerFactory == nil {
		workerFactory = func() workers.Worker {
			return fakeWorker{
				runResult: &workers.RunResult{
					Workspace: ".",
					Command:   []string{"codex", "exec", "--json", "--sandbox", "read-only", "--cd", ".", "-"},
				},
			}
		}
	}
	workspaceManager := opts.WorkspaceManager
	if workspaceManager == nil {
		workspaceManager = fakeWorkspacePreparer{}
	}
	callCount := 0
	gitSnapshotRunner := opts.GitSnapshotRunner
	if gitSnapshotRunner == nil {
		gitSnapshotRunner = func(ctx context.Context, workspace string) (*git.Snapshot, error) {
			callCount++
			if callCount == 1 {
				return opts.Baseline, nil
			}
			return opts.PostRun, nil
		}
	}
	return CodexRunner{
		WorkerFactory: workerFactory,
		RunIDFactory: func() string {
			return opts.RunID
		},
		GitDiffRunner: func(context.Context, string) ([]byte, error) {
			return []byte(opts.Diff), nil
		},
		GitSnapshotRunner: gitSnapshotRunner,
		ValidationRunner:  opts.ValidationRunner,
		WorkspaceManagerFactory: func() WorkspacePreparer {
			return workspaceManager
		},
	}
}

func validDockerWorkerRuntimeConfig() *runtimeconfig.Config {
	return &runtimeconfig.Config{
		Runtime: runtimeconfig.Runtime{
			Mode: runtimeconfig.ModeDocker,
			Docker: runtimeconfig.DockerConfig{
				Image:        "deonclaw-runner:latest",
				Workdir:      "/workspace",
				Network:      "none",
				ReadOnlyRoot: true,
				Mounts: []runtimeconfig.MountSpec{
					{Source: ".", Target: "/workspace", Mode: "rw"},
				},
			},
		},
	}
}

type fakeWorkspacePreparer struct {
	prepareFunc func(context.Context, runtime.WorkspaceSpec) (*runtime.Workspace, error)
	cleanupFunc func(context.Context, *runtime.Workspace) error
}

func (f fakeWorkspacePreparer) Prepare(ctx context.Context, spec runtime.WorkspaceSpec) (*runtime.Workspace, error) {
	if f.prepareFunc != nil {
		return f.prepareFunc(ctx, spec)
	}
	sourcePath := strings.TrimSpace(spec.SourcePath)
	if sourcePath == "" {
		sourcePath = "."
	}
	sourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, err
	}
	workspacePath, err := filepath.Abs(filepath.Join(spec.RootDir, spec.RunID, "workspace"))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return nil, err
	}
	return &runtime.Workspace{
		Path:       workspacePath,
		SourcePath: sourcePath,
		Method:     runtime.MethodGitWorktree,
	}, nil
}

func (f fakeWorkspacePreparer) Cleanup(ctx context.Context, workspace *runtime.Workspace) error {
	if f.cleanupFunc != nil {
		return f.cleanupFunc(ctx, workspace)
	}
	return nil
}

type fakeWorker struct {
	dryRunEvent *workers.WorkerEvent
	runFunc     func(context.Context, workers.RunSpec) (*workers.RunResult, error)
	runResult   *workers.RunResult
	runErr      error
}

func (f fakeWorker) DryRun(context.Context, workers.RunSpec) (*workers.WorkerEvent, error) {
	return f.dryRunEvent, nil
}

func (f fakeWorker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	if f.runFunc != nil {
		return f.runFunc(ctx, spec)
	}
	return f.runResult, f.runErr
}

func writeTaskFile(t *testing.T, worker string) string {
	t.Helper()
	content := `id: worker-mismatch-001
title: "Worker mismatch"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeTaskFileWithDomainAndGoal(t *testing.T, worker string, domain string, goal string) string {
	t.Helper()
	return writeTaskFileContent(t, `id: context-task-`+domain+`
title: "Context task `+domain+`"
domain: `+domain+`
worker: `+worker+`
goal: "`+goal+`"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: domain
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - context pack is available
`)
}

func writeTaskFileWithMCPContext(t *testing.T, worker string, attachments []tasks.MCPContextAttachment) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString(`id: mcp-context-task-001
title: "MCP context task"
domain: general
worker: ` + worker + `
goal: "Use attached MCP context passively"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
mcp_context:
  attachments:
`)
	for _, attachment := range attachments {
		builder.WriteString("    - name: " + attachment.Name + "\n")
		builder.WriteString("      kind: " + attachment.Kind + "\n")
		builder.WriteString("      path: " + filepath.ToSlash(attachment.Path) + "\n")
	}
	builder.WriteString(`allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - MCP context is attached passively
`)
	return writeTaskFileContent(t, builder.String())
}

func writeTaskFileWithRetrievalContext(t *testing.T, worker string, attachments []tasks.RetrievalContextAttachment) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString(`id: retrieval-context-task-001
title: "Retrieval context task"
domain: general
worker: ` + worker + `
goal: "Use attached LanceDB retrieval metadata passively"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
retrieval_context:
  attachments:
`)
	for _, attachment := range attachments {
		builder.WriteString("    - kind: " + attachment.Kind + "\n")
		builder.WriteString("      path: " + filepath.ToSlash(attachment.Path) + "\n")
		builder.WriteString("      report_path: " + filepath.ToSlash(attachment.ReportPath) + "\n")
		builder.WriteString("      policy: " + filepath.ToSlash(attachment.Policy) + "\n")
		builder.WriteString("      max_results: " + fmt.Sprintf("%d", attachment.MaxResults) + "\n")
	}
	builder.WriteString(`allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - Retrieval context is attached passively
`)
	return writeTaskFileContent(t, builder.String())
}

func writeRunnerGovernanceBundleJSON(t *testing.T, path string) {
	t.Helper()
	payload := map[string]any{
		"status":                          "ok",
		"contains_text":                   false,
		"runner_execution":                false,
		"injection_authorized_for_future": true,
		"execution_supported_now":         false,
		"materialized_sha256":             strings.Repeat("a", 64),
		"policy_sha256":                   strings.Repeat("b", 64),
		"governance_report_sha256":        strings.Repeat("c", 64),
		"approval_sha256":                 strings.Repeat("d", 64),
		"execution_plan_sha256":           strings.Repeat("e", 64),
		"prompt_preview_manifest_sha256":  strings.Repeat("f", 64),
		"prompt_preview_sha256":           strings.Repeat("1", 64),
		"caps": map[string]any{
			"max_total_chars":     6000,
			"max_chars_per_chunk": 1200,
			"max_chunks":          5,
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

type runnerSearchEnvelopeFixture struct {
	GeneratedAt string `json:"generated_at"`
	Search      struct {
		Status    string            `json:"status"`
		QueryMode string            `json:"query_mode"`
		TopK      int               `json:"top_k"`
		Results   []json.RawMessage `json:"results"`
	} `json:"search"`
	Summary lancedbpolicy.SearchSmokeSummary `json:"summary"`
}

func runnerValidSearchEnvelope() runnerSearchEnvelopeFixture {
	envelope := runnerSearchEnvelopeFixture{GeneratedAt: "2026-01-01T00:00:00Z"}
	envelope.Search.Status = lancedbpolicy.StatusOK
	envelope.Search.QueryMode = "vector"
	envelope.Search.TopK = 5
	envelope.Search.Results = []json.RawMessage{runnerMarshalSearchHit(lancedbpolicy.SearchHit{
		Rank: 1, ChunkID: "chunk-a", VectorID: "vec-a", Distance: 0.12, Domain: "general",
		SourcePath: "notes/a.md", SourceSHA256: "sha-source", TextSHA256: "sha-text",
		EmbeddingModel: "deterministic-hash-v1", Provider: "local",
	})}
	envelope.Summary = lancedbpolicy.SearchSmokeSummary{
		QueryMode: "vector", TopK: 5, ResultCount: 1,
		DatabasePath: "artifacts/lancedb-smoke", Table: "memory_vectors",
		RetrievalPerformed: true, RunnerIntegration: false,
	}
	return envelope
}

func runnerMarshalSearchHit(hit lancedbpolicy.SearchHit) json.RawMessage {
	raw, err := json.Marshal(hit)
	if err != nil {
		panic(err)
	}
	return raw
}

func writeRunnerLanceDBRetrievalFixtures(t *testing.T, envelope runnerSearchEnvelopeFixture) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	return writeRunnerLanceDBRetrievalFixturesInDir(t, dir, envelope)
}

func writeRunnerLanceDBRetrievalFixturesInDir(t *testing.T, dir string, envelope runnerSearchEnvelopeFixture) (string, string, string) {
	t.Helper()
	policyPath := filepath.Join(dir, "lancedb-policy.yaml")
	resultPath := filepath.Join(dir, "lancedb-search-smoke-result.json")
	reportPath := filepath.Join(dir, "lancedb-search-report.json")

	cfg := lancedbpolicy.Config{LanceDBPolicy: lancedbpolicy.Policy{
		Input: lancedbpolicy.InputConfig{
			EmbeddingManifest: "artifacts/memory-embedding-manifest.json",
			VectorsPath:       "artifacts/memory-index-vectors.jsonl",
		},
		Database: lancedbpolicy.DatabaseConfig{Path: "artifacts/lancedb-smoke", Table: "memory_vectors"},
		Schema: lancedbpolicy.SchemaConfig{
			VectorColumn:  "vector",
			TextRefColumn: "chunk_id",
			MetadataColumns: []string{
				"domain", "source_path", "source_sha256", "text_sha256", "embedding_model", "provider",
			},
		},
		Limits: lancedbpolicy.LimitsConfig{MaxVectors: 10000, ExpectedDimensions: 16},
		Mode:   lancedbpolicy.ModeWriteSmoke,
	}}
	policyBytes, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(policyPath, policyBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	resultBytes, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	resultBytes = append(resultBytes, '\n')
	if err := os.WriteFile(resultPath, resultBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	report, err := lancedbpolicy.SearchReport(resultPath, cfg, lancedbpolicy.SearchReportOptions{})
	if err != nil {
		t.Fatalf("SearchReport() error = %v", err)
	}
	reportBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	reportBytes = append(reportBytes, '\n')
	if err := os.WriteFile(reportPath, reportBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return resultPath, reportPath, policyPath
}

func writeTaskFileWithMCPProposalPolicy(t *testing.T, worker string, configPath string, policyPath string, runtimeConfigPath string, requirePreflight bool) string {
	t.Helper()
	builder := strings.Builder{}
	builder.WriteString(`id: mcp-proposal-task-001
title: "MCP proposal task"
domain: general
worker: ` + worker + `
goal: "Review worker MCP proposal without execution"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
mcp_proposal_policy:
  config: ` + filepath.ToSlash(configPath) + `
  policy: ` + filepath.ToSlash(policyPath) + `
`)
	if strings.TrimSpace(runtimeConfigPath) != "" {
		builder.WriteString("  runtime_config: " + filepath.ToSlash(runtimeConfigPath) + "\n")
	}
	builder.WriteString("  require_preflight: ")
	if requirePreflight {
		builder.WriteString("true\n")
	} else {
		builder.WriteString("false\n")
	}
	builder.WriteString(`allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - MCP proposal is linted without execution
`)
	return writeTaskFileContent(t, builder.String())
}

func writeRunnerMCPAttachment(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write MCP attachment: %v", err)
	}
	return path
}

func runnerCallBundleJSON(fields string) string {
	return `{
  "proposal_id":"proposal-001",
  "approval_sha256":"approval-hash",
  "proposal_sha256":"proposal-hash",
  "policy_sha256":"policy-hash",
  "server":"filesystem-readonly",
  "tool":"fs.read",
  "arguments_sha256":"arguments-hash",
  "runtime":"docker",
  ` + fields + `,
  "artifacts":{
    "mcp-call-smoke-summary.md":"artifacts/mcp-call-smoke-summary.md",
    "mcp-call-transcript.jsonl":"artifacts/mcp-call-transcript.jsonl",
    "mcp-call-result.json":"artifacts/mcp-call-result.json",
    "mcp-call-stdout.log":"artifacts/mcp-call-stdout.log",
    "mcp-call-stderr.log":"artifacts/mcp-call-stderr.log",
    "mcp-call-response.json":"artifacts/mcp-call-response.json"
  }
}`
}

func writeDomainsConfigWithBridge(t *testing.T, bridgePath string) string {
	t.Helper()
	content := `domains:
  general:
    type: canonical_memory
    root: /vault/mysecondbrain
    default: true

  escalasoft:
    type: isolated_domain
    root: /domains/escalasoft_brain
    default: false
    bridge_files:
      - ` + filepath.ToSlash(bridgePath) + `
    structured_data:
      historical_sqlite: /data/escalasoft.db
    staging:
      - /tmp/escalasoft
    default_agent: escalasoft-agent
`
	path := filepath.Join(t.TempDir(), "domains.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write domains file: %v", err)
	}
	return path
}

func memoryProposalJSON(t *testing.T, id string, domain string, targetPath string, operation memory.MemoryOperation) []byte {
	t.Helper()
	proposal := memory.NewProposal(memory.NewProposalOptions{
		ProposalID: id,
		RunID:      "run-001",
		TaskID:     "task-001",
		Domain:     domain,
		TargetPath: targetPath,
		Operation:  operation,
		Reason:     "Worker proposed memory update.",
	})
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	return data
}

func writeTaskFileWithValidation(t *testing.T, worker string) string {
	t.Helper()
	return writeTaskFileContent(t, `id: validation-task-001
title: "Validation task"
domain: general
worker: `+worker+`
goal: "Do not execute"
mode: read_only
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
validation:
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
      timeout_seconds: 300
allowed_paths: []
forbidden_paths:
  - secrets/**
expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - validation executes
`)
}

func writeTaskFileWithPolicy(t *testing.T, worker string, mode string, allowedPaths []string, forbiddenPaths []string) string {
	t.Helper()

	content := `id: task-` + strings.ReplaceAll(mode, "_", "-") + `-001
title: "Policy task"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: ` + mode + `
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
allowed_paths:
` + yamlStringList(allowedPaths) + `forbidden_paths:
` + yamlStringList(forbiddenPaths) + `expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - command fails before worker execution
`
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func writeTaskFileWithValidationAndPolicy(t *testing.T, worker string, mode string, allowedPaths []string, forbiddenPaths []string) string {
	t.Helper()

	content := `id: task-validation-` + strings.ReplaceAll(mode, "_", "-") + `-001
title: "Validation policy task"
domain: general
worker: ` + worker + `
goal: "Do not execute"
mode: ` + mode + `
workspace:
  strategy: local_repo
  path: .
memory:
  scope: none
validation:
  commands:
    - name: go-test
      command: go
      args:
        - test
        - ./...
      timeout_seconds: 300
allowed_paths:
` + yamlStringList(allowedPaths) + `forbidden_paths:
` + yamlStringList(forbiddenPaths) + `expected_outputs:
  - artifacts/summary.md
definition_of_done:
  - validation executes
`
	return writeTaskFileContent(t, content)
}

func writeTaskFileContent(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "task.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write task file: %v", err)
	}
	return path
}

func yamlStringList(values []string) string {
	if len(values) == 0 {
		return "  []\n"
	}
	var output strings.Builder
	for _, value := range values {
		output.WriteString("  - ")
		output.WriteString(value)
		output.WriteByte('\n')
	}
	return output.String()
}

func assertFileContent(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertFileContains(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if !strings.Contains(string(got), want) {
		t.Fatalf("ReadFile(%q) = %q, want %q", path, got, want)
	}
}

func assertFileNotContains(t *testing.T, path string, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	if strings.Contains(string(got), want) {
		t.Fatalf("ReadFile(%q) = %q, did not want %q", path, got, want)
	}
}

func assertChangedFile(t *testing.T, path string, wantPath string, wantStaged string, wantUnstaged string, wantSource string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var changedFiles []ChangedFile
	if err := json.Unmarshal(got, &changedFiles); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	for _, changed := range changedFiles {
		if changed.Path != wantPath {
			continue
		}
		if changed.Staged != wantStaged || changed.Unstaged != wantUnstaged || changed.Source != wantSource {
			t.Fatalf("changed file %+v, want staged=%q unstaged=%q source=%q", changed, wantStaged, wantUnstaged, wantSource)
		}
		return
	}
	t.Fatalf("changed file %q not found in %q: %s", wantPath, path, got)
}

func assertValidationStatus(t *testing.T, path string, wantStatus string, wantCommandCount int) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var result ValidationResult
	if err := json.Unmarshal(got, &result); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	if result.Status != wantStatus {
		t.Fatalf("validation status = %q, want %q", result.Status, wantStatus)
	}
	if result.CommandCount != wantCommandCount {
		t.Fatalf("validation command_count = %d, want %d", result.CommandCount, wantCommandCount)
	}
}

func assertMemoryProposalLintStatus(t *testing.T, path string, wantStatus string, minViolations int, wantWarnings int) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var decoded struct {
		Status     string   `json:"status"`
		Violations []string `json:"violations"`
		Warnings   []string `json:"warnings"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	if decoded.Status != wantStatus {
		t.Fatalf("lint status = %q, want %q; json=%s", decoded.Status, wantStatus, got)
	}
	if len(decoded.Violations) < minViolations {
		t.Fatalf("lint violations = %d, want at least %d; json=%s", len(decoded.Violations), minViolations, got)
	}
	if len(decoded.Warnings) != wantWarnings {
		t.Fatalf("lint warnings = %d, want %d; json=%s", len(decoded.Warnings), wantWarnings, got)
	}
}

func assertMCPToolProposalLintStatus(t *testing.T, path string, wantStatus string, minViolations int, wantWarnings int) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var decoded struct {
		Status     string   `json:"status"`
		Violations []string `json:"violations"`
		Warnings   []string `json:"warnings"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	if decoded.Status != wantStatus {
		t.Fatalf("MCP proposal lint status = %q, want %q; json=%s", decoded.Status, wantStatus, got)
	}
	if len(decoded.Violations) < minViolations {
		t.Fatalf("MCP proposal lint violations = %d, want at least %d; json=%s", len(decoded.Violations), minViolations, got)
	}
	if len(decoded.Warnings) != wantWarnings {
		t.Fatalf("MCP proposal lint warnings = %d, want %d; json=%s", len(decoded.Warnings), wantWarnings, got)
	}
}

func assertMCPToolPreflightStatus(t *testing.T, path string, wantStatus string, minFailures int) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var decoded struct {
		Status   string   `json:"status"`
		Failures []string `json:"failures"`
	}
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	if decoded.Status != wantStatus {
		t.Fatalf("MCP proposal preflight status = %q, want %q; json=%s", decoded.Status, wantStatus, got)
	}
	if len(decoded.Failures) < minFailures {
		t.Fatalf("MCP proposal preflight failures = %d, want at least %d; json=%s", len(decoded.Failures), minFailures, got)
	}
}

func mcpToolProposalJSON(t *testing.T, server string, tool string, arguments string, policyPath string, configPath string, runtimeConfigPath string) []byte {
	t.Helper()
	proposal, err := mcpapproval.NewProposal(mcpapproval.NewProposalOptions{
		ID:                "mcp-call-runner-test-001",
		Server:            server,
		Tool:              tool,
		Arguments:         []byte(arguments),
		Reason:            "runner test proposal",
		RequestedBy:       "worker",
		SourceType:        mcpapproval.SourceTypeManual,
		PolicyPath:        policyPath,
		ConfigPath:        configPath,
		Runtime:           "local",
		RuntimeConfigPath: runtimeConfigPath,
		Workspace:         ".",
	})
	if err != nil {
		t.Fatalf("NewProposal() error = %v", err)
	}
	data, err := proposal.JSON()
	if err != nil {
		t.Fatalf("proposal.JSON() error = %v", err)
	}
	return data
}

func writeRunnerMCPProposalPolicyFiles(t *testing.T, dir string, capabilities []string, allowedTools []string) (string, string, string) {
	t.Helper()
	configPath := filepath.Join(dir, "mcp.yaml")
	policyPath := filepath.Join(dir, "mcp-call-policy.yaml")
	runtimeConfigPath := filepath.Join(dir, "runtime.yaml")

	capabilityLines := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		capabilityLines = append(capabilityLines, "        - "+capability)
	}
	toolLines := make([]string, 0, len(allowedTools))
	for _, tool := range allowedTools {
		toolLines = append(toolLines, "    - "+tool)
	}
	config := `mcp:
  servers:
    fake-stdio:
      command: deonctl
      args:
        - mcp
        - fake-server
      enabled: false
      test_only: true
      protocol: stdio
      trust: local
      capabilities:
` + strings.Join(capabilityLines, "\n") + `
      env:
        passthrough: []
`
	policy := `mcp_call_policy:
  allow_real_readonly: true
  require_docker_for_real: false
  max_tool_calls: 1
  allowed_servers:
    - fake-stdio
  allowed_tools:
` + strings.Join(toolLines, "\n") + `
  allowed_capabilities:
    - read
  max_arguments_bytes: 65536
  max_response_bytes: 1048576
`
	runtimeConfig := `runtime:
  mode: docker
  docker:
    image: deonclaw-runner:latest
    workdir: /workspace
    network: none
    read_only_root: true
    mounts:
      - source: .
        target: /workspace
        mode: rw
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("WriteFile(mcp config) error = %v", err)
	}
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatalf("WriteFile(call policy) error = %v", err)
	}
	if err := os.WriteFile(runtimeConfigPath, []byte(runtimeConfig), 0o600); err != nil {
		t.Fatalf("WriteFile(runtime config) error = %v", err)
	}
	return configPath, policyPath, runtimeConfigPath
}

func assertArtifactManifestContains(t *testing.T, path string, wantPath string, wantKind artifacts.Kind) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}

	var manifest artifactManifest
	if err := json.Unmarshal(got, &manifest); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	for _, artifact := range manifest.Artifacts {
		if artifact.Path != wantPath {
			continue
		}
		if artifact.Kind != wantKind {
			t.Fatalf("manifest artifact kind = %q, want %q", artifact.Kind, wantKind)
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("manifest artifact sha256 = %q, want 64 hex chars", artifact.SHA256)
		}
		return
	}
	t.Fatalf("artifact %q not found in manifest %q", wantPath, path)
}

func readExecutionTrace(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	var trace map[string]interface{}
	if err := json.Unmarshal(got, &trace); err != nil {
		t.Fatalf("Unmarshal(%q) error = %v", path, err)
	}
	return trace
}

func assertTraceString(t *testing.T, trace map[string]interface{}, field string, want string) {
	t.Helper()
	got, ok := trace[field].(string)
	if !ok {
		t.Fatalf("trace[%q] = %#v, want string %q", field, trace[field], want)
	}
	if got != want {
		t.Fatalf("trace[%q] = %q, want %q", field, got, want)
	}
}

func assertTraceNonEmptyString(t *testing.T, trace map[string]interface{}, field string) {
	t.Helper()
	got, ok := trace[field].(string)
	if !ok || got == "" {
		t.Fatalf("trace[%q] = %#v, want non-empty string", field, trace[field])
	}
}

func assertTraceTimelineContains(t *testing.T, trace map[string]interface{}, events ...string) {
	t.Helper()
	rawTimeline, ok := trace["timeline"].([]interface{})
	if !ok {
		t.Fatalf("trace timeline = %#v, want array", trace["timeline"])
	}
	seen := map[string]struct{}{}
	for _, rawEvent := range rawTimeline {
		event, ok := rawEvent.(map[string]interface{})
		if !ok {
			continue
		}
		name, ok := event["event"].(string)
		if ok {
			seen[name] = struct{}{}
		}
	}
	for _, event := range events {
		if _, ok := seen[event]; !ok {
			t.Fatalf("trace timeline missing %q: %#v", event, rawTimeline)
		}
	}
}

func assertPersistedArtifact(t *testing.T, db *storepkg.SQLiteStore, runID string, wantPath string) {
	t.Helper()
	gotArtifacts, err := db.ArtifactsByRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	for _, artifact := range gotArtifacts {
		if artifact.Path != wantPath {
			continue
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("artifact %q sha256 = %q, want 64 hex chars", artifact.Path, artifact.SHA256)
		}
		return
	}
	t.Fatalf("artifact %q was not persisted", wantPath)
}

func assertPersistedArtifactWithMetadata(t *testing.T, db *storepkg.SQLiteStore, runID string, wantPath string) {
	t.Helper()
	gotArtifacts, err := db.ArtifactsByRun(context.Background(), runID)
	if err != nil {
		t.Fatalf("ArtifactsByRun() error = %v", err)
	}
	for _, artifact := range gotArtifacts {
		if artifact.Path != wantPath {
			continue
		}
		if artifact.SizeBytes <= 0 {
			t.Fatalf("artifact %q size_bytes = %d, want > 0", artifact.Path, artifact.SizeBytes)
		}
		if len(artifact.SHA256) != 64 {
			t.Fatalf("artifact %q sha256 = %q, want 64 hex chars", artifact.Path, artifact.SHA256)
		}
		return
	}
	t.Fatalf("artifact %q was not persisted", wantPath)
}

func validationPassed(commands []tasks.ValidationCommand, name string) ValidationResult {
	return ValidationResult{
		Status:       ValidationPassed,
		CommandCount: len(commands),
		Commands: []ValidationCommandResult{
			{
				Name:           name,
				Command:        "go",
				Args:           []string{"test", "./..."},
				TimeoutSeconds: defaultValidationTimeoutSeconds,
				ExitCode:       0,
				DurationMS:     1,
				Status:         ValidationPassed,
				Stdout:         "ok\n",
			},
		},
	}
}

func validationFailed(commands []tasks.ValidationCommand, name string) ValidationResult {
	return ValidationResult{
		Status:       ValidationFailed,
		CommandCount: len(commands),
		Error:        `validation command "` + name + `" failed with exit code 1`,
		Commands: []ValidationCommandResult{
			{
				Name:           name,
				Command:        "go",
				Args:           []string{"test", "./..."},
				TimeoutSeconds: defaultValidationTimeoutSeconds,
				ExitCode:       1,
				DurationMS:     1,
				Status:         ValidationFailed,
				Stdout:         "fail\n",
				Stderr:         "boom\n",
				Error:          "exit status 1",
			},
		},
	}
}

func runGitTestCommand(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
