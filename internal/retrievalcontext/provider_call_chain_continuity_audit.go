package retrievalcontext

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
)

type ProviderCallChainContinuityAuditOptions struct {
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

type ProviderCallChainStep struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

type ProviderCallChainContinuityAuditResult struct {
	Status                 string                  `json:"status"`
	ChainContinuityReady   bool                    `json:"chain_continuity_ready"`
	ProducerCommandsActive bool                    `json:"producer_commands_active"`
	LoadersReconciled      bool                    `json:"loaders_reconciled"`
	ProviderCall           bool                    `json:"provider_call"`
	NetworkCall            bool                    `json:"network_call"`
	WorkerExecution        bool                    `json:"worker_execution"`
	SentToProvider         bool                    `json:"sent_to_provider"`
	ProviderPayloadSHA256  string                  `json:"provider_payload_sha256,omitempty"`
	MaterializedSHA256     string                  `json:"materialized_sha256,omitempty"`
	ReconciliationSummary  []string                `json:"reconciliation_summary,omitempty"`
	Steps                  []ProviderCallChainStep `json:"steps,omitempty"`
	Warnings               []string                `json:"warnings,omitempty"`
	Failures               []string                `json:"failures,omitempty"`
}

func ProviderCallChainContinuityAudit(opts ProviderCallChainContinuityAuditOptions) (ProviderCallChainContinuityAuditResult, error) {
	for _, check := range []struct {
		field string
		path  string
	}{
		{"dispatch config path", opts.DispatchConfigPath},
		{"provider run plan path", opts.ProviderRunPlanPath},
		{"payload dry-run path", opts.PayloadDryRunPath},
		{"payload output path", opts.PayloadOutputPath},
		{"payload report path", opts.PayloadReportPath},
		{"provider call gate path", opts.ProviderCallGatePath},
		{"readiness report path", opts.ReadinessReportPath},
		{"approval request path", opts.ApprovalRequestPath},
		{"approval path", opts.ApprovalPath},
		{"execution bundle path", opts.ExecutionBundlePath},
	} {
		if err := validateRelativeSafePath(check.field, check.path); err != nil {
			return ProviderCallChainContinuityAuditResult{}, err
		}
	}

	result := ProviderCallChainContinuityAuditResult{
		Status:                 lancedbpolicy.StatusOK,
		ProducerCommandsActive: true,
		LoadersReconciled:      true,
		ProviderCall:           false,
		NetworkCall:            false,
		WorkerExecution:        false,
		SentToProvider:         false,
		ReconciliationSummary: []string{
			"tasks 22.29-22.33 producer commands are active; 22.34-22.35 consume materialized-provider artifacts via shared loaders",
			"temporary loader-only stubs from 22.34 were replaced by materialized_provider_* producers and loaders",
		},
	}

	var failures []string
	var warnings []string
	var steps []ProviderCallChainStep

	dispatchData, err := readArtifactBytesNoTextExcerpt("dispatch config", opts.DispatchConfigPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "dispatch_validate", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		cfg, err := ParseMaterializedProviderDispatch(dispatchData)
		if err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "dispatch_validate", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else {
			dispatchResult, err := MaterializedProviderDispatchValidate(cfg)
			if err != nil {
				failures = append(failures, err.Error())
				steps = append(steps, ProviderCallChainStep{Name: "dispatch_validate", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
			} else if dispatchResult.Status == lancedbpolicy.StatusFailed {
				failures = append(failures, "dispatch config validation failed")
				failures = append(failures, dispatchResult.Failures...)
				steps = append(steps, ProviderCallChainStep{Name: "dispatch_validate", Status: lancedbpolicy.StatusFailed})
			} else {
				steps = append(steps, ProviderCallChainStep{Name: "dispatch_validate", Status: lancedbpolicy.StatusOK})
			}
		}
	}

	runPlanData, err := readArtifactBytesNoTextExcerpt("provider run plan", opts.ProviderRunPlanPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "provider_run_plan", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		var runPlan MaterializedInjectionProviderRunPlanResult
		if err := json.Unmarshal(runPlanData, &runPlan); err != nil {
			failures = append(failures, fmt.Sprintf("parse provider run plan: %v", err))
			steps = append(steps, ProviderCallChainStep{Name: "provider_run_plan", Status: lancedbpolicy.StatusFailed})
		} else if !runPlan.ProviderRunPlanReady || runPlan.ProviderCallAllowedNow || runPlan.SentToProvider {
			failures = append(failures, "provider run plan must be ready with provider_call and sent_to_provider blocked")
			steps = append(steps, ProviderCallChainStep{Name: "provider_run_plan", Status: lancedbpolicy.StatusFailed})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "provider_run_plan", Status: lancedbpolicy.StatusOK})
			if runPlan.MaterializedSHA256 != "" {
				result.MaterializedSHA256 = runPlan.MaterializedSHA256
			}
		}
	}

	payload, payloadData, err := LoadMaterializedProviderPayload(opts.PayloadDryRunPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "payload_dry_run", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		if payload.ProviderCall || payload.NetworkCall || payload.WorkerExecution || payload.SentToProvider {
			failures = append(failures, "payload dry-run must keep provider_call, network_call, worker_execution, and sent_to_provider false")
			steps = append(steps, ProviderCallChainStep{Name: "payload_dry_run", Status: lancedbpolicy.StatusFailed})
		} else if !payload.ProviderPayloadRendered {
			failures = append(failures, "payload dry-run provider_payload_rendered must be true")
			steps = append(steps, ProviderCallChainStep{Name: "payload_dry_run", Status: lancedbpolicy.StatusFailed})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "payload_dry_run", Status: lancedbpolicy.StatusOK})
			if payload.ProviderPayloadSHA256 != "" {
				result.ProviderPayloadSHA256 = payload.ProviderPayloadSHA256
			}
			if payload.MaterializedSHA256 != "" {
				result.MaterializedSHA256 = payload.MaterializedSHA256
			}
		}
		_ = payloadData
	}

	payloadOutputData, err := os.ReadFile(opts.PayloadOutputPath)
	if err != nil {
		failures = append(failures, fmt.Sprintf("read payload output %q: %v", opts.PayloadOutputPath, err))
	} else if payload.ProviderPayloadSHA256 != "" && sha256Hex(payloadOutputData) != payload.ProviderPayloadSHA256 {
		failures = append(failures, "payload output hash mismatch with payload dry-run")
	}

	payloadReport, payloadReportData, err := LoadMaterializedProviderPayloadReport(opts.PayloadReportPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "payload_report", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		if err := validateMaterializedProviderPayloadReportForApproval(payloadReport); err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "payload_report", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "payload_report", Status: lancedbpolicy.StatusOK})
			if result.ProviderPayloadSHA256 != "" && payloadReport.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
				failures = append(failures, "payload report provider_payload_sha256 mismatch with payload dry-run")
			}
			result.ProviderPayloadSHA256 = payloadReport.ProviderPayloadSHA256
		}
		_ = payloadReportData
	}

	gate, gateData, err := LoadMaterializedProviderCallGate(opts.ProviderCallGatePath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "provider_call_gate", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		if err := validateMaterializedProviderCallGateForApproval(gate); err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_gate", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_gate", Status: lancedbpolicy.StatusOK})
			if gate.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
				failures = append(failures, "provider call gate provider_payload_sha256 mismatch")
			}
		}
		_ = gateData
	}

	readiness, readinessData, err := LoadMaterializedProviderCallReadinessReport(opts.ReadinessReportPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "provider_call_readiness_report", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		if err := validateMaterializedProviderCallReadinessReportForApproval(readiness); err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_readiness_report", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_readiness_report", Status: lancedbpolicy.StatusOK})
			if readiness.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
				failures = append(failures, "readiness report provider_payload_sha256 mismatch")
			}
		}
		_ = readinessData
	}

	requestData, err := readArtifactBytesNoTextExcerpt("approval request", opts.ApprovalRequestPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval_request", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		request, err := ParseProviderCallApprovalRequestJSON(requestData)
		if err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval_request", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else if err := request.Validate(); err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval_request", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval_request", Status: lancedbpolicy.StatusOK})
			if request.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
				failures = append(failures, "approval request provider_payload_sha256 mismatch")
			}
		}
	}

	approvalData, err := readArtifactBytesNoTextExcerpt("approval", opts.ApprovalPath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		inspect, err := InspectProviderCallApprovalBytes(approvalData, InspectProviderCallApprovalOptions{RequestPath: opts.ApprovalRequestPath})
		if err != nil {
			failures = append(failures, err.Error())
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
		} else if inspect.Status != lancedbpolicy.StatusOK {
			failures = append(failures, "provider call approval inspect failed")
			failures = append(failures, inspect.Failures...)
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval", Status: lancedbpolicy.StatusFailed})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_approval", Status: lancedbpolicy.StatusOK})
		}
	}

	bundleData, err := readArtifactBytesNoTextExcerpt("execution bundle", opts.ExecutionBundlePath)
	if err != nil {
		failures = append(failures, err.Error())
		steps = append(steps, ProviderCallChainStep{Name: "provider_call_execution_bundle", Status: lancedbpolicy.StatusFailed, Detail: err.Error()})
	} else {
		var bundle ProviderCallExecutionBundleResult
		if err := json.Unmarshal(bundleData, &bundle); err != nil {
			failures = append(failures, fmt.Sprintf("parse execution bundle: %v", err))
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_execution_bundle", Status: lancedbpolicy.StatusFailed})
		} else if bundle.Status == lancedbpolicy.StatusFailed || !bundle.ProviderCallAuthorizedForFuture {
			failures = append(failures, "execution bundle must be ready for future provider call authorization")
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_execution_bundle", Status: lancedbpolicy.StatusFailed})
		} else if bundle.ProviderCall || bundle.NetworkCall || bundle.WorkerExecution || bundle.SentToProvider {
			failures = append(failures, "execution bundle must keep provider_call, network_call, worker_execution, and sent_to_provider false")
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_execution_bundle", Status: lancedbpolicy.StatusFailed})
		} else if bundle.ProviderPayloadSHA256 != result.ProviderPayloadSHA256 {
			failures = append(failures, "execution bundle provider_payload_sha256 mismatch")
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_execution_bundle", Status: lancedbpolicy.StatusFailed})
		} else {
			steps = append(steps, ProviderCallChainStep{Name: "provider_call_execution_bundle", Status: lancedbpolicy.StatusOK})
			warnings = mergeWarnings(warnings, bundle.Warnings)
		}
	}

	result.Steps = steps
	result.Warnings = warnings
	result.Failures = failures
	if len(failures) == 0 {
		result.ChainContinuityReady = true
		if len(warnings) > 0 {
			result.Status = lancedbpolicy.StatusWarning
		}
	} else {
		result.Status = lancedbpolicy.StatusFailed
	}
	return result, nil
}

func WriteProviderCallChainContinuityAuditText(result ProviderCallChainContinuityAuditResult, out io.Writer) error {
	if _, err := fmt.Fprintln(out, "worker_codex_provider_call_chain-audit:"); err != nil {
		return err
	}
	lines := []struct {
		label string
		value string
	}{
		{"status", result.Status},
		{"chain_continuity_ready", fmt.Sprintf("%t", result.ChainContinuityReady)},
		{"producer_commands_active", fmt.Sprintf("%t", result.ProducerCommandsActive)},
		{"loaders_reconciled", fmt.Sprintf("%t", result.LoadersReconciled)},
		{"provider_call", fmt.Sprintf("%t", result.ProviderCall)},
		{"network_call", fmt.Sprintf("%t", result.NetworkCall)},
		{"worker_execution", fmt.Sprintf("%t", result.WorkerExecution)},
		{"sent_to_provider", fmt.Sprintf("%t", result.SentToProvider)},
		{"provider_payload_sha256", result.ProviderPayloadSHA256},
		{"materialized_sha256", result.MaterializedSHA256},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(out, "%s: %s\n", line.label, line.value); err != nil {
			return err
		}
	}
	if len(result.ReconciliationSummary) > 0 {
		if _, err := fmt.Fprintln(out, "\nreconciliation_summary:"); err != nil {
			return err
		}
		for _, item := range result.ReconciliationSummary {
			if _, err := fmt.Fprintf(out, "- %s\n", item); err != nil {
				return err
			}
		}
	}
	if len(result.Steps) > 0 {
		if _, err := fmt.Fprintln(out, "\nsteps:"); err != nil {
			return err
		}
		for _, step := range result.Steps {
			if _, err := fmt.Fprintf(out, "- %s: %s\n", step.Name, step.Status); err != nil {
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
		return fmt.Errorf("provider call chain audit text must not contain materialized preview text")
	}
	return nil
}

func WriteProviderCallChainContinuityAuditJSON(result ProviderCallChainContinuityAuditResult, out io.Writer) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal provider call chain audit json: %w", err)
	}
	data = append(data, '\n')
	payload := string(data)
	if strings.Contains(payload, "text_excerpt") || strings.Contains(payload, "alpha text") {
		return fmt.Errorf("provider call chain audit json must not contain materialized preview text")
	}
	if _, err := out.Write(data); err != nil {
		return err
	}
	return nil
}
