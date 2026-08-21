package musharaka

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

// Two partners: A puts 70% of capital but agreed to 50% of profit
// (A is passive, B runs the business).
func partners() []Partner {
	return []Partner{
		{Name: "A", Capital: d("70000000"), ProfitSharePercent: d("50")},
		{Name: "B", Capital: d("30000000"), ProfitSharePercent: d("50")},
	}
}

func TestCalculate_ProfitFollowsAgreedRatio(t *testing.T) {
	res, err := Calculate(Input{
		Partners:   partners(),
		ResultType: ResultProfit,
		Amount:     d("20000000"),
	})
	require.NoError(t, err)

	assert.Equal(t, "agreed_ratio", res.Basis)
	assert.True(t, res.Shares[0].Amount.Equal(d("10000000")), "A gets the agreed 50%%, not the 70%% capital share")
	assert.True(t, res.Shares[1].Amount.Equal(d("10000000")))
	assert.True(t, res.Shares[0].CapitalShare.Equal(d("0.7")))
}

func TestCalculate_LossAlwaysFollowsCapital(t *testing.T) {
	// Same agreed 50/50 profit split — but the loss MUST fall 70/30.
	res, err := Calculate(Input{
		Partners:   partners(),
		ResultType: ResultLoss,
		Amount:     d("10000000"),
	})
	require.NoError(t, err)

	assert.Equal(t, "capital_ratio", res.Basis)
	assert.True(t, res.Shares[0].Amount.Equal(d("7000000")), "A bears 70%% of the loss per capital, agreed ratio ignored")
	assert.True(t, res.Shares[1].Amount.Equal(d("3000000")))
	assert.True(t, res.Shares[0].AppliedShare.Equal(d("0.7")))
}

func TestCalculate_DistributionInvariant(t *testing.T) {
	in := Input{
		Partners: []Partner{
			{Name: "A", Capital: d("1000000"), ProfitSharePercent: d("33.33")},
			{Name: "B", Capital: d("2000000"), ProfitSharePercent: d("33.33")},
			{Name: "C", Capital: d("4000000"), ProfitSharePercent: d("33.34")},
		},
		ResultType: ResultProfit,
		Amount:     d("1000.01"),
	}
	res, err := Calculate(in)
	require.NoError(t, err)

	sum := decimal.Zero
	for _, s := range res.Shares {
		sum = sum.Add(s.Amount)
	}
	assert.True(t, sum.Equal(d("1000.01")), "shares sum exactly to the distributed amount, got %s", sum)
}

func TestCalculate_Validation(t *testing.T) {
	t.Run("profit shares must sum to 100", func(t *testing.T) {
		in := Input{
			Partners: []Partner{
				{Capital: d("100"), ProfitSharePercent: d("60")},
				{Capital: d("100"), ProfitSharePercent: d("30")},
			},
			ResultType: ResultProfit,
			Amount:     d("10"),
		}
		_, err := Calculate(in)
		require.Error(t, err)
		assert.Equal(t, "profit_shares_must_sum_to_100", apperr.From(err).Fields["partners"])
	})

	t.Run("a single partner is not a partnership", func(t *testing.T) {
		in := Input{
			Partners:   []Partner{{Capital: d("100"), ProfitSharePercent: d("100")}},
			ResultType: ResultProfit,
			Amount:     d("10"),
		}
		_, err := Calculate(in)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "partners")
	})

	t.Run("zero-capital and zero-profit-share partners rejected", func(t *testing.T) {
		in := Input{
			Partners: []Partner{
				{Capital: d("0"), ProfitSharePercent: d("50")},
				{Capital: d("100"), ProfitSharePercent: d("0")},
			},
			ResultType: ResultLoss,
			Amount:     d("10"),
		}
		_, err := Calculate(in)
		require.Error(t, err)
		e := apperr.From(err)
		assert.Contains(t, e.Fields, "partners[0].capital")
		assert.Contains(t, e.Fields, "partners[1].profitSharePercent")
	})

	t.Run("bad result type", func(t *testing.T) {
		in := Input{Partners: partners(), ResultType: "breakeven", Amount: d("10")}
		_, err := Calculate(in)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "resultType")
	})
}
