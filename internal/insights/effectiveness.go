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
	EffectivenessMetricValidationPassRate = "validation_pass_rate"
	EffectivenessMetricRepeatedFailure    = "repeated_failure_count"
	EffectivenessMetricTimeToFixMS        = "time_to_fix_ms"
)

type EffectivenessRecord struct {
	EffectivenessID string  `json:"effectiveness_id"`
	ProposalID      string  `json:"proposal_id"`
	InsightID       string  `json:"insight_id"`
	RunID           string  `json:"run_id"`
	Metric          string  `json:"metric"`
	Value           float64 `json:"value"`
	ObservedAt      string  `json:"observed_at"`
	SHA256          string  `json:"sha256"`
}

type EffectivenessBundle struct {
	ProposalID string                `json:"proposal_id"`
	InsightID  string                `json:"insight_id"`
	Records    []EffectivenessRecord `json:"records"`
	SHA256     string                `json:"sha256"`
}

type RecordEffectivenessOptions struct {
	Proposal LearningProposal
	RunID    string
	Metric   string
	Value    float64
	Now      time.Time
}

func RecordEffectiveness(opts RecordEffectivenessOptions) (EffectivenessRecord, error) {
	if err := ValidateProposal(opts.Proposal); err != nil {
		return EffectivenessRecord{}, err
	}
	runID := strings.TrimSpace(opts.RunID)
	if runID == "" {
		return EffectivenessRecord{}, errors.New("run_id is required")
	}
	metric := strings.TrimSpace(opts.Metric)
	if metric == "" {
		return EffectivenessRecord{}, errors.New("metric is required")
	}
	if err := validateEffectivenessMetric(metric); err != nil {
		return EffectivenessRecord{}, err
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}

	record := EffectivenessRecord{
		EffectivenessID: newEffectivenessID(opts.Proposal.ProposalID, runID, metric, now),
		ProposalID:      opts.Proposal.ProposalID,
		InsightID:       opts.Proposal.InsightID,
		RunID:           runID,
		Metric:          metric,
		Value:           opts.Value,
		ObservedAt:      now.Format(time.RFC3339Nano),
	}
	hash, err := effectivenessRecordHash(record)
	if err != nil {
		return EffectivenessRecord{}, err
	}
	record.SHA256 = hash
	if err := scanJSONForSecrets(record); err != nil {
		return EffectivenessRecord{}, err
	}
	return record, nil
}

func validateEffectivenessMetric(metric string) error {
	switch metric {
	case EffectivenessMetricValidationPassRate,
		EffectivenessMetricRepeatedFailure,
		EffectivenessMetricTimeToFixMS:
		return nil
	default:
		return fmt.Errorf("metric %q is not allowed", metric)
	}
}

func newEffectivenessID(proposalID string, runID string, metric string, now time.Time) string {
	sum := sha256.Sum256([]byte(proposalID + "|" + runID + "|" + metric + "|" + now.Format(time.RFC3339Nano)))
	return "lpe_" + hex.EncodeToString(sum[:8])
}

func effectivenessRecordHash(record EffectivenessRecord) (string, error) {
	copy := record
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func AppendEffectivenessRecord(bundle EffectivenessBundle, record EffectivenessRecord) (EffectivenessBundle, error) {
	if strings.TrimSpace(bundle.ProposalID) == "" {
		bundle.ProposalID = record.ProposalID
	}
	if strings.TrimSpace(bundle.InsightID) == "" {
		bundle.InsightID = record.InsightID
	}
	if bundle.ProposalID != record.ProposalID {
		return EffectivenessBundle{}, errors.New("effectiveness bundle proposal_id mismatch")
	}
	bundle.Records = append(bundle.Records, record)
	hash, err := effectivenessBundleHash(bundle)
	if err != nil {
		return EffectivenessBundle{}, err
	}
	bundle.SHA256 = hash
	return bundle, nil
}

func effectivenessBundleHash(bundle EffectivenessBundle) (string, error) {
	copy := bundle
	copy.SHA256 = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func WriteEffectivenessBundleJSON(bundle EffectivenessBundle, path string) error {
	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal effectiveness bundle: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write effectiveness bundle %q: %w", path, err)
	}
	return nil
}

func ReadEffectivenessBundleJSON(path string) (EffectivenessBundle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return EffectivenessBundle{}, fmt.Errorf("read effectiveness bundle %q: %w", path, err)
	}
	var bundle EffectivenessBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return EffectivenessBundle{}, fmt.Errorf("parse effectiveness bundle %q: %w", path, err)
	}
	return bundle, nil
}

func WriteEffectivenessReportText(bundle EffectivenessBundle) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "effectiveness_report:\n")
	fmt.Fprintf(&builder, "  proposal_id: %s\n", bundle.ProposalID)
	fmt.Fprintf(&builder, "  insight_id: %s\n", bundle.InsightID)
	fmt.Fprintf(&builder, "  record_count: %d\n", len(bundle.Records))
	for _, record := range bundle.Records {
		fmt.Fprintf(&builder, "  - run_id: %s\n", record.RunID)
		fmt.Fprintf(&builder, "    metric: %s\n", record.Metric)
		fmt.Fprintf(&builder, "    value: %.4f\n", record.Value)
		fmt.Fprintf(&builder, "    observed_at: %s\n", record.ObservedAt)
	}
	return builder.String()
}
