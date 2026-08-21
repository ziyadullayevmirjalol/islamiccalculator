package qardhasan

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
	res, err := Calculate(Input{
		Principal:  d("10000000"),
		ServiceFee: d("100000"),
		TermMonths: 10,
	})
	require.NoError(t, err)

	assert.True(t, res.TotalRepayment.Equal(d("10100000")))
	assert.True(t, res.MonthlyInstallment.Equal(d("1000000")))
	require.Len(t, res.Schedule, 10)
	assert.True(t, res.Schedule[9].Balance.IsZero())

	sum := decimal.Zero
	for _, inst := range res.Schedule {
		sum = sum.Add(inst.Amount)
	}
	assert.True(t, sum.Equal(res.Principal), "schedule repays exactly the principal — no markup, ever")
}

func TestCalculate_ZeroFeeIsTheNorm(t *testing.T) {
	res, err := Calculate(Input{Principal: d("5000000"), TermMonths: 5})
	require.NoError(t, err)
	assert.True(t, res.TotalRepayment.Equal(d("5000000")))
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"zero principal", Input{TermMonths: 10}, "principal"},
		{"negative fee", Input{Principal: d("1000"), ServiceFee: d("-1"), TermMonths: 10}, "serviceFee"},
		{"fee rivals principal", Input{Principal: d("1000"), ServiceFee: d("1000"), TermMonths: 10}, "serviceFee"},
		{"zero term", Input{Principal: d("1000")}, "termMonths"},
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
