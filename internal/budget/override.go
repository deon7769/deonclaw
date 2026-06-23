package budget

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const OverrideApprovalArtifactName = "budget-override-approval.json"

type OverrideApproval struct {
	ApprovalID        string `json:"approval_id"`
	PolicyID          string `json:"policy_id"`
	AgentID           string `json:"agent_id,omitempty"`
	WorkItemID        string `json:"work_item_id,omitempty"`
	RequestedMicroUSD int64  `json:"requested_microusd"`
	Reviewer          string `json:"reviewer"`
	Reason            string `json:"reason"`
	Decision          string `json:"decision"`
	CreatedAt         string `json:"created_at"`
	SHA256            string `json:"sha256"`
}

type NewOverrideApprovalOptions struct {
	ApprovalID        string
	PolicyID          string
	AgentID           string
	WorkItemID        string
	RequestedMicroUSD int64
	Reviewer          string
	Reason            string
	Now               time.Time
}

func BuildOverrideApproval(opts NewOverrideApprovalOptions) (OverrideApproval, error) {
	if strings.TrimSpace(opts.PolicyID) == "" {
		return OverrideApproval{}, fmt.Errorf("policy id is required")
	}
	if opts.RequestedMicroUSD <= 0 {
		return OverrideApproval{}, fmt.Errorf("requested_microusd must be positive")
	}
	if strings.TrimSpace(opts.Reviewer) == "" {
		return OverrideApproval{}, fmt.Errorf("reviewer is required")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	approvalID := strings.TrimSpace(opts.ApprovalID)
	if approvalID == "" {
		approvalID = fmt.Sprintf("budovr_%d", now.UnixNano())
	}
	approval := OverrideApproval{
		ApprovalID:        approvalID,
		PolicyID:          opts.PolicyID,
		AgentID:           strings.TrimSpace(opts.AgentID),
		WorkItemID:        strings.TrimSpace(opts.WorkItemID),
		RequestedMicroUSD: opts.RequestedMicroUSD,
		Reviewer:          strings.TrimSpace(opts.Reviewer),
		Reason:            strings.TrimSpace(opts.Reason),
		Decision:          "approved",
		CreatedAt:         now.Format(time.RFC3339Nano),
	}
	hash, err := OverrideApprovalSHA256(approval)
	if err != nil {
		return OverrideApproval{}, err
	}
	approval.SHA256 = hash
	return approval, nil
}

func OverrideApprovalSHA256(approval OverrideApproval) (string, error) {
	copy := approval
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("marshal override approval: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func ValidateOverrideApproval(approval OverrideApproval) error {
	if strings.TrimSpace(approval.ApprovalID) == "" {
		return fmt.Errorf("approval id is required")
	}
	if approval.Decision != "approved" {
		return fmt.Errorf("override decision %q is not approved", approval.Decision)
	}
	if approval.SHA256 == "" {
		return fmt.Errorf("override approval sha256 is required")
	}
	expected, err := OverrideApprovalSHA256(approval)
	if err != nil {
		return err
	}
	if approval.SHA256 != expected {
		return fmt.Errorf("override approval sha256 mismatch")
	}
	return nil
}
