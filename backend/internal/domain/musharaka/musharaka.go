// Package musharaka implements partnership profit/loss distribution.
// The defining Shariah rule is structural here: profit may be split by
// any agreed ratio, but LOSS is always distributed in proportion to
// capital — the API offers no way to allocate a loss differently, just
// as qard al-hasan offers no way to express a percentage fee.
package musharaka

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	ResultProfit = "profit"
	ResultLoss   = "loss"

	MaxPartners = 20
)

type Partner struct {
	Name               string
	Capital            decimal.Decimal
	ProfitSharePercent decimal.Decimal // agreed profit ratio, (0..100); must sum to 100
}

type Input struct {
	Partners   []Partner
	ResultType string          // "profit" or "loss"
	Amount     decimal.Decimal // the period's profit or loss, always positive
}

type PartnerShare struct {
	Name               string
	Capital            decimal.Decimal
	CapitalShare       decimal.Decimal // capital / total capital, 4 decimal places
	ProfitSharePercent decimal.Decimal
	AppliedShare       decimal.Decimal // the ratio actually used for this distribution
	Amount             decimal.Decimal // this partner's slice of the profit or loss
}

type Result struct {
	TotalCapital decimal.Decimal
	ResultType   string
	Amount       decimal.Decimal
	Basis        string // "agreed_ratio" for profit, "capital_ratio" for loss
	Shares       []PartnerShare
}

func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	totalCapital := decimal.Zero
	for _, p := range in.Partners {
		totalCapital = totalCapital.Add(money.Round2(p.Capital))
	}

	amount := money.Round2(in.Amount)
	hundred := decimal.NewFromInt(100)

	basis := "agreed_ratio"
	if in.ResultType == ResultLoss {
		basis = "capital_ratio"
	}

	shares := make([]PartnerShare, len(in.Partners))
	distributed := decimal.Zero
	for i, p := range in.Partners {
		capital := money.Round2(p.Capital)

		var applied decimal.Decimal
		if in.ResultType == ResultLoss {
			// Loss follows capital — always. The agreed ratio is ignored
			// by construction, per the Shariah rule.
			applied = capital.Div(totalCapital)
		} else {
			applied = p.ProfitSharePercent.Div(hundred)
		}

		var slice decimal.Decimal
		if i == len(in.Partners)-1 {
			slice = amount.Sub(distributed) // absorb the rounding residual
		} else {
			slice = money.Round2(amount.Mul(applied))
		}
		distributed = distributed.Add(slice)

		shares[i] = PartnerShare{
			Name:               p.Name,
			Capital:            capital,
			CapitalShare:       capital.Div(totalCapital).Round(4),
			ProfitSharePercent: p.ProfitSharePercent,
			AppliedShare:       applied.Round(4),
			Amount:             slice,
		}
	}

	return Result{
		TotalCapital: totalCapital,
		ResultType:   in.ResultType,
		Amount:       amount,
		Basis:        basis,
		Shares:       shares,
	}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if in.ResultType != ResultProfit && in.ResultType != ResultLoss {
		fields["resultType"] = "must_be_profit_or_loss"
	}
	if !in.Amount.IsPositive() {
		fields["amount"] = "must_be_positive"
	}
	if len(in.Partners) < 2 || len(in.Partners) > MaxPartners {
		fields["partners"] = "need_2_to_20_partners"
		return apperr.Validation("invalid musharaka input", fields)
	}

	shareSum := decimal.Zero
	for i, p := range in.Partners {
		if !p.Capital.IsPositive() {
			fields[fmt.Sprintf("partners[%d].capital", i)] = "must_be_positive"
		}
		if !p.ProfitSharePercent.IsPositive() {
			// Every musharaka partner must share in profit; a zero share
			// would make this a loan, not a partnership.
			fields[fmt.Sprintf("partners[%d].profitSharePercent", i)] = "must_be_positive"
		}
		shareSum = shareSum.Add(p.ProfitSharePercent)
	}
	if len(fields) == 0 && !shareSum.Equal(decimal.NewFromInt(100)) {
		fields["partners"] = "profit_shares_must_sum_to_100"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid musharaka input", fields)
	}
	return nil
}
