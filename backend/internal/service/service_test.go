package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/diyorbek/islamiccalculator/internal/domain/ijara"
	"github.com/diyorbek/islamiccalculator/internal/domain/istisna"
	"github.com/diyorbek/islamiccalculator/internal/domain/latepayment"
	"github.com/diyorbek/islamiccalculator/internal/domain/mudaraba"
	"github.com/diyorbek/islamiccalculator/internal/domain/murabaha"
	"github.com/diyorbek/islamiccalculator/internal/domain/musharaka"
	"github.com/diyorbek/islamiccalculator/internal/domain/qardhasan"
	"github.com/diyorbek/islamiccalculator/internal/domain/salam"
	"github.com/diyorbek/islamiccalculator/internal/domain/screener"
	"github.com/diyorbek/islamiccalculator/internal/domain/sukuk"
	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

// --- fakes -----------------------------------------------------------------

type fakeSettings map[string]string

func (f fakeSettings) Value(_ context.Context, key string) (json.RawMessage, error) {
	v, ok := f[key]
	if !ok {
		return nil, ErrNotFound
	}
	return json.RawMessage(v), nil
}

type fakeMetals map[string]MetalPrice

func (f fakeMetals) Latest(_ context.Context, metal string) (MetalPrice, error) {
	p, ok := f[metal]
	if !ok {
		return MetalPrice{}, ErrNotFound
	}
	return p, nil
}

type fakeHistory struct {
	saved []CalculationRecord
	err   error
}

func (f *fakeHistory) Save(_ context.Context, rec CalculationRecord) error {
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, rec)
	return nil
}

type fakeRules []zakat.LivestockRule

func (f fakeRules) BySpecies(_ context.Context, species string) ([]zakat.LivestockRule, error) {
	var out []zakat.LivestockRule
	for _, r := range f {
		if r.Species == species {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f fakeRules) All(_ context.Context) ([]zakat.LivestockRule, error) { return f, nil }

func intPtr(n int) *int { return &n }

func seededRules() fakeRules {
	return fakeRules{
		{Species: zakat.SpeciesSheepGoats, MinCount: 40, MaxCount: intPtr(120), Due: []zakat.AnimalDue{{Animal: "sheep", Count: 1}}},
		{Species: zakat.SpeciesCattle, MinCount: 30, MaxCount: intPtr(39), Due: []zakat.AnimalDue{{Animal: "tabi", Count: 1}}},
	}
}

func seededSettings() fakeSettings {
	return fakeSettings{
		"zakat.rate":               `{"rate": "0.025"}`,
		"zakat.nisab_gold_grams":   `{"grams": "87.48"}`,
		"zakat.nisab_silver_grams": `{"grams": "612.36"}`,
		"ushr.natural_rate":        `{"rate": "0.10"}`,
		"ushr.irrigated_rate":      `{"rate": "0.05"}`,
		"fidya.daily":              `{"amount": "15000", "currency": "UZS", "needs_review": true}`,
		"kaffarah.days":            `{"days": 60}`,
		"kaffarah.oath_feedings":   `{"count": 10}`,
	}
}

func seededMetals() fakeMetals {
	now := time.Now()
	return fakeMetals{
		"gold":   {Metal: "gold", PricePerGram: d("1450000"), Currency: "UZS", Source: "seed", FetchedAt: now},
		"silver": {Metal: "silver", PricePerGram: d("17500"), Currency: "UZS", Source: "seed", FetchedAt: now},
	}
}

// --- finance workflows -----------------------------------------------------

func TestFinanceService_Workflows(t *testing.T) {
	t.Run("murabaha calculates and records history", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewFinance(history)

		res, err := svc.Murabaha(context.Background(), murabaha.Input{
			Cost:       d("120000000"),
			Markup:     murabaha.Markup{Mode: murabaha.MarkupModeRate, Value: d("0.10")},
			TermMonths: 12,
		})
		require.NoError(t, err)
		assert.True(t, res.MonthlyInstallment.Equal(d("11000000")))

		require.Len(t, history.saved, 1)
		assert.Equal(t, "finance.murabaha", history.saved[0].CalcType)
	})

	t.Run("ijara records under its own calc type", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewFinance(history)

		res, err := svc.Ijara(context.Background(), ijara.Input{
			Mode:          ijara.ModeProfit,
			AssetCost:     d("100000000"),
			Profit:        ijara.Profit{Mode: ijara.ProfitModeRate, Value: d("0.20")},
			TransferPrice: d("10000000"),
			TermMonths:    24,
		})
		require.NoError(t, err)
		assert.True(t, res.ProfitTotal.Equal(d("20000000")))
		require.Len(t, history.saved, 1)
		assert.Equal(t, "finance.ijara", history.saved[0].CalcType)
	})

	t.Run("qard al-hasan records under its own calc type", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewFinance(history)

		res, err := svc.QardHasan(context.Background(), qardhasan.Input{
			Principal:  d("10000000"),
			ServiceFee: d("100000"),
			TermMonths: 10,
		})
		require.NoError(t, err)
		assert.True(t, res.TotalRepayment.Equal(d("10100000")))
		require.Len(t, history.saved, 1)
		assert.Equal(t, "finance.qard_hasan", history.saved[0].CalcType)
	})

	t.Run("mudaraba records under its own calc type", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewFinance(history)

		res, err := svc.Mudaraba(context.Background(), mudaraba.Input{
			Mode:           mudaraba.ModeMudaraba,
			Amount:         d("10000000"),
			PoolRateAnnual: d("0.18"),
			ShareRatio:     d("0.60"),
			TermMonths:     12,
		})
		require.NoError(t, err)
		assert.True(t, res.ExpectedProfit.Equal(d("1080000")))
		require.Len(t, history.saved, 1)
		assert.Equal(t, "finance.mudaraba", history.saved[0].CalcType)
	})

	t.Run("validation error stops the workflow before persistence", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewFinance(history)

		_, err := svc.Murabaha(context.Background(), murabaha.Input{TermMonths: 12})
		require.Error(t, err)
		assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
		assert.Empty(t, history.saved)
	})

	t.Run("history failure does not fail the calculation", func(t *testing.T) {
		svc := NewFinance(&fakeHistory{err: errors.New("db down")})

		res, err := svc.Murabaha(context.Background(), murabaha.Input{
			Cost:       d("1000"),
			Markup:     murabaha.Markup{Mode: murabaha.MarkupModeAmount, Value: d("1")},
			TermMonths: 3,
		})
		require.NoError(t, err)
		assert.True(t, res.SalePrice.Equal(d("1001")))
	})
}

// --- zakat wealth workflow -------------------------------------------------

func TestZakatService_WealthWorkflow(t *testing.T) {
	t.Run("loads params from reference data and calculates", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewZakat(seededSettings(), seededMetals(), seededRules(), history, 48*time.Hour)

		out, err := svc.Wealth(context.Background(), zakat.WealthInput{Cash: d("50000000"), HawlComplete: true})
		require.NoError(t, err)

		assert.True(t, out.Result.ZakatDue.Equal(d("1250000")))
		assert.Equal(t, "UZS", out.Result.Currency)
		assert.Equal(t, "seed", out.Gold.Source, "price provenance is surfaced")
		require.Len(t, history.saved, 1)
		assert.Equal(t, "zakat.wealth", history.saved[0].CalcType)
	})

	t.Run("missing metal price is an internal error", func(t *testing.T) {
		metals := seededMetals()
		delete(metals, "silver")
		svc := NewZakat(seededSettings(), metals, seededRules(), &fakeHistory{}, 48*time.Hour)

		_, err := svc.Wealth(context.Background(), zakat.WealthInput{Cash: d("100")})
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	})

	t.Run("missing setting is an internal error", func(t *testing.T) {
		settings := seededSettings()
		delete(settings, "zakat.rate")
		svc := NewZakat(settings, seededMetals(), seededRules(), &fakeHistory{}, 48*time.Hour)

		_, err := svc.Wealth(context.Background(), zakat.WealthInput{Cash: d("100")})
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	})

	t.Run("mixed price currencies are rejected", func(t *testing.T) {
		metals := seededMetals()
		p := metals["silver"]
		p.Currency = "USD"
		metals["silver"] = p
		svc := NewZakat(seededSettings(), metals, seededRules(), &fakeHistory{}, 48*time.Hour)

		_, err := svc.Wealth(context.Background(), zakat.WealthInput{Cash: d("100")})
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	})
}

// --- zakat family workflows ------------------------------------------------

func TestZakatService_FamilyWorkflows(t *testing.T) {
	newSvc := func(history *fakeHistory) *Zakat {
		return NewZakat(seededSettings(), seededMetals(), seededRules(), history, 48*time.Hour)
	}

	t.Run("business zakat", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := newSvc(history).Business(context.Background(), zakat.BusinessInput{
			Cash: d("30000000"), Receivables: d("20000000"), Inventory: d("50000000"),
			ShortTermLiabilities: d("40000000"), HawlComplete: true,
		})
		require.NoError(t, err)
		assert.True(t, res.ZakatDue.Equal(d("1500000")))
		require.Len(t, history.saved, 1)
		assert.Equal(t, "zakat.business", history.saved[0].CalcType)
	})

	t.Run("ushr rates come from settings", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := newSvc(history).Ushr(context.Background(), zakat.UshrInput{
			IrrigationType: zakat.IrrigationIrrigated, HarvestValue: d("20000000"),
		})
		require.NoError(t, err)
		assert.True(t, res.UshrDue.Equal(d("1000000")))
		assert.Equal(t, "zakat.ushr", history.saved[0].CalcType)
	})

	t.Run("livestock rules come from the repo", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := newSvc(history).Livestock(context.Background(), zakat.LivestockInput{
			Species: zakat.SpeciesSheepGoats, Count: 50,
		})
		require.NoError(t, err)
		assert.Equal(t, []zakat.AnimalDue{{Animal: "sheep", Count: 1}}, res.Due)
		assert.Equal(t, "zakat.livestock", history.saved[0].CalcType)
	})

	t.Run("fidya rates come from settings", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := newSvc(history).Fidya(context.Background(), zakat.FidyaInput{
			Kind: zakat.KindKaffarahFast, Count: 1,
		})
		require.NoError(t, err)
		assert.True(t, res.TotalDue.Equal(d("900000")), "60 feedings × 15000")
		assert.True(t, res.NeedsReview)
		assert.Equal(t, "zakat.fidya", history.saved[0].CalcType)
	})

	t.Run("tazkiya", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := newSvc(history).Tazkiya(context.Background(), zakat.TazkiyaInput{
			Mode: zakat.TazkiyaDeclared, TotalIncome: d("10000000"), ImpureAmount: d("350000"),
		})
		require.NoError(t, err)
		assert.True(t, res.CleanAmount.Equal(d("9650000")))
		assert.Equal(t, "zakat.tazkiya", history.saved[0].CalcType)
	})
}

func TestFinanceService_CorporateWorkflows(t *testing.T) {
	t.Run("salam", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := NewFinance(history).Salam(context.Background(), salam.Input{
			Quantity: d("100"), UnitPrice: d("2400000"), ExpectedUnitPrice: d("3000000"),
		})
		require.NoError(t, err)
		assert.True(t, res.ExpectedMargin.Equal(d("60000000")))
		assert.Equal(t, "finance.salam", history.saved[0].CalcType)
	})

	t.Run("istisna", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := NewFinance(history).Istisna(context.Background(), istisna.Input{
			Mode: istisna.ModeEqual, ContractPrice: d("90000000"), Stages: 3,
		})
		require.NoError(t, err)
		assert.True(t, res.Schedule[0].Amount.Equal(d("30000000")))
		assert.Equal(t, "finance.istisna", history.saved[0].CalcType)
	})

	t.Run("musharaka", func(t *testing.T) {
		history := &fakeHistory{}
		res, err := NewFinance(history).Musharaka(context.Background(), musharaka.Input{
			Partners: []musharaka.Partner{
				{Capital: d("70"), ProfitSharePercent: d("50")},
				{Capital: d("30"), ProfitSharePercent: d("50")},
			},
			ResultType: musharaka.ResultLoss,
			Amount:     d("100"),
		})
		require.NoError(t, err)
		assert.True(t, res.Shares[0].Amount.Equal(d("70")), "loss by capital")
		assert.Equal(t, "finance.musharaka", history.saved[0].CalcType)
	})
}

func TestFinanceService_LatePayment(t *testing.T) {
	history := &fakeHistory{}
	svc := NewFinance(history)

	res, err := svc.LatePayment(context.Background(), latepayment.Input{
		Mode: latepayment.ModeRate, OverdueAmount: d("10000000"), DaysLate: 73, AnnualRate: d("0.10"),
	})
	require.NoError(t, err)
	assert.True(t, res.CharityDue.Equal(d("200000")))
	assert.Equal(t, latepayment.DispositionCharity, res.Disposition)
	assert.Equal(t, "finance.late_payment", history.saved[0].CalcType)
}

// --- invest workflows ------------------------------------------------------

type fakeScreenerRules []ScreenerRule

func (f fakeScreenerRules) All(context.Context) ([]ScreenerRule, error) { return f, nil }

func seededScreenerRules() fakeScreenerRules {
	return fakeScreenerRules{
		{Key: screener.CheckDebt, Threshold: d("0.30")},
		{Key: screener.CheckInvest, Threshold: d("0.30")},
		{Key: screener.CheckImpure, Threshold: d("0.05")},
	}
}

func TestInvestService_Workflows(t *testing.T) {
	t.Run("screener thresholds come from the repo", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewInvest(seededScreenerRules(), history)

		res, err := svc.Screen(context.Background(), screener.Input{
			InterestBearingDebt:        d("200000000"),
			InterestBearingInvestments: d("100000000"),
			MarketCap:                  d("1000000000"),
			ImpureIncome:               d("4000000"),
			TotalRevenue:               d("200000000"),
		})
		require.NoError(t, err)
		assert.Equal(t, screener.VerdictCompliant, res.Verdict)
		assert.Equal(t, "invest.screener", history.saved[0].CalcType)
	})

	t.Run("missing screener rule is an internal error", func(t *testing.T) {
		svc := NewInvest(seededScreenerRules()[:2], &fakeHistory{})
		_, err := svc.Screen(context.Background(), screener.Input{
			MarketCap: d("100"), TotalRevenue: d("100"),
		})
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	})

	t.Run("sukuk records history", func(t *testing.T) {
		history := &fakeHistory{}
		svc := NewInvest(seededScreenerRules(), history)

		res, err := svc.Sukuk(context.Background(), []sukuk.Position{{
			FaceValue: d("100000000"), PurchasePrice: d("95000000"),
			DistributionRateAnnual: d("0.09"), Frequency: 2, TermMonths: 60,
		}})
		require.NoError(t, err)
		assert.True(t, res.TotalExpectedGain.Equal(d("50000000")))
		assert.Equal(t, "invest.sukuk", history.saved[0].CalcType)
	})
}

func TestReferenceService_LivestockRules(t *testing.T) {
	svc := NewReference(seededRules())
	rules, err := svc.LivestockRules(context.Background())
	require.NoError(t, err)
	assert.Len(t, rules, 2)
}

// --- rates workflow --------------------------------------------------------

func TestRatesService_Metals(t *testing.T) {
	svc := NewRates(seededSettings(), seededMetals(), 48*time.Hour)

	out, err := svc.Metals(context.Background())
	require.NoError(t, err)

	assert.True(t, out.Gold.PricePerGram.Equal(d("1450000")))
	assert.True(t, out.Nisab.SilverValue.Equal(d("10716300")))
	assert.Equal(t, zakat.NisabBasisSilver, out.Nisab.Basis)
	assert.Equal(t, "UZS", out.Currency)
}
