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
	AllowedUseRunnerInjectionPolicyOnly = "runner_injection_policy_only"
	confirmAllowRunnerInjection         = "confirm_allow_runner_injection"
)

type InjectionApprovalRequest struct {
	Status                          string    `json:"status"`
	CreatedAt                       time.Time `json:"created_at"`
	GovernanceReportSHA256          string    `json:"governance_report_sha256"`
	PolicySHA256                    string    `json:"policy_sha256"`
	MaterializedSHA256              string    `json:"materialized_sha256"`
	BundleSHA256                    string    `json:"bundle_sha256"`
	MaxTotalChars                   int       `json:"max_total_chars"`
	MaxCharsPerChunk                int       `json:"max_chars_per_chunk"`
	MaxChunks                       int       `json:"max_chunks"`
	RequestedRunnerInjectionAllowed bool      `json:"requested_runner_injection_allowed"`
	ContainsText                    bool      `json:"contains_text"`
	Warnings                        []string  `json:"warnings,omitempty"`
}

type RunnerInjectionApproval struct {
	Approved                    bool      `json:"approved"`
	ApprovedAt                  time.Time `json:"approved_at"`
	RunnerInjectionAllowed      bool      `json:"runner_injection_allowed"`
	AllowedUse                  string    `json:"allowed_use"`
	RequestSHA256               string    `json:"request_sha256"`
	GovernanceReportSHA256      string    `json:"governance_report_sha256"`
	PolicySHA256                string    `json:"policy_sha256"`
	MaterializedSHA256          string    `json:"materialized_sha256"`
	BundleSHA256                string    `json:"bundle_sha256"`
	MaxTotalChars               int       `json:"max_total_chars"`
	MaxCharsPerChunk            int       `json:"max_chars_per_chunk"`
	MaxChunks                   int       `json:"max_chunks"`
	ConfirmAllowRunnerInjection bool      `json:"confirm_allow_runner_injection"`
	Warnings                    []string  `json:"warnings,omitempty"`
}

type NewInjectionApprovalRequestOptions struct {
	GovernanceReportPath string
	PolicyPath           string
	OutputPath           string
}

type ApproveRunnerInjectionOptions struct {
	RequestPath                 string
	OutputPath                  string
	ConfirmAllowRunnerInjection bool
	ApprovedAt                  time.Time
}

type InspectInjectionApprovalOptions struct {
	RequestPath string
}

type InspectInjectionApprovalResult struct {
	Status                 string   `json:"status"`
	Approved               bool     `json:"approved"`
	RunnerInjectionAllowed bool     `json:"runner_injection_allowed"`
	AllowedUse             string   `json:"allowed_use"`
	MaterializedSHA256     string   `json:"materialized_sha256"`
	MaxTotalChars          int      `json:"max_total_chars"`
	MaxChunks              int      `json:"max_chunks"`
	Warnings               []string `json:"warnings"`
	Failures               []string `json:"failures,omitempty"`
}

func LoadGovernanceReport(path string) (GovernanceReportResult, []byte, error) {
	if err := validateRelativeSafePath("governance report path", path); err != nil {
		return GovernanceReportResult{}, nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return GovernanceReportResult{}, nil, fmt.Errorf("read governance report %q: %w", path, err)
	}
	return ParseGovernanceReportJSON(data)
}

func ParseGovernanceReportJSON(data []byte) (GovernanceReportResult, []byte, error) {
	if strings.Contains(string(data), `"text_excerpt"`) {
		return GovernanceReportResult{}, nil, fmt.Errorf("governance report must not contain text_excerpt")
	}
	var report GovernanceReportResult
	if err := json.Unmarshal(data, &report); err != nil {
		return GovernanceReportResult{}, nil, fmt.Errorf("parse governance report json: %w", err)
	}
	return report, data, nil
}

func NewInjectionApprovalRequest(opts NewInjectionApprovalRequestOptions) (InjectionApprovalRequest, error) {
	if err := validateRelativeSafePath("governance report path", opts.GovernanceReportPath); err != nil {
		return InjectionApprovalRequest{}, err
	}
	if err := validateRelativeSafePath("policy path", opts.PolicyPath); err != nil {
		return InjectionApprovalRequest{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return InjectionApprovalRequest{}, err
	}

	report, governanceData, err := LoadGovernanceReport(opts.GovernanceReportPath)
	if err != nil {
		return InjectionApprovalRequest{}, err
	}
	if err := validateGovernanceReportForInjectionApproval(report); err != nil {
		return InjectionApprovalRequest{}, err
	}

	policyData, err := os.ReadFile(opts.PolicyPath)
	if err != nil {
		return InjectionApprovalRequest{}, fmt.Errorf("read injection policy %q: %w", opts.PolicyPath, err)
	}
	cfg, err := ParseInjectionPolicy(policyData)
	if err != nil {
		return InjectionApprovalRequest{}, err
	}
	if err := ValidateInjectionPolicy(cfg); err != nil {
		return InjectionApprovalRequest{}, fmt.Errorf("injection policy invalid: %w", err)
	}

	plan, err := InjectionPolicyPlan(cfg)
	if err != nil {
		return InjectionApprovalRequest{}, err
	}
	if plan.Status == lancedbpolicy.StatusFailed {
		return InjectionApprovalRequest{}, fmt.Errorf("injection policy plan status %q", plan.Status)
	}

	limits := cfg.RetrievalInjectionPolicy.Limits
	request := InjectionApprovalRequest{
		Status:                          RequestStatusPending,
		CreatedAt:                       time.Now().UTC(),
		GovernanceReportSHA256:          sha256Hex(governanceData),
		PolicySHA256:                    sha256Hex(policyData),
		MaterializedSHA256:              report.Hashes.MaterializedSHA256,
		BundleSHA256:                    report.Hashes.BundleSHA256,
		MaxTotalChars:                   limits.MaxTotalChars,
		MaxCharsPerChunk:                limits.MaxCharsPerChunk,
		MaxChunks:                       limits.MaxChunks,
		RequestedRunnerInjectionAllowed: true,
		ContainsText:                    false,
		Warnings: mergeWarnings(
			append([]string(nil), report.Warnings...),
			append([]string(nil), plan.Warnings...),
		),
	}
	if report.Status == lancedbpolicy.StatusWarning {
		request.Warnings = mergeWarnings(request.Warnings, []string{"governance report status is warning; injection approval request recorded with documented acceptance"})
	}

	if err := request.Validate(); err != nil {
		return InjectionApprovalRequest{}, err
	}
	if err := writeInjectionApprovalRequestJSON(opts.OutputPath, request); err != nil {
		return InjectionApprovalRequest{}, err
	}
	return request, nil
}

func ApproveRunnerInjection(opts ApproveRunnerInjectionOptions) (RunnerInjectionApproval, error) {
	if !opts.ConfirmAllowRunnerInjection {
		return RunnerInjectionApproval{}, fmt.Errorf("--confirm-allow-runner-injection is required")
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return RunnerInjectionApproval{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return RunnerInjectionApproval{}, err
	}

	requestData, err := os.ReadFile(opts.RequestPath)
	if err != nil {
		return RunnerInjectionApproval{}, fmt.Errorf("read injection approval request %q: %w", opts.RequestPath, err)
	}
	request, err := ParseInjectionApprovalRequestJSON(requestData)
	if err != nil {
		return RunnerInjectionApproval{}, err
	}
	if err := request.Validate(); err != nil {
		return RunnerInjectionApproval{}, fmt.Errorf("injection approval request invalid: %w", err)
	}
	if request.Status != RequestStatusPending {
		return RunnerInjectionApproval{}, fmt.Errorf("injection approval request status %q must be pending", request.Status)
	}

	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}

	approval := RunnerInjectionApproval{
		Approved:                    true,
		ApprovedAt:                  approvedAt.UTC(),
		RunnerInjectionAllowed:      true,
		AllowedUse:                  AllowedUseRunnerInjectionPolicyOnly,
		RequestSHA256:               sha256Hex(requestData),
		GovernanceReportSHA256:      request.GovernanceReportSHA256,
		PolicySHA256:                request.PolicySHA256,
		MaterializedSHA256:          request.MaterializedSHA256,
		BundleSHA256:                request.BundleSHA256,
		MaxTotalChars:               request.MaxTotalChars,
		MaxCharsPerChunk:            request.MaxCharsPerChunk,
		MaxChunks:                   request.MaxChunks,
		ConfirmAllowRunnerInjection: true,
		Warnings:                    append([]string(nil), request.Warnings...),
	}
	if err := approval.Validate(); err != nil {
		return RunnerInjectionApproval{}, err
	}
	if err := writeRunnerInjectionApprovalJSON(opts.OutputPath, approval); err != nil {
		return RunnerInjectionApproval{}, err
	}
	return approval, nil
}

func LoadInjectionApprovalRequest(path string) (InjectionApprovalRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InjectionApprovalRequest{}, fmt.Errorf("read injection approval request %q: %w", path, err)
	}
	return ParseInjectionApprovalRequestJSON(data)
}

func ParseInjectionApprovalRequestJSON(data []byte) (InjectionApprovalRequest, error) {
	var request InjectionApprovalRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return InjectionApprovalRequest{}, fmt.Errorf("parse injection approval request json: %w", err)
	}
	return request, nil
}

func LoadRunnerInjectionApproval(path string) (RunnerInjectionApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunnerInjectionApproval{}, fmt.Errorf("read runner injection approval %q: %w", path, err)
	}
	return ParseRunnerInjectionApprovalJSON(data)
}

func ParseRunnerInjectionApprovalJSON(data []byte) (RunnerInjectionApproval, error) {
	var approval RunnerInjectionApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return RunnerInjectionApproval{}, fmt.Errorf("parse runner injection approval json: %w", err)
	}
	return approval, nil
}

func InspectInjectionApproval(path string, opts InspectInjectionApprovalOptions) (InspectInjectionApprovalResult, error) {
	if err := validateRelativeSafePath("approval path", path); err != nil {
		return InspectInjectionApprovalResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InspectInjectionApprovalResult{}, fmt.Errorf("read runner injection approval %q: %w", path, err)
	}
	return InspectInjectionApprovalBytes(data, opts)
}

func InspectInjectionApprovalBytes(data []byte, opts InspectInjectionApprovalOptions) (InspectInjectionApprovalResult, error) {
	if strings.Contains(string(data), `"text_excerpt"`) {
		return InspectInjectionApprovalResult{Status: lancedbpolicy.StatusFailed, Failures: []string{"approval contains forbidden field text_excerpt"}}, nil
	}
	approval, err := ParseRunnerInjectionApprovalJSON(data)
	if err != nil {
		return InspectInjectionApprovalResult{}, err
	}

	result := InspectInjectionApprovalResult{
		Status:                 lancedbpolicy.StatusOK,
		Approved:               approval.Approved,
		RunnerInjectionAllowed: approval.RunnerInjectionAllowed,
		AllowedUse:             approval.AllowedUse,
		MaterializedSHA256:     approval.MaterializedSHA256,
		MaxTotalChars:          approval.MaxTotalChars,
		MaxChunks:              approval.MaxChunks,
		Warnings:               append([]string(nil), approval.Warnings...),
	}

	var failures []string
	if err := approval.Validate(); err != nil {
		failures = append(failures, err.Error())
	}
	if opts.RequestPath != "" {
		if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
			return InspectInjectionApprovalResult{}, err
		}
		requestData, err := os.ReadFile(opts.RequestPath)
		if err != nil {
			return InspectInjectionApprovalResult{}, fmt.Errorf("read injection approval request %q: %w", opts.RequestPath, err)
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

func validateGovernanceReportForInjectionApproval(report GovernanceReportResult) error {
	switch report.Status {
	case lancedbpolicy.StatusOK, lancedbpolicy.StatusWarning:
	default:
		return fmt.Errorf("governance report status %q must be ok or warning", report.Status)
	}
	if strings.TrimSpace(report.Hashes.MaterializedSHA256) == "" {
		return fmt.Errorf("governance report materialized_sha256 is required")
	}
	if strings.TrimSpace(report.Hashes.BundleSHA256) == "" {
		return fmt.Errorf("governance report bundle_sha256 is required")
	}
	return nil
}

func (r InjectionApprovalRequest) Validate() error {
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
		{"governance_report_sha256", r.GovernanceReportSHA256},
		{"policy_sha256", r.PolicySHA256},
		{"materialized_sha256", r.MaterializedSHA256},
		{"bundle_sha256", r.BundleSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if r.MaxTotalChars <= 0 || r.MaxCharsPerChunk <= 0 || r.MaxChunks <= 0 {
		return fmt.Errorf("char and chunk caps must be > 0")
	}
	if !r.RequestedRunnerInjectionAllowed {
		return fmt.Errorf("requested_runner_injection_allowed must be true")
	}
	if r.ContainsText {
		return fmt.Errorf("contains_text must be false")
	}
	return nil
}

func (a RunnerInjectionApproval) Validate() error {
	if !a.Approved {
		return fmt.Errorf("approved must be true")
	}
	if a.ApprovedAt.IsZero() {
		return fmt.Errorf("approved_at is required")
	}
	if !a.RunnerInjectionAllowed {
		return fmt.Errorf("runner_injection_allowed must be true")
	}
	if a.AllowedUse != AllowedUseRunnerInjectionPolicyOnly {
		return fmt.Errorf("allowed_use %q must be %q", a.AllowedUse, AllowedUseRunnerInjectionPolicyOnly)
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"request_sha256", a.RequestSHA256},
		{"governance_report_sha256", a.GovernanceReportSHA256},
		{"policy_sha256", a.PolicySHA256},
		{"materialized_sha256", a.MaterializedSHA256},
		{"bundle_sha256", a.BundleSHA256},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.name)
		}
	}
	if a.MaxTotalChars <= 0 || a.MaxCharsPerChunk <= 0 || a.MaxChunks <= 0 {
		return fmt.Errorf("char and chunk caps must be > 0")
	}
	if !a.ConfirmAllowRunnerInjection {
		return fmt.Errorf("%s must be true", confirmAllowRunnerInjection)
	}
	return nil
}

func writeInjectionApprovalRequestJSON(path string, request InjectionApprovalRequest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create injection approval request output dir: %w", err)
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal injection approval request json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("injection approval request must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write injection approval request %q: %w", path, err)
	}
	return nil
}

func writeRunnerInjectionApprovalJSON(path string, approval RunnerInjectionApproval) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create runner injection approval output dir: %w", err)
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal runner injection approval json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("runner injection approval must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write runner injection approval %q: %w", path, err)
	}
	return nil
}

func WriteInspectInjectionApprovalText(result InspectInjectionApprovalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_injection_approval_inspect:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "approved: %t\n", result.Approved); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "runner_injection_allowed: %t\n", result.RunnerInjectionAllowed); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "allowed_use: %s\n", result.AllowedUse); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "materialized_sha256: %s\n", result.MaterializedSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_total_chars: %d\n", result.MaxTotalChars); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "max_chunks: %d\n", result.MaxChunks); err != nil {
		return err
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
	if len(result.Warnings) > 0 {
		if _, err := fmt.Fprintln(out, "\nwarnings:"); err != nil {
			return err
		}
		for _, warning := range result.Warnings {
			if _, err := fmt.Fprintf(out, "- %s\n", warning); err != nil {
				return err
			}
		}
	}
	return nil
}

func WriteInspectInjectionApprovalJSON(result InspectInjectionApprovalResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
