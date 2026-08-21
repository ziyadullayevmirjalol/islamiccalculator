package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	SpeciesSheepGoats  = "sheep_goats"
	SpeciesCattle      = "cattle"
	SpeciesCamels      = "camels"
	SpeciesSilkCocoons = "silk_cocoons"

	// cattleComboFrom is where the tier table hands over to the
	// per-30/per-40 combination rule.
	cattleComboFrom = 90
)

// AnimalDue is one line of what is owed: e.g. {"sheep", 2} or
// {"musinna", 1} (a two-year-old cow).
type AnimalDue struct {
	Animal string `json:"animal"`
	Count  int    `json:"count"`
}

// LivestockRule is a tier from the livestock_zakat_rules table.
type LivestockRule struct {
	Species       string
	MinCount      int
	MaxCount      *int // nil = open-ended
	Due           []AnimalDue
	PerExtraEvery *int // one more of Due[0].Animal per this many head above MinCount
	Note          string
}

type LivestockInput struct {
	Species     string
	Count       int             // head count (animal species)
	MarketValue decimal.Decimal // silk_cocoons only
}

type LivestockResult struct {
	Species     string
	Count       int
	Due         []AnimalDue     // empty when below nisab
	BelowNisab  bool
	CashDue     decimal.Decimal // silk_cocoons only
	Rate        decimal.Decimal // silk_cocoons only
	Note        string
}

// CalculateLivestock resolves the zakat due on herd animals from the rule
// tiers, or on silk cocoons as saleable produce at cocoonRate.
func CalculateLivestock(in LivestockInput, rules []LivestockRule, cocoonRate decimal.Decimal) (LivestockResult, error) {
	switch in.Species {
	case SpeciesSilkCocoons:
		return cocoonZakat(in, cocoonRate)
	case SpeciesSheepGoats, SpeciesCattle, SpeciesCamels:
	default:
		return LivestockResult{}, apperr.Validation("invalid livestock input",
			map[string]string{"species": "unknown_species"})
	}
	if in.Count < 1 {
		return LivestockResult{}, apperr.Validation("invalid livestock input",
			map[string]string{"count": "must_be_positive"})
	}

	if in.Species == SpeciesCattle && in.Count >= cattleComboFrom {
		return LivestockResult{
			Species: in.Species,
			Count:   in.Count,
			Due:     cattleCombination(in.Count),
			Note:    "computed_by_combination_rule",
		}, nil
	}

	rule, ok := matchRule(rules, in.Species, in.Count)
	if !ok {
		return LivestockResult{
			Species:    in.Species,
			Count:      in.Count,
			Due:        []AnimalDue{},
			BelowNisab: true,
		}, nil
	}

	due := make([]AnimalDue, len(rule.Due))
	copy(due, rule.Due)
	if rule.PerExtraEvery != nil && len(due) > 0 {
		due[0].Count += (in.Count - rule.MinCount) / *rule.PerExtraEvery
	}

	return LivestockResult{
		Species: in.Species,
		Count:   in.Count,
		Due:     due,
		Note:    rule.Note,
	}, nil
}

func cocoonZakat(in LivestockInput, rate decimal.Decimal) (LivestockResult, error) {
	if !in.MarketValue.IsPositive() {
		return LivestockResult{}, apperr.Validation("invalid livestock input",
			map[string]string{"marketValue": "must_be_positive"})
	}
	if !rate.IsPositive() || rate.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return LivestockResult{}, apperr.Internal("invalid cocoon zakat rate", nil)
	}
	value := money.Round2(in.MarketValue)
	return LivestockResult{
		Species: in.Species,
		Due:     []AnimalDue{},
		CashDue: money.Round2(value.Mul(rate)),
		Rate:    rate,
		Note:    "cocoons_zakat_as_saleable_produce",
	}, nil
}

func matchRule(rules []LivestockRule, species string, count int) (LivestockRule, bool) {
	for _, r := range rules {
		if r.Species != species || count < r.MinCount {
			continue
		}
		if r.MaxCount == nil || count <= *r.MaxCount {
			return r, true
		}
	}
	return LivestockRule{}, false
}

// cattleCombination applies the Hanafi rule for 90+ cattle: one tabi'
// (yearling) per 30 head and one musinna (two-year-old) per 40 head,
// choosing the combination that covers the most head; ties prefer
// musinna. The remainder below a complete 30 is exempt.
func cattleCombination(n int) []AnimalDue {
	bestTabi, bestMusinna, bestCovered := 0, 0, -1
	for musinna := 0; 40*musinna <= n; musinna++ {
		tabi := (n - 40*musinna) / 30
		covered := 30*tabi + 40*musinna
		if covered > bestCovered || (covered == bestCovered && musinna > bestMusinna) {
			bestTabi, bestMusinna, bestCovered = tabi, musinna, covered
		}
	}
	due := []AnimalDue{}
	if bestTabi > 0 {
		due = append(due, AnimalDue{Animal: "tabi", Count: bestTabi})
	}
	if bestMusinna > 0 {
		due = append(due, AnimalDue{Animal: "musinna", Count: bestMusinna})
	}
	return due
}
