package budget

import "testing"

func TestCanReserveWithinLimit(t *testing.T) {
	window := Window{HardLimitMicroUSD: 1_000_000, ReservedMicroUSD: 100_000, CommittedMicroUSD: 200_000}
	policy := Policy{MaxSingleRunMicroUSD: 500_000}
	if err := CanReserve(window, policy, 250_000); err != nil {
		t.Fatalf("CanReserve() error = %v", err)
	}
}

func TestCanReserveFailsWhenExhausted(t *testing.T) {
	window := Window{HardLimitMicroUSD: 1_000_000, ReservedMicroUSD: 400_000, CommittedMicroUSD: 700_000}
	policy := Policy{MaxSingleRunMicroUSD: 500_000}
	if err := CanReserve(window, policy, 100_000); err == nil {
		t.Fatal("CanReserve() expected budget exhausted error")
	}
}

func TestCommitAmount(t *testing.T) {
	reservation := Reservation{Status: ReservationReserved, EstimatedMicroUSD: 300_000}
	updated, release, commit, overage, err := CommitAmount(reservation, 180_000)
	if err != nil {
		t.Fatalf("CommitAmount() error = %v", err)
	}
	if updated.Status != ReservationCommitted || release != 300_000 || commit != 180_000 || overage != 0 {
		t.Fatalf("CommitAmount() = %+v release=%d commit=%d overage=%d", updated, release, commit, overage)
	}
}

func TestCommitAmountOverage(t *testing.T) {
	reservation := Reservation{Status: ReservationReserved, EstimatedMicroUSD: 300_000}
	updated, release, commit, overage, err := CommitAmount(reservation, 450_000)
	if err != nil {
		t.Fatalf("CommitAmount() error = %v", err)
	}
	if updated.Status != ReservationOverBudget || release != 300_000 || commit != 450_000 || overage != 150_000 {
		t.Fatalf("CommitAmount() = %+v release=%d commit=%d overage=%d", updated, release, commit, overage)
	}
}
