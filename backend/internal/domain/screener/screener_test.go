package screener

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

func params() Params {
	return Params{
		DebtToMarketCapMax:   d("0.30"),
		InvestToMarketCapMax: d("0.30"),
		ImpureIncomeMax:      d("0.05"),
	}
}

// A clean company: 20% debt, 10% interest investments, 2% impure income.
func cleanInput() Input {
	return Input{
		InterestBearingDebt:        d("200000000"),
		InterestBearingInvestments: d("100000000"),
		MarketCap:                  d("1000000000"),
		ImpureIncome:               d("4000000"),
		TotalRevenue:               d("200000000"),
	}
}

func TestScreen_Compliant(t *testing.T) {
	res, err := Screen(cleanInput(), params())
	require.NoError(t, err)

	assert.Equal(t, VerdictCompliant, res.Verdict)
	assert.True(t, res.ActivityPassed)
	require.Len(t, res.Checks, 3)
	for _, c := range res.Checks {
		assert.True(t, c.Passed, "check %s should pass", c.Key)
	}
	assert.True(t, res.PurificationRatio.Equal(d("0.02")))
}

func TestScreen_EachRuleFailsIndependently(t *testing.T) {
	t.Run("debt at the threshold fails — must stay below", func(t *testing.T) {
		in := cleanInput()
		in.InterestBearingDebt = d("300000000") // exactly 30%
		res, err := Screen(in, params())
		require.NoError(t, err)
		assert.Equal(t, VerdictNonCompliant, res.Verdict)
		assert.False(t, res.Checks[0].Passed)
		assert.True(t, res.Checks[1].Passed)
	})

	t.Run("debt just under the threshold passes", func(t *testing.T) {
		in := cleanInput()
		in.InterestBearingDebt = d("299900000") // 29.99%
		res, err := Screen(in, params())
		require.NoError(t, err)
		assert.Equal(t, VerdictCompliant, res.Verdict)
	})

	t.Run("interest investments over threshold fail", func(t *testing.T) {
		in := cleanInput()
		in.InterestBearingInvestments = d("350000000") // 35%
		res, err := Screen(in, params())
		require.NoError(t, err)
		assert.Equal(t, VerdictNonCompliant, res.Verdict)
		assert.False(t, res.Checks[1].Passed)
	})

	t.Run("impure income at 5% fails", func(t *testing.T) {
		in := cleanInput()
		in.ImpureIncome = d("10000000") // exactly 5% of 200M
		res, err := Screen(in, params())
		require.NoError(t, err)
		assert.Equal(t, VerdictNonCompliant, res.Verdict)
		assert.False(t, res.Checks[2].Passed)
		assert.True(t, res.PurificationRatio.Equal(d("0.05")), "purification ratio still reported")
	})
}

func TestScreen_ProhibitedActivityFailsRegardlessOfRatios(t *testing.T) {
	in := cleanInput()
	in.ProhibitedActivities = []string{"conventional_banking"}
	res, err := Screen(in, params())
	require.NoError(t, err)

	assert.Equal(t, VerdictNonCompliant, res.Verdict)
	assert.False(t, res.ActivityPassed)
	assert.Equal(t, []string{"conventional_banking"}, res.FailedActivities)
	for _, c := range res.Checks {
		assert.True(t, c.Passed, "ratios themselves still pass — activity alone fails the stock")
	}
}

func TestScreen_Validation(t *testing.T) {
	t.Run("zero market cap", func(t *testing.T) {
		in := cleanInput()
		in.MarketCap = decimal.Zero
		_, err := Screen(in, params())
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "marketCap")
	})

	t.Run("impure income above revenue", func(t *testing.T) {
		in := cleanInput()
		in.ImpureIncome = d("300000000")
		_, err := Screen(in, params())
		require.Error(t, err)
		assert.Equal(t, "exceeds_total_revenue", apperr.From(err).Fields["impureIncome"])
	})

	t.Run("broken thresholds are internal errors", func(t *testing.T) {
		p := params()
		p.ImpureIncomeMax = decimal.Zero
		_, err := Screen(cleanInput(), p)
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	})
}
