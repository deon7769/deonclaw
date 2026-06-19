package main

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderCallExecutorDispatchApprovalNewOptions struct {
	executorConfigPath  string
	dryRunPath          string
	dryRunReportPath    string
	preflightPath       string
	executionBundlePath string
	approvalPath        string
	outputPath          string
}

type codexProviderCallExecutorDispatchApprovalApproveOptions struct {
	requestPath                  string
	outputPath                   string
	confirmExecutorConfigSHA256  string
	confirmExecutionBundleSHA256 string
	confirmProviderPayloadSHA256 string
}

type codexProviderCallExecutorDispatchApprovalInspectOptions struct {
	approvalPath string
	requestPath  string
	outputFormat string
}

func parseCodexProviderCallExecutorDispatchApprovalNewOptions(args []string) (codexProviderCallExecutorDispatchApprovalNewOptions, error) {
	opts := codexProviderCallExecutorDispatchApprovalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--dry-run":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --dry-run")
			}
			opts.dryRunPath = args[i+1]
			i++
		case "--dry-run-report":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --dry-run-report")
			}
			opts.dryRunReportPath = args[i+1]
			i++
		case "--preflight":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --preflight")
			}
			opts.preflightPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--executor-config": opts.executorConfigPath, "--dry-run": opts.dryRunPath,
		"--dry-run-report": opts.dryRunReportPath, "--preflight": opts.preflightPath,
		"--execution-bundle": opts.executionBundlePath, "--approval": opts.approvalPath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderCallExecutorDispatchApprovalNewOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	return opts, nil
}

func parseCodexProviderCallExecutorDispatchApprovalApproveOptions(args []string) (codexProviderCallExecutorDispatchApprovalApproveOptions, error) {
	opts := codexProviderCallExecutorDispatchApprovalApproveOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-executor-config-sha256":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-executor-config-sha256")
			}
			opts.confirmExecutorConfigSHA256 = args[i+1]
			i++
		case "--confirm-execution-bundle-sha256":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-execution-bundle-sha256")
			}
			opts.confirmExecutionBundleSHA256 = args[i+1]
			i++
		case "--confirm-provider-payload-sha256":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-provider-payload-sha256")
			}
			opts.confirmProviderPayloadSHA256 = args[i+1]
			i++
		default:
			return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--request": opts.requestPath, "--output": opts.outputPath,
		"--confirm-executor-config-sha256":  opts.confirmExecutorConfigSHA256,
		"--confirm-execution-bundle-sha256": opts.confirmExecutionBundleSHA256,
		"--confirm-provider-payload-sha256": opts.confirmProviderPayloadSHA256,
	} {
		if value == "" {
			return codexProviderCallExecutorDispatchApprovalApproveOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	return opts, nil
}

func parseCodexProviderCallExecutorDispatchApprovalInspectOptions(args []string) (codexProviderCallExecutorDispatchApprovalInspectOptions, error) {
	opts := codexProviderCallExecutorDispatchApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalInspectOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallExecutorDispatchApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallExecutorDispatchApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return codexProviderCallExecutorDispatchApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallExecutorDispatchApprovalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderCallExecutorDispatchApprovalNew(opts codexProviderCallExecutorDispatchApprovalNewOptions, stdout io.Writer, stderr io.Writer) int {
	_, err := retrievalcontext.NewProviderCallExecutorDispatchApprovalRequest(retrievalcontext.NewProviderCallExecutorDispatchApprovalRequestOptions{
		ExecutorConfigPath: opts.executorConfigPath, DryRunPath: opts.dryRunPath,
		DryRunReportPath: opts.dryRunReportPath, PreflightPath: opts.preflightPath,
		ExecutionBundlePath: opts.executionBundlePath, ApprovalPath: opts.approvalPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor-dispatch-approval new failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-call-executor-dispatch-approval new: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderCallExecutorDispatchApprovalApprove(opts codexProviderCallExecutorDispatchApprovalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	_, err := retrievalcontext.ApproveProviderCallExecutorDispatch(retrievalcontext.ApproveProviderCallExecutorDispatchOptions{
		RequestPath: opts.requestPath, OutputPath: opts.outputPath,
		ConfirmExecutorConfigSHA256:  opts.confirmExecutorConfigSHA256,
		ConfirmExecutionBundleSHA256: opts.confirmExecutionBundleSHA256,
		ConfirmProviderPayloadSHA256: opts.confirmProviderPayloadSHA256,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor-dispatch-approval approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-call-executor-dispatch-approval approve: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderCallExecutorDispatchApprovalInspect(opts codexProviderCallExecutorDispatchApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectProviderCallExecutorDispatchApproval(opts.approvalPath, retrievalcontext.InspectProviderCallExecutorDispatchApprovalOptions{
		RequestPath: opts.requestPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor-dispatch-approval inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCallExecutorDispatchApprovalInspectJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCallExecutorDispatchApprovalInspectText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-executor-dispatch-approval inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderTransportPlanOptions struct {
	executorConfigPath    string
	executorPreflightPath string
	dispatchApprovalPath  string
	outputPath            string
	outputFormat          string
}

func parseCodexProviderTransportPlanOptions(args []string) (codexProviderTransportPlanOptions, error) {
	opts := codexProviderTransportPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderTransportPlanOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--executor-preflight":
			if i+1 >= len(args) {
				return codexProviderTransportPlanOptions{}, fmt.Errorf("missing value for --executor-preflight")
			}
			opts.executorPreflightPath = args[i+1]
			i++
		case "--dispatch-approval":
			if i+1 >= len(args) {
				return codexProviderTransportPlanOptions{}, fmt.Errorf("missing value for --dispatch-approval")
			}
			opts.dispatchApprovalPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderTransportPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderTransportPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderTransportPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--executor-config": opts.executorConfigPath, "--executor-preflight": opts.executorPreflightPath,
		"--dispatch-approval": opts.dispatchApprovalPath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderTransportPlanOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderTransportPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderTransportPlan(opts codexProviderTransportPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderTransportPlan(retrievalcontext.ProviderTransportPlanOptions{
		ExecutorConfigPath: opts.executorConfigPath, ExecutorPreflightPath: opts.executorPreflightPath,
		DispatchApprovalPath: opts.dispatchApprovalPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-transport-plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		data, mErr := json.MarshalIndent(result, "", "  ")
		if mErr != nil {
			err = mErr
		} else {
			data = append(data, '\n')
			_, err = stdout.Write(data)
		}
	default:
		err = retrievalcontext.WriteProviderTransportPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-transport-plan failed: %v\n", err)
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

type codexProviderExecutorReleaseBundleOptions struct {
	executorConfigPath    string
	executionBundlePath   string
	dryRunPath            string
	dryRunReportPath      string
	executorPreflightPath string
	dispatchApprovalPath  string
	transportPlanPath     string
	outputPath            string
	outputFormat          string
}

type codexProviderExecutorReleaseGateOptions struct {
	executorConfigPath    string
	executionBundlePath   string
	dryRunPath            string
	dryRunReportPath      string
	executorPreflightPath string
	dispatchApprovalPath  string
	transportPlanPath     string
	releaseBundlePath     string
	outputFormat          string
}

func parseCodexProviderExecutorReleaseBundleOptions(args []string) (codexProviderExecutorReleaseBundleOptions, error) {
	opts := codexProviderExecutorReleaseBundleOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--dry-run":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --dry-run")
			}
			opts.dryRunPath = args[i+1]
			i++
		case "--dry-run-report":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --dry-run-report")
			}
			opts.dryRunReportPath = args[i+1]
			i++
		case "--executor-preflight":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --executor-preflight")
			}
			opts.executorPreflightPath = args[i+1]
			i++
		case "--dispatch-approval":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --dispatch-approval")
			}
			opts.dispatchApprovalPath = args[i+1]
			i++
		case "--transport-plan":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --transport-plan")
			}
			opts.transportPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--executor-config": opts.executorConfigPath, "--execution-bundle": opts.executionBundlePath,
		"--dry-run": opts.dryRunPath, "--dry-run-report": opts.dryRunReportPath,
		"--executor-preflight": opts.executorPreflightPath, "--dispatch-approval": opts.dispatchApprovalPath,
		"--transport-plan": opts.transportPlanPath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderExecutorReleaseBundleOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderExecutorReleaseGateOptions(args []string) (codexProviderExecutorReleaseGateOptions, error) {
	opts := codexProviderExecutorReleaseGateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--dry-run":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --dry-run")
			}
			opts.dryRunPath = args[i+1]
			i++
		case "--dry-run-report":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --dry-run-report")
			}
			opts.dryRunReportPath = args[i+1]
			i++
		case "--executor-preflight":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --executor-preflight")
			}
			opts.executorPreflightPath = args[i+1]
			i++
		case "--dispatch-approval":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --dispatch-approval")
			}
			opts.dispatchApprovalPath = args[i+1]
			i++
		case "--transport-plan":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --transport-plan")
			}
			opts.transportPlanPath = args[i+1]
			i++
		case "--release-bundle":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --release-bundle")
			}
			opts.releaseBundlePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--executor-config": opts.executorConfigPath, "--execution-bundle": opts.executionBundlePath,
		"--dry-run": opts.dryRunPath, "--dry-run-report": opts.dryRunReportPath,
		"--executor-preflight": opts.executorPreflightPath, "--dispatch-approval": opts.dispatchApprovalPath,
		"--transport-plan": opts.transportPlanPath, "--release-bundle": opts.releaseBundlePath,
	} {
		if value == "" {
			return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderExecutorReleaseGateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderExecutorReleaseBundle(opts codexProviderExecutorReleaseBundleOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderExecutorReleaseBundle(retrievalcontext.ProviderExecutorReleaseBundleOptions{
		ExecutorConfigPath: opts.executorConfigPath, ExecutionBundlePath: opts.executionBundlePath,
		DryRunPath: opts.dryRunPath, DryRunReportPath: opts.dryRunReportPath,
		ExecutorPreflightPath: opts.executorPreflightPath, DispatchApprovalPath: opts.dispatchApprovalPath,
		TransportPlanPath: opts.transportPlanPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-executor-release-bundle failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderExecutorReleaseBundleJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderExecutorReleaseBundleText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-executor-release-bundle failed: %v\n", err)
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

func runCodexProviderExecutorReleaseGate(opts codexProviderExecutorReleaseGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderExecutorReleaseGate(retrievalcontext.ProviderExecutorReleaseGateOptions{
		ExecutorConfigPath: opts.executorConfigPath, ExecutionBundlePath: opts.executionBundlePath,
		DryRunPath: opts.dryRunPath, DryRunReportPath: opts.dryRunReportPath,
		ExecutorPreflightPath: opts.executorPreflightPath, DispatchApprovalPath: opts.dispatchApprovalPath,
		TransportPlanPath: opts.transportPlanPath, ReleaseBundlePath: opts.releaseBundlePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-executor-release-gate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderExecutorReleaseGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderExecutorReleaseGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-executor-release-gate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
