package insights

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ApprovalDecisionApproved = "approved"
	ApprovalDecisionRejected = "rejected"
)

type LearningProposalApproval struct {
	ApprovalID           string `json:"approval_id"`
	ProposalID           string `json:"proposal_id"`
	ProposalSHA256       string `json:"proposal_sha256"`
	BundleSHA256         string `json:"bundle_sha256"`
	InsightID            string `json:"insight_id"`
	EvidenceBundleSHA256 string `json:"evidence_bundle_sha256"`
	Reviewer             string `json:"reviewer"`
	Decision             string `json:"decision"`
	Reason               string `json:"reason"`
	CreatedAt            string `json:"created_at"`
	SHA256               string `json:"sha256"`
}

type BuildProposalApprovalOptions struct {
	Reviewer   string
	Decision   string
	Reason     string
	ApprovalID string
	Now        time.Time
}

func BuildProposalApproval(proposal LearningProposal, bundle LearningProposalBundle, opts BuildProposalApprovalOptions) (LearningProposalApproval, error) {
	if err := ValidateProposal(proposal); err != nil {
		return LearningProposalApproval{}, fmt.Errorf("proposal: %w", err)
	}
	if err := ValidateProposalBundle(bundle); err != nil {
		return LearningProposalApproval{}, fmt.Errorf("bundle: %w", err)
	}
	if proposal.InsightID != bundle.InsightID {
		return LearningProposalApproval{}, errors.New("proposal insight_id does not match bundle")
	}
	if proposal.EvidenceBundleSHA256 != bundle.EvidenceBundleSHA256 {
		return LearningProposalApproval{}, errors.New("proposal evidence_bundle_sha256 does not match bundle")
	}

	reviewer := strings.TrimSpace(opts.Reviewer)
	if reviewer == "" {
		return LearningProposalApproval{}, errors.New("reviewer is required")
	}
	if _, ok := allowedReviewers[reviewer]; !ok {
		return LearningProposalApproval{}, fmt.Errorf("reviewer %q is not allowed", reviewer)
	}

	decision := strings.TrimSpace(opts.Decision)
	switch decision {
	case ApprovalDecisionApproved, ApprovalDecisionRejected:
	default:
		return LearningProposalApproval{}, fmt.Errorf("decision %q is not allowed", decision)
	}

	reason := strings.TrimSpace(opts.Reason)
	if reason == "" {
		return LearningProposalApproval{}, errors.New("reason is required")
	}

	if decision == ApprovalDecisionApproved && proposal.ApprovalRequired && proposal.Status != ProposalStatusPending {
		return LearningProposalApproval{}, fmt.Errorf("proposal status %q cannot be approved", proposal.Status)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	approvalID := strings.TrimSpace(opts.ApprovalID)
	if approvalID == "" {
		approvalID = newApprovalID(proposal.ProposalID, reviewer, now)
	}

	approval := LearningProposalApproval{
		ApprovalID:           approvalID,
		ProposalID:           proposal.ProposalID,
		ProposalSHA256:       proposal.SHA256,
		BundleSHA256:         bundle.SHA256,
		InsightID:            proposal.InsightID,
		EvidenceBundleSHA256: proposal.EvidenceBundleSHA256,
		Reviewer:             reviewer,
		Decision:             decision,
		Reason:               reason,
		CreatedAt:            now.Format(time.RFC3339Nano),
	}

	hash, err := ApprovalHash(approval)
	if err != nil {
		return LearningProposalApproval{}, err
	}
	approval.SHA256 = hash

	if err := ValidateApproval(approval); err != nil {
		return LearningProposalApproval{}, err
	}
	if err := scanJSONForSecrets(approval); err != nil {
		return LearningProposalApproval{}, err
	}
	return approval, nil
}

func ValidateApproval(approval LearningProposalApproval) error {
	var errs []error
	if strings.TrimSpace(approval.ApprovalID) == "" {
		errs = append(errs, errors.New("approval_id is required"))
	}
	if strings.TrimSpace(approval.ProposalID) == "" {
		errs = append(errs, errors.New("proposal_id is required"))
	}
	if strings.TrimSpace(approval.ProposalSHA256) == "" {
		errs = append(errs, errors.New("proposal_sha256 is required"))
	}
	if strings.TrimSpace(approval.BundleSHA256) == "" {
		errs = append(errs, errors.New("bundle_sha256 is required"))
	}
	if strings.TrimSpace(approval.EvidenceBundleSHA256) == "" {
		errs = append(errs, errors.New("evidence_bundle_sha256 is required"))
	}
	if strings.TrimSpace(approval.Reviewer) == "" {
		errs = append(errs, errors.New("reviewer is required"))
	}
	switch approval.Decision {
	case ApprovalDecisionApproved, ApprovalDecisionRejected:
	default:
		errs = append(errs, fmt.Errorf("decision %q is not allowed", approval.Decision))
	}
	if strings.TrimSpace(approval.Reason) == "" {
		errs = append(errs, errors.New("reason is required"))
	}
	if strings.TrimSpace(approval.SHA256) == "" {
		errs = append(errs, errors.New("sha256 is required"))
	}
	return errors.Join(errs...)
}

func ValidateApprovalAgainstProposal(approval LearningProposalApproval, proposal LearningProposal) error {
	if err := ValidateApproval(approval); err != nil {
		return err
	}
	if err := ValidateProposal(proposal); err != nil {
		return err
	}
	if approval.ProposalID != proposal.ProposalID {
		return fmt.Errorf("approval proposal_id %q does not match proposal %q", approval.ProposalID, proposal.ProposalID)
	}
	if approval.ProposalSHA256 != proposal.SHA256 {
		return errors.New("approval proposal_sha256 is stale")
	}
	if approval.EvidenceBundleSHA256 != proposal.EvidenceBundleSHA256 {
		return errors.New("approval evidence_bundle_sha256 does not match proposal")
	}
	return nil
}

func ApprovalHash(approval LearningProposalApproval) (string, error) {
	copy := approval
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func newApprovalID(proposalID string, reviewer string, now time.Time) string {
	sum := sha256.Sum256([]byte(proposalID + "|" + reviewer + "|" + now.Format(time.RFC3339Nano)))
	return "lpa_" + hex.EncodeToString(sum[:8])
}

func WriteApprovalJSON(approval LearningProposalApproval, path string) error {
	if err := ValidateApproval(approval); err != nil {
		return err
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal learning proposal approval: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write learning proposal approval %q: %w", path, err)
	}
	return nil
}

func ReadApprovalJSON(path string) (LearningProposalApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LearningProposalApproval{}, fmt.Errorf("read learning proposal approval %q: %w", path, err)
	}
	var approval LearningProposalApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return LearningProposalApproval{}, fmt.Errorf("parse learning proposal approval %q: %w", path, err)
	}
	if err := ValidateApproval(approval); err != nil {
		return LearningProposalApproval{}, err
	}
	return approval, nil
}

func WithProposalStatus(proposal LearningProposal, status string) (LearningProposal, error) {
	proposal.Status = status
	hash, err := ProposalHash(proposal)
	if err != nil {
		return LearningProposal{}, err
	}
	proposal.SHA256 = hash
	if err := ValidateProposal(proposal); err != nil {
		return LearningProposal{}, err
	}
	return proposal, nil
}

func UpdateBundleProposalStatus(bundle LearningProposalBundle, proposalID string, status string) (LearningProposalBundle, error) {
	found := false
	for i, proposal := range bundle.Proposals {
		if proposal.ProposalID != proposalID {
			continue
		}
		updated, err := WithProposalStatus(proposal, status)
		if err != nil {
			return LearningProposalBundle{}, err
		}
		bundle.Proposals[i] = updated
		found = true
		break
	}
	if !found {
		return LearningProposalBundle{}, fmt.Errorf("proposal %q not found in bundle", proposalID)
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
