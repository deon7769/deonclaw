package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderCallChainAuditOptions struct {
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

func parseCodexProviderCallChainAuditOptions(args []string) (codexProviderCallChainAuditOptions, error) {
	opts := codexProviderCallChainAuditOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dispatch-config":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --dispatch-config")
			}
			opts.dispatchConfigPath = args[i+1]
			i++
		case "--provider-run-plan":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --provider-run-plan")
			}
			opts.providerRunPlanPath = args[i+1]
			i++
		case "--payload-dry-run":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --payload-dry-run")
			}
			opts.payloadDryRunPath = args[i+1]
			i++
		case "--payload-output":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --payload-output")
			}
			opts.payloadOutputPath = args[i+1]
			i++
		case "--payload-report":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --payload-report")
			}
			opts.payloadReportPath = args[i+1]
			i++
		case "--provider-call-gate":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --provider-call-gate")
			}
			opts.providerCallGatePath = args[i+1]
			i++
		case "--readiness-report":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --readiness-report")
			}
			opts.readinessReportPath = args[i+1]
			i++
		case "--approval-request":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --approval-request")
			}
			opts.approvalRequestPath = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--execution-bundle":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --execution-bundle")
			}
			opts.executionBundlePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCallChainAuditOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	required := map[string]string{
		"--dispatch-config": opts.dispatchConfigPath, "--provider-run-plan": opts.providerRunPlanPath,
		"--payload-dry-run": opts.payloadDryRunPath, "--payload-output": opts.payloadOutputPath,
		"--payload-report": opts.payloadReportPath, "--provider-call-gate": opts.providerCallGatePath,
		"--readiness-report": opts.readinessReportPath, "--approval-request": opts.approvalRequestPath,
		"--approval": opts.approvalPath, "--execution-bundle": opts.executionBundlePath,
	}
	for flag, value := range required {
		if value == "" {
			return codexProviderCallChainAuditOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCallChainAuditOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderCallChainAudit(opts codexProviderCallChainAuditOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCallChainContinuityAudit(retrievalcontext.ProviderCallChainContinuityAuditOptions{
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
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-chain-audit failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCallChainContinuityAuditJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCallChainContinuityAuditText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-call-chain-audit failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
