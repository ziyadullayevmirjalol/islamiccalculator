package money

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

func TestParse(t *testing.T) {
	v, err := Parse("12500000.50")
	require.NoError(t, err)
	assert.True(t, v.Equal(d("12500000.5")))

	_, err = Parse("12,5")
	assert.Error(t, err)
	_, err = Parse("")
	assert.Error(t, err)
}

func TestRound2(t *testing.T) {
	cases := []struct{ in, want string }{
		{"1.005", "1.01"},  // half rounds away from zero
		{"1.004", "1.00"},
		{"-1.005", "-1.01"},
		{"333.6666", "333.67"},
		{"100", "100"},
	}
	for _, tc := range cases {
		assert.True(t, Round2(d(tc.in)).Equal(d(tc.want)), "Round2(%s) = %s, want %s", tc.in, Round2(d(tc.in)), tc.want)
	}
}

func TestSplit(t *testing.T) {
	cases := []struct {
		name  string
		total string
		n     int
		want  []string
	}{
		{"even split", "132000000", 12, []string{
			"11000000", "11000000", "11000000", "11000000", "11000000", "11000000",
			"11000000", "11000000", "11000000", "11000000", "11000000", "11000000"}},
		{"residual goes to final installment", "1001", 3, []string{"333.66", "333.66", "333.68"}},
		{"single installment", "500.55", 1, []string{"500.55"}},
		{"tiny total never yields negative part", "0.50", 100, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parts, err := Split(d(tc.total), tc.n)
			require.NoError(t, err)
			require.Len(t, parts, tc.n)

			sum := decimal.Zero
			for _, p := range parts {
				assert.False(t, p.IsNegative(), "part %s is negative", p)
				sum = sum.Add(p)
			}
			assert.True(t, sum.Equal(Round2(d(tc.total))), "parts sum to %s, want %s", sum, tc.total)

			if tc.want != nil {
				for i, w := range tc.want {
					assert.True(t, parts[i].Equal(d(w)), "part %d = %s, want %s", i, parts[i], w)
				}
			}
		})
	}

	t.Run("rejects invalid input", func(t *testing.T) {
		_, err := Split(d("100"), 0)
		assert.Error(t, err)
		_, err = Split(d("-1"), 2)
		assert.Error(t, err)
	})
}

func TestSplitProportional(t *testing.T) {
	amounts, err := Split(d("1001"), 3)
	require.NoError(t, err)

	// 1000 of the 1001 is principal → ratio 1000/1001.
	shares := SplitProportional(amounts, d("1000").Div(d("1001")), d("1000"))

	sum := decimal.Zero
	for _, s := range shares {
		sum = sum.Add(s)
	}
	assert.True(t, sum.Equal(d("1000")), "shares sum to %s, want 1000", sum)
	assert.True(t, shares[0].Equal(d("333.33")))
	assert.True(t, shares[2].Equal(d("333.34")))
}
