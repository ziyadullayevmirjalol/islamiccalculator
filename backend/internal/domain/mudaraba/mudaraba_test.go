package mudaraba

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

func TestCalculate_Mudaraba(t *testing.T) {
	res, err := Calculate(Input{
		Mode:           ModeMudaraba,
		Amount:         d("10000000"),
		PoolRateAnnual: d("0.18"),
		ShareRatio:     d("0.60"),
		TermMonths:     12,
	})
	require.NoError(t, err)

	assert.True(t, res.EffectiveAnnualRate.Equal(d("0.108")))
	assert.True(t, res.ExpectedProfit.Equal(d("1080000")))
	assert.True(t, res.ExpectedTotal.Equal(d("11080000")))
	assert.True(t, res.ExpectedMonthlyProfit.Equal(d("90000")))
	assert.False(t, res.Guaranteed, "an Islamic deposit never promises profit")
}

func TestCalculate_Wakala(t *testing.T) {
	res, err := Calculate(Input{
		Mode:           ModeWakala,
		Amount:         d("10000000"),
		PoolRateAnnual: d("0.18"),
		WakalaFeeRate:  d("0.02"),
		TermMonths:     6,
	})
	require.NoError(t, err)

	assert.True(t, res.EffectiveAnnualRate.Equal(d("0.16")))
	assert.True(t, res.ExpectedProfit.Equal(d("800000")), "10M × 16%% × 6/12, got %s", res.ExpectedProfit)
	assert.True(t, res.ExpectedTotal.Equal(d("10800000")))
}

func TestCalculate_PartialYearProRata(t *testing.T) {
	res, err := Calculate(Input{
		Mode:           ModeMudaraba,
		Amount:         d("1000000"),
		PoolRateAnnual: d("0.12"),
		ShareRatio:     d("1"),
		TermMonths:     1,
	})
	require.NoError(t, err)
	assert.True(t, res.ExpectedProfit.Equal(d("10000")), "one month of 12%% annual = 1%%")
}

func TestCalculate_Validation(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		field string
	}{
		{"bad mode", Input{Mode: "deposit", Amount: d("1"), PoolRateAnnual: d("0.1"), TermMonths: 12}, "mode"},
		{"zero amount", Input{Mode: ModeMudaraba, PoolRateAnnual: d("0.1"), ShareRatio: d("0.5"), TermMonths: 12}, "amount"},
		{"pool rate 100%", Input{Mode: ModeMudaraba, Amount: d("1"), PoolRateAnnual: d("1"), ShareRatio: d("0.5"), TermMonths: 12}, "poolRateAnnual"},
		{"share above 1", Input{Mode: ModeMudaraba, Amount: d("1"), PoolRateAnnual: d("0.1"), ShareRatio: d("1.1"), TermMonths: 12}, "shareRatio"},
		{"zero share", Input{Mode: ModeMudaraba, Amount: d("1"), PoolRateAnnual: d("0.1"), TermMonths: 12}, "shareRatio"},
		{"wakala fee above pool", Input{Mode: ModeWakala, Amount: d("1"), PoolRateAnnual: d("0.1"), WakalaFeeRate: d("0.11"), TermMonths: 12}, "wakalaFeeRate"},
		{"term too long", Input{Mode: ModeWakala, Amount: d("1"), PoolRateAnnual: d("0.1"), TermMonths: 121}, "termMonths"},
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
