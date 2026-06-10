package runner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

func writeCodexRunArtifacts(workerName string, runDir string, runID string, task *tasks.Task, result *workers.RunResult, status runs.RunStatus, runErr error, policySummary string, changedPathCount int, cleanup workspaceCleanup, diffPatch []byte, changedFiles []ChangedFile, validation ValidationResult, contextPackMarkdown []byte, contextPackWarnings []string, memoryProposal memoryProposalCheck, trace executionTraceOptions, createdAt time.Time) ([]artifacts.Artifact, error) {
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
	artifactCount := 10 + countWorkerArtifacts(result.Artifacts)
	if len(contextPackMarkdown) > 0 {
		artifactCount++
	}
	if len(memoryProposal.LintJSON) > 0 {
		artifactCount++
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
	if err := writer.write("validation-log", "validation.log", artifacts.KindLog, validationLog(validation)); err != nil {
		return nil, err
	}
	validationJSONData, err := validationJSON(validation)
	if err != nil {
		return nil, err
	}
	if err := writer.write("validation-json", "validation.json", artifacts.KindOther, validationJSONData); err != nil {
		return nil, err
	}
	if len(contextPackMarkdown) > 0 {
		if err := writer.write("context-pack", "context-pack.md", artifacts.KindOther, contextPackMarkdown); err != nil {
			return nil, err
		}
	}
	if len(memoryProposal.LintJSON) > 0 {
		if err := writer.write("memory-proposal-lint", "memory-proposal-lint.json", artifacts.KindOther, memoryProposal.LintJSON); err != nil {
			return nil, err
		}
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
	traceJSON, err := executionTraceJSON(trace)
	if err != nil {
		return nil, err
	}
	if err := writer.write("execution-trace", "execution-trace.json", artifacts.KindOther, traceJSON); err != nil {
		return nil, err
	}
	if err := writer.write("summary", "summary.md", artifacts.KindSummary, codexRunSummary(workerName, runID, task, result, status, runErr, policySummary, changedPathCount, cleanup, validation, trace.WorkerRuntime, contextPackWarnings, memoryProposal, artifactCount)); err != nil {
		return nil, err
	}
	manifest, err := artifactManifestJSON(writer.artifacts)
	if err != nil {
		return nil, err
	}
	if err := writer.write("artifact-manifest", "artifact-manifest.json", artifacts.KindOther, manifest); err != nil {
		return nil, err
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
	hash := sha256.Sum256(content)
	w.artifacts = append(w.artifacts, artifacts.Artifact{
		ID:        fmt.Sprintf("%s-artifact-%s", w.runID, idSuffix),
		RunID:     w.runID,
		Path:      path,
		Kind:      kind,
		SizeBytes: int64(len(content)),
		SHA256:    hex.EncodeToString(hash[:]),
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
	case "stdout.jsonl", "stderr.log", "events.jsonl", "diff.patch", "changed-files.json", "validation.log", "validation.json", "context-pack.md", "memory-proposal-lint.json", "execution-trace.json", "summary.md", "artifact-manifest.json":
		return true
	default:
		return false
	}
}

func countWorkerArtifacts(workerArtifacts []artifacts.Artifact) int {
	var count int
	for _, artifact := range workerArtifacts {
		name := artifactFileName(artifact.Path)
		if name == "" || isCLIOwnedArtifact(name) {
			continue
		}
		count++
	}
	return count
}

type artifactManifest struct {
	Artifacts []artifactManifestEntry `json:"artifacts"`
}

type artifactManifestEntry struct {
	Path      string         `json:"path"`
	Kind      artifacts.Kind `json:"kind"`
	SizeBytes int64          `json:"size_bytes"`
	SHA256    string         `json:"sha256"`
}

func artifactManifestJSON(runArtifacts []artifacts.Artifact) ([]byte, error) {
	manifest := artifactManifest{
		Artifacts: make([]artifactManifestEntry, 0, len(runArtifacts)),
	}
	for _, artifact := range runArtifacts {
		manifest.Artifacts = append(manifest.Artifacts, artifactManifestEntry{
			Path:      artifact.Path,
			Kind:      artifact.Kind,
			SizeBytes: artifact.SizeBytes,
			SHA256:    artifact.SHA256,
		})
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
