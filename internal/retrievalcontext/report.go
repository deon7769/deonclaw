package retrievalcontext

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

type RetrievalReportOptions struct {
	RunID string
}

type RetrievalReport struct {
	Runs []RetrievalRunEntry `json:"runs"`
}

type RetrievalRunEntry struct {
	RunID                    string   `json:"run_id"`
	TaskID                   string   `json:"task_id"`
	Status                   string   `json:"status"`
	RetrievalContextAttached bool     `json:"retrieval_context_attached"`
	RetrievalContextStatus   string   `json:"retrieval_context_status"`
	RetrievalContextCount    int      `json:"retrieval_context_count"`
	RetrievalContextSHA256   string   `json:"retrieval_context_sha256,omitempty"`
	ArtifactPaths            []string `json:"artifact_paths"`
}

type retrievalExecutionTrace struct {
	TaskID                   string `json:"task_id"`
	RetrievalContextAttached bool   `json:"retrieval_context_attached"`
	RetrievalContextCount    int    `json:"retrieval_context_count"`
	RetrievalContextSHA256   string `json:"retrieval_context_sha256"`
	RetrievalContextStatus   string `json:"retrieval_context_status"`
}

func BuildRetrievalReport(ctx context.Context, db store.Store, opts RetrievalReportOptions) (RetrievalReport, error) {
	runRecords, err := db.ListRuns(ctx)
	if err != nil {
		return RetrievalReport{}, err
	}
	sort.Slice(runRecords, func(i, j int) bool {
		return runRecords[i].CreatedAt.After(runRecords[j].CreatedAt)
	})

	report := RetrievalReport{Runs: []RetrievalRunEntry{}}
	for _, runRecord := range runRecords {
		if opts.RunID != "" && runRecord.ID != opts.RunID {
			continue
		}
		entry, include, err := retrievalRunEntry(ctx, db, runRecord)
		if err != nil {
			return RetrievalReport{}, err
		}
		if !include {
			continue
		}
		report.Runs = append(report.Runs, entry)
	}
	return report, nil
}

func retrievalRunEntry(ctx context.Context, db store.Store, runRecord runs.Run) (RetrievalRunEntry, bool, error) {
	trace, hasTrace, err := loadRetrievalExecutionTrace(ctx, db, runRecord.ID)
	if err != nil {
		return RetrievalRunEntry{}, false, err
	}
	if !hasTrace || !trace.RetrievalContextAttached {
		return RetrievalRunEntry{}, false, nil
	}

	artifactPaths, err := retrievalArtifactPaths(ctx, db, runRecord.ID)
	if err != nil {
		return RetrievalRunEntry{}, false, err
	}

	taskID := strings.TrimSpace(trace.TaskID)
	if taskID == "" {
		taskID = runRecord.TaskID
	}
	return RetrievalRunEntry{
		RunID:                    runRecord.ID,
		TaskID:                   taskID,
		Status:                   string(runRecord.Status),
		RetrievalContextAttached: trace.RetrievalContextAttached,
		RetrievalContextStatus:   trace.RetrievalContextStatus,
		RetrievalContextCount:    trace.RetrievalContextCount,
		RetrievalContextSHA256:   trace.RetrievalContextSHA256,
		ArtifactPaths:            artifactPaths,
	}, true, nil
}

func loadRetrievalExecutionTrace(ctx context.Context, db store.Store, runID string) (retrievalExecutionTrace, bool, error) {
	runArtifacts, err := db.ArtifactsByRun(ctx, runID)
	if err != nil {
		return retrievalExecutionTrace{}, false, err
	}
	for _, artifact := range runArtifacts {
		if artifactBaseName(artifact) != "execution-trace.json" {
			continue
		}
		content, err := os.ReadFile(artifact.Path)
		if err != nil {
			return retrievalExecutionTrace{}, false, nil
		}
		var trace retrievalExecutionTrace
		if err := json.Unmarshal(content, &trace); err != nil {
			return retrievalExecutionTrace{}, false, nil
		}
		return trace, true, nil
	}
	return retrievalExecutionTrace{}, false, nil
}

func retrievalArtifactPaths(ctx context.Context, db store.Store, runID string) ([]string, error) {
	runArtifacts, err := db.ArtifactsByRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, artifact := range runArtifacts {
		switch artifactBaseName(artifact) {
		case "retrieval-context.json", "retrieval-context.md":
			paths = append(paths, artifact.Path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func artifactBaseName(artifact artifacts.Artifact) string {
	name := filepath.Base(filepath.Clean(artifact.Path))
	if name == "." || name == string(filepath.Separator) {
		return ""
	}
	return name
}

func WriteRetrievalReportText(report RetrievalReport, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "runs_retrieval_report:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "runs: %d\n", len(report.Runs)); err != nil {
		return err
	}
	if len(report.Runs) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(out, "\nentries:"); err != nil {
		return err
	}
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "run_id\ttask_id\tstatus\tretrieval_context_status\tretrieval_context_count\tretrieval_context_sha256"); err != nil {
		return err
	}
	for _, entry := range report.Runs {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%d\t%s\n",
			entry.RunID,
			entry.TaskID,
			entry.Status,
			entry.RetrievalContextStatus,
			entry.RetrievalContextCount,
			entry.RetrievalContextSHA256,
		); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	for _, entry := range report.Runs {
		if len(entry.ArtifactPaths) == 0 {
			continue
		}
		if _, err := fmt.Fprintf(out, "\n%s artifact_paths:\n", entry.RunID); err != nil {
			return err
		}
		for _, path := range entry.ArtifactPaths {
			if _, err := fmt.Fprintf(out, "- %s\n", path); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteRetrievalReportJSON(report RetrievalReport, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
