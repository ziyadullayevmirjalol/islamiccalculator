// Package ijara implements Ijara Muntahia Bittamleek (lease-to-own):
// the bank buys the asset, leases it for a term, and transfers ownership
// at the end for a separate transfer price. Rent is a true rental — the
// transfer is a distinct promised sale, never folded into one contract.
package ijara

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// ModeProfit derives the rent from a target profit over the term.
	ModeProfit = "profit"
	// ModeRent derives the implied profit from a known monthly rent.
	ModeRent = "rent"

	ProfitModeRate   = "rate"
	ProfitModeAmount = "amount"

	MaxTermMonths = 360
)

type Profit struct {
	Mode  string
	Value decimal.Decimal
}

type Input struct {
	Mode           string
	AssetCost      decimal.Decimal
	Profit         Profit          // when Mode == profit
	MonthlyRent    decimal.Decimal // when Mode == rent
	TransferPrice  decimal.Decimal // ownership-transfer payment at term end
	AdvancePayment decimal.Decimal // advance rent paid at signing
	TermMonths     int
	FirstDueDate   time.Time // optional
}

type Rental struct {
	N       int
	DueDate time.Time
	Amount  decimal.Decimal
	Balance decimal.Decimal // scheduled rentals remaining after this payment
}

type Result struct {
	AssetCost      decimal.Decimal
	TransferPrice  decimal.Decimal
	AdvancePayment decimal.Decimal
	TotalRentals   decimal.Decimal // scheduled rentals (excl. advance and transfer)
	TotalReceipts  decimal.Decimal // advance + rentals + transfer: lessee's total outlay
	ProfitTotal    decimal.Decimal // receipts - asset cost (negative possible in rent mode)
	ProfitRate     decimal.Decimal // profit / cost, 4 decimal places
	MonthlyRent    decimal.Decimal
	TermMonths     int
	Schedule       []Rental
}

func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	cost := money.Round2(in.AssetCost)
	transfer := money.Round2(in.TransferPrice)
	advance := money.Round2(in.AdvancePayment)

	var rentalsTotal decimal.Decimal
	if in.Mode == ModeProfit {
		var profitTotal decimal.Decimal
		if in.Profit.Mode == ProfitModeRate {
			profitTotal = money.Round2(cost.Mul(in.Profit.Value))
		} else {
			profitTotal = money.Round2(in.Profit.Value)
		}
		rentalsTotal = cost.Add(profitTotal).Sub(transfer).Sub(advance)
		if !rentalsTotal.IsPositive() {
			return Result{}, apperr.Validation(
				"transfer price plus advance payment cover the whole target; nothing to schedule",
				map[string]string{"transferPrice": "too_large"})
		}
	} else {
		rentalsTotal = money.Round2(in.MonthlyRent).Mul(decimal.NewFromInt(int64(in.TermMonths)))
	}

	amounts, err := money.Split(rentalsTotal, in.TermMonths)
	if err != nil {
		return Result{}, apperr.Validation(err.Error(), nil)
	}

	receipts := advance.Add(rentalsTotal).Add(transfer)
	profit := receipts.Sub(cost)

	schedule := make([]Rental, in.TermMonths)
	balance := rentalsTotal
	for i, amount := range amounts {
		balance = balance.Sub(amount)
		r := Rental{N: i + 1, Amount: amount, Balance: balance}
		if !in.FirstDueDate.IsZero() {
			r.DueDate = in.FirstDueDate.AddDate(0, i, 0)
		}
		schedule[i] = r
	}

	return Result{
		AssetCost:      cost,
		TransferPrice:  transfer,
		AdvancePayment: advance,
		TotalRentals:   rentalsTotal,
		TotalReceipts:  receipts,
		ProfitTotal:    profit,
		ProfitRate:     profit.Div(cost).Round(4),
		MonthlyRent:    amounts[0],
		TermMonths:     in.TermMonths,
		Schedule:       schedule,
	}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if in.Mode != ModeProfit && in.Mode != ModeRent {
		fields["mode"] = "must_be_profit_or_rent"
	}
	if !in.AssetCost.IsPositive() {
		fields["assetCost"] = "must_be_positive"
	}
	if in.TermMonths < 1 || in.TermMonths > MaxTermMonths {
		fields["termMonths"] = "out_of_range"
	}
	if in.TransferPrice.IsNegative() {
		fields["transferPrice"] = "must_not_be_negative"
	}
	if in.AdvancePayment.IsNegative() {
		fields["advancePayment"] = "must_not_be_negative"
	}
	switch in.Mode {
	case ModeProfit:
		if in.Profit.Mode != ProfitModeRate && in.Profit.Mode != ProfitModeAmount {
			fields["profit.mode"] = "must_be_rate_or_amount"
		}
		if in.Profit.Value.IsNegative() {
			fields["profit.value"] = "must_not_be_negative"
		}
		// Same sanity bound as murabaha: >500% of cost in rate mode is a
		// percent-vs-fraction unit mistake, not a contract.
		if in.Profit.Mode == ProfitModeRate && in.Profit.Value.GreaterThan(decimal.NewFromInt(5)) {
			fields["profit.value"] = "out_of_range"
		}
	case ModeRent:
		if !in.MonthlyRent.IsPositive() {
			fields["monthlyRent"] = "must_be_positive"
		}
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid ijara input", fields)
	}
	return nil
}
