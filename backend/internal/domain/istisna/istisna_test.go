package istisna

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

func TestCalculate_Milestones(t *testing.T) {
	res, err := Calculate(Input{
		Mode:          ModeMilestones,
		ContractPrice: d("90000000"),
		Milestones: []Milestone{
			{Name: "foundation", Percent: d("30")},
			{Name: "structure", Percent: d("50")},
			{Name: "handover", Percent: d("20")},
		},
	})
	require.NoError(t, err)

	require.Len(t, res.Schedule, 3)
	assert.True(t, res.Schedule[0].Amount.Equal(d("27000000")))
	assert.True(t, res.Schedule[1].Amount.Equal(d("45000000")))
	assert.True(t, res.Schedule[2].Amount.Equal(d("18000000")))
	assert.Equal(t, "foundation", res.Schedule[0].Name)
}

func TestCalculate_MilestoneResidualGoesToFinalStage(t *testing.T) {
	res, err := Calculate(Input{
		Mode:          ModeMilestones,
		ContractPrice: d("100"),
		Milestones: []Milestone{
			{Percent: d("33.33")},
			{Percent: d("33.33")},
			{Percent: d("33.34")},
		},
	})
	require.NoError(t, err)

	sum := decimal.Zero
	for _, s := range res.Schedule {
		sum = sum.Add(s.Amount)
	}
	assert.True(t, sum.Equal(d("100")), "stages sum exactly to the contract price")
	assert.True(t, res.Schedule[2].Amount.Equal(d("33.34")))
}

func TestCalculate_EqualStages(t *testing.T) {
	res, err := Calculate(Input{
		Mode:          ModeEqual,
		ContractPrice: d("1001"),
		Stages:        3,
	})
	require.NoError(t, err)
	assert.True(t, res.Schedule[0].Amount.Equal(d("333.66")))
	assert.True(t, res.Schedule[2].Amount.Equal(d("333.68")), "final stage absorbs residual")
}

func TestCalculate_Validation(t *testing.T) {
	t.Run("percents not covering the price are rejected", func(t *testing.T) {
		_, err := Calculate(Input{
			Mode:          ModeMilestones,
			ContractPrice: d("1000"),
			Milestones:    []Milestone{{Percent: d("60")}, {Percent: d("30")}},
		})
		require.Error(t, err)
		e := apperr.From(err)
		assert.Equal(t, apperr.CodeValidation, e.Code)
		assert.Equal(t, "percents_must_sum_to_100", e.Fields["milestones"])
	})

	t.Run("over-collecting schedule rejected too", func(t *testing.T) {
		_, err := Calculate(Input{
			Mode:          ModeMilestones,
			ContractPrice: d("1000"),
			Milestones:    []Milestone{{Percent: d("60")}, {Percent: d("50")}},
		})
		require.Error(t, err)
		assert.Equal(t, "percents_must_sum_to_100", apperr.From(err).Fields["milestones"])
	})

	t.Run("zero-percent milestone rejected", func(t *testing.T) {
		_, err := Calculate(Input{
			Mode:          ModeMilestones,
			ContractPrice: d("1000"),
			Milestones:    []Milestone{{Percent: d("100")}, {Percent: d("0")}},
		})
		require.Error(t, err)
		assert.Equal(t, "percent_must_be_positive", apperr.From(err).Fields["milestones"])
	})

	t.Run("bad mode and missing stages", func(t *testing.T) {
		_, err := Calculate(Input{Mode: "phased", ContractPrice: d("1000")})
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "mode")

		_, err = Calculate(Input{Mode: ModeEqual, ContractPrice: d("1000")})
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "stages")
	})
}
