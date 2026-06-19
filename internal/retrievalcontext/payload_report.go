package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type PayloadReportResult struct {
	Status                string   `json:"status"`
	PayloadValidated      bool     `json:"payload_validated"`
	ContainsText          bool     `json:"contains_text"`
	PreviewOnly           bool     `json:"preview_only"`
	WorkerExecution       bool     `json:"worker_execution"`
	SentToProvider        bool     `json:"sent_to_provider"`
	PayloadOutputSHA256   string   `json:"payload_output_sha256"`
	MaterializedSHA256    string   `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256 string   `json:"assembled_output_sha256,omitempty"`
	ProviderRunPlanSHA256 string   `json:"provider_run_plan_sha256,omitempty"`
	Warnings              []string `json:"warnings,omitempty"`
	Failures              []string `json:"failures,omitempty"`
}

func LoadPayloadReport(path string) (PayloadReportResult, []byte, error) {
	if err := validateRelativeSafePath("payload report path", path); err != nil {
		return PayloadReportResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PayloadReportResult{}, nil, fmt.Errorf("read payload report %q: %w", path, err)
	}
	report, err := ParsePayloadReportJSON(data)
	if err != nil {
		return PayloadReportResult{}, nil, err
	}
	return report, data, nil
}

func ParsePayloadReportJSON(data []byte) (PayloadReportResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return PayloadReportResult{}, fmt.Errorf("payload report must not contain materialized preview text")
	}
	var report PayloadReportResult
	if err := json.Unmarshal(data, &report); err != nil {
		return PayloadReportResult{}, fmt.Errorf("parse payload report json: %w", err)
	}
	return report, nil
}

func validatePayloadReportForProviderCallChain(report PayloadReportResult) error {
	switch report.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("payload report status %q must be ok or warning", report.Status)
	}
	if !report.PayloadValidated {
		return fmt.Errorf("payload report payload_validated must be true")
	}
	if !report.ContainsText {
		return fmt.Errorf("payload report contains_text must be true")
	}
	if !report.PreviewOnly {
		return fmt.Errorf("payload report preview_only must be true")
	}
	if report.WorkerExecution {
		return fmt.Errorf("payload report worker_execution must be false")
	}
	if report.SentToProvider {
		return fmt.Errorf("payload report sent_to_provider must be false")
	}
	if strings.TrimSpace(report.PayloadOutputSHA256) == "" {
		return fmt.Errorf("payload report payload_output_sha256 is required")
	}
	if strings.TrimSpace(report.ProviderRunPlanSHA256) == "" {
		return fmt.Errorf("payload report provider_run_plan_sha256 is required")
	}
	return nil
}
