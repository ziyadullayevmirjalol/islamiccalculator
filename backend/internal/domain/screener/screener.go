// Package screener implements AAOIFI-style halal stock screening: a
// business-activity screen followed by three financial-ratio screens.
// Thresholds arrive via Params — loaded from the screener_rules table —
// so a standards update never needs a code change. A ratio at or above
// its threshold fails ("must stay below").
package screener

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	VerdictCompliant    = "compliant"
	VerdictNonCompliant = "non_compliant"

	CheckDebt    = "debt_to_market_cap"
	CheckInvest  = "interest_investments_to_market_cap"
	CheckImpure  = "impure_income_to_revenue"
	CheckActivty = "business_activity"
)

type Params struct {
	DebtToMarketCapMax   decimal.Decimal // e.g. 0.30
	InvestToMarketCapMax decimal.Decimal // e.g. 0.30
	ImpureIncomeMax      decimal.Decimal // e.g. 0.05
}

type Input struct {
	// ProhibitedActivities the company engages in (alcohol, gambling,
	// conventional banking, …). Any entry fails the activity screen.
	ProhibitedActivities       []string
	InterestBearingDebt        decimal.Decimal
	InterestBearingInvestments decimal.Decimal // cash + interest-bearing securities
	MarketCap                  decimal.Decimal
	ImpureIncome               decimal.Decimal
	TotalRevenue               decimal.Decimal
}

type RuleCheck struct {
	Key       string
	Ratio     decimal.Decimal // 4 decimal places
	Threshold decimal.Decimal
	Passed    bool
}

type Result struct {
	Verdict           string
	ActivityPassed    bool
	FailedActivities  []string
	Checks            []RuleCheck
	PurificationRatio decimal.Decimal // impure income / revenue, for dividend purification
}

func Screen(in Input, p Params) (Result, error) {
	if err := validate(in, p); err != nil {
		return Result{}, err
	}

	check := func(key string, num, den, threshold decimal.Decimal) RuleCheck {
		ratio := num.Div(den).Round(4)
		return RuleCheck{
			Key:       key,
			Ratio:     ratio,
			Threshold: threshold,
			Passed:    ratio.LessThan(threshold),
		}
	}

	checks := []RuleCheck{
		check(CheckDebt, in.InterestBearingDebt, in.MarketCap, p.DebtToMarketCapMax),
		check(CheckInvest, in.InterestBearingInvestments, in.MarketCap, p.InvestToMarketCapMax),
		check(CheckImpure, in.ImpureIncome, in.TotalRevenue, p.ImpureIncomeMax),
	}

	activityPassed := len(in.ProhibitedActivities) == 0
	verdict := VerdictCompliant
	if !activityPassed {
		verdict = VerdictNonCompliant
	}
	for _, c := range checks {
		if !c.Passed {
			verdict = VerdictNonCompliant
		}
	}

	return Result{
		Verdict:           verdict,
		ActivityPassed:    activityPassed,
		FailedActivities:  in.ProhibitedActivities,
		Checks:            checks,
		PurificationRatio: in.ImpureIncome.Div(in.TotalRevenue).Round(6),
	}, nil
}

func validate(in Input, p Params) error {
	fields := map[string]string{}
	if !in.MarketCap.IsPositive() {
		fields["marketCap"] = "must_be_positive"
	}
	if !in.TotalRevenue.IsPositive() {
		fields["totalRevenue"] = "must_be_positive"
	}
	if in.InterestBearingDebt.IsNegative() {
		fields["interestBearingDebt"] = "must_not_be_negative"
	}
	if in.InterestBearingInvestments.IsNegative() {
		fields["interestBearingInvestments"] = "must_not_be_negative"
	}
	if in.ImpureIncome.IsNegative() {
		fields["impureIncome"] = "must_not_be_negative"
	} else if in.TotalRevenue.IsPositive() && in.ImpureIncome.GreaterThan(in.TotalRevenue) {
		fields["impureIncome"] = "exceeds_total_revenue"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid screener input", fields)
	}

	one := decimal.NewFromInt(1)
	for _, threshold := range []decimal.Decimal{p.DebtToMarketCapMax, p.InvestToMarketCapMax, p.ImpureIncomeMax} {
		if !threshold.IsPositive() || threshold.GreaterThanOrEqual(one) {
			return apperr.Internal("invalid screener thresholds", nil)
		}
	}
	return nil
}
