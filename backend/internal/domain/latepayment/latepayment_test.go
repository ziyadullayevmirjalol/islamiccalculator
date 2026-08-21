package latepayment

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

func TestCalculate(t *testing.T) {
	t.Run("rate mode accrues per day", func(t *testing.T) {
		res, err := Calculate(Input{
			Mode:          ModeRate,
			OverdueAmount: d("10000000"),
			DaysLate:      73, // 73/365 = exactly 0.2 of a year
			AnnualRate:    d("0.10"),
		})
		require.NoError(t, err)
		assert.True(t, res.CharityDue.Equal(d("200000")), "10M × 10%% × 0.2y, got %s", res.CharityDue)
		assert.Equal(t, DispositionCharity, res.Disposition, "late fees are never income")
	})

	t.Run("flat mode charges the fee once", func(t *testing.T) {
		res, err := Calculate(Input{
			Mode:          ModeFlat,
			OverdueAmount: d("10000000"),
			DaysLate:      30,
			FlatFee:       d("50000"),
		})
		require.NoError(t, err)
		assert.True(t, res.CharityDue.Equal(d("50000")))
	})
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"bad mode", Input{Mode: "daily", OverdueAmount: d("1"), DaysLate: 1}, "mode"},
		{"zero overdue", Input{Mode: ModeFlat, DaysLate: 1}, "overdueAmount"},
		{"zero days", Input{Mode: ModeFlat, OverdueAmount: d("1")}, "daysLate"},
		{"rate at 100%", Input{Mode: ModeRate, OverdueAmount: d("1"), DaysLate: 1, AnnualRate: d("1")}, "annualRate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Calculate(tc.in)
			require.Error(t, err)
			assert.Contains(t, apperr.From(err).Fields, tc.field)
		})
	}
}
