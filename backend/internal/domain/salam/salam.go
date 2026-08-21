// Package salam implements the Salam forward purchase: the buyer pays the
// FULL price at contract for a commodity (typically a future harvest)
// delivered later. Partial advance is not valid Salam, so the calculator
// only deals in the full advance amount.
package salam

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type Input struct {
	Quantity          decimal.Decimal // e.g. tons of wheat
	UnitPrice         decimal.Decimal // contracted advance price per unit
	ExpectedUnitPrice decimal.Decimal // expected market price per unit at delivery
	DeliveryDate      time.Time       // optional, echoed to the schedule
}

type Result struct {
	Quantity            decimal.Decimal
	UnitPrice           decimal.Decimal
	AdvanceTotal        decimal.Decimal // paid in full at contract
	ExpectedUnitPrice   decimal.Decimal
	ExpectedMarketValue decimal.Decimal
	ExpectedMargin      decimal.Decimal // market value - advance; the buyer's expected gain
	MarginRate          decimal.Decimal // margin / advance, 4 decimal places
	DeliveryDate        time.Time
}

func Calculate(in Input) (Result, error) {
	fields := map[string]string{}
	if !in.Quantity.IsPositive() {
		fields["quantity"] = "must_be_positive"
	}
	if !in.UnitPrice.IsPositive() {
		fields["unitPrice"] = "must_be_positive"
	}
	if !in.ExpectedUnitPrice.IsPositive() {
		fields["expectedUnitPrice"] = "must_be_positive"
	}
	if len(fields) > 0 {
		return Result{}, apperr.Validation("invalid salam input", fields)
	}

	advance := money.Round2(in.Quantity.Mul(in.UnitPrice))
	market := money.Round2(in.Quantity.Mul(in.ExpectedUnitPrice))
	margin := market.Sub(advance)

	return Result{
		Quantity:            in.Quantity,
		UnitPrice:           in.UnitPrice,
		AdvanceTotal:        advance,
		ExpectedUnitPrice:   in.ExpectedUnitPrice,
		ExpectedMarketValue: market,
		ExpectedMargin:      margin,
		MarginRate:          margin.Div(advance).Round(4),
		DeliveryDate:        in.DeliveryDate,
	}, nil
}
