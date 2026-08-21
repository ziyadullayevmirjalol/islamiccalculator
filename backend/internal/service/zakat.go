package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type Zakat struct {
	settings   SettingsRepo
	metals     MetalPriceRepo
	rules      LivestockRuleRepo
	history    CalculationRepo
	staleAfter time.Duration
}

func NewZakat(settings SettingsRepo, metals MetalPriceRepo, rules LivestockRuleRepo, history CalculationRepo, staleAfter time.Duration) *Zakat {
	return &Zakat{settings: settings, metals: metals, rules: rules, history: history, staleAfter: staleAfter}
}

func (s *Zakat) record(ctx context.Context, calcType string, inputs, result any) {
	if err := s.history.Save(ctx, newRecord(ctx, calcType, inputs, result)); err != nil {
		slog.Warn("save calculation history", "calc_type", calcType, "err", err)
	}
}

// WealthOutput pairs the domain result with the market prices it was
// based on, so clients can show provenance (source, freshness).
type WealthOutput struct {
	Result zakat.WealthResult
	Gold   MetalPrice
	Silver MetalPrice
}

func (s *Zakat) Wealth(ctx context.Context, in zakat.WealthInput) (WealthOutput, error) {
	params, gold, silver, err := s.loadParams(ctx)
	if err != nil {
		return WealthOutput{}, err
	}

	res, err := zakat.CalculateWealth(in, params)
	if err != nil {
		return WealthOutput{}, err
	}

	out := WealthOutput{Result: res, Gold: gold, Silver: silver}
	s.record(ctx, "zakat.wealth", in, res)
	return out, nil
}

func (s *Zakat) Business(ctx context.Context, in zakat.BusinessInput) (zakat.BusinessResult, error) {
	params, _, _, err := s.loadParams(ctx)
	if err != nil {
		return zakat.BusinessResult{}, err
	}
	res, err := zakat.CalculateBusiness(in, params)
	if err != nil {
		return zakat.BusinessResult{}, err
	}
	s.record(ctx, "zakat.business", in, res)
	return res, nil
}

func (s *Zakat) Ushr(ctx context.Context, in zakat.UshrInput) (zakat.UshrResult, error) {
	natural, err := settingDecimal(ctx, s.settings, "ushr.natural_rate", "rate")
	if err != nil {
		return zakat.UshrResult{}, err
	}
	irrigated, err := settingDecimal(ctx, s.settings, "ushr.irrigated_rate", "rate")
	if err != nil {
		return zakat.UshrResult{}, err
	}
	res, err := zakat.CalculateUshr(in, zakat.UshrParams{NaturalRate: natural, IrrigatedRate: irrigated})
	if err != nil {
		return zakat.UshrResult{}, err
	}
	s.record(ctx, "zakat.ushr", in, res)
	return res, nil
}

func (s *Zakat) Livestock(ctx context.Context, in zakat.LivestockInput) (zakat.LivestockResult, error) {
	var rules []zakat.LivestockRule
	if in.Species != zakat.SpeciesSilkCocoons {
		var err error
		if rules, err = s.rules.BySpecies(ctx, in.Species); err != nil {
			return zakat.LivestockResult{}, apperr.Internal("livestock rules unavailable", err)
		}
	}
	cocoonRate, err := settingDecimal(ctx, s.settings, "zakat.rate", "rate")
	if err != nil {
		return zakat.LivestockResult{}, err
	}
	res, err := zakat.CalculateLivestock(in, rules, cocoonRate)
	if err != nil {
		return zakat.LivestockResult{}, err
	}
	s.record(ctx, "zakat.livestock", in, res)
	return res, nil
}

func (s *Zakat) Fidya(ctx context.Context, in zakat.FidyaInput) (zakat.FidyaResult, error) {
	var daily struct {
		Amount      string `json:"amount"`
		Currency    string `json:"currency"`
		NeedsReview bool   `json:"needs_review"`
	}
	if err := settingJSON(ctx, s.settings, "fidya.daily", &daily); err != nil {
		return zakat.FidyaResult{}, err
	}
	rate, err := decimal.NewFromString(daily.Amount)
	if err != nil {
		return zakat.FidyaResult{}, apperr.Internal("fidya rate malformed", err)
	}
	fastFeedings, err := settingInt(ctx, s.settings, "kaffarah.days", "days")
	if err != nil {
		return zakat.FidyaResult{}, err
	}
	oathFeedings, err := settingInt(ctx, s.settings, "kaffarah.oath_feedings", "count")
	if err != nil {
		return zakat.FidyaResult{}, err
	}

	res, err := zakat.CalculateFidya(in, zakat.FidyaParams{
		DailyRate:    rate,
		Currency:     daily.Currency,
		FastFeedings: fastFeedings,
		OathFeedings: oathFeedings,
		NeedsReview:  daily.NeedsReview,
	})
	if err != nil {
		return zakat.FidyaResult{}, err
	}
	s.record(ctx, "zakat.fidya", in, res)
	return res, nil
}

func (s *Zakat) Tazkiya(ctx context.Context, in zakat.TazkiyaInput) (zakat.TazkiyaResult, error) {
	res, err := zakat.CalculateTazkiya(in)
	if err != nil {
		return zakat.TazkiyaResult{}, err
	}
	s.record(ctx, "zakat.tazkiya", in, res)
	return res, nil
}

func (s *Zakat) loadParams(ctx context.Context) (zakat.Params, MetalPrice, MetalPrice, error) {
	var zero zakat.Params
	gold, err := s.metals.Latest(ctx, "gold")
	if err != nil {
		return zero, MetalPrice{}, MetalPrice{}, apperr.Internal("gold price unavailable", err)
	}
	silver, err := s.metals.Latest(ctx, "silver")
	if err != nil {
		return zero, MetalPrice{}, MetalPrice{}, apperr.Internal("silver price unavailable", err)
	}
	if gold.Currency != silver.Currency {
		return zero, MetalPrice{}, MetalPrice{}, apperr.Internal("metal prices in mixed currencies", nil)
	}
	markStale(&gold, s.staleAfter)
	markStale(&silver, s.staleAfter)

	nisabGold, err := settingDecimal(ctx, s.settings, "zakat.nisab_gold_grams", "grams")
	if err != nil {
		return zero, MetalPrice{}, MetalPrice{}, err
	}
	nisabSilver, err := settingDecimal(ctx, s.settings, "zakat.nisab_silver_grams", "grams")
	if err != nil {
		return zero, MetalPrice{}, MetalPrice{}, err
	}
	rate, err := settingDecimal(ctx, s.settings, "zakat.rate", "rate")
	if err != nil {
		return zero, MetalPrice{}, MetalPrice{}, err
	}

	return zakat.Params{
		GoldPricePerGram:   gold.PricePerGram,
		SilverPricePerGram: silver.PricePerGram,
		NisabGoldGrams:     nisabGold,
		NisabSilverGrams:   nisabSilver,
		Rate:               rate,
		Currency:           gold.Currency,
	}, gold, silver, nil
}
