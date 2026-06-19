package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

const (
	AllowedUseProviderCallPolicyOnly = "provider_call_policy_only"
	confirmPayloadOutputSHA256       = "confirm_payload_output_sha256"
)

type ProviderCallApprovalRequest struct {
	Status                          string    `json:"status"`
	CreatedAt                       time.Time `json:"created_at"`
	ReadinessReportSHA256           string    `json:"readiness_report_sha256"`
	ProviderCallGateSHA256          string    `json:"provider_call_gate_sha256"`
	PayloadReportSHA256             string    `json:"payload_report_sha256"`
	PayloadOutputSHA256             string    `json:"payload_output_sha256"`
	ProviderRunPlanSHA256           string    `json:"provider_run_plan_sha256"`
	MaterializedSHA256              string    `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256           string    `json:"assembled_output_sha256,omitempty"`
	RequestedProviderCallAuthorized bool      `json:"requested_provider_call_authorized"`
	ProviderCallAllowedNow          bool      `json:"provider_call_allowed_now"`
	SentToProvider                  bool      `json:"sent_to_provider"`
	ContainsText                    bool      `json:"contains_text"`
	Warnings                        []string  `json:"warnings,omitempty"`
}

type ProviderCallApproval struct {
	Approved                        bool      `json:"approved"`
	ApprovedAt                      time.Time `json:"approved_at"`
	RequestSHA256                   string    `json:"request_sha256"`
	ReadinessReportSHA256           string    `json:"readiness_report_sha256"`
	ProviderCallGateSHA256          string    `json:"provider_call_gate_sha256"`
	PayloadReportSHA256             string    `json:"payload_report_sha256"`
	PayloadOutputSHA256             string    `json:"payload_output_sha256"`
	ProviderRunPlanSHA256           string    `json:"provider_run_plan_sha256"`
	MaterializedSHA256              string    `json:"materialized_sha256,omitempty"`
	AssembledOutputSHA256           string    `json:"assembled_output_sha256,omitempty"`
	AllowedUse                      string    `json:"allowed_use"`
	ProviderCallAuthorizedForFuture bool      `json:"provider_call_authorized_for_future"`
	ProviderCallAllowedNow          bool      `json:"provider_call_allowed_now"`
	SentToProvider                  bool      `json:"sent_to_provider"`
	ConfirmPayloadOutputSHA256      bool      `json:"confirm_payload_output_sha256"`
	Warnings                        []string  `json:"warnings,omitempty"`
}

type NewProviderCallApprovalRequestOptions struct {
	ReadinessReportPath  string
	ProviderCallGatePath string
	PayloadReportPath    string
	OutputPath           string
}

type ApproveProviderCallOptions struct {
	RequestPath                string
	OutputPath                 string
	ConfirmPayloadOutputSHA256 string
	ApprovedAt                 time.Time
}

type InspectProviderCallApprovalOptions struct {
	RequestPath string
}

type InspectProviderCallApprovalResult struct {
	Status                          string   `json:"status"`
	Approved                        bool     `json:"approved"`
	ProviderCallAuthorizedForFuture bool     `json:"provider_call_authorized_for_future"`
	ProviderCallAllowedNow          bool     `json:"provider_call_allowed_now"`
	SentToProvider                  bool     `json:"sent_to_provider"`
	AllowedUse                      string   `json:"allowed_use"`
	PayloadOutputSHA256             string   `json:"payload_output_sha256"`
	MaterializedSHA256              string   `json:"materialized_sha256,omitempty"`
	Warnings                        []string `json:"warnings"`
	Failures                        []string `json:"failures,omitempty"`
}

func NewProviderCallApprovalRequest(opts NewProviderCallApprovalRequestOptions) (ProviderCallApprovalRequest, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"readiness report path", opts.ReadinessReportPath},
		{"provider call gate path", opts.ProviderCallGatePath},
		{"payload report path", opts.PayloadReportPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallApprovalRequest{}, err
		}
	}

	readiness, readinessData, err := LoadProviderCallReadinessReport(opts.ReadinessReportPath)
	if err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := validateProviderCallReadinessReportForProviderCallChain(readiness); err != nil {
		return ProviderCallApprovalRequest{}, err
	}

	gate, gateData, err := LoadProviderCallGate(opts.ProviderCallGatePath)
	if err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := validateProviderCallGateForProviderCallChain(gate); err != nil {
		return ProviderCallApprovalRequest{}, err
	}

	payloadReport, payloadReportData, err := LoadPayloadReport(opts.PayloadReportPath)
	if err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := validatePayloadReportForProviderCallChain(payloadReport); err != nil {
		return ProviderCallApprovalRequest{}, err
	}

	readinessSHA := sha256Hex(readinessData)
	gateSHA := sha256Hex(gateData)
	payloadReportSHA := sha256Hex(payloadReportData)

	var failures []string
	if gate.ProviderRunPlanSHA256 != readiness.ProviderRunPlanSHA256 {
		failures = append(failures, "provider_run_plan_sha256 mismatch between readiness report and gate")
	}
	if gate.PayloadReportSHA256 != readiness.PayloadReportSHA256 || payloadReportSHA != readiness.PayloadReportSHA256 {
		failures = append(failures, "payload_report_sha256 mismatch across readiness report, gate, and payload report")
	}
	if gate.PayloadOutputSHA256 != readiness.PayloadOutputSHA256 || payloadReport.PayloadOutputSHA256 != readiness.PayloadOutputSHA256 {
		failures = append(failures, "payload_output_sha256 mismatch across readiness report, gate, and payload report")
	}
	if gateSHA != readiness.ProviderCallGateSHA256 {
		failures = append(failures, "provider_call_gate_sha256 mismatch between readiness report and gate artifact")
	}
	if payloadReport.ProviderRunPlanSHA256 != readiness.ProviderRunPlanSHA256 {
		failures = append(failures, "provider_run_plan_sha256 mismatch between readiness report and payload report")
	}
	if len(failures) > 0 {
		return ProviderCallApprovalRequest{}, fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	request := ProviderCallApprovalRequest{
		Status:                          RequestStatusPending,
		CreatedAt:                       time.Now().UTC(),
		ReadinessReportSHA256:           readinessSHA,
		ProviderCallGateSHA256:          gateSHA,
		PayloadReportSHA256:             payloadReportSHA,
		PayloadOutputSHA256:             payloadReport.PayloadOutputSHA256,
		ProviderRunPlanSHA256:           readiness.ProviderRunPlanSHA256,
		MaterializedSHA256:              firstNonEmpty(readiness.MaterializedSHA256, gate.MaterializedSHA256, payloadReport.MaterializedSHA256),
		AssembledOutputSHA256:           firstNonEmpty(readiness.AssembledOutputSHA256, gate.AssembledOutputSHA256, payloadReport.AssembledOutputSHA256),
		RequestedProviderCallAuthorized: true,
		ProviderCallAllowedNow:          false,
		SentToProvider:                  false,
		ContainsText:                    false,
		Warnings: mergeWarnings(
			append([]string(nil), readiness.Warnings...),
			append([]string(nil), gate.Warnings...),
			append([]string(nil), payloadReport.Warnings...),
		),
	}
	if readiness.Status == lancedbpolicy.StatusWarning {
		request.Warnings = mergeWarnings(request.Warnings, []string{"readiness report status is warning; provider call approval request recorded with documented acceptance"})
	}

	if err := request.Validate(); err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := writeProviderCallApprovalRequestJSON(opts.OutputPath, request); err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	return request, nil
}

func ApproveProviderCall(opts ApproveProviderCallOptions) (ProviderCallApproval, error) {
	if strings.TrimSpace(opts.ConfirmPayloadOutputSHA256) == "" {
		return ProviderCallApproval{}, fmt.Errorf("--confirm-payload-output-sha256 is required")
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return ProviderCallApproval{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderCallApproval{}, err
	}

	requestData, err := os.ReadFile(opts.RequestPath)
	if err != nil {
		return ProviderCallApproval{}, fmt.Errorf("read provider call approval request %q: %w", opts.RequestPath, err)
	}
	request, err := ParseProviderCallApprovalRequestJSON(requestData)
	if err != nil {
		return ProviderCallApproval{}, err
	}
	if err := request.Validate(); err != nil {
		return ProviderCallApproval{}, fmt.Errorf("provider call approval request invalid: %w", err)
	}
	if request.Status != RequestStatusPending {
		return ProviderCallApproval{}, fmt.Errorf("provider call approval request status %q must be pending", request.Status)
	}
	if request.PayloadOutputSHA256 != opts.ConfirmPayloadOutputSHA256 {
		return ProviderCallApproval{}, fmt.Errorf("confirm payload output sha256 mismatch")
	}

	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}

	approval := ProviderCallApproval{
		Approved:                        true,
		ApprovedAt:                      approvedAt.UTC(),
		RequestSHA256:                   sha256Hex(requestData),
		ReadinessReportSHA256:           request.ReadinessReportSHA256,
		ProviderCallGateSHA256:          request.ProviderCallGateSHA256,
		PayloadReportSHA256:             request.PayloadReportSHA256,
		PayloadOutputSHA256:             request.PayloadOutputSHA256,
		ProviderRunPlanSHA256:           request.ProviderRunPlanSHA256,
		MaterializedSHA256:              request.MaterializedSHA256,
		AssembledOutputSHA256:           request.AssembledOutputSHA256,
		AllowedUse:                      AllowedUseProviderCallPolicyOnly,
		ProviderCallAuthorizedForFuture: true,
		ProviderCallAllowedNow:          false,
		SentToProvider:                  false,
		ConfirmPayloadOutputSHA256:      true,
		Warnings:                        append([]string(nil), request.Warnings...),
	}
	if err := approval.Validate(); err != nil {
		return ProviderCallApproval{}, err
	}
	if err := writeProviderCallApprovalJSON(opts.OutputPath, approval); err != nil {
		return ProviderCallApproval{}, err
	}
	return approval, nil
}

func LoadProviderCallApprovalRequest(path string) (ProviderCallApprovalRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCallApprovalRequest{}, fmt.Errorf("read provider call approval request %q: %w", path, err)
	}
	return ParseProviderCallApprovalRequestJSON(data)
}

func ParseProviderCallApprovalRequestJSON(data []byte) (ProviderCallApprovalRequest, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallApprovalRequest{}, fmt.Errorf("provider call approval request must not contain materialized preview text")
	}
	var request ProviderCallApprovalRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return ProviderCallApprovalRequest{}, fmt.Errorf("parse provider call approval request json: %w", err)
	}
	return request, nil
}

func LoadProviderCallApproval(path string) (ProviderCallApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCallApproval{}, fmt.Errorf("read provider call approval %q: %w", path, err)
	}
	return ParseProviderCallApprovalJSON(data)
}

func ParseProviderCallApprovalJSON(data []byte) (ProviderCallApproval, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallApproval{}, fmt.Errorf("provider call approval must not contain materialized preview text")
	}
	var approval ProviderCallApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return ProviderCallApproval{}, fmt.Errorf("parse provider call approval json: %w", err)
	}
	return approval, nil
}

func InspectProviderCallApproval(path string, opts InspectProviderCallApprovalOptions) (InspectProviderCallApprovalResult, error) {
	if err := validateRelativeSafePath("approval path", path); err != nil {
		return InspectProviderCallApprovalResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InspectProviderCallApprovalResult{}, fmt.Errorf("read provider call approval %q: %w", path, err)
	}
	return InspectProviderCallApprovalBytes(data, opts)
}

func InspectProviderCallApprovalBytes(data []byte, opts InspectProviderCallApprovalOptions) (InspectProviderCallApprovalResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return InspectProviderCallApprovalResult{Status: lancedbpolicy.StatusFailed, Failures: []string{"approval contains forbidden field text_excerpt"}}, nil
	}
	approval, err := ParseProviderCallApprovalJSON(data)
	if err != nil {
		return InspectProviderCallApprovalResult{}, err
	}

	result := InspectProviderCallApprovalResult{
		Status:                          lancedbpolicy.StatusOK,
		Approved:                        approval.Approved,
		ProviderCallAuthorizedForFuture: approval.ProviderCallAuthorizedForFuture,
		ProviderCallAllowedNow:          approval.ProviderCallAllowedNow,
		SentToProvider:                  approval.SentToProvider,
		AllowedUse:                      approval.AllowedUse,
		PayloadOutputSHA256:             approval.PayloadOutputSHA256,
		MaterializedSHA256:              approval.MaterializedSHA256,
		Warnings:                        append([]string(nil), approval.Warnings...),
	}

	var failures []string
	if err := approval.Validate(); err != nil {
		failures = append(failures, err.Error())
	}
	if opts.RequestPath != "" {
		if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
			return InspectProviderCallApprovalResult{}, err
		}
		requestData, err := os.ReadFile(opts.RequestPath)
		if err != nil {
			return InspectProviderCallApprovalResult{}, fmt.Errorf("read provider call approval request %q: %w", opts.RequestPath, err)
		}
		if approval.RequestSHA256 != sha256Hex(requestData) {
			failures = append(failures, "request_sha256 mismatch")
		}
	}
	result.Failures = failures
	if len(failures) > 0 {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func (r ProviderCallApprovalRequest) Validate() error {
	if r.Status != RequestStatusPending {
		return fmt.Errorf("status %q must be pending", r.Status)
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"readiness_report_sha256", r.ReadinessReportSHA256},
		{"provider_call_gate_sha256", r.ProviderCallGateSHA256},
		{"payload_report_sha256", r.PayloadReportSHA256},
		{"payload_output_sha256", r.PayloadOutputSHA256},
		{"provider_run_plan_sha256", r.ProviderRunPlanSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if !r.RequestedProviderCallAuthorized {
		return fmt.Errorf("requested_provider_call_authorized must be true")
	}
	if r.ProviderCallAllowedNow {
		return fmt.Errorf("provider_call_allowed_now must be false")
	}
	if r.SentToProvider {
		return fmt.Errorf("sent_to_provider must be false")
	}
	if r.ContainsText {
		return fmt.Errorf("contains_text must be false")
	}
	return nil
}

func (a ProviderCallApproval) Validate() error {
	if !a.Approved {
		return fmt.Errorf("approved must be true")
	}
	if a.ApprovedAt.IsZero() {
		return fmt.Errorf("approved_at is required")
	}
	if !a.ProviderCallAuthorizedForFuture {
		return fmt.Errorf("provider_call_authorized_for_future must be true")
	}
	if a.ProviderCallAllowedNow {
		return fmt.Errorf("provider_call_allowed_now must be false")
	}
	if a.SentToProvider {
		return fmt.Errorf("sent_to_provider must be false")
	}
	if a.AllowedUse != AllowedUseProviderCallPolicyOnly {
		return fmt.Errorf("allowed_use %q must be %q", a.AllowedUse, AllowedUseProviderCallPolicyOnly)
	}
	if !a.ConfirmPayloadOutputSHA256 {
		return fmt.Errorf("%s must be true", confirmPayloadOutputSHA256)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"request_sha256", a.RequestSHA256},
		{"readiness_report_sha256", a.ReadinessReportSHA256},
		{"provider_call_gate_sha256", a.ProviderCallGateSHA256},
		{"payload_report_sha256", a.PayloadReportSHA256},
		{"payload_output_sha256", a.PayloadOutputSHA256},
		{"provider_run_plan_sha256", a.ProviderRunPlanSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	return nil
}

func writeProviderCallApprovalRequestJSON(path string, request ProviderCallApprovalRequest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider call approval request output dir: %w", err)
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call approval request json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider call approval request must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider call approval request %q: %w", path, err)
	}
	return nil
}

func writeProviderCallApprovalJSON(path string, approval ProviderCallApproval) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider call approval output dir: %w", err)
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call approval json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider call approval must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider call approval %q: %w", path, err)
	}
	return nil
}

func WriteInspectProviderCallApprovalText(result InspectProviderCallApprovalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_approval_inspect:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"approved", fmt.Sprintf("%t", result.Approved)},
		{"provider_call_authorized_for_future", fmt.Sprintf("%t", result.ProviderCallAuthorizedForFuture)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"allowed_use", result.AllowedUse},
		{"payload_output_sha256", result.PayloadOutputSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.Failures) > 0 {
		if _, err := fmt.Fprintln(out, "\nfailures:"); err != nil {
			return err
		}
		for _, failure := range result.Failures {
			if _, err := fmt.Fprintf(out, "- %s\n", failure); err != nil {
				return err
			}
		}
	}
	text := fmt.Sprintf("%+v", result)
	if strings.Contains(text, "text_excerpt") || strings.Contains(text, "alpha text") {
		return fmt.Errorf("provider call approval inspect text must not contain materialized preview text")
	}
	return nil
}

func WriteInspectProviderCallApprovalJSON(result InspectProviderCallApprovalResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call approval inspect json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call approval inspect json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
