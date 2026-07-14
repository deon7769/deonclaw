package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/events"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

type externalWorkerRunner struct {
	worker workers.Worker
}

func NewExternalWorkerRunner(worker workers.Worker) WorkerRunner {
	return externalWorkerRunner{worker: worker}
}

func (r externalWorkerRunner) Run(ctx context.Context, opts WorkerRunOptions) (WorkerRunResult, error) {
	if r.worker == nil {
		return WorkerRunResult{}, fmt.Errorf("external worker is not configured")
	}
	if opts.Mode != ModeReal {
		return WorkerRunResult{}, fmt.Errorf("external worker requires real dispatch mode")
	}
	if !opts.ConfirmReal {
		return WorkerRunResult{}, fmt.Errorf("%s", BlockedRealModeConfirmationRequired)
	}
	task, err := decodeWorkerTask(opts.TaskJSON)
	if err != nil {
		return WorkerRunResult{}, err
	}
	workspace := strings.TrimSpace(task.Workspace.Path)
	start := time.Now()
	result, runErr := r.worker.Run(ctx, workers.RunSpec{Task: &task, Workspace: workspace, Prompt: task.Goal})
	durationMS := time.Since(start).Milliseconds()
	workerResult := WorkerRunResult{
		Status:     runs.StatusSucceeded,
		UsageMeta:  workerMetadata(result, opts),
		DurationMS: durationMS,
	}
	if result != nil {
		workerResult.Artifacts = append(workerResult.Artifacts, result.Artifacts...)
		workerResult.Events = workerEvents(result.Events, opts.RunID, start)
	}
	if runErr != nil {
		workerResult.Status = runs.StatusFailed
		workerResult.ErrorMessage = runErr.Error()
		return workerResult, runErr
	}
	return workerResult, nil
}

func workerEvents(workerEvents []workers.WorkerEvent, runID string, start time.Time) []events.Event {
	eventsOut := make([]events.Event, 0, len(workerEvents))
	for i, event := range workerEvents {
		eventType := events.EventType(strings.TrimSpace(event.Type))
		if eventType == "" {
			eventType = events.TypeWorkerMessage
		}
		eventsOut = append(eventsOut, events.Event{
			ID:        fmt.Sprintf("evt_%s_worker_%03d", runID, i+1),
			RunID:     runID,
			Type:      eventType,
			Timestamp: start.Add(time.Duration(i) * time.Nanosecond),
			Payload:   append([]byte(nil), event.Payload...),
		})
	}
	return eventsOut
}

func decodeWorkerTask(taskJSON string) (tasks.Task, error) {
	if strings.TrimSpace(taskJSON) == "" {
		return tasks.Task{}, fmt.Errorf("task json is required")
	}
	var task tasks.Task
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return tasks.Task{}, fmt.Errorf("decode task json: %w", err)
	}
	if err := tasks.Validate(&task); err != nil {
		return tasks.Task{}, err
	}
	return task, nil
}

func workerMetadata(result *workers.RunResult, opts WorkerRunOptions) map[string]any {
	meta := map[string]any{}
	if result != nil {
		for key, value := range result.Metadata {
			meta[key] = value
		}
		if strings.TrimSpace(result.Worker) != "" {
			meta["worker"] = result.Worker
		}
		meta["artifact_count"] = len(result.Artifacts)
		meta["event_count"] = len(result.Events)
	}
	if _, ok := meta["model_profile"]; !ok && strings.TrimSpace(opts.ModelProfile) != "" {
		meta["model_profile"] = opts.ModelProfile
	}
	return meta
}
