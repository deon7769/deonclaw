package retrievalcontext_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

var providerActivationFinalAuditBlockedJSONKeys = []string{
	"real_activation_supported_now",
	"activation_allowed_now",
	"provider_call",
	"network_call",
	"secret_values_read",
	"transport_called",
	"sent_to_provider",
	"received_from_provider",
	"workspace_modified",
	"diff_applied",
	"commit_created",
	"pr_created",
	"worker_execution",
	"prompt_injection_real_runner",
}

func assertProviderActivationFinalAuditJSONBlocked(t *testing.T, data []byte) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal(final audit json) error = %v", err)
	}
	for _, key := range providerActivationFinalAuditBlockedJSONKeys {
		value, ok := raw[key]
		if !ok {
			t.Fatalf("final audit json missing required field %q", key)
		}
		var blocked bool
		if err := json.Unmarshal(value, &blocked); err != nil {
			t.Fatalf("Unmarshal(%q) error = %v", key, err)
		}
		if blocked {
			t.Fatalf("final audit json %q must be false", key)
		}
	}
}

func TestWriteProviderActivationFinalAuditJSONAntiLeak(t *testing.T) {
	result := retrievalcontext.ProviderActivationFinalAuditResult{
		Status:           "ok",
		FinalAuditReady:  true,
		KillSwitchActive: true,
		BlockedReason:    retrievalcontext.ProviderCallExecutorBlockedReason,
		Failures:         []string{"note: text_excerpt marker must not appear"},
	}
	var buf bytes.Buffer
	err := retrievalcontext.WriteProviderActivationFinalAuditJSON(result, &buf)
	if err == nil || !strings.Contains(err.Error(), "materialized preview text") {
		t.Fatalf("WriteProviderActivationFinalAuditJSON() error = %v, want preview text rejection", err)
	}
}

func TestProviderActivationFinalAuditJSONIncludesAllBlockedFlags(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationHardeningChain(t, chain)

	audit, err := retrievalcontext.ProviderActivationFinalAudit(retrievalcontext.ProviderActivationFinalAuditOptions{
		ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
		ActivationReleaseGatePath:        artifacts.activationReleaseGatePath,
		OperatorReviewBundlePath:         artifacts.operatorReviewBundlePath,
		KillSwitchPlanPath:               artifacts.killSwitchPlanPath,
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationFinalAudit() error = %v", err)
	}
	assertProviderActivationFinalAuditBlocked(t, audit)

	var buf bytes.Buffer
	if err := retrievalcontext.WriteProviderActivationFinalAuditJSON(audit, &buf); err != nil {
		t.Fatalf("WriteProviderActivationFinalAuditJSON() error = %v", err)
	}
	assertProviderActivationFinalAuditJSONBlocked(t, buf.Bytes())

	ciReport, err := retrievalcontext.ProviderActivationCIReport(retrievalcontext.ProviderActivationCIReportOptions{
		ProviderActivationFinalAuditOptions: retrievalcontext.ProviderActivationFinalAuditOptions{
			ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
			ActivationReleaseGatePath:        artifacts.activationReleaseGatePath,
			OperatorReviewBundlePath:         artifacts.operatorReviewBundlePath,
			KillSwitchPlanPath:               artifacts.killSwitchPlanPath,
			ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
			ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
			RealCallProposalPath:             artifacts.realCallProposalPath,
			CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
			ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
			ActivationApprovalPath:           artifacts.activationApprovalPath,
			ActivationRehearsalPath:          artifacts.activationRehearsalPath,
			ExecutionSimulationReportPath:    artifacts.simulationReportPath,
		},
	})
	if err != nil {
		t.Fatalf("ProviderActivationCIReport() error = %v", err)
	}
	assertProviderActivationCIReportBlocked(t, ciReport)

	var ciBuf bytes.Buffer
	if err := retrievalcontext.WriteProviderActivationCIReportJSON(ciReport, &ciBuf); err != nil {
		t.Fatalf("WriteProviderActivationCIReportJSON() error = %v", err)
	}
	assertProviderActivationFinalAuditJSONBlocked(t, ciBuf.Bytes())
}
