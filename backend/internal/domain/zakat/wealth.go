// Package zakat implements the zakat family of calculators. Fiqh
// parameters (nisab grams, rate) always arrive via Params — loaded from
// the app_settings table — never as constants in code.
package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	NisabBasisGold   = "gold"
	NisabBasisSilver = "silver"
)

// Params are the fiqh + market parameters for wealth zakat.
type Params struct {
	GoldPricePerGram   decimal.Decimal
	SilverPricePerGram decimal.Decimal
	NisabGoldGrams     decimal.Decimal
	NisabSilverGrams   decimal.Decimal
	Rate               decimal.Decimal // e.g. 0.025
	Currency           string
}

type WealthInput struct {
	GoldGrams    decimal.Decimal
	SilverGrams  decimal.Decimal
	Cash         decimal.Decimal // cash + bank balances
	OtherAssets  decimal.Decimal // trade goods, receivables expected back
	HawlComplete bool            // a full lunar year has passed on this wealth
}

type Nisab struct {
	GoldValue   decimal.Decimal
	SilverValue decimal.Decimal
	Applied     decimal.Decimal // the lower of the two — most protective of the poor
	Basis       string          // "gold" or "silver"
}

type WealthResult struct {
	GoldValue    decimal.Decimal
	SilverValue  decimal.Decimal
	TotalWealth  decimal.Decimal
	Nisab        Nisab
	AboveNisab   bool
	HawlComplete bool
	ZakatDue     decimal.Decimal // zero unless above nisab and hawl complete
	Currency     string
}

// NisabValues computes both nisab thresholds and selects the applied one.
func NisabValues(p Params) Nisab {
	gold := money.Round2(p.NisabGoldGrams.Mul(p.GoldPricePerGram))
	silver := money.Round2(p.NisabSilverGrams.Mul(p.SilverPricePerGram))
	n := Nisab{GoldValue: gold, SilverValue: silver, Applied: silver, Basis: NisabBasisSilver}
	if gold.LessThan(silver) {
		n.Applied, n.Basis = gold, NisabBasisGold
	}
	return n
}

// CalculateWealth computes zakat due on gold, silver, cash, and other
// zakatable assets.
func CalculateWealth(in WealthInput, p Params) (WealthResult, error) {
	if err := validateWealth(in, p); err != nil {
		return WealthResult{}, err
	}

	goldValue := money.Round2(in.GoldGrams.Mul(p.GoldPricePerGram))
	silverValue := money.Round2(in.SilverGrams.Mul(p.SilverPricePerGram))
	total := goldValue.Add(silverValue).Add(money.Round2(in.Cash)).Add(money.Round2(in.OtherAssets))

	nisab := NisabValues(p)
	above := total.GreaterThanOrEqual(nisab.Applied)

	due := decimal.Zero
	if above && in.HawlComplete {
		due = money.Round2(total.Mul(p.Rate))
	}

	return WealthResult{
		GoldValue:    goldValue,
		SilverValue:  silverValue,
		TotalWealth:  total,
		Nisab:        nisab,
		AboveNisab:   above,
		HawlComplete: in.HawlComplete,
		ZakatDue:     due,
		Currency:     p.Currency,
	}, nil
}

func validateWealth(in WealthInput, p Params) error {
	fields := map[string]string{}
	if in.GoldGrams.IsNegative() {
		fields["goldGrams"] = "must_not_be_negative"
	}
	if in.SilverGrams.IsNegative() {
		fields["silverGrams"] = "must_not_be_negative"
	}
	if in.Cash.IsNegative() {
		fields["cash"] = "must_not_be_negative"
	}
	if in.OtherAssets.IsNegative() {
		fields["otherAssets"] = "must_not_be_negative"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid zakat input", fields)
	}

	// Params come from our own reference data; failures here are server
	// misconfiguration, not client error.
	if !p.GoldPricePerGram.IsPositive() || !p.SilverPricePerGram.IsPositive() ||
		!p.NisabGoldGrams.IsPositive() || !p.NisabSilverGrams.IsPositive() ||
		!p.Rate.IsPositive() || p.Rate.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return apperr.Internal("invalid zakat parameters", nil)
	}
	return nil
}
