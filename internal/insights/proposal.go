package insights

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	ProposalStatusPending   = "pending"
	ProposalStatusApproved  = "approved"
	ProposalStatusRejected  = "rejected"
	ProposalStatusApplied   = "applied"
	ProposalStatusCancelled = "cancelled"

	ProposalTypeMemoryAdd     = "memory_add"
	ProposalTypeMemoryReplace = "memory_replace"
	ProposalTypeSkillCreate   = "skill_create"
	ProposalTypeSkillPatch    = "skill_patch"
	ProposalTypeEvalCase      = "eval_case"
	ProposalTypeRule          = "rule"
	ProposalTypeWorkflow      = "workflow"
	ProposalTypeRoutingPolicy = "routing_policy"
	ProposalTypeBudgetPolicy  = "budget_policy"
	ProposalTypeDocumentation = "documentation"
	ProposalTypeTechnicalDebt = "technical_debt"
	ProposalTypeFollowUpTask  = "follow_up_task"
)

var allowedProposalTypes = map[string]struct{}{
	ProposalTypeMemoryAdd:     {},
	ProposalTypeMemoryReplace: {},
	ProposalTypeSkillCreate:   {},
	ProposalTypeSkillPatch:    {},
	ProposalTypeEvalCase:      {},
	ProposalTypeRule:          {},
	ProposalTypeWorkflow:      {},
	ProposalTypeRoutingPolicy: {},
	ProposalTypeBudgetPolicy:  {},
	ProposalTypeDocumentation: {},
	ProposalTypeTechnicalDebt: {},
	ProposalTypeFollowUpTask:  {},
}

var highRiskProposalTypes = map[string]struct{}{
	ProposalTypeMemoryReplace: {},
	ProposalTypeSkillPatch:    {},
	ProposalTypeRoutingPolicy: {},
	ProposalTypeBudgetPolicy:  {},
}

type LearningProposal struct {
	ProposalID            string  `json:"proposal_id"`
	Type                  string  `json:"type"`
	Status                string  `json:"status"`
	Target                string  `json:"target"`
	InsightID             string  `json:"insight_id"`
	InsightSHA256         string  `json:"insight_sha256"`
	EvidenceBundleSHA256  string  `json:"evidence_bundle_sha256"`
	Reason                string  `json:"reason"`
	ProposedChangeSummary string  `json:"proposed_change_summary"`
	PatchPath             string  `json:"patch_path,omitempty"`
	Confidence            float64 `json:"confidence"`
	AutoApplyAllowed      bool    `json:"auto_apply_allowed"`
	ApprovalRequired      bool    `json:"approval_required"`
	CreatedAt             string  `json:"created_at"`
	SHA256                string  `json:"sha256"`
}

type LearningProposalBundle struct {
	InsightID            string             `json:"insight_id"`
	InsightSHA256        string             `json:"insight_sha256"`
	EvidenceBundleSHA256 string             `json:"evidence_bundle_sha256"`
	CreatedAt            string             `json:"created_at"`
	Proposals            []LearningProposal `json:"proposals"`
	SHA256               string             `json:"sha256"`
}

type MaterializeProposalsOptions struct {
	Report   InsightReport
	Response ReviewerResponse
	Policy   Policy
	Now      time.Time
}

func MaterializeProposals(opts MaterializeProposalsOptions) (LearningProposalBundle, error) {
	if err := ValidateReport(opts.Report); err != nil {
		return LearningProposalBundle{}, fmt.Errorf("insight report: %w", err)
	}
	if strings.TrimSpace(opts.Report.EvidenceBundleSHA256) == "" {
		return LearningProposalBundle{}, errors.New("insight report evidence_bundle_sha256 is required")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	proposals := make([]LearningProposal, 0, len(opts.Response.Proposals))
	for index, draft := range opts.Response.Proposals {
		proposal, err := materializeProposal(draft, opts.Report, opts.Policy, index, now)
		if err != nil {
			return LearningProposalBundle{}, fmt.Errorf("proposal[%d]: %w", index, err)
		}
		proposals = append(proposals, proposal)
	}

	bundle := LearningProposalBundle{
		InsightID:            opts.Report.InsightID,
		InsightSHA256:        opts.Report.SHA256,
		EvidenceBundleSHA256: opts.Report.EvidenceBundleSHA256,
		CreatedAt:            now.Format(time.RFC3339Nano),
		Proposals:            proposals,
	}
	hash, err := ProposalBundleHash(bundle)
	if err != nil {
		return LearningProposalBundle{}, err
	}
	bundle.SHA256 = hash

	if err := ValidateProposalBundle(bundle); err != nil {
		return LearningProposalBundle{}, err
	}
	return bundle, nil
}

func materializeProposal(draft ReviewerProposalDraft, report InsightReport, policy Policy, index int, now time.Time) (LearningProposal, error) {
	proposalType := strings.TrimSpace(draft.Type)
	if err := validateProposalType(proposalType); err != nil {
		return LearningProposal{}, err
	}
	target := strings.TrimSpace(draft.Target)
	if target == "" {
		return LearningProposal{}, errors.New("target is required")
	}
	reason := strings.TrimSpace(draft.Reason)
	if reason == "" {
		return LearningProposal{}, errors.New("reason is required")
	}
	summary := strings.TrimSpace(draft.ProposedChangeSummary)
	if summary == "" {
		return LearningProposal{}, errors.New("proposed_change_summary is required")
	}

	proposal := LearningProposal{
		ProposalID:            newProposalID(report.InsightID, proposalType, target, index, now),
		Type:                  proposalType,
		Status:                ProposalStatusPending,
		Target:                target,
		InsightID:             report.InsightID,
		InsightSHA256:         report.SHA256,
		EvidenceBundleSHA256:  report.EvidenceBundleSHA256,
		Reason:                reason,
		ProposedChangeSummary: summary,
		Confidence:            draft.Confidence,
		AutoApplyAllowed:      autoApplyAllowed(policy, proposalType),
		ApprovalRequired:      approvalRequired(policy, proposalType),
		CreatedAt:             now.Format(time.RFC3339Nano),
	}

	hash, err := ProposalHash(proposal)
	if err != nil {
		return LearningProposal{}, err
	}
	proposal.SHA256 = hash

	if err := ValidateProposal(proposal); err != nil {
		return LearningProposal{}, err
	}
	if err := scanJSONForSecrets(proposal); err != nil {
		return LearningProposal{}, err
	}
	return proposal, nil
}

func autoApplyAllowed(policy Policy, proposalType string) bool {
	if !policy.AutoApply {
		return false
	}
	if _, highRisk := highRiskProposalTypes[proposalType]; highRisk {
		return false
	}
	return proposalType == ProposalTypeFollowUpTask
}

func approvalRequired(policy Policy, proposalType string) bool {
	_ = policy
	_ = proposalType
	return true
}

func validateProposalType(proposalType string) error {
	if _, ok := allowedProposalTypes[proposalType]; !ok {
		return fmt.Errorf("type %q is not allowed", proposalType)
	}
	return nil
}

func ValidateProposal(proposal LearningProposal) error {
	var errs []error
	if strings.TrimSpace(proposal.ProposalID) == "" {
		errs = append(errs, errors.New("proposal_id is required"))
	}
	if err := validateProposalType(proposal.Type); err != nil {
		errs = append(errs, err)
	}
	switch proposal.Status {
	case ProposalStatusPending, ProposalStatusApproved, ProposalStatusRejected, ProposalStatusApplied, ProposalStatusCancelled:
	default:
		errs = append(errs, fmt.Errorf("status %q is not allowed", proposal.Status))
	}
	if strings.TrimSpace(proposal.Target) == "" {
		errs = append(errs, errors.New("target is required"))
	}
	if strings.TrimSpace(proposal.InsightID) == "" {
		errs = append(errs, errors.New("insight_id is required"))
	}
	if strings.TrimSpace(proposal.InsightSHA256) == "" {
		errs = append(errs, errors.New("insight_sha256 is required"))
	}
	if strings.TrimSpace(proposal.EvidenceBundleSHA256) == "" {
		errs = append(errs, errors.New("evidence_bundle_sha256 is required"))
	}
	if strings.TrimSpace(proposal.Reason) == "" {
		errs = append(errs, errors.New("reason is required"))
	}
	if strings.TrimSpace(proposal.ProposedChangeSummary) == "" {
		errs = append(errs, errors.New("proposed_change_summary is required"))
	}
	if proposal.Confidence < 0 || proposal.Confidence > 1 {
		errs = append(errs, errors.New("confidence must be between 0 and 1"))
	}
	if strings.TrimSpace(proposal.SHA256) == "" {
		errs = append(errs, errors.New("sha256 is required"))
	}
	if strings.TrimSpace(proposal.CreatedAt) == "" {
		errs = append(errs, errors.New("created_at is required"))
	}
	return errors.Join(errs...)
}

func ValidateProposalBundle(bundle LearningProposalBundle) error {
	var errs []error
	if strings.TrimSpace(bundle.InsightID) == "" {
		errs = append(errs, errors.New("insight_id is required"))
	}
	if strings.TrimSpace(bundle.InsightSHA256) == "" {
		errs = append(errs, errors.New("insight_sha256 is required"))
	}
	if strings.TrimSpace(bundle.EvidenceBundleSHA256) == "" {
		errs = append(errs, errors.New("evidence_bundle_sha256 is required"))
	}
	if strings.TrimSpace(bundle.SHA256) == "" {
		errs = append(errs, errors.New("sha256 is required"))
	}
	for i, proposal := range bundle.Proposals {
		if proposal.InsightID != bundle.InsightID {
			errs = append(errs, fmt.Errorf("proposals[%d].insight_id mismatch", i))
		}
		if proposal.EvidenceBundleSHA256 != bundle.EvidenceBundleSHA256 {
			errs = append(errs, fmt.Errorf("proposals[%d].evidence_bundle_sha256 mismatch", i))
		}
		if err := ValidateProposal(proposal); err != nil {
			errs = append(errs, fmt.Errorf("proposals[%d]: %w", i, err))
		}
	}
	return errors.Join(errs...)
}

func ValidateProposalPolicy(policy Policy) error {
	if !policy.AutoApply {
		return nil
	}
	for _, proposalType := range highRiskProposalTypesList() {
		if autoApplyAllowed(policy, proposalType) {
			return fmt.Errorf("auto_apply is not allowed for high-risk proposal type %q", proposalType)
		}
	}
	return nil
}

func highRiskProposalTypesList() []string {
	return []string{
		ProposalTypeMemoryReplace,
		ProposalTypeSkillPatch,
		ProposalTypeRoutingPolicy,
		ProposalTypeBudgetPolicy,
	}
}

func ProposalHash(proposal LearningProposal) (string, error) {
	copy := proposal
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func ProposalBundleHash(bundle LearningProposalBundle) (string, error) {
	copy := bundle
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func newProposalID(insightID string, proposalType string, target string, index int, now time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%s", insightID, proposalType, target, index, now.Format(time.RFC3339Nano))))
	return "lp_" + hex.EncodeToString(sum[:8])
}

func WriteProposalBundleJSON(bundle LearningProposalBundle, path string) error {
	if err := ValidateProposalBundle(bundle); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal learning proposal bundle: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write learning proposal bundle %q: %w", path, err)
	}
	return nil
}

func ReadProposalBundleJSON(path string) (LearningProposalBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LearningProposalBundle{}, fmt.Errorf("read learning proposal bundle %q: %w", path, err)
	}
	var bundle LearningProposalBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return LearningProposalBundle{}, fmt.Errorf("parse learning proposal bundle %q: %w", path, err)
	}
	if err := ValidateProposalBundle(bundle); err != nil {
		return LearningProposalBundle{}, err
	}
	return bundle, nil
}

func FindProposal(bundle LearningProposalBundle, proposalID string) (LearningProposal, bool) {
	proposalID = strings.TrimSpace(proposalID)
	for _, proposal := range bundle.Proposals {
		if proposal.ProposalID == proposalID {
			return proposal, true
		}
	}
	return LearningProposal{}, false
}

func WriteProposalListText(bundle LearningProposalBundle, out *tabwriter.Writer) error {
	fmt.Fprintf(out, "insight_id:\t%s\n", bundle.InsightID)
	fmt.Fprintf(out, "proposal_count:\t%d\n", len(bundle.Proposals))
	fmt.Fprintln(out, "proposal_id\ttype\tstatus\ttarget\tapproval_required")
	for _, proposal := range bundle.Proposals {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%t\n",
			proposal.ProposalID,
			proposal.Type,
			proposal.Status,
			proposal.Target,
			proposal.ApprovalRequired,
		)
	}
	return out.Flush()
}

func WriteProposalShowText(proposal LearningProposal, out *tabwriter.Writer) error {
	rows := []struct {
		key   string
		value string
	}{
		{"proposal_id", proposal.ProposalID},
		{"type", proposal.Type},
		{"status", proposal.Status},
		{"target", proposal.Target},
		{"insight_id", proposal.InsightID},
		{"evidence_bundle_sha256", proposal.EvidenceBundleSHA256},
		{"reason", proposal.Reason},
		{"proposed_change_summary", proposal.ProposedChangeSummary},
		{"confidence", fmt.Sprintf("%.2f", proposal.Confidence)},
		{"auto_apply_allowed", fmt.Sprintf("%t", proposal.AutoApplyAllowed)},
		{"approval_required", fmt.Sprintf("%t", proposal.ApprovalRequired)},
		{"sha256", proposal.SHA256},
	}
	for _, row := range rows {
		fmt.Fprintf(out, "%s:\t%s\n", row.key, row.value)
	}
	return out.Flush()
}
