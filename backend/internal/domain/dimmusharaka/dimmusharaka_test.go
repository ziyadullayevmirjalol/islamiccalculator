package dimmusharaka

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

// Reference case from CALCULATION_FORMULA_RULES.md §1.6:
// 300,000 property, 60,000 down (20%), 5%/yr on the bank's share, 20 years.
func TestCalculate_GoldenValues(t *testing.T) {
	res, err := Calculate(Input{
		PropertyValue:    d("300000"),
		DownPayment:      d("60000"),
		AnnualRentalRate: d("0.05"),
		TermMonths:       240,
	})
	require.NoError(t, err)

	assert.True(t, res.BankFinancing.Equal(d("240000")))
	assert.True(t, res.InitialOwnershipPercent.Equal(d("0.2")))
	assert.True(t, res.MonthlyAcquisition.Equal(d("1000")))
	assert.True(t, res.Schedule[0].Rent.Equal(d("1000")), "240,000 × 5% / 12")
	assert.True(t, res.FirstMonthPayment.Equal(d("2000")))

	// Declining-share rent: sum over shares 240k, 239k, …, 1k at 5%/12.
	assert.True(t, res.TotalRent.Equal(d("120500")), "total rent = %s", res.TotalRent)
	assert.True(t, res.TotalAcquisition.Equal(d("240000")))
	assert.True(t, res.TotalPaid.Equal(d("420500")))

	last := res.Schedule[239]
	assert.True(t, last.OwnershipPercent.Equal(d("1")), "full ownership at term end")
	assert.True(t, last.BankShareBefore.Equal(d("1000")))
}

func TestCalculate_RentDeclines(t *testing.T) {
	res, err := Calculate(Input{
		PropertyValue:    d("100000000"),
		DownPayment:      d("20000000"),
		AnnualRentalRate: d("0.12"),
		TermMonths:       120,
	})
	require.NoError(t, err)

	for i := 1; i < len(res.Schedule); i++ {
		assert.True(t, res.Schedule[i].Rent.LessThanOrEqual(res.Schedule[i-1].Rent),
			"rent must never increase (month %d)", i+1)
	}

	// Acquisitions must sum exactly to the financing (residual pattern).
	sum := decimal.Zero
	for _, m := range res.Schedule {
		sum = sum.Add(m.Acquisition)
	}
	assert.True(t, sum.Equal(res.BankFinancing))
}

func TestCalculate_ZeroRateIsValidCoOwnership(t *testing.T) {
	res, err := Calculate(Input{
		PropertyValue: d("1000"), DownPayment: d("0"),
		AnnualRentalRate: d("0"), TermMonths: 10,
	})
	require.NoError(t, err)
	assert.True(t, res.TotalRent.IsZero())
	assert.True(t, res.TotalPaid.Equal(d("1000")))
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"zero property", Input{DownPayment: d("0"), AnnualRentalRate: d("0.05"), TermMonths: 12}, "propertyValue"},
		{"down swallows property", Input{PropertyValue: d("100"), DownPayment: d("100"), AnnualRentalRate: d("0.05"), TermMonths: 12}, "downPayment"},
		{"negative down", Input{PropertyValue: d("100"), DownPayment: d("-1"), AnnualRentalRate: d("0.05"), TermMonths: 12}, "downPayment"},
		{"rate at 100%", Input{PropertyValue: d("100"), AnnualRentalRate: d("1"), TermMonths: 12}, "annualRentalRate"},
		{"term too long", Input{PropertyValue: d("100"), AnnualRentalRate: d("0.05"), TermMonths: 361}, "termMonths"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Calculate(tc.in)
			require.Error(t, err)
			assert.Contains(t, apperr.From(err).Fields, tc.field)
		})
	}
}
