package sukuk

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

func TestCalculate_SinglePosition(t *testing.T) {
	// 100M face bought at 95M, 9% expected, semi-annual, 5 years.
	res, err := Calculate([]Position{{
		Name:                   "UzAuto Sukuk 2031",
		FaceValue:              d("100000000"),
		PurchasePrice:          d("95000000"),
		DistributionRateAnnual: d("0.09"),
		Frequency:              2,
		TermMonths:             60,
	}})
	require.NoError(t, err)

	p := res.Positions[0]
	assert.Equal(t, 10, p.Payments)
	assert.True(t, p.PeriodicDistribution.Equal(d("4500000")))
	assert.True(t, p.TotalExpectedDistributions.Equal(d("45000000")))
	assert.True(t, p.ExpectedGain.Equal(d("50000000")), "45M distributions + 100M redemption - 95M cost")
	assert.True(t, p.CurrentYield.Equal(d("0.0947")), "9M / 95M, got %s", p.CurrentYield)
	assert.False(t, res.Guaranteed, "sukuk income is expected, never promised")
}

func TestCalculate_PortfolioAggregation(t *testing.T) {
	res, err := Calculate([]Position{
		{FaceValue: d("100000000"), PurchasePrice: d("95000000"), DistributionRateAnnual: d("0.09"), Frequency: 2, TermMonths: 60},
		{FaceValue: d("50000000"), PurchasePrice: d("50000000"), DistributionRateAnnual: d("0.12"), Frequency: 4, TermMonths: 36},
	})
	require.NoError(t, err)

	assert.True(t, res.TotalInvested.Equal(d("145000000")))
	assert.True(t, res.TotalFace.Equal(d("150000000")))
	assert.True(t, res.TotalAnnualIncome.Equal(d("15000000")), "9M + 6M")
	assert.True(t, res.PortfolioCurrentYield.Equal(d("0.1034")), "15M / 145M, got %s", res.PortfolioCurrentYield)

	// Position 2: 1.5M quarterly × 12 payments = 18M distributions, zero capital gain.
	assert.True(t, res.Positions[1].TotalExpectedDistributions.Equal(d("18000000")))
	assert.True(t, res.Positions[1].ExpectedGain.Equal(d("18000000")))
}

func TestCalculate_Validation(t *testing.T) {
	valid := Position{FaceValue: d("100"), PurchasePrice: d("100"), DistributionRateAnnual: d("0.1"), Frequency: 2, TermMonths: 12}

	t.Run("empty portfolio", func(t *testing.T) {
		_, err := Calculate(nil)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "positions")
	})

	t.Run("term not aligned with frequency", func(t *testing.T) {
		p := valid
		p.TermMonths = 13 // semi-annual payments need a multiple of 6
		_, err := Calculate([]Position{p})
		require.Error(t, err)
		assert.Equal(t, "must_align_with_frequency", apperr.From(err).Fields["positions[0].termMonths"])
	})

	t.Run("bad frequency", func(t *testing.T) {
		p := valid
		p.Frequency = 3
		_, err := Calculate([]Position{p})
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "positions[0].frequency")
	})

	t.Run("field errors carry the position index", func(t *testing.T) {
		bad := valid
		bad.FaceValue = decimal.Zero
		_, err := Calculate([]Position{valid, bad})
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "positions[1].faceValue")
	})
}
