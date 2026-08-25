package murabaha

import (
	"testing"
	"time"

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

func TestCalculate_GoldenValues(t *testing.T) {
	t.Run("12-month car, 10% markup, no down payment", func(t *testing.T) {
		res, err := Calculate(Input{
			Cost:       d("120000000"),
			Markup:     Markup{Mode: MarkupModeRate, Value: d("0.10")},
			TermMonths: 12,
		})
		require.NoError(t, err)

		assert.True(t, res.MarkupTotal.Equal(d("12000000")))
		assert.True(t, res.SalePrice.Equal(d("132000000")))
		assert.True(t, res.Financed.Equal(d("132000000")))
		assert.True(t, res.MonthlyInstallment.Equal(d("11000000")))
		require.Len(t, res.Schedule, 12)
		assert.True(t, res.Schedule[0].Principal.Equal(d("10000000")))
		assert.True(t, res.Schedule[0].Markup.Equal(d("1000000")))
	})

	t.Run("markup as absolute amount with rounding residual", func(t *testing.T) {
		res, err := Calculate(Input{
			Cost:       d("1000"),
			Markup:     Markup{Mode: MarkupModeAmount, Value: d("1")},
			TermMonths: 3,
		})
		require.NoError(t, err)

		assert.True(t, res.SalePrice.Equal(d("1001")))
		assert.True(t, res.Schedule[0].Amount.Equal(d("333.66")))
		assert.True(t, res.Schedule[2].Amount.Equal(d("333.68")), "final installment absorbs residual")
	})

	t.Run("down payment reduces financed and principal", func(t *testing.T) {
		res, err := Calculate(Input{
			Cost:        d("120000000"),
			Markup:      Markup{Mode: MarkupModeRate, Value: d("0.10")},
			DownPayment: d("20000000"),
			TermMonths:  10,
		})
		require.NoError(t, err)

		assert.True(t, res.Financed.Equal(d("112000000")))
		assert.True(t, res.MonthlyInstallment.Equal(d("11200000")))

		principal := decimal.Zero
		for _, inst := range res.Schedule {
			principal = principal.Add(inst.Principal)
		}
		assert.True(t, principal.Equal(d("100000000")), "principals sum to cost - down, got %s", principal)
	})

	t.Run("due dates advance monthly", func(t *testing.T) {
		start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		res, err := Calculate(Input{
			Cost:         d("1000"),
			Markup:       Markup{Mode: MarkupModeRate, Value: d("0")},
			TermMonths:   3,
			FirstDueDate: start,
		})
		require.NoError(t, err)
		assert.Equal(t, start, res.Schedule[0].DueDate)
		assert.Equal(t, time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), res.Schedule[2].DueDate)
	})
}

// The invariants every schedule must hold, whatever the inputs.
func TestCalculate_ScheduleInvariants(t *testing.T) {
	cases := []Input{
		{Cost: d("120000000"), Markup: Markup{MarkupModeRate, d("0.10")}, TermMonths: 12},
		{Cost: d("1000"), Markup: Markup{MarkupModeAmount, d("1")}, TermMonths: 3},
		{Cost: d("54999999.99"), Markup: Markup{MarkupModeRate, d("0.1775")}, TermMonths: 37, DownPayment: d("5000000")},
		{Cost: d("777.77"), Markup: Markup{MarkupModeAmount, d("0.01")}, TermMonths: 360},
	}
	for _, in := range cases {
		res, err := Calculate(in)
		require.NoError(t, err)

		sum, principal, markup := decimal.Zero, decimal.Zero, decimal.Zero
		for _, inst := range res.Schedule {
			assert.False(t, inst.Amount.IsNegative())
			assert.False(t, inst.Principal.IsNegative())
			sum = sum.Add(inst.Amount)
			principal = principal.Add(inst.Principal)
			markup = markup.Add(inst.Markup)
		}
		assert.True(t, sum.Equal(res.Financed), "schedule sums to financed")
		assert.True(t, principal.Add(markup).Equal(res.Financed), "principal+markup = financed")
		assert.True(t, res.Schedule[len(res.Schedule)-1].Balance.IsZero(), "final balance is zero")
	}
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"zero cost", Input{Markup: Markup{MarkupModeRate, d("0.1")}, TermMonths: 12}, "cost"},
		{"zero term", Input{Cost: d("1000"), Markup: Markup{MarkupModeRate, d("0.1")}}, "termMonths"},
		{"term too long", Input{Cost: d("1000"), Markup: Markup{MarkupModeRate, d("0.1")}, TermMonths: 361}, "termMonths"},
		{"bad markup mode", Input{Cost: d("1000"), Markup: Markup{"percent", d("0.1")}, TermMonths: 12}, "markup.mode"},
		{"negative markup", Input{Cost: d("1000"), Markup: Markup{MarkupModeRate, d("-0.1")}, TermMonths: 12}, "markup.value"},
		{"rate above 500% is a unit mistake", Input{Cost: d("8000000"), Markup: Markup{MarkupModeRate, d("20")}, TermMonths: 12}, "markup.value"},
		{"negative down payment", Input{Cost: d("1000"), Markup: Markup{MarkupModeRate, d("0.1")}, DownPayment: d("-1"), TermMonths: 12}, "downPayment"},
		{"down payment swallows sale", Input{Cost: d("1000"), Markup: Markup{MarkupModeRate, d("0.1")}, DownPayment: d("1100"), TermMonths: 12}, "downPayment"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Calculate(tc.in)
			require.Error(t, err)
			e := apperr.From(err)
			assert.Equal(t, apperr.CodeValidation, e.Code)
			assert.Contains(t, e.Fields, tc.field)
		})
	}
}
