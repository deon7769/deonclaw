package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/insights"
	storepkg "github.com/deon7769/deonclaw/internal/store"
)

type insightsPolicyValidateOptions struct {
	configPath string
}

type insightsEvidenceBuildOptions struct {
	storePath  string
	runID      string
	outputPath string
	trigger    string
	policyRef  string
}

type insightsTriggerEvaluateOptions struct {
	configPath   string
	evidencePath string
	outputFormat string
}

type insightsEvaluateDryRunOptions struct {
	evidencePath string
	reviewer     string
	promptOutput string
	planOutput   string
}

type insightsEvaluateOptions struct {
	evidencePath string
	reviewer     string
	responsePath string
	outputPath   string
}

type insightsReportOptions struct {
	insightPath  string
	outputFormat string
}

func parseInsightsPolicyValidateOptions(args []string) (insightsPolicyValidateOptions, error) {
	var opts insightsPolicyValidateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return insightsPolicyValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		default:
			return insightsPolicyValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return insightsPolicyValidateOptions{}, fmt.Errorf("missing --config")
	}
	return opts, nil
}

func parseInsightsEvidenceBuildOptions(args []string) (insightsEvidenceBuildOptions, error) {
	var opts insightsEvidenceBuildOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--store":
			if i+1 >= len(args) {
				return insightsEvidenceBuildOptions{}, fmt.Errorf("missing value for --store")
			}
			opts.storePath = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return insightsEvidenceBuildOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return insightsEvidenceBuildOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--trigger":
			if i+1 >= len(args) {
				return insightsEvidenceBuildOptions{}, fmt.Errorf("missing value for --trigger")
			}
			opts.trigger = args[i+1]
			i++
		case "--policy-ref":
			if i+1 >= len(args) {
				return insightsEvidenceBuildOptions{}, fmt.Errorf("missing value for --policy-ref")
			}
			opts.policyRef = args[i+1]
			i++
		default:
			return insightsEvidenceBuildOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.storePath == "" {
		return insightsEvidenceBuildOptions{}, fmt.Errorf("missing --store")
	}
	if opts.runID == "" {
		return insightsEvidenceBuildOptions{}, fmt.Errorf("missing --run")
	}
	if opts.outputPath == "" {
		return insightsEvidenceBuildOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseInsightsTriggerEvaluateOptions(args []string) (insightsTriggerEvaluateOptions, error) {
	opts := insightsTriggerEvaluateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return insightsTriggerEvaluateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--evidence":
			if i+1 >= len(args) {
				return insightsTriggerEvaluateOptions{}, fmt.Errorf("missing value for --evidence")
			}
			opts.evidencePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return insightsTriggerEvaluateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return insightsTriggerEvaluateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return insightsTriggerEvaluateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.evidencePath == "" {
		return insightsTriggerEvaluateOptions{}, fmt.Errorf("missing --evidence")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return insightsTriggerEvaluateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runInsightsPolicyValidate(opts insightsPolicyValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := insights.LoadPolicy(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "insights policy validate failed: %v\n", err)
		return 1
	}
	if err := insights.ValidatePolicy(cfg); err != nil {
		fmt.Fprintf(stderr, "insights policy validate failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "insights policy validate: ok")
	return 0
}

func runInsightsEvidenceBuild(opts insightsEvidenceBuildOptions, stdout io.Writer, stderr io.Writer) int {
	db, err := storepkg.OpenSQLite(opts.storePath)
	if err != nil {
		fmt.Fprintf(stderr, "open store failed: %v\n", err)
		return 1
	}
	defer db.Close()

	bundle, err := insights.BuildEvidenceFromRun(context.Background(), db, insights.BuildEvidenceOptions{
		RunID:     opts.runID,
		Trigger:   opts.trigger,
		PolicyRef: opts.policyRef,
	})
	if err != nil {
		fmt.Fprintf(stderr, "insights evidence build failed: %v\n", err)
		return 1
	}
	if err := insights.WriteEvidenceJSON(bundle, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "insights evidence build failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "insights evidence build: ok evidence_bundle_id=%s sha256=%s\n", bundle.EvidenceBundleID, bundle.SHA256)
	return 0
}

func runInsightsTriggerEvaluate(opts insightsTriggerEvaluateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := insights.LoadPolicy(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "insights trigger evaluate failed: %v\n", err)
		return 1
	}
	if err := insights.ValidatePolicy(cfg); err != nil {
		fmt.Fprintf(stderr, "insights trigger evaluate failed: %v\n", err)
		return 1
	}

	bundle, err := insights.ReadEvidenceJSON(opts.evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "insights trigger evaluate failed: %v\n", err)
		return 1
	}

	decision, err := insights.EvaluateTrigger(cfg.InsightPolicy, bundle.Trigger, insights.TriggerCounters{
		RunsSinceLastInsight:    1,
		CommitsSinceLastInsight: 0,
		TurnsSinceLastInsight:   0,
	})
	if err != nil {
		fmt.Fprintf(stderr, "insights trigger evaluate failed: %v\n", err)
		return 1
	}

	switch opts.outputFormat {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(decision); err != nil {
			fmt.Fprintf(stderr, "insights trigger evaluate failed: %v\n", err)
			return 1
		}
	default:
		fmt.Fprintf(stdout, "insights trigger evaluate:\n")
		fmt.Fprintf(stdout, "  should_evaluate: %t\n", decision.ShouldEvaluate)
		fmt.Fprintf(stdout, "  trigger: %s\n", decision.Trigger)
		fmt.Fprintf(stdout, "  reason: %s\n", decision.Reason)
		fmt.Fprintf(stdout, "  policy_enabled: %t\n", decision.PolicyEnabled)
	}
	return 0
}

func parseInsightsEvaluateDryRunOptions(args []string) (insightsEvaluateDryRunOptions, error) {
	var opts insightsEvaluateDryRunOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--evidence":
			if i+1 >= len(args) {
				return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing value for --evidence")
			}
			opts.evidencePath = args[i+1]
			i++
		case "--reviewer":
			if i+1 >= len(args) {
				return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing value for --reviewer")
			}
			opts.reviewer = args[i+1]
			i++
		case "--prompt-output":
			if i+1 >= len(args) {
				return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing value for --prompt-output")
			}
			opts.promptOutput = args[i+1]
			i++
		case "--plan-output":
			if i+1 >= len(args) {
				return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing value for --plan-output")
			}
			opts.planOutput = args[i+1]
			i++
		default:
			return insightsEvaluateDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.evidencePath == "" {
		return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing --evidence")
	}
	if opts.reviewer == "" {
		return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing --reviewer")
	}
	if opts.promptOutput == "" {
		return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing --prompt-output")
	}
	if opts.planOutput == "" {
		return insightsEvaluateDryRunOptions{}, fmt.Errorf("missing --plan-output")
	}
	return opts, nil
}

func parseInsightsEvaluateOptions(args []string) (insightsEvaluateOptions, error) {
	var opts insightsEvaluateOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--evidence":
			if i+1 >= len(args) {
				return insightsEvaluateOptions{}, fmt.Errorf("missing value for --evidence")
			}
			opts.evidencePath = args[i+1]
			i++
		case "--reviewer":
			if i+1 >= len(args) {
				return insightsEvaluateOptions{}, fmt.Errorf("missing value for --reviewer")
			}
			opts.reviewer = args[i+1]
			i++
		case "--response":
			if i+1 >= len(args) {
				return insightsEvaluateOptions{}, fmt.Errorf("missing value for --response")
			}
			opts.responsePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return insightsEvaluateOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return insightsEvaluateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.evidencePath == "" {
		return insightsEvaluateOptions{}, fmt.Errorf("missing --evidence")
	}
	if opts.reviewer == "" {
		return insightsEvaluateOptions{}, fmt.Errorf("missing --reviewer")
	}
	if opts.responsePath == "" {
		return insightsEvaluateOptions{}, fmt.Errorf("missing --response")
	}
	if opts.outputPath == "" {
		return insightsEvaluateOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseInsightsReportOptions(args []string) (insightsReportOptions, error) {
	opts := insightsReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--insight":
			if i+1 >= len(args) {
				return insightsReportOptions{}, fmt.Errorf("missing value for --insight")
			}
			opts.insightPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return insightsReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return insightsReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.insightPath == "" {
		return insightsReportOptions{}, fmt.Errorf("missing --insight")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return insightsReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runInsightsEvaluateDryRun(opts insightsEvaluateDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	bundle, err := insights.ReadEvidenceJSON(opts.evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "insights evaluate dry-run failed: %v\n", err)
		return 1
	}
	plan, prompt, err := insights.PlanEvaluation(bundle, opts.reviewer)
	if err != nil {
		fmt.Fprintf(stderr, "insights evaluate dry-run failed: %v\n", err)
		return 1
	}
	promptSHA, err := insights.WriteEvaluationPrompt(prompt, opts.promptOutput)
	if err != nil {
		fmt.Fprintf(stderr, "insights evaluate dry-run failed: %v\n", err)
		return 1
	}
	plan.PromptPath = opts.promptOutput
	plan.PromptSHA256 = promptSHA
	if err := insights.WriteEvaluationPlan(plan, opts.planOutput); err != nil {
		fmt.Fprintf(stderr, "insights evaluate dry-run failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "insights evaluate dry-run: ok reviewer=%s worker_execution=false prompt_sha256=%s\n", plan.Reviewer, plan.PromptSHA256)
	return 0
}

func runInsightsEvaluate(opts insightsEvaluateOptions, stdout io.Writer, stderr io.Writer) int {
	bundle, err := insights.ReadEvidenceJSON(opts.evidencePath)
	if err != nil {
		fmt.Fprintf(stderr, "insights evaluate failed: %v\n", err)
		return 1
	}
	report, err := insights.MaterializeInsightReport(insights.EvaluateOptions{
		Evidence:     bundle,
		Reviewer:     opts.reviewer,
		ResponsePath: opts.responsePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "insights evaluate failed: %v\n", err)
		return 1
	}
	if err := insights.WriteReportJSON(report, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "insights evaluate failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "insights evaluate: ok insight_id=%s sha256=%s proposal_count=%d\n", report.InsightID, report.SHA256, report.ProposalCount)
	return 0
}

func runInsightsReport(opts insightsReportOptions, stdout io.Writer, stderr io.Writer) int {
	report, err := insights.ReadReportJSON(opts.insightPath)
	if err != nil {
		fmt.Fprintf(stderr, "insights report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "insights report failed: %v\n", err)
			return 1
		}
	default:
		if err := insights.WriteReportText(report, stdout); err != nil {
			fmt.Fprintf(stderr, "insights report failed: %v\n", err)
			return 1
		}
	}
	return 0
}
