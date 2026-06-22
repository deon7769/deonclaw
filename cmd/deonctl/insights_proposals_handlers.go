package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/deon7769/deonclaw/internal/insights"
)

type insightsProposalsApproveOptions struct {
	bundlePath   string
	proposalID   string
	reviewer     string
	decision     string
	reason       string
	outputPath   string
	bundleOutput string
}

type insightsProposalsApplyDryRunOptions struct {
	bundlePath   string
	proposalID   string
	approvalPath string
	outputPath   string
}

type insightsProposalsApplyOptions struct {
	bundlePath        string
	proposalID        string
	approvalPath      string
	previewOutputPath string
	resultPath        string
	bundleOutput      string
	confirmApply      bool
}

type insightsEffectivenessRecordOptions struct {
	bundlePath         string
	proposalID         string
	runID              string
	metric             string
	value              float64
	outputPath         string
	effectivenessInput string
}

type insightsEffectivenessReportOptions struct {
	effectivenessPath string
	outputFormat      string
}

type insightsTimelineOptions struct {
	bundlePath        string
	effectivenessPath string
	outputFormat      string
}

func parseInsightsProposalsApproveOptions(args []string) (insightsProposalsApproveOptions, error) {
	var opts insightsProposalsApproveOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bundle":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--proposal":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalID = args[i+1]
			i++
		case "--reviewer":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --reviewer")
			}
			opts.reviewer = args[i+1]
			i++
		case "--decision":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --decision")
			}
			opts.decision = args[i+1]
			i++
		case "--reason":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --reason")
			}
			opts.reason = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--bundle-output":
			if i+1 >= len(args) {
				return insightsProposalsApproveOptions{}, fmt.Errorf("missing value for --bundle-output")
			}
			opts.bundleOutput = args[i+1]
			i++
		default:
			return insightsProposalsApproveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.bundlePath == "" {
		return insightsProposalsApproveOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.proposalID == "" {
		return insightsProposalsApproveOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.reviewer == "" {
		return insightsProposalsApproveOptions{}, fmt.Errorf("missing --reviewer")
	}
	if opts.decision == "" {
		return insightsProposalsApproveOptions{}, fmt.Errorf("missing --decision")
	}
	if opts.reason == "" {
		return insightsProposalsApproveOptions{}, fmt.Errorf("missing --reason")
	}
	if opts.outputPath == "" {
		return insightsProposalsApproveOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseInsightsProposalsApplyDryRunOptions(args []string) (insightsProposalsApplyDryRunOptions, error) {
	var opts insightsProposalsApplyDryRunOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bundle":
			if i+1 >= len(args) {
				return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--proposal":
			if i+1 >= len(args) {
				return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalID = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		default:
			return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.bundlePath == "" {
		return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.proposalID == "" {
		return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.outputPath == "" {
		return insightsProposalsApplyDryRunOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseInsightsProposalsApplyOptions(args []string) (insightsProposalsApplyOptions, error) {
	var opts insightsProposalsApplyOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bundle":
			if i+1 >= len(args) {
				return insightsProposalsApplyOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--proposal":
			if i+1 >= len(args) {
				return insightsProposalsApplyOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalID = args[i+1]
			i++
		case "--approval":
			if i+1 >= len(args) {
				return insightsProposalsApplyOptions{}, fmt.Errorf("missing value for --approval")
			}
			opts.approvalPath = args[i+1]
			i++
		case "--preview-output":
			if i+1 >= len(args) {
				return insightsProposalsApplyOptions{}, fmt.Errorf("missing value for --preview-output")
			}
			opts.previewOutputPath = args[i+1]
			i++
		case "--output":
			if i+1 >= len(args) {
				return insightsProposalsApplyOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.resultPath = args[i+1]
			i++
		case "--bundle-output":
			if i+1 >= len(args) {
				return insightsProposalsApplyOptions{}, fmt.Errorf("missing value for --bundle-output")
			}
			opts.bundleOutput = args[i+1]
			i++
		case "--confirm-apply":
			opts.confirmApply = true
		default:
			return insightsProposalsApplyOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.bundlePath == "" {
		return insightsProposalsApplyOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.proposalID == "" {
		return insightsProposalsApplyOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.approvalPath == "" {
		return insightsProposalsApplyOptions{}, fmt.Errorf("missing --approval")
	}
	if opts.previewOutputPath == "" {
		return insightsProposalsApplyOptions{}, fmt.Errorf("missing --preview-output")
	}
	if opts.resultPath == "" {
		return insightsProposalsApplyOptions{}, fmt.Errorf("missing --output")
	}
	if !opts.confirmApply {
		return insightsProposalsApplyOptions{}, fmt.Errorf("missing --confirm-apply")
	}
	return opts, nil
}

func loadProposalFromBundle(path string, proposalID string) (insights.LearningProposalBundle, insights.LearningProposal, error) {
	bundle, err := insights.ReadProposalBundleJSON(path)
	if err != nil {
		return insights.LearningProposalBundle{}, insights.LearningProposal{}, err
	}
	proposal, ok := insights.FindProposal(bundle, proposalID)
	if !ok {
		return insights.LearningProposalBundle{}, insights.LearningProposal{}, fmt.Errorf("proposal %q not found", proposalID)
	}
	return bundle, proposal, nil
}

func runInsightsProposalsApprove(opts insightsProposalsApproveOptions, stdout io.Writer, stderr io.Writer) int {
	bundle, proposal, err := loadProposalFromBundle(opts.bundlePath, opts.proposalID)
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals approve failed: %v\n", err)
		return 1
	}
	approval, err := insights.BuildProposalApproval(proposal, bundle, insights.BuildProposalApprovalOptions{
		Reviewer: opts.reviewer,
		Decision: opts.decision,
		Reason:   opts.reason,
	})
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals approve failed: %v\n", err)
		return 1
	}
	if err := insights.WriteApprovalJSON(approval, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "insights proposals approve failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(opts.bundleOutput) != "" {
		status := insights.ProposalStatusRejected
		if approval.Decision == insights.ApprovalDecisionApproved {
			status = insights.ProposalStatusApproved
		}
		updatedBundle, err := insights.UpdateBundleProposalStatus(bundle, proposal.ProposalID, status)
		if err != nil {
			fmt.Fprintf(stderr, "insights proposals approve failed: %v\n", err)
			return 1
		}
		if err := insights.WriteProposalBundleJSON(updatedBundle, opts.bundleOutput); err != nil {
			fmt.Fprintf(stderr, "insights proposals approve failed: %v\n", err)
			return 1
		}
	}
	fmt.Fprintf(stdout, "insights proposals approve: ok approval_id=%s decision=%s\n", approval.ApprovalID, approval.Decision)
	return 0
}

func runInsightsProposalsApplyDryRun(opts insightsProposalsApplyDryRunOptions, stdout io.Writer, stderr io.Writer) int {
	_, proposal, err := loadProposalFromBundle(opts.bundlePath, opts.proposalID)
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals apply dry-run failed: %v\n", err)
		return 1
	}
	var approval insights.LearningProposalApproval
	if strings.TrimSpace(opts.approvalPath) != "" {
		approval, err = insights.ReadApprovalJSON(opts.approvalPath)
		if err != nil {
			fmt.Fprintf(stderr, "insights proposals apply dry-run failed: %v\n", err)
			return 1
		}
	}
	result, err := insights.ApplyDryRun(insights.ApplyOptions{Proposal: proposal, Approval: approval})
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals apply dry-run failed: %v\n", err)
		return 1
	}
	if err := insights.WriteApplyDryRunJSON(result, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "insights proposals apply dry-run failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "insights proposals apply dry-run: ok status=%s would_apply=%t\n", result.Status, result.WouldApply)
	return 0
}

func runInsightsProposalsApply(opts insightsProposalsApplyOptions, stdout io.Writer, stderr io.Writer) int {
	bundle, proposal, err := loadProposalFromBundle(opts.bundlePath, opts.proposalID)
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals apply failed: %v\n", err)
		return 1
	}
	approval, err := insights.ReadApprovalJSON(opts.approvalPath)
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals apply failed: %v\n", err)
		return 1
	}
	result, err := insights.ApplyExecute(insights.ApplyExecuteOptions{
		ApplyOptions:      insights.ApplyOptions{Proposal: proposal, Approval: approval},
		PreviewOutputPath: opts.previewOutputPath,
		ConfirmApply:      opts.confirmApply,
	})
	if err != nil {
		fmt.Fprintf(stderr, "insights proposals apply failed: %v\n", err)
		return 1
	}
	if err := insights.WriteApplyExecuteJSON(result, opts.resultPath); err != nil {
		fmt.Fprintf(stderr, "insights proposals apply failed: %v\n", err)
		return 1
	}
	if result.Executed && strings.TrimSpace(opts.bundleOutput) != "" {
		updatedBundle, err := insights.UpdateBundleProposalStatus(bundle, proposal.ProposalID, insights.ProposalStatusApplied)
		if err != nil {
			fmt.Fprintf(stderr, "insights proposals apply failed: %v\n", err)
			return 1
		}
		if err := insights.WriteProposalBundleJSON(updatedBundle, opts.bundleOutput); err != nil {
			fmt.Fprintf(stderr, "insights proposals apply failed: %v\n", err)
			return 1
		}
	}
	if !result.Executed {
		fmt.Fprintf(stderr, "insights proposals apply blocked: %s\n", result.BlockedReason)
		return 1
	}
	fmt.Fprintf(stdout, "insights proposals apply: ok preview=%s\n", result.PreviewPath)
	return 0
}

func parseInsightsEffectivenessRecordOptions(args []string) (insightsEffectivenessRecordOptions, error) {
	var opts insightsEffectivenessRecordOptions
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bundle":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--proposal":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --proposal")
			}
			opts.proposalID = args[i+1]
			i++
		case "--run":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --run")
			}
			opts.runID = args[i+1]
			i++
		case "--metric":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --metric")
			}
			opts.metric = args[i+1]
			i++
		case "--value":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --value")
			}
			value, err := parseFloatArg(args[i+1])
			if err != nil {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("invalid --value: %w", err)
			}
			opts.value = value
			i++
		case "--output":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --output")
			}
			opts.outputPath = args[i+1]
			i++
		case "--effectiveness":
			if i+1 >= len(args) {
				return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing value for --effectiveness")
			}
			opts.effectivenessInput = args[i+1]
			i++
		default:
			return insightsEffectivenessRecordOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.bundlePath == "" {
		return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.proposalID == "" {
		return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing --proposal")
	}
	if opts.runID == "" {
		return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing --run")
	}
	if opts.metric == "" {
		return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing --metric")
	}
	if opts.outputPath == "" {
		return insightsEffectivenessRecordOptions{}, fmt.Errorf("missing --output")
	}
	return opts, nil
}

func parseFloatArg(value string) (float64, error) {
	var parsed float64
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &parsed)
	return parsed, err
}

func runInsightsEffectivenessRecord(opts insightsEffectivenessRecordOptions, stdout io.Writer, stderr io.Writer) int {
	_, proposal, err := loadProposalFromBundle(opts.bundlePath, opts.proposalID)
	if err != nil {
		fmt.Fprintf(stderr, "insights effectiveness record failed: %v\n", err)
		return 1
	}
	record, err := insights.RecordEffectiveness(insights.RecordEffectivenessOptions{
		Proposal: proposal,
		RunID:    opts.runID,
		Metric:   opts.metric,
		Value:    opts.value,
	})
	if err != nil {
		fmt.Fprintf(stderr, "insights effectiveness record failed: %v\n", err)
		return 1
	}
	bundle := insights.EffectivenessBundle{}
	if strings.TrimSpace(opts.effectivenessInput) != "" {
		bundle, err = insights.ReadEffectivenessBundleJSON(opts.effectivenessInput)
		if err != nil {
			fmt.Fprintf(stderr, "insights effectiveness record failed: %v\n", err)
			return 1
		}
	}
	bundle, err = insights.AppendEffectivenessRecord(bundle, record)
	if err != nil {
		fmt.Fprintf(stderr, "insights effectiveness record failed: %v\n", err)
		return 1
	}
	if err := insights.WriteEffectivenessBundleJSON(bundle, opts.outputPath); err != nil {
		fmt.Fprintf(stderr, "insights effectiveness record failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "insights effectiveness record: ok effectiveness_id=%s\n", record.EffectivenessID)
	return 0
}

func parseInsightsEffectivenessReportOptions(args []string) (insightsEffectivenessReportOptions, error) {
	opts := insightsEffectivenessReportOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--effectiveness":
			if i+1 >= len(args) {
				return insightsEffectivenessReportOptions{}, fmt.Errorf("missing value for --effectiveness")
			}
			opts.effectivenessPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return insightsEffectivenessReportOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return insightsEffectivenessReportOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.effectivenessPath == "" {
		return insightsEffectivenessReportOptions{}, fmt.Errorf("missing --effectiveness")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return insightsEffectivenessReportOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runInsightsEffectivenessReport(opts insightsEffectivenessReportOptions, stdout io.Writer, stderr io.Writer) int {
	bundle, err := insights.ReadEffectivenessBundleJSON(opts.effectivenessPath)
	if err != nil {
		fmt.Fprintf(stderr, "insights effectiveness report failed: %v\n", err)
		return 1
	}
	switch opts.outputFormat {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(bundle); err != nil {
			fmt.Fprintf(stderr, "insights effectiveness report failed: %v\n", err)
			return 1
		}
	default:
		fmt.Fprint(stdout, insights.WriteEffectivenessReportText(bundle))
	}
	return 0
}

func parseInsightsTimelineOptions(args []string) (insightsTimelineOptions, error) {
	opts := insightsTimelineOptions{outputFormat: "text"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--bundle":
			if i+1 >= len(args) {
				return insightsTimelineOptions{}, fmt.Errorf("missing value for --bundle")
			}
			opts.bundlePath = args[i+1]
			i++
		case "--effectiveness":
			if i+1 >= len(args) {
				return insightsTimelineOptions{}, fmt.Errorf("missing value for --effectiveness")
			}
			opts.effectivenessPath = args[i+1]
			i++
		case "--output-format":
			if i+1 >= len(args) {
				return insightsTimelineOptions{}, fmt.Errorf("missing value for --output-format")
			}
			opts.outputFormat = args[i+1]
			i++
		default:
			return insightsTimelineOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if opts.bundlePath == "" {
		return insightsTimelineOptions{}, fmt.Errorf("missing --bundle")
	}
	if opts.outputFormat != "text" && opts.outputFormat != "json" {
		return insightsTimelineOptions{}, fmt.Errorf("unsupported output format %q", opts.outputFormat)
	}
	return opts, nil
}

func runInsightsTimeline(opts insightsTimelineOptions, stdout io.Writer, stderr io.Writer) int {
	bundle, err := insights.ReadProposalBundleJSON(opts.bundlePath)
	if err != nil {
		fmt.Fprintf(stderr, "insights timeline failed: %v\n", err)
		return 1
	}
	var effectiveness insights.EffectivenessBundle
	if strings.TrimSpace(opts.effectivenessPath) != "" {
		effectiveness, err = insights.ReadEffectivenessBundleJSON(opts.effectivenessPath)
		if err != nil {
			fmt.Fprintf(stderr, "insights timeline failed: %v\n", err)
			return 1
		}
	}
	switch opts.outputFormat {
	case "json":
		payload := map[string]any{
			"insight_id":             bundle.InsightID,
			"evidence_bundle_sha256": bundle.EvidenceBundleSHA256,
			"proposals":              bundle.Proposals,
			"effectiveness":          effectiveness.Records,
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(payload); err != nil {
			fmt.Fprintf(stderr, "insights timeline failed: %v\n", err)
			return 1
		}
	default:
		writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(writer, "insight_id:\t%s\n", bundle.InsightID)
		fmt.Fprintf(writer, "evidence_bundle_sha256:\t%s\n", bundle.EvidenceBundleSHA256)
		fmt.Fprintln(writer, "event\tproposal_id\ttype\tstatus\tdetail")
		for _, proposal := range bundle.Proposals {
			fmt.Fprintf(writer, "proposal\t%s\t%s\t%s\t%s\n", proposal.ProposalID, proposal.Type, proposal.Status, proposal.Target)
		}
		for _, record := range effectiveness.Records {
			fmt.Fprintf(writer, "effectiveness\t%s\t%s\t%s\t%s=%.4f\n", record.ProposalID, record.Metric, record.RunID, record.Metric, record.Value)
		}
		_ = writer.Flush()
	}
	return 0
}
