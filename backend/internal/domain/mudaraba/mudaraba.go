// Package mudaraba implements the deposit-side expectation calculator for
// Mudaraba (profit-sharing) and Wakala (agency) deposits. The output is
// an EXPECTATION from an indicative pool rate — never a guarantee; a
// guaranteed return on a deposit would be riba, and Result.Guaranteed is
// hard-wired false to keep clients honest about that.
package mudaraba

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// ModeMudaraba shares pool profit by an agreed ratio.
	ModeMudaraba = "mudaraba"
	// ModeWakala pays pool profit minus a fixed agency fee rate.
	ModeWakala = "wakala"

	MaxTermMonths = 120
)

type Input struct {
	Mode           string
	Amount         decimal.Decimal
	PoolRateAnnual decimal.Decimal // indicative pool profit rate, e.g. 0.18
	ShareRatio     decimal.Decimal // mudaraba: depositor's share of pool profit, (0..1]
	WakalaFeeRate  decimal.Decimal // wakala: annual agency fee rate, [0..pool rate]
	TermMonths     int
}

type Result struct {
	Mode                  string
	Amount                decimal.Decimal
	EffectiveAnnualRate   decimal.Decimal // 6 decimal places
	ExpectedProfit        decimal.Decimal
	ExpectedMonthlyProfit decimal.Decimal // informational: profit / term, rounded
	ExpectedTotal         decimal.Decimal
	TermMonths            int
	Guaranteed            bool // always false — expected profit is not a promise
}

func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	var effective decimal.Decimal
	if in.Mode == ModeMudaraba {
		effective = in.PoolRateAnnual.Mul(in.ShareRatio)
	} else {
		effective = in.PoolRateAnnual.Sub(in.WakalaFeeRate)
	}
	effective = effective.Round(6)

	amount := money.Round2(in.Amount)
	months := decimal.NewFromInt(int64(in.TermMonths))
	profit := money.Round2(amount.Mul(effective).Mul(months).Div(decimal.NewFromInt(12)))

	return Result{
		Mode:                  in.Mode,
		Amount:                amount,
		EffectiveAnnualRate:   effective,
		ExpectedProfit:        profit,
		ExpectedMonthlyProfit: money.Round2(profit.Div(months)),
		ExpectedTotal:         amount.Add(profit),
		TermMonths:            in.TermMonths,
		Guaranteed:            false,
	}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if in.Mode != ModeMudaraba && in.Mode != ModeWakala {
		fields["mode"] = "must_be_mudaraba_or_wakala"
	}
	if !in.Amount.IsPositive() {
		fields["amount"] = "must_be_positive"
	}
	one := decimal.NewFromInt(1)
	if in.PoolRateAnnual.IsNegative() || in.PoolRateAnnual.GreaterThanOrEqual(one) {
		fields["poolRateAnnual"] = "out_of_range"
	}
	if in.TermMonths < 1 || in.TermMonths > MaxTermMonths {
		fields["termMonths"] = "out_of_range"
	}
	switch in.Mode {
	case ModeMudaraba:
		if !in.ShareRatio.IsPositive() || in.ShareRatio.GreaterThan(one) {
			fields["shareRatio"] = "out_of_range"
		}
	case ModeWakala:
		if in.WakalaFeeRate.IsNegative() {
			fields["wakalaFeeRate"] = "must_not_be_negative"
		} else if in.WakalaFeeRate.GreaterThan(in.PoolRateAnnual) {
			fields["wakalaFeeRate"] = "exceeds_pool_rate"
		}
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid deposit input", fields)
	}
	return nil
}
