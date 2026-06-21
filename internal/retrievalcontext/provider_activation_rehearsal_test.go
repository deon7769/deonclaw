package retrievalcontext_test

import (
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestProviderActivationRehearsalOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationControlPlaneChain(t, chain)

	rehearsal, err := retrievalcontext.ProviderActivationRehearsal(retrievalcontext.ProviderActivationRehearsalOptions{
		ActivationPolicyPlanPath:      artifacts.activationPolicyPlanPath,
		ActivationApprovalPath:        artifacts.activationApprovalPath,
		ActivationReadinessAuditPath:  artifacts.activationReadinessAuditPath,
		RealCallProposalPath:          artifacts.realCallProposalPath,
		CredentialPolicyPlanPath:      artifacts.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: artifacts.simulationReportPath,
		OutputPath:                    artifacts.activationRehearsalPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationRehearsal() error = %v", err)
	}
	assertProviderActivationRehearsalBlocked(t, rehearsal)
	if len(rehearsal.FutureSteps) == 0 {
		t.Fatal("future_steps must be populated")
	}
	assertNoTextExcerpt(t, artifacts.activationRehearsalPath)

	report, err := retrievalcontext.ProviderActivationRehearsalReport(retrievalcontext.ProviderActivationRehearsalReportOptions{
		RehearsalPath:                 artifacts.activationRehearsalPath,
		ActivationPolicyPlanPath:      artifacts.activationPolicyPlanPath,
		ActivationApprovalPath:        artifacts.activationApprovalPath,
		ActivationReadinessAuditPath:  artifacts.activationReadinessAuditPath,
		RealCallProposalPath:          artifacts.realCallProposalPath,
		CredentialPolicyPlanPath:      artifacts.credentialPolicyPlanPath,
		ExecutionSimulationReportPath: artifacts.simulationReportPath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationRehearsalReport() error = %v", err)
	}
	assertProviderActivationRehearsalBlocked(t, report)
}

func assertProviderActivationRehearsalBlocked(t *testing.T, result retrievalcontext.ProviderActivationRehearsalResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("rehearsal status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ActivationRehearsalReady || !result.ActivationSequenceValidated {
		t.Fatalf("rehearsal ready=%t sequence_validated=%t", result.ActivationRehearsalReady, result.ActivationSequenceValidated)
	}
	if result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.WorkspaceModified {
		t.Fatalf("activation rehearsal flags must stay blocked: %#v", result)
	}
	if result.BlockedReason != retrievalcontext.ProviderCallExecutorBlockedReason {
		t.Fatalf("blocked_reason = %q", result.BlockedReason)
	}
}
