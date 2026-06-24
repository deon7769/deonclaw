package usage

import "testing"

func TestCalculateCostMicroUSD(t *testing.T) {
	price := ModelPrice{
		InputMicroUSDPerMillion:       200_000,
		OutputMicroUSDPerMillion:      800_000,
		CachedInputMicroUSDPerMillion: 20_000,
	}
	got, err := CalculateCostMicroUSD(price, 1_000_000, 500_000, 100_000)
	if err != nil {
		t.Fatalf("CalculateCostMicroUSD() error = %v", err)
	}
	if got != 582_000 {
		t.Fatalf("CalculateCostMicroUSD() = %d, want 582000", got)
	}
}
