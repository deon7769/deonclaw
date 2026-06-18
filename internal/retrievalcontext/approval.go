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
	RequestStatusPending          = "pending"
	AllowedUseManualReviewOnly    = "manual_review_only"
	confirmApproveMaterializedCtx = "confirm_approve_materialized_context"
)

type ApprovalRequest struct {
	Status                   string    `json:"status"`
	CreatedAt                time.Time `json:"created_at"`
	BundleSHA256             string    `json:"bundle_sha256"`
	RetrievalContextSHA256   string    `json:"retrieval_context_sha256"`
	MaterializedSHA256       string    `json:"materialized_sha256"`
	ContainsText             bool      `json:"contains_text"`
	MaterializedTextArtifact string    `json:"materialized_text_artifact"`
	IncludedChunkCount       int       `json:"included_chunk_count"`
	OmittedChunkCount        int       `json:"omitted_chunk_count"`
	TotalCharsIncluded       int       `json:"total_chars_included"`
	Warnings                 []string  `json:"warnings,omitempty"`
}

type MaterializedContextApproval struct {
	Approved                          bool      `json:"approved"`
	ApprovedAt                        time.Time `json:"approved_at"`
	RequestSHA256                     string    `json:"request_sha256"`
	BundleSHA256                      string    `json:"bundle_sha256"`
	MaterializedSHA256                string    `json:"materialized_sha256"`
	AllowedUse                        string    `json:"allowed_use"`
	RunnerInjectionAllowed            bool      `json:"runner_injection_allowed"`
	ConfirmApproveMaterializedContext bool      `json:"confirm_approve_materialized_context"`
	Warnings                          []string  `json:"warnings,omitempty"`
}

type NewApprovalRequestOptions struct {
	BundlePath string
	OutputPath string
}

type ApproveMaterializedContextOptions struct {
	RequestPath                       string
	OutputPath                        string
	ConfirmApproveMaterializedContext bool
	ApprovedAt                        time.Time
}

type InspectApprovalResult struct {
	Status                 string   `json:"status"`
	Approved               bool     `json:"approved"`
	BundleSHA256           string   `json:"bundle_sha256"`
	MaterializedSHA256     string   `json:"materialized_sha256"`
	RunnerInjectionAllowed bool     `json:"runner_injection_allowed"`
	AllowedUse             string   `json:"allowed_use"`
	Warnings               []string `json:"warnings"`
	Failures               []string `json:"failures,omitempty"`
}

type InspectApprovalOptions struct {
	RequestPath string
}

func LoadBundle(path string) (BundleResult, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BundleResult{}, nil, fmt.Errorf("read bundle %q: %w", path, err)
	}
	var bundle BundleResult
	if err := json.Unmarshal(data, &bundle); err != nil {
		return BundleResult{}, nil, fmt.Errorf("parse bundle %q: %w", path, err)
	}
	return bundle, data, nil
}

func NewApprovalRequest(opts NewApprovalRequestOptions) (ApprovalRequest, error) {
	if err := validateRelativeSafePath("bundle path", opts.BundlePath); err != nil {
		return ApprovalRequest{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return ApprovalRequest{}, err
	}

	bundle, bundleData, err := LoadBundle(opts.BundlePath)
	if err != nil {
		return ApprovalRequest{}, err
	}
	if bundle.Status == lancedbpolicy.StatusFailed {
		return ApprovalRequest{}, fmt.Errorf("bundle status %q", bundle.Status)
	}
	if bundle.ContainsText {
		return ApprovalRequest{}, fmt.Errorf("bundle contains_text must be false")
	}
	if strings.Contains(string(bundleData), `"text_excerpt"`) {
		return ApprovalRequest{}, fmt.Errorf("bundle must not contain text_excerpt")
	}

	request := ApprovalRequest{
		Status:                   RequestStatusPending,
		CreatedAt:                time.Now().UTC(),
		BundleSHA256:             sha256Hex(bundleData),
		RetrievalContextSHA256:   bundle.RetrievalContextSHA256,
		MaterializedSHA256:       bundle.MaterializedSHA256,
		ContainsText:             false,
		MaterializedTextArtifact: bundle.MaterializedTextArtifact,
		IncludedChunkCount:       bundle.IncludedChunkCount,
		OmittedChunkCount:        bundle.OmittedChunkCount,
		TotalCharsIncluded:       bundle.TotalCharsIncluded,
		Warnings:                 append([]string(nil), bundle.Warnings...),
	}
	if err := request.Validate(); err != nil {
		return ApprovalRequest{}, err
	}
	if err := writeApprovalRequestJSON(opts.OutputPath, request); err != nil {
		return ApprovalRequest{}, err
	}
	return request, nil
}

func ApproveMaterializedContext(opts ApproveMaterializedContextOptions) (MaterializedContextApproval, error) {
	if !opts.ConfirmApproveMaterializedContext {
		return MaterializedContextApproval{}, fmt.Errorf("--confirm-approve-materialized-context is required")
	}
	if err := validateRelativeSafePath("request path", opts.RequestPath); err != nil {
		return MaterializedContextApproval{}, err
	}
	if err := validateRelativeSafePath("output path", opts.OutputPath); err != nil {
		return MaterializedContextApproval{}, err
	}

	requestData, err := os.ReadFile(opts.RequestPath)
	if err != nil {
		return MaterializedContextApproval{}, fmt.Errorf("read approval request %q: %w", opts.RequestPath, err)
	}
	request, err := ParseApprovalRequestJSON(requestData)
	if err != nil {
		return MaterializedContextApproval{}, err
	}
	if err := request.Validate(); err != nil {
		return MaterializedContextApproval{}, fmt.Errorf("approval request invalid: %w", err)
	}
	if request.Status != RequestStatusPending {
		return MaterializedContextApproval{}, fmt.Errorf("approval request status %q must be pending", request.Status)
	}
	if request.ContainsText {
		return MaterializedContextApproval{}, fmt.Errorf("approval request contains_text must be false")
	}

	approvedAt := opts.ApprovedAt
	if approvedAt.IsZero() {
		approvedAt = time.Now().UTC()
	}

	approval := MaterializedContextApproval{
		Approved:                          true,
		ApprovedAt:                        approvedAt.UTC(),
		RequestSHA256:                     sha256Hex(requestData),
		BundleSHA256:                      request.BundleSHA256,
		MaterializedSHA256:                request.MaterializedSHA256,
		AllowedUse:                        AllowedUseManualReviewOnly,
		RunnerInjectionAllowed:            false,
		ConfirmApproveMaterializedContext: true,
		Warnings:                          append([]string(nil), request.Warnings...),
	}
	if err := approval.Validate(); err != nil {
		return MaterializedContextApproval{}, err
	}
	if err := writeMaterializedContextApprovalJSON(opts.OutputPath, approval); err != nil {
		return MaterializedContextApproval{}, err
	}
	return approval, nil
}

func LoadApprovalRequest(path string) (ApprovalRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("read approval request %q: %w", path, err)
	}
	return ParseApprovalRequestJSON(data)
}

func ParseApprovalRequestJSON(data []byte) (ApprovalRequest, error) {
	var request ApprovalRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return ApprovalRequest{}, fmt.Errorf("parse approval request json: %w", err)
	}
	return request, nil
}

func LoadMaterializedContextApproval(path string) (MaterializedContextApproval, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return MaterializedContextApproval{}, fmt.Errorf("read materialized context approval %q: %w", path, err)
	}
	return ParseMaterializedContextApprovalJSON(data)
}

func ParseMaterializedContextApprovalJSON(data []byte) (MaterializedContextApproval, error) {
	var approval MaterializedContextApproval
	if err := json.Unmarshal(data, &approval); err != nil {
		return MaterializedContextApproval{}, fmt.Errorf("parse materialized context approval json: %w", err)
	}
	return approval, nil
}

func InspectApproval(path string, opts InspectApprovalOptions) (InspectApprovalResult, error) {
	if err := validateRelativeSafePath("approval path", path); err != nil {
		return InspectApprovalResult{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return InspectApprovalResult{}, fmt.Errorf("read materialized context approval %q: %w", path, err)
	}
	return InspectApprovalBytes(data, opts)
}

func InspectApprovalBytes(data []byte, opts InspectApprovalOptions) (InspectApprovalResult, error) {
	if strings.Contains(string(data), `"text_excerpt"`) {
		return InspectApprovalResult{Status: lancedbpolicy.StatusFailed, Failures: []string{"approval contains forbidden field text_excerpt"}}, nil
	}
	approval, err := ParseMaterializedContextApprovalJSON(data)
	if err != nil {
		return InspectApprovalResult{}, err
	}

	result := InspectApprovalResult{
		Status:                 lancedbpolicy.StatusOK,
		Approved:               approval.Approved,
		BundleSHA256:           approval.BundleSHA256,
		MaterializedSHA256:     approval.MaterializedSHA256,
		RunnerInjectionAllowed: approval.RunnerInjectionAllowed,
		AllowedUse:             approval.AllowedUse,
		Warnings:               append([]string(nil), approval.Warnings...),
	}

	var failures []string
	if err := approval.Validate(); err != nil {
		failures = append(failures, err.Error())
	}
	if opts.RequestPath != "" {
		requestData, err := os.ReadFile(opts.RequestPath)
		if err != nil {
			return InspectApprovalResult{}, fmt.Errorf("read approval request %q: %w", opts.RequestPath, err)
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

func (r ApprovalRequest) Validate() error {
	if r.Status != RequestStatusPending {
		return fmt.Errorf("status %q must be pending", r.Status)
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	if strings.TrimSpace(r.BundleSHA256) == "" {
		return fmt.Errorf("bundle_sha256 is required")
	}
	if strings.TrimSpace(r.RetrievalContextSHA256) == "" {
		return fmt.Errorf("retrieval_context_sha256 is required")
	}
	if strings.TrimSpace(r.MaterializedSHA256) == "" {
		return fmt.Errorf("materialized_sha256 is required")
	}
	if r.ContainsText {
		return fmt.Errorf("contains_text must be false")
	}
	if strings.TrimSpace(r.MaterializedTextArtifact) == "" {
		return fmt.Errorf("materialized_text_artifact is required")
	}
	if r.IncludedChunkCount < 0 || r.OmittedChunkCount < 0 || r.TotalCharsIncluded < 0 {
		return fmt.Errorf("chunk and char counts must be >= 0")
	}
	return nil
}

func (a MaterializedContextApproval) Validate() error {
	if !a.Approved {
		return fmt.Errorf("approved must be true")
	}
	if a.ApprovedAt.IsZero() {
		return fmt.Errorf("approved_at is required")
	}
	if strings.TrimSpace(a.RequestSHA256) == "" {
		return fmt.Errorf("request_sha256 is required")
	}
	if strings.TrimSpace(a.BundleSHA256) == "" {
		return fmt.Errorf("bundle_sha256 is required")
	}
	if strings.TrimSpace(a.MaterializedSHA256) == "" {
		return fmt.Errorf("materialized_sha256 is required")
	}
	if a.AllowedUse != AllowedUseManualReviewOnly {
		return fmt.Errorf("allowed_use %q must be %q", a.AllowedUse, AllowedUseManualReviewOnly)
	}
	if a.RunnerInjectionAllowed {
		return fmt.Errorf("runner_injection_allowed must be false")
	}
	if !a.ConfirmApproveMaterializedContext {
		return fmt.Errorf("%s must be true", confirmApproveMaterializedCtx)
	}
	return nil
}

func writeApprovalRequestJSON(path string, request ApprovalRequest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create request output dir: %w", err)
	}
	data, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval request json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("approval request must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write approval request %q: %w", path, err)
	}
	return nil
}

func writeMaterializedContextApprovalJSON(path string, approval MaterializedContextApproval) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create approval output dir: %w", err)
	}
	data, err := json.MarshalIndent(approval, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal materialized context approval json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), `"text_excerpt"`) {
		return fmt.Errorf("approval must not contain text_excerpt")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write materialized context approval %q: %w", path, err)
	}
	return nil
}

func WriteInspectApprovalText(result InspectApprovalResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "retrieval_context_approval_inspect:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "status: %s\n", result.Status); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "approved: %t\n", result.Approved); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "bundle_sha256: %s\n", result.BundleSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "materialized_sha256: %s\n", result.MaterializedSHA256); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "runner_injection_allowed: %t\n", result.RunnerInjectionAllowed); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "allowed_use: %s\n", result.AllowedUse); err != nil {
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

func WriteInspectApprovalJSON(result InspectApprovalResult, out io.Writer) error {
	encoder := json.NewEncoder(out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
