package insights

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type ReviewerResponse struct {
	Observations    []string                `json:"observations"`
	WhatWorked      []string                `json:"what_worked"`
	WhatFailed      []string                `json:"what_failed"`
	ReusableLessons []string                `json:"reusable_lessons"`
	Uncertainties   []string                `json:"uncertainties"`
	RiskNotes       []string                `json:"risk_notes"`
	Proposals       []ReviewerProposalDraft `json:"proposals"`
	ActionRequired  bool                    `json:"action_required"`
}

type ReviewerProposalDraft struct {
	Type                  string  `json:"type"`
	Target                string  `json:"target"`
	Reason                string  `json:"reason"`
	ProposedChangeSummary string  `json:"proposed_change_summary"`
	Confidence            float64 `json:"confidence"`
}

type EvaluationPlan struct {
	Reviewer             string `json:"reviewer"`
	EvidenceBundleID     string `json:"evidence_bundle_id"`
	EvidenceBundleSHA256 string `json:"evidence_bundle_sha256"`
	Trigger              string `json:"trigger"`
	WorkerExecution      bool   `json:"worker_execution"`
	PromptPath           string `json:"prompt_path,omitempty"`
	PromptSHA256         string `json:"prompt_sha256,omitempty"`
	ResponseRequired     bool   `json:"response_required"`
	CreatedAt            string `json:"created_at"`
}

type EvaluateOptions struct {
	Evidence     EvidenceBundle
	Reviewer     string
	ResponsePath string
	Now          time.Time
}

func BuildReviewerPrompt(bundle EvidenceBundle) (string, error) {
	summary, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", err
	}
	var prompt bytes.Buffer
	prompt.WriteString("You are the DeonClaw learning reviewer.\n")
	prompt.WriteString("Review only the provided evidence bundle and referenced summaries.\n")
	prompt.WriteString("Do not infer secrets or unstated facts.\n")
	prompt.WriteString("Produce observations, reusable lessons, uncertainties, and proposals.\n")
	prompt.WriteString("Do not claim a proposal should be applied automatically unless policy allows it.\n")
	prompt.WriteString("Do not include chain-of-thought.\n\n")
	prompt.WriteString("Respond with JSON only using this shape:\n")
	prompt.WriteString(`{
  "observations": ["..."],
  "what_worked": ["..."],
  "what_failed": ["..."],
  "reusable_lessons": ["..."],
  "uncertainties": ["..."],
  "risk_notes": ["..."],
  "proposals": [
    {
      "type": "skill_patch",
      "target": "...",
      "reason": "...",
      "proposed_change_summary": "...",
      "confidence": 0.0
    }
  ],
  "action_required": true
}`)
	prompt.WriteString("\n\nEvidence bundle:\n")
	prompt.Write(summary)
	prompt.WriteByte('\n')
	return prompt.String(), nil
}

func PlanEvaluation(bundle EvidenceBundle, reviewer string) (EvaluationPlan, string, error) {
	reviewer = strings.TrimSpace(reviewer)
	if reviewer == "" {
		return EvaluationPlan{}, "", errors.New("reviewer is required")
	}
	if _, ok := allowedReviewers[reviewer]; !ok {
		return EvaluationPlan{}, "", fmt.Errorf("reviewer %q is not allowed", reviewer)
	}
	if strings.TrimSpace(bundle.SHA256) == "" {
		return EvaluationPlan{}, "", errors.New("evidence bundle sha256 is required")
	}

	prompt, err := BuildReviewerPrompt(bundle)
	if err != nil {
		return EvaluationPlan{}, "", err
	}

	now := time.Now().UTC()
	plan := EvaluationPlan{
		Reviewer:             reviewer,
		EvidenceBundleID:     bundle.EvidenceBundleID,
		EvidenceBundleSHA256: bundle.SHA256,
		Trigger:              bundle.Trigger,
		WorkerExecution:      false,
		ResponseRequired:     true,
		CreatedAt:            now.Format(time.RFC3339Nano),
	}
	return plan, prompt, nil
}

func WriteEvaluationPrompt(prompt string, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("prompt output path is required")
	}
	if err := os.WriteFile(path, []byte(prompt), 0o644); err != nil {
		return "", fmt.Errorf("write reviewer prompt %q: %w", path, err)
	}
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:]), nil
}

func WriteEvaluationPlan(plan EvaluationPlan, path string) error {
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal evaluation plan: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write evaluation plan %q: %w", path, err)
	}
	return nil
}

func ReadReviewerResponse(path string) (ReviewerResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReviewerResponse{}, fmt.Errorf("read reviewer response %q: %w", path, err)
	}
	if err := rejectForbiddenReportFields(data); err != nil {
		return ReviewerResponse{}, err
	}
	var response ReviewerResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return ReviewerResponse{}, fmt.Errorf("parse reviewer response %q: %w", path, err)
	}
	return response, nil
}

func MaterializeInsightReport(opts EvaluateOptions) (InsightReport, error) {
	reviewer := strings.TrimSpace(opts.Reviewer)
	if reviewer == "" {
		return InsightReport{}, errors.New("reviewer is required")
	}
	if _, ok := allowedReviewers[reviewer]; !ok {
		return InsightReport{}, fmt.Errorf("reviewer %q is not allowed", reviewer)
	}
	if strings.TrimSpace(opts.Evidence.SHA256) == "" {
		return InsightReport{}, errors.New("evidence bundle sha256 is required")
	}
	responsePath := strings.TrimSpace(opts.ResponsePath)
	if responsePath == "" {
		return InsightReport{}, errors.New("reviewer response path is required; worker execution is not enabled in this sprint")
	}

	response, err := ReadReviewerResponse(responsePath)
	if err != nil {
		return InsightReport{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	report := InsightReport{
		InsightID:              newInsightID(opts.Evidence.EvidenceBundleID, reviewer, now),
		Status:                 ReportStatusOK,
		CreatedAt:              now.Format(time.RFC3339Nano),
		Trigger:                opts.Evidence.Trigger,
		Scope:                  opts.Evidence.Scope,
		EvidenceBundleSHA256:   opts.Evidence.SHA256,
		EvidenceBundleID:       opts.Evidence.EvidenceBundleID,
		Reviewer:               reviewer,
		Observations:           nonNilStrings(response.Observations),
		WhatWorked:             nonNilStrings(response.WhatWorked),
		WhatFailed:             nonNilStrings(response.WhatFailed),
		ReusableLessons:        nonNilStrings(response.ReusableLessons),
		Uncertainties:          nonNilStrings(response.Uncertainties),
		RiskNotes:              nonNilStrings(response.RiskNotes),
		ProposalCount:          len(response.Proposals),
		ActionRequired:         response.ActionRequired,
		ContainsChainOfThought: false,
	}

	hash, err := ReportHash(report)
	if err != nil {
		return InsightReport{}, err
	}
	report.SHA256 = hash

	if err := ValidateReport(report); err != nil {
		return InsightReport{}, err
	}
	if err := scanJSONForSecrets(report); err != nil {
		return InsightReport{}, err
	}
	return report, nil
}

func newInsightID(evidenceBundleID string, reviewer string, now time.Time) string {
	sum := sha256.Sum256([]byte(evidenceBundleID + "|" + reviewer + "|" + now.Format(time.RFC3339Nano)))
	return "ins_" + hex.EncodeToString(sum[:8])
}

func nonNilStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if out == nil {
		return []string{}
	}
	return out
}
