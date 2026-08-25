package ijara

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

func TestCalculate_ProfitMode(t *testing.T) {
	res, err := Calculate(Input{
		Mode:          ModeProfit,
		AssetCost:     d("100000000"),
		Profit:        Profit{Mode: ProfitModeRate, Value: d("0.20")},
		TransferPrice: d("10000000"),
		TermMonths:    24,
	})
	require.NoError(t, err)

	assert.True(t, res.TotalRentals.Equal(d("110000000")), "rentals = cost+profit-transfer, got %s", res.TotalRentals)
	assert.True(t, res.TotalReceipts.Equal(d("120000000")))
	assert.True(t, res.ProfitTotal.Equal(d("20000000")))
	assert.True(t, res.ProfitRate.Equal(d("0.2")))
	assert.True(t, res.MonthlyRent.Equal(d("4583333.33")))
	require.Len(t, res.Schedule, 24)
	assert.True(t, res.Schedule[23].Amount.Equal(d("4583333.41")), "final rental absorbs residual, got %s", res.Schedule[23].Amount)
	assert.True(t, res.Schedule[23].Balance.IsZero())
}

func TestCalculate_RentMode(t *testing.T) {
	res, err := Calculate(Input{
		Mode:          ModeRent,
		AssetCost:     d("100000000"),
		MonthlyRent:   d("5000000"),
		TransferPrice: d("10000000"),
		TermMonths:    24,
	})
	require.NoError(t, err)

	assert.True(t, res.TotalRentals.Equal(d("120000000")))
	assert.True(t, res.ProfitTotal.Equal(d("30000000")))
	assert.True(t, res.ProfitRate.Equal(d("0.3")))
	assert.True(t, res.MonthlyRent.Equal(d("5000000")))
}

func TestCalculate_RentModeCanShowLoss(t *testing.T) {
	res, err := Calculate(Input{
		Mode:        ModeRent,
		AssetCost:   d("100000000"),
		MonthlyRent: d("1000000"),
		TermMonths:  12,
	})
	require.NoError(t, err)
	assert.True(t, res.ProfitTotal.IsNegative(), "rent too low shows a loss, not an error")
}

func TestCalculate_AdvanceReducesSchedule(t *testing.T) {
	res, err := Calculate(Input{
		Mode:           ModeProfit,
		AssetCost:      d("100000000"),
		Profit:         Profit{Mode: ProfitModeAmount, Value: d("20000000")},
		TransferPrice:  d("10000000"),
		AdvancePayment: d("10000000"),
		TermMonths:     20,
	})
	require.NoError(t, err)
	assert.True(t, res.TotalRentals.Equal(d("100000000")))
	assert.True(t, res.TotalReceipts.Equal(d("120000000")), "receipts include advance and transfer")
	assert.True(t, res.MonthlyRent.Equal(d("5000000")))
}

func TestCalculate_ScheduleInvariant(t *testing.T) {
	res, err := Calculate(Input{
		Mode:          ModeProfit,
		AssetCost:     d("77777777.77"),
		Profit:        Profit{Mode: ProfitModeRate, Value: d("0.1234")},
		TransferPrice: d("1000000"),
		TermMonths:    37,
	})
	require.NoError(t, err)

	sum := decimal.Zero
	for _, r := range res.Schedule {
		assert.False(t, r.Amount.IsNegative())
		sum = sum.Add(r.Amount)
	}
	assert.True(t, sum.Equal(res.TotalRentals), "schedule sums to total rentals")
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"bad mode", Input{Mode: "lease", AssetCost: d("1"), TermMonths: 12}, "mode"},
		{"zero cost", Input{Mode: ModeRent, MonthlyRent: d("1"), TermMonths: 12}, "assetCost"},
		{"zero rent in rent mode", Input{Mode: ModeRent, AssetCost: d("100"), TermMonths: 12}, "monthlyRent"},
		{"bad profit mode", Input{Mode: ModeProfit, AssetCost: d("100"), Profit: Profit{"pct", d("1")}, TermMonths: 12}, "profit.mode"},
		{"profit rate above 500% is a unit mistake", Input{Mode: ModeProfit, AssetCost: d("100"), Profit: Profit{ProfitModeRate, d("20")}, TermMonths: 12}, "profit.value"},
		{"negative transfer", Input{Mode: ModeRent, AssetCost: d("100"), MonthlyRent: d("1"), TransferPrice: d("-1"), TermMonths: 12}, "transferPrice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Calculate(tc.in)
			require.Error(t, err)
			assert.Contains(t, apperr.From(err).Fields, tc.field)
		})
	}

	t.Run("transfer swallowing the whole target is rejected", func(t *testing.T) {
		_, err := Calculate(Input{
			Mode:          ModeProfit,
			AssetCost:     d("100"),
			Profit:        Profit{Mode: ProfitModeRate, Value: d("0.10")},
			TransferPrice: d("110"),
			TermMonths:    12,
		})
		require.Error(t, err)
		assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
	})
}
