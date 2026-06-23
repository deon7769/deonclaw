package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/budget"
	"github.com/deon7769/deonclaw/internal/store"
	usagepkg "github.com/deon7769/deonclaw/internal/usage"
)

func runBudgetsValidate(configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := budget.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets validate failed: %v\n", err)
		return 1
	}
	if err := budget.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "budgets validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "budgets validate: ok")
	return 0
}

func runBudgetsSync(storePath, configPath string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := budget.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets sync failed: %v\n", err)
		return 1
	}
	if err := budget.ValidateConfig(cfg); err != nil {
		fmt.Fprintf(stderr, "budgets sync failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets sync failed: %v\n", err)
		return 1
	}
	defer db.Close()
	policies := make([]budget.Policy, 0, len(cfg.BudgetPolicies))
	for _, policy := range cfg.BudgetPolicies {
		policies = append(policies, policy)
	}
	if err := db.SyncBudgetPolicies(context.Background(), policies); err != nil {
		fmt.Fprintf(stderr, "budgets sync failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "budgets sync: ok")
	return 0
}

func runBudgetsStatus(storePath, agentID string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets status failed: %v\n", err)
		return 1
	}
	defer db.Close()
	report, err := db.BudgetStatus(context.Background(), agentID)
	if err != nil {
		fmt.Fprintf(stderr, "budgets status failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
	return 0
}

func runBudgetsPlan(configPath, agentID string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := budget.LoadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets plan failed: %v\n", err)
		return 1
	}
	report := budget.BuildPlanReport(cfg, agentID)
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
	return 0
}

type budgetReserveOptions struct {
	storePath   string
	workItemID  string
	policyID    string
	estimateUSD float64
}

func runBudgetsReserve(opts budgetReserveOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets reserve failed: %v\n", err)
		return 1
	}
	defer db.Close()
	estimateMicro := int64(opts.estimateUSD * 1_000_000)
	result, err := db.ReserveBudget(context.Background(), budget.ReserveBudgetOptions{
		PolicyID: opts.policyID, WorkItemID: opts.workItemID, EstimatedMicroUSD: estimateMicro, Now: time.Now().UTC(),
	})
	if err != nil {
		fmt.Fprintf(stderr, "budgets reserve failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(result)
	return 0
}

func runBudgetsCommit(storePath, reservationID, usagePath string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets commit failed: %v\n", err)
		return 1
	}
	defer db.Close()
	data, err := os.ReadFile(usagePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets commit failed: %v\n", err)
		return 1
	}
	var event usagepkg.Event
	if err := json.Unmarshal(data, &event); err != nil {
		fmt.Fprintf(stderr, "budgets commit failed: %v\n", err)
		return 1
	}
	actual := event.ActualCostMicroUSD
	if actual == 0 {
		actual = event.EstimatedCostMicroUSD
	}
	reservation, err := db.CommitReservation(context.Background(), reservationID, event, actual, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "budgets commit failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(reservation)
	return 0
}

func runBudgetsRelease(storePath, reservationID, reason string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets release failed: %v\n", err)
		return 1
	}
	defer db.Close()
	reservation, err := db.ReleaseReservation(context.Background(), reservationID, reason, time.Now().UTC())
	if err != nil {
		fmt.Fprintf(stderr, "budgets release failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(reservation)
	return 0
}

func runBudgetsReport(storePath, agentID string, stdout io.Writer, stderr io.Writer) int {
	return runBudgetsStatus(storePath, agentID, stdout, stderr)
}

func runBudgetsOverrideNew(configPath, outputPath string, args []string, stdout io.Writer, stderr io.Writer) int {
	policyID := flagValue(args, "--policy")
	agentID := flagValue(args, "--agent")
	workItemID := flagValue(args, "--work-item")
	reviewer := flagValue(args, "--reviewer")
	reason := flagValue(args, "--reason")
	microStr := flagValue(args, "--requested-microusd")
	micro, _ := strconv.ParseInt(microStr, 10, 64)
	if micro <= 0 {
		micro = 1_000_000
	}
	approval, err := budget.BuildOverrideApproval(budget.NewOverrideApprovalOptions{
		PolicyID: policyID, AgentID: agentID, WorkItemID: workItemID, RequestedMicroUSD: micro, Reviewer: reviewer, Reason: reason,
	})
	if err != nil {
		fmt.Fprintf(stderr, "budgets override new failed: %v\n", err)
		return 1
	}
	_ = configPath
	data, _ := json.MarshalIndent(approval, "", "  ")
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "budgets override new failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "budgets override new: ok")
	return 0
}

func runBudgetsOverrideApprove(storePath, inputPath string, stdout io.Writer, stderr io.Writer) int {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets override approve failed: %v\n", err)
		return 1
	}
	var approval budget.OverrideApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		fmt.Fprintf(stderr, "budgets override approve failed: %v\n", err)
		return 1
	}
	if err := budget.ValidateOverrideApproval(approval); err != nil {
		fmt.Fprintf(stderr, "budgets override approve failed: %v\n", err)
		return 1
	}
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets override approve failed: %v\n", err)
		return 1
	}
	defer db.Close()
	if err := db.SaveBudgetOverrideApproval(context.Background(), approval); err != nil {
		fmt.Fprintf(stderr, "budgets override approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "budgets override approve: ok")
	return 0
}

func runBudgetsOverrideInspect(storePath, id string, stdout io.Writer, stderr io.Writer) int {
	db, err := store.OpenSQLite(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "budgets override inspect failed: %v\n", err)
		return 1
	}
	defer db.Close()
	approval, err := db.BudgetOverrideApproval(context.Background(), id)
	if err != nil {
		fmt.Fprintf(stderr, "budgets override inspect failed: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(approval)
	return 0
}

func flagValue(args []string, name string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == name && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
	}
	return ""
}
