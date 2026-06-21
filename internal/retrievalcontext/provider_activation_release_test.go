package retrievalcontext_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/deon7769/deonclaw/internal/lancedbpolicy"
	"github.com/deon7769/deonclaw/internal/retrievalcontext"
)

func TestProviderActivationReleasePackageOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationControlPlaneChain(t, chain)

	pkg, err := retrievalcontext.ProviderActivationReleasePackage(retrievalcontext.ProviderActivationReleasePackageOptions{
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		OutputPath:                       artifacts.activationReleasePackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReleasePackage() error = %v", err)
	}
	assertProviderActivationReleasePackageBlocked(t, pkg)
	assertNoTextExcerpt(t, artifacts.activationReleasePackagePath)
}

func TestProviderActivationReleaseGateOK(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationControlPlaneChain(t, chain)

	gate, err := retrievalcontext.ProviderActivationReleaseGate(retrievalcontext.ProviderActivationReleaseGateOptions{
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReleaseGate() error = %v", err)
	}
	assertProviderActivationReleaseGateBlocked(t, gate)
}

func TestProviderActivationReleaseGateFailsWhenActivationAllowedNowTrue(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	chain := setupProviderCallChainFixture(t)
	artifacts := runProviderActivationControlPlaneChain(t, chain)

	pkgData, err := os.ReadFile(artifacts.activationReleasePackagePath)
	if err != nil {
		t.Fatalf("ReadFile(release package) error = %v", err)
	}
	patched := bytes.Replace(pkgData, []byte(`"activation_allowed_now": false`), []byte(`"activation_allowed_now": true`), 1)
	if err := os.WriteFile(artifacts.activationReleasePackagePath, patched, 0o644); err != nil {
		t.Fatalf("WriteFile(release package) error = %v", err)
	}

	gate, err := retrievalcontext.ProviderActivationReleaseGate(retrievalcontext.ProviderActivationReleaseGateOptions{
		ActivationPolicyPlanPath:         artifacts.activationPolicyPlanPath,
		ActivationApprovalPath:           artifacts.activationApprovalPath,
		ActivationRehearsalPath:          artifacts.activationRehearsalPath,
		ActivationReadinessAuditPath:     artifacts.activationReadinessAuditPath,
		RealCallProposalPath:             artifacts.realCallProposalPath,
		ExecutionSimulationReportPath:    artifacts.simulationReportPath,
		CredentialPolicyPlanPath:         artifacts.credentialPolicyPlanPath,
		ResponseChangeProposalReportPath: artifacts.changeProposalReportPath,
		ActivationReleasePackagePath:     artifacts.activationReleasePackagePath,
	})
	if err != nil {
		t.Fatalf("ProviderActivationReleaseGate() error = %v", err)
	}
	if gate.Status != lancedbpolicy.StatusFailed {
		t.Fatal("release gate status must be failed after activation_allowed_now tamper")
	}
}

func assertProviderActivationReleasePackageBlocked(t *testing.T, result retrievalcontext.ProviderActivationReleasePackageResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("release package status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ActivationReleasePackageReady {
		t.Fatal("activation_release_package_ready = false, want true")
	}
	if result.RealActivationSupportedNow || result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.SentToProvider || result.WorkspaceModified {
		t.Fatalf("activation release package flags must stay blocked: %#v", result)
	}
}

func assertProviderActivationReleaseGateBlocked(t *testing.T, result retrievalcontext.ProviderActivationReleaseGateResult) {
	t.Helper()
	if result.Status != lancedbpolicy.StatusOK {
		t.Fatalf("release gate status = %q, failures=%#v", result.Status, result.Failures)
	}
	if !result.ActivationGateReady || !result.ActivationReleasePackageReady {
		t.Fatalf("activation_gate_ready=%t activation_release_package_ready=%t, want both true", result.ActivationGateReady, result.ActivationReleasePackageReady)
	}
	if result.RealActivationSupportedNow || result.ActivationAllowedNow || result.ProviderCall || result.NetworkCall || result.SecretValuesRead || result.TransportCalled || result.SentToProvider || result.WorkspaceModified {
		t.Fatalf("activation release gate flags must stay blocked: %#v", result)
	}
}
