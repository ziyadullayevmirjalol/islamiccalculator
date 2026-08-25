package zakat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

func TestCalculateFitrah(t *testing.T) {
	p := FitrahParams{SaKg: d("2.5")}

	t.Run("family of five, two paid in food", func(t *testing.T) {
		res, err := CalculateFitrah(FitrahInput{
			People: 5, PeoplePaidInFood: 2, PricePerKg: d("12000"),
		}, p)
		require.NoError(t, err)

		assert.True(t, res.PerPerson.Equal(d("30000")), "2.5 kg × 12,000")
		assert.True(t, res.TotalDue.Equal(d("150000")))
		assert.True(t, res.FoodKg.Equal(d("5")), "2 people × 2.5 kg")
		assert.True(t, res.CashDue.Equal(d("90000")), "3 people in cash")
	})

	t.Run("sa override is honored", func(t *testing.T) {
		res, err := CalculateFitrah(FitrahInput{
			People: 1, PricePerKg: d("10000"), SaKgOverride: d("3"),
		}, p)
		require.NoError(t, err)
		assert.True(t, res.SaKg.Equal(d("3")))
		assert.True(t, res.TotalDue.Equal(d("30000")))
	})

	t.Run("no nisab: a single person always owes", func(t *testing.T) {
		res, err := CalculateFitrah(FitrahInput{People: 1, PricePerKg: d("1")}, p)
		require.NoError(t, err)
		assert.True(t, res.TotalDue.Equal(d("2.5")))
	})

	t.Run("validation", func(t *testing.T) {
		_, err := CalculateFitrah(FitrahInput{People: 0, PricePerKg: d("1")}, p)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "people")

		_, err = CalculateFitrah(FitrahInput{People: 2, PeoplePaidInFood: 3, PricePerKg: d("1")}, p)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "peoplePaidInFood")

		_, err = CalculateFitrah(FitrahInput{People: 1, PricePerKg: d("1"), SaKgOverride: d("9")}, p)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "saKg")
	})
}
