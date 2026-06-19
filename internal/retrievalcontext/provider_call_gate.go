package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderCallGateResult struct {
	Status                              string   `json:"status"`
	ProviderCallGateReady               bool     `json:"provider_call_gate_ready"`
	ProviderCallAllowedNow              bool     `json:"provider_call_allowed_now"`
	SentToProvider                      bool     `json:"sent_to_provider"`
	ImplementationAllowsProviderCallNow bool     `json:"implementation_allows_provider_call_now"`
	BlockedReason                       string   `json:"blocked_reason"`
	ProviderRunPlanSHA256               string   `json:"provider_run_plan_sha256"`
	PayloadReportSHA256                 string   `json:"payload_report_sha256"`
	PayloadOutputSHA256                 string   `json:"payload_output_sha256"`
	MaterializedSHA256                  string   `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256               string   `json:"assembled_output_sha256,omitempty"`
	ConfirmFlagUsed                     bool     `json:"confirm_flag_used,omitempty"`
	Warnings                            []string `json:"warnings,omitempty"`
	Failures                            []string `json:"failures,omitempty"`
}

func LoadProviderCallGate(path string) (ProviderCallGateResult, []byte, error) {
	if err := validateRelativeSafePath("provider call gate path", path); err != nil {
		return ProviderCallGateResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCallGateResult{}, nil, fmt.Errorf("read provider call gate %q: %w", path, err)
	}
	gate, err := ParseProviderCallGateJSON(data)
	if err != nil {
		return ProviderCallGateResult{}, nil, err
	}
	return gate, data, nil
}

func ParseProviderCallGateJSON(data []byte) (ProviderCallGateResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallGateResult{}, fmt.Errorf("provider call gate must not contain materialized preview text")
	}
	var gate ProviderCallGateResult
	if err := json.Unmarshal(data, &gate); err != nil {
		return ProviderCallGateResult{}, fmt.Errorf("parse provider call gate json: %w", err)
	}
	return gate, nil
}

func validateProviderCallGateForProviderCallChain(gate ProviderCallGateResult) error {
	switch gate.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("provider call gate status %q must be ok or warning", gate.Status)
	}
	if !gate.ProviderCallGateReady {
		return fmt.Errorf("provider call gate provider_call_gate_ready must be true")
	}
	if gate.ProviderCallAllowedNow {
		return fmt.Errorf("provider call gate provider_call_allowed_now must be false")
	}
	if gate.SentToProvider {
		return fmt.Errorf("provider call gate sent_to_provider must be false")
	}
	if gate.ImplementationAllowsProviderCallNow {
		return fmt.Errorf("provider call gate implementation_allows_provider_call_now must be false")
	}
	if gate.BlockedReason != MaterializedInjectionExecutionEnableBlockedReason {
		return fmt.Errorf("provider call gate blocked_reason %q must be %q", gate.BlockedReason, MaterializedInjectionExecutionEnableBlockedReason)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"provider_run_plan_sha256", gate.ProviderRunPlanSHA256},
		{"payload_report_sha256", gate.PayloadReportSHA256},
		{"payload_output_sha256", gate.PayloadOutputSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("provider call gate %s is required", field.name)
		}
	}
	return nil
}
