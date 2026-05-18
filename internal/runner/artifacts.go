package runner

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/artifacts"
	"github.com/deon7769/deonclaw/internal/runs"
	"github.com/deon7769/deonclaw/internal/tasks"
	"github.com/deon7769/deonclaw/internal/workers"
)

func writeCodexRunArtifacts(runDir string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup, diffPatch []byte, changedFiles []ChangedFile, createdAt time.Time) ([]artifacts.Artifact, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}

	rawStdout, _ := workerArtifactContent(result.Artifacts, "stdout.jsonl")
	rawStderr, ok := workerArtifactContent(result.Artifacts, "stderr.log")
	if !ok {
		rawStderr = []byte(result.Stderr)
	}
	if cleanup.Warning != "" {
		rawStderr = append(rawStderr, []byte(fmt.Sprintf("workspace cleanup warning: %s\n", cleanup.Warning))...)
	}

	writer := runArtifactWriter{
		runDir:    runDir,
		runID:     runID,
		createdAt: createdAt,
		written:   make(map[string]struct{}),
	}
	if err := writer.write("stdout", "stdout.jsonl", artifacts.KindEvents, rawStdout); err != nil {
		return nil, err
	}
	if err := writer.write("events", "events.jsonl", artifacts.KindEvents, workerEventsJSONL(result.Events)); err != nil {
		return nil, err
	}
	if err := writer.write("stderr", "stderr.log", artifacts.KindLog, rawStderr); err != nil {
		return nil, err
	}
	if err := writer.write("diff", "diff.patch", artifacts.KindDiff, diffPatch); err != nil {
		return nil, err
	}
	if changedFiles == nil {
		changedFiles = []ChangedFile{}
	}
	changedFilesJSON, err := json.MarshalIndent(changedFiles, "", "  ")
	if err != nil {
		return nil, err
	}
	changedFilesJSON = append(changedFilesJSON, '\n')
	if err := writer.write("changed-files", "changed-files.json", artifacts.KindOther, changedFilesJSON); err != nil {
		return nil, err
	}
	if err := writer.write("summary", "summary.md", artifacts.KindSummary, codexRunSummary(runID, task, result, status, runErr, policySummary, changedPathCount, cleanup)); err != nil {
		return nil, err
	}

	for i, artifact := range result.Artifacts {
		name := artifactFileName(artifact.Path)
		if name == "" || isCLIOwnedArtifact(name) {
			continue
		}
		kind := artifact.Kind
		if kind == "" {
			kind = artifacts.KindOther
		}
		if err := writer.write(fmt.Sprintf("worker-%03d", i+1), name, kind, artifact.Content); err != nil {
			return nil, err
		}
	}
	return writer.artifacts, nil
}

func workerEventsJSONL(workerEvents []workers.WorkerEvent) []byte {
	var output bytes.Buffer
	for _, event := range workerEvents {
		line := event.Payload
		if len(line) == 0 {
			marshaled, err := json.Marshal(event)
			if err != nil {
				continue
			}
			line = marshaled
		}
		output.Write(line)
		output.WriteByte('\n')
	}
	return output.Bytes()
}

type runArtifactWriter struct {
	runDir    string
	runID     string
	createdAt time.Time
	written   map[string]struct{}
	artifacts []artifacts.Artifact
}

func (w *runArtifactWriter) write(idSuffix string, name string, kind artifacts.Kind, content []byte) error {
	name = w.uniqueName(name)
	path := filepath.Join(w.runDir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return err
	}
	w.artifacts = append(w.artifacts, artifacts.Artifact{
		ID:        fmt.Sprintf("%s-artifact-%s", w.runID, idSuffix),
		RunID:     w.runID,
		Path:      path,
		Kind:      kind,
		CreatedAt: w.createdAt,
	})
	return nil
}

func (w *runArtifactWriter) uniqueName(name string) string {
	if _, ok := w.written[name]; !ok {
		w.written[name] = struct{}{}
		return name
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", stem, i, ext)
		if _, ok := w.written[candidate]; !ok {
			w.written[candidate] = struct{}{}
			return candidate
		}
	}
}

func workerArtifactContent(workerArtifacts []artifacts.Artifact, name string) ([]byte, bool) {
	for _, artifact := range workerArtifacts {
		if artifactFileName(artifact.Path) == name {
			return append([]byte(nil), artifact.Content...), true
		}
	}
	return nil, false
}

func artifactFileName(path string) string {
	name := filepath.Base(filepath.Clean(path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func isCLIOwnedArtifact(name string) bool {
	switch name {
	case "stdout.jsonl", "stderr.log", "events.jsonl", "diff.patch", "changed-files.json", "summary.md":
		return true
	default:
		return false
	}
}
