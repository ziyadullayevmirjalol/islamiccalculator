// Package sukuk computes expected income and yield for sukuk (Islamic
// certificate) positions and portfolios. Distributions come from the
// underlying assets' performance, so — as with mudaraba deposits — every
// figure is an EXPECTATION, and Guaranteed is hard-wired false.
package sukuk

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	MaxTermMonths = 600
	MaxPositions  = 50
)

type Position struct {
	Name                   string
	FaceValue              decimal.Decimal
	PurchasePrice          decimal.Decimal
	DistributionRateAnnual decimal.Decimal // expected, on face value
	Frequency              int             // payments per year: 1, 2, 4, or 12
	TermMonths             int             // must align with frequency
}

type PositionResult struct {
	Name                       string
	FaceValue                  decimal.Decimal
	PurchasePrice              decimal.Decimal
	DistributionRateAnnual     decimal.Decimal
	Frequency                  int
	TermMonths                 int
	Payments                   int
	PeriodicDistribution       decimal.Decimal
	TotalExpectedDistributions decimal.Decimal
	RedemptionAtFace           decimal.Decimal
	ExpectedGain               decimal.Decimal // distributions + redemption - purchase
	CurrentYield               decimal.Decimal // annual income / purchase price, 4dp
}

type PortfolioResult struct {
	Positions             []PositionResult
	TotalInvested         decimal.Decimal
	TotalFace             decimal.Decimal
	TotalAnnualIncome     decimal.Decimal
	PortfolioCurrentYield decimal.Decimal // total annual income / total invested, 4dp
	TotalExpectedGain     decimal.Decimal
	Guaranteed            bool // always false
}

func Calculate(positions []Position) (PortfolioResult, error) {
	if len(positions) < 1 || len(positions) > MaxPositions {
		return PortfolioResult{}, apperr.Validation("invalid sukuk input",
			map[string]string{"positions": "need_1_to_50_positions"})
	}

	out := PortfolioResult{Positions: make([]PositionResult, len(positions))}
	invested, face, annualIncome, gain := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero

	for i, p := range positions {
		res, err := calculatePosition(i, p)
		if err != nil {
			return PortfolioResult{}, err
		}
		out.Positions[i] = res
		invested = invested.Add(res.PurchasePrice)
		face = face.Add(res.FaceValue)
		annualIncome = annualIncome.Add(money.Round2(res.FaceValue.Mul(res.DistributionRateAnnual)))
		gain = gain.Add(res.ExpectedGain)
	}

	out.TotalInvested = invested
	out.TotalFace = face
	out.TotalAnnualIncome = annualIncome
	out.PortfolioCurrentYield = annualIncome.Div(invested).Round(4)
	out.TotalExpectedGain = gain
	out.Guaranteed = false
	return out, nil
}

func calculatePosition(i int, p Position) (PositionResult, error) {
	fields := map[string]string{}
	f := func(name string) string { return fmt.Sprintf("positions[%d].%s", i, name) }

	if !p.FaceValue.IsPositive() {
		fields[f("faceValue")] = "must_be_positive"
	}
	if !p.PurchasePrice.IsPositive() {
		fields[f("purchasePrice")] = "must_be_positive"
	}
	one := decimal.NewFromInt(1)
	if p.DistributionRateAnnual.IsNegative() || p.DistributionRateAnnual.GreaterThanOrEqual(one) {
		fields[f("distributionRateAnnual")] = "out_of_range"
	}
	monthsPerPeriod := 0
	switch p.Frequency {
	case 1, 2, 4, 12:
		monthsPerPeriod = 12 / p.Frequency
	default:
		fields[f("frequency")] = "must_be_1_2_4_or_12"
	}
	if p.TermMonths < 1 || p.TermMonths > MaxTermMonths {
		fields[f("termMonths")] = "out_of_range"
	} else if monthsPerPeriod > 0 && p.TermMonths%monthsPerPeriod != 0 {
		fields[f("termMonths")] = "must_align_with_frequency"
	}
	if len(fields) > 0 {
		return PositionResult{}, apperr.Validation("invalid sukuk position", fields)
	}

	faceValue := money.Round2(p.FaceValue)
	purchase := money.Round2(p.PurchasePrice)
	payments := p.TermMonths / monthsPerPeriod
	periodic := money.Round2(faceValue.Mul(p.DistributionRateAnnual).Div(decimal.NewFromInt(int64(p.Frequency))))
	totalDist := periodic.Mul(decimal.NewFromInt(int64(payments)))
	annual := money.Round2(faceValue.Mul(p.DistributionRateAnnual))

	return PositionResult{
		Name:                       p.Name,
		FaceValue:                  faceValue,
		PurchasePrice:              purchase,
		DistributionRateAnnual:     p.DistributionRateAnnual,
		Frequency:                  p.Frequency,
		TermMonths:                 p.TermMonths,
		Payments:                   payments,
		PeriodicDistribution:       periodic,
		TotalExpectedDistributions: totalDist,
		RedemptionAtFace:           faceValue,
		ExpectedGain:               totalDist.Add(faceValue).Sub(purchase),
		CurrentYield:               annual.Div(purchase).Round(4),
	}, nil
}
