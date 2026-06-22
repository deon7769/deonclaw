package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/deon7769/deonclaw/internal/store"
	"github.com/deon7769/deonclaw/internal/workqueue"
)

func runWorkList(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work list failed: %v\n", err)
		return 1
	}
	defer db.Close()
	items, err := db.ListWorkItems(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "work list failed: %v\n", err)
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "id\tstatus\tagent\tpriority\tattempt")
	for _, item := range items {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%d\t%d\n", item.ID, item.Status, item.AssignedAgentID, item.Priority, item.Attempt)
	}
	_ = writer.Flush()
	return 0
}

func runWorkShow(storePath string, workItemID string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work show failed: %v\n", err)
		return 1
	}
	defer db.Close()
	item, err := db.WorkItem(context.Background(), workItemID)
	if err != nil {
		fmt.Fprintf(stderr, "work show failed: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(item)
	return 0
}

type workClaimOptions struct {
	storePath  string
	agentID    string
	workItemID string
	ttlSeconds int
}

func runWorkClaim(opts workClaimOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work claim failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ttl := time.Duration(opts.ttlSeconds) * time.Second
	if opts.ttlSeconds == 0 {
		ttl = workqueue.DefaultLeaseTTLSeconds * time.Second
	}
	result, err := db.ClaimWorkItem(context.Background(), opts.agentID, opts.workItemID, ttl, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "work claim failed: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
	if !result.Claimed {
		return 1
	}
	return 0
}

type workReleaseOptions struct {
	storePath string
	leaseID   string
	reason    string
	requeue   bool
}

func runWorkRelease(opts workReleaseOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work release failed: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := db.ReleaseLease(context.Background(), opts.leaseID, opts.reason, opts.requeue, time.Now().UTC()); err != nil {
		fmt.Fprintf(stderr, "work release failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "work release: ok lease=%s requeue=%t\n", opts.leaseID, opts.requeue)
	return 0
}

func runWorkRecover(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work recover failed: %v\n", err)
		return 1
	}
	defer db.Close()
	now := time.Now().UTC()
	report, err := db.RecoverWorkQueue(context.Background(), now)
	if err != nil {
		fmt.Fprintf(stderr, "work recover failed: %v\n", err)
		return 1
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(report)
	if report.Status != "ok" {
		return 1
	}
	return 0
}

func runWorkDoctor(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work doctor failed: %v\n", err)
		return 1
	}
	defer db.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	recovery, _ := db.RecoverWorkQueue(ctx, now)
	items, err := db.ListWorkItems(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "work doctor failed: %v\n", err)
		return 1
	}
	leases, err := db.ListLeases(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "work doctor failed: %v\n", err)
		return 1
	}
	report := workqueue.Doctor(items, leases, recovery)
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(report)
	if report.Status != "ok" {
		return 1
	}
	return 0
}

func runWorkLeasesList(storePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		return 1
	}
	defer db.Close()
	leases, err := db.ListLeases(context.Background())
	if err != nil {
		return 1
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "id\twork_item\tagent\tstatus\texpires_at")
	for _, lease := range leases {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", lease.ID, lease.WorkItemID, lease.AgentID, lease.Status, lease.ExpiresAt)
	}
	_ = writer.Flush()
	return 0
}

func parseWorkStoreArg(args []string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--store" {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for --store")
			}
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("missing --store")
}

func parseWorkClaimOptions(args []string) (workClaimOptions, error) {
	var opts workClaimOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return workClaimOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--agent":
			if i+1 >= len(args) {
				return workClaimOptions{}, fmt.Errorf("missing value for --agent")
			}
			opts.agentID = args[i+1]
			i++
		case "--work-item":
			if i+1 >= len(args) {
				return workClaimOptions{}, fmt.Errorf("missing value for --work-item")
			}
			opts.workItemID = args[i+1]
			i++
		case "--ttl-seconds":
			if i+1 >= len(args) {
				return workClaimOptions{}, fmt.Errorf("missing value for --ttl-seconds")
			}
			var n int
			if _, err := fmt.Sscanf(args[i+1], "%d", &n); err != nil {
				return workClaimOptions{}, fmt.Errorf("invalid --ttl-seconds")
			}
			opts.ttlSeconds = n
			i++
		default:
			return workClaimOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return workClaimOptions{}, fmt.Errorf("missing --store")
	}
	if opts.agentID == "" && strings.TrimSpace(opts.workItemID) == "" {
		return workClaimOptions{}, fmt.Errorf("missing --agent or --work-item")
	}
	return opts, nil
}

func parseWorkReleaseOptions(args []string) (workReleaseOptions, error) {
	var opts workReleaseOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return workReleaseOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--lease":
			if i+1 >= len(args) {
				return workReleaseOptions{}, fmt.Errorf("missing value for --lease")
			}
			opts.leaseID = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return workReleaseOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--requeue":
			opts.requeue = true
		default:
			return workReleaseOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" || opts.leaseID == "" {
		return workReleaseOptions{}, fmt.Errorf("missing --store or --lease")
	}
	return opts, nil
}
