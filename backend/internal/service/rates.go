package service

import (
	"context"
	"time"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type Rates struct {
	settings   SettingsRepo
	metals     MetalPriceRepo
	staleAfter time.Duration
}

func NewRates(settings SettingsRepo, metals MetalPriceRepo, staleAfter time.Duration) *Rates {
	return &Rates{settings: settings, metals: metals, staleAfter: staleAfter}
}

type MetalsOutput struct {
	Gold     MetalPrice
	Silver   MetalPrice
	Nisab    zakat.Nisab
	Currency string
}

// Metals returns current spot prices with the nisab thresholds they imply.
func (s *Rates) Metals(ctx context.Context) (MetalsOutput, error) {
	gold, err := s.metals.Latest(ctx, "gold")
	if err != nil {
		return MetalsOutput{}, apperr.Internal("gold price unavailable", err)
	}
	silver, err := s.metals.Latest(ctx, "silver")
	if err != nil {
		return MetalsOutput{}, apperr.Internal("silver price unavailable", err)
	}
	markStale(&gold, s.staleAfter)
	markStale(&silver, s.staleAfter)

	nisabGold, err := settingDecimal(ctx, s.settings, "zakat.nisab_gold_grams", "grams")
	if err != nil {
		return MetalsOutput{}, err
	}
	nisabSilver, err := settingDecimal(ctx, s.settings, "zakat.nisab_silver_grams", "grams")
	if err != nil {
		return MetalsOutput{}, err
	}

	nisab := zakat.NisabValues(zakat.Params{
		GoldPricePerGram:   gold.PricePerGram,
		SilverPricePerGram: silver.PricePerGram,
		NisabGoldGrams:     nisabGold,
		NisabSilverGrams:   nisabSilver,
	})

	return MetalsOutput{Gold: gold, Silver: silver, Nisab: nisab, Currency: gold.Currency}, nil
}
