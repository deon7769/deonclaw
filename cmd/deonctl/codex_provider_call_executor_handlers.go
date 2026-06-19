package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderCallExecutorOptions struct {
	executorConfigPath   string
	dispatchConfigPath   string
	providerRunPlanPath  string
	payloadDryRunPath    string
	payloadOutputPath    string
	payloadReportPath    string
	providerCallGatePath string
	readinessReportPath  string
	approvalRequestPath  string
	approvalPath         string
	executionBundlePath  string
	outputFormat         string
}

type codexProviderCallExecutorPlanOptions struct {
	codexProviderCallExecutorOptions
	outputPath string
}

type codexProviderCallExecutorConfigValidateOptions struct {
	configPath   string
	outputFormat string
}

func parseCodexProviderCallExecutorConfigValidateOptions(args []string) (codexProviderCallExecutorConfigValidateOptions, error) {
	opts := codexProviderCallExecutorConfigValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorConfigValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutorConfigValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutorConfigValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderCallExecutorConfigValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallExecutorConfigValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderCallExecutorConfigValidate(opts codexProviderCallExecutorConfigValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadProviderCallExecutorConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor config validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.ProviderCallExecutorConfigValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor config validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCallExecutorConfigValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCallExecutorConfigValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor config validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func parseCodexProviderCallExecutorOptions(args []string) (codexProviderCallExecutorOptions, error) {
	opts := codexProviderCallExecutorOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--provider-run-plan":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --provider-run-plan")
			}
			opts.providerRunPlanPath = args[i+1]
			i++
		case "--payload-dry-run":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --payload-dry-run")
			}
			opts.payloadDryRunPath = args[i+1]
			i++
		case "--payload-output":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --payload-output")
			}
			opts.payloadOutputPath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--approval-request":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --approval-request")
			}
			opts.approvalRequestPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutorOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutorOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	required := map[string]string{
		"--executor-config": opts.executorConfigPath, "--dispatch-config": opts.dispatchConfigPath, "--provider-run-plan": opts.providerRunPlanPath,
		"--payload-dry-run": opts.payloadDryRunPath, "--payload-output": opts.payloadOutputPath,
		"--payload-report": opts.payloadReportPath, "--provider-call-gate": opts.providerCallGatePath,
		"--readiness-report": opts.readinessReportPath, "--approval-request": opts.approvalRequestPath,
		"--approval": opts.approvalPath, "--execution-bundle": opts.executionBundlePath,
	}
	for flag, value := range required {
		if value == "" {
			return codexProviderCallExecutorOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallExecutorOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderCallExecutorPlanOptions(args []string) (codexProviderCallExecutorPlanOptions, error) {
	opts := codexProviderCallExecutorPlanOptions{
		codexProviderCallExecutorOptions: codexProviderCallExecutorOptions{outputFormat: "text"},
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--provider-run-plan":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --provider-run-plan")
			}
			opts.providerRunPlanPath = args[i+1]
			i++
		case "--payload-dry-run":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --payload-dry-run")
			}
			opts.payloadDryRunPath = args[i+1]
			i++
		case "--payload-output":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --payload-output")
			}
			opts.payloadOutputPath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--approval-request":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --approval-request")
			}
			opts.approvalRequestPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	required := map[string]string{
		"--executor-config": opts.executorConfigPath, "--dispatch-config": opts.dispatchConfigPath, "--provider-run-plan": opts.providerRunPlanPath,
		"--payload-dry-run": opts.payloadDryRunPath, "--payload-output": opts.payloadOutputPath,
		"--payload-report": opts.payloadReportPath, "--provider-call-gate": opts.providerCallGatePath,
		"--readiness-report": opts.readinessReportPath, "--approval-request": opts.approvalRequestPath,
		"--approval": opts.approvalPath, "--execution-bundle": opts.executionBundlePath,
		"--output": opts.outputPath,
	}
	for flag, value := range required {
		if value == "" {
			return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallExecutorPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func toProviderCallExecutorOptions(opts codexProviderCallExecutorOptions) retrievalcontext.ProviderCallExecutorOptions {
	return retrievalcontext.ProviderCallExecutorOptions{
		ExecutorConfigPath:   opts.executorConfigPath,
		DispatchConfigPath:   opts.dispatchConfigPath,
		ProviderRunPlanPath:  opts.providerRunPlanPath,
		PayloadDryRunPath:    opts.payloadDryRunPath,
		PayloadOutputPath:    opts.payloadOutputPath,
		PayloadReportPath:    opts.payloadReportPath,
		ProviderCallGatePath: opts.providerCallGatePath,
		ReadinessReportPath:  opts.readinessReportPath,
		ApprovalRequestPath:  opts.approvalRequestPath,
		ApprovalPath:         opts.approvalPath,
		ExecutionBundlePath:  opts.executionBundlePath,
	}
}

func writeProviderCallExecutorResult(result retrievalcontext.ProviderCallExecutorResult, format string, stdout io.Writer) error {
	switch format {
	case "json":
		return retrievalcontext.WriteProviderCallExecutorJSON(result, stdout)
	default:
		return retrievalcontext.WriteProviderCallExecutorText(result, stdout)
	}
}

func runCodexProviderCallExecutorValidate(opts codexProviderCallExecutorOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCallExecutorValidate(toProviderCallExecutorOptions(opts))
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor validate failed: %v\n", err)
		return 1
	}
	if err := writeProviderCallExecutorResult(result, opts.outputFormat, stdout); err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderCallExecutorPlan(opts codexProviderCallExecutorPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCallExecutorPlan(retrievalcontext.ProviderCallExecutorPlanOptions{
		ProviderCallExecutorOptions: toProviderCallExecutorOptions(opts.codexProviderCallExecutorOptions),
		OutputPath:                  opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor plan failed: %v\n", err)
		return 1
	}
	if err := writeProviderCallExecutorResult(result, opts.outputFormat, stdout); err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor plan failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderCallExecutorDryRunOptions struct {
	executorConfigPath   string
	dispatchConfigPath   string
	providerRunPlanPath  string
	payloadDryRunPath    string
	payloadReportPath    string
	providerCallGatePath string
	readinessReportPath  string
	approvalRequestPath  string
	approvalPath         string
	executionBundlePath  string
	outputPath           string
	outputFormat         string
}

type codexProviderCallExecutorDryRunReportOptions struct {
	dryRunPath         string
	executorConfigPath string
	outputFormat       string
}

func parseCodexProviderCallExecutorDryRunOptions(args []string) (codexProviderCallExecutorDryRunOptions, error) {
	opts := codexProviderCallExecutorDryRunOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--provider-run-plan":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --provider-run-plan")
			}
			opts.providerRunPlanPath = args[i+1]
			i++
		case "--payload-dry-run":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --payload-dry-run")
			}
			opts.payloadDryRunPath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--approval-request":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --approval-request")
			}
			opts.approvalRequestPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	required := map[string]string{
		"--executor-config": opts.executorConfigPath, "--dispatch-config": opts.dispatchConfigPath,
		"--provider-run-plan": opts.providerRunPlanPath, "--payload-dry-run": opts.payloadDryRunPath,
		"--payload-report": opts.payloadReportPath, "--provider-call-gate": opts.providerCallGatePath,
		"--readiness-report": opts.readinessReportPath, "--approval-request": opts.approvalRequestPath,
		"--approval": opts.approvalPath, "--execution-bundle": opts.executionBundlePath, "--output": opts.outputPath,
	}
	for flag, value := range required {
		if value == "" {
			return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallExecutorDryRunOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderCallExecutorDryRunReportOptions(args []string) (codexProviderCallExecutorDryRunReportOptions, error) {
	opts := codexProviderCallExecutorDryRunReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("missing value for --dry-run")
			}
			opts.dryRunPath = args[i+1]
			i++
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.dryRunPath == "" {
		return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("missing --dry-run")
	}
	if opts.executorConfigPath == "" {
		return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("missing --executor-config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallExecutorDryRunReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderCallExecutorDryRun(opts codexProviderCallExecutorDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCallExecutorDryRun(retrievalcontext.ProviderCallExecutorDryRunOptions{
		ProviderCallExecutorOptions: retrievalcontext.ProviderCallExecutorOptions{
			ExecutorConfigPath:   opts.executorConfigPath,
			DispatchConfigPath:   opts.dispatchConfigPath,
			ProviderRunPlanPath:  opts.providerRunPlanPath,
			PayloadDryRunPath:    opts.payloadDryRunPath,
			PayloadReportPath:    opts.payloadReportPath,
			ProviderCallGatePath: opts.providerCallGatePath,
			ReadinessReportPath:  opts.readinessReportPath,
			ApprovalRequestPath:  opts.approvalRequestPath,
			ApprovalPath:         opts.approvalPath,
			ExecutionBundlePath:  opts.executionBundlePath,
		},
		OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor dry-run failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCallExecutorDryRunJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCallExecutorDryRunText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor dry-run failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderCallExecutorDryRunReport(opts codexProviderCallExecutorDryRunReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCallExecutorDryRunReport(retrievalcontext.ProviderCallExecutorDryRunReportOptions{
		DryRunPath:         opts.dryRunPath,
		ExecutorConfigPath: opts.executorConfigPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor dry-run-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCallExecutorDryRunReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCallExecutorDryRunReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor dry-run-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
