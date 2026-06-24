package usage

import (
	"fmt"
	"math"
)

func CalculateCostMicroUSD(price ModelPrice, inputTokens, outputTokens, cachedInputTokens int64) (int64, error) {
	if inputTokens < 0 || outputTokens < 0 || cachedInputTokens < 0 {
		return 0, fmt.Errorf("token counts must be non-negative")
	}
	uncached := inputTokens - cachedInputTokens
	if uncached < 0 {
		uncached = 0
	}
	parts := []struct {
		tokens int64
		rate   int64
	}{
		{uncached, price.InputMicroUSDPerMillion},
		{cachedInputTokens, price.CachedInputMicroUSDPerMillion},
		{outputTokens, price.OutputMicroUSDPerMillion},
	}
	var total int64
	for _, part := range parts {
		if part.tokens == 0 || part.rate == 0 {
			continue
		}
		if part.tokens > math.MaxInt64/2 {
			return 0, fmt.Errorf("token count overflow")
		}
		numer := part.tokens * part.rate
		if part.rate != 0 && numer/part.rate != part.tokens {
			return 0, fmt.Errorf("cost calculation overflow")
		}
		cost := (numer + 1_000_000 - 1) / 1_000_000
		next, err := addCost(total, cost)
		if err != nil {
			return 0, err
		}
		total = next
	}
	return total, nil
}

func addCost(a, b int64) (int64, error) {
	if a > math.MaxInt64-b {
		return 0, fmt.Errorf("overflow")
	}
	return a + b, nil
}
