package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderRequestEnvelopeDryRunOptions struct {
	executorConfigPath    string
	executionBundlePath   string
	executorPreflightPath string
	dispatchApprovalPath  string
	transportPlanPath     string
	releaseBundlePath     string
	releaseGatePath       string
	outputPath            string
	outputFormat          string
}

type codexProviderRequestEnvelopeReportOptions struct {
	codexProviderRequestEnvelopeDryRunOptions
	requestEnvelopePath string
}

func parseCodexProviderRequestEnvelopeDryRunOptions(args []string) (codexProviderRequestEnvelopeDryRunOptions, error) {
	opts := codexProviderRequestEnvelopeDryRunOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--executor-preflight":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --executor-preflight")
			}
			opts.executorPreflightPath = args[i+1]
			i++
		case "--dispatch-approval":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --dispatch-approval")
			}
			opts.dispatchApprovalPath = args[i+1]
			i++
		case "--transport-plan":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --transport-plan")
			}
			opts.transportPlanPath = args[i+1]
			i++
		case "--release-bundle":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --release-bundle")
			}
			opts.releaseBundlePath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--executor-config": opts.executorConfigPath, "--execution-bundle": opts.executionBundlePath,
		"--executor-preflight": opts.executorPreflightPath, "--dispatch-approval": opts.dispatchApprovalPath,
		"--transport-plan": opts.transportPlanPath, "--release-bundle": opts.releaseBundlePath,
		"--release-gate": opts.releaseGatePath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRequestEnvelopeDryRunOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRequestEnvelopeReportOptions(args []string) (codexProviderRequestEnvelopeReportOptions, error) {
	opts := codexProviderRequestEnvelopeReportOptions{
		codexProviderRequestEnvelopeDryRunOptions: codexProviderRequestEnvelopeDryRunOptions{outputFormat: "text"},
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--executor-preflight":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --executor-preflight")
			}
			opts.executorPreflightPath = args[i+1]
			i++
		case "--dispatch-approval":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --dispatch-approval")
			}
			opts.dispatchApprovalPath = args[i+1]
			i++
		case "--transport-plan":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --transport-plan")
			}
			opts.transportPlanPath = args[i+1]
			i++
		case "--release-bundle":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --release-bundle")
			}
			opts.releaseBundlePath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.requestEnvelopePath == "" {
		return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing --request-envelope")
	}
	for flag, value := range map[string]string{
		"--executor-config": opts.executorConfigPath, "--execution-bundle": opts.executionBundlePath,
		"--executor-preflight": opts.executorPreflightPath, "--dispatch-approval": opts.dispatchApprovalPath,
		"--transport-plan": opts.transportPlanPath, "--release-bundle": opts.releaseBundlePath,
		"--release-gate": opts.releaseGatePath,
	} {
		if value == "" {
			return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRequestEnvelopeReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderRequestEnvelopeDryRun(opts codexProviderRequestEnvelopeDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRequestEnvelopeDryRun(retrievalcontext.ProviderRequestEnvelopeDryRunOptions{
		ExecutorConfigPath: opts.executorConfigPath, ExecutionBundlePath: opts.executionBundlePath,
		ExecutorPreflightPath: opts.executorPreflightPath, DispatchApprovalPath: opts.dispatchApprovalPath,
		TransportPlanPath: opts.transportPlanPath, ReleaseBundlePath: opts.releaseBundlePath,
		ReleaseGatePath: opts.releaseGatePath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-request-envelope dry-run failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRequestEnvelopeJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRequestEnvelopeText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-request-envelope dry-run failed: %v\n", err)
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

func runCodexProviderRequestEnvelopeReport(opts codexProviderRequestEnvelopeReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRequestEnvelopeReport(retrievalcontext.ProviderRequestEnvelopeReportOptions{
		RequestEnvelopePath: opts.requestEnvelopePath, ExecutorConfigPath: opts.executorConfigPath,
		ExecutionBundlePath: opts.executionBundlePath, ExecutorPreflightPath: opts.executorPreflightPath,
		DispatchApprovalPath: opts.dispatchApprovalPath, TransportPlanPath: opts.transportPlanPath,
		ReleaseBundlePath: opts.releaseBundlePath, ReleaseGatePath: opts.releaseGatePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-request-envelope report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRequestEnvelopeJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRequestEnvelopeText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-request-envelope report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderAdapterRegistryValidateOptions struct {
	configPath   string
	outputFormat string
}

type codexProviderAdapterPlanOptions struct {
	configPath          string
	requestEnvelopePath string
	outputPath          string
	outputFormat        string
}

func parseCodexProviderAdapterRegistryValidateOptions(args []string) (codexProviderAdapterRegistryValidateOptions, error) {
	opts := codexProviderAdapterRegistryValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderAdapterRegistryValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderAdapterRegistryValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderAdapterRegistryValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderAdapterRegistryValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderAdapterRegistryValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderAdapterPlanOptions(args []string) (codexProviderAdapterPlanOptions, error) {
	opts := codexProviderAdapterPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderAdapterPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderAdapterPlanOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderAdapterPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderAdapterPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderAdapterPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--config": opts.configPath, "--request-envelope": opts.requestEnvelopePath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderAdapterPlanOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderAdapterPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderAdapterRegistryValidate(opts codexProviderAdapterRegistryValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadProviderAdaptersConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-adapter-registry validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.ProviderAdapterRegistryValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-adapter-registry validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderAdapterRegistryValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderAdapterRegistryValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-adapter-registry validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderAdapterPlan(opts codexProviderAdapterPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderAdapterPlan(retrievalcontext.ProviderAdapterPlanOptions{
		ConfigPath: opts.configPath, RequestEnvelopePath: opts.requestEnvelopePath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-adapter-plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderAdapterPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderAdapterPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-adapter-plan failed: %v\n", err)
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

type codexProviderResponseFixtureGenerateOptions struct {
	requestEnvelopePath string
	adapterPlanPath     string
	outputPath          string
	outputFormat        string
}

type codexProviderResponseFixtureInspectOptions struct {
	fixturePath         string
	requestEnvelopePath string
	adapterPlanPath     string
	outputFormat        string
}

func parseCodexProviderResponseFixtureGenerateOptions(args []string) (codexProviderResponseFixtureGenerateOptions, error) {
	opts := codexProviderResponseFixtureGenerateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--adapter-plan":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("missing value for --adapter-plan")
			}
			opts.adapterPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--request-envelope": opts.requestEnvelopePath, "--adapter-plan": opts.adapterPlanPath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderResponseFixtureGenerateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderResponseFixtureInspectOptions(args []string) (codexProviderResponseFixtureInspectOptions, error) {
	opts := codexProviderResponseFixtureInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--fixture":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("missing value for --fixture")
			}
			opts.fixturePath = args[i+1]
			i++
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--adapter-plan":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("missing value for --adapter-plan")
			}
			opts.adapterPlanPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.fixturePath == "" {
		return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("missing --fixture")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderResponseFixtureInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderResponseFixtureGenerate(opts codexProviderResponseFixtureGenerateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderResponseFixtureGenerate(retrievalcontext.ProviderResponseFixtureGenerateOptions{
		RequestEnvelopePath: opts.requestEnvelopePath, AdapterPlanPath: opts.adapterPlanPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-fixture generate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderResponseFixtureGenerateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderResponseFixtureGenerateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-fixture generate failed: %v\n", err)
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

func runCodexProviderResponseFixtureInspect(opts codexProviderResponseFixtureInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderResponseFixtureInspect(opts.fixturePath, retrievalcontext.ProviderResponseFixtureInspectOptions{
		RequestEnvelopePath: opts.requestEnvelopePath, AdapterPlanPath: opts.adapterPlanPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-fixture inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderResponseFixtureInspectJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderResponseFixtureInspectText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-fixture inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderExecutionSimulationBundleOptions struct {
	requestEnvelopePath string
	adapterPlanPath     string
	responseFixturePath string
	releaseGatePath     string
	executorConfigPath  string
	outputPath          string
	outputFormat        string
}

type codexProviderExecutionSimulationReportOptions struct {
	codexProviderExecutionSimulationBundleOptions
	simulationBundlePath string
}

func parseCodexProviderExecutionSimulationBundleOptions(args []string) (codexProviderExecutionSimulationBundleOptions, error) {
	opts := codexProviderExecutionSimulationBundleOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--adapter-plan":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --adapter-plan")
			}
			opts.adapterPlanPath = args[i+1]
			i++
		case "--response-fixture":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --response-fixture")
			}
			opts.responseFixturePath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--request-envelope": opts.requestEnvelopePath, "--adapter-plan": opts.adapterPlanPath,
		"--response-fixture": opts.responseFixturePath, "--release-gate": opts.releaseGatePath,
		"--executor-config": opts.executorConfigPath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderExecutionSimulationBundleOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderExecutionSimulationReportOptions(args []string) (codexProviderExecutionSimulationReportOptions, error) {
	opts := codexProviderExecutionSimulationReportOptions{
		codexProviderExecutionSimulationBundleOptions: codexProviderExecutionSimulationBundleOptions{outputFormat: "text"},
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--simulation-bundle":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --simulation-bundle")
			}
			opts.simulationBundlePath = args[i+1]
			i++
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--adapter-plan":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --adapter-plan")
			}
			opts.adapterPlanPath = args[i+1]
			i++
		case "--response-fixture":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --response-fixture")
			}
			opts.responseFixturePath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--executor-config":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --executor-config")
			}
			opts.executorConfigPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.simulationBundlePath == "" {
		return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing --simulation-bundle")
	}
	for flag, value := range map[string]string{
		"--request-envelope": opts.requestEnvelopePath, "--adapter-plan": opts.adapterPlanPath,
		"--response-fixture": opts.responseFixturePath, "--release-gate": opts.releaseGatePath,
		"--executor-config": opts.executorConfigPath,
	} {
		if value == "" {
			return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderExecutionSimulationReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderExecutionSimulationBundle(opts codexProviderExecutionSimulationBundleOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderExecutionSimulationBundle(retrievalcontext.ProviderExecutionSimulationBundleOptions{
		RequestEnvelopePath: opts.requestEnvelopePath, AdapterPlanPath: opts.adapterPlanPath,
		ResponseFixturePath: opts.responseFixturePath, ReleaseGatePath: opts.releaseGatePath,
		ExecutorConfigPath: opts.executorConfigPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-execution-simulation-bundle failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderExecutionSimulationBundleJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderExecutionSimulationBundleText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-execution-simulation-bundle failed: %v\n", err)
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

func runCodexProviderExecutionSimulationReport(opts codexProviderExecutionSimulationReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderExecutionSimulationReport(retrievalcontext.ProviderExecutionSimulationReportOptions{
		SimulationBundlePath: opts.simulationBundlePath, RequestEnvelopePath: opts.requestEnvelopePath,
		AdapterPlanPath: opts.adapterPlanPath, ResponseFixturePath: opts.responseFixturePath,
		ReleaseGatePath: opts.releaseGatePath, ExecutorConfigPath: opts.executorConfigPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-execution-simulation-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderExecutionSimulationReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderExecutionSimulationReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-execution-simulation-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
