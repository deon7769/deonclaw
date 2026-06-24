package dispatch

import (
	"context"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/runs"
)

func FakeWorkerRun(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
	_ = ctx
	taskID := strings.TrimSpace(opts.TaskID)
	switch {
	case strings.HasSuffix(taskID, "fake-failure"):
		return WorkerRunResult{Status: runs.StatusFailed, ErrorMessage: "fake worker failure"}, nil
	case strings.HasSuffix(taskID, "fake-long"):
		time.Sleep(25 * time.Millisecond)
		return WorkerRunResult{Status: runs.StatusSucceeded, DurationMS: 25000}, nil
	case strings.HasSuffix(taskID, "fake-usage-known"):
		return WorkerRunResult{
			Status:     runs.StatusSucceeded,
			DurationMS: 1200,
			UsageMeta: map[string]any{
				"provider":      "z-ai",
				"model":         "glm-5.1",
				"input_tokens":  12000,
				"output_tokens": 2300,
				"tool_calls":    2,
			},
		}, nil
	default:
		return WorkerRunResult{Status: runs.StatusSucceeded, DurationMS: 500}, nil
	}
}

func NewFakeWorkerRunner() WorkerRunner {
	return workerFunc(FakeWorkerRun)
}

type workerFunc func(context.Context, WorkerRunOptions) (WorkerRunResult, error)

func (f workerFunc) Run(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
	return f(ctx, opts)
}
