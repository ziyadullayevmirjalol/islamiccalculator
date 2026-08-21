package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// TazkiyaDeclared: the impure amount is known directly.
	TazkiyaDeclared = "declared"
	// TazkiyaDividend: purge a dividend by the company's impure-income ratio.
	TazkiyaDividend = "dividend"
)

type TazkiyaInput struct {
	Mode           string
	TotalIncome    decimal.Decimal // declared mode
	ImpureAmount   decimal.Decimal // declared mode
	DividendAmount decimal.Decimal // dividend mode
	ImpureRatio    decimal.Decimal // dividend mode, [0..1]
}

type TazkiyaResult struct {
	Mode        string
	PurgeAmount decimal.Decimal // must be given to charity — not deductible, no reward expected
	CleanAmount decimal.Decimal
}

// CalculateTazkiya computes how much of an income must be purged to
// charity because it is interest or otherwise impure.
func CalculateTazkiya(in TazkiyaInput) (TazkiyaResult, error) {
	fields := map[string]string{}
	switch in.Mode {
	case TazkiyaDeclared:
		if !in.TotalIncome.IsPositive() {
			fields["totalIncome"] = "must_be_positive"
		}
		if in.ImpureAmount.IsNegative() {
			fields["impureAmount"] = "must_not_be_negative"
		}
		if in.ImpureAmount.GreaterThan(in.TotalIncome) {
			fields["impureAmount"] = "exceeds_total_income"
		}
	case TazkiyaDividend:
		if !in.DividendAmount.IsPositive() {
			fields["dividendAmount"] = "must_be_positive"
		}
		one := decimal.NewFromInt(1)
		if in.ImpureRatio.IsNegative() || in.ImpureRatio.GreaterThan(one) {
			fields["impureRatio"] = "out_of_range"
		}
	default:
		fields["mode"] = "must_be_declared_or_dividend"
	}
	if len(fields) > 0 {
		return TazkiyaResult{}, apperr.Validation("invalid tazkiya input", fields)
	}

	var purge, total decimal.Decimal
	if in.Mode == TazkiyaDeclared {
		total = money.Round2(in.TotalIncome)
		purge = money.Round2(in.ImpureAmount)
	} else {
		total = money.Round2(in.DividendAmount)
		purge = money.Round2(total.Mul(in.ImpureRatio))
	}

	return TazkiyaResult{
		Mode:        in.Mode,
		PurgeAmount: purge,
		CleanAmount: total.Sub(purge),
	}, nil
}
