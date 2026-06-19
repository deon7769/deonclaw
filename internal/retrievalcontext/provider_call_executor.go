package retrievalcontext

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

var ErrProviderTransportNotEnabled = errors.New("provider transport not enabled by policy")

// ProviderExecutor validates and plans provider calls without real execution (Task 22.38).
type ProviderExecutor interface {
	Validate(ctx context.Context, opts ProviderCallExecutorOptions) (ProviderCallExecutorResult, error)
	Plan(ctx context.Context, opts ProviderCallExecutorPlanOptions) (ProviderCallExecutorResult, error)
	DryRun(ctx context.Context, opts ProviderCallExecutorDryRunOptions) (ProviderCallExecutorDryRunResult, error)
}

// SkeletonProviderExecutor is a policy-blocked executor with no real transport.
type SkeletonProviderExecutor struct {
	Transport ProviderTransport
}

func NewSkeletonProviderExecutor() *SkeletonProviderExecutor {
	return &SkeletonProviderExecutor{Transport: BlockedProviderTransport{}}
}

func (e *SkeletonProviderExecutor) Validate(ctx context.Context, opts ProviderCallExecutorOptions) (ProviderCallExecutorResult, error) {
	_ = ctx
	return providerCallExecutorCore(opts, providerCallExecutorCoreMode{})
}

func (e *SkeletonProviderExecutor) Plan(ctx context.Context, opts ProviderCallExecutorPlanOptions) (ProviderCallExecutorResult, error) {
	_ = ctx
	if err := validateRelativeSafePath("plan output path", opts.OutputPath); err != nil {
		return ProviderCallExecutorResult{}, err
	}
	return providerCallExecutorCore(opts.ProviderCallExecutorOptions, providerCallExecutorCoreMode{plan: true, outputPath: opts.OutputPath})
}

func (e *SkeletonProviderExecutor) DryRun(ctx context.Context, opts ProviderCallExecutorDryRunOptions) (ProviderCallExecutorDryRunResult, error) {
	_ = ctx
	_ = e
	return ProviderCallExecutorDryRun(opts)
}

type ProviderCallExecutorOptions struct {
	ExecutorConfigPath   string
	DispatchConfigPath   string
	ProviderRunPlanPath  string
	PayloadDryRunPath    string
	PayloadOutputPath    string
	PayloadReportPath    string
	ProviderCallGatePath string
	ReadinessReportPath  string
	ApprovalRequestPath  string
	ApprovalPath         string
	ExecutionBundlePath  string
}

type ProviderCallExecutorPlanOptions struct {
	ProviderCallExecutorOptions
	OutputPath string
}

type ProviderCallExecutorResult struct {
	Status                          string   `json:"status"`
	ExecutorPolicyValidated         bool     `json:"executor_policy_validated"`
	ExecutorConfigValidated         bool     `json:"executor_config_validated"`
	ExecutionSupportedNow           bool     `json:"execution_supported_now"`
	ProviderCallAuthorizedForFuture bool     `json:"provider_call_authorized_for_future"`
	ChainContinuityReady            bool     `json:"chain_continuity_ready"`
	ProviderCallAllowedNow          bool     `json:"provider_call_allowed_now"`
	ProviderCall                    bool     `json:"provider_call"`
	NetworkCall                     bool     `json:"network_call"`
	WorkerExecution                 bool     `json:"worker_execution"`
	SentToProvider                  bool     `json:"sent_to_provider"`
	PromptInjectionRealRunner       bool     `json:"prompt_injection_real_runner"`
	BlockedReason                   string   `json:"blocked_reason"`
	ExecutorConfigSHA256            string   `json:"executor_config_sha256"`
	ExecutionBundleSHA256           string   `json:"execution_bundle_sha256"`
	ApprovalSHA256                  string   `json:"approval_sha256"`
	ProviderPayloadSHA256           string   `json:"provider_payload_sha256"`
	MaterializedSHA256              string   `json:"materialized_sha256,omitempty"`
	PlanSteps                       []string `json:"plan_steps,omitempty"`
	Warnings                        []string `json:"warnings,omitempty"`
	Failures                        []string `json:"failures,omitempty"`
}

func ProviderCallExecutorValidate(opts ProviderCallExecutorOptions) (ProviderCallExecutorResult, error) {
	return NewSkeletonProviderExecutor().Validate(context.Background(), opts)
}

func ProviderCallExecutorPlan(opts ProviderCallExecutorPlanOptions) (ProviderCallExecutorResult, error) {
	return NewSkeletonProviderExecutor().Plan(context.Background(), opts)
}

func LoadProviderCallExecutionBundle(path string) (ProviderCallExecutionBundleResult, []byte, error) {
	if err := validateRelativeSafePath("execution bundle path", path); err != nil {
		return ProviderCallExecutionBundleResult{}, nil, err
	}
	data, err := readArtifactBytesNoTextExcerpt("execution bundle", path)
	if err != nil {
		return ProviderCallExecutionBundleResult{}, nil, err
	}
	var bundle ProviderCallExecutionBundleResult
	if err := json.Unmarshal(data, &bundle); err != nil {
		return ProviderCallExecutionBundleResult{}, nil, fmt.Errorf("parse execution bundle: %w", err)
	}
	return bundle, data, nil
}

type providerCallExecutorCoreMode struct {
	plan              bool
	skipPayloadOutput bool
	outputPath        string
}

func providerCallExecutorCore(opts ProviderCallExecutorOptions, mode providerCallExecutorCoreMode) (ProviderCallExecutorResult, error) {
	pathChecks := []struct {
		field string
		path  string
	}{
		{"executor config path", opts.ExecutorConfigPath},
		{"dispatch config path", opts.DispatchConfigPath},
		{"provider run plan path", opts.ProviderRunPlanPath},
		{"payload dry-run path", opts.PayloadDryRunPath},
		{"payload report path", opts.PayloadReportPath},
		{"provider call gate path", opts.ProviderCallGatePath},
		{"readiness report path", opts.ReadinessReportPath},
		{"approval request path", opts.ApprovalRequestPath},
		{"approval path", opts.ApprovalPath},
		{"execution bundle path", opts.ExecutionBundlePath},
	}
	if !mode.skipPayloadOutput {
		pathChecks = append(pathChecks, struct {
			field string
			path  string
		}{"payload output path", opts.PayloadOutputPath})
	}
	for _, check := range pathChecks {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallExecutorResult{}, err
		}
	}

	result := ProviderCallExecutorResult{
		Status:                    lancedbpolicy.StatusOK,
		ExecutorPolicyValidated:   false,
		ExecutorConfigValidated:   false,
		ExecutionSupportedNow:     false,
		ProviderCallAllowedNow:    false,
		ProviderCall:              false,
		NetworkCall:               false,
		WorkerExecution:           false,
		SentToProvider:            false,
		PromptInjectionRealRunner: false,
		BlockedReason:             ProviderCallExecutorBlockedReason,
	}

	var failures []string
	var warnings []string

	executorCfgData, err := readArtifactBytesNoTextExcerpt("executor config", opts.ExecutorConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.ExecutorConfigSHA256 = sha256Hex(executorCfgData)
		executorCfg, err := ParseProviderCallExecutorConfig(executorCfgData)
		if err != nil {
			failures = append(failures, err.Error())
		} else {
			validateResult, err := ProviderCallExecutorConfigValidate(executorCfg)
			if err != nil {
				failures = append(failures, err.Error())
			} else if validateResult.Status == lancedbpolicy.StatusFailed {
				failures = append(failures, "executor config validation failed")
				failures = append(failures, validateResult.Failures...)
			} else {
				result.ExecutorPolicyValidated = true
				warnings = mergeWarnings(warnings, validateResult.Warnings)
			}
		}
	}

	bundle, bundleData, err := LoadProviderCallExecutionBundle(opts.ExecutionBundlePath)
	bundleLoaded := err == nil
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		result.ExecutionBundleSHA256 = sha256Hex(bundleData)
		result.ProviderPayloadSHA256 = bundle.ProviderPayloadSHA256
		result.MaterializedSHA256 = bundle.MaterializedSHA256
		warnings = mergeWarnings(warnings, bundle.Warnings)

		if bundle.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "execution bundle status must not be failed")
			failures = append(failures, bundle.Failures...)
		}
		if !bundle.ProviderCallAuthorizedForFuture {
			failures = append(failures, "execution bundle provider_call_authorized_for_future must be true")
		}
		if bundle.ProviderCallAllowedNow {
			failures = append(failures, "execution bundle provider_call_allowed_now must be false")
		}
		if bundle.SentToProvider {
			failures = append(failures, "execution bundle sent_to_provider must be false")
		}
		if bundle.ProviderCall || bundle.NetworkCall || bundle.WorkerExecution {
			failures = append(failures, "execution bundle must keep provider_call, network_call, and worker_execution false")
		}
		if bundle.BlockedReason != MaterializedProviderDispatchBlockedReason && bundle.BlockedReason != ProviderCallExecutorBlockedReason {
			failures = append(failures, fmt.Sprintf("execution bundle blocked_reason %q must be %q", bundle.BlockedReason, ProviderCallExecutorBlockedReason))
		}
	}

	audit, err := ProviderCallChainContinuityAudit(ProviderCallChainContinuityAuditOptions{
		DispatchConfigPath:       opts.DispatchConfigPath,
		ProviderRunPlanPath:      opts.ProviderRunPlanPath,
		PayloadDryRunPath:        opts.PayloadDryRunPath,
		PayloadOutputPath:        opts.PayloadOutputPath,
		PayloadReportPath:        opts.PayloadReportPath,
		ProviderCallGatePath:     opts.ProviderCallGatePath,
		ReadinessReportPath:      opts.ReadinessReportPath,
		ApprovalRequestPath:      opts.ApprovalRequestPath,
		ApprovalPath:             opts.ApprovalPath,
		ExecutionBundlePath:      opts.ExecutionBundlePath,
		SkipPayloadOutputContent: mode.skipPayloadOutput,
	})
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		warnings = mergeWarnings(warnings, audit.Warnings)
		if !audit.ChainContinuityReady {
			failures = append(failures, "chain audit chain_continuity_ready must be true")
		}
		if audit.ProviderCall || audit.NetworkCall || audit.WorkerExecution || audit.SentToProvider {
			failures = append(failures, "chain audit must keep provider_call, network_call, worker_execution, and sent_to_provider false")
		}
		if audit.Status == lancedbpolicy.StatusFailed {
			failures = append(failures, "chain audit failed")
			failures = append(failures, audit.Failures...)
		}
		if result.ProviderPayloadSHA256 == "" {
			result.ProviderPayloadSHA256 = audit.ProviderPayloadSHA256
		} else if audit.ProviderPayloadSHA256 != "" && audit.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
			failures = append(failures, "provider_payload_sha256 mismatch between execution bundle and chain audit")
		}
		if result.MaterializedSHA256 == "" {
			result.MaterializedSHA256 = audit.MaterializedSHA256
		}
	}

	approvalData, err := readArtifactBytesNoTextExcerpt("provider call approval", opts.ApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
	} else {
		approvalInspect, err := InspectProviderCallApprovalBytes(approvalData, InspectProviderCallApprovalOptions{RequestPath: opts.ApprovalRequestPath})
		if err != nil {
			failures = append(failures, err.Error())
		} else if approvalInspect.Status != lancedbpolicy.StatusOK {
			failures = append(failures, "provider call approval inspect failed")
			failures = append(failures, approvalInspect.Failures...)
		} else {
			approval, err := ParseProviderCallApprovalJSON(approvalData)
			if err != nil {
				failures = append(failures, err.Error())
			} else {
				result.ApprovalSHA256 = sha256Hex(approvalData)
				if bundleLoaded && bundle.ApprovalSHA256 != "" && result.ApprovalSHA256 != bundle.ApprovalSHA256 {
					failures = append(failures, "approval sha256 mismatch with execution bundle")
				}
				if !approval.ProviderCallAuthorizedForFuture {
					failures = append(failures, "approval provider_call_authorized_for_future must be true")
				}
				if approval.ProviderCallAllowedNow {
					failures = append(failures, "approval provider_call_allowed_now must be false")
				}
				if approval.SentToProvider {
					failures = append(failures, "approval sent_to_provider must be false")
				}
				if approval.ProviderCallAllowedNow || approval.NetworkCallAllowedNow || approval.WorkerExecutionAllowedNow {
					failures = append(failures, "approval must keep provider_call, network_call, and worker_execution blocked")
				}
				if result.ProviderPayloadSHA256 != "" && approval.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
					failures = append(failures, "approval provider_payload_sha256 mismatch")
				}
				warnings = mergeWarnings(warnings, approval.Warnings)
			}
		}
	}

	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 && result.ExecutorPolicyValidated {
		result.ExecutorConfigValidated = true
		result.ProviderCallAuthorizedForFuture = true
		result.ChainContinuityReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
		if planMode := mode.plan; planMode {
			result.PlanSteps = []string{
				"validate_executor_policy",
				"load_execution_bundle",
				"run_chain_continuity_audit",
				"inspect_provider_call_approval",
				"reconcile_hashes_and_policy_flags",
				"blocked: implementation_not_enabled",
			}
			if err := writeProviderCallExecutorPlanJSON(mode.outputPath, result); err != nil {
				return ProviderCallExecutorResult{}, err
			}
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func writeProviderCallExecutorPlanJSON(path string, result ProviderCallExecutorResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create provider call executor plan output dir: %w", err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor plan json: %w", err)
	}
	data = append(data, '\n')
	if strings.Contains(string(data), "text_excerpt") || strings.Contains(string(data), "alpha text") {
		return fmt.Errorf("provider call executor plan must not contain materialized preview text")
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write provider call executor plan %q: %w", path, err)
	}
	return nil
}

func WriteProviderCallExecutorText(result ProviderCallExecutorResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_executor:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"executor_policy_validated", fmt.Sprintf("%t", result.ExecutorPolicyValidated)},
		{"executor_config_validated", fmt.Sprintf("%t", result.ExecutorConfigValidated)},
		{"execution_supported_now", fmt.Sprintf("%t", result.ExecutionSupportedNow)},
		{"provider_call_authorized_for_future", fmt.Sprintf("%t", result.ProviderCallAuthorizedForFuture)},
		{"chain_continuity_ready", fmt.Sprintf("%t", result.ChainContinuityReady)},
		{"provider_call_allowed_now", fmt.Sprintf("%t", result.ProviderCallAllowedNow)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"prompt_injection_real_runner", fmt.Sprintf("%t", result.PromptInjectionRealRunner)},
		{"blocked_reason", result.BlockedReason},
		{"executor_config_sha256", result.ExecutorConfigSHA256},
		{"execution_bundle_sha256", result.ExecutionBundleSHA256},
		{"approval_sha256", result.ApprovalSHA256},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.PlanSteps) > 0 {
		if _, err := fmt.Fprintln(out, "\nplan_steps:"); err != nil {
			return err
		}
		for _, step := range result.PlanSteps {
			if _, err := fmt.Fprintf(out, "- %s\n", step); err != nil {
				return err
			}
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
		return fmt.Errorf("provider call executor text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallExecutorJSON(result ProviderCallExecutorResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call executor json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call executor json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
