package money

import (
	"fmt"
	"strconv"
)

func FormatUSD(micro int64) string {
	sign := ""
	if micro < 0 {
		sign = "-"
		micro = -micro
	}
	dollars := micro / 1_000_000
	frac := micro % 1_000_000
	return sign + fmt.Sprintf("%d.%06d", dollars, frac)
}

func ParseUSDString(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, err
	}
	return int64(f * 1_000_000), nil
}
