package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

type stubExternalWorker struct {
	called bool
	spec   workers.RunSpec
	result *workers.RunResult
	err    error
}

func (w *stubExternalWorker) DryRun(context.Context, workers.RunSpec) (*workers.WorkerEvent, error) {
	return nil, nil
}

func (w *stubExternalWorker) Run(ctx context.Context, spec workers.RunSpec) (*workers.RunResult, error) {
	_ = ctx
	w.called = true
	w.spec = spec
	return w.result, w.err
}

func TestExternalWorkerRunnerRequiresRealConfirmation(t *testing.T) {
	worker := &stubExternalWorker{}
	_, err := NewExternalWorkerRunner(worker).Run(context.Background(), WorkerRunOptions{
		TaskJSON: mustTaskJSON(t, "task-real-confirm"),
		Mode:     ModeReal,
	})
	if err == nil || err.Error() != BlockedRealModeConfirmationRequired {
		t.Fatalf("err = %v", err)
	}
	if worker.called {
		t.Fatal("worker should not run without real confirmation")
	}
}

func TestExternalWorkerRunnerRejectsNonRealMode(t *testing.T) {
	worker := &stubExternalWorker{}
	_, err := NewExternalWorkerRunner(worker).Run(context.Background(), WorkerRunOptions{
		TaskJSON:    mustTaskJSON(t, "task-fake"),
		Mode:        ModeFake,
		ConfirmReal: true,
	})
	if err == nil || err.Error() != "external worker requires real dispatch mode" {
		t.Fatalf("err = %v", err)
	}
	if worker.called {
		t.Fatal("external worker should not run in fake mode")
	}
}

func TestExternalWorkerRunnerRunsWorkerAndNormalizesMetadata(t *testing.T) {
	worker := &stubExternalWorker{result: &workers.RunResult{
		Worker: "codex",
		Events: []workers.WorkerEvent{{Type: workers.EventStdoutJSON, Payload: json.RawMessage(`{"ok":true}`)}},
		Artifacts: []artifacts.Artifact{
			{Path: "artifacts/stdout.jsonl", Kind: artifacts.KindEvents, Content: []byte("{\"ok\":true}\n")},
			{Path: "artifacts/stderr.log", Kind: artifacts.KindLog, Content: []byte{}},
		},
		Metadata: map[string]string{"provider": "test-provider", "model": "test-model"},
	}}
	result, err := NewExternalWorkerRunner(worker).Run(context.Background(), WorkerRunOptions{
		TaskJSON:     mustTaskJSON(t, "task-real-run"),
		TaskID:       "task-real-run",
		Mode:         ModeReal,
		ConfirmReal:  true,
		ModelProfile: "test-profile",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !worker.called || worker.spec.Task.ID != "task-real-run" || worker.spec.Workspace != "." || worker.spec.Prompt != "do real work" {
		t.Fatalf("worker called=%v spec=%+v", worker.called, worker.spec)
	}
	if result.Status == "" || result.ErrorMessage != "" {
		t.Fatalf("result = %+v", result)
	}
	if result.UsageMeta["provider"] != "test-provider" || result.UsageMeta["model_profile"] != "test-profile" || result.UsageMeta["artifact_count"] != 2 {
		t.Fatalf("usage meta = %+v", result.UsageMeta)
	}
	if len(result.Artifacts) != 2 || len(result.Events) != 1 || string(result.Events[0].Payload) != `{"ok":true}` {
		t.Fatalf("outputs = artifacts:%+v events:%+v", result.Artifacts, result.Events)
	}
}

func TestExternalWorkerRunnerReturnsFailedStatusOnWorkerError(t *testing.T) {
	worker := &stubExternalWorker{err: errors.New("worker failed")}
	result, err := NewExternalWorkerRunner(worker).Run(context.Background(), WorkerRunOptions{
		TaskJSON:    mustTaskJSON(t, "task-real-fail"),
		Mode:        ModeReal,
		ConfirmReal: true,
	})
	if err == nil {
		t.Fatal("expected worker error")
	}
	if !worker.called || result.ErrorMessage != "worker failed" {
		t.Fatalf("called=%v result=%+v err=%v", worker.called, result, err)
	}
}

func mustTaskJSON(t *testing.T, id string) string {
	t.Helper()
	task := tasks.Task{
		ID: id, Title: "Real", Domain: "general", Worker: "codex", Goal: "do real work", Mode: "read_only",
		Workspace: tasks.WorkspaceSpec{Strategy: "local_repo", Path: "."}, Memory: tasks.MemorySpec{Scope: "none"},
		AllowedPaths: []string{"/"}, ForbiddenPaths: []string{"/secrets"}, ExpectedOutputs: []string{"ok"},
		DefinitionOfDone: []string{"ok"},
	}
	data, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
