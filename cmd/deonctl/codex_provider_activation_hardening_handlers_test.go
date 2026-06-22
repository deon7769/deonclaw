package main

import (
	"strings"
	"testing"
)

func TestParseCodexProviderActivationOperatorReviewBundleRequiresAllFlags(t *testing.T) {
	_, err := parseCodexProviderActivationOperatorReviewBundleOptions(nil)
	if err == nil || !strings.Contains(err.Error(), "missing required flags") {
		t.Fatalf("error = %v, want missing required flags", err)
	}
}

func TestParseCodexProviderActivationOperatorReviewReportRequiresAllFlags(t *testing.T) {
	_, err := parseCodexProviderActivationOperatorReviewReportOptions([]string{
		"--operator-review-bundle", "bundle.json",
	})
	if err == nil || !strings.Contains(err.Error(), "missing required flags") {
		t.Fatalf("error = %v, want missing required flags", err)
	}
}

func TestParseCodexProviderActivationFinalAuditRequiresAllFlags(t *testing.T) {
	_, err := parseCodexProviderActivationFinalAuditOptions([]string{
		"--activation-release-package", "pkg.json",
		"--activation-release-gate", "gate.json",
	})
	if err == nil || !strings.Contains(err.Error(), "missing required flags") {
		t.Fatalf("error = %v, want missing required flags", err)
	}
}

func TestParseCodexProviderActivationCIReportRequiresAllFlags(t *testing.T) {
	_, err := parseCodexProviderActivationCIReportOptions([]string{
		"--kill-switch-plan", "kill.json",
	})
	if err == nil || !strings.Contains(err.Error(), "missing required flags") {
		t.Fatalf("error = %v, want missing required flags", err)
	}
}
