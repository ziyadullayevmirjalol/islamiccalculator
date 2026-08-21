// Package money defines the one rounding policy used by every calculator:
// amounts are decimal (never float), presented at 2 decimal places, and
// installment schedules always sum exactly to their total — the final
// installment absorbs the rounding residual.
package money

import (
	"fmt"

	"github.com/shopspring/decimal"
)

// Parse parses a decimal amount string such as "12500000.50".
func Parse(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, fmt.Errorf("invalid decimal %q", s)
	}
	return d, nil
}

// Round2 rounds to 2 decimal places, half away from zero.
func Round2(d decimal.Decimal) decimal.Decimal {
	return d.Round(2)
}

// Split divides total into n installments at 2 decimal places. The first
// n-1 installments are equal (total/n rounded down so no part overdraws
// the total); the final installment absorbs the residual, so the parts
// always sum exactly to Round2(total).
func Split(total decimal.Decimal, n int) ([]decimal.Decimal, error) {
	if n <= 0 {
		return nil, fmt.Errorf("split count must be positive, got %d", n)
	}
	if total.IsNegative() {
		return nil, fmt.Errorf("split total must not be negative, got %s", total)
	}
	total = Round2(total)

	per := total.Div(decimal.NewFromInt(int64(n))).RoundDown(2)
	parts := make([]decimal.Decimal, n)
	for i := range n - 1 {
		parts[i] = per
	}
	parts[n-1] = total.Sub(per.Mul(decimal.NewFromInt(int64(n - 1))))
	return parts, nil
}

// SplitProportional splits each part of amounts by ratio (0..1), rounding
// every share to 2 decimals except the last, which absorbs the residual so
// the shares sum exactly to shareTotal.
func SplitProportional(amounts []decimal.Decimal, ratio, shareTotal decimal.Decimal) []decimal.Decimal {
	shares := make([]decimal.Decimal, len(amounts))
	rest := Round2(shareTotal)
	for i, a := range amounts {
		if i == len(amounts)-1 {
			shares[i] = rest
			break
		}
		shares[i] = Round2(a.Mul(ratio))
		rest = rest.Sub(shares[i])
	}
	return shares
}
