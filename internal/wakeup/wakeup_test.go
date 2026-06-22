package wakeup

import (
	"testing"
	"time"
)

func TestRecoverStaleClaimsMarksLost(t *testing.T) {
	now := time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)
	claimedAt := now.Add(-20 * time.Minute).Format(time.RFC3339Nano)
	items := []Wakeup{{
		ID: "w1", Status: StatusClaimed, ClaimedAt: claimedAt,
	}}
	updated, report := RecoverStaleClaims(items, now, 15*time.Minute)
	if updated[0].Status != StatusLost {
		t.Fatalf("status = %q", updated[0].Status)
	}
	if len(report.Findings) != 1 {
		t.Fatalf("findings = %d", len(report.Findings))
	}
}

func TestClaimIncrementsAttempt(t *testing.T) {
	now := time.Now().UTC()
	result, err := Claim(Wakeup{ID: "w1", Status: StatusQueued}, now)
	if err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if !result.Claimed || result.Wakeup.Attempt != 1 {
		t.Fatalf("claim result = %+v", result)
	}
}
