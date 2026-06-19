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
	confirmProviderPayloadSHA256     = "confirm_provider_payload_sha256"
)

type ProviderCallApprovalRequest struct {
	Status                          string    `json:"status"`
	CreatedAt                       time.Time `json:"created_at"`
	ReadinessReportSHA256           string    `json:"readiness_report_sha256"`
	ProviderCallGateSHA256          string    `json:"provider_call_gate_sha256"`
	PayloadReportSHA256             string    `json:"payload_report_sha256"`
	ProviderPayloadSHA256           string    `json:"provider_payload_sha256"`
	MaterializedSHA256              string    `json:"materialized_sha256,omitempty"`
	RequestedProviderCallAuthorized bool      `json:"requested_provider_call_authorized"`
	ProviderCallAllowedNow          bool      `json:"provider_call_allowed_now"`
	NetworkCallAllowedNow           bool      `json:"network_call_allowed_now"`
	WorkerExecutionAllowedNow       bool      `json:"worker_execution_allowed_now"`
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
	ProviderPayloadSHA256           string    `json:"provider_payload_sha256"`
	MaterializedSHA256              string    `json:"materialized_sha256,omitempty"`
	AllowedUse                      string    `json:"allowed_use"`
	ProviderCallAuthorizedForFuture bool      `json:"provider_call_authorized_for_future"`
	ProviderCallAllowedNow          bool      `json:"provider_call_allowed_now"`
	NetworkCallAllowedNow           bool      `json:"network_call_allowed_now"`
	WorkerExecutionAllowedNow       bool      `json:"worker_execution_allowed_now"`
	SentToProvider                  bool      `json:"sent_to_provider"`
	ConfirmProviderPayloadSHA256    bool      `json:"confirm_provider_payload_sha256"`
	Warnings                        []string  `json:"warnings,omitempty"`
}

type NewProviderCallApprovalRequestOptions struct {
	ReadinessReportPath  string
	ProviderCallGatePath string
	PayloadReportPath    string
	OutputPath           string
}

type ApproveProviderCallOptions struct {
	RequestPath                  string
	OutputPath                   string
	ConfirmProviderPayloadSHA256 string
	ApprovedAt                   time.Time
}

type InspectProviderCallApprovalOptions struct {
	RequestPath string
}

type InspectProviderCallApprovalResult struct {
	Status                          string   `json:"status"`
	Approved                        bool     `json:"approved"`
	ProviderCallAuthorizedForFuture bool     `json:"provider_call_authorized_for_future"`
	ProviderCallAllowedNow          bool     `json:"provider_call_allowed_now"`
	NetworkCallAllowedNow           bool     `json:"network_call_allowed_now"`
	WorkerExecutionAllowedNow       bool     `json:"worker_execution_allowed_now"`
	SentToProvider                  bool     `json:"sent_to_provider"`
	AllowedUse                      string   `json:"allowed_use"`
	ProviderPayloadSHA256           string   `json:"provider_payload_sha256"`
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

	readiness, readinessData, err := LoadMaterializedProviderCallReadinessReport(opts.ReadinessReportPath)
	if err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := validateMaterializedProviderCallReadinessReportForApproval(readiness); err != nil {
		return ProviderCallApprovalRequest{}, err
	}

	gate, gateData, err := LoadMaterializedProviderCallGate(opts.ProviderCallGatePath)
	if err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := validateMaterializedProviderCallGateForApproval(gate); err != nil {
		return ProviderCallApprovalRequest{}, err
	}

	payloadReport, payloadReportData, err := LoadMaterializedProviderPayloadReport(opts.PayloadReportPath)
	if err != nil {
		return ProviderCallApprovalRequest{}, err
	}
	if err := validateMaterializedProviderPayloadReportForApproval(payloadReport); err != nil {
		return ProviderCallApprovalRequest{}, err
	}

	readinessSHA := sha256Hex(readinessData)
	gateSHA := sha256Hex(gateData)
	payloadReportSHA := sha256Hex(payloadReportData)

	var failures []string
	if gate.ProviderPayloadSHA256 != readiness.ProviderPayloadSHA256 || payloadReport.ProviderPayloadSHA256 != readiness.ProviderPayloadSHA256 {
		failures = append(failures, "provider_payload_sha256 mismatch across readiness report, gate, and payload report")
	}
	if gate.MaterializedSHA256 != "" && readiness.MaterializedSHA256 != "" && gate.MaterializedSHA256 != readiness.MaterializedSHA256 {
		failures = append(failures, "materialized_sha256 mismatch between readiness report and gate")
	}
	if payloadReport.MaterializedSHA256 != "" && readiness.MaterializedSHA256 != "" && payloadReport.MaterializedSHA256 != readiness.MaterializedSHA256 {
		failures = append(failures, "materialized_sha256 mismatch between readiness report and payload report")
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
		ProviderPayloadSHA256:           payloadReport.ProviderPayloadSHA256,
		MaterializedSHA256:              firstNonEmpty(readiness.MaterializedSHA256, gate.MaterializedSHA256, payloadReport.MaterializedSHA256),
		RequestedProviderCallAuthorized: true,
		ProviderCallAllowedNow:          false,
		NetworkCallAllowedNow:           false,
		WorkerExecutionAllowedNow:       false,
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
	if strings.TrimSpace(opts.ConfirmProviderPayloadSHA256) == "" {
		return ProviderCallApproval{}, fmt.Errorf("--confirm-provider-payload-sha256 is required")
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
	if request.ProviderPayloadSHA256 != opts.ConfirmProviderPayloadSHA256 {
		return ProviderCallApproval{}, fmt.Errorf("confirm provider payload sha256 mismatch")
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
		ProviderPayloadSHA256:           request.ProviderPayloadSHA256,
		MaterializedSHA256:              request.MaterializedSHA256,
		AllowedUse:                      AllowedUseProviderCallPolicyOnly,
		ProviderCallAuthorizedForFuture: true,
		ProviderCallAllowedNow:          false,
		NetworkCallAllowedNow:           false,
		WorkerExecutionAllowedNow:       false,
		SentToProvider:                  false,
		ConfirmProviderPayloadSHA256:    true,
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
		NetworkCallAllowedNow:           approval.NetworkCallAllowedNow,
		WorkerExecutionAllowedNow:       approval.WorkerExecutionAllowedNow,
		SentToProvider:                  approval.SentToProvider,
		AllowedUse:                      approval.AllowedUse,
		ProviderPayloadSHA256:           approval.ProviderPayloadSHA256,
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

func validateMaterializedProviderCallReadinessReportForApproval(report MaterializedProviderCallReadinessReportResult) error {
	switch report.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("readiness report status %q must be ok or warning", report.Status)
	}
	if !report.ProviderCallReadinessReady {
		return fmt.Errorf("readiness report provider_call_readiness_ready must be true")
	}
	if report.ProviderCallAllowedNow || report.NetworkCallAllowedNow || report.WorkerExecutionAllowedNow || report.SentToProvider {
		return fmt.Errorf("readiness report must keep provider_call, network_call, worker_execution, and sent_to_provider blocked")
	}
	if strings.TrimSpace(report.ProviderPayloadSHA256) == "" {
		return fmt.Errorf("readiness report provider_payload_sha256 is required")
	}
	return nil
}

func validateMaterializedProviderCallGateForApproval(gate MaterializedProviderCallGateResult) error {
	switch gate.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("provider call gate status %q must be ok or warning", gate.Status)
	}
	if !gate.ProviderCallGateReady {
		return fmt.Errorf("provider call gate provider_call_gate_ready must be true")
	}
	if gate.ProviderCallAllowedNow || gate.NetworkCallAllowedNow || gate.WorkerExecutionAllowedNow || gate.SentToProvider {
		return fmt.Errorf("provider call gate must keep provider_call, network_call, worker_execution, and sent_to_provider blocked")
	}
	if strings.TrimSpace(gate.ProviderPayloadSHA256) == "" {
		return fmt.Errorf("provider call gate provider_payload_sha256 is required")
	}
	return nil
}

func validateMaterializedProviderPayloadReportForApproval(report MaterializedProviderPayloadReportResult) error {
	switch report.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("payload report status %q must be ok or warning", report.Status)
	}
	if !report.ProviderPayloadValidated {
		return fmt.Errorf("payload report provider_payload_validated must be true")
	}
	if report.ProviderCall || report.NetworkCall || report.WorkerExecution || report.SentToProvider {
		return fmt.Errorf("payload report must keep provider_call, network_call, worker_execution, and sent_to_provider false")
	}
	if strings.TrimSpace(report.ProviderPayloadSHA256) == "" {
		return fmt.Errorf("payload report provider_payload_sha256 is required")
	}
	return nil
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
		{"provider_payload_sha256", r.ProviderPayloadSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if !r.RequestedProviderCallAuthorized {
		return fmt.Errorf("requested_provider_call_authorized must be true")
	}
	if r.ProviderCallAllowedNow || r.NetworkCallAllowedNow || r.WorkerExecutionAllowedNow || r.SentToProvider {
		return fmt.Errorf("approval request must keep provider_call, network_call, worker_execution, and sent_to_provider blocked")
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
	if a.ProviderCallAllowedNow || a.NetworkCallAllowedNow || a.WorkerExecutionAllowedNow || a.SentToProvider {
		return fmt.Errorf("approval must keep provider_call, network_call, worker_execution, and sent_to_provider blocked")
	}
	if a.AllowedUse != AllowedUseProviderCallPolicyOnly {
		return fmt.Errorf("allowed_use %q must be %q", a.AllowedUse, AllowedUseProviderCallPolicyOnly)
	}
	if !a.ConfirmProviderPayloadSHA256 {
		return fmt.Errorf("%s must be true", confirmProviderPayloadSHA256)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"request_sha256", a.RequestSHA256},
		{"readiness_report_sha256", a.ReadinessReportSHA256},
		{"provider_call_gate_sha256", a.ProviderCallGateSHA256},
		{"payload_report_sha256", a.PayloadReportSHA256},
		{"provider_payload_sha256", a.ProviderPayloadSHA256},
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
		{"network_call_allowed_now", fmt.Sprintf("%t", result.NetworkCallAllowedNow)},
		{"worker_execution_allowed_now", fmt.Sprintf("%t", result.WorkerExecutionAllowedNow)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"allowed_use", result.AllowedUse},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
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
