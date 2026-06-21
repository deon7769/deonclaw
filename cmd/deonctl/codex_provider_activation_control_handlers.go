package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderActivationPolicyValidateOptions struct {
	configPath   string
	outputFormat string
}

type codexProviderActivationPolicyPlanOptions struct {
	configPath   string
	outputPath   string
	outputFormat string
}

func parseCodexProviderActivationPolicyValidateOptions(args []string) (codexProviderActivationPolicyValidateOptions, error) {
	opts := codexProviderActivationPolicyValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderActivationPolicyValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationPolicyValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationPolicyValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderActivationPolicyValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationPolicyValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderActivationPolicyPlanOptions(args []string) (codexProviderActivationPolicyPlanOptions, error) {
	opts := codexProviderActivationPolicyPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputPath == "" {
		return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("missing --output")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationPolicyPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderActivationPolicyValidate(opts codexProviderActivationPolicyValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadProviderActivationPolicyConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-policy validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.ProviderActivationPolicyValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-policy validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationPolicyValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationPolicyValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-policy validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderActivationPolicyPlan(opts codexProviderActivationPolicyPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationPolicyPlan(retrievalcontext.ProviderActivationPolicyPlanOptions{
		ConfigPath: opts.configPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-policy plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationPolicyPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationPolicyPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-policy plan failed: %v\n", err)
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

type codexProviderActivationApprovalNewOptions struct {
	activationPolicyPlanPath      string
	activationReadinessAuditPath  string
	realCallProposalPath          string
	credentialPolicyPlanPath      string
	executionSimulationReportPath string
	outputPath                    string
}

type codexProviderActivationApprovalApproveOptions struct {
	requestPath                   string
	outputPath                    string
	confirmActivationPolicySHA256 string
	confirmReadinessAuditSHA256   string
	confirmRealCallProposalSHA256 string
	confirmProviderPayloadSHA256  string
}

type codexProviderActivationApprovalInspectOptions struct {
	approvalPath string
	requestPath  string
	outputFormat string
}

func parseCodexProviderActivationApprovalNewOptions(args []string) (codexProviderActivationApprovalNewOptions, error) {
	opts := codexProviderActivationApprovalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for _, check := range []struct {
		flag string
		val  string
	}{
		{"--activation-policy-plan", opts.activationPolicyPlanPath},
		{"--activation-readiness-audit", opts.activationReadinessAuditPath},
		{"--real-call-proposal", opts.realCallProposalPath},
		{"--credential-policy-plan", opts.credentialPolicyPlanPath},
		{"--execution-simulation-report", opts.executionSimulationReportPath},
		{"--output", opts.outputPath},
	} {
		if check.val == "" {
			return codexProviderActivationApprovalNewOptions{}, fmt.Errorf("missing %s", check.flag)
		}
	}
	return opts, nil
}

func parseCodexProviderActivationApprovalApproveOptions(args []string) (codexProviderActivationApprovalApproveOptions, error) {
	opts := codexProviderActivationApprovalApproveOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-activation-policy-sha256":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-activation-policy-sha256")
			}
			opts.confirmActivationPolicySHA256 = args[i+1]
			i++
		case "--confirm-readiness-audit-sha256":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-readiness-audit-sha256")
			}
			opts.confirmReadinessAuditSHA256 = args[i+1]
			i++
		case "--confirm-real-call-proposal-sha256":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-real-call-proposal-sha256")
			}
			opts.confirmRealCallProposalSHA256 = args[i+1]
			i++
		case "--confirm-provider-payload-sha256":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-provider-payload-sha256")
			}
			opts.confirmProviderPayloadSHA256 = args[i+1]
			i++
		default:
			return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.requestPath == "" || opts.outputPath == "" {
		return codexProviderActivationApprovalApproveOptions{}, fmt.Errorf("missing --request or --output")
	}
	return opts, nil
}

func parseCodexProviderActivationApprovalInspectOptions(args []string) (codexProviderActivationApprovalInspectOptions, error) {
	opts := codexProviderActivationApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalInspectOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return codexProviderActivationApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationApprovalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderActivationApprovalNew(opts codexProviderActivationApprovalNewOptions, stdout io.Writer, stderr io.Writer) int {
	_, err := retrievalcontext.NewProviderActivationApprovalRequest(retrievalcontext.NewProviderActivationApprovalRequestOptions{
		ActivationPolicyPlanPath:      opts.activationPolicyPlanPath,
		ActivationReadinessAuditPath:  opts.activationReadinessAuditPath,
		RealCallProposalPath:          opts.realCallProposalPath,
		CredentialPolicyPlanPath:      opts.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: opts.executionSimulationReportPath,
		OutputPath:                    opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-approval new failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderActivationApprovalApprove(opts codexProviderActivationApprovalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	_, err := retrievalcontext.ApproveProviderActivation(retrievalcontext.ApproveProviderActivationOptions{
		RequestPath:                   opts.requestPath,
		OutputPath:                    opts.outputPath,
		ConfirmActivationPolicySHA256: opts.confirmActivationPolicySHA256,
		ConfirmReadinessAuditSHA256:   opts.confirmReadinessAuditSHA256,
		ConfirmRealCallProposalSHA256: opts.confirmRealCallProposalSHA256,
		ConfirmProviderPayloadSHA256:  opts.confirmProviderPayloadSHA256,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-approval approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderActivationApprovalInspect(opts codexProviderActivationApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectProviderActivationApproval(opts.approvalPath, retrievalcontext.InspectProviderActivationApprovalOptions{
		RequestPath: opts.requestPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-approval inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationApprovalInspectJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationApprovalInspectText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-approval inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderActivationRehearsalOptions struct {
	activationPolicyPlanPath      string
	activationApprovalPath        string
	activationReadinessAuditPath  string
	realCallProposalPath          string
	credentialPolicyPlanPath      string
	executionSimulationReportPath string
	outputPath                    string
	outputFormat                  string
}

type codexProviderActivationRehearsalReportOptions struct {
	rehearsalPath                 string
	activationPolicyPlanPath      string
	activationApprovalPath        string
	activationReadinessAuditPath  string
	realCallProposalPath          string
	credentialPolicyPlanPath      string
	executionSimulationReportPath string
	outputFormat                  string
}

func parseCodexProviderActivationRehearsalOptions(args []string) (codexProviderActivationRehearsalOptions, error) {
	opts := codexProviderActivationRehearsalOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationRehearsalOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputPath == "" {
		return codexProviderActivationRehearsalOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseCodexProviderActivationRehearsalReportOptions(args []string) (codexProviderActivationRehearsalReportOptions, error) {
	opts := codexProviderActivationRehearsalReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--rehearsal":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --rehearsal")
			}
			opts.rehearsalPath = args[i+1]
			i++
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.rehearsalPath == "" {
		return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("missing --rehearsal")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationRehearsalReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderActivationRehearsal(opts codexProviderActivationRehearsalOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationRehearsal(retrievalcontext.ProviderActivationRehearsalOptions{
		ActivationPolicyPlanPath:      opts.activationPolicyPlanPath,
		ActivationApprovalPath:        opts.activationApprovalPath,
		ActivationReadinessAuditPath:  opts.activationReadinessAuditPath,
		RealCallProposalPath:          opts.realCallProposalPath,
		CredentialPolicyPlanPath:      opts.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: opts.executionSimulationReportPath,
		OutputPath:                    opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-rehearsal failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationRehearsalJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationRehearsalText(result, stdout)
		if err == nil {
			fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-rehearsal failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderActivationRehearsalReport(opts codexProviderActivationRehearsalReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationRehearsalReport(retrievalcontext.ProviderActivationRehearsalReportOptions{
		RehearsalPath:                 opts.rehearsalPath,
		ActivationPolicyPlanPath:      opts.activationPolicyPlanPath,
		ActivationApprovalPath:        opts.activationApprovalPath,
		ActivationReadinessAuditPath:  opts.activationReadinessAuditPath,
		RealCallProposalPath:          opts.realCallProposalPath,
		CredentialPolicyPlanPath:      opts.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: opts.executionSimulationReportPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-rehearsal-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationRehearsalReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationRehearsalReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-rehearsal-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderActivationReleasePackageOptions struct {
	activationPolicyPlanPath         string
	activationApprovalPath           string
	activationRehearsalPath          string
	activationReadinessAuditPath     string
	realCallProposalPath             string
	executionSimulationReportPath    string
	credentialPolicyPlanPath         string
	responseChangeProposalReportPath string
	outputPath                       string
	outputFormat                     string
}

type codexProviderActivationReleaseGateOptions struct {
	activationPolicyPlanPath         string
	activationApprovalPath           string
	activationRehearsalPath          string
	activationReadinessAuditPath     string
	realCallProposalPath             string
	executionSimulationReportPath    string
	credentialPolicyPlanPath         string
	responseChangeProposalReportPath string
	activationReleasePackagePath     string
	outputFormat                     string
}

func parseCodexProviderActivationReleasePackageOptions(args []string) (codexProviderActivationReleasePackageOptions, error) {
	opts := codexProviderActivationReleasePackageOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-rehearsal":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --activation-rehearsal")
			}
			opts.activationRehearsalPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--response-change-proposal-report":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --response-change-proposal-report")
			}
			opts.responseChangeProposalReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.outputPath == "" {
		return codexProviderActivationReleasePackageOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseCodexProviderActivationReleaseGateOptions(args []string) (codexProviderActivationReleaseGateOptions, error) {
	opts := codexProviderActivationReleaseGateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-rehearsal":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --activation-rehearsal")
			}
			opts.activationRehearsalPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--response-change-proposal-report":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --response-change-proposal-report")
			}
			opts.responseChangeProposalReportPath = args[i+1]
			i++
		case "--activation-release-package":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --activation-release-package")
			}
			opts.activationReleasePackagePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.activationReleasePackagePath == "" {
		return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("missing --activation-release-package")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationReleaseGateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderActivationReleasePackage(opts codexProviderActivationReleasePackageOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationReleasePackage(retrievalcontext.ProviderActivationReleasePackageOptions{
		ActivationPolicyPlanPath:         opts.activationPolicyPlanPath,
		ActivationApprovalPath:           opts.activationApprovalPath,
		ActivationRehearsalPath:          opts.activationRehearsalPath,
		ActivationReadinessAuditPath:     opts.activationReadinessAuditPath,
		RealCallProposalPath:             opts.realCallProposalPath,
		ExecutionSimulationReportPath:    opts.executionSimulationReportPath,
		CredentialPolicyPlanPath:         opts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: opts.responseChangeProposalReportPath,
		OutputPath:                       opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-release-package failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationReleasePackageJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationReleasePackageText(result, stdout)
		if err == nil {
			fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-release-package failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderActivationReleaseGate(opts codexProviderActivationReleaseGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationReleaseGate(retrievalcontext.ProviderActivationReleaseGateOptions{
		ActivationPolicyPlanPath:         opts.activationPolicyPlanPath,
		ActivationApprovalPath:           opts.activationApprovalPath,
		ActivationRehearsalPath:          opts.activationRehearsalPath,
		ActivationReadinessAuditPath:     opts.activationReadinessAuditPath,
		RealCallProposalPath:             opts.realCallProposalPath,
		ExecutionSimulationReportPath:    opts.executionSimulationReportPath,
		CredentialPolicyPlanPath:         opts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: opts.responseChangeProposalReportPath,
		ActivationReleasePackagePath:     opts.activationReleasePackagePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-release-gate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationReleaseGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationReleaseGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-release-gate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
