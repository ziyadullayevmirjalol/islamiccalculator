package salam

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
	// 100 tons of wheat at 2.4M advance vs 3M expected spot.
	res, err := Calculate(Input{
		Quantity:          d("100"),
		UnitPrice:         d("2400000"),
		ExpectedUnitPrice: d("3000000"),
	})
	require.NoError(t, err)

	assert.True(t, res.AdvanceTotal.Equal(d("240000000")))
	assert.True(t, res.ExpectedMarketValue.Equal(d("300000000")))
	assert.True(t, res.ExpectedMargin.Equal(d("60000000")))
	assert.True(t, res.MarginRate.Equal(d("0.25")))
}

func TestCalculate_MarginCanBeNegative(t *testing.T) {
	res, err := Calculate(Input{
		Quantity:          d("10"),
		UnitPrice:         d("1000"),
		ExpectedUnitPrice: d("900"),
	})
	require.NoError(t, err)
	assert.True(t, res.ExpectedMargin.Equal(d("-1000")), "expected price below advance shows the buyer's risk")
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"zero quantity", Input{UnitPrice: d("1"), ExpectedUnitPrice: d("1")}, "quantity"},
		{"zero unit price", Input{Quantity: d("1"), ExpectedUnitPrice: d("1")}, "unitPrice"},
		{"zero expected price", Input{Quantity: d("1"), UnitPrice: d("1")}, "expectedUnitPrice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Calculate(tc.in)
			require.Error(t, err)
			assert.Contains(t, apperr.From(err).Fields, tc.field)
		})
	}
}
