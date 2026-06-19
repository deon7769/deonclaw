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

const AllowedUseProviderExecutorDispatchOnly = "provider_executor_dispatch_only"

type ProviderCallExecutorDispatchApprovalRequest struct {
	Status                      string    `json:"status"`
	CreatedAt                   time.Time `json:"created_at"`
	ExecutorConfigSHA256        string    `json:"executor_config_sha256"`
	DryRunSHA256                string    `json:"dry_run_sha256"`
	DryRunReportSHA256          string    `json:"dry_run_report_sha256"`
	PreflightSHA256             string    `json:"preflight_sha256"`
	ExecutionBundleSHA256       string    `json:"execution_bundle_sha256"`
	ApprovalSHA256              string    `json:"approval_sha256"`
	ProviderPayloadSHA256       string    `json:"provider_payload_sha256"`
	MaterializedSHA256          string    `json:"materialized_sha256,omitempty"`
	RequestedDispatchAuthorized bool      `json:"requested_dispatch_authorized"`
	DispatchAllowedNow          bool      `json:"dispatch_allowed_now"`
	ProviderCall                bool      `json:"provider_call"`
	NetworkCall                 bool      `json:"network_call"`
	TransportCalled             bool      `json:"transport_called"`
	SentToProvider              bool      `json:"sent_to_provider"`
	ContainsText                bool      `json:"contains_text"`
	Warnings                    []string  `json:"warnings,omitempty"`
}

type ProviderCallExecutorDispatchApproval struct {
	Approved                     bool      `json:"approved"`
	ApprovedAt                   time.Time `json:"approved_at"`
	RequestSHA256                string    `json:"request_sha256"`
	ExecutorConfigSHA256         string    `json:"executor_config_sha256"`
	DryRunSHA256                 string    `json:"dry_run_sha256"`
	DryRunReportSHA256           string    `json:"dry_run_report_sha256"`
	PreflightSHA256              string    `json:"preflight_sha256"`
	ExecutionBundleSHA256        string    `json:"execution_bundle_sha256"`
	ApprovalSHA256               string    `json:"approval_sha256"`
	ProviderPayloadSHA256        string    `json:"provider_payload_sha256"`
	MaterializedSHA256           string    `json:"materialized_sha256,omitempty"`
	AllowedUse                   string    `json:"allowed_use"`
	DispatchAuthorizedForFuture  bool      `json:"dispatch_authorized_for_future"`
	DispatchAllowedNow           bool      `json:"dispatch_allowed_now"`
	ProviderCall                 bool      `json:"provider_call"`
	NetworkCall                  bool      `json:"network_call"`
	TransportCalled              bool      `json:"transport_called"`
	SentToProvider               bool      `json:"sent_to_provider"`
	ConfirmExecutorConfigSHA256  bool      `json:"confirm_executor_config_sha256"`
	ConfirmExecutionBundleSHA256 bool      `json:"confirm_execution_bundle_sha256"`
	ConfirmProviderPayloadSHA256 bool      `json:"confirm_provider_payload_sha256"`
	Warnings                     []string  `json:"warnings,omitempty"`
}

type NewProviderCallExecutorDispatchApprovalRequestOptions struct {
	ExecutorConfigPath  string
	DryRunPath          string
	DryRunReportPath    string
	PreflightPath       string
	ExecutionBundlePath string
	ApprovalPath        string
	OutputPath          string
}

type ApproveProviderCallExecutorDispatchOptions struct {
	RequestPath                  string
	OutputPath                   string
	ConfirmExecutorConfigSHA256  string
	ConfirmExecutionBundleSHA256 string
	ConfirmProviderPayloadSHA256 string
	ApprovedAt                   time.Time
}

type InspectProviderCallExecutorDispatchApprovalOptions struct {
	RequestPath string
}

type InspectProviderCallExecutorDispatchApprovalResult struct {
	Status                      string   `json:"status"`
	Approved                    bool     `json:"approved"`
	DispatchAuthorizedForFuture bool     `json:"dispatch_authorized_for_future"`
	DispatchAllowedNow          bool     `json:"dispatch_allowed_now"`
	ProviderCall                bool     `json:"provider_call"`
	NetworkCall                 bool     `json:"network_call"`
	TransportCalled             bool     `json:"transport_called"`
	SentToProvider              bool     `json:"sent_to_provider"`
	ProviderPayloadSHA256       string   `json:"provider_payload_sha256"`
	MaterializedSHA256          string   `json:"materialized_sha256,omitempty"`
	Warnings                    []string `json:"warnings"`
	Failures                    []string `json:"failures,omitempty"`
}

func NewProviderCallExecutorDispatchApprovalRequest(opts NewProviderCallExecutorDispatchApprovalRequestOptions) (ProviderCallExecutorDispatchApprovalRequest, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"executor config path", opts.ExecutorConfigPath},
		{"dry-run path", opts.DryRunPath},
		{"dry-run report path", opts.DryRunReportPath},
		{"preflight path", opts.PreflightPath},
		{"execution bundle path", opts.ExecutionBundlePath},
		{"approval path", opts.ApprovalPath},
		{"output path", opts.OutputPath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallExecutorDispatchApprovalRequest{}, err
		}
	}

	preflight, preflightData, err := LoadProviderCallExecutorPreflight(opts.PreflightPath)
	if err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	if !preflight.ExecutorPreflightReady {
		return ProviderCallExecutorDispatchApprovalRequest{}, fmt.Errorf("preflight executor_preflight_ready must be true")
	}

	executorCfgData, err := readArtifactBytesNoTextExcerpt("executor config", opts.ExecutorConfigPath)
	if err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	dryRunData, err := readArtifactBytesNoTextExcerpt("dry-run", opts.DryRunPath)
	if err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	dryRunReportData, err := readArtifactBytesNoTextExcerpt("dry-run report", opts.DryRunReportPath)
	if err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	bundleData, err := readArtifactBytesNoTextExcerpt("execution bundle", opts.ExecutionBundlePath)
	if err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	approvalData, err := readArtifactBytesNoTextExcerpt("provider call approval", opts.ApprovalPath)
	if err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}

	executorSHA := sha256Hex(executorCfgData)
	dryRunSHA := sha256Hex(dryRunData)
	dryRunReportSHA := sha256Hex(dryRunReportData)
	preflightSHA := sha256Hex(preflightData)
	bundleSHA := sha256Hex(bundleData)
	approvalSHA := sha256Hex(approvalData)

	var failures []string
	if preflight.ExecutorConfigSHA256 != executorSHA {
		failures = append(failures, "executor_config_sha256 mismatch with preflight")
	}
	if preflight.DryRunSHA256 != dryRunSHA {
		failures = append(failures, "dry_run_sha256 mismatch with preflight")
	}
	if preflight.DryRunReportSHA256 != dryRunReportSHA {
		failures = append(failures, "dry_run_report_sha256 mismatch with preflight")
	}
	if preflight.ExecutionBundleSHA256 != bundleSHA {
		failures = append(failures, "execution_bundle_sha256 mismatch with preflight")
	}
	if preflight.ApprovalSHA256 != approvalSHA {
		failures = append(failures, "approval_sha256 mismatch with preflight")
	}
	if len(failures) > 0 {
		return ProviderCallExecutorDispatchApprovalRequest{}, fmt.Errorf("%s", strings.Join(failures, "; "))
	}

	request := ProviderCallExecutorDispatchApprovalRequest{
		Status:                      RequestStatusPending,
		CreatedAt:                   time.Now().UTC(),
		ExecutorConfigSHA256:        executorSHA,
		DryRunSHA256:                dryRunSHA,
		DryRunReportSHA256:          dryRunReportSHA,
		PreflightSHA256:             preflightSHA,
		ExecutionBundleSHA256:       bundleSHA,
		ApprovalSHA256:              approvalSHA,
		ProviderPayloadSHA256:       preflight.ProviderPayloadSHA256,
		MaterializedSHA256:          preflight.MaterializedSHA256,
		RequestedDispatchAuthorized: true,
		DispatchAllowedNow:          false,
		ProviderCall:                false,
		NetworkCall:                 false,
		TransportCalled:             false,
		SentToProvider:              false,
		ContainsText:                false,
		Warnings:                    append([]string(nil), preflight.Warnings...),
	}
	if err := request.Validate(); err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	if err := writeProviderCallExecutorDispatchApprovalRequestJSON(opts.OutputPath, request); err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, err
	}
	return request, nil
}

func ApproveProviderCallExecutorDispatch(opts ApproveProviderCallExecutorDispatchOptions) (ProviderCallExecutorDispatchApproval, error) {
	if strings.TrimSpace(opts.ConfirmExecutorConfigSHA256) == "" {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("--confirm-executor-config-sha256 is required")
	}
	if strings.TrimSpace(opts.ConfirmExecutionBundleSHA256) == "" {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("--confirm-execution-bundle-sha256 is required")
	}
	if strings.TrimSpace(opts.ConfirmProviderPayloadSHA256) == "" {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("--confirm-provider-payload-sha256 is required")
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return ProviderCallExecutorDispatchApproval{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ProviderCallExecutorDispatchApproval{}, err
	}

	requestData, err := os.ReadFile(opts.RequestPath)
	if err != nil {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("read dispatch approval request %q: %w", opts.RequestPath, err)
	}
	request, err := ParseProviderCallExecutorDispatchApprovalRequestJSON(requestData)
	if err != nil {
		return ProviderCallExecutorDispatchApproval{}, err
	}
	if err := request.Validate(); err != nil {
		return ProviderCallExecutorDispatchApproval{}, err
	}
	if request.Status != RequestStatusPending {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("dispatch approval request status %q must be pending", request.Status)
	}
	if request.ExecutorConfigSHA256 != opts.ConfirmExecutorConfigSHA256 {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("confirm executor config sha256 mismatch")
	}
	if request.ExecutionBundleSHA256 != opts.ConfirmExecutionBundleSHA256 {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("confirm execution bundle sha256 mismatch")
	}
	if request.ProviderPayloadSHA256 != opts.ConfirmProviderPayloadSHA256 {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("confirm provider payload sha256 mismatch")
	}

	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}

	approval := ProviderCallExecutorDispatchApproval{
		Approved:                     true,
		ApprovedAt:                   approvedAt.UTC(),
		RequestSHA256:                sha256Hex(requestData),
		ExecutorConfigSHA256:         request.ExecutorConfigSHA256,
		DryRunSHA256:                 request.DryRunSHA256,
		DryRunReportSHA256:           request.DryRunReportSHA256,
		PreflightSHA256:              request.PreflightSHA256,
		ExecutionBundleSHA256:        request.ExecutionBundleSHA256,
		ApprovalSHA256:               request.ApprovalSHA256,
		ProviderPayloadSHA256:        request.ProviderPayloadSHA256,
		MaterializedSHA256:           request.MaterializedSHA256,
		AllowedUse:                   AllowedUseProviderExecutorDispatchOnly,
		DispatchAuthorizedForFuture:  true,
		DispatchAllowedNow:           false,
		ProviderCall:                 false,
		NetworkCall:                  false,
		TransportCalled:              false,
		SentToProvider:               false,
		ConfirmExecutorConfigSHA256:  true,
		ConfirmExecutionBundleSHA256: true,
		ConfirmProviderPayloadSHA256: true,
		Warnings:                     append([]string(nil), request.Warnings...),
	}
	if err := approval.Validate(); err != nil {
		return ProviderCallExecutorDispatchApproval{}, err
	}
	if err := writeProviderCallExecutorDispatchApprovalJSON(opts.OutputPath, approval); err != nil {
		return ProviderCallExecutorDispatchApproval{}, err
	}
	return approval, nil
}

func InspectProviderCallExecutorDispatchApproval(path string, opts InspectProviderCallExecutorDispatchApprovalOptions) (InspectProviderCallExecutorDispatchApprovalResult, error) {
	if err := validateRelativeSafePath("dispatch approval path", path); err != nil {
		return InspectProviderCallExecutorDispatchApprovalResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InspectProviderCallExecutorDispatchApprovalResult{}, fmt.Errorf("read dispatch approval %q: %w", path, err)
	}
	return InspectProviderCallExecutorDispatchApprovalBytes(data, opts)
}

func InspectProviderCallExecutorDispatchApprovalBytes(data []byte, opts InspectProviderCallExecutorDispatchApprovalOptions) (InspectProviderCallExecutorDispatchApprovalResult, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return InspectProviderCallExecutorDispatchApprovalResult{Status: lancedbpolicy.StatusFailed, Failures: []string{"dispatch approval contains forbidden field text_excerpt"}}, nil
	}
	approval, err := ParseProviderCallExecutorDispatchApprovalJSON(data)
	if err != nil {
		return InspectProviderCallExecutorDispatchApprovalResult{}, err
	}
	result := InspectProviderCallExecutorDispatchApprovalResult{
		Status:                      lancedbpolicy.StatusOK,
		Approved:                    approval.Approved,
		DispatchAuthorizedForFuture: approval.DispatchAuthorizedForFuture,
		DispatchAllowedNow:          approval.DispatchAllowedNow,
		ProviderCall:                approval.ProviderCall,
		NetworkCall:                 approval.NetworkCall,
		TransportCalled:             approval.TransportCalled,
		SentToProvider:              approval.SentToProvider,
		ProviderPayloadSHA256:       approval.ProviderPayloadSHA256,
		MaterializedSHA256:          approval.MaterializedSHA256,
		Warnings:                    append([]string(nil), approval.Warnings...),
	}
	var failures []string
	if err := approval.Validate(); err != nil {
		failures = append(failures, err.Error())
	}
	if opts.RequestPath != "" {
		if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
			return InspectProviderCallExecutorDispatchApprovalResult{}, err
		}
		requestData, err := os.ReadFile(opts.RequestPath)
		if err != nil {
			return InspectProviderCallExecutorDispatchApprovalResult{}, fmt.Errorf("read dispatch approval request %q: %w", opts.RequestPath, err)
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

func LoadProviderCallExecutorDispatchApproval(path string) (ProviderCallExecutorDispatchApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("read dispatch approval %q: %w", path, err)
	}
	return ParseProviderCallExecutorDispatchApprovalJSON(data)
}

func ParseProviderCallExecutorDispatchApprovalRequestJSON(data []byte) (ProviderCallExecutorDispatchApprovalRequest, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallExecutorDispatchApprovalRequest{}, fmt.Errorf("dispatch approval request must not contain materialized preview text")
	}
	var request ProviderCallExecutorDispatchApprovalRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return ProviderCallExecutorDispatchApprovalRequest{}, fmt.Errorf("parse dispatch approval request json: %w", err)
	}
	return request, nil
}

func ParseProviderCallExecutorDispatchApprovalJSON(data []byte) (ProviderCallExecutorDispatchApproval, error) {
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("dispatch approval must not contain materialized preview text")
	}
	var approval ProviderCallExecutorDispatchApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return ProviderCallExecutorDispatchApproval{}, fmt.Errorf("parse dispatch approval json: %w", err)
	}
	return approval, nil
}

func (r ProviderCallExecutorDispatchApprovalRequest) Validate() error {
	if r.Status != RequestStatusPending {
		return fmt.Errorf("dispatch approval request status %q must be pending", r.Status)
	}
	if !r.RequestedDispatchAuthorized {
		return fmt.Errorf("requested_dispatch_authorized must be true")
	}
	if r.DispatchAllowedNow || r.ProviderCall || r.NetworkCall || r.TransportCalled || r.SentToProvider || r.ContainsText {
		return fmt.Errorf("dispatch approval request must keep dispatch and execution flags blocked")
	}
	if strings.TrimSpace(r.ExecutorConfigSHA256) == "" || strings.TrimSpace(r.ExecutionBundleSHA256) == "" || strings.TrimSpace(r.ProviderPayloadSHA256) == "" {
		return fmt.Errorf("dispatch approval request hash fields are required")
	}
	return nil
}

func (a ProviderCallExecutorDispatchApproval) Validate() error {
	if !a.Approved {
		return fmt.Errorf("dispatch approval approved must be true")
	}
	if !a.DispatchAuthorizedForFuture {
		return fmt.Errorf("dispatch_authorized_for_future must be true")
	}
	if a.DispatchAllowedNow || a.ProviderCall || a.NetworkCall || a.TransportCalled || a.SentToProvider {
		return fmt.Errorf("dispatch approval must keep dispatch and execution flags blocked")
	}
	if a.AllowedUse != AllowedUseProviderExecutorDispatchOnly {
		return fmt.Errorf("allowed_use %q must be %q", a.AllowedUse, AllowedUseProviderExecutorDispatchOnly)
	}
	if !a.ConfirmExecutorConfigSHA256 || !a.ConfirmExecutionBundleSHA256 || !a.ConfirmProviderPayloadSHA256 {
		return fmt.Errorf("dispatch approval confirm flags must be true")
	}
	return nil
}

func writeProviderCallExecutorDispatchApprovalRequestJSON(path string, request ProviderCallExecutorDispatchApprovalRequest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dispatch approval request output dir: %w", err)
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dispatch approval request json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("dispatch approval request must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func writeProviderCallExecutorDispatchApprovalJSON(path string, approval ProviderCallExecutorDispatchApproval) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create dispatch approval output dir: %w", err)
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dispatch approval json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("dispatch approval must not contain materialized preview text")
	}
	return os.WriteFile(path, data, 0o644)
}

func WriteProviderCallExecutorDispatchApprovalInspectJSON(result InspectProviderCallExecutorDispatchApprovalResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dispatch approval inspect json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("dispatch approval inspect json must not contain materialized preview text")
	}
	_, err = out.Write(data)
	return err
}

func WriteProviderCallExecutorDispatchApprovalInspectText(result InspectProviderCallExecutorDispatchApprovalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_executor_dispatch_approval_inspect:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"approved", fmt.Sprintf("%t", result.Approved)},
		{"dispatch_authorized_for_future", fmt.Sprintf("%t", result.DispatchAuthorizedForFuture)},
		{"dispatch_allowed_now", fmt.Sprintf("%t", result.DispatchAllowedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"transport_called", fmt.Sprintf("%t", result.TransportCalled)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
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
	return nil
}
