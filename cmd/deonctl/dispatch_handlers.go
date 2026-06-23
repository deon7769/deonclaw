package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/dispatch"
	"github.com/deon7769/deonclaw/internal/store"
)

type workDispatchOnceOptions struct {
	storePath             string
	workItemID            string
	leaseID               string
	mode                  string
	confirmWorkerDispatch bool
}

func runWorkDispatchOnce(opts workDispatchOnceOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "work dispatch-once failed: %v\n", err)
		return 1
	}
	defer db.Close()
	mode := strings.TrimSpace(opts.mode)
	if mode == "" {
		mode = dispatch.ModeFake
	}
	svc := dispatch.Service{Repo: db}
	result, err := svc.DispatchOnce(context.Background(), dispatch.OnceOptions{
		WorkItemID:            opts.workItemID,
		LeaseID:               opts.leaseID,
		Mode:                  mode,
		ConfirmWorkerDispatch: opts.confirmWorkerDispatch,
		CodexRunner:           dispatch.NewFakeWorkerRunner(),
		OpenCodeRunner:        dispatch.NewFakeWorkerRunner(),
		PricingLoader:         dispatch.DefaultPricingLoader,
		Now:                   time.Now().UTC(),
	})
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	if err != nil && result.Status != "budget_blocked" {
		fmt.Fprintf(stderr, "work dispatch-once failed: %v\n", err)
		return 1
	}
	if result.Status == "failed" {
		return 1
	}
	return 0
}

type usageReportOptions struct {
	storePath string
	groupBy   string
	since     string
}

func parseConfigFlag(args []string, flag string) (string, error) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for %s", flag)
			}
			return args[i+1], nil
		}
	}
	return "", fmt.Errorf("%s is required", flag)
}

func parseStoreConfigFlags(args []string) (string, string, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return "", "", err
	}
	configPath, err := parseConfigFlag(args, "--config")
	if err != nil {
		return "", "", err
	}
	return storePath, configPath, nil
}

func parseStoreInputFlags(args []string) (string, string, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return "", "", err
	}
	inputPath, err := parseConfigFlag(args, "--input")
	if err != nil {
		return "", "", err
	}
	return storePath, inputPath, nil
}

func parseStoreAgentFlags(args []string) (string, string, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return "", "", err
	}
	agentID, _ := parseOptionalFlag(args, "--agent")
	return storePath, agentID, nil
}

func parseConfigAgentFlags(args []string) (string, string, error) {
	configPath, err := parseConfigFlag(args, "--config")
	if err != nil {
		return "", "", err
	}
	agentID, _ := parseOptionalFlag(args, "--agent")
	return configPath, agentID, nil
}

func parseUsageReportOptions(args []string) (usageReportOptions, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return usageReportOptions{}, err
	}
	groupBy, err := parseConfigFlag(args, "--by")
	if err != nil {
		return usageReportOptions{}, err
	}
	since, _ := parseOptionalFlag(args, "--since")
	return usageReportOptions{storePath: storePath, groupBy: groupBy, since: since}, nil
}

func parseBudgetReserveOptions(args []string) (budgetReserveOptions, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return budgetReserveOptions{}, err
	}
	workItemID, err := parseConfigFlag(args, "--work-item")
	if err != nil {
		return budgetReserveOptions{}, err
	}
	policyID, err := parseConfigFlag(args, "--policy")
	if err != nil {
		return budgetReserveOptions{}, err
	}
	estimate := 0.25
	if v, ok := parseOptionalFlag(args, "--estimate-usd"); ok && strings.TrimSpace(v) != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			estimate = parsed
		}
	}
	return budgetReserveOptions{storePath: storePath, workItemID: workItemID, policyID: policyID, estimateUSD: estimate}, nil
}

func parseBudgetCommitOptions(args []string) (string, string, string, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return "", "", "", err
	}
	reservationID, err := parseConfigFlag(args, "--reservation")
	if err != nil {
		return "", "", "", err
	}
	usagePath, err := parseConfigFlag(args, "--usage")
	if err != nil {
		return "", "", "", err
	}
	return storePath, reservationID, usagePath, nil
}

func parseBudgetReleaseOptions(args []string) (string, string, string, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return "", "", "", err
	}
	reservationID, err := parseConfigFlag(args, "--reservation")
	if err != nil {
		return "", "", "", err
	}
	reason, _ := parseOptionalFlag(args, "--reason")
	return storePath, reservationID, reason, nil
}

func parseOverrideNewOptions(args []string) (string, string, []string, error) {
	configPath, _ := parseOptionalFlag(args, "--config")
	outputPath, err := parseConfigFlag(args, "--output")
	if err != nil {
		return "", "", nil, err
	}
	return configPath, outputPath, args, nil
}

func parseWorkDispatchOnceOptions(args []string) (workDispatchOnceOptions, error) {
	storePath, err := parseWorkStoreArg(args)
	if err != nil {
		return workDispatchOnceOptions{}, err
	}
	workItemID, err := parseConfigFlag(args, "--work-item")
	if err != nil {
		return workDispatchOnceOptions{}, err
	}
	leaseID, _ := parseOptionalFlag(args, "--lease")
	mode, _ := parseOptionalFlag(args, "--mode")
	_, confirm := parseOptionalFlag(args, "--confirm-worker-dispatch")
	return workDispatchOnceOptions{
		storePath: storePath, workItemID: workItemID, leaseID: leaseID, mode: mode, confirmWorkerDispatch: confirm,
	}, nil
}

func parseOptionalFlag(args []string, flag string) (string, bool) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				return args[i+1], true
			}
			return "", true
		}
	}
	return "", false
}
