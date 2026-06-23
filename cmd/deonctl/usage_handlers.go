package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/store"
	usagepkg "github.com/deon7769/deonclaw/internal/usage"
)

func runUsageRecord(storePath, inputPath string, stdout io.Writer, stderr io.Writer) int {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(stderr, "usage record failed: %v\n", err)
		return 1
	}
	var event usagepkg.Event
	if err := json.Unmarshal(data, &event); err != nil {
		fmt.Fprintf(stderr, "usage record failed: %v\n", err)
		return 1
	}
	if err := usagepkg.ValidateEvent(event); err != nil {
		fmt.Fprintf(stderr, "usage record failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "usage record failed: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := db.SaveUsageEvent(context.Background(), event); err != nil {
		fmt.Fprintf(stderr, "usage record failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "usage record: ok")
	return 0
}

func runUsageShow(storePath, id string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "usage show failed: %v\n", err)
		return 1
	}
	defer db.Close()
	event, err := db.UsageEvent(context.Background(), id)
	if err != nil {
		fmt.Fprintf(stderr, "usage show failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(event)
	return 0
}

func runUsageReport(storePath, groupBy, since string, stdout io.Writer, stderr io.Writer) int {
	if err := usagepkg.ValidateReportGroupBy(groupBy); err != nil {
		fmt.Fprintf(stderr, "usage report failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "usage report failed: %v\n", err)
		return 1
	}
	defer db.Close()
	var sinceTime *time.Time
	if strings.TrimSpace(since) != "" {
		parsed, err := time.Parse(time.RFC3339, since)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", since)
			if err != nil {
				fmt.Fprintf(stderr, "usage report failed: %v\n", err)
				return 1
			}
		}
		sinceTime = &parsed
	}
	events, err := db.ListUsageEvents(context.Background(), sinceTime)
	if err != nil {
		fmt.Fprintf(stderr, "usage report failed: %v\n", err)
		return 1
	}
	report := usagepkg.BuildReport(events, groupBy)
	if sinceTime != nil {
		report.Since = sinceTime.Format(time.RFC3339)
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
	return 0
}
