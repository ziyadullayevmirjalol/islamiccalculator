package zakat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

// --- business zakat --------------------------------------------------------

func TestCalculateBusiness(t *testing.T) {
	t.Run("net working capital above nisab", func(t *testing.T) {
		res, err := CalculateBusiness(BusinessInput{
			Cash:                 d("30000000"),
			Receivables:          d("20000000"),
			Inventory:            d("50000000"),
			ShortTermLiabilities: d("40000000"),
			HawlComplete:         true,
		}, params())
		require.NoError(t, err)

		assert.True(t, res.ZakatBase.Equal(d("60000000")))
		assert.True(t, res.AboveNisab)
		assert.True(t, res.ZakatDue.Equal(d("1500000")), "2.5%% of 60M, got %s", res.ZakatDue)
	})

	t.Run("liabilities exceeding assets floor the base at zero", func(t *testing.T) {
		res, err := CalculateBusiness(BusinessInput{
			Cash:                 d("1000000"),
			ShortTermLiabilities: d("5000000"),
			HawlComplete:         true,
		}, params())
		require.NoError(t, err)
		assert.True(t, res.ZakatBase.IsZero())
		assert.False(t, res.AboveNisab)
		assert.True(t, res.ZakatDue.IsZero())
	})

	t.Run("negative inventory rejected", func(t *testing.T) {
		_, err := CalculateBusiness(BusinessInput{Inventory: d("-1")}, params())
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "inventory")
	})
}

// --- ushr ------------------------------------------------------------------

func TestCalculateUshr(t *testing.T) {
	p := UshrParams{NaturalRate: d("0.10"), IrrigatedRate: d("0.05")}

	t.Run("natural water pays 10%", func(t *testing.T) {
		res, err := CalculateUshr(UshrInput{IrrigationType: IrrigationNatural, HarvestValue: d("20000000")}, p)
		require.NoError(t, err)
		assert.True(t, res.UshrDue.Equal(d("2000000")))
	})

	t.Run("irrigated land pays 5%", func(t *testing.T) {
		res, err := CalculateUshr(UshrInput{IrrigationType: IrrigationIrrigated, HarvestValue: d("20000000")}, p)
		require.NoError(t, err)
		assert.True(t, res.UshrDue.Equal(d("1000000")))
	})

	t.Run("unknown irrigation type rejected", func(t *testing.T) {
		_, err := CalculateUshr(UshrInput{IrrigationType: "drip", HarvestValue: d("100")}, p)
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "irrigationType")
	})
}

// --- livestock -------------------------------------------------------------

func intPtr(n int) *int { return &n }

func testRules() []LivestockRule {
	return []LivestockRule{
		{Species: SpeciesSheepGoats, MinCount: 40, MaxCount: intPtr(120), Due: []AnimalDue{{"sheep", 1}}},
		{Species: SpeciesSheepGoats, MinCount: 121, MaxCount: intPtr(200), Due: []AnimalDue{{"sheep", 2}}},
		{Species: SpeciesSheepGoats, MinCount: 201, MaxCount: intPtr(399), Due: []AnimalDue{{"sheep", 3}}},
		{Species: SpeciesSheepGoats, MinCount: 400, MaxCount: nil, Due: []AnimalDue{{"sheep", 4}}, PerExtraEvery: intPtr(100)},
		{Species: SpeciesCattle, MinCount: 30, MaxCount: intPtr(39), Due: []AnimalDue{{"tabi", 1}}},
		{Species: SpeciesCattle, MinCount: 40, MaxCount: intPtr(59), Due: []AnimalDue{{"musinna", 1}}},
		{Species: SpeciesCattle, MinCount: 70, MaxCount: intPtr(79), Due: []AnimalDue{{"tabi", 1}, {"musinna", 1}}},
		{Species: SpeciesCamels, MinCount: 5, MaxCount: intPtr(9), Due: []AnimalDue{{"sheep", 1}}},
		{Species: SpeciesCamels, MinCount: 25, MaxCount: intPtr(35), Due: []AnimalDue{{"bint_makhad", 1}}},
	}
}

func TestCalculateLivestock_TierBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		species string
		count   int
		want    []AnimalDue
		below   bool
	}{
		{"39 sheep below nisab", SpeciesSheepGoats, 39, nil, true},
		{"40 sheep owe one", SpeciesSheepGoats, 40, []AnimalDue{{"sheep", 1}}, false},
		{"120 sheep still owe one", SpeciesSheepGoats, 120, []AnimalDue{{"sheep", 1}}, false},
		{"121 sheep owe two", SpeciesSheepGoats, 121, []AnimalDue{{"sheep", 2}}, false},
		{"201 sheep owe three", SpeciesSheepGoats, 201, []AnimalDue{{"sheep", 3}}, false},
		{"400 sheep owe four", SpeciesSheepGoats, 400, []AnimalDue{{"sheep", 4}}, false},
		{"499 sheep still owe four", SpeciesSheepGoats, 499, []AnimalDue{{"sheep", 4}}, false},
		{"500 sheep owe five (open tier)", SpeciesSheepGoats, 500, []AnimalDue{{"sheep", 5}}, false},
		{"29 cattle below nisab", SpeciesCattle, 29, nil, true},
		{"30 cattle owe one tabi", SpeciesCattle, 30, []AnimalDue{{"tabi", 1}}, false},
		{"70 cattle owe tabi plus musinna", SpeciesCattle, 70, []AnimalDue{{"tabi", 1}, {"musinna", 1}}, false},
		{"4 camels below nisab", SpeciesCamels, 4, nil, true},
		{"5 camels owe a sheep", SpeciesCamels, 5, []AnimalDue{{"sheep", 1}}, false},
		{"25 camels owe bint makhad", SpeciesCamels, 25, []AnimalDue{{"bint_makhad", 1}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := CalculateLivestock(LivestockInput{Species: tc.species, Count: tc.count}, testRules(), d("0.025"))
			require.NoError(t, err)
			assert.Equal(t, tc.below, res.BelowNisab)
			if !tc.below {
				assert.Equal(t, tc.want, res.Due)
			} else {
				assert.Empty(t, res.Due)
			}
		})
	}
}

func TestCalculateLivestock_CattleCombination(t *testing.T) {
	cases := []struct {
		count int
		want  []AnimalDue
	}{
		{90, []AnimalDue{{"tabi", 3}}},
		{100, []AnimalDue{{"tabi", 2}, {"musinna", 1}}},
		{110, []AnimalDue{{"tabi", 1}, {"musinna", 2}}},
		{120, []AnimalDue{{"musinna", 3}}}, // exact tie prefers musinna
		{125, []AnimalDue{{"musinna", 3}}}, // remainder below 30 exempt
		{130, []AnimalDue{{"tabi", 3}, {"musinna", 1}}},
	}
	for _, tc := range cases {
		res, err := CalculateLivestock(LivestockInput{Species: SpeciesCattle, Count: tc.count}, testRules(), d("0.025"))
		require.NoError(t, err)
		assert.Equal(t, tc.want, res.Due, "cattle count %d", tc.count)
		assert.Equal(t, "computed_by_combination_rule", res.Note)
	}
}

func TestCalculateLivestock_SilkCocoons(t *testing.T) {
	res, err := CalculateLivestock(LivestockInput{Species: SpeciesSilkCocoons, MarketValue: d("8000000")}, nil, d("0.025"))
	require.NoError(t, err)
	assert.True(t, res.CashDue.Equal(d("200000")))
	assert.Empty(t, res.Due)
}

func TestCalculateLivestock_Validation(t *testing.T) {
	_, err := CalculateLivestock(LivestockInput{Species: "horses", Count: 10}, testRules(), d("0.025"))
	require.Error(t, err)
	assert.Contains(t, apperr.From(err).Fields, "species")

	_, err = CalculateLivestock(LivestockInput{Species: SpeciesCattle}, testRules(), d("0.025"))
	require.Error(t, err)
	assert.Contains(t, apperr.From(err).Fields, "count")
}

// --- fidya / kaffarah ------------------------------------------------------

func fidyaParams() FidyaParams {
	return FidyaParams{DailyRate: d("15000"), Currency: "UZS", FastFeedings: 60, OathFeedings: 10, NeedsReview: true}
}

func TestCalculateFidya(t *testing.T) {
	cases := []struct {
		name     string
		in       FidyaInput
		feedings int
		want     string
	}{
		{"fidya for 10 missed days", FidyaInput{Kind: KindFidya, Count: 10}, 1, "150000"},
		{"kaffarah for one broken fast", FidyaInput{Kind: KindKaffarahFast, Count: 1}, 60, "900000"},
		{"kaffarah for two broken oaths", FidyaInput{Kind: KindKaffarahOath, Count: 2}, 10, "300000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := CalculateFidya(tc.in, fidyaParams())
			require.NoError(t, err)
			assert.Equal(t, tc.feedings, res.Feedings)
			assert.True(t, res.TotalDue.Equal(d(tc.want)), "due = %s, want %s", res.TotalDue, tc.want)
			assert.True(t, res.NeedsReview, "seed rate must carry the review flag")
		})
	}

	t.Run("unknown kind rejected", func(t *testing.T) {
		_, err := CalculateFidya(FidyaInput{Kind: "sadaqa", Count: 1}, fidyaParams())
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "kind")
	})
}

// --- tazkiya ---------------------------------------------------------------

func TestCalculateTazkiya(t *testing.T) {
	t.Run("declared impure amount", func(t *testing.T) {
		res, err := CalculateTazkiya(TazkiyaInput{
			Mode:         TazkiyaDeclared,
			TotalIncome:  d("10000000"),
			ImpureAmount: d("350000"),
		})
		require.NoError(t, err)
		assert.True(t, res.PurgeAmount.Equal(d("350000")))
		assert.True(t, res.CleanAmount.Equal(d("9650000")))
	})

	t.Run("dividend purged by company ratio", func(t *testing.T) {
		res, err := CalculateTazkiya(TazkiyaInput{
			Mode:           TazkiyaDividend,
			DividendAmount: d("2000000"),
			ImpureRatio:    d("0.03"),
		})
		require.NoError(t, err)
		assert.True(t, res.PurgeAmount.Equal(d("60000")))
		assert.True(t, res.CleanAmount.Equal(d("1940000")))
	})

	t.Run("impure exceeding total rejected", func(t *testing.T) {
		_, err := CalculateTazkiya(TazkiyaInput{
			Mode:         TazkiyaDeclared,
			TotalIncome:  d("100"),
			ImpureAmount: d("101"),
		})
		require.Error(t, err)
		assert.Contains(t, apperr.From(err).Fields, "impureAmount")
	})
}
