package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexMaterializedProviderDispatchValidateOptions struct {
	configPath   string
	outputFormat string
}

func parseCodexMaterializedProviderDispatchValidateOptions(args []string) (codexMaterializedProviderDispatchValidateOptions, error) {
	opts := codexMaterializedProviderDispatchValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexMaterializedProviderDispatchValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedProviderDispatchValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedProviderDispatchValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexMaterializedProviderDispatchValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedProviderDispatchValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedProviderDispatchValidate(opts codexMaterializedProviderDispatchValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadMaterializedProviderDispatch(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-dispatch validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.MaterializedProviderDispatchValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-dispatch validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedProviderDispatchValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedProviderDispatchValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-dispatch validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedProviderPayloadDryRunOptions struct {
	dispatchConfigPath               string
	providerRunPlanPath              string
	assembledOutputPath              string
	outputPath                       string
	payloadOutputPath                string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedProviderPayloadDryRunOptions(args []string) (codexMaterializedProviderPayloadDryRunOptions, error) {
	opts := codexMaterializedProviderPayloadDryRunOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--provider-run-plan":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing value for --provider-run-plan")
			}
			opts.providerRunPlanPath = args[i+1]
			i++
		case "--assembled-output":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing value for --assembled-output")
			}
			opts.assembledOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--payload-output":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing value for --payload-output")
			}
			opts.payloadOutputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.dispatchConfigPath == "" {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing --dispatch-config")
	}
	if opts.providerRunPlanPath == "" {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing --provider-run-plan")
	}
	if opts.assembledOutputPath == "" {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing --assembled-output")
	}
	if opts.outputPath == "" {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing --output")
	}
	if opts.payloadOutputPath == "" {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing --payload-output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedProviderPayloadDryRunOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedProviderPayloadDryRun(opts codexMaterializedProviderPayloadDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedProviderPayload(retrievalcontext.MaterializedProviderPayloadOptions{
		DispatchConfigPath:               opts.dispatchConfigPath,
		ProviderRunPlanPath:              opts.providerRunPlanPath,
		AssembledOutputPath:              opts.assembledOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
		OutputPath:                       opts.outputPath,
		PayloadOutputPath:                opts.payloadOutputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-payload-dry-run failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedProviderPayloadJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedProviderPayloadText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-payload-dry-run failed: %v\n", err)
		return 1
	}
	if opts.outputFormat != "json" {
		fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
		fmt.Fprintf(stdout, "payload_output: %s\n", opts.payloadOutputPath)
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedProviderPayloadReportOptions struct {
	payloadDryRunPath string
	payloadOutputPath string
	outputFormat      string
}

func parseCodexMaterializedProviderPayloadReportOptions(args []string) (codexMaterializedProviderPayloadReportOptions, error) {
	opts := codexMaterializedProviderPayloadReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--payload-dry-run":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("missing value for --payload-dry-run")
			}
			opts.payloadDryRunPath = args[i+1]
			i++
		case "--payload-output":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("missing value for --payload-output")
			}
			opts.payloadOutputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.payloadDryRunPath == "" {
		return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("missing --payload-dry-run")
	}
	if opts.payloadOutputPath == "" {
		return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("missing --payload-output")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedProviderPayloadReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedProviderPayloadReport(opts codexMaterializedProviderPayloadReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedProviderPayloadReport(retrievalcontext.MaterializedProviderPayloadReportOptions{
		PayloadDryRunPath: opts.payloadDryRunPath,
		PayloadOutputPath: opts.payloadOutputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-payload-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedProviderPayloadReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedProviderPayloadReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-payload-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexMaterializedProviderCallGateOptions struct {
	dispatchConfigPath               string
	payloadReportPath                string
	payloadOutputPath                string
	outputPath                       string
	confirmInjectMaterializedContext bool
	outputFormat                     string
}

func parseCodexMaterializedProviderCallGateOptions(args []string) (codexMaterializedProviderCallGateOptions, error) {
	opts := codexMaterializedProviderCallGateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--payload-output":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing value for --payload-output")
			}
			opts.payloadOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-inject-materialized-context":
			opts.confirmInjectMaterializedContext = true
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.dispatchConfigPath == "" {
		return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing --dispatch-config")
	}
	if opts.payloadReportPath == "" {
		return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing --payload-report")
	}
	if opts.payloadOutputPath == "" {
		return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing --payload-output")
	}
	if opts.outputPath == "" {
		return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmInjectMaterializedContext {
		return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("missing --confirm-inject-materialized-context")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedProviderCallGateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedProviderCallGate(opts codexMaterializedProviderCallGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedProviderCallGate(retrievalcontext.MaterializedProviderCallGateOptions{
		DispatchConfigPath:               opts.dispatchConfigPath,
		PayloadReportPath:                opts.payloadReportPath,
		PayloadOutputPath:                opts.payloadOutputPath,
		ConfirmInjectMaterializedContext: opts.confirmInjectMaterializedContext,
		OutputPath:                       opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-call-gate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedProviderCallGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedProviderCallGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-call-gate failed: %v\n", err)
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

type codexMaterializedProviderCallReadinessReportOptions struct {
	providerCallGatePath string
	payloadReportPath    string
	outputFormat         string
}

func parseCodexMaterializedProviderCallReadinessReportOptions(args []string) (codexMaterializedProviderCallReadinessReportOptions, error) {
	opts := codexMaterializedProviderCallReadinessReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.providerCallGatePath == "" {
		return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("missing --provider-call-gate")
	}
	if opts.payloadReportPath == "" {
		return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("missing --payload-report")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexMaterializedProviderCallReadinessReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexMaterializedProviderCallReadinessReport(opts codexMaterializedProviderCallReadinessReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.MaterializedProviderCallReadinessReport(retrievalcontext.MaterializedProviderCallReadinessReportOptions{
		ProviderCallGatePath: opts.providerCallGatePath,
		PayloadReportPath:    opts.payloadReportPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-call-readiness-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteMaterializedProviderCallReadinessReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteMaterializedProviderCallReadinessReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex materialized-provider-call-readiness-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
