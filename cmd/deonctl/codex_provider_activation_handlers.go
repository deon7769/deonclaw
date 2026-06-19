package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderCredentialPolicyValidateOptions struct {
	configPath   string
	outputFormat string
}

type codexProviderCredentialPolicyPlanOptions struct {
	configPath   string
	outputPath   string
	outputFormat string
}

func parseCodexProviderCredentialPolicyValidateOptions(args []string) (codexProviderCredentialPolicyValidateOptions, error) {
	opts := codexProviderCredentialPolicyValidateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderCredentialPolicyValidateOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCredentialPolicyValidateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCredentialPolicyValidateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderCredentialPolicyValidateOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCredentialPolicyValidateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderCredentialPolicyPlanOptions(args []string) (codexProviderCredentialPolicyPlanOptions, error) {
	opts := codexProviderCredentialPolicyPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config":
			if i+1 >= len(args) {
				return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("missing value for --config")
			}
			opts.configPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.configPath == "" {
		return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("missing --config")
	}
	if opts.outputPath == "" {
		return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("missing --output")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderCredentialPolicyPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderCredentialPolicyValidate(opts codexProviderCredentialPolicyValidateOptions, stdout io.Writer, stderr io.Writer) int {
	cfg, err := retrievalcontext.LoadProviderCredentialPolicyConfig(opts.configPath)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-credential-policy validate failed: %v\n", err)
		return 1
	}
	result, err := retrievalcontext.ProviderCredentialPolicyValidate(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-credential-policy validate failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCredentialPolicyValidateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCredentialPolicyValidateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-credential-policy validate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderCredentialPolicyPlan(opts codexProviderCredentialPolicyPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderCredentialPolicyPlan(retrievalcontext.ProviderCredentialPolicyPlanOptions{
		ConfigPath: opts.configPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-credential-policy plan failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderCredentialPolicyPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderCredentialPolicyPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-credential-policy plan failed: %v\n", err)
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

type codexProviderRealCallProposalNewOptions struct {
	simulationReportPath     string
	requestEnvelopePath      string
	adapterPlanPath          string
	credentialPolicyPlanPath string
	releaseGatePath          string
	outputPath               string
}

type codexProviderRealCallProposalInspectOptions struct {
	proposalPath             string
	simulationReportPath     string
	requestEnvelopePath      string
	adapterPlanPath          string
	credentialPolicyPlanPath string
	releaseGatePath          string
	outputFormat             string
}

func parseCodexProviderRealCallProposalNewOptions(args []string) (codexProviderRealCallProposalNewOptions, error) {
	opts := codexProviderRealCallProposalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.simulationReportPath = args[i+1]
			i++
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--adapter-plan":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing value for --adapter-plan")
			}
			opts.adapterPlanPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--execution-simulation-report": opts.simulationReportPath,
		"--request-envelope":            opts.requestEnvelopePath,
		"--adapter-plan":                opts.adapterPlanPath,
		"--credential-policy-plan":      opts.credentialPolicyPlanPath,
		"--release-gate":                opts.releaseGatePath,
		"--output":                      opts.outputPath,
	} {
		if value == "" {
			return codexProviderRealCallProposalNewOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	return opts, nil
}

func parseCodexProviderRealCallProposalInspectOptions(args []string) (codexProviderRealCallProposalInspectOptions, error) {
	opts := codexProviderRealCallProposalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.simulationReportPath = args[i+1]
			i++
		case "--request-envelope":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --request-envelope")
			}
			opts.requestEnvelopePath = args[i+1]
			i++
		case "--adapter-plan":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --adapter-plan")
			}
			opts.adapterPlanPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.proposalPath == "" {
		return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealCallProposalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderRealCallProposalNew(opts codexProviderRealCallProposalNewOptions, stdout io.Writer, stderr io.Writer) int {
	proposal, err := retrievalcontext.NewProviderRealCallProposal(retrievalcontext.ProviderRealCallProposalNewOptions{
		SimulationReportPath: opts.simulationReportPath, RequestEnvelopePath: opts.requestEnvelopePath,
		AdapterPlanPath: opts.adapterPlanPath, CredentialPolicyPlanPath: opts.credentialPolicyPlanPath,
		ReleaseGatePath: opts.releaseGatePath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-call-proposal new failed: %v\n", err)
		return 1
	}
	if err := retrievalcontext.WriteProviderRealCallProposalNewText(proposal, stdout); err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-call-proposal new failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	if proposal.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderRealCallProposalInspect(opts codexProviderRealCallProposalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectProviderRealCallProposal(opts.proposalPath, retrievalcontext.ProviderRealCallProposalInspectOptions{
		SimulationReportPath: opts.simulationReportPath, RequestEnvelopePath: opts.requestEnvelopePath,
		AdapterPlanPath: opts.adapterPlanPath, CredentialPolicyPlanPath: opts.credentialPolicyPlanPath,
		ReleaseGatePath: opts.releaseGatePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-call-proposal inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealCallProposalInspectJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealCallProposalInspectText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-call-proposal inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderResponseChangeProposalOptions struct {
	responseFixturePath  string
	simulationReportPath string
	realCallProposalPath string
	outputPath           string
	outputFormat         string
}

type codexProviderResponseChangeProposalReportOptions struct {
	codexProviderResponseChangeProposalOptions
	changeProposalPath string
}

func parseCodexProviderResponseChangeProposalOptions(args []string) (codexProviderResponseChangeProposalOptions, error) {
	opts := codexProviderResponseChangeProposalOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--response-fixture":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("missing value for --response-fixture")
			}
			opts.responseFixturePath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.simulationReportPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--response-fixture": opts.responseFixturePath, "--execution-simulation-report": opts.simulationReportPath,
		"--real-call-proposal": opts.realCallProposalPath, "--output": opts.outputPath,
	} {
		if value == "" {
			return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderResponseChangeProposalOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderResponseChangeProposalReportOptions(args []string) (codexProviderResponseChangeProposalReportOptions, error) {
	opts := codexProviderResponseChangeProposalReportOptions{
		codexProviderResponseChangeProposalOptions: codexProviderResponseChangeProposalOptions{outputFormat: "text"},
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--change-proposal":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing value for --change-proposal")
			}
			opts.changeProposalPath = args[i+1]
			i++
		case "--response-fixture":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing value for --response-fixture")
			}
			opts.responseFixturePath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.simulationReportPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.changeProposalPath == "" {
		return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing --change-proposal")
	}
	for flag, value := range map[string]string{
		"--response-fixture": opts.responseFixturePath, "--execution-simulation-report": opts.simulationReportPath,
		"--real-call-proposal": opts.realCallProposalPath,
	} {
		if value == "" {
			return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderResponseChangeProposalReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderResponseChangeProposal(opts codexProviderResponseChangeProposalOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderResponseChangeProposal(retrievalcontext.ProviderResponseChangeProposalOptions{
		ResponseFixturePath: opts.responseFixturePath, SimulationReportPath: opts.simulationReportPath,
		RealCallProposalPath: opts.realCallProposalPath, OutputPath: opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-change-proposal failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderResponseChangeProposalJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderResponseChangeProposalText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-change-proposal failed: %v\n", err)
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

func runCodexProviderResponseChangeProposalReport(opts codexProviderResponseChangeProposalReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderResponseChangeProposalReport(retrievalcontext.ProviderResponseChangeProposalReportOptions{
		ChangeProposalPath: opts.changeProposalPath, ResponseFixturePath: opts.responseFixturePath,
		SimulationReportPath: opts.simulationReportPath, RealCallProposalPath: opts.realCallProposalPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-change-proposal-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderResponseChangeProposalReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderResponseChangeProposalReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-response-change-proposal-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

type codexProviderActivationReadinessAuditOptions struct {
	credentialPolicyPlanPath string
	realCallProposalPath     string
	changeProposalPath       string
	simulationReportPath     string
	releaseGatePath          string
	outputFormat             string
}

func parseCodexProviderActivationReadinessAuditOptions(args []string) (codexProviderActivationReadinessAuditOptions, error) {
	opts := codexProviderActivationReadinessAuditOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing value for --real-call-proposal")
			}
			opts.realCallProposalPath = args[i+1]
			i++
		case "--change-proposal":
			if i+1 >= len(args) {
				return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing value for --change-proposal")
			}
			opts.changeProposalPath = args[i+1]
			i++
		case "--execution-simulation-report":
			if i+1 >= len(args) {
				return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing value for --execution-simulation-report")
			}
			opts.simulationReportPath = args[i+1]
			i++
		case "--release-gate":
			if i+1 >= len(args) {
				return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing value for --release-gate")
			}
			opts.releaseGatePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	for flag, value := range map[string]string{
		"--credential-policy-plan": opts.credentialPolicyPlanPath, "--real-call-proposal": opts.realCallProposalPath,
		"--change-proposal": opts.changeProposalPath, "--execution-simulation-report": opts.simulationReportPath,
		"--release-gate": opts.releaseGatePath,
	} {
		if value == "" {
			return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("missing %s", flag)
		}
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderActivationReadinessAuditOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderActivationReadinessAudit(opts codexProviderActivationReadinessAuditOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationReadinessAudit(retrievalcontext.ProviderActivationReadinessAuditOptions{
		CredentialPolicyPlanPath: opts.credentialPolicyPlanPath, RealCallProposalPath: opts.realCallProposalPath,
		ChangeProposalPath: opts.changeProposalPath, SimulationReportPath: opts.simulationReportPath,
		ReleaseGatePath: opts.releaseGatePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-readiness-audit failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationReadinessAuditJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationReadinessAuditText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-readiness-audit failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderActivationReadinessReport(opts codexProviderActivationReadinessAuditOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderActivationReadinessReport(retrievalcontext.ProviderActivationReadinessAuditOptions{
		CredentialPolicyPlanPath: opts.credentialPolicyPlanPath, RealCallProposalPath: opts.realCallProposalPath,
		ChangeProposalPath: opts.changeProposalPath, SimulationReportPath: opts.simulationReportPath,
		ReleaseGatePath: opts.releaseGatePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-readiness-report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderActivationReadinessReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderActivationReadinessReportText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-activation-readiness-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}
