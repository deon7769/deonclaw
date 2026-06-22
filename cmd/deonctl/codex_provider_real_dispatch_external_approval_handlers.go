package main

import (
	"fmt"
	"io"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

type codexProviderRealDispatchExternalApprovalNewOptions struct {
	designReviewPackagePath             string
	designReviewGatePath                string
	secretReadProposalPath              string
	realDispatchDesignPath              string
	realTransportImplementationPlanPath string
	activationFinalAuditPath            string
	activationCIReportPath              string
	killSwitchPlanPath                  string
	operatorReviewBundlePath            string
	outputPath                          string
}

type codexProviderRealDispatchExternalApprovalApproveOptions struct {
	requestPath                      string
	outputPath                       string
	confirmDesignReviewPackageSHA256 string
	confirmDesignReviewGateSHA256    string
	confirmSecretReadProposalSHA256  string
	confirmRealDispatchDesignSHA256  string
	confirmProviderPayloadSHA256     string
}

type codexProviderRealDispatchExternalApprovalInspectOptions struct {
	approvalPath         string
	requestPath          string
	designReviewGatePath string
	outputFormat         string
}

type codexProviderRealDispatchRunbookGenerateOptions struct {
	externalApprovalPath                string
	externalApprovalRequestPath         string
	designReviewGatePath                string
	realDispatchDesignPath              string
	secretReadProposalPath              string
	realTransportImplementationPlanPath string
	activationFinalAuditPath            string
	activationCIReportPath              string
	killSwitchPlanPath                  string
	outputPath                          string
}

type codexProviderRealDispatchRunbookReportOptions struct {
	codexProviderRealDispatchRunbookGenerateOptions
	runbookPath  string
	outputFormat string
}

type codexProviderRealDispatchRiskRegisterOptions struct {
	runbookPath                         string
	externalApprovalPath                string
	designReviewGatePath                string
	secretReadProposalPath              string
	realTransportImplementationPlanPath string
	realDispatchDesignPath              string
	activationFinalAuditPath            string
	killSwitchPlanPath                  string
	outputPath                          string
}

type codexProviderRealDispatchRiskReportOptions struct {
	codexProviderRealDispatchRiskRegisterOptions
	riskRegisterPath string
	outputFormat     string
}

type codexProviderRealDispatchPreimplementationGateOptions struct {
	externalApprovalPath                string
	runbookPath                         string
	riskRegisterPath                    string
	designReviewGatePath                string
	secretReadProposalPath              string
	realTransportImplementationPlanPath string
	realDispatchDesignPath              string
	activationFinalAuditPath            string
	activationCIReportPath              string
	killSwitchPlanPath                  string
	operatorReviewBundlePath            string
	outputFormat                        string
}

func parseCodexProviderRealDispatchExternalApprovalNewOptions(args []string) (codexProviderRealDispatchExternalApprovalNewOptions, error) {
	opts := codexProviderRealDispatchExternalApprovalNewOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--design-review-package":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --design-review-package")
			}
			opts.designReviewPackagePath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderRealDispatchExternalApprovalNewOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--design-review-package":              opts.designReviewPackagePath,
		"--design-review-gate":                 opts.designReviewGatePath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
		"--operator-review-bundle":             opts.operatorReviewBundlePath,
		"--output":                             opts.outputPath,
	}); err != nil {
		return codexProviderRealDispatchExternalApprovalNewOptions{}, err
	}
	return opts, nil
}

func parseCodexProviderRealDispatchExternalApprovalApproveOptions(args []string) (codexProviderRealDispatchExternalApprovalApproveOptions, error) {
	opts := codexProviderRealDispatchExternalApprovalApproveOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--request":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--confirm-design-review-package-sha256":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-design-review-package-sha256")
			}
			opts.confirmDesignReviewPackageSHA256 = args[i+1]
			i++
		case "--confirm-design-review-gate-sha256":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-design-review-gate-sha256")
			}
			opts.confirmDesignReviewGateSHA256 = args[i+1]
			i++
		case "--confirm-secret-read-proposal-sha256":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-secret-read-proposal-sha256")
			}
			opts.confirmSecretReadProposalSHA256 = args[i+1]
			i++
		case "--confirm-real-dispatch-design-sha256":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-real-dispatch-design-sha256")
			}
			opts.confirmRealDispatchDesignSHA256 = args[i+1]
			i++
		case "--confirm-provider-payload-sha256":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("missing value for --confirm-provider-payload-sha256")
			}
			opts.confirmProviderPayloadSHA256 = args[i+1]
			i++
		default:
			return codexProviderRealDispatchExternalApprovalApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--request":                              opts.requestPath,
		"--output":                               opts.outputPath,
		"--confirm-design-review-package-sha256": opts.confirmDesignReviewPackageSHA256,
		"--confirm-design-review-gate-sha256":    opts.confirmDesignReviewGateSHA256,
		"--confirm-secret-read-proposal-sha256":  opts.confirmSecretReadProposalSHA256,
		"--confirm-real-dispatch-design-sha256":  opts.confirmRealDispatchDesignSHA256,
		"--confirm-provider-payload-sha256":      opts.confirmProviderPayloadSHA256,
	}); err != nil {
		return codexProviderRealDispatchExternalApprovalApproveOptions{}, err
	}
	return opts, nil
}

func parseCodexProviderRealDispatchExternalApprovalInspectOptions(args []string) (codexProviderRealDispatchExternalApprovalInspectOptions, error) {
	opts := codexProviderRealDispatchExternalApprovalInspectOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--approval":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--request":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("missing value for --request")
			}
			opts.requestPath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.approvalPath == "" {
		return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealDispatchExternalApprovalInspectOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealDispatchRunbookGenerateOptions(args []string) (codexProviderRealDispatchRunbookGenerateOptions, error) {
	opts := codexProviderRealDispatchRunbookGenerateOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--external-approval":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --external-approval")
			}
			opts.externalApprovalPath = args[i+1]
			i++
		case "--external-approval-request":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --external-approval-request")
			}
			opts.externalApprovalRequestPath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderRealDispatchRunbookGenerateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--external-approval":                  opts.externalApprovalPath,
		"--external-approval-request":          opts.externalApprovalRequestPath,
		"--design-review-gate":                 opts.designReviewGatePath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
		"--output":                             opts.outputPath,
	}); err != nil {
		return codexProviderRealDispatchRunbookGenerateOptions{}, err
	}
	return opts, nil
}

func parseCodexProviderRealDispatchRunbookReportOptions(args []string) (codexProviderRealDispatchRunbookReportOptions, error) {
	opts := codexProviderRealDispatchRunbookReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runbook":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --runbook")
			}
			opts.runbookPath = args[i+1]
			i++
		case "--external-approval":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --external-approval")
			}
			opts.externalApprovalPath = args[i+1]
			i++
		case "--external-approval-request":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --external-approval-request")
			}
			opts.externalApprovalRequestPath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.runbookPath == "" {
		return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("missing --runbook")
	}
	if err := requireCLIPaths(map[string]string{
		"--external-approval":                  opts.externalApprovalPath,
		"--external-approval-request":          opts.externalApprovalRequestPath,
		"--design-review-gate":                 opts.designReviewGatePath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
	}); err != nil {
		return codexProviderRealDispatchRunbookReportOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealDispatchRunbookReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealDispatchRiskRegisterOptions(args []string) (codexProviderRealDispatchRiskRegisterOptions, error) {
	opts := codexProviderRealDispatchRiskRegisterOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--runbook":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --runbook")
			}
			opts.runbookPath = args[i+1]
			i++
		case "--external-approval":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --external-approval")
			}
			opts.externalApprovalPath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return codexProviderRealDispatchRiskRegisterOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--runbook":                            opts.runbookPath,
		"--external-approval":                  opts.externalApprovalPath,
		"--design-review-gate":                 opts.designReviewGatePath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
		"--output":                             opts.outputPath,
	}); err != nil {
		return codexProviderRealDispatchRiskRegisterOptions{}, err
	}
	return opts, nil
}

func parseCodexProviderRealDispatchRiskReportOptions(args []string) (codexProviderRealDispatchRiskReportOptions, error) {
	opts := codexProviderRealDispatchRiskReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--risk-register":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --risk-register")
			}
			opts.riskRegisterPath = args[i+1]
			i++
		case "--runbook":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --runbook")
			}
			opts.runbookPath = args[i+1]
			i++
		case "--external-approval":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --external-approval")
			}
			opts.externalApprovalPath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.riskRegisterPath == "" {
		return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("missing --risk-register")
	}
	if err := requireCLIPaths(map[string]string{
		"--runbook":                            opts.runbookPath,
		"--external-approval":                  opts.externalApprovalPath,
		"--design-review-gate":                 opts.designReviewGatePath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
	}); err != nil {
		return codexProviderRealDispatchRiskReportOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealDispatchRiskReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func parseCodexProviderRealDispatchPreimplementationGateOptions(args []string) (codexProviderRealDispatchPreimplementationGateOptions, error) {
	opts := codexProviderRealDispatchPreimplementationGateOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--external-approval":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --external-approval")
			}
			opts.externalApprovalPath = args[i+1]
			i++
		case "--runbook":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --runbook")
			}
			opts.runbookPath = args[i+1]
			i++
		case "--risk-register":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --risk-register")
			}
			opts.riskRegisterPath = args[i+1]
			i++
		case "--design-review-gate":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --design-review-gate")
			}
			opts.designReviewGatePath = args[i+1]
			i++
		case "--secret-read-proposal":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --secret-read-proposal")
			}
			opts.secretReadProposalPath = args[i+1]
			i++
		case "--real-transport-implementation-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --real-transport-implementation-plan")
			}
			opts.realTransportImplementationPlanPath = args[i+1]
			i++
		case "--real-dispatch-design":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --real-dispatch-design")
			}
			opts.realDispatchDesignPath = args[i+1]
			i++
		case "--activation-final-audit":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --activation-final-audit")
			}
			opts.activationFinalAuditPath = args[i+1]
			i++
		case "--activation-ci-report":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --activation-ci-report")
			}
			opts.activationCIReportPath = args[i+1]
			i++
		case "--kill-switch-plan":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --kill-switch-plan")
			}
			opts.killSwitchPlanPath = args[i+1]
			i++
		case "--operator-review-bundle":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --operator-review-bundle")
			}
			opts.operatorReviewBundlePath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if err := requireCLIPaths(map[string]string{
		"--external-approval":                  opts.externalApprovalPath,
		"--runbook":                            opts.runbookPath,
		"--risk-register":                      opts.riskRegisterPath,
		"--design-review-gate":                 opts.designReviewGatePath,
		"--secret-read-proposal":               opts.secretReadProposalPath,
		"--real-transport-implementation-plan": opts.realTransportImplementationPlanPath,
		"--real-dispatch-design":               opts.realDispatchDesignPath,
		"--activation-final-audit":             opts.activationFinalAuditPath,
		"--activation-ci-report":               opts.activationCIReportPath,
		"--kill-switch-plan":                   opts.killSwitchPlanPath,
		"--operator-review-bundle":             opts.operatorReviewBundlePath,
	}); err != nil {
		return codexProviderRealDispatchPreimplementationGateOptions{}, err
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return codexProviderRealDispatchPreimplementationGateOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func providerRealDispatchPreimplementationLibraryOptions(opts codexProviderRealDispatchPreimplementationGateOptions) retrievalcontext.ProviderRealDispatchPreimplementationGateOptions {
	return retrievalcontext.ProviderRealDispatchPreimplementationGateOptions{
		ExternalApprovalPath:                opts.externalApprovalPath,
		RunbookPath:                         opts.runbookPath,
		RiskRegisterPath:                    opts.riskRegisterPath,
		DesignReviewGatePath:                opts.designReviewGatePath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
		OperatorReviewBundlePath:            opts.operatorReviewBundlePath,
	}
}

func runCodexProviderRealDispatchExternalApprovalNew(opts codexProviderRealDispatchExternalApprovalNewOptions, stdout io.Writer, stderr io.Writer) int {
	_, err := retrievalcontext.NewProviderRealDispatchExternalApprovalRequest(retrievalcontext.NewProviderRealDispatchExternalApprovalRequestOptions{
		DesignReviewPackagePath:             opts.designReviewPackagePath,
		DesignReviewGatePath:                opts.designReviewGatePath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
		OperatorReviewBundlePath:            opts.operatorReviewBundlePath,
		OutputPath:                          opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-external-approval new failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-real-dispatch-external-approval new: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderRealDispatchExternalApprovalApprove(opts codexProviderRealDispatchExternalApprovalApproveOptions, stdout io.Writer, stderr io.Writer) int {
	_, err := retrievalcontext.ApproveProviderRealDispatchExternalApproval(retrievalcontext.ApproveProviderRealDispatchExternalApprovalOptions{
		RequestPath:                      opts.requestPath,
		OutputPath:                       opts.outputPath,
		ConfirmDesignReviewPackageSHA256: opts.confirmDesignReviewPackageSHA256,
		ConfirmDesignReviewGateSHA256:    opts.confirmDesignReviewGateSHA256,
		ConfirmSecretReadProposalSHA256:  opts.confirmSecretReadProposalSHA256,
		ConfirmRealDispatchDesignSHA256:  opts.confirmRealDispatchDesignSHA256,
		ConfirmProviderPayloadSHA256:     opts.confirmProviderPayloadSHA256,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-external-approval approve failed: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-real-dispatch-external-approval approve: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderRealDispatchExternalApprovalInspect(opts codexProviderRealDispatchExternalApprovalInspectOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.InspectProviderRealDispatchExternalApproval(opts.approvalPath, retrievalcontext.InspectProviderRealDispatchExternalApprovalOptions{
		RequestPath:          opts.requestPath,
		DesignReviewGatePath: opts.designReviewGatePath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-external-approval inspect failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchExternalApprovalInspectJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchExternalApprovalInspectText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-external-approval inspect failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		return 1
	}
	return 0
}

func runCodexProviderRealDispatchRunbookGenerate(opts codexProviderRealDispatchRunbookGenerateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchRunbook(retrievalcontext.ProviderRealDispatchRunbookOptions{
		ExternalApprovalPath:                opts.externalApprovalPath,
		ExternalApprovalRequestPath:         opts.externalApprovalRequestPath,
		DesignReviewGatePath:                opts.designReviewGatePath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
		OutputPath:                          opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-runbook generate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-runbook generate failed: %v\n", result.Failures)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-real-dispatch-runbook generate: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderRealDispatchRunbookReport(opts codexProviderRealDispatchRunbookReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchRunbookReport(retrievalcontext.ProviderRealDispatchRunbookReportOptions{
		RunbookPath:                         opts.runbookPath,
		ExternalApprovalPath:                opts.externalApprovalPath,
		ExternalApprovalRequestPath:         opts.externalApprovalRequestPath,
		DesignReviewGatePath:                opts.designReviewGatePath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		ActivationCIReportPath:              opts.activationCIReportPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-runbook report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-runbook report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchRunbookJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchRunbookText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-runbook report failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealDispatchRiskRegister(opts codexProviderRealDispatchRiskRegisterOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchRiskRegister(retrievalcontext.ProviderRealDispatchRiskRegisterOptions{
		RunbookPath:                         opts.runbookPath,
		ExternalApprovalPath:                opts.externalApprovalPath,
		DesignReviewGatePath:                opts.designReviewGatePath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
		OutputPath:                          opts.outputPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-risk-register failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-risk-register failed: %v\n", result.Failures)
		return 1
	}
	fmt.Fprintln(stdout, "worker codex provider-real-dispatch-risk-register: ok")
	fmt.Fprintf(stdout, "output: %s\n", opts.outputPath)
	return 0
}

func runCodexProviderRealDispatchRiskReport(opts codexProviderRealDispatchRiskReportOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchRiskReport(retrievalcontext.ProviderRealDispatchRiskReportOptions{
		RiskRegisterPath:                    opts.riskRegisterPath,
		RunbookPath:                         opts.runbookPath,
		ExternalApprovalPath:                opts.externalApprovalPath,
		DesignReviewGatePath:                opts.designReviewGatePath,
		SecretReadProposalPath:              opts.secretReadProposalPath,
		RealTransportImplementationPlanPath: opts.realTransportImplementationPlanPath,
		RealDispatchDesignPath:              opts.realDispatchDesignPath,
		ActivationFinalAuditPath:            opts.activationFinalAuditPath,
		KillSwitchPlanPath:                  opts.killSwitchPlanPath,
	})
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-risk-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-risk-report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchRiskReportJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchRiskRegisterText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-risk-report failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealDispatchPreimplementationGate(opts codexProviderRealDispatchPreimplementationGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchPreimplementationGate(providerRealDispatchPreimplementationLibraryOptions(opts))
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-preimplementation-gate failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-preimplementation-gate failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchPreimplementationGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchPreimplementationGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-preimplementation-gate failed: %v\n", err)
		return 1
	}
	return 0
}

func runCodexProviderRealDispatchPreimplementationReport(opts codexProviderRealDispatchPreimplementationGateOptions, stdout io.Writer, stderr io.Writer) int {
	result, err := retrievalcontext.ProviderRealDispatchPreimplementationReport(providerRealDispatchPreimplementationLibraryOptions(opts))
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-preimplementation-report failed: %v\n", err)
		return 1
	}
	if result.Status == lancedbpolicy.StatusFailed {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-preimplementation-report failed: %v\n", result.Failures)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		err = retrievalcontext.WriteProviderRealDispatchPreimplementationGateJSON(result, stdout)
	default:
		err = retrievalcontext.WriteProviderRealDispatchPreimplementationGateText(result, stdout)
	}
	if err != nil {
		fmt.Fprintf(stderr, "worker codex provider-real-dispatch-preimplementation-report failed: %v\n", err)
		return 1
	}
	return 0
}
