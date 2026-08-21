package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// IrrigationNatural: rain-fed or naturally watered land — 10%.
	IrrigationNatural = "natural"
	// IrrigationIrrigated: artificially irrigated at cost — 5%.
	IrrigationIrrigated = "irrigated"
)

type UshrParams struct {
	NaturalRate   decimal.Decimal // e.g. 0.10
	IrrigatedRate decimal.Decimal // e.g. 0.05
}

type UshrInput struct {
	IrrigationType string
	HarvestValue   decimal.Decimal // market value of the harvest
}

type UshrResult struct {
	IrrigationType string
	HarvestValue   decimal.Decimal
	Rate           decimal.Decimal
	UshrDue        decimal.Decimal
}

// CalculateUshr computes the harvest zakat. Following the Hanafi position
// (the default for this app), ushr applies to any harvest amount — there
// is no minimum threshold.
func CalculateUshr(in UshrInput, p UshrParams) (UshrResult, error) {
	fields := map[string]string{}
	if in.IrrigationType != IrrigationNatural && in.IrrigationType != IrrigationIrrigated {
		fields["irrigationType"] = "must_be_natural_or_irrigated"
	}
	if !in.HarvestValue.IsPositive() {
		fields["harvestValue"] = "must_be_positive"
	}
	if len(fields) > 0 {
		return UshrResult{}, apperr.Validation("invalid ushr input", fields)
	}
	if !p.NaturalRate.IsPositive() || !p.IrrigatedRate.IsPositive() {
		return UshrResult{}, apperr.Internal("invalid ushr parameters", nil)
	}

	rate := p.NaturalRate
	if in.IrrigationType == IrrigationIrrigated {
		rate = p.IrrigatedRate
	}
	value := money.Round2(in.HarvestValue)

	return UshrResult{
		IrrigationType: in.IrrigationType,
		HarvestValue:   value,
		Rate:           rate,
		UshrDue:        money.Round2(value.Mul(rate)),
	}, nil
}
