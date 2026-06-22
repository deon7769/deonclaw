package insights

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	ReportStatusOK     = "ok"
	ReportStatusFailed = "failed"
)

var forbiddenReportFields = []string{
	"chain_of_thought",
	"chain-of-thought",
	"hidden_reasoning",
	"reasoning_trace",
	"scratchpad",
}

type InsightReport struct {
	InsightID              string        `json:"insight_id"`
	Status                 string        `json:"status"`
	CreatedAt              string        `json:"created_at"`
	Trigger                string        `json:"trigger"`
	Scope                  EvidenceScope `json:"scope"`
	EvidenceBundleSHA256   string        `json:"evidence_bundle_sha256"`
	EvidenceBundleID       string        `json:"evidence_bundle_id,omitempty"`
	Reviewer               string        `json:"reviewer"`
	Observations           []string      `json:"observations"`
	WhatWorked             []string      `json:"what_worked"`
	WhatFailed             []string      `json:"what_failed"`
	ReusableLessons        []string      `json:"reusable_lessons"`
	Uncertainties          []string      `json:"uncertainties"`
	RiskNotes              []string      `json:"risk_notes"`
	ProposalCount          int           `json:"proposal_count"`
	ActionRequired         bool          `json:"action_required"`
	ContainsChainOfThought bool          `json:"contains_chain_of_thought"`
	SHA256                 string        `json:"sha256"`
}

func ValidateReport(report InsightReport) error {
	var errs []error
	if strings.TrimSpace(report.InsightID) == "" {
		errs = append(errs, errors.New("insight_id is required"))
	}
	if report.Status != ReportStatusOK && report.Status != ReportStatusFailed {
		errs = append(errs, fmt.Errorf("status %q is not allowed", report.Status))
	}
	if strings.TrimSpace(report.CreatedAt) == "" {
		errs = append(errs, errors.New("created_at is required"))
	}
	if err := validateTriggerType("trigger", report.Trigger); err != nil {
		errs = append(errs, err)
	}
	if strings.TrimSpace(report.EvidenceBundleSHA256) == "" {
		errs = append(errs, errors.New("evidence_bundle_sha256 is required"))
	}
	if strings.TrimSpace(report.Reviewer) == "" {
		errs = append(errs, errors.New("reviewer is required"))
	} else if _, ok := allowedReviewers[report.Reviewer]; !ok {
		errs = append(errs, fmt.Errorf("reviewer %q is not allowed", report.Reviewer))
	}
	if report.ContainsChainOfThought {
		errs = append(errs, errors.New("contains_chain_of_thought must be false"))
	}
	if report.ProposalCount < 0 {
		errs = append(errs, errors.New("proposal_count must be >= 0"))
	}
	if strings.TrimSpace(report.SHA256) == "" {
		errs = append(errs, errors.New("sha256 is required"))
	}
	return errors.Join(errs...)
}

func ReportHash(report InsightReport) (string, error) {
	copy := report
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func WriteReportJSON(report InsightReport, path string) error {
	if err := ValidateReport(report); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal insight report: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write insight report %q: %w", path, err)
	}
	return nil
}

func ReadReportJSON(path string) (InsightReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InsightReport{}, fmt.Errorf("read insight report %q: %w", path, err)
	}
	var report InsightReport
	if err := json.Unmarshal(data, &report); err != nil {
		return InsightReport{}, fmt.Errorf("parse insight report %q: %w", path, err)
	}
	if err := rejectForbiddenReportFields(data); err != nil {
		return InsightReport{}, err
	}
	return report, nil
}

func WriteReportText(report InsightReport, out io.Writer) error {
	if err := ValidateReport(report); err != nil {
		return err
	}
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "insight_id:\t%s\n", report.InsightID)
	fmt.Fprintf(writer, "status:\t%s\n", report.Status)
	fmt.Fprintf(writer, "created_at:\t%s\n", report.CreatedAt)
	fmt.Fprintf(writer, "trigger:\t%s\n", report.Trigger)
	fmt.Fprintf(writer, "reviewer:\t%s\n", report.Reviewer)
	fmt.Fprintf(writer, "evidence_bundle_sha256:\t%s\n", report.EvidenceBundleSHA256)
	fmt.Fprintf(writer, "proposal_count:\t%d\n", report.ProposalCount)
	fmt.Fprintf(writer, "action_required:\t%t\n", report.ActionRequired)
	_ = writer.Flush()

	sections := []struct {
		title string
		items []string
	}{
		{"observations", report.Observations},
		{"what_worked", report.WhatWorked},
		{"what_failed", report.WhatFailed},
		{"reusable_lessons", report.ReusableLessons},
		{"uncertainties", report.Uncertainties},
		{"risk_notes", report.RiskNotes},
	}
	for _, section := range sections {
		if _, err := fmt.Fprintf(out, "\n%s:\n", section.title); err != nil {
			return err
		}
		if len(section.items) == 0 {
			if _, err := fmt.Fprintln(out, "  (none)"); err != nil {
				return err
			}
			continue
		}
		for _, item := range section.items {
			if _, err := fmt.Fprintf(out, "  - %s\n", item); err != nil {
				return err
			}
		}
	}
	return nil
}

func rejectForbiddenReportFields(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse insight report json: %w", err)
	}
	for key := range raw {
		lower := strings.ToLower(strings.TrimSpace(key))
		for _, forbidden := range forbiddenReportFields {
			if lower == forbidden {
				return fmt.Errorf("insight report contains forbidden field %q", key)
			}
		}
	}
	return nil
}
