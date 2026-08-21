package zakat

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

// Hanafi defaults with the seeded UZS prices.
func params() Params {
	return Params{
		GoldPricePerGram:   d("1450000"),
		SilverPricePerGram: d("17500"),
		NisabGoldGrams:     d("87.48"),
		NisabSilverGrams:   d("612.36"),
		Rate:               d("0.025"),
		Currency:           "UZS",
	}
}

func TestNisabValues(t *testing.T) {
	n := NisabValues(params())
	assert.True(t, n.GoldValue.Equal(d("126846000")))
	assert.True(t, n.SilverValue.Equal(d("10716300")))
	assert.True(t, n.Applied.Equal(d("10716300")), "the lower threshold applies")
	assert.Equal(t, NisabBasisSilver, n.Basis)
}

func TestCalculateWealth(t *testing.T) {
	cases := []struct {
		name       string
		in         WealthInput
		wantDue    string
		wantAbove  bool
		wantTotal  string
	}{
		{"cash above nisab with hawl",
			WealthInput{Cash: d("50000000"), HawlComplete: true},
			"1250000", true, "50000000"},
		{"exactly at nisab is zakatable",
			WealthInput{Cash: d("10716300"), HawlComplete: true},
			"267907.50", true, "10716300"},
		{"below nisab owes nothing",
			WealthInput{Cash: d("10716299.99"), HawlComplete: true},
			"0", false, "10716299.99"},
		{"above nisab but hawl incomplete owes nothing yet",
			WealthInput{Cash: d("50000000"), HawlComplete: false},
			"0", true, "50000000"},
		{"gold and silver valued at spot",
			WealthInput{GoldGrams: d("100"), SilverGrams: d("1000"), Cash: d("500000"), HawlComplete: true},
			// 100×1.45M + 1000×17.5k + 500k = 163,000,000 → 2.5% = 4,075,000
			"4075000", true, "163000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := CalculateWealth(tc.in, params())
			require.NoError(t, err)
			assert.True(t, res.TotalWealth.Equal(d(tc.wantTotal)), "total = %s, want %s", res.TotalWealth, tc.wantTotal)
			assert.Equal(t, tc.wantAbove, res.AboveNisab)
			assert.True(t, res.ZakatDue.Equal(d(tc.wantDue)), "due = %s, want %s", res.ZakatDue, tc.wantDue)
			assert.Equal(t, "UZS", res.Currency)
		})
	}
}

func TestCalculateWealth_Validation(t *testing.T) {
	t.Run("negative amounts rejected per field", func(t *testing.T) {
		_, err := CalculateWealth(WealthInput{Cash: d("-1"), GoldGrams: d("-2")}, params())
		require.Error(t, err)
		e := apperr.From(err)
		assert.Equal(t, apperr.CodeValidation, e.Code)
		assert.Contains(t, e.Fields, "cash")
		assert.Contains(t, e.Fields, "goldGrams")
	})

	t.Run("broken reference params are an internal error", func(t *testing.T) {
		p := params()
		p.Rate = d("0")
		_, err := CalculateWealth(WealthInput{Cash: d("100")}, p)
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	})
}
