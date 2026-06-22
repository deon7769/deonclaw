package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderActivationOperatorReviewBundleOptions struct {
	activationReleasePackagePath     string
	activationReleaseGatePath        string
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

type codexProviderActivationOperatorReviewReportOptions struct {
	operatorReviewBundlePath         string
	activationReleasePackagePath     string
	activationReleaseGatePath        string
	activationPolicyPlanPath         string
	activationApprovalPath           string
	activationRehearsalPath          string
	activationReadinessAuditPath     string
	realCallProposalPath             string
	executionSimulationReportPath    string
	credentialPolicyPlanPath         string
	responseChangeProposalReportPath string
	outputFormat                     string
}

type codexProviderActivationKillSwitchValidateOptions struct {
	configPath   string
	outputFormat string
}

type codexProviderActivationKillSwitchPlanOptions struct {
	configPath   string
	outputPath   string
	outputFormat string
}

type codexProviderActivationFinalAuditOptions struct {
	activationReleasePackagePath     string
	activationReleaseGatePath        string
	operatorReviewBundlePath         string
	killSwitchPlanPath               string
	activationPolicyPlanPath         string
	activationReadinessAuditPath     string
	realCallProposalPath             string
	credentialPolicyPlanPath         string
	responseChangeProposalReportPath string
	activationApprovalPath           string
	activationRehearsalPath          string
	executionSimulationReportPath    string
	outputFormat                     string
}

type codexProviderActivationCIReportOptions struct {
	codexProviderActivationFinalAuditOptions
}

func parseCodexProviderActivationOperatorReviewBundleOptions(args []string) (codexProviderActivationOperatorReviewBundleOptions, error) {
	opts := codexProviderActivationOperatorReviewBundleOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activation-release-package":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --activation-release-package")
			}
			opts.activationReleasePackagePath = args[i+1]
			i++
		case "--activation-release-gate":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --activation-release-gate")
			}
			opts.activationReleaseGatePath = args[i+1]
			i++
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-rehearsal":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --activation-rehearsal")
			}
			opts.activationRehearsalPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--response-change-proposal-report":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --response-change-proposal-report")
			}
			opts.responseChangeProposalReportPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.activationReleasePackagePath == "" || opts.outputPath == "" {
		return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("missing required flags")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationOperatorReviewBundleOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderActivationOperatorReviewReportOptions(args []string) (codexProviderActivationOperatorReviewReportOptions, error) {
	opts := codexProviderActivationOperatorReviewReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--activation-release-package":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --activation-release-package")
			}
			opts.activationReleasePackagePath = args[i+1]
			i++
		case "--activation-release-gate":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --activation-release-gate")
			}
			opts.activationReleaseGatePath = args[i+1]
			i++
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-rehearsal":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --activation-rehearsal")
			}
			opts.activationRehearsalPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--response-change-proposal-report":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --response-change-proposal-report")
			}
			opts.responseChangeProposalReportPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.operatorReviewBundlePath == "" {
		return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("missing --operator-review-bundle")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationOperatorReviewReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderActivationKillSwitchValidateOptions(args []string) (codexProviderActivationKillSwitchValidateOptions, error) {
	opts := codexProviderActivationKillSwitchValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderActivationKillSwitchValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationKillSwitchValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationKillSwitchValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderActivationKillSwitchValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationKillSwitchValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderActivationKillSwitchPlanOptions(args []string) (codexProviderActivationKillSwitchPlanOptions, error) {
	opts := codexProviderActivationKillSwitchPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderActivationKillSwitchPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderActivationKillSwitchPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationKillSwitchPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationKillSwitchPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" || opts.outputPath == "" {
		return codexProviderActivationKillSwitchPlanOptions{}, fmt.Errorf("missing required flags")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationKillSwitchPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderActivationFinalAuditOptions(args []string) (codexProviderActivationFinalAuditOptions, error) {
	opts := codexProviderActivationFinalAuditOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--activation-release-package":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --activation-release-package")
			}
			opts.activationReleasePackagePath = args[i+1]
			i++
		case "--activation-release-gate":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --activation-release-gate")
			}
			opts.activationReleaseGatePath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--activation-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --activation-policy-plan")
			}
			opts.activationPolicyPlanPath = args[i+1]
			i++
		case "--activation-readiness-audit":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --activation-readiness-audit")
			}
			opts.activationReadinessAuditPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--response-change-proposal-report":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --response-change-proposal-report")
			}
			opts.responseChangeProposalReportPath = args[i+1]
			i++
		case "--activation-approval":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --activation-approval")
			}
			opts.activationApprovalPath = args[i+1]
			i++
		case "--activation-rehearsal":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --activation-rehearsal")
			}
			opts.activationRehearsalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.executionSimulationReportPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.activationReleasePackagePath == "" || opts.activationReleaseGatePath == "" || opts.operatorReviewBundlePath == "" || opts.killSwitchPlanPath == "" {
		return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("missing required flags")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationFinalAuditOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderActivationCIReportOptions(args []string) (codexProviderActivationCIReportOptions, error) {
	auditOpts, err := parseCodexProviderActivationFinalAuditOptions(args)
	if err != nil {
		return codexProviderActivationCIReportOptions{}, err
	}
	return codexProviderActivationCIReportOptions{codexProviderActivationFinalAuditOptions: auditOpts}, nil
}

func runCodexProviderActivationOperatorReviewBundle(opts codexProviderActivationOperatorReviewBundleOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationOperatorReviewBundle(retrievalcontext.ProviderActivationOperatorReviewBundleOptions{
		ActivationReleasePackagePath:     opts.activationReleasePackagePath,
		ActivationReleaseGatePath:        opts.activationReleaseGatePath,
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
		fmt.Fprintf(stderr, "worker codex provider-activation-operator-review-bundle failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-activation-operator-review-bundle failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationOperatorReviewBundleJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationOperatorReviewBundleText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-operator-review-bundle failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderActivationOperatorReviewReport(opts codexProviderActivationOperatorReviewReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationOperatorReviewReport(retrievalcontext.ProviderActivationOperatorReviewReportOptions{
		OperatorReviewBundlePath:         opts.operatorReviewBundlePath,
		ActivationReleasePackagePath:     opts.activationReleasePackagePath,
		ActivationReleaseGatePath:        opts.activationReleaseGatePath,
		ActivationPolicyPlanPath:         opts.activationPolicyPlanPath,
		ActivationApprovalPath:           opts.activationApprovalPath,
		ActivationRehearsalPath:          opts.activationRehearsalPath,
		ActivationReadinessAuditPath:     opts.activationReadinessAuditPath,
		RealCallProposalPath:             opts.realCallProposalPath,
		ExecutionSimulationReportPath:    opts.executionSimulationReportPath,
		CredentialPolicyPlanPath:         opts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: opts.responseChangeProposalReportPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-operator-review-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-activation-operator-review-report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationOperatorReviewReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationOperatorReviewReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-operator-review-report failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderActivationKillSwitchValidate(opts codexProviderActivationKillSwitchValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadProviderActivationKillSwitchConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.ProviderActivationKillSwitchValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch validate failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationKillSwitchValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationKillSwitchValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch validate failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderActivationKillSwitchPlan(opts codexProviderActivationKillSwitchPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationKillSwitchPlan(retrievalcontext.ProviderActivationKillSwitchPlanOptions{
		ConfigPath: opts.configPath,
		OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch plan failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch plan failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationKillSwitchPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationKillSwitchPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-kill-switch plan failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderActivationFinalAudit(opts codexProviderActivationFinalAuditOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationFinalAudit(retrievalcontext.ProviderActivationFinalAuditOptions{
		ActivationReleasePackagePath:     opts.activationReleasePackagePath,
		ActivationReleaseGatePath:        opts.activationReleaseGatePath,
		OperatorReviewBundlePath:         opts.operatorReviewBundlePath,
		KillSwitchPlanPath:               opts.killSwitchPlanPath,
		ActivationPolicyPlanPath:         opts.activationPolicyPlanPath,
		ActivationReadinessAuditPath:     opts.activationReadinessAuditPath,
		RealCallProposalPath:             opts.realCallProposalPath,
		CredentialPolicyPlanPath:         opts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: opts.responseChangeProposalReportPath,
		ActivationApprovalPath:           opts.activationApprovalPath,
		ActivationRehearsalPath:          opts.activationRehearsalPath,
		ExecutionSimulationReportPath:    opts.executionSimulationReportPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-final-audit failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-activation-final-audit failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationFinalAuditJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationFinalAuditText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-final-audit failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderActivationCIReport(opts codexProviderActivationCIReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationCIReport(retrievalcontext.ProviderActivationCIReportOptions{
		ProviderActivationFinalAuditOptions: retrievalcontext.ProviderActivationFinalAuditOptions{
			ActivationReleasePackagePath:     opts.activationReleasePackagePath,
			ActivationReleaseGatePath:        opts.activationReleaseGatePath,
			OperatorReviewBundlePath:         opts.operatorReviewBundlePath,
			KillSwitchPlanPath:               opts.killSwitchPlanPath,
			ActivationPolicyPlanPath:         opts.activationPolicyPlanPath,
			ActivationReadinessAuditPath:     opts.activationReadinessAuditPath,
			RealCallProposalPath:             opts.realCallProposalPath,
			CredentialPolicyPlanPath:         opts.credentialPolicyPlanPath,
			ResponseChangeProposalReportPath: opts.responseChangeProposalReportPath,
			ActivationApprovalPath:           opts.activationApprovalPath,
			ActivationRehearsalPath:          opts.activationRehearsalPath,
			ExecutionSimulationReportPath:    opts.executionSimulationReportPath,
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-ci-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-activation-ci-report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationCIReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationCIReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-ci-report failed: %v\n", err)
		return 1
	}
	return 0
}
