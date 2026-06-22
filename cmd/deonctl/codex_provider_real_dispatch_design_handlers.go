package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderSecretReadProposalNewOptions struct {
	credentialPolicyPlanPath  string
	activationFinalAuditPath  string
	activationCIReportPath    string
	activationReleaseGatePath string
	outputPath                string
	outputFormat              string
}

type codexProviderSecretReadProposalInspectOptions struct {
	proposalPath              string
	credentialPolicyPlanPath  string
	activationFinalAuditPath  string
	activationCIReportPath    string
	activationReleaseGatePath string
	outputFormat              string
}

type codexProviderRealTransportImplementationPlanOptions struct {
	secretReadProposalPath   string
	providerAdapterPlanPath  string
	activationFinalAuditPath string
	operatorReviewBundlePath string
	killSwitchPlanPath       string
	outputPath               string
	outputFormat             string
}

type codexProviderRealTransportImplementationReportOptions struct {
	realTransportImplementationPlanPath string
	secretReadProposalPath              string
	providerAdapterPlanPath             string
	activationFinalAuditPath            string
	operatorReviewBundlePath            string
	killSwitchPlanPath                  string
	outputFormat                        string
}

type codexProviderRealDispatchDesignOptions struct {
	realTransportImplementationPlanPath string
	secretReadProposalPath              string
	activationFinalAuditPath            string
	activationCIReportPath              string
	providerRequestEnvelopePath         string
	providerRealCallProposalPath        string
	outputPath                          string
	outputFormat                        string
}

type codexProviderRealDispatchDesignReportOptions struct {
	realDispatchDesignPath              string
	realTransportImplementationPlanPath string
	secretReadProposalPath              string
	activationFinalAuditPath            string
	activationCIReportPath              string
	providerRequestEnvelopePath         string
	providerRealCallProposalPath        string
	outputFormat                        string
}

type codexProviderRealActivationDesignReviewPackageOptions struct {
	secretReadProposalPath              string
	realTransportImplementationPlanPath string
	realDispatchDesignPath              string
	activationFinalAuditPath            string
	activationCIReportPath              string
	killSwitchPlanPath                  string
	operatorReviewBundlePath            string
	outputPath                          string
	outputFormat                        string
}

type codexProviderRealActivationDesignReviewGateOptions struct {
	designReviewPackagePath             string
	secretReadProposalPath              string
	realTransportImplementationPlanPath string
	realDispatchDesignPath              string
	activationFinalAuditPath            string
	activationCIReportPath              string
	killSwitchPlanPath                  string
	operatorReviewBundlePath            string
	outputFormat                        string
}

func parseOutputFormatFlag(args []string, defaultFormat string) (string, int, error) {
	format := defaultFormat
	for i := 0; i < len(args); i++ {
		if args[i] == "--output-format" {
			if i+1 >= len(args) {
				return "", 0, fmt.Errorf("missing value for --output-format")
			}
			format = args[i+1]
			return format, i + 2, nil
		}
	}
	return format, 0, nil
}

func parseCodexProviderSecretReadProposalNewOptions(args []string) (codexProviderSecretReadProposalNewOptions, error) {
	opts := codexProviderSecretReadProposalNewOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--activation-release-gate":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("missing value for --activation-release-gate")
			}
			opts.activationReleaseGatePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--credential-policy-plan":  opts.credentialPolicyPlanPath,
		"--activation-final-audit":  opts.activationFinalAuditPath,
		"--activation-ci-report":    opts.activationCIReportPath,
		"--activation-release-gate": opts.activationReleaseGatePath,
		"--output":                  opts.outputPath,
	}); err != nil {
		return codexProviderSecretReadProposalNewOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderSecretReadProposalNewOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderSecretReadProposalInspectOptions(args []string) (codexProviderSecretReadProposalInspectOptions, error) {
	opts := codexProviderSecretReadProposalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--proposal":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalPath = args[i+1]
			i++
		case "--credential-policy-plan":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("missing value for --credential-policy-plan")
			}
			opts.credentialPolicyPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--activation-release-gate":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("missing value for --activation-release-gate")
			}
			opts.activationReleaseGatePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--proposal":                opts.proposalPath,
		"--credential-policy-plan":  opts.credentialPolicyPlanPath,
		"--activation-final-audit":  opts.activationFinalAuditPath,
		"--activation-ci-report":    opts.activationCIReportPath,
		"--activation-release-gate": opts.activationReleaseGatePath,
	}); err != nil {
		return codexProviderSecretReadProposalInspectOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderSecretReadProposalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealTransportImplementationPlanOptions(args []string) (codexProviderRealTransportImplementationPlanOptions, error) {
	opts := codexProviderRealTransportImplementationPlanOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--provider-adapter-plan":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --provider-adapter-plan")
			}
			opts.providerAdapterPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--secret-read-proposal":   opts.secretReadProposalPath,
		"--provider-adapter-plan":  opts.providerAdapterPlanPath,
		"--activation-final-audit": opts.activationFinalAuditPath,
		"--operator-review-bundle": opts.operatorReviewBundlePath,
		"--kill-switch-plan":       opts.killSwitchPlanPath,
		"--output":                 opts.outputPath,
	}); err != nil {
		return codexProviderRealTransportImplementationPlanOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealTransportImplementationPlanOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealTransportImplementationReportOptions(args []string) (codexProviderRealTransportImplementationReportOptions, error) {
	opts := codexProviderRealTransportImplementationReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--provider-adapter-plan":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --provider-adapter-plan")
			}
			opts.providerAdapterPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--provider-adapter-plan":              opts.providerAdapterPlanPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--operator-review-bundle":             opts.operatorReviewBundlePath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
	}); err != nil {
		return codexProviderRealTransportImplementationReportOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealTransportImplementationReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealDispatchDesignOptions(args []string) (codexProviderRealDispatchDesignOptions, error) {
	opts := codexProviderRealDispatchDesignOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--provider-request-envelope":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --provider-request-envelope")
			}
			opts.providerRequestEnvelopePath = args[i+1]
			i++
		case "--provider-real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --provider-real-call-proposal")
			}
			opts.providerRealCallProposalPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--provider-request-envelope":          opts.providerRequestEnvelopePath,
		"--provider-real-call-proposal":        opts.providerRealCallProposalPath,
		"--output":                             opts.outputPath,
	}); err != nil {
		return codexProviderRealDispatchDesignOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealDispatchDesignOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealDispatchDesignReportOptions(args []string) (codexProviderRealDispatchDesignReportOptions, error) {
	opts := codexProviderRealDispatchDesignReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--provider-request-envelope":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --provider-request-envelope")
			}
			opts.providerRequestEnvelopePath = args[i+1]
			i++
		case "--provider-real-call-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --provider-real-call-proposal")
			}
			opts.providerRealCallProposalPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--provider-request-envelope":          opts.providerRequestEnvelopePath,
		"--provider-real-call-proposal":        opts.providerRealCallProposalPath,
	}); err != nil {
		return codexProviderRealDispatchDesignReportOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealDispatchDesignReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealActivationDesignReviewPackageOptions(args []string) (codexProviderRealActivationDesignReviewPackageOptions, error) {
	opts := codexProviderRealActivationDesignReviewPackageOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
		"--operator-review-bundle":             opts.operatorReviewBundlePath,
		"--output":                             opts.outputPath,
	}); err != nil {
		return codexProviderRealActivationDesignReviewPackageOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealActivationDesignReviewPackageOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealActivationDesignReviewGateOptions(args []string) (codexProviderRealActivationDesignReviewGateOptions, error) {
	opts := codexProviderRealActivationDesignReviewGateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--design-review-package":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --design-review-package")
			}
			opts.designReviewPackagePath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--design-review-package":              opts.designReviewPackagePath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
		"--operator-review-bundle":             opts.operatorReviewBundlePath,
	}); err != nil {
		return codexProviderRealActivationDesignReviewGateOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealActivationDesignReviewGateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runCodexProviderSecretReadProposalNew(opts codexProviderSecretReadProposalNewOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.NewProviderSecretReadProposal(retrievalcontext.ProviderSecretReadProposalNewOptions{
		CredentialPolicyPlanPath:  opts.credentialPolicyPlanPath,
		ActivationFinalAuditPath:  opts.activationFinalAuditPath,
		ActivationCIReportPath:    opts.activationCIReportPath,
		ActivationReleaseGatePath: opts.activationReleaseGatePath,
		OutputPath:                opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-secret-read-proposal new failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-secret-read-proposal new failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderSecretReadProposalJSON(result, stdout)
	default:
		_, err = fmt.Fprintf(stdout, "worker_codex_provider_secret_read_proposal_new:\n  status: %s\n  secret_read_proposal_ready: %t\n  blocked_reason: %s\n",
			result.Status, result.SecretReadProposalReady, result.BlockedReason)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-secret-read-proposal new failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderSecretReadProposalInspect(opts codexProviderSecretReadProposalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectProviderSecretReadProposal(opts.proposalPath, retrievalcontext.ProviderSecretReadProposalInspectOptions{
		CredentialPolicyPlanPath:  opts.credentialPolicyPlanPath,
		ActivationFinalAuditPath:  opts.activationFinalAuditPath,
		ActivationCIReportPath:    opts.activationCIReportPath,
		ActivationReleaseGatePath: opts.activationReleaseGatePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-secret-read-proposal inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-secret-read-proposal inspect failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderSecretReadProposalInspectJSON(result, stdout)
	default:
		_, err = fmt.Fprintf(stdout, "worker_codex_provider_secret_read_proposal_inspect:\n  status: %s\n  secret_read_proposal_ready: %t\n  blocked_reason: %s\n",
			result.Status, result.SecretReadProposalReady, result.BlockedReason)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-secret-read-proposal inspect failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealTransportImplementationPlan(opts codexProviderRealTransportImplementationPlanOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealTransportImplementationPlan(retrievalcontext.ProviderRealTransportImplementationPlanOptions{
		SecretReadProposalPath:   opts.secretReadProposalPath,
		ProviderAdapterPlanPath:  opts.providerAdapterPlanPath,
		ActivationFinalAuditPath: opts.activationFinalAuditPath,
		OperatorReviewBundlePath: opts.operatorReviewBundlePath,
		KillSwitchPlanPath:       opts.killSwitchPlanPath,
		OutputPath:               opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-transport-implementation-plan failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-transport-implementation-plan failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealTransportImplementationPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealTransportImplementationPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-transport-implementation-plan failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealTransportImplementationReport(opts codexProviderRealTransportImplementationReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealTransportImplementationReport(retrievalcontext.ProviderRealTransportImplementationReportOptions{
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		ProviderAdapterPlanPath:             opts.providerAdapterPlanPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		OperatorReviewBundlePath:            opts.operatorReviewBundlePath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-transport-implementation-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-transport-implementation-report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealTransportImplementationPlanJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealTransportImplementationPlanText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-transport-implementation-report failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealDispatchDesign(opts codexProviderRealDispatchDesignOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchDesign(retrievalcontext.ProviderRealDispatchDesignOptions{
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		ProviderRequestEnvelopePath:         opts.providerRequestEnvelopePath,
		ProviderRealCallProposalPath:        opts.providerRealCallProposalPath,
		OutputPath:                          opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-design failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-design failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchDesignJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchDesignText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-design failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealDispatchDesignReport(opts codexProviderRealDispatchDesignReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchDesignReport(retrievalcontext.ProviderRealDispatchDesignReportOptions{
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		ProviderRequestEnvelopePath:         opts.providerRequestEnvelopePath,
		ProviderRealCallProposalPath:        opts.providerRealCallProposalPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-design-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-design-report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchDesignJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchDesignText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-design-report failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealActivationDesignReviewPackage(opts codexProviderRealActivationDesignReviewPackageOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealActivationDesignReviewPackage(retrievalcontext.ProviderRealActivationDesignReviewPackageOptions{
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
		OperatorReviewBundlePath:            opts.operatorReviewBundlePath,
		OutputPath:                          opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-activation-design-review-package failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-activation-design-review-package failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealActivationDesignReviewPackageJSON(result, stdout)
	default:
		_, err = fmt.Fprintf(stdout, "worker_codex_provider_real_activation_design_review_package:\n  status: %s\n  real_activation_design_review_ready: %t\n  blocked_reason: %s\n",
			result.Status, result.RealActivationDesignReviewReady, result.BlockedReason)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-activation-design-review-package failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealActivationDesignReviewGate(opts codexProviderRealActivationDesignReviewGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealActivationDesignReviewGate(retrievalcontext.ProviderRealActivationDesignReviewGateOptions{
		DesignReviewPackagePath:             opts.designReviewPackagePath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
		OperatorReviewBundlePath:            opts.operatorReviewBundlePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-activation-design-review-gate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-activation-design-review-gate failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealActivationDesignReviewGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealActivationDesignReviewGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-activation-design-review-gate failed: %v\n", err)
		return 1
	}
	return 0
}
