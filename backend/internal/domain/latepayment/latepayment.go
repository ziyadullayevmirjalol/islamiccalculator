// Package latepayment computes the charity amount charged on overdue
// installments. The amount is a deterrent, not income: it is routed 100%
// to charity, which is why the result's Disposition is hard-wired — an
// Islamic financier may never profit from a customer's delay.
package latepayment

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// ModeRate accrues per day from an annual rate on the overdue amount.
	ModeRate = "rate"
	// ModeFlat charges a fixed fee per overdue installment.
	ModeFlat = "flat"

	DispositionCharity = "charity"
)

type Input struct {
	Mode          string
	OverdueAmount decimal.Decimal
	DaysLate      int
	AnnualRate    decimal.Decimal // rate mode, e.g. 0.10
	FlatFee       decimal.Decimal // flat mode
}

type Result struct {
	Mode          string
	OverdueAmount decimal.Decimal
	DaysLate      int
	CharityDue    decimal.Decimal
	Disposition   string // always "charity"
}

func Calculate(in Input) (Result, error) {
	fields := map[string]string{}
	if !in.OverdueAmount.IsPositive() {
		fields["overdueAmount"] = "must_be_positive"
	}
	if in.DaysLate < 1 {
		fields["daysLate"] = "must_be_positive"
	}
	one := decimal.NewFromInt(1)
	switch in.Mode {
	case ModeRate:
		if in.AnnualRate.IsNegative() || in.AnnualRate.GreaterThanOrEqual(one) {
			fields["annualRate"] = "out_of_range"
		}
	case ModeFlat:
		if in.FlatFee.IsNegative() {
			fields["flatFee"] = "must_not_be_negative"
		}
	default:
		fields["mode"] = "must_be_rate_or_flat"
	}
	if len(fields) > 0 {
		return Result{}, apperr.Validation("invalid late payment input", fields)
	}

	overdue := money.Round2(in.OverdueAmount)
	var due decimal.Decimal
	if in.Mode == ModeRate {
		due = money.Round2(overdue.
			Mul(in.AnnualRate).
			Mul(decimal.NewFromInt(int64(in.DaysLate))).
			Div(decimal.NewFromInt(365)))
	} else {
		due = money.Round2(in.FlatFee)
	}

	return Result{
		Mode:          in.Mode,
		OverdueAmount: overdue,
		DaysLate:      in.DaysLate,
		CharityDue:    due,
		Disposition:   DispositionCharity,
	}, nil
}
