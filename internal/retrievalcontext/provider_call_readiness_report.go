package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderCallReadinessReportResult struct {
	Status                              string   `json:"status"`
	ProviderCallReadinessReady          bool     `json:"provider_call_readiness_ready"`
	ProviderCallAllowedNow              bool     `json:"provider_call_allowed_now"`
	SentToProvider                      bool     `json:"sent_to_provider"`
	ImplementationAllowsProviderCallNow bool     `json:"implementation_allows_provider_call_now"`
	BlockedReason                       string   `json:"blocked_reason"`
	ProviderRunPlanReady                bool     `json:"provider_run_plan_ready"`
	PayloadReportValidated              bool     `json:"payload_report_validated"`
	ProviderCallGateReady               bool     `json:"provider_call_gate_ready"`
	ProviderRunPlanSHA256               string   `json:"provider_run_plan_sha256"`
	PayloadReportSHA256                 string   `json:"payload_report_sha256"`
	ProviderCallGateSHA256              string   `json:"provider_call_gate_sha256"`
	PayloadOutputSHA256                 string   `json:"payload_output_sha256"`
	MaterializedSHA256                  string   `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256               string   `json:"assembled_output_sha256,omitempty"`
	Warnings                            []string `json:"warnings,omitempty"`
	Failures                            []string `json:"failures,omitempty"`
}

func LoadProviderCallReadinessReport(path string) (ProviderCallReadinessReportResult, []byte, error) {
	if err := validateRelativeSafePath("provider call readiness report path", path); err != nil {
		return ProviderCallReadinessReportResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCallReadinessReportResult{}, nil, fmt.Errorf("read provider call readiness report %q: %w", path, err)
	}
	report, err := ParseProviderCallReadinessReportJSON(data)
	if err != nil {
		return ProviderCallReadinessReportResult{}, nil, err
	}
	return report, data, nil
}

func ParseProviderCallReadinessReportJSON(data []byte) (ProviderCallReadinessReportResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallReadinessReportResult{}, fmt.Errorf("provider call readiness report must not contain materialized preview text")
	}
	var report ProviderCallReadinessReportResult
	if err := json.Unmarshal(data, &report); err != nil {
		return ProviderCallReadinessReportResult{}, fmt.Errorf("parse provider call readiness report json: %w", err)
	}
	return report, nil
}

func validateProviderCallReadinessReportForProviderCallChain(report ProviderCallReadinessReportResult) error {
	switch report.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("provider call readiness report status %q must be ok or warning", report.Status)
	}
	if !report.ProviderCallReadinessReady {
		return fmt.Errorf("provider call readiness report provider_call_readiness_ready must be true")
	}
	if report.ProviderCallAllowedNow {
		return fmt.Errorf("provider call readiness report provider_call_allowed_now must be false")
	}
	if report.SentToProvider {
		return fmt.Errorf("provider call readiness report sent_to_provider must be false")
	}
	if report.ImplementationAllowsProviderCallNow {
		return fmt.Errorf("provider call readiness report implementation_allows_provider_call_now must be false")
	}
	if report.BlockedReason != MaterializedInjectionExecutionEnableBlockedReason {
		return fmt.Errorf("provider call readiness report blocked_reason %q must be %q", report.BlockedReason, MaterializedInjectionExecutionEnableBlockedReason)
	}
	if !report.ProviderRunPlanReady {
		return fmt.Errorf("provider call readiness report provider_run_plan_ready must be true")
	}
	if !report.PayloadReportValidated {
		return fmt.Errorf("provider call readiness report payload_report_validated must be true")
	}
	if !report.ProviderCallGateReady {
		return fmt.Errorf("provider call readiness report provider_call_gate_ready must be true")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"provider_run_plan_sha256", report.ProviderRunPlanSHA256},
		{"payload_report_sha256", report.PayloadReportSHA256},
		{"provider_call_gate_sha256", report.ProviderCallGateSHA256},
		{"payload_output_sha256", report.PayloadOutputSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("provider call readiness report %s is required", field.name)
		}
	}
	return nil
}
