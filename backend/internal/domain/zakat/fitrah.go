package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

// FitrahParams holds the sa' measure (a volume unit, ~2.0–3.0 kg of
// staple depending on the food; 2.5 kg is the common default). It lives
// in app_settings — never in code.
type FitrahParams struct {
	SaKg decimal.Decimal
}

type FitrahInput struct {
	People           int             // everyone covered, including infants and dependants
	PeoplePaidInFood int             // how many are paid with the staple itself
	PricePerKg       decimal.Decimal // local price of the chosen staple
	SaKgOverride     decimal.Decimal // optional; zero uses the configured default
}

type FitrahResult struct {
	People           int
	PeoplePaidInFood int
	SaKg             decimal.Decimal
	PricePerKg       decimal.Decimal
	PerPerson        decimal.Decimal // value of one sa' at the staple price
	TotalDue         decimal.Decimal
	FoodKg           decimal.Decimal // staple to hand over for people paid in food
	CashDue          decimal.Decimal // cash for the rest
}

// CalculateFitrah computes Zakat al-Fitr. Unlike Zakat al-Mal there is
// NO nisab: it is due from every Muslim with surplus food on Eid day,
// and must reach recipients before the Eid prayer.
func CalculateFitrah(in FitrahInput, p FitrahParams) (FitrahResult, error) {
	fields := map[string]string{}
	if in.People < 1 {
		fields["people"] = "must_be_positive"
	}
	if in.PeoplePaidInFood < 0 || in.PeoplePaidInFood > in.People {
		fields["peoplePaidInFood"] = "exceeds_people_covered"
	}
	if !in.PricePerKg.IsPositive() {
		fields["pricePerKg"] = "must_be_positive"
	}
	saKg := p.SaKg
	if !in.SaKgOverride.IsZero() {
		// Scholarly estimates run 2.0–3.0 kg; accept a sane superset.
		if in.SaKgOverride.LessThan(decimal.NewFromInt(1)) || in.SaKgOverride.GreaterThan(decimal.NewFromInt(5)) {
			fields["saKg"] = "out_of_range"
		}
		saKg = in.SaKgOverride
	}
	if len(fields) > 0 {
		return FitrahResult{}, apperr.Validation("invalid fitrah input", fields)
	}
	if !saKg.IsPositive() {
		return FitrahResult{}, apperr.Internal("invalid fitrah parameters", nil)
	}

	perPerson := money.Round2(saKg.Mul(in.PricePerKg))
	cashPeople := decimal.NewFromInt(int64(in.People - in.PeoplePaidInFood))

	return FitrahResult{
		People:           in.People,
		PeoplePaidInFood: in.PeoplePaidInFood,
		SaKg:             saKg,
		PricePerKg:       money.Round2(in.PricePerKg),
		PerPerson:        perPerson,
		TotalDue:         perPerson.Mul(decimal.NewFromInt(int64(in.People))),
		FoodKg:           saKg.Mul(decimal.NewFromInt(int64(in.PeoplePaidInFood))),
		CashDue:          perPerson.Mul(cashPeople),
	}, nil
}
