package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/tasks"
)

type DelegationProposal struct {
	ProposalID       string   `json:"proposal_id"`
	ParentAgentID    string   `json:"parent_agent_id"`
	ChildAgentID     string   `json:"child_agent_id"`
	ParentWorkItemID string   `json:"parent_work_item_id,omitempty"`
	TaskID           string   `json:"task_id"`
	Title            string   `json:"title"`
	AllowedPaths     []string `json:"allowed_paths"`
	ForbiddenPaths   []string `json:"forbidden_paths"`
	Reason           string   `json:"reason"`
	Depth            int      `json:"depth"`
	ApprovalRequired bool     `json:"approval_required"`
	CreatedAt        string   `json:"created_at"`
	SHA256           string   `json:"sha256"`
}

type BuildDelegationProposalOptions struct {
	ParentAgent    Agent
	ChildAgent     Agent
	ParentWorkItem WorkItem
	Task           tasks.Task
	Reason         string
	Depth          int
	MaxDepth       int
	Now            time.Time
}

func BuildDelegationProposal(opts BuildDelegationProposalOptions) (DelegationProposal, error) {
	if err := CanReceiveWork(opts.ChildAgent); err != nil {
		return DelegationProposal{}, fmt.Errorf("child agent: %w", err)
	}
	reason := strings.TrimSpace(opts.Reason)
	if reason == "" {
		return DelegationProposal{}, errors.New("reason is required")
	}
	maxDepth := opts.MaxDepth
	if maxDepth == 0 {
		maxDepth = DefaultMaxDelegationDepth
	}
	depth := opts.Depth
	if depth == 0 {
		depth = 1
	}
	if depth > maxDepth {
		return DelegationProposal{}, fmt.Errorf("delegation depth %d exceeds max %d", depth, maxDepth)
	}
	if opts.ChildAgent.Role == "manager" && opts.ParentAgent.Role != "manager" {
		return DelegationProposal{}, fmt.Errorf("delegating to manager role child from non-manager parent requires explicit review")
	}
	if !PathsSubset(opts.Task.AllowedPaths, opts.ParentWorkItem.AllowedPaths) {
		return DelegationProposal{}, errors.New("child task paths must be a subset of parent work item paths")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	proposal := DelegationProposal{
		ProposalID:       newDelegationProposalID(opts.ParentAgent.ID, opts.ChildAgent.ID, now),
		ParentAgentID:    opts.ParentAgent.ID,
		ChildAgentID:     opts.ChildAgent.ID,
		ParentWorkItemID: opts.ParentWorkItem.ID,
		TaskID:           opts.Task.ID,
		Title:            opts.Task.Title,
		AllowedPaths:     append([]string(nil), opts.Task.AllowedPaths...),
		ForbiddenPaths:   append([]string(nil), opts.Task.ForbiddenPaths...),
		Reason:           reason,
		Depth:            depth,
		ApprovalRequired: true,
		CreatedAt:        now.Format(time.RFC3339Nano),
	}
	hash, err := delegationProposalHash(proposal)
	if err != nil {
		return DelegationProposal{}, err
	}
	proposal.SHA256 = hash
	return proposal, nil
}

func delegationProposalHash(proposal DelegationProposal) (string, error) {
	copy := proposal
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func newDelegationProposalID(parentID string, childID string, now time.Time) string {
	sum := sha256.Sum256([]byte(parentID + "|" + childID + "|" + now.Format(time.RFC3339Nano)))
	return "dlg_" + hex.EncodeToString(sum[:8])
}

func WriteDelegationProposalJSON(proposal DelegationProposal, path string) error {
	data, err := json.MarshalIndent(proposal, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal delegation proposal: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write delegation proposal %q: %w", path, err)
	}
	return nil
}

type TerminationApproval struct {
	ApprovalID string `json:"approval_id"`
	AgentID    string `json:"agent_id"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason"`
	Reviewer   string `json:"reviewer"`
	CreatedAt  string `json:"created_at"`
}

func ValidateTerminationApproval(approval TerminationApproval, agentID string) error {
	if strings.TrimSpace(approval.AgentID) != strings.TrimSpace(agentID) {
		return fmt.Errorf("approval agent_id %q does not match %q", approval.AgentID, agentID)
	}
	if strings.TrimSpace(approval.Decision) != "approved" {
		return fmt.Errorf("termination requires decision approved, got %q", approval.Decision)
	}
	if strings.TrimSpace(approval.Reason) == "" {
		return errors.New("approval reason is required")
	}
	return nil
}

func ReadTerminationApprovalJSON(path string) (TerminationApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TerminationApproval{}, fmt.Errorf("read termination approval %q: %w", path, err)
	}
	var approval TerminationApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return TerminationApproval{}, fmt.Errorf("parse termination approval %q: %w", path, err)
	}
	return approval, nil
}
